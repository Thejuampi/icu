package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	icu "github.com/Thejuampi/icu"
)

const (
	estimatePowerDefaultTypes = "watts,cadence,left_right_balance,altitude,distance,velocity_smooth,heartrate,time"
	estimatePowerDefaultCrr   = 0.0045
	estimatePowerDefaultEta   = 0.975
	estimatePowerCrrSource    = "user_default_flag_omitted"
	estimatePowerEtaSource    = "user_default_flag_omitted"
	estimatePowerSourceUser   = "user"
	outdoorWeatherHTTPTimeout = 8 * time.Second
	weatherRhoRoundScale      = 10000.0
)

func registerActivityEstimatePowerCommands(registry *CommandRegistry) {
	registry.Register("activity", "estimate-power", estimatePowerShowCommand())
	registry.Register("activity", "estimate-power-accept", estimatePowerAcceptCommand())
	registry.Register("activity", "estimate-power-backtest", estimatePowerBacktestCommand())
}

func estimatePowerShowCommand() *Command {
	return &Command{
		Name: "estimate-power",
		Usage: "activity <id> estimate-power [--rider-mass-kg N] [--bike-mass-kg N] [--cda N] [--crr N] " +
			"[--drivetrain-eff N] [--air-density N] [--calibrate-from-measured] [--min-gap-seconds N] " +
			"[--include-streams] [--file PATH] [--types CSV] [--no-weather] " +
			"[--refill-from-index N | --refill-after-pm-death | --refill-after-cadence-death]",
		Description: "Dry-run multi-source power estimation for missing power gaps (null watts or PM-death zeros). " +
			"Outdoor aero uses real weather (activity fields, then free Open-Meteo archive/forecast fallbacks) " +
			"with track heading for relative wind. Gap detection prefers left_right_balance (dual-sided PM L/R): " +
			"first half with real power+RPM+balance stays measured; long L/R null tail = death. " +
			"--refill-after-pm-death re-opens that tail even after a prior fill (balance → cadence fallback). " +
			"Does not mutate Intervals.icu.",
		Run: runEstimatePowerShowCommand,
	}
}

func runEstimatePowerShowCommand(args []string, flags map[string]string, client *icu.Client) error {
	if len(args) == 0 {
		return errMissing("activity id")
	}
	activityID := args[0]

	result, err := runEstimatePowerShow(client, activityID, flags)
	if err != nil {
		return err
	}

	if path := icu.StringFlag(flags, "file", ""); path != "" {
		if err := writeEstimatePowerFile(path, &result); err != nil {
			return err
		}
	}

	if !BoolFlag(flags, "include-streams") {
		result.FilledWatts = nil
		result.SampleSource = nil
	}

	return writeJSON(result)
}

func estimatePowerAcceptCommand() *Command {
	return &Command{
		Name:        "estimate-power-accept",
		Usage:       "activity <id> estimate-power-accept --file PATH",
		Description: "Write filled watts stream from a prior estimate-power file to Intervals.icu (mutates activity streams; supporter feature).",
		Run:         runEstimatePowerAcceptCommand,
	}
}

func estimatePowerBacktestCommand() *Command {
	return &Command{
		Name: "estimate-power-backtest",
		Usage: "activity <id> estimate-power-backtest --bike-mass-kg N [--rider-mass-kg N] [--crr N] " +
			"[--drivetrain-eff N] [--calibrate-from-measured] [--mode mask_second_half|mask_after_fraction] " +
			"[--mask-fraction 0.5] [--types CSV] [--no-weather]",
		Description: "Replay-style validation: mask measured power (PM-death simulation), re-estimate from GPS+mass+weather, " +
			"and score pearsonR/RMSE/bias 1:1 against held-out measured watts. Read-only.",
		Run: runEstimatePowerBacktestCommand,
	}
}

func runEstimatePowerBacktestCommand(args []string, flags map[string]string, client *icu.Client) error {
	if len(args) == 0 {
		return errMissing("activity id")
	}
	activityID := args[0]

	var activity icu.Activity
	if err := client.Get("activity", []string{activityID}, nil, &activity); err != nil {
		return wrapCommandError(err)
	}

	types := icu.StringFlag(flags, "types", estimatePowerDefaultTypes)
	var raw []icu.ActivityStream
	if err := client.Get("activity", []string{activityID, "streams"}, map[string]string{"types": types}, &raw); err != nil {
		return wrapCommandError(err)
	}
	streams, err := icu.PreserveNullableStreams(raw)
	if err != nil {
		return fmt.Errorf("normalize streams: %w", err)
	}

	// Prefer calibrate-from-measured for backtests unless user supplies CdA.
	if !BoolFlag(flags, "calibrate-from-measured") && floatFlagVal(flags, "cda", 0) <= 0 {
		flags["calibrate-from-measured"] = "true"
	}

	params, paramWarnings, err := resolveEstimatePowerParams(client, flags)
	if err != nil {
		return err
	}
	ftp, ftpSource := resolveEstimatePowerFTP(&activity, flags)

	aero, headwindSeries, rhoSeries, weatherWarnings := resolveEstimatePowerWeather(
		client, &activity, streams, flags, &params,
	)
	paramWarnings = append(paramWarnings, weatherWarnings...)

	mode := icu.StringFlag(flags, "mode", icu.PowerBacktestMaskSecondHalf)
	maskFrac := floatFlagVal(flags, "mask-fraction", 0.5)
	result := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		ActivityID:            activityID,
		Streams:               streams,
		Params:                params,
		Aero:                  aero,
		HeadwindMSSeries:      headwindSeries,
		RhoSeries:             rhoSeries,
		CalibrateFromMeasured: BoolFlag(flags, "calibrate-from-measured"),
		FTP:                   ftp,
		FTPSource:             ftpSource,
		Mode:                  mode,
		MaskAfterFraction:     maskFrac,
		ActivityType:          activity.Type,
		DistanceMeters:        activity.Distance,
	})
	result.Warnings = append(paramWarnings, result.Warnings...)
	// Drop bulky fill streams from default backtest JSON.
	result.Fill.FilledWatts = nil
	result.Fill.SampleSource = nil

	return writeJSON(result)
}

func runEstimatePowerAcceptCommand(args []string, flags map[string]string, client *icu.Client) error {
	if len(args) == 0 {
		return errMissing("activity id")
	}
	activityID := args[0]
	path := icu.StringFlag(flags, "file", "")
	if path == "" {
		return errors.New("estimate-power-accept requires --file from a prior estimate-power dry-run")
	}

	result, err := readEstimatePowerFile(path)
	if err != nil {
		return err
	}
	if result.ActivityID != "" && result.ActivityID != activityID {
		return fmt.Errorf("file activityId %s does not match %s", result.ActivityID, activityID)
	}
	if result.BlockingError != "" {
		return fmt.Errorf("cannot accept blocked estimate: %s", result.BlockingError)
	}
	watts := icu.BuildAcceptWattsStream(&result)
	if len(watts) == 0 {
		return errors.New("estimate file has no filledWatts; re-run estimate-power with --file (streams are stored in the file)")
	}

	body := []icu.ActivityStream{{
		Type: "watts",
		Data: watts,
	}}
	var updateResult icu.UpdateStreamsResult
	if err := client.Put("activity", []string{activityID, "streams"}, nil, body, &updateResult); err != nil {
		return wrapCommandError(err)
	}

	var activity icu.Activity
	if getErr := client.Get("activity", []string{activityID}, nil, &activity); getErr != nil {
		activity = icu.Activity{ID: activityID}
	}

	out := map[string]any{
		"activityId":  activityID,
		"updated":     updateResult,
		"sideEffects": map[string]bool{"mutatesActivity": true},
		"activity":    activity,
		"fill":        result.Fill,
		"metrics":     result.Metrics,
		"warnings":    append([]string{}, result.Warnings...),
	}

	return writeJSON(out)
}

func runEstimatePowerShow(client *icu.Client, activityID string, flags map[string]string) (icu.PowerFillResult, error) {
	var activity icu.Activity
	if err := client.Get("activity", []string{activityID}, nil, &activity); err != nil {
		return icu.PowerFillResult{}, wrapCommandError(err)
	}

	types := icu.StringFlag(flags, "types", estimatePowerDefaultTypes)
	var raw []icu.ActivityStream
	if err := client.Get("activity", []string{activityID, "streams"}, map[string]string{"types": types}, &raw); err != nil {
		return icu.PowerFillResult{}, wrapCommandError(err)
	}

	streams, err := icu.PreserveNullableStreams(raw)
	if err != nil {
		return icu.PowerFillResult{}, fmt.Errorf("normalize streams: %w", err)
	}

	var refillWarnings []string
	streams, refillWarnings, err = applyEstimatePowerRefillMask(streams, flags)
	if err != nil {
		return icu.PowerFillResult{}, err
	}

	params, paramWarnings, err := resolveEstimatePowerParams(client, flags)
	if err != nil {
		return icu.PowerFillResult{}, err
	}
	paramWarnings = append(refillWarnings, paramWarnings...)

	ftp, ftpSource := resolveEstimatePowerFTP(&activity, flags)
	aero, headwindSeries, rhoSeries, weatherWarnings := resolveEstimatePowerWeather(
		client, &activity, streams, flags, &params,
	)
	paramWarnings = append(paramWarnings, weatherWarnings...)

	result := icu.EstimateAndFillPower(icu.PowerFillRequest{
		ActivityID:            activityID,
		Streams:               streams,
		Params:                params,
		Aero:                  aero,
		HeadwindMSSeries:      headwindSeries,
		RhoSeries:             rhoSeries,
		CalibrateFromMeasured: BoolFlag(flags, "calibrate-from-measured"),
		FTP:                   ftp,
		FTPSource:             ftpSource,
		IncludeStreams:        true,
		MinGapSeconds:         IntFlag(flags, "min-gap-seconds", 0),
	})
	result.Warnings = append(paramWarnings, result.Warnings...)
	result.SideEffects.MutatesActivity = false

	return result, nil
}

// applyEstimatePowerRefillMask optionally re-opens a PM-death tail so a prior
// accepted fill can be re-estimated with the current weather/physics model.
// Priority: --refill-from-index > --refill-after-pm-death (L/R balance → cadence)
// > --refill-after-cadence-death (cadence-only).
func applyEstimatePowerRefillMask(
	streams icu.NullableStreamData,
	flags map[string]string,
) (icu.NullableStreamData, []string, error) {
	var warnings []string
	fromIndex := -1
	source := ""

	if raw := strings.TrimSpace(flags["refill-from-index"]); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return streams, warnings, fmt.Errorf("refill-from-index must be a non-negative integer, got %q", raw)
		}
		fromIndex = parsed
		source = "user"
	} else if BoolFlag(flags, "refill-after-pm-death") || BoolFlag(flags, "refill-after-cadence-death") {
		balance := icu.NullableStream(streams, "left_right_balance")
		cadence := icu.NullableStream(streams, "cadence")
		if BoolFlag(flags, "refill-after-pm-death") {
			fromIndex, source = icu.DetectPowerMeterDeathIndex(balance, cadence)
			if fromIndex < 0 {
				return streams, warnings, errors.New(
					"refill-after-pm-death: no L/R balance or cadence death tail found " +
						"(need dual-sided balance null after live segment, or end null-cadence tail)",
				)
			}
		} else {
			fromIndex = icu.DetectCadenceDeathIndex(cadence)
			source = icu.PowerDeathSourceCadence
			if fromIndex < 0 {
				return streams, warnings, errors.New(
					"refill-after-cadence-death: no end-anchored null-cadence tail found",
				)
			}
		}
	}

	if fromIndex < 0 {
		// Soft hint when a prior fill is sitting on a balance-death tail.
		if death, src := icu.DetectPowerMeterDeathIndex(
			icu.NullableStream(streams, "left_right_balance"),
			icu.NullableStream(streams, "cadence"),
		); death >= 0 {
			synthetic := 0
			watts := icu.NullableStream(streams, "watts")
			balance := icu.NullableStream(streams, "left_right_balance")
			for i := death; i < watts.Len(); i++ {
				w, wOK := watts.At(i)
				_, bOK := balance.At(i)
				if wOK && w > 0 && !bOK {
					synthetic++
				}
			}
			if synthetic >= 30 {
				warnings = append(warnings, fmt.Sprintf(
					"note: %d samples after %s death index %d have positive watts but no L/R balance "+
						"(likely prior fill); pass --refill-after-pm-death to re-estimate second half only",
					synthetic, src, death,
				))
			}
		}

		return streams, warnings, nil
	}

	n := 0
	if w := icu.NullableStream(streams, "watts"); w.Len() > 0 {
		n = w.Len()
	}
	if n > 0 && fromIndex >= n {
		return streams, warnings, fmt.Errorf("refill-from-index %d >= stream length %d", fromIndex, n)
	}
	warnings = append(warnings, fmt.Sprintf(
		"refill mask from index %d via %s (first half with real power/RPM/balance preserved; second-half gap only)",
		fromIndex, source,
	))

	return icu.MaskStreamsAsPowerMeterDeathFrom(streams, fromIndex), warnings, nil
}

// resolveEstimatePowerWeather loads real outdoor weather for aero.
// Priority: activity wind/temp fields → free Open-Meteo archive/forecast APIs.
// Optional --no-weather skips external fetch (still uses activity fields).
//
//nolint:gocritic,gocognit // multi-value weather bundle + multi-source resolve is intentional at CLI edge
func resolveEstimatePowerWeather(
	client *icu.Client,
	activity *icu.Activity,
	streams icu.NullableStreamData,
	flags map[string]string,
	params *icu.PowerModelParams,
) (icu.PowerAeroInputs, []float64, []float64, []string) {
	var aero icu.PowerAeroInputs
	var warnings []string
	if activity == nil || params == nil {
		return aero, nil, nil, warnings
	}
	if activity.Type != "" && !icu.IsOutdoorCyclingActivity(activity.Type) {
		warnings = append(warnings, fmt.Sprintf(
			"weather/aero wind skipped: activity type %q is not outdoor free-air cycling",
			activity.Type,
		))

		return aero, nil, nil, warnings
	}

	aero = activityAeroInputs(activity)
	sampleCount := streamSampleCount(streams)
	lat, lon, latlngs := activityMapLocation(client, activity.ID)
	startUTC := parseActivityStartUTC(activity)
	durationSec := activityDurationSec(activity, sampleCount)
	query := outdoorWeatherQueryFromActivity(activity, lat, lon, startUTC, durationSec)

	weather, weatherWarnings := fetchEstimateWeather(flags, query)
	warnings = append(warnings, weatherWarnings...)
	warnings = append(warnings, icu.ApplyOutdoorWeatherToAero(weather, params, &aero)...)

	headwindSeries, rhoSeries := buildWeatherSeries(
		streams, latlngs, weather, startUTC, sampleCount, params,
	)

	return aero, headwindSeries, rhoSeries, warnings
}

func activityAeroInputs(activity *icu.Activity) icu.PowerAeroInputs {
	return icu.PowerAeroInputs{
		MeanAltitudeM:     activity.AverageAltitude,
		MeanTempC:         firstPositive(activity.AverageWeatherTemp, activity.AverageTemp),
		WindSpeed:         activity.AverageWindSpeed,
		WindSpeedIsKmh:    true, // Intervals outdoor wind is typically km/h
		HeadwindPercent:   activity.HeadwindPercent,
		TailwindPercent:   activity.TailwindPercent,
		PrevailingWindDeg: float64(activity.PrevailingWindDeg),
	}
}

func streamSampleCount(streams icu.NullableStreamData) int {
	if watts := icu.NullableStream(streams, "watts"); watts.Len() > 0 {
		return watts.Len()
	}
	if dist := icu.NullableStream(streams, "distance"); dist.Len() > 0 {
		return dist.Len()
	}

	return 0
}

func activityMapLocation(client *icu.Client, activityID string) (float64, float64, [][]float64) {
	var mapData icu.MapData
	if err := client.Get("activity", []string{activityID, "map"}, nil, &mapData); err != nil {
		return 0, 0, nil
	}
	lat, lon, ok := icu.MapCentroid(mapData.Bounds, mapData.LatLngs)
	if !ok {
		return 0, 0, mapData.LatLngs
	}

	return lat, lon, mapData.LatLngs
}

func activityDurationSec(activity *icu.Activity, sampleCount int) int {
	if activity.ElapsedTime > 0 {
		return activity.ElapsedTime
	}
	if activity.MovingTime > 0 {
		return activity.MovingTime
	}
	if sampleCount > 0 {
		return sampleCount
	}

	return 0
}

func outdoorWeatherQueryFromActivity(
	activity *icu.Activity,
	lat, lon float64,
	startUTC time.Time,
	durationSec int,
) icu.OutdoorWeatherQuery {
	return icu.OutdoorWeatherQuery{
		Lat:                       lat,
		Lon:                       lon,
		StartUTC:                  startUTC,
		DurationSec:               durationSec,
		ActivityWindSpeed:         activity.AverageWindSpeed,
		ActivityWindSpeedIsKmh:    true,
		ActivityHeadwindPercent:   activity.HeadwindPercent,
		ActivityTailwindPercent:   activity.TailwindPercent,
		ActivityPrevailingWindDeg: float64(activity.PrevailingWindDeg),
		ActivityWeatherTempC:      activity.AverageWeatherTemp,
		ActivityDeviceTempC:       activity.AverageTemp,
		ActivityAvgAltitudeM:      activity.AverageAltitude,
	}
}

func fetchEstimateWeather(flags map[string]string, query icu.OutdoorWeatherQuery) (icu.OutdoorWeatherResult, []string) {
	if BoolFlag(flags, "no-weather") {
		local := query
		local.Lat, local.Lon = 0, 0

		return icu.ResolveOutdoorWeather(nil, local), []string{
			"external weather fetch disabled (--no-weather); activity fields only",
		}
	}
	httpClient := &http.Client{Timeout: outdoorWeatherHTTPTimeout}

	return icu.ResolveOutdoorWeather(httpClient, query), nil
}

func buildWeatherSeries(
	streams icu.NullableStreamData,
	latlngs [][]float64,
	weather icu.OutdoorWeatherResult,
	startUTC time.Time,
	sampleCount int,
	params *icu.PowerModelParams,
) ([]float64, []float64) {
	if sampleCount <= 0 || len(weather.Hours) == 0 || startUTC.IsZero() || params == nil {
		return nil, nil
	}
	timeSecs := denseTimeSecs(streams, sampleCount)
	var headwindSeries []float64
	headings := icu.HeadingSeriesFromLatLngs(latlngs, sampleCount)
	if len(headings) == sampleCount {
		headwindSeries = icu.HeadwindSeriesFromHours(weather.Hours, headings, timeSecs, startUTC, sampleCount)
	}
	altitude := icu.NullableStream(streams, "altitude")
	rhoSeries := icu.DensitySeriesFromHours(
		weather.Hours, altitude, timeSecs, startUTC, sampleCount,
		weather.MeanTempC, params.AirDensity.Value,
	)
	if params.AirDensity.Value <= 0 && weather.MeanRho > 0 {
		params.AirDensity = icu.LabeledParam{
			Value:  math.Round(weather.MeanRho*weatherRhoRoundScale) / weatherRhoRoundScale,
			Source: weather.Source + "_density",
		}
	}

	return headwindSeries, rhoSeries
}

func denseTimeSecs(streams icu.NullableStreamData, sampleCount int) []float64 {
	timeSeries := icu.NullableStream(streams, "time")
	timeSecs := make([]float64, sampleCount)
	for idx := 0; idx < sampleCount; idx++ {
		if value, ok := timeSeries.At(idx); ok {
			timeSecs[idx] = value
		} else {
			timeSecs[idx] = float64(idx)
		}
	}

	return timeSecs
}

func parseActivityStartUTC(activity *icu.Activity) time.Time {
	if activity == nil {
		return time.Time{}
	}
	// Prefer absolute start_date (UTC / offset).
	if activity.StartDate != "" {
		if t, err := time.Parse(time.RFC3339, activity.StartDate); err == nil {
			return t.UTC()
		}
		if t, err := time.Parse("2006-01-02T15:04:05Z", activity.StartDate); err == nil {
			return t.UTC()
		}
	}
	if activity.StartDateLocal != "" {
		// Local without zone: treat as UTC approximation for weather hour lookup.
		if t, err := time.Parse("2006-01-02T15:04:05", activity.StartDateLocal); err == nil {
			return t.UTC()
		}
	}

	return time.Time{}
}

func firstPositive(values ...float64) float64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}

	return 0
}

func resolveEstimatePowerParams(client *icu.Client, flags map[string]string) (icu.PowerModelParams, []string, error) {
	var warnings []string
	params := icu.PowerModelParams{}

	rider, riderSource, riderErr := resolveRiderMass(client, flags)
	if riderErr != nil {
		return params, warnings, riderErr
	}
	params.RiderMassKg = icu.LabeledParam{Value: rider, Source: riderSource}

	bike := floatFlagVal(flags, "bike-mass-kg", 0)
	if bike <= 0 {
		return params, warnings, errors.New("bike mass required: pass --bike-mass-kg")
	}
	params.BikeMassKg = icu.LabeledParam{Value: bike, Source: estimatePowerSourceUser}

	params.Crr, warnings = resolveCrr(flags, warnings)
	params.DrivetrainEff, warnings = resolveDrivetrainEff(flags, warnings)

	if air := floatFlagVal(flags, "air-density", 0); air > 0 {
		params.AirDensity = icu.LabeledParam{Value: air, Source: estimatePowerSourceUser}
	}
	if cda := floatFlagVal(flags, "cda", 0); cda > 0 {
		params.CdA = icu.LabeledParam{Value: cda, Source: estimatePowerSourceUser}
	} else if !BoolFlag(flags, "calibrate-from-measured") {
		return params, warnings, errors.New("CdA required: pass --cda or --calibrate-from-measured")
	}

	return params, warnings, nil
}

//nolint:gocritic // value/source/error triple is intentional
func resolveRiderMass(client *icu.Client, flags map[string]string) (float64, string, error) {
	if rider := floatFlagVal(flags, "rider-mass-kg", 0); rider > 0 {
		return rider, estimatePowerSourceUser, nil
	}
	var athlete icu.Athlete
	if err := client.Get("athlete", nil, nil, &athlete); err == nil {
		if athlete.ICUWeight > 0 {
			return athlete.ICUWeight, "athlete_icu_weight", nil
		}
		if athlete.Weight > 0 {
			return athlete.Weight, "athlete_weight", nil
		}
	}
	// Fall back to most recent wellness weight when athlete profile has none.
	if mass, source, ok := resolveRiderMassFromWellness(client); ok {
		return mass, source, nil
	}

	return 0, "", errors.New("rider mass required: pass --rider-mass-kg (athlete/wellness weight unavailable)")
}

//nolint:gocritic // mass/source/ok triple
func resolveRiderMassFromWellness(client *icu.Client) (float64, string, bool) {
	// Short recent window is enough for a mass anchor; avoid unbounded history fetch.
	newest := time.Now().UTC().Format("2006-01-02")
	oldest := time.Now().UTC().AddDate(0, 0, -42).Format("2006-01-02")
	var records []icu.Wellness
	query := map[string]string{"oldest": oldest, "newest": newest}
	if err := client.Get("wellness", nil, query, &records); err != nil || len(records) == 0 {
		return 0, "", false
	}
	// Prefer the newest record with a positive weight.
	for index := len(records) - 1; index >= 0; index-- {
		if records[index].Weight > 0 {
			return records[index].Weight, "wellness_weight", true
		}
	}

	return 0, "", false
}

//nolint:gocritic // param + warnings pair
func resolveCrr(flags map[string]string, warnings []string) (icu.LabeledParam, []string) {
	if crr := floatFlagVal(flags, "crr", 0); crr > 0 {
		return icu.LabeledParam{Value: crr, Source: estimatePowerSourceUser}, warnings
	}
	// When calibrating, leave Crr unset so the pure core can fit it from climbs.
	if BoolFlag(flags, "calibrate-from-measured") {
		warnings = append(warnings, "Crr not provided; will calibrate from low-speed measured climbs when possible")
		return icu.LabeledParam{}, warnings
	}
	warnings = append(warnings, fmt.Sprintf(
		"using Crr=%.4f (pass --crr to override; user-omission default, not a silent coaching threshold)",
		estimatePowerDefaultCrr,
	))

	return icu.LabeledParam{Value: estimatePowerDefaultCrr, Source: estimatePowerCrrSource}, warnings
}

//nolint:gocritic // param + warnings pair
func resolveDrivetrainEff(flags map[string]string, warnings []string) (icu.LabeledParam, []string) {
	if eta := floatFlagVal(flags, "drivetrain-eff", 0); eta > 0 {
		return icu.LabeledParam{Value: eta, Source: estimatePowerSourceUser}, warnings
	}
	warnings = append(warnings, fmt.Sprintf("using drivetrain-eff=%.3f (pass --drivetrain-eff to override)", estimatePowerDefaultEta))

	return icu.LabeledParam{Value: estimatePowerDefaultEta, Source: estimatePowerEtaSource}, warnings
}

//nolint:gocritic // ftp/source pair
func resolveEstimatePowerFTP(activity *icu.Activity, flags map[string]string) (int, string) {
	if value := IntFlag(flags, "ftp", 0); value > 0 {
		return value, estimatePowerSourceUser
	}
	if activity != nil && activity.FTP > 0 {
		return activity.FTP, "activity_icu_ftp"
	}
	if activity != nil && activity.FTPWatts > 0 {
		return activity.FTPWatts, "activity_pm_ftp"
	}

	return 0, ""
}

func writeEstimatePowerFile(path string, result *icu.PowerFillResult) error {
	if result == nil || len(result.FilledWatts) == 0 {
		return errors.New("internal: filled watts missing before write")
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal estimate-power file: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write estimate-power file: %w", err)
	}

	return nil
}

func readEstimatePowerFile(path string) (icu.PowerFillResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return icu.PowerFillResult{}, fmt.Errorf("read estimate-power file: %w", err)
	}
	var result icu.PowerFillResult
	if err := json.Unmarshal(data, &result); err != nil {
		return icu.PowerFillResult{}, fmt.Errorf("parse estimate-power file: %w", err)
	}

	return result, nil
}
