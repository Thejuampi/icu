package icu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Free, no-key Open-Meteo endpoints (fast public APIs).
// Overridable in tests via setOpenMeteoEndpointsForTest (internal_test).
//
//nolint:gochecknoglobals // test-only URL override for httptest; production defaults below.
var (
	openMeteoArchiveURL            = "https://archive-api.open-meteo.com/v1/archive"
	openMeteoForecastURL           = "https://api.open-meteo.com/v1/forecast"
	openMeteoHistoricalForecastURL = "https://historical-forecast-api.open-meteo.com/v1/forecast"
)

const (
	openMeteoUserAgent           = "icu-cli/outdoor-weather (https://github.com/Thejuampi/icu)"
	outdoorWeatherDefaultTimeout = 8 * time.Second
	outdoorWeatherMaxBodyBytes   = 4 << 20
	// Wind speeds above this as raw activity values are almost certainly km/h means.
	activityWindKmhHeuristicMS = 25.0
	// Mild altitude (m) before applying surface-pressure altitude scale.
	densityAltitudeScaleFloorM = 50.0
	// Open-Meteo forecast past_days caps.
	openMeteoMinPastDays = 1
	openMeteoMaxPastDays = 92
	openMeteoPastPadDays = 2
	// Hour pad for interpolation around the ride window.
	weatherHourPad = time.Hour
)

// OutdoorWeatherHour is one hourly reanalysis/forecast sample (SI units).
type OutdoorWeatherHour struct {
	TimeUnix    int64   `json:"timeUnix"`
	TempC       float64 `json:"tempC"`
	PressureHPa float64 `json:"pressureHpa,omitempty"` // surface pressure; 0 if unknown
	WindSpeedMS float64 `json:"windSpeedMs"`
	WindFromDeg float64 `json:"windFromDeg"` // meteorological: direction wind comes FROM
}

// OutdoorWeatherQuery is the input for resolving real outdoor weather for aero.
// Prefer activity-attached fields; external free APIs fill gaps.
type OutdoorWeatherQuery struct {
	Lat                       float64
	Lon                       float64
	StartUTC                  time.Time
	DurationSec               int
	ActivityWindSpeed         float64 // often km/h on Intervals outdoor rides
	ActivityWindSpeedIsKmh    bool
	ActivityHeadwindPercent   float64
	ActivityTailwindPercent   float64
	ActivityPrevailingWindDeg float64
	ActivityWeatherTempC      float64
	ActivityDeviceTempC       float64
	ActivityAvgAltitudeM      float64
}

// OutdoorWeatherResult is resolved weather used for outdoor aero density/wind.
type OutdoorWeatherResult struct {
	Source            string               `json:"source"`
	OK                bool                 `json:"ok"`
	Hours             []OutdoorWeatherHour `json:"hours,omitempty"`
	MeanTempC         float64              `json:"meanTempC,omitempty"`
	MeanWindSpeedMS   float64              `json:"meanWindSpeedMs,omitempty"`
	MeanWindFromDeg   float64              `json:"meanWindFromDeg,omitempty"`
	MeanPressureHPa   float64              `json:"meanPressureHpa,omitempty"`
	MeanHeadwindMS    float64              `json:"meanHeadwindMs,omitempty"`
	MeanRho           float64              `json:"meanRho,omitempty"`
	HasWindDirection  bool                 `json:"hasWindDirection,omitempty"`
	HasHeadTailShares bool                 `json:"hasHeadTailShares,omitempty"`
	Warnings          []string             `json:"warnings,omitempty"`
}

// ResolveOutdoorWeather builds outdoor weather from activity fields and free
// reanalysis/forecast APIs. Order:
//  1. Activity wind + temp when present (Intervals already matched GPS weather)
//  2. Open-Meteo archive (ERA5-class historical)
//  3. Open-Meteo forecast with past_days (recent rides)
//  4. Open-Meteo historical-forecast API
//
// Never invents wind from constants; still-air is an explicit failure mode.
func ResolveOutdoorWeather(httpClient *http.Client, query OutdoorWeatherQuery) OutdoorWeatherResult {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: outdoorWeatherDefaultTimeout}
	}
	temp := firstNonZero(query.ActivityWeatherTempC, query.ActivityDeviceTempC)
	if query.ActivityWindSpeed > 0 {
		return resolveFromActivityWind(httpClient, query, temp)
	}

	return resolveFromExternalWeather(httpClient, query, temp)
}

func resolveFromActivityWind(httpClient *http.Client, query OutdoorWeatherQuery, temp float64) OutdoorWeatherResult {
	windMS, src := activityWindToMS(query)
	result := OutdoorWeatherResult{
		OK:              true,
		Source:          src,
		MeanWindSpeedMS: windMS,
		MeanTempC:       temp,
		MeanRho:         resolveMeanRho(query.ActivityAvgAltitudeM, temp, 0),
	}
	result.MeanWindFromDeg = query.ActivityPrevailingWindDeg
	result.HasWindDirection = query.ActivityPrevailingWindDeg != 0 ||
		query.ActivityHeadwindPercent > 0 || query.ActivityTailwindPercent > 0
	if query.ActivityHeadwindPercent > 0 || query.ActivityTailwindPercent > 0 {
		net := (query.ActivityHeadwindPercent - query.ActivityTailwindPercent) / 100
		result.MeanHeadwindMS = windMS * net
		result.HasHeadTailShares = true
		result.Source = src + "_head_tail_pct"
	}
	if temp == 0 {
		result.Warnings = append(result.Warnings,
			"activity wind present but temperature unknown; density uses ISA altitude profile")
	}
	attachHourlyIfLocated(httpClient, query, &result)

	return result
}

func activityWindToMS(query OutdoorWeatherQuery) (float64, string) {
	if query.ActivityWindSpeedIsKmh || query.ActivityWindSpeed > activityWindKmhHeuristicMS {
		return query.ActivityWindSpeed * powerEstimateKmhToMS, "activity_wind_kmh"
	}

	return query.ActivityWindSpeed, "activity_wind_ms"
}

func attachHourlyIfLocated(httpClient *http.Client, query OutdoorWeatherQuery, result *OutdoorWeatherResult) {
	if result == nil || (query.Lat == 0 && query.Lon == 0) {
		return
	}
	hours, hourSrc, err := fetchOpenMeteoChain(httpClient, query)
	if err != nil || len(hours) == 0 {
		return
	}
	result.Hours = hours
	result.Warnings = append(result.Warnings,
		fmt.Sprintf("hourly weather from %s (activity wind kept as mean anchor)", hourSrc))
	means := meanWeatherHours(hours)
	if means.count == 0 {
		return
	}
	if result.MeanTempC == 0 {
		result.MeanTempC = means.temp
	}
	if means.pressure > 0 {
		result.MeanPressureHPa = means.pressure
		result.MeanRho = resolveMeanRho(query.ActivityAvgAltitudeM, result.MeanTempC, means.pressure)
	}
}

func resolveFromExternalWeather(httpClient *http.Client, query OutdoorWeatherQuery, temp float64) OutdoorWeatherResult {
	if query.Lat == 0 && query.Lon == 0 {
		return densityOnlyFallback(temp, query.ActivityAvgAltitudeM,
			"no activity wind and no lat/lon for external weather; aero uses still air")
	}
	if query.StartUTC.IsZero() {
		return OutdoorWeatherResult{
			Warnings: []string{"no activity start time for weather lookup; aero uses still air"},
		}
	}
	hours, src, err := fetchOpenMeteoChain(httpClient, query)
	if err != nil || len(hours) == 0 {
		warn := "weather APIs returned no hourly samples"
		if err != nil {
			warn = fmt.Sprintf("weather fetch failed: %v", err)
		}

		return densityOnlyFallback(temp, query.ActivityAvgAltitudeM, warn)
	}
	means := meanWeatherHours(hours)
	meanTemp := means.temp
	if meanTemp == 0 {
		meanTemp = temp
	}
	rho := resolveMeanRho(query.ActivityAvgAltitudeM, meanTemp, means.pressure)
	result := OutdoorWeatherResult{
		OK:               true,
		Source:           src,
		Hours:            hours,
		MeanTempC:        meanTemp,
		MeanWindSpeedMS:  means.wind,
		MeanWindFromDeg:  means.windDir,
		MeanPressureHPa:  means.pressure,
		HasWindDirection: true,
		MeanRho:          rho,
	}
	result.Warnings = append(result.Warnings, fmt.Sprintf(
		"outdoor weather from %s: wind=%.1f m/s from %.0f° temp=%.1f°C ρ=%.3f kg/m³",
		src, result.MeanWindSpeedMS, result.MeanWindFromDeg, result.MeanTempC, result.MeanRho,
	))

	return result
}

func densityOnlyFallback(temp, altitudeM float64, warn string) OutdoorWeatherResult {
	result := OutdoorWeatherResult{Warnings: []string{warn}}
	if temp == 0 && altitudeM == 0 {
		return result
	}
	result.MeanTempC = temp
	result.MeanRho = resolveMeanRho(altitudeM, temp, 0)
	result.OK = result.MeanRho > 0
	result.Source = "activity_temp_altitude_only"

	return result
}

func resolveMeanRho(altitudeM, tempC, pressureHPa float64) float64 {
	if pressureHPa > 0 && tempC != 0 {
		return AirDensityFromPressureTemp(pressureHPa, tempC)
	}
	if altitudeM != 0 || tempC != 0 {
		return airDensityFromAltitudeTemp(altitudeM, tempC)
	}

	return 0
}

// AirDensityFromPressureTemp is ideal-gas density from surface pressure (hPa) and °C.
func AirDensityFromPressureTemp(pressureHPa, tempC float64) float64 {
	if pressureHPa <= 0 {
		return 0
	}
	tempK := tempC + 273.15
	if tempK < 200 {
		tempK = 288.15
	}

	return (pressureHPa * 100) / (powerEstimateDryAirR * tempK)
}

// RelativeHeadwindMS is the along-path wind component (m/s).
// Positive = headwind (increases airspeed). WindFromDeg is meteorological (FROM).
// HeadingDeg is travel direction (TO).
func RelativeHeadwindMS(windSpeedMS, windFromDeg, headingDeg float64) float64 {
	if windSpeedMS <= 0 {
		return 0
	}
	delta := (windFromDeg - headingDeg) * math.Pi / 180

	return windSpeedMS * math.Cos(delta)
}

// BearingDeg returns initial bearing from (lat1,lon1) to (lat2,lon2) in degrees [0,360).
func BearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180
	yComp := math.Sin(deltaLon) * math.Cos(phi2)
	xComp := math.Cos(phi1)*math.Sin(phi2) - math.Sin(phi1)*math.Cos(phi2)*math.Cos(deltaLon)
	theta := math.Atan2(yComp, xComp) * 180 / math.Pi
	if theta < 0 {
		theta += 360
	}

	return theta
}

// HeadingSeriesFromLatLngs resamples map/track points to sampleCount headings (degrees).
// Missing/short tracks return nil.
func HeadingSeriesFromLatLngs(latlngs [][]float64, sampleCount int) []float64 {
	if sampleCount <= 0 || len(latlngs) < 2 {
		return nil
	}
	pts := compactLatLngs(latlngs)
	if len(pts) < 2 {
		return nil
	}
	trackHeadings := make([]float64, len(pts))
	for idx := range len(pts) - 1 {
		trackHeadings[idx] = BearingDeg(pts[idx][0], pts[idx][1], pts[idx+1][0], pts[idx+1][1])
	}
	trackHeadings[len(pts)-1] = trackHeadings[len(pts)-2]

	out := make([]float64, sampleCount)
	last := float64(len(pts) - 1)
	for idx := range sampleCount {
		trackIdx := 0
		if sampleCount > 1 {
			trackIdx = int(math.Round(float64(idx) * last / float64(sampleCount-1)))
		}
		if trackIdx < 0 {
			trackIdx = 0
		}
		if trackIdx >= len(trackHeadings) {
			trackIdx = len(trackHeadings) - 1
		}
		out[idx] = trackHeadings[trackIdx]
	}

	return out
}

func compactLatLngs(latlngs [][]float64) [][2]float64 {
	pts := make([][2]float64, 0, len(latlngs))
	for _, point := range latlngs {
		if len(point) < 2 {
			continue
		}
		if point[0] == 0 && point[1] == 0 {
			continue
		}
		pts = append(pts, [2]float64{point[0], point[1]})
	}

	return pts
}

// HeadwindSeriesFromHours builds per-sample along-path headwind (m/s) by
// interpolating hourly wind and combining with heading degrees.
func HeadwindSeriesFromHours(
	hours []OutdoorWeatherHour,
	headings []float64,
	timeSecs []float64,
	startUTC time.Time,
	sampleCount int,
) []float64 {
	if len(hours) == 0 || sampleCount <= 0 || len(headings) == 0 {
		return nil
	}
	out := make([]float64, sampleCount)
	for idx := range sampleCount {
		sec := sampleTimeSec(timeSecs, idx)
		tUnix := startUTC.Unix() + int64(sec)
		windSpeed, windFrom := interpolateWind(hours, tUnix)
		heading := 0.0
		if idx < len(headings) {
			heading = headings[idx]
		}
		out[idx] = RelativeHeadwindMS(windSpeed, windFrom, heading)
	}

	return out
}

// DensitySeriesFromHours builds per-sample air density from hourly P,T when
// available, else falls back to altitude+temp ISA series.
func DensitySeriesFromHours(
	hours []OutdoorWeatherHour,
	altitude NullableSeries,
	timeSecs []float64,
	startUTC time.Time,
	sampleCount int,
	meanTempC, fallbackRho float64,
) []float64 {
	out := make([]float64, sampleCount)
	for idx := range sampleCount {
		out[idx] = densityAtSample(hours, altitude, timeSecs, startUTC, idx, meanTempC, fallbackRho)
	}

	return out
}

func densityAtSample(
	hours []OutdoorWeatherHour,
	altitude NullableSeries,
	timeSecs []float64,
	startUTC time.Time,
	index int,
	meanTempC, fallbackRho float64,
) float64 {
	sec := sampleTimeSec(timeSecs, index)
	tUnix := startUTC.Unix() + int64(sec)
	temp, pressure := interpolateTempPressure(hours, tUnix)
	if temp == 0 {
		temp = meanTempC
	}
	if pressure > 0 && temp != 0 {
		rho := AirDensityFromPressureTemp(pressure, temp)
		if alt, ok := altitude.At(index); ok && math.Abs(alt) > densityAltitudeScaleFloorM {
			isaH := airDensityFromAltitudeTemp(alt, temp)
			isa0 := airDensityFromAltitudeTemp(0, temp)
			if isa0 > 0 && isaH > 0 {
				rho *= isaH / isa0
			}
		}

		return rho
	}
	if alt, ok := altitude.At(index); ok {
		return airDensityFromAltitudeTemp(alt, meanTempC)
	}
	if fallbackRho > 0 {
		return fallbackRho
	}

	return powerEstimateDefaultRho
}

func sampleTimeSec(timeSecs []float64, index int) float64 {
	if index < len(timeSecs) {
		return timeSecs[index]
	}

	return float64(index)
}

func interpolateWind(hours []OutdoorWeatherHour, tUnix int64) (float64, float64) {
	left, right, frac, ok := hourBracket(hours, tUnix)
	if !ok {
		return 0, 0
	}
	speed := left.WindSpeedMS + frac*(right.WindSpeedMS-left.WindSpeedMS)
	from := lerpAngleDeg(left.WindFromDeg, right.WindFromDeg, frac)

	return speed, from
}

func interpolateTempPressure(hours []OutdoorWeatherHour, tUnix int64) (float64, float64) {
	left, right, frac, ok := hourBracket(hours, tUnix)
	if !ok {
		return 0, 0
	}
	temp := left.TempC + frac*(right.TempC-left.TempC)
	pressure := left.PressureHPa + frac*(right.PressureHPa-left.PressureHPa)

	return temp, pressure
}

func hourBracket(hours []OutdoorWeatherHour, tUnix int64) (OutdoorWeatherHour, OutdoorWeatherHour, float64, bool) {
	if len(hours) == 0 {
		return OutdoorWeatherHour{}, OutdoorWeatherHour{}, 0, false
	}
	if tUnix <= hours[0].TimeUnix {
		return hours[0], hours[0], 0, true
	}
	last := hours[len(hours)-1]
	if tUnix >= last.TimeUnix {
		return last, last, 0, true
	}
	for idx := range len(hours) - 1 {
		left, right := hours[idx], hours[idx+1]
		if tUnix < left.TimeUnix || tUnix > right.TimeUnix {
			continue
		}
		span := float64(right.TimeUnix - left.TimeUnix)
		if span <= 0 {
			return left, left, 0, true
		}
		frac := float64(tUnix-left.TimeUnix) / span

		return left, right, frac, true
	}

	return last, last, 0, true
}

func lerpAngleDeg(startDeg, endDeg, frac float64) float64 {
	diff := math.Mod(endDeg-startDeg+540, 360) - 180

	return math.Mod(startDeg+frac*diff+360, 360)
}

type weatherMean struct {
	temp, wind, windDir, pressure float64
	count                         int
}

func meanWeatherHours(hours []OutdoorWeatherHour) weatherMean {
	var means weatherMean
	var sinSum, cosSum float64
	for _, hour := range hours {
		means.count++
		means.temp += hour.TempC
		means.wind += hour.WindSpeedMS
		means.pressure += hour.PressureHPa
		rad := hour.WindFromDeg * math.Pi / 180
		sinSum += math.Sin(rad)
		cosSum += math.Cos(rad)
	}
	if means.count == 0 {
		return means
	}
	n := float64(means.count)
	means.temp /= n
	means.wind /= n
	means.pressure /= n
	means.windDir = math.Atan2(sinSum, cosSum) * 180 / math.Pi
	if means.windDir < 0 {
		means.windDir += 360
	}

	return means
}

func fetchOpenMeteoChain(httpClient *http.Client, query OutdoorWeatherQuery) ([]OutdoorWeatherHour, string, error) {
	start := query.StartUTC.UTC()
	end := start.Add(time.Duration(maxInt(query.DurationSec, 3600)) * time.Second)
	startDate := start.Format("2006-01-02")
	endDate := end.Format("2006-01-02")
	age := time.Since(start)

	type attempt struct {
		base     string
		label    string
		pastDays int
	}
	attempts := make([]attempt, 0, 5)
	if age > 7*24*time.Hour {
		attempts = append(
			attempts,
			attempt{openMeteoArchiveURL, "open_meteo_archive", 0},
			attempt{openMeteoHistoricalForecastURL, "open_meteo_historical_forecast", 0},
		)
	}
	pastDays := int(age.Hours()/24) + openMeteoPastPadDays
	if pastDays < openMeteoMinPastDays {
		pastDays = openMeteoMinPastDays
	}
	if pastDays > openMeteoMaxPastDays {
		pastDays = openMeteoMaxPastDays
	}
	attempts = append(
		attempts,
		attempt{openMeteoForecastURL, "open_meteo_forecast_past", pastDays},
		attempt{openMeteoArchiveURL, "open_meteo_archive", 0},
		attempt{openMeteoHistoricalForecastURL, "open_meteo_historical_forecast", 0},
	)

	var lastErr error
	for _, item := range attempts {
		hours, err := fetchOpenMeteo(httpClient, item.base, query.Lat, query.Lon, startDate, endDate, item.pastDays)
		if err != nil {
			lastErr = err
			continue
		}
		filtered := filterHoursToWindow(hours, start, end)
		if len(filtered) == 0 {
			lastErr = errors.New("no hourly weather in ride window")
			continue
		}

		return filtered, item.label, nil
	}
	if lastErr != nil {
		return nil, "", lastErr
	}

	return nil, "", errors.New("no hourly weather from open-meteo endpoints")
}

func fetchOpenMeteo(
	httpClient *http.Client,
	base string,
	lat, lon float64,
	startDate, endDate string,
	pastDays int,
) ([]OutdoorWeatherHour, error) {
	values := url.Values{}
	values.Set("latitude", strconv.FormatFloat(lat, 'f', 5, 64))
	values.Set("longitude", strconv.FormatFloat(lon, 'f', 5, 64))
	values.Set("hourly", "temperature_2m,surface_pressure,wind_speed_10m,wind_direction_10m")
	values.Set("wind_speed_unit", "ms")
	values.Set("timezone", "UTC")
	values.Set("timeformat", "unixtime")
	if pastDays > 0 {
		values.Set("past_days", strconv.Itoa(pastDays))
		values.Set("forecast_days", "1")
	} else {
		values.Set("start_date", startDate)
		values.Set("end_date", endDate)
	}

	reqURL := base + "?" + values.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), outdoorWeatherDefaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("open-meteo request: %w", err)
	}
	req.Header.Set("User-Agent", openMeteoUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open-meteo fetch: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, outdoorWeatherMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("open-meteo read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200] + "…"
		}

		return nil, fmt.Errorf("open-meteo HTTP %d: %s", resp.StatusCode, msg)
	}

	return ParseOpenMeteoHours(body)
}

// ParseOpenMeteoHours decodes an Open-Meteo hourly JSON body into SI weather samples.
func ParseOpenMeteoHours(body []byte) ([]OutdoorWeatherHour, error) {
	var raw openMeteoResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode open-meteo: %w", err)
	}
	if raw.Hourly == nil || len(raw.Hourly.Time) == 0 {
		return nil, errors.New("open-meteo: empty hourly block")
	}
	out := make([]OutdoorWeatherHour, 0, len(raw.Hourly.Time))
	for idx := range raw.Hourly.Time {
		hour := hourFromOpenMeteo(raw.Hourly, idx)
		if hour.WindSpeedMS == 0 && hour.TempC == 0 && hour.PressureHPa == 0 {
			continue
		}
		out = append(out, hour)
	}
	if len(out) == 0 {
		return nil, errors.New("open-meteo: no usable hourly samples")
	}

	return out, nil
}

type openMeteoResponse struct {
	Hourly *openMeteoHourly `json:"hourly"`
}

type openMeteoHourly struct {
	Time             []int64    `json:"time"`
	Temperature2m    []*float64 `json:"temperature_2m"`
	SurfacePressure  []*float64 `json:"surface_pressure"`
	WindSpeed10m     []*float64 `json:"wind_speed_10m"`
	WindDirection10m []*float64 `json:"wind_direction_10m"`
}

func hourFromOpenMeteo(hourly *openMeteoHourly, index int) OutdoorWeatherHour {
	hour := OutdoorWeatherHour{TimeUnix: hourly.Time[index]}
	hour.TempC = derefFloatAt(hourly.Temperature2m, index)
	hour.PressureHPa = derefFloatAt(hourly.SurfacePressure, index)
	hour.WindSpeedMS = derefFloatAt(hourly.WindSpeed10m, index)
	hour.WindFromDeg = derefFloatAt(hourly.WindDirection10m, index)

	return hour
}

func derefFloatAt(values []*float64, index int) float64 {
	if index < 0 || index >= len(values) || values[index] == nil {
		return 0
	}

	return *values[index]
}

func filterHoursToWindow(hours []OutdoorWeatherHour, start, end time.Time) []OutdoorWeatherHour {
	lo := start.Add(-weatherHourPad).Unix()
	hi := end.Add(weatherHourPad).Unix()
	out := make([]OutdoorWeatherHour, 0, len(hours))
	for _, hour := range hours {
		if hour.TimeUnix >= lo && hour.TimeUnix <= hi {
			out = append(out, hour)
		}
	}
	if len(out) == 0 {
		return hours
	}

	return out
}

// MapCentroid returns the average of valid latlng points, or bounds center.
func MapCentroid(bounds, latlngs [][]float64) (float64, float64, bool) {
	var sumLat, sumLon float64
	var count int
	for _, point := range latlngs {
		if len(point) < 2 {
			continue
		}
		if point[0] == 0 && point[1] == 0 {
			continue
		}
		sumLat += point[0]
		sumLon += point[1]
		count++
	}
	if count > 0 {
		return sumLat / float64(count), sumLon / float64(count), true
	}
	if len(bounds) >= 2 && len(bounds[0]) >= 2 && len(bounds[1]) >= 2 {
		lat := (bounds[0][0] + bounds[1][0]) / 2
		lon := (bounds[0][1] + bounds[1][1]) / 2
		if lat != 0 || lon != 0 {
			return lat, lon, true
		}
	}

	return 0, 0, false
}

// ApplyOutdoorWeatherToAero fills PowerAeroInputs and optional mean headwind/density
// params from a resolved OutdoorWeatherResult. Pure (no IO).
func ApplyOutdoorWeatherToAero(
	weather OutdoorWeatherResult,
	params *PowerModelParams,
	aero *PowerAeroInputs,
) []string {
	var warnings []string
	if aero == nil || params == nil {
		return warnings
	}
	if weather.MeanTempC != 0 {
		aero.MeanTempC = weather.MeanTempC
	}
	if weather.MeanWindSpeedMS > 0 {
		aero.WindSpeed = weather.MeanWindSpeedMS
		aero.WindSpeedIsKmh = false
	}
	if weather.MeanRho > 0 && params.AirDensity.Value <= 0 {
		params.AirDensity = LabeledParam{Value: round4(weather.MeanRho), Source: weather.Source + "_density"}
		warnings = append(warnings, fmt.Sprintf(
			"air density ρ=%.3f kg/m³ from %s", weather.MeanRho, weather.Source,
		))
	}
	if weather.HasHeadTailShares && weather.MeanHeadwindMS != 0 && params.HeadwindMS.Source == "" {
		params.HeadwindMS = LabeledParam{Value: round4(weather.MeanHeadwindMS), Source: weather.Source}
		warnings = append(warnings, fmt.Sprintf(
			"aero mean headwind %.2f m/s from activity head/tail wind shares (%s)",
			weather.MeanHeadwindMS, weather.Source,
		))
	} else if weather.HasWindDirection && weather.MeanWindSpeedMS > 0 && params.HeadwindMS.Source == "" {
		warnings = append(warnings, fmt.Sprintf(
			"weather wind %.1f m/s from %.0f° available; per-sample headwind used when track headings exist",
			weather.MeanWindSpeedMS, weather.MeanWindFromDeg,
		))
	}
	warnings = append(warnings, weather.Warnings...)

	return warnings
}

// MeanHeadwindFromSeries is the robust mean of a headwind series (for param label).
func MeanHeadwindFromSeries(series []float64) float64 {
	if len(series) == 0 {
		return 0
	}
	cp := append([]float64(nil), series...)

	return medianFloat64(cp)
}

func firstNonZero(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}

	return 0
}
