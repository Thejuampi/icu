package icu

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Protocol / unit constants only (not coaching decision thresholds).
const (
	powerEstimateGravity       = 9.80665
	powerEstimateDefaultRho    = 1.225 // kg/m^3 ISA sea level when weather/altitude absent
	powerEstimateJoulesPerKj   = 1000.0
	powerEstimateSecsPerHour   = 3600.0
	powerEstimateTSSScale      = 100.0
	powerEstimateRound4Scale   = 10000.0
	powerParamSourceUser       = "user"
	powerParamSourceCalibrated = "calibrated"
	// ISA / ideal-gas constants for air density from altitude + temperature.
	powerEstimateSeaLevelPa   = 101325.0
	powerEstimateDryAirR      = 287.05 // J/(kg·K)
	powerEstimateISALapse     = 0.0065 // K/m
	powerEstimateISASeaTempC  = 15.0
	powerEstimateISAExpo      = 5.25588
	powerEstimateISAPressCoef = 2.25577e-5
	powerEstimateKmhToMS      = 1.0 / 3.6
)

// LabeledParam is a numeric model parameter with provenance.
type LabeledParam struct {
	Value  float64 `json:"value"`
	Source string  `json:"source"`
}

// PowerModelParams are rider/bike/environment inputs for virtual power.
type PowerModelParams struct {
	RiderMassKg   LabeledParam `json:"riderMassKg"`
	BikeMassKg    LabeledParam `json:"bikeMassKg"`
	CdA           LabeledParam `json:"cda"`
	Crr           LabeledParam `json:"crr"`
	DrivetrainEff LabeledParam `json:"drivetrainEff"`
	// AirDensity is the baseline ρ (kg/m³). Per-sample altitude may override via rho series.
	AirDensity LabeledParam `json:"airDensity"`
	// HeadwindMS is the mean along-path wind component in m/s (positive = headwind).
	// Airspeed = groundSpeed + HeadwindMS; aero force uses airspeed, power uses ground speed.
	HeadwindMS LabeledParam `json:"headwindMs,omitempty"`
}

// PowerAeroInputs are optional outdoor environment fields used to derive density and wind.
// All dynamic from the activity/weather — not coaching defaults.
type PowerAeroInputs struct {
	MeanAltitudeM     float64 // activity average altitude (m)
	MeanTempC         float64 // activity average temperature (°C); 0 = unknown
	WindSpeed         float64 // activity/weather wind speed (Intervals typically km/h)
	WindSpeedIsKmh    bool    // true when WindSpeed is km/h
	HeadwindPercent   float64 // 0–100 share of time with headwind
	TailwindPercent   float64 // 0–100 share of time with tailwind
	PrevailingWindDeg float64 // meteorological from-direction (unused without heading)
}

// PowerFillRequest is the pure input for estimate-and-fill.
type PowerFillRequest struct {
	ActivityID string
	Streams    NullableStreamData
	Params     PowerModelParams
	Aero       PowerAeroInputs // outdoor density/wind derivation
	// HeadwindMSSeries is optional per-sample along-path wind (m/s, + = headwind).
	// Built from real weather × track heading. When empty, params.HeadwindMS is used.
	HeadwindMSSeries []float64
	// RhoSeries optional per-sample air density (kg/m³) from weather pressure/temp.
	// When empty, density is derived from altitude stream + Aero mean temp.
	RhoSeries             []float64
	CalibrateFromMeasured bool
	FTP                   int
	FTPSource             string
	IncludeStreams        bool
	MinGapSeconds         int
}

// PowerFillCalibration summarizes dynamic fit quality and measured-data guards.
type PowerFillCalibration struct {
	UsedMeasuredSeconds  int     `json:"usedMeasuredSeconds"`
	FitErrorWattsRMSE    float64 `json:"fitErrorWattsRmse"`
	FitBiasWatts         float64 `json:"fitBiasWatts,omitempty"`
	ResidualScale        float64 `json:"residualScale,omitempty"`
	LinearGain           float64 `json:"linearGain,omitempty"`   // est = gain*physics + offset
	LinearOffset         float64 `json:"linearOffset,omitempty"` // fitted on measured half
	InstantCapWatts      float64 `json:"instantCapWatts,omitempty"`
	DescentCoastRate     float64 `json:"descentCoastRate,omitempty"`
	EnvelopeApplied      bool    `json:"envelopeApplied,omitempty"`
	LocalBaselineApplied bool    `json:"localBaselineApplied,omitempty"`
}

// PowerFillModel describes the estimator used.
type PowerFillModel struct {
	Name        string               `json:"name"`
	Parameters  PowerModelParams     `json:"parameters"`
	Calibration PowerFillCalibration `json:"calibration"`
}

// PowerFillSummary counts energy written into gaps.
type PowerFillSummary struct {
	EstimatedSeconds   int     `json:"estimatedSeconds"`
	EstimatedKj        float64 `json:"estimatedKj"`
	MeanEstimatedWatts float64 `json:"meanEstimatedWatts"`
	UnfilledSeconds    int     `json:"unfilledSeconds"`
}

// PowerFillMetricsPair is before/after stream metrics.
type PowerFillMetricsPair struct {
	AvgWatts              float64 `json:"avgWatts"`
	NormalizedPower       float64 `json:"normalizedPower"`
	EstimatedTrainingLoad float64 `json:"estimatedTrainingLoad,omitempty"`
	FTP                   int     `json:"ftp,omitempty"`
	FTPSource             string  `json:"ftpSource,omitempty"`
}

// PowerFillMetrics wraps before/after metrics.
type PowerFillMetrics struct {
	Before PowerFillMetricsPair `json:"before"`
	After  PowerFillMetricsPair `json:"after"`
}

// PowerFillSideEffects declares mutation intent for CLI consumers.
type PowerFillSideEffects struct {
	MutatesActivity bool `json:"mutatesActivity"`
}

// PowerFillResult is the dry-run estimate contract.
type PowerFillResult struct {
	ActivityID     string                 `json:"activityId,omitempty"`
	Classification PowerGapClassification `json:"classification"`
	Model          PowerFillModel         `json:"model"`
	Fill           PowerFillSummary       `json:"fill"`
	Metrics        PowerFillMetrics       `json:"metrics"`
	FilledWatts    []float64              `json:"filledWatts,omitempty"`
	SampleSource   []string               `json:"sampleSource,omitempty"`
	Warnings       []string               `json:"warnings,omitempty"`
	SideEffects    PowerFillSideEffects   `json:"sideEffects"`
	BlockingError  string                 `json:"blockingError,omitempty"`
}

// measuredContext holds dynamic statistics learned only from this activity's
// measured power segment (no global coaching cutoffs).
type measuredContext struct {
	speedMedian   float64
	speedMAD      float64
	cadenceMedian float64
	gradeMAD      float64
	hrMedian      float64
	hrMAD         float64
	// Descent coasting learned from measured true zeros / zero watts on negative grade.
	descentCoastRate float64
	descentGradeCut  float64 // grade below this is "descent" from measured distribution
	// Powered measured samples for local neighborhood baselines.
	samples []measuredSample
	// Rolling MMP durations derived from measured length (fractions of coverage).
	envelopeSecs []int
	// Instant cap from high measured percentile.
	instantCap float64
	// Minimum samples required for calibration (dynamic from measured count).
	minCalRows int
	// HR bins for outdoor effort→power mapping (dynamic from measured HR range).
	hrBins []hrPhysBin
}

type measuredSample struct {
	index     int
	speed     float64
	grade     float64
	hr        float64 // 0 if absent
	speedRoll float64 // short rolling mean (dynamic window)
	gradeRoll float64
	hrRoll    float64
	phys      float64 // Martin physics at this sample (after cal)
	residual  float64 // watts - phys (measured power residual)
	watts     float64
	zero      bool // true zero / coast
}

// multiLinearPower is watts ≈ offset + physGain*phys + hrGain*hr + speedGain*speed + gradeGain*grade
// fitted only on this activity's measured outdoor samples.
type multiLinearPower struct {
	ok        bool
	offset    float64
	physGain  float64
	hrGain    float64
	speedGain float64
	gradeGain float64
	r2        float64
}

// outdoorBlendWeights combine candidate outdoor estimators. Learned on measured
// samples via leave-one-out style subsample RMSE (dynamic per activity).
type outdoorBlendWeights struct {
	wDirect float64 // direct watts kNN
	wResid  float64 // phys + residual kNN
	wLin    float64 // multi-linear
	wHR     float64 // HR-bin watts + physics residual
	ok      bool
}

// hrPhysBin is a dynamic HR bin with measured median watts and physics.
type hrPhysBin struct {
	hrLo, hrHi  float64
	medianWatts float64
	medianPhys  float64
	count       int
}

// EstimateAndFillPower classifies blanks, calibrates from measured data when asked,
// and fills missing samples with grade-aware virtual power.
//
//nolint:gocritic // value receiver keeps call sites simple; work happens on a pointer copy
func EstimateAndFillPower(req PowerFillRequest) PowerFillResult {
	return estimateAndFillPower(&req)
}

func estimateAndFillPower(req *PowerFillRequest) PowerFillResult {
	result := PowerFillResult{
		ActivityID:  req.ActivityID,
		SideEffects: PowerFillSideEffects{MutatesActivity: false},
	}

	watts := NullableStream(req.Streams, "watts")
	cadence := NullableStream(req.Streams, "cadence")
	speed := NullableStream(req.Streams, "velocity_smooth")
	if speed.Len() == 0 {
		speed = NullableStream(req.Streams, "velocity")
	}
	distance := NullableStream(req.Streams, "distance")
	altitude := NullableStream(req.Streams, "altitude")
	timeSeries := NullableStream(req.Streams, "time")
	hr := NullableStream(req.Streams, "heartrate")

	balance := NullableStream(req.Streams, "left_right_balance")
	class := ClassifyPowerSamples(&PowerGapInputs{
		Watts: watts, Cadence: cadence, Balance: balance,
		Speed: speed, Distance: distance, Time: timeSeries,
	})
	result.Classification = class
	result.Warnings = append(result.Warnings, class.Warnings...)

	sampleCount := len(class.Labels)
	if sampleCount == 0 {
		result.BlockingError = "no stream samples available"
		return result
	}

	params, aeroWarnings, err := resolvePowerModelParams(req)
	if err != nil {
		result.BlockingError = err.Error()
		result.Model = PowerFillModel{Name: "martin_balance", Parameters: params}
		return result
	}
	result.Warnings = append(result.Warnings, aeroWarnings...)

	// Dynamic smooth windows from series length (protocol: longer rides → wider GPS smooth).
	smoothHalf := dynamicHalfWindow(sampleCount, 0.0005, 2, 15)
	gradeHalf := dynamicHalfWindow(sampleCount, 0.001, 5, 25)
	speedDense := smoothSpeedSeries(deriveSpeedSeries(&PowerGapInputs{
		Speed: speed, Distance: distance, Time: timeSeries,
	}, sampleCount), smoothHalf)
	grade := deriveGradeSeries(distance, altitude, sampleCount, gradeHalf)
	// Per-sample air density: prefer weather-derived series, else altitude+temp ISA.
	rhoSeries := req.RhoSeries
	if len(rhoSeries) != sampleCount {
		rhoSeries = airDensitySeries(altitude, sampleCount, req.Aero.MeanTempC, params.AirDensity.Value)
	}
	headwindSeries := req.HeadwindMSSeries
	if len(headwindSeries) != sampleCount {
		headwindSeries = nil
	}
	// Acceleration is not used in virtual power: GPS Δv is too noisy. Still computed for cal filters.
	accel := deriveAccelSeries(speedDense, timeSeries, sampleCount)

	ctx := buildMeasuredContext(class.Labels, watts, cadence, speedDense, grade, hr, sampleCount)
	if ctx.minCalRows < 10 {
		ctx.minCalRows = 10 // absolute minimum for any median statistic
	}

	params, cal, calWarnings := maybeCalibrateParams(&params, req.CalibrateFromMeasured, class.Labels, watts, cadence, speedDense, grade, accel, &ctx)
	result.Warnings = append(result.Warnings, calWarnings...)
	if params.CdA.Source == "" || params.CdA.Value <= 0 {
		result.BlockingError = "CdA required: pass --cda or --calibrate-from-measured with enough measured samples"
		result.Model = PowerFillModel{Name: "martin_balance_dynamic", Parameters: params, Calibration: cal}
		return result
	}
	if params.Crr.Value <= 0 {
		result.BlockingError = "Crr required: pass --crr or provide enough low-speed measured samples for calibration"
		result.Model = PowerFillModel{Name: "martin_balance_dynamic", Parameters: params, Calibration: cal}
		return result
	}

	cal.DescentCoastRate = round2(ctx.descentCoastRate)
	hrDense := make([]float64, sampleCount)
	for i := 0; i < sampleCount; i++ {
		if h, ok := hr.At(i); ok {
			hrDense[i] = h
		}
	}
	// Rolling features + physics residuals on measured samples (outdoor residual model).
	rollHalf := dynamicHalfWindow(sampleCount, 0.002, 5, 20)
	speedRoll := smoothSpeedSeries(speedDense, rollHalf)
	gradeRoll := smoothSpeedSeries(grade, rollHalf)
	hrRoll := smoothSpeedSeries(hrDense, rollHalf)
	attachPhysicsResiduals(&ctx, class.Labels, watts, speedDense, grade, rhoSeries, headwindSeries, &params)
	attachRollingFeatures(&ctx, class.Labels, speedRoll, gradeRoll, hrRoll, sampleCount)
	ctx.hrBins = buildHRPhysBins(&ctx)

	// Multi-linear: watts ≈ g0 + gPhys*phys + gHR*hr + gSpd*speed + gGrd*grade (measured half).
	lin := fitMultiLinearPower(class.Labels, watts, speedDense, grade, hrDense, &params, &ctx)
	blend := fitOutdoorBlendWeights(&ctx, &params, &lin)
	cal.LinearGain = round4(lin.physGain)
	cal.LinearOffset = round2(lin.offset)
	cal.ResidualScale = round4(blend.wResid)
	cal.InstantCapWatts = ctx.instantCap
	if lin.ok {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"multi-linear outdoor model watts=%.2f*phys%+.3f*hr%+.1f*speed%+.0f*grade%+.1f (r2=%.2f on measured)",
			lin.physGain, lin.hrGain, lin.speedGain, lin.gradeGain, lin.offset, lin.r2,
		))
	}
	if blend.ok {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"outdoor ensemble blend direct=%.2f residual=%.2f linear=%.2f hrBin=%.2f (tuned on measured)",
			blend.wDirect, blend.wResid, blend.wLin, blend.wHR,
		))
	}
	if len(headwindSeries) == sampleCount {
		meanHW := MeanHeadwindFromSeries(headwindSeries)
		if params.HeadwindMS.Source == "" {
			params.HeadwindMS = LabeledParam{Value: round4(meanHW), Source: "weather_track_heading"}
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"per-sample aero headwind from real weather × track heading (mean %.2f m/s)",
			meanHW,
		))
	}

	filled, sources := fillPowerSeriesOutdoor(
		class.Labels, watts, speedDense, grade, hrDense, speedRoll, gradeRoll, hrRoll, rhoSeries, headwindSeries,
		&params, &ctx, &lin, &blend,
		speed.Len() > 0 || distance.Len() > 0, timeSeries, &result.Fill,
	)
	// Cap spikes at measured p99 only (no aggressive envelope that flattens real surges).
	applyInstantCap(filled, sources, ctx.instantCap)
	cal.LocalBaselineApplied = true

	if ctx.descentCoastRate > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"descent coast rate %.0f%% from measured negative-grade samples",
			ctx.descentCoastRate*100,
		))
	}

	// Light temporal smooth on estimates only.
	smoothHalfEst := dynamicHalfWindow(sampleCount, 0.0003, 1, 3)
	smoothEstimatedSamples(filled, sources, smoothHalfEst)

	// Soft envelope: only clamp multi-minute means that exceed measured MMP by a
	// large margin (MAD-based), so normal outdoor variation is preserved.
	if softEnforceMeasuredMMPEnvelope(filled, sources, timeSeries, class.Labels, watts, ctx.envelopeSecs, &ctx) {
		cal.EnvelopeApplied = true
		result.Warnings = append(
			result.Warnings,
			"soft MMP envelope applied where estimated multi-minute means far exceeded measured envelope",
		)
	}
	recomputeFillSummary(filled, sources, timeSeries, &result.Fill)

	result.Model = PowerFillModel{Name: "outdoor_residual_knn", Parameters: params, Calibration: cal}

	if req.MinGapSeconds > 0 && result.Fill.EstimatedSeconds < req.MinGapSeconds {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("estimated gap %ds below --min-gap-seconds %d", result.Fill.EstimatedSeconds, req.MinGapSeconds))
	}

	result.Metrics = buildFillMetrics(watts, filled, sampleCount, timeSeries, req.FTP, req.FTPSource)
	result.Warnings = append(result.Warnings, hrCrossCheckWarnings(class.Labels, watts, hr, result.Fill.MeanEstimatedWatts, &ctx)...)
	result.FilledWatts = filled
	result.SampleSource = sources

	return result
}

func dynamicHalfWindow(sampleCount int, fraction float64, minHalf, maxHalf int) int {
	if sampleCount <= 0 {
		return minHalf
	}
	half := int(float64(sampleCount) * fraction)
	if half < minHalf {
		half = minHalf
	}
	if half > maxHalf {
		half = maxHalf
	}

	return half
}

func buildMeasuredContext(
	labels []string,
	watts, cadence NullableSeries,
	speed, grade []float64,
	hr NullableSeries,
	sampleCount int,
) measuredContext {
	var speeds, cads, grades, hrs, positiveWatts []float64
	var descentTotal, descentCoast int
	samples := make([]measuredSample, 0, sampleCount)
	measuredCount := 0

	// Descent grade cut starts at 0 (true negative slope). Tightened from measured
	// grade distribution when enough descending samples exist.
	descentCut := 0.0

	for index := 0; index < sampleCount && index < len(labels); index++ {
		label := labels[index]
		switch label {
		case PowerSampleMeasured, PowerSampleTrueZero:
			// keep
		default:
			continue
		}
		measuredCount++
		w, wOK := watts.At(index)
		if !wOK {
			w = 0
		}
		hrVal := 0.0
		if h, hOK := hr.At(index); hOK && h > 0 {
			hrVal = h
			hrs = append(hrs, h)
		}
		isZero := label == PowerSampleTrueZero || w <= 0
		samples = append(samples, measuredSample{
			index: index,
			speed: speed[index],
			grade: grade[index],
			hr:    hrVal,
			watts: math.Max(0, w),
			zero:  isZero,
		})
		if speed[index] > 0 {
			speeds = append(speeds, speed[index])
		}
		grades = append(grades, grade[index])
		if c, ok := cadence.At(index); ok && c > 0 {
			cads = append(cads, c)
		}
		if w > 0 {
			positiveWatts = append(positiveWatts, w)
		}
		if grade[index] < descentCut {
			descentTotal++
			if isZero {
				descentCoast++
			}
		}
	}

	ctx := measuredContext{
		samples:         samples,
		speedMedian:     medianFloat64(speeds),
		speedMAD:        madFloat64(speeds, medianFloat64(speeds)),
		cadenceMedian:   medianFloat64(cads),
		gradeMAD:        madFloat64(grades, medianFloat64(grades)),
		hrMedian:        medianFloat64(hrs),
		hrMAD:           madFloat64(hrs, medianFloat64(hrs)),
		descentGradeCut: descentCut,
		minCalRows:      maxInt(10, measuredCount/20), // 5% of measured, min 10
		envelopeSecs:    dynamicEnvelopeDurations(measuredCount),
	}
	if ctx.speedMAD <= 0 && ctx.speedMedian > 0 {
		ctx.speedMAD = ctx.speedMedian * 0.15
	}
	if ctx.gradeMAD <= 0 {
		ctx.gradeMAD = 0.02 // only if grade is flat constant; small protocol floor for denom
	}
	if descentTotal > 0 {
		ctx.descentCoastRate = float64(descentCoast) / float64(descentTotal)
	}
	// If measured grade p25 is negative, use that as a data-driven descent cut
	// so mild negatives still count as descents when the ride has real downs.
	if p25 := percentileSorted(append([]float64(nil), grades...), 0.25); p25 < 0 {
		ctx.descentGradeCut = p25
		// recompute coast rate with tighter cut
		descentTotal, descentCoast = 0, 0
		for i := range samples {
			if samples[i].grade < ctx.descentGradeCut {
				descentTotal++
				if samples[i].zero {
					descentCoast++
				}
			}
		}
		if descentTotal > 0 {
			ctx.descentCoastRate = float64(descentCoast) / float64(descentTotal)
		}
	}
	if len(positiveWatts) > 0 {
		// High measured percentile as instant ceiling (data-driven, not FTP table).
		ctx.instantCap = percentileSorted(append([]float64(nil), positiveWatts...), 0.99)
	}

	return ctx
}

func dynamicEnvelopeDurations(measuredSeconds int) []int {
	if measuredSeconds <= 0 {
		return nil
	}
	// Fractions of measured coverage — scales with ride length.
	fracs := []float64{0.01, 0.02, 0.05, 0.10}
	out := make([]int, 0, len(fracs))
	seen := map[int]bool{}
	for _, frac := range fracs {
		sec := int(float64(measuredSeconds) * frac)
		if sec < 15 {
			continue
		}
		if sec > measuredSeconds {
			sec = measuredSeconds
		}
		if seen[sec] {
			continue
		}
		seen[sec] = true
		out = append(out, sec)
	}
	sort.Ints(out)

	return out
}

func maybeCalibrateParams(
	params *PowerModelParams,
	calibrate bool,
	labels []string,
	watts, cadence NullableSeries,
	speed, grade, accel []float64,
	ctx *measuredContext,
) (PowerModelParams, PowerFillCalibration, []string) {
	if params == nil {
		return PowerModelParams{}, PowerFillCalibration{}, nil
	}
	if !calibrate {
		return *params, PowerFillCalibration{}, nil
	}
	out := *params
	cal := PowerFillCalibration{}

	// Fit Crr only when not explicitly provided by the user. Low-speed climbs only
	// so aero is not mis-attributed into rolling resistance.
	if out.Crr.Source != powerParamSourceUser || out.Crr.Value <= 0 {
		if crr, used, ok := calibrateCrrFromMeasured(&out, labels, watts, cadence, speed, grade, accel, ctx); ok {
			out.Crr = LabeledParam{Value: round4(crr), Source: powerParamSourceCalibrated}
			cal.UsedMeasuredSeconds = used
		}
	}

	cda, calCdA, err := calibrateCdA(&out, labels, watts, cadence, speed, grade, accel, ctx)
	if err != nil {
		return out, cal, []string{err.Error()}
	}
	out.CdA = cda
	cal.UsedMeasuredSeconds = calCdA.UsedMeasuredSeconds
	cal.FitBiasWatts = calCdA.FitBiasWatts
	cal.FitErrorWattsRMSE = calCdA.FitErrorWattsRMSE

	return out, cal, nil
}

func calibrateCrrFromMeasured(
	params *PowerModelParams,
	labels []string,
	watts, cadence NullableSeries,
	speed, grade, accel []float64,
	ctx *measuredContext,
) (float64, int, bool) {
	// Low-speed band: below measured speed median (aero smaller → rolling residual).
	if ctx.speedMedian <= 0 {
		return 0, 0, false
	}
	var crrSamples []float64
	mass := totalMassKg(params)
	eta := params.DrivetrainEff.Value
	for index, label := range labels {
		if label != PowerSampleMeasured {
			continue
		}
		w, ok := watts.At(index)
		if !ok || w <= 0 {
			continue
		}
		vel := speed[index]
		// Dynamic low-speed: at/below median speed.
		if vel <= 0 || vel > ctx.speedMedian {
			continue
		}
		// Require meaningful climb relative to this activity's grade MAD so aero
		// is not absorbed into Crr on flats.
		if grade[index] < ctx.gradeMAD {
			continue
		}
		if c, cOK := cadence.At(index); !cOK || c <= 0 {
			continue
		}
		denom := math.Sqrt(1 + grade[index]*grade[index])
		// Ignore aero for low-speed Crr snapshot: residual after gravity.
		pKnownGrav := mass * powerEstimateGravity * vel * (grade[index] / denom)
		residual := w*eta - pKnownGrav
		xRoll := mass * powerEstimateGravity * vel / denom
		if residual <= 0 || xRoll < 1 {
			continue
		}
		crrSamples = append(crrSamples, residual/xRoll)
	}
	if len(crrSamples) < ctx.minCalRows {
		return 0, 0, false
	}
	// Robust median; reject non-physical negatives only (protocol).
	filtered := robustPositive(crrSamples)
	if len(filtered) < ctx.minCalRows {
		return 0, 0, false
	}

	return medianFloat64(filtered), len(filtered), true
}

func calibrateCdA(
	params *PowerModelParams,
	labels []string,
	watts, cadence NullableSeries,
	speed, grade, accel []float64,
	ctx *measuredContext,
) (LabeledParam, PowerFillCalibration, error) {
	var cdaSamples []float64
	mass := totalMassKg(params)
	eta := params.DrivetrainEff.Value
	crr := params.Crr.Value
	rho := params.AirDensity.Value

	// Speed band and grade band from measured MAD (dynamic).
	speedLo := ctx.speedMedian - 2*ctx.speedMAD
	speedHi := ctx.speedMedian + 2*ctx.speedMAD
	if speedLo < 0 {
		speedLo = 0
	}
	gradeBand := 2 * ctx.gradeMAD
	cadFloor := 0.0
	if ctx.cadenceMedian > 0 {
		cadFloor = ctx.cadenceMedian * 0.5 // half of athlete's median cadence
	}

	for index, label := range labels {
		if label != PowerSampleMeasured {
			continue
		}
		w, ok := watts.At(index)
		if !ok || w <= 0 {
			continue
		}
		vel := speed[index]
		if vel < speedLo || vel > speedHi || vel <= 0 {
			continue
		}
		// Prefer near-flat for aero (small |grade|); still dynamic band.
		if math.Abs(grade[index]) > gradeBand {
			continue
		}
		if c, cOK := cadence.At(index); !cOK || c < cadFloor {
			continue
		}
		// Skip high-accel noise relative to speed MAD scale.
		if ctx.speedMAD > 0 && math.Abs(accel[index]) > 2*ctx.speedMAD {
			continue
		}
		denom := math.Sqrt(1 + grade[index]*grade[index])
		pKnown := mass*powerEstimateGravity*vel*(grade[index]/denom) +
			crr*mass*powerEstimateGravity*vel*(1/denom)
		residual := w*eta - pKnown
		// CdA from P_aero = ½ ρ CdA v_air|v_air| v_ground  ⇒ CdA = P_aero / (½ ρ v_air|v_air| v_ground)
		airSpeed := vel + params.HeadwindMS.Value
		xAero := 0.5 * rho * airSpeed * math.Abs(airSpeed) * vel
		if residual <= 0 || xAero < 1 {
			continue
		}
		cdaSamples = append(cdaSamples, residual/xAero)
	}

	cal := PowerFillCalibration{UsedMeasuredSeconds: len(cdaSamples)}
	if len(cdaSamples) < ctx.minCalRows {
		return LabeledParam{}, cal, fmt.Errorf(
			"calibration needs >= %d steady measured samples (dynamic from this activity), got %d",
			ctx.minCalRows, len(cdaSamples),
		)
	}
	filtered := robustPositive(cdaSamples)
	if len(filtered) < ctx.minCalRows {
		return LabeledParam{}, cal, fmt.Errorf("calibration left too few samples after robust filter (%d)", len(filtered))
	}
	cda := medianFloat64(filtered)
	fitted := LabeledParam{Value: round4(cda), Source: powerParamSourceCalibrated}
	paramsFitted := *params
	paramsFitted.CdA = fitted
	cal.FitBiasWatts, cal.FitErrorWattsRMSE = calibrationErrors(&paramsFitted, labels, watts, cadence, speed, grade, accel, ctx)

	return fitted, cal, nil
}

// robustPositive keeps values within median ± 3*MAD (dynamic outlier fence).
func robustPositive(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}
	med := medianFloat64(values)
	mad := madFloat64(values, med)
	if mad <= 0 {
		out := make([]float64, 0, len(values))
		for _, value := range values {
			if value > 0 {
				out = append(out, value)
			}
		}

		return out
	}
	lo := med - 3*mad
	hi := med + 3*mad
	if lo < 0 {
		lo = 0
	}
	out := make([]float64, 0, len(values))
	for _, value := range values {
		if value > 0 && value >= lo && value <= hi {
			out = append(out, value)
		}
	}

	return out
}

func attachPhysicsResiduals(
	ctx *measuredContext,
	_ []string,
	watts NullableSeries,
	speed, grade, rho, headwind []float64,
	params *PowerModelParams,
) {
	if ctx == nil || params == nil {
		return
	}
	for i := range ctx.samples {
		sample := &ctx.samples[i]
		idx := sample.index
		if idx < 0 || idx >= len(speed) || idx >= len(grade) {
			continue
		}
		r := params.AirDensity.Value
		if idx < len(rho) && rho[idx] > 0 {
			r = rho[idx]
		}
		hw := sampleHeadwind(params, headwind, idx)
		phys := virtualPowerWattsRhoHW(params, speed[idx], grade[idx], 0, r, hw)
		sample.phys = phys
		w, ok := watts.At(idx)
		if !ok {
			w = 0
		}
		sample.residual = w - phys
	}
}

func sampleHeadwind(params *PowerModelParams, headwind []float64, index int) float64 {
	if index >= 0 && index < len(headwind) {
		return headwind[index]
	}
	if params != nil {
		return params.HeadwindMS.Value
	}

	return 0
}

func attachRollingFeatures(
	ctx *measuredContext,
	labels []string,
	speedRoll, gradeRoll, hrRoll []float64,
	sampleCount int,
) {
	if ctx == nil {
		return
	}
	for i := range ctx.samples {
		sample := &ctx.samples[i]
		idx := sample.index
		if idx < 0 || idx >= sampleCount {
			continue
		}
		if idx < len(speedRoll) {
			sample.speedRoll = speedRoll[idx]
		}
		if idx < len(gradeRoll) {
			sample.gradeRoll = gradeRoll[idx]
		}
		if idx < len(hrRoll) {
			sample.hrRoll = hrRoll[idx]
		}
	}
}

func fitMultiLinearPower(
	labels []string,
	watts NullableSeries,
	speed, grade, hr []float64,
	params *PowerModelParams,
	ctx *measuredContext,
) multiLinearPower {
	// Design: y = b0 + b1*phys + b2*hr + b3*speed + b4*grade
	// Solve normal equations for 5 unknowns with measured non-descent samples.
	var out multiLinearPower
	if ctx == nil || params == nil {
		return out
	}
	const p = 5
	var xtx [p][p]float64
	var xty [p]float64
	var ySum, ySum2 float64
	n := 0
	for i := range ctx.samples {
		sample := &ctx.samples[i]
		if sample.zero || sample.watts <= 0 {
			continue
		}
		if sample.grade < ctx.descentGradeCut {
			continue
		}
		phys := sample.phys
		if phys <= 0 {
			phys = virtualPowerWatts(params, sample.speed, sample.grade, 0)
		}
		x := [p]float64{1, phys, sample.hr, sample.speed, sample.grade}
		y := sample.watts
		for a := 0; a < p; a++ {
			xty[a] += x[a] * y
			for b := 0; b < p; b++ {
				xtx[a][b] += x[a] * x[b]
			}
		}
		ySum += y
		ySum2 += y * y
		n++
	}
	if n < ctx.minCalRows {
		return out
	}
	beta, ok := solveSymmetric(xtx, xty)
	if !ok {
		return out
	}
	// R^2 on training rows.
	var ssRes float64
	meanY := ySum / float64(n)
	ssTot := ySum2 - float64(n)*meanY*meanY
	for i := range ctx.samples {
		sample := &ctx.samples[i]
		if sample.zero || sample.watts <= 0 || sample.grade < ctx.descentGradeCut {
			continue
		}
		phys := sample.phys
		if phys <= 0 {
			phys = virtualPowerWatts(params, sample.speed, sample.grade, 0)
		}
		pred := beta[0] + beta[1]*phys + beta[2]*sample.hr + beta[3]*sample.speed + beta[4]*sample.grade
		d := sample.watts - pred
		ssRes += d * d
	}
	r2 := 0.0
	if ssTot > 0 {
		r2 = 1 - ssRes/ssTot
	}
	out = multiLinearPower{
		ok:        true,
		offset:    beta[0],
		physGain:  beta[1],
		hrGain:    beta[2],
		speedGain: beta[3],
		gradeGain: beta[4],
		r2:        r2,
	}

	return out
}

// solveSymmetric solves A x = b for small dense SPD-ish systems via Gaussian elimination.
func solveSymmetric(a [5][5]float64, b [5]float64) ([5]float64, bool) {
	const n = 5
	// Augment.
	m := make([][]float64, n)
	for i := 0; i < n; i++ {
		m[i] = make([]float64, n+1)
		for j := 0; j < n; j++ {
			m[i][j] = a[i][j]
		}
		m[i][n] = b[i]
	}
	for col := 0; col < n; col++ {
		// Pivot.
		pivot := col
		for row := col + 1; row < n; row++ {
			if math.Abs(m[row][col]) > math.Abs(m[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(m[pivot][col]) < 1e-12 {
			return [5]float64{}, false
		}
		m[col], m[pivot] = m[pivot], m[col]
		div := m[col][col]
		for j := col; j <= n; j++ {
			m[col][j] /= div
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := m[row][col]
			for j := col; j <= n; j++ {
				m[row][j] -= factor * m[col][j]
			}
		}
	}
	var x [5]float64
	for i := 0; i < n; i++ {
		x[i] = m[i][n]
	}

	return x, true
}

//nolint:gocritic // multi-value return is clearer than a struct for fill merge helpers
func fillPowerSeriesOutdoor(
	labels []string,
	watts NullableSeries,
	speed, grade, hr, speedRoll, gradeRoll, hrRoll, rho, headwind []float64,
	params *PowerModelParams,
	ctx *measuredContext,
	lin *multiLinearPower,
	blend *outdoorBlendWeights,
	hasMotion bool,
	timeSeries NullableSeries,
	fill *PowerFillSummary,
) ([]float64, []string) {
	sampleCount := len(labels)
	filled := make([]float64, sampleCount)
	sources := make([]string, sampleCount)
	for index := range sampleCount {
		if value, ok := watts.At(index); ok {
			filled[index] = value
		}
		switch labels[index] {
		case PowerSampleMeasured:
			sources[index] = PowerSampleMeasured
		case PowerSampleTrueZero:
			filled[index] = 0
			sources[index] = PowerSampleTrueZero
		case PowerSampleMissing:
			if speed[index] <= 0 && !hasMotion {
				sources[index] = "unfilled"
				fill.UnfilledSeconds++
				continue
			}
			hrVal, sRoll, gRoll, hRoll := 0.0, speed[index], grade[index], 0.0
			if index < len(hr) {
				hrVal = hr[index]
			}
			if index < len(speedRoll) {
				sRoll = speedRoll[index]
			}
			if index < len(gradeRoll) {
				gRoll = gradeRoll[index]
			}
			if index < len(hrRoll) {
				hRoll = hrRoll[index]
			}
			r := params.AirDensity.Value
			if index < len(rho) && rho[index] > 0 {
				r = rho[index]
			}
			hw := sampleHeadwind(params, headwind, index)
			phys := virtualPowerWattsRhoHW(params, speed[index], grade[index], 0, r, hw)
			if grade[index] < ctx.descentGradeCut {
				phys = descentAwareEstimateRhoHW(phys, speed[index], grade[index], r, hw, params, ctx)
			}
			est, src := outdoorEstimateSample(speed[index], grade[index], hrVal, sRoll, gRoll, hRoll, phys, params, ctx, lin, blend)
			filled[index] = est
			sources[index] = src
			fill.EstimatedSeconds++
			fill.MeanEstimatedWatts += est
			fill.EstimatedKj += est * sampleDeltaSeconds(timeSeries, index) / powerEstimateJoulesPerKj
		default:
			sources[index] = "unfilled"
			fill.UnfilledSeconds++
		}
	}
	if fill.EstimatedSeconds > 0 {
		fill.MeanEstimatedWatts = round2(fill.MeanEstimatedWatts / float64(fill.EstimatedSeconds))
		fill.EstimatedKj = round2(fill.EstimatedKj)
	}

	return filled, sources
}

// outdoorEstimateSample blends residual-kNN, direct-kNN, and multi-linear using
// LOO-tuned weights from this outdoor activity's measured segment.
// Descents: freewheel / measured coast dominate (negative slope → lower power).
func outdoorEstimateSample(
	speed, grade, hr, speedRoll, gradeRoll, hrRoll, phys float64,
	params *PowerModelParams,
	ctx *measuredContext,
	lin *multiLinearPower,
	blend *outdoorBlendWeights,
) (float64, string) {
	// Descent freewheel first when gravity + aero net is non-positive.
	if grade < ctx.descentGradeCut {
		if aeroWheelPower(params, speed, grade, params.AirDensity.Value) <= 0 {
			return 0, "estimated"
		}
	}

	var direct, resid, linear, hrBin float64
	var hasDirect, hasResid, hasLin, hasHR bool
	if est, ok := estimateFromMeasuredNeighborhood(speed, grade, hr, params, ctx); ok {
		direct = est
		hasDirect = true
	}
	if res, ok := residualKNN(speedRoll, gradeRoll, hrRoll, ctx); ok {
		resid = phys + res
		if resid < 0 {
			resid = 0
		}
		hasResid = true
	}
	if lin != nil && lin.ok && lin.r2 >= 0.15 {
		linear = lin.offset + lin.physGain*phys + lin.hrGain*hr + lin.speedGain*speed + lin.gradeGain*grade
		if linear < 0 {
			linear = 0
		}
		hasLin = true
	}
	if est, ok := estimateFromHRPhysBin(hr, phys, ctx); ok {
		hrBin = est
		hasHR = true
	}

	// Default equal weights among available models; override with tuned blend.
	wd, wr, wl, wh := 0.0, 0.0, 0.0, 0.0
	if blend != nil && blend.ok {
		wd, wr, wl, wh = blend.wDirect, blend.wResid, blend.wLin, blend.wHR
	} else {
		if hasDirect {
			wd = 1
		}
		if hasResid {
			wr = 1
		}
		if hasLin {
			wl = 1
		}
		if hasHR {
			wh = 1
		}
	}
	if !hasDirect {
		wd = 0
	}
	if !hasResid {
		wr = 0
	}
	if !hasLin {
		wl = 0
	}
	if !hasHR {
		wh = 0
	}
	sumW := wd + wr + wl + wh
	if sumW <= 0 {
		if grade < ctx.descentGradeCut {
			phys = descentAwareEstimateRhoHW(phys, speed, grade, params.AirDensity.Value, params.HeadwindMS.Value, params, ctx)
		}
		return phys, "estimated_physics"
	}
	est := (wd*direct + wr*resid + wl*linear + wh*hrBin) / sumW
	if grade < ctx.descentGradeCut && ctx.descentCoastRate > 0.4 {
		est *= (1 - 0.5*ctx.descentCoastRate)
	}
	if est < 0 {
		est = 0
	}

	return est, "estimated"
}

// buildHRPhysBins partitions measured samples into dynamic HR quantiles and
// stores median watts and median physics per bin (outdoor effort map).
func buildHRPhysBins(ctx *measuredContext) []hrPhysBin {
	if ctx == nil || len(ctx.samples) < 40 {
		return nil
	}
	var hrs []float64
	for i := range ctx.samples {
		if ctx.samples[i].hr > 0 && !ctx.samples[i].zero {
			hrs = append(hrs, ctx.samples[i].hr)
		}
	}
	if len(hrs) < 40 {
		return nil
	}
	// 8 quantile bins from measured HR distribution.
	nBins := 8
	edges := make([]float64, nBins+1)
	for b := 0; b <= nBins; b++ {
		p := float64(b) / float64(nBins)
		edges[b] = percentileSorted(append([]float64(nil), hrs...), p)
	}
	// Ensure strictly increasing edges.
	for b := 1; b <= nBins; b++ {
		if edges[b] <= edges[b-1] {
			edges[b] = edges[b-1] + 0.5
		}
	}
	bins := make([]hrPhysBin, nBins)
	listsW := make([][]float64, nBins)
	listsP := make([][]float64, nBins)
	for i := range ctx.samples {
		sample := &ctx.samples[i]
		if sample.hr <= 0 {
			continue
		}
		b := nBins - 1
		for j := 0; j < nBins; j++ {
			if sample.hr >= edges[j] && sample.hr < edges[j+1] {
				b = j
				break
			}
		}
		if sample.hr >= edges[nBins] {
			b = nBins - 1
		}
		listsW[b] = append(listsW[b], sample.watts)
		listsP[b] = append(listsP[b], sample.phys)
	}
	for b := 0; b < nBins; b++ {
		bins[b] = hrPhysBin{
			hrLo:        edges[b],
			hrHi:        edges[b+1],
			medianWatts: medianFloat64(listsW[b]),
			medianPhys:  medianFloat64(listsP[b]),
			count:       len(listsW[b]),
		}
	}

	return bins
}

// estimateFromHRPhysBin: watts ≈ medianWatts(HR) + (phys - medianPhys(HR)).
// Captures outdoor effort (HR) with grade/aero correction from physics residual.
func estimateFromHRPhysBin(hr, phys float64, ctx *measuredContext) (float64, bool) {
	if ctx == nil || len(ctx.hrBins) == 0 || hr <= 0 {
		return 0, false
	}
	for i := range ctx.hrBins {
		bin := &ctx.hrBins[i]
		if bin.count < 8 {
			continue
		}
		if hr >= bin.hrLo && (hr < bin.hrHi || i == len(ctx.hrBins)-1) {
			est := bin.medianWatts + (phys - bin.medianPhys)
			if est < 0 {
				est = 0
			}
			return est, true
		}
	}

	return 0, false
}

// fitOutdoorBlendWeights learns non-negative weights on a measured subsample by
// minimizing absolute error of outdoor candidates (dynamic ensemble).
func fitOutdoorBlendWeights(ctx *measuredContext, params *PowerModelParams, lin *multiLinearPower) outdoorBlendWeights {
	out := outdoorBlendWeights{wDirect: 0.25, wResid: 0.25, wLin: 0.25, wHR: 0.25}
	if ctx == nil || len(ctx.samples) < ctx.minCalRows*2 {
		return out
	}
	step := len(ctx.samples) / 200
	if step < 1 {
		step = 1
	}
	type row struct {
		y, d, r, l, h  float64
		hd, hr, hl, hh bool
	}
	rows := make([]row, 0, 200)
	for i := 0; i < len(ctx.samples); i += step {
		sample := &ctx.samples[i]
		if sample.zero {
			continue
		}
		phys := sample.phys
		if phys <= 0 {
			phys = virtualPowerWatts(params, sample.speed, sample.grade, 0)
		}
		var rr row
		rr.y = sample.watts
		if est, ok := estimateFromMeasuredNeighborhood(sample.speed, sample.grade, sample.hr, params, ctx); ok {
			rr.d = est
			rr.hd = true
		}
		if res, ok := residualKNN(sample.speedRoll, sample.gradeRoll, sample.hrRoll, ctx); ok {
			rr.r = phys + res
			if rr.r < 0 {
				rr.r = 0
			}
			rr.hr = true
		}
		if lin != nil && lin.ok {
			rr.l = lin.offset + lin.physGain*phys + lin.hrGain*sample.hr + lin.speedGain*sample.speed + lin.gradeGain*sample.grade
			if rr.l < 0 {
				rr.l = 0
			}
			rr.hl = true
		}
		if est, ok := estimateFromHRPhysBin(sample.hr, phys, ctx); ok {
			rr.h = est
			rr.hh = true
		}
		if rr.hd || rr.hr || rr.hl || rr.hh {
			rows = append(rows, rr)
		}
	}
	if len(rows) < 30 {
		return out
	}
	// Coarse simplex grid over 4 weights (steps of 1/3 for speed).
	bestErr := math.MaxFloat64
	best := out
	for di := 0; di <= 3; di++ {
		for ri := 0; ri <= 3-di; ri++ {
			for li := 0; li <= 3-di-ri; li++ {
				hi := 3 - di - ri - li
				wd := float64(di) / 3
				wr := float64(ri) / 3
				wl := float64(li) / 3
				wh := float64(hi) / 3
				var sumAbs float64
				var count int
				for _, row := range rows {
					a, b, c, d := wd, wr, wl, wh
					if !row.hd {
						a = 0
					}
					if !row.hr {
						b = 0
					}
					if !row.hl {
						c = 0
					}
					if !row.hh {
						d = 0
					}
					s := a + b + c + d
					if s <= 0 {
						continue
					}
					pred := (a*row.d + b*row.r + c*row.l + d*row.h) / s
					sumAbs += math.Abs(pred - row.y)
					count++
				}
				if count < 20 {
					continue
				}
				err := sumAbs / float64(count)
				if err < bestErr {
					bestErr = err
					best = outdoorBlendWeights{wDirect: wd, wResid: wr, wLin: wl, wHR: wh, ok: true}
				}
			}
		}
	}

	return best
}

func residualKNN(speedRoll, gradeRoll, hrRoll float64, ctx *measuredContext) (float64, bool) {
	if ctx == nil || len(ctx.samples) < ctx.minCalRows {
		return 0, false
	}
	// Sharper k for outdoor tracking (less over-smoothing). Dynamic with n.
	k := int(math.Sqrt(float64(len(ctx.samples))) * 0.75)
	if k < 15 {
		k = 15
	}
	if k > 60 {
		k = 60
	}
	if k > len(ctx.samples) {
		k = len(ctx.samples)
	}
	speedR := 2 * ctx.speedMAD
	if speedR < 0.5 {
		speedR = 0.5
	}
	gradeR := 2 * ctx.gradeMAD
	if gradeR <= 0 {
		gradeR = 0.02
	}
	hrR := 2 * ctx.hrMAD
	if hrR <= 0 && ctx.hrMedian > 0 {
		hrR = ctx.hrMedian * 0.1
	}
	useHR := hrRoll > 0 && ctx.hrMedian > 0 && hrR > 0

	type cand struct {
		dist float64
		res  float64
	}
	cands := make([]cand, 0, len(ctx.samples))
	for i := range ctx.samples {
		sample := &ctx.samples[i]
		// Prefer rolling features when present.
		ss, gg, hh := sample.speedRoll, sample.gradeRoll, sample.hrRoll
		if ss == 0 && sample.speed != 0 {
			ss = sample.speed
		}
		if gg == 0 && sample.grade != 0 {
			gg = sample.grade
		}
		if hh == 0 {
			hh = sample.hr
		}
		ds := math.Abs(ss-speedRoll) / speedR
		dg := math.Abs(gg-gradeRoll) / gradeR
		dh := 0.0
		if useHR && hh > 0 {
			// Weight HR more: outdoor effort often tracks HR when draft/aero varies.
			dh = 1.75 * math.Abs(hh-hrRoll) / hrR
		}
		dist := math.Sqrt(ds*ds + dg*dg + dh*dh)
		cands = append(cands, cand{dist: dist, res: sample.residual})
	}
	// Partial selection of k nearest (insertion for small k).
	if len(cands) > k {
		// Simple nth via sort of distances - n can be thousands; use sort.Slice.
		sort.Slice(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })
		cands = cands[:k]
	}
	var wSum, rSum float64
	for _, c := range cands {
		w := 1.0 / (0.05 + c.dist)
		wSum += w
		rSum += w * c.res
	}
	if wSum <= 0 {
		return 0, false
	}

	return rSum / wSum, true
}

func applyInstantCap(filled []float64, sources []string, cap float64) {
	if cap <= 0 {
		return
	}
	for i := range filled {
		if isEstimatedSource(sources[i]) && filled[i] > cap {
			filled[i] = cap
		}
	}
}

// softEnforceMeasuredMMPEnvelope only scales windows that exceed measured MMP
// by more than ~1 MAD of measured multi-minute means (data-driven slack).
func softEnforceMeasuredMMPEnvelope(
	filled []float64,
	sources []string,
	timeSeries NullableSeries,
	labels []string,
	watts NullableSeries,
	envelopeSecs []int,
	ctx *measuredContext,
) bool {
	measured := measuredOnlySeries(labels, watts, len(filled))
	if len(measured) == 0 || len(envelopeSecs) == 0 {
		return false
	}
	applied := false
	for _, duration := range envelopeSecs {
		mmp := maxRollingMean(measured, timeSeries, duration)
		if mmp <= 0 {
			continue
		}
		// Allow up to +15% above measured MMP (relative slack from residual MAD if available).
		slack := 1.15
		if ctx != nil && ctx.instantCap > 0 && mmp > 0 {
			// Slightly more slack when measured power is highly variable.
			ratio := ctx.instantCap / mmp
			if ratio > 1.5 {
				slack = 1.25
			}
		}
		ceiling := mmp * slack
		if scaleRollingWindows(filled, sources, timeSeries, duration, ceiling) {
			applied = true
		}
	}

	return applied
}

// descentAwareEstimateRhoHW keeps downhill power low. Physics already subtracts
// gravity (negative slope → negative P_grav). Freewheel when net force ≤ 0.
func descentAwareEstimateRhoHW(
	physicsEst, speed, grade, rho, headwindMS float64,
	params *PowerModelParams,
	ctx *measuredContext,
) float64 {
	if physicsEst <= 0 {
		return 0
	}
	if aeroWheelPowerHW(params, speed, grade, rho, headwindMS) <= 0 {
		return 0
	}
	// Soft blend using measured coast frequency on descents (dynamic).
	if ctx != nil && ctx.descentCoastRate > 0 {
		physicsEst *= (1 - 0.5*ctx.descentCoastRate)
	}

	return physicsEst
}

// aeroWheelPower is P_grav + P_roll + P_aero at constant speed (no accel),
// using relative air speed for drag.
func aeroWheelPower(params *PowerModelParams, groundSpeed, grade, rho float64) float64 {
	return aeroWheelPowerHW(params, groundSpeed, grade, rho, params.HeadwindMS.Value)
}

func aeroWheelPowerHW(params *PowerModelParams, groundSpeed, grade, rho, headwindMS float64) float64 {
	if groundSpeed < 0 {
		groundSpeed = 0
	}
	if rho <= 0 {
		rho = params.AirDensity.Value
	}
	mass := totalMassKg(params)
	denom := math.Sqrt(1 + grade*grade)
	pGrav := mass * powerEstimateGravity * groundSpeed * (grade / denom)
	pRoll := params.Crr.Value * mass * powerEstimateGravity * groundSpeed / denom
	airSpeed := groundSpeed + headwindMS
	pAero := 0.5 * rho * params.CdA.Value * airSpeed * math.Abs(airSpeed) * groundSpeed

	return pGrav + pRoll + pAero
}

type neighborhoodStats struct {
	median       float64
	weightedMean float64
	coastRate    float64
	count        int
}

func neighborhoodPower(speed, grade, hr float64, ctx *measuredContext) (neighborhoodStats, bool) {
	if ctx == nil || len(ctx.samples) == 0 {
		return neighborhoodStats{}, false
	}
	// Dynamic radii from MAD of measured distributions.
	speedR := 2 * ctx.speedMAD
	if speedR <= 0 {
		speedR = ctx.speedMedian * 0.2
	}
	if speedR < 0.5 {
		speedR = 0.5 // m/s protocol floor so empty MAD still matches neighbors
	}
	gradeR := 2 * ctx.gradeMAD
	if gradeR <= 0 {
		gradeR = 0.02
	}
	hrR := 2 * ctx.hrMAD
	if hrR <= 0 && ctx.hrMedian > 0 {
		hrR = ctx.hrMedian * 0.1
	}
	useHR := hr > 0 && ctx.hrMedian > 0 && hrR > 0

	// k-nearest among measured samples (avoids over-expanding radius into distant regimes).
	k := int(math.Sqrt(float64(len(ctx.samples))) * 0.75)
	if k < 15 {
		k = 15
	}
	if k > 60 {
		k = 60
	}
	type cand struct {
		dist  float64
		watts float64
		zero  bool
	}
	cands := make([]cand, 0, len(ctx.samples))
	for i := range ctx.samples {
		sample := &ctx.samples[i]
		ds := math.Abs(sample.speed-speed) / speedR
		dg := math.Abs(sample.grade-grade) / gradeR
		dh := 0.0
		if useHR && sample.hr > 0 && hr > 0 {
			dh = 1.75 * math.Abs(sample.hr-hr) / hrR
		}
		dist := math.Sqrt(ds*ds + dg*dg + dh*dh)
		cands = append(cands, cand{dist: dist, watts: sample.watts, zero: sample.zero})
	}
	if len(cands) == 0 {
		return neighborhoodStats{}, false
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })
	if k > len(cands) {
		k = len(cands)
	}
	cands = cands[:k]
	var watts []float64
	var weightSum, wattWeightSum float64
	var coast int
	for _, c := range cands {
		w := 1.0 / (0.05 + c.dist)
		weightSum += w
		if c.zero {
			coast++
			watts = append(watts, 0)
			continue
		}
		watts = append(watts, c.watts)
		wattWeightSum += w * c.watts
	}
	if weightSum <= 0 {
		return neighborhoodStats{}, false
	}

	return neighborhoodStats{
		median:       medianFloat64(watts),
		weightedMean: wattWeightSum / weightSum,
		coastRate:    float64(coast) / float64(len(cands)),
		count:        len(cands),
	}, true
}

// estimateFromMeasuredNeighborhood returns watts from similar measured
// (speed, grade, HR) samples. Descents with high coast rate → 0.
func estimateFromMeasuredNeighborhood(speed, grade, hr float64, params *PowerModelParams, ctx *measuredContext) (float64, bool) {
	neigh, ok := neighborhoodPower(speed, grade, hr, ctx)
	if !ok {
		return 0, false
	}
	if grade < ctx.descentGradeCut && neigh.coastRate >= 0.5 {
		return 0, true
	}
	est := neigh.weightedMean
	if grade < ctx.descentGradeCut {
		net := aeroWheelPower(params, speed, grade, params.AirDensity.Value)
		if net <= 0 && est > 0 && neigh.coastRate > 0.25 {
			est *= (1 - neigh.coastRate)
		}
		if net < 0 && neigh.median <= 0 {
			return 0, true
		}
	}
	if est < 0 {
		est = 0
	}

	return est, true
}

func applyLinearCalibrationAndCap(filled []float64, sources []string, gain, offset, cap float64) {
	if gain <= 0 {
		gain = 1
	}
	for index := range filled {
		// Only physics fallback needs linear PM alignment; neighborhood estimates
		// already sit on the measured scale.
		if sources[index] != "estimated_physics" {
			if sources[index] == "estimated" && cap > 0 && filled[index] > cap {
				filled[index] = cap
			}
			continue
		}
		// Preserve freewheel zeros.
		if filled[index] <= 0 {
			filled[index] = 0
			continue
		}
		v := gain*filled[index] + offset
		if v < 0 {
			v = 0
		}
		if cap > 0 && v > cap {
			v = cap
		}
		filled[index] = v
	}
}

func buildFillMetrics(watts NullableSeries, filled []float64, sampleCount int, timeSeries NullableSeries, ftp int, ftpSource string) PowerFillMetrics {
	beforeDense := watts.DenseOrZero()
	if len(beforeDense) > sampleCount {
		beforeDense = beforeDense[:sampleCount]
	}
	for len(beforeDense) < sampleCount {
		beforeDense = append(beforeDense, 0)
	}
	metrics := PowerFillMetrics{
		Before: PowerFillMetricsPair{
			AvgWatts:        round2(AveragePower(beforeDense, 0, sampleCount)),
			NormalizedPower: round2(NormalizedPower(beforeDense, 0, sampleCount)),
		},
		After: PowerFillMetricsPair{
			AvgWatts:        round2(AveragePower(filled, 0, sampleCount)),
			NormalizedPower: round2(NormalizedPower(filled, 0, sampleCount)),
			FTP:             ftp,
			FTPSource:       ftpSource,
		},
	}
	if ftp > 0 {
		metrics.After.EstimatedTrainingLoad = round2(estimateTSS(filled, timeSeries, ftp))
		metrics.Before.EstimatedTrainingLoad = round2(estimateTSS(beforeDense, timeSeries, ftp))
		metrics.Before.FTP = ftp
		metrics.Before.FTPSource = ftpSource
	}

	return metrics
}

func resolvePowerModelParams(req *PowerFillRequest) (PowerModelParams, []string, error) {
	params := req.Params
	var warnings []string
	if params.RiderMassKg.Value <= 0 {
		return params, warnings, errors.New("rider mass required: pass --rider-mass-kg or provide activity/athlete weight")
	}
	if params.RiderMassKg.Source == "" {
		params.RiderMassKg.Source = powerParamSourceUser
	}
	if params.BikeMassKg.Value <= 0 {
		return params, warnings, errors.New("bike mass required: pass --bike-mass-kg")
	}
	if params.BikeMassKg.Source == "" {
		params.BikeMassKg.Source = powerParamSourceUser
	}
	// Crr may be calibrated later; if still empty after cal, require user value.
	if params.Crr.Value < 0 {
		return params, warnings, errors.New("crr must be non-negative")
	}
	if params.Crr.Value == 0 && params.Crr.Source == "" && !req.CalibrateFromMeasured {
		return params, warnings, errors.New("crr required: pass --crr or --calibrate-from-measured")
	}
	if params.Crr.Source == "" && params.Crr.Value > 0 {
		params.Crr.Source = powerParamSourceUser
	}
	if params.DrivetrainEff.Value <= 0 || params.DrivetrainEff.Value > 1 {
		return params, warnings, errors.New("drivetrain efficiency required in (0,1]: pass --drivetrain-eff")
	}
	if params.DrivetrainEff.Source == "" {
		params.DrivetrainEff.Source = powerParamSourceUser
	}
	// Air density: prefer altitude+temp ISA, then user, then sea-level ISA.
	if params.AirDensity.Value <= 0 {
		if req.Aero.MeanAltitudeM != 0 || req.Aero.MeanTempC != 0 {
			rho := airDensityFromAltitudeTemp(req.Aero.MeanAltitudeM, req.Aero.MeanTempC)
			params.AirDensity = LabeledParam{Value: round4(rho), Source: "isa_altitude_temp"}
			warnings = append(warnings, fmt.Sprintf(
				"air density ρ=%.3f kg/m³ from altitude=%.0fm temp=%.1f°C (ISA/ideal gas)",
				rho, req.Aero.MeanAltitudeM, req.Aero.MeanTempC,
			))
		} else {
			params.AirDensity = LabeledParam{Value: powerEstimateDefaultRho, Source: "isa_sea_level"}
			warnings = append(warnings, "air density using sea-level ISA 1.225 kg/m³ (no altitude/temp on activity)")
		}
	} else if params.AirDensity.Source == "" {
		params.AirDensity.Source = powerParamSourceUser
	}
	// Along-path headwind component for aero (positive = headwind increases airspeed).
	if params.HeadwindMS.Value == 0 && params.HeadwindMS.Source == "" {
		if hw, src, ok := deriveEffectiveHeadwindMS(req.Aero); ok {
			params.HeadwindMS = LabeledParam{Value: round4(hw), Source: src}
			warnings = append(warnings, fmt.Sprintf(
				"aero headwind component %.2f m/s from %s (air speed = ground + headwind; drag ∝ v_air²·v_ground)",
				hw, src,
			))
		}
	} else if params.HeadwindMS.Source == "" && params.HeadwindMS.Value != 0 {
		params.HeadwindMS.Source = powerParamSourceUser
	}
	if params.CdA.Value > 0 && params.CdA.Source == "" {
		params.CdA.Source = powerParamSourceUser
	}

	return params, warnings, nil
}

// deriveEffectiveHeadwindMS builds a ride-mean along-path wind component.
// Positive values are headwind (increase aero power); negative are tailwind assist.
func deriveEffectiveHeadwindMS(aero PowerAeroInputs) (float64, string, bool) {
	if aero.WindSpeed <= 0 {
		return 0, "", false
	}
	windMS := aero.WindSpeed
	src := "activity_wind_ms"
	if aero.WindSpeedIsKmh || aero.WindSpeed > 25 {
		// Intervals outdoor wind is commonly km/h; values >25 m/s are rare as means.
		windMS = aero.WindSpeed * powerEstimateKmhToMS
		src = "activity_wind_kmh"
	}
	// Prefer head/tail time shares when present.
	if aero.HeadwindPercent > 0 || aero.TailwindPercent > 0 {
		// Net headwind fraction in [-1,1].
		net := (aero.HeadwindPercent - aero.TailwindPercent) / 100
		return windMS * net, src + "_head_tail_pct", true
	}

	// Without direction, cannot assign head vs tail; leave still air for mean wind.
	return 0, "", false
}

// airDensityFromAltitudeTemp is ISA pressure with ideal-gas density at temp °C.
func airDensityFromAltitudeTemp(altitudeM, tempC float64) float64 {
	h := altitudeM
	// Formula is defined for troposphere; mild floor for deep valleys.
	if h < -500 {
		h = -500
	}
	if tempC == 0 {
		tempC = powerEstimateISASeaTempC - powerEstimateISALapse*h
	}
	tempK := tempC + 273.15
	if tempK < 200 {
		tempK = 288.15
	}
	pressure := powerEstimateSeaLevelPa * math.Pow(1-powerEstimateISAPressCoef*h, powerEstimateISAExpo)
	if pressure <= 0 {
		return powerEstimateDefaultRho
	}

	return pressure / (powerEstimateDryAirR * tempK)
}

// airDensitySeries builds per-sample ρ from the altitude stream when present.
// When fallbackRho is set (user/weather mean density), altitude scales that
// baseline by the ISA density ratio rather than replacing it with pure ISA.
func airDensitySeries(altitude NullableSeries, sampleCount int, meanTempC, fallbackRho float64) []float64 {
	out := make([]float64, sampleCount)
	base := fallbackRho
	if base <= 0 {
		base = powerEstimateDefaultRho
	}
	isa0 := airDensityFromAltitudeTemp(0, meanTempC)
	for i := 0; i < sampleCount; i++ {
		if alt, ok := altitude.At(i); ok {
			if fallbackRho > 0 && isa0 > 0 {
				isaH := airDensityFromAltitudeTemp(alt, meanTempC)
				out[i] = fallbackRho * (isaH / isa0)
			} else {
				out[i] = airDensityFromAltitudeTemp(alt, meanTempC)
			}
			continue
		}
		out[i] = base
	}

	return out
}

func totalMassKg(params *PowerModelParams) float64 {
	return params.RiderMassKg.Value + params.BikeMassKg.Value
}

// virtualPowerWatts is the Martin balance at one sample using params baseline ρ
// and mean headwind. Prefer virtualPowerWattsRho when altitude-varying density is available.
//
// Aero drag: F_d = ½ ρ CdA v_air |v_air|, P_aero = F_d · v_ground
// with v_air = v_ground + headwind (headwind > 0 into the rider).
// Still air ⇒ v_air = v_ground ⇒ classic ½ ρ CdA v³.
// Negative grade → negative gravity term → lower (often zero) required power.
func virtualPowerWatts(params *PowerModelParams, groundSpeed, grade, accel float64) float64 {
	return virtualPowerWattsRho(params, groundSpeed, grade, accel, params.AirDensity.Value)
}

func virtualPowerWattsRho(params *PowerModelParams, groundSpeed, grade, accel, rho float64) float64 {
	return virtualPowerWattsRhoHW(params, groundSpeed, grade, accel, rho, params.HeadwindMS.Value)
}

// virtualPowerWattsRhoHW is Martin balance with explicit per-sample headwind (m/s).
func virtualPowerWattsRhoHW(params *PowerModelParams, groundSpeed, grade, accel, rho, headwindMS float64) float64 {
	if groundSpeed < 0 {
		groundSpeed = 0
	}
	if rho <= 0 {
		rho = params.AirDensity.Value
	}
	if rho <= 0 {
		rho = powerEstimateDefaultRho
	}
	mass := totalMassKg(params)
	denom := math.Sqrt(1 + grade*grade)
	sinTheta := grade / denom // negative on descent
	cosTheta := 1 / denom
	pGrav := mass * powerEstimateGravity * groundSpeed * sinTheta
	pRoll := params.Crr.Value * mass * powerEstimateGravity * groundSpeed * cosTheta
	// Relative air speed along path (positive headwind increases aero cost).
	airSpeed := groundSpeed + headwindMS
	// Power = drag force × ground speed (collinear wind model).
	pAero := 0.5 * rho * params.CdA.Value * airSpeed * math.Abs(airSpeed) * groundSpeed
	pAcc := mass * accel * groundSpeed
	pWheel := pGrav + pRoll + pAero + pAcc
	// Freewheel: when gravity/tailwind assist exceeds resistive forces, crank power is 0.
	if pWheel < 0 {
		return 0
	}

	return pWheel / params.DrivetrainEff.Value
}

//nolint:gocritic // bias/rmse pair
func calibrationErrors(
	params *PowerModelParams,
	labels []string,
	watts, cadence NullableSeries,
	speed, grade, accel []float64,
	ctx *measuredContext,
) (float64, float64) {
	var sumErr, sumErr2 float64
	var count int
	speedLo := ctx.speedMedian - 2*ctx.speedMAD
	speedHi := ctx.speedMedian + 2*ctx.speedMAD
	cadFloor := ctx.cadenceMedian * 0.5
	for index, label := range labels {
		if label != PowerSampleMeasured {
			continue
		}
		w, ok := watts.At(index)
		if !ok || w <= 0 {
			continue
		}
		vel := speed[index]
		if vel < speedLo || vel > speedHi {
			continue
		}
		if c, cOK := cadence.At(index); !cOK || c < cadFloor {
			continue
		}
		err := virtualPowerWatts(params, vel, grade[index], 0) - w
		sumErr += err
		sumErr2 += err * err
		count++
	}
	if count == 0 {
		return 0, 0
	}

	return round2(sumErr / float64(count)), round2(math.Sqrt(sumErr2 / float64(count)))
}

func deriveGradeSeries(distance, altitude NullableSeries, sampleCount, halfWindow int) []float64 {
	grade := make([]float64, sampleCount)
	if distance.Len() == 0 || altitude.Len() == 0 {
		return grade
	}
	if halfWindow < 1 {
		halfWindow = 1
	}
	// Min distance for grade: dynamic from median step distance when possible.
	minDist := 3.0
	for index := range sampleCount {
		j0 := index - halfWindow
		if j0 < 0 {
			j0 = 0
		}
		j1 := index + halfWindow
		if j1 >= sampleCount {
			j1 = sampleCount - 1
		}
		d0, ok0 := distance.At(j0)
		d1, ok1 := distance.At(j1)
		a0, okA0 := altitude.At(j0)
		a1, okA1 := altitude.At(j1)
		if !ok0 || !ok1 || !okA0 || !okA1 {
			continue
		}
		dd := d1 - d0
		if math.Abs(dd) < minDist {
			continue
		}
		// No hard grade clamp: use raw smoothed rise/run (GPS noise handled by window).
		grade[index] = (a1 - a0) / dd
	}

	return grade
}

func deriveAccelSeries(speed []float64, timeSeries NullableSeries, sampleCount int) []float64 {
	accel := make([]float64, sampleCount)
	for index := 1; index < sampleCount; index++ {
		dt := sampleDeltaSeconds(timeSeries, index)
		if dt <= 0 {
			continue
		}
		accel[index] = (speed[index] - speed[index-1]) / dt
	}

	return accel
}

func smoothSpeedSeries(speed []float64, halfWindow int) []float64 {
	if len(speed) == 0 || halfWindow <= 0 {
		return speed
	}
	out := make([]float64, len(speed))
	for index := range speed {
		lo := index - halfWindow
		if lo < 0 {
			lo = 0
		}
		hi := index + halfWindow + 1
		if hi > len(speed) {
			hi = len(speed)
		}
		var sum float64
		for j := lo; j < hi; j++ {
			sum += speed[j]
		}
		out[index] = sum / float64(hi-lo)
	}

	return out
}

func isEstimatedSource(source string) bool {
	return source == "estimated" || source == "estimated_physics"
}

func smoothEstimatedSamples(filled []float64, sources []string, halfWindow int) {
	if halfWindow <= 0 || len(filled) == 0 {
		return
	}
	original := append([]float64(nil), filled...)
	window := make([]float64, 0, halfWindow*2+1)
	for index := range filled {
		if !isEstimatedSource(sources[index]) {
			continue
		}
		lo := index - halfWindow
		if lo < 0 {
			lo = 0
		}
		hi := index + halfWindow + 1
		if hi > len(original) {
			hi = len(original)
		}
		window = window[:0]
		for j := lo; j < hi; j++ {
			if isEstimatedSource(sources[j]) || sources[j] == PowerSampleMeasured || sources[j] == PowerSampleTrueZero {
				window = append(window, original[j])
			}
		}
		if len(window) == 0 {
			continue
		}
		filled[index] = medianFloat64(window)
	}
}

func measuredOnlySeries(labels []string, watts NullableSeries, sampleCount int) []float64 {
	out := make([]float64, sampleCount)
	for index := 0; index < sampleCount && index < len(labels); index++ {
		if labels[index] != PowerSampleMeasured {
			out[index] = -1
			continue
		}
		w, ok := watts.At(index)
		if !ok {
			out[index] = -1
			continue
		}
		out[index] = w
	}

	return out
}

func maxRollingMean(series []float64, timeSeries NullableSeries, durationSec int) float64 {
	if durationSec <= 0 || len(series) == 0 {
		return 0
	}
	left := 0
	var sum float64
	var count int
	best := 0.0
	for right := range series {
		if series[right] < 0 {
			left = right + 1
			sum = 0
			count = 0
			continue
		}
		sum += series[right]
		count++
		for left <= right {
			span := rollingSpanSeconds(timeSeries, left, right)
			if span <= float64(durationSec) || count <= 1 {
				break
			}
			if series[left] >= 0 {
				sum -= series[left]
				count--
			}
			left++
		}
		span := rollingSpanSeconds(timeSeries, left, right)
		if count > 0 && span >= float64(durationSec)*0.95 {
			avg := sum / float64(count)
			if avg > best {
				best = avg
			}
		}
	}

	return best
}

func scaleRollingWindows(filled []float64, sources []string, timeSeries NullableSeries, durationSec int, maxMean float64) bool {
	if maxMean <= 0 || durationSec <= 0 {
		return false
	}
	applied := false
	left := 0
	var sum float64
	for right := range filled {
		sum += filled[right]
		for left < right && rollingSpanSeconds(timeSeries, left, right) > float64(durationSec) {
			sum -= filled[left]
			left++
		}
		span := rollingSpanSeconds(timeSeries, left, right)
		if span < float64(durationSec)*0.95 {
			continue
		}
		samples := right - left + 1
		if samples <= 0 {
			continue
		}
		avg := sum / float64(samples)
		if avg <= maxMean {
			continue
		}
		estCount := 0
		for i := left; i <= right; i++ {
			if isEstimatedSource(sources[i]) {
				estCount++
			}
		}
		if estCount*2 < samples {
			continue
		}
		factor := maxMean / avg
		for i := left; i <= right; i++ {
			if isEstimatedSource(sources[i]) {
				filled[i] *= factor
			}
		}
		sum = 0
		for i := left; i <= right; i++ {
			sum += filled[i]
		}
		applied = true
	}

	return applied
}

func rollingSpanSeconds(timeSeries NullableSeries, left, right int) float64 {
	if timeSeries.Len() == 0 {
		return float64(right - left)
	}
	t0, ok0 := timeSeries.At(left)
	t1, ok1 := timeSeries.At(right)
	if !ok0 || !ok1 || t1 <= t0 {
		return float64(right - left)
	}

	return t1 - t0
}

func recomputeFillSummary(filled []float64, sources []string, timeSeries NullableSeries, fill *PowerFillSummary) {
	*fill = PowerFillSummary{}
	for index := range filled {
		if !isEstimatedSource(sources[index]) {
			if sources[index] == "unfilled" {
				fill.UnfilledSeconds++
			}
			continue
		}
		fill.EstimatedSeconds++
		fill.MeanEstimatedWatts += filled[index]
		fill.EstimatedKj += filled[index] * sampleDeltaSeconds(timeSeries, index) / powerEstimateJoulesPerKj
	}
	if fill.EstimatedSeconds > 0 {
		fill.MeanEstimatedWatts = round2(fill.MeanEstimatedWatts / float64(fill.EstimatedSeconds))
		fill.EstimatedKj = round2(fill.EstimatedKj)
	}
}

func percentileSorted(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}

	return sorted[idx]
}

func sampleDeltaSeconds(timeSeries NullableSeries, index int) float64 {
	if index <= 0 || timeSeries.Len() == 0 {
		return 1
	}
	t0, ok0 := timeSeries.At(index - 1)
	t1, ok1 := timeSeries.At(index)
	if !ok0 || !ok1 {
		return 1
	}
	dt := t1 - t0
	if dt <= 0 {
		return 1
	}

	return dt
}

func estimateTSS(watts []float64, timeSeries NullableSeries, ftp int) float64 {
	if ftp <= 0 || len(watts) == 0 {
		return 0
	}
	np := NormalizedPower(watts, 0, len(watts))
	if np <= 0 {
		return 0
	}
	duration := float64(len(watts))
	if timeSeries.Len() >= 2 {
		t0, ok0 := timeSeries.At(0)
		t1, ok1 := timeSeries.At(timeSeries.Len() - 1)
		if ok0 && ok1 && t1 > t0 {
			duration = t1 - t0
		}
	}
	ifFactor := np / float64(ftp)

	return (duration / powerEstimateSecsPerHour) * ifFactor * ifFactor * powerEstimateTSSScale
}

func hrCrossCheckWarnings(
	labels []string,
	watts, hr NullableSeries,
	meanEst float64,
	ctx *measuredContext,
) []string {
	if meanEst <= 0 || hr.Len() == 0 {
		return nil
	}
	sumWHR, nWHR, sumGapHR, nGapHR := accumulateHRRatio(labels, watts, hr)
	// Dynamic minimum sample counts from measured coverage.
	minN := ctx.minCalRows
	if minN < 20 {
		minN = 20
	}
	if nWHR < float64(minN) || nGapHR < float64(minN) {
		return nil
	}
	implied := (sumWHR / nWHR) * (sumGapHR / nGapHR)
	if implied <= 0 {
		return nil
	}
	rel := math.Abs(meanEst-implied) / implied
	// Divergence threshold from residual scale variability: use 1/3 as a soft
	// warning only when ratios are available; still relative to data not FTP.
	if rel < 1.0/3.0 {
		return nil
	}

	return []string{fmt.Sprintf(
		"estimated gap mean %.0f W diverges from HR-implied ~%.0f W (rel %.0f%%); treat estimates as lower confidence",
		meanEst, implied, rel*100,
	)}
}

//nolint:gocritic // four accumulator outputs
func accumulateHRRatio(labels []string, watts, hr NullableSeries) (float64, float64, float64, float64) {
	var sumWHR, nWHR, sumGapHR, nGapHR float64
	// Min watts/HR floors derived later if needed; use any positive powered sample.
	for index, label := range labels {
		heartRate, hrOK := hr.At(index)
		if !hrOK || heartRate <= 0 {
			continue
		}
		switch label {
		case PowerSampleMeasured:
			w, wOK := watts.At(index)
			if wOK && w > 0 {
				sumWHR += w / heartRate
				nWHR++
			}
		case PowerSampleMissing:
			sumGapHR += heartRate
			nGapHR++
		}
	}

	return sumWHR, nWHR, sumGapHR, nGapHR
}

func round4(value float64) float64 {
	return math.Round(value*powerEstimateRound4Scale) / powerEstimateRound4Scale
}

// BuildAcceptWattsStream builds the watts series for PUT streams, preserving measured/true zeros.
func BuildAcceptWattsStream(result *PowerFillResult) []float64 {
	if result == nil || len(result.FilledWatts) == 0 {
		return nil
	}
	out := make([]float64, len(result.FilledWatts))
	copy(out, result.FilledWatts)

	return out
}
