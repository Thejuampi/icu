package icu

import (
	"fmt"
	"math"
)

// Power backtest mask modes.
const (
	// PowerBacktestMaskSecondHalf keeps the first half of samples as measured
	// truth and masks the second half like a mid-ride power-meter failure
	// (watts forced to 0, cadence removed) while GPS/HR remain.
	PowerBacktestMaskSecondHalf = "mask_second_half"
	// PowerBacktestMaskAfterFraction masks samples after fraction*n (0..1).
	PowerBacktestMaskAfterFraction = "mask_after_fraction"
	// PowerBacktestMaskScatter masks a fraction of samples pseudo-randomly across
	// the ride (deterministic). Outdoor model fidelity when similar neighbors
	// still exist; not a PM-death simulation.
	PowerBacktestMaskScatter = "mask_scatter"
)

// PowerBacktestRequest is a pure mask/replay evaluation of the estimator.
type PowerBacktestRequest struct {
	ActivityID            string
	Streams               NullableStreamData // original streams with measured power
	Params                PowerModelParams
	Aero                  PowerAeroInputs
	HeadwindMSSeries      []float64 // optional per-sample headwind from real weather
	RhoSeries             []float64 // optional per-sample density from weather
	CalibrateFromMeasured bool
	FTP                   int
	FTPSource             string
	// Mode selects the masking pattern. Empty defaults to mask_second_half.
	Mode string
	// MaskAfterFraction is used with PowerBacktestMaskAfterFraction and MaskScatter (0 exclusive, 1 exclusive).
	MaskAfterFraction float64
	// ActivityType is the Intervals activity type (Ride, VirtualRide, ...).
	// Indoor/virtual types are rejected: free-air physics and GPS grade are not meaningful.
	ActivityType string
	// DistanceMeters outdoor distance; used as a secondary outdoor gate when type is ambiguous.
	DistanceMeters float64
}

// IsOutdoorCyclingActivity reports whether an activity type is outdoor cycling
// suitable for GPS/physics power estimation backtests.
func IsOutdoorCyclingActivity(activityType string) bool {
	switch activityType {
	case "Ride", "GravelRide", "MountainBikeRide", "EBikeRide", "EMountainBikeRide",
		"Cyclocross", "Handcycle", "TrackRide":
		return true
	case "VirtualRide", "IndoorCycling", "Workout":
		return false
	default:
		// Unknown types are not treated as outdoor for backtest purposes.
		return false
	}
}

// PowerBacktestScores holds multi-metric agreement between estimated and measured
// watts. Correlation is necessary but not sufficient; MAD-based residual z-scores
// and outlier-aware robust errors guard against a few spikes dominating fitness.
type PowerBacktestScores struct {
	// Linear / rank correlation
	PearsonR    float64 `json:"pearsonR"`
	SpearmanRho float64 `json:"spearmanRho"`
	R2          float64 `json:"r2"`
	// Error (all compared samples)
	RMSE        float64 `json:"rmse"`
	MAE         float64 `json:"mae"`
	Bias        float64 `json:"bias"` // mean(est - actual)
	P95AbsError float64 `json:"p95AbsError"`
	// Robust error after excluding residual MAD-z outliers (|z| > fence)
	RobustRMSE      float64 `json:"robustRmse"`
	RobustMAE       float64 `json:"robustMae"`
	RobustBias      float64 `json:"robustBias"`
	InlierSeconds   int     `json:"inlierSeconds"`
	OutlierSeconds  int     `json:"outlierSeconds"`
	OutlierFraction float64 `json:"outlierFraction"`
	// Residual z-scores: z_i = (est_i - actual_i - medRes) / (1.4826 * MAD(res))
	// Uses MAD so a few GPS spikes do not inflate the scale.
	ResidualZMeanAbs   float64 `json:"residualZMeanAbs"`
	ResidualZMedianAbs float64 `json:"residualZMedianAbs"`
	ResidualZP95Abs    float64 `json:"residualZP95Abs"`
	// Series z-score concordance: each series MAD-standardized, then Pearson.
	// Captures shape agreement after robust scale/location normalization.
	ZScorePearsonR float64 `json:"zScorePearsonR"`
	// MAD scale of residuals in watts (for interpretation of residual z).
	ResidualMADWatts float64 `json:"residualMadWatts"`
}

// PowerBacktestResult scores estimated vs actual watts on the masked region only.
type PowerBacktestResult struct {
	ActivityID      string `json:"activityId,omitempty"`
	Mode            string `json:"mode"`
	MaskStartIndex  int    `json:"maskStartIndex"`
	MaskedSeconds   int    `json:"maskedSeconds"`
	ComparedSeconds int    `json:"comparedSeconds"`
	// Scores is the multi-metric fitness block (preferred for interpretation).
	Scores PowerBacktestScores `json:"scores"`
	// Top-level correlation/error mirrors for convenience / backward compatibility.
	PearsonR           float64         `json:"pearsonR"`
	SpearmanRho        float64         `json:"spearmanRho"`
	R2                 float64         `json:"r2"`
	RMSE               float64         `json:"rmse"`
	MAE                float64         `json:"mae"`
	Bias               float64         `json:"bias"`
	MeanActual         float64         `json:"meanActual"`
	MeanEstimated      float64         `json:"meanEstimated"`
	MedianActual       float64         `json:"medianActual"`
	MedianEstimated    float64         `json:"medianEstimated"`
	P95AbsError        float64         `json:"p95AbsError"`
	Best12MinActual    float64         `json:"best12MinActual,omitempty"`
	Best12MinEstimated float64         `json:"best12MinEstimated,omitempty"`
	Fill               PowerFillResult `json:"fill,omitempty"`
	Warnings           []string        `json:"warnings,omitempty"`
	BlockingError      string          `json:"blockingError,omitempty"`
}

// BacktestPowerEstimate masks measured power (simulating PM death), re-estimates
// from GPS + mass + calibrated aero/roll, and scores the estimate 1:1 against
// the held-out measured watts on the masked region.
//
// This is the required replay-style validation for the dynamic estimator.
func BacktestPowerEstimate(req PowerBacktestRequest) PowerBacktestResult {
	result := PowerBacktestResult{
		ActivityID: req.ActivityID,
		Mode:       req.Mode,
	}
	if result.Mode == "" {
		result.Mode = PowerBacktestMaskSecondHalf
	}

	// Outdoor-only: indoor/virtual rides have no real slope/aero physics.
	if req.ActivityType != "" && !IsOutdoorCyclingActivity(req.ActivityType) {
		result.BlockingError = fmt.Sprintf(
			"backtest skipped: activity type %q is not outdoor cycling (indoor/virtual power is not free-air physics)",
			req.ActivityType,
		)
		return result
	}
	if req.DistanceMeters > 0 && req.DistanceMeters < 5000 {
		result.BlockingError = "backtest skipped: distance too short for outdoor GPS/physics validation"
		return result
	}

	originalWatts := NullableStream(req.Streams, "watts")
	if originalWatts.Len() == 0 {
		result.BlockingError = "backtest requires a watts stream with measured power"
		return result
	}
	n := originalWatts.Len()
	var maskedStreams NullableStreamData
	var maskStart int
	var maskedSeconds int
	switch result.Mode {
	case PowerBacktestMaskScatter:
		frac := req.MaskAfterFraction
		if frac <= 0 || frac >= 1 {
			frac = 0.35
		}
		var err error
		maskedStreams, maskedSeconds, err = maskStreamsScatter(req.Streams, frac)
		if err != nil {
			result.BlockingError = err.Error()
			return result
		}
		maskStart = -1 // not a single contiguous block
	default:
		var err error
		maskStart, err = backtestMaskStart(result.Mode, req.MaskAfterFraction, n)
		if err != nil {
			result.BlockingError = err.Error()
			return result
		}
		maskedSeconds = n - maskStart
		maskedStreams = maskStreamsForBacktest(req.Streams, maskStart)
	}
	result.MaskStartIndex = maskStart
	result.MaskedSeconds = maskedSeconds
	if result.MaskedSeconds < 30 {
		result.BlockingError = "masked region too short for a meaningful backtest"
		return result
	}
	fill := EstimateAndFillPower(PowerFillRequest{
		ActivityID:            req.ActivityID,
		Streams:               maskedStreams,
		Params:                req.Params,
		Aero:                  req.Aero,
		HeadwindMSSeries:      req.HeadwindMSSeries,
		RhoSeries:             req.RhoSeries,
		CalibrateFromMeasured: req.CalibrateFromMeasured,
		FTP:                   req.FTP,
		FTPSource:             req.FTPSource,
		IncludeStreams:        true,
	})
	result.Fill = fill
	result.Warnings = append(result.Warnings, fill.Warnings...)
	if fill.BlockingError != "" {
		result.BlockingError = fill.BlockingError
		return result
	}
	if len(fill.FilledWatts) < n || len(fill.SampleSource) < n {
		result.BlockingError = "estimator returned incomplete filled series"
		return result
	}

	actual := make([]float64, 0, result.MaskedSeconds)
	estimated := make([]float64, 0, result.MaskedSeconds)
	absErrs := make([]float64, 0, result.MaskedSeconds)
	scoreStart := maskStart
	if scoreStart < 0 {
		scoreStart = 0 // scatter: scan full series
	}
	for i := scoreStart; i < n; i++ {
		// Score only samples the estimator treated as estimated (missing gaps),
		// and where original power was present (ground truth).
		if !isEstimatedSource(fill.SampleSource[i]) {
			continue
		}
		truth, ok := originalWatts.At(i)
		if !ok {
			continue
		}
		// Include true zeros in the score (coasting) so freewheel behavior is tested.
		est := fill.FilledWatts[i]
		actual = append(actual, truth)
		estimated = append(estimated, est)
		absErrs = append(absErrs, math.Abs(est-truth))
	}
	result.ComparedSeconds = len(actual)
	if result.ComparedSeconds < 30 {
		result.BlockingError = fmt.Sprintf(
			"too few comparable estimated samples (%d); mask may not create missing power gaps",
			result.ComparedSeconds,
		)
		return result
	}

	result.MeanActual = round2(meanFloat64(actual))
	result.MeanEstimated = round2(meanFloat64(estimated))
	result.MedianActual = round2(medianFloat64(actual))
	result.MedianEstimated = round2(medianFloat64(estimated))

	scores := scorePowerBacktest(actual, estimated)
	result.Scores = scores
	// Mirror primary fields for convenience.
	result.PearsonR = scores.PearsonR
	result.SpearmanRho = scores.SpearmanRho
	result.R2 = scores.R2
	result.RMSE = scores.RMSE
	result.MAE = scores.MAE
	result.Bias = scores.Bias
	result.P95AbsError = scores.P95AbsError

	// Best ~12 min means on masked region for realism checks (contiguous mask only).
	timeSeries := NullableStream(req.Streams, "time")
	if maskStart >= 0 {
		result.Best12MinActual = round2(bestRollingMeanOnSeries(actualWattsDense(originalWatts, n), timeSeries, maskStart, n, 720))
		result.Best12MinEstimated = round2(bestRollingMeanOnSeries(fill.FilledWatts, timeSeries, maskStart, n, 720))
	}

	result.Warnings = append(result.Warnings, backtestScoreWarnings(scores)...)

	return result
}

// madZScale is the consistency constant so MAD matches σ for Gaussian noise.
const madZScale = 1.4826

// residualOutlierFence is the robust z cutoff for inlier/outlier split.
// Protocol constant for MAD-z screening, not a coaching decision threshold.
const residualOutlierFence = 3.0

// ScorePowerBacktestForTest exposes scorePowerBacktest for unit tests.
func ScorePowerBacktestForTest(actual, estimated []float64) PowerBacktestScores {
	return scorePowerBacktest(actual, estimated)
}

// scorePowerBacktest computes multi-metric fitness with MAD-based outlier handling.
func scorePowerBacktest(actual, estimated []float64) PowerBacktestScores {
	n := len(actual)
	scores := PowerBacktestScores{}
	if n == 0 || n != len(estimated) {
		return scores
	}

	residuals := make([]float64, n)
	absErrs := make([]float64, n)
	for i := range actual {
		residuals[i] = estimated[i] - actual[i]
		absErrs[i] = math.Abs(residuals[i])
	}

	scores.PearsonR = round4(pearsonR(actual, estimated))
	scores.SpearmanRho = round4(spearmanRho(actual, estimated))
	scores.R2 = round4(scores.PearsonR * scores.PearsonR)
	scores.RMSE = round2(rmseFloat64(actual, estimated))
	scores.MAE = round2(meanFloat64(absErrs))
	scores.Bias = round2(meanFloat64(estimated) - meanFloat64(actual))
	scores.P95AbsError = round2(percentileSorted(append([]float64(nil), absErrs...), 0.95))

	// Residual MAD-z (location = median residual, scale = 1.4826*MAD).
	medRes := medianFloat64(residuals)
	madRes := madFloat64(residuals, medRes)
	scores.ResidualMADWatts = round2(madRes)
	scale := madZScale * madRes
	absZ := make([]float64, n)
	inlierAct := make([]float64, 0, n)
	inlierEst := make([]float64, 0, n)
	outliers := 0
	for i := range residuals {
		z := 0.0
		if scale > 0 {
			z = (residuals[i] - medRes) / scale
		}
		absZ[i] = math.Abs(z)
		if absZ[i] > residualOutlierFence {
			outliers++
			continue
		}
		inlierAct = append(inlierAct, actual[i])
		inlierEst = append(inlierEst, estimated[i])
	}
	scores.OutlierSeconds = outliers
	scores.InlierSeconds = n - outliers
	scores.OutlierFraction = round4(float64(outliers) / float64(n))
	scores.ResidualZMeanAbs = round4(meanFloat64(absZ))
	scores.ResidualZMedianAbs = round4(medianFloat64(absZ))
	scores.ResidualZP95Abs = round4(percentileSorted(append([]float64(nil), absZ...), 0.95))

	if len(inlierAct) >= 10 {
		inlierAbs := make([]float64, len(inlierAct))
		for i := range inlierAct {
			inlierAbs[i] = math.Abs(inlierEst[i] - inlierAct[i])
		}
		scores.RobustRMSE = round2(rmseFloat64(inlierAct, inlierEst))
		scores.RobustMAE = round2(meanFloat64(inlierAbs))
		scores.RobustBias = round2(meanFloat64(inlierEst) - meanFloat64(inlierAct))
	}

	// Shape agreement after robust z-standardization of each series.
	zAct := madStandardize(actual)
	zEst := madStandardize(estimated)
	scores.ZScorePearsonR = round4(pearsonR(zAct, zEst))

	return scores
}

func backtestScoreWarnings(scores PowerBacktestScores) []string {
	var warnings []string
	if scores.PearsonR < 0.5 {
		warnings = append(warnings, fmt.Sprintf(
			"low pearsonR=%.3f — linear tracking of measured power is weak", scores.PearsonR,
		))
	}
	if scores.SpearmanRho < 0.5 {
		warnings = append(warnings, fmt.Sprintf(
			"low spearmanRho=%.3f — rank order of estimated vs measured power is weak (outlier-robust)",
			scores.SpearmanRho,
		))
	}
	if scores.ZScorePearsonR < 0.5 {
		warnings = append(warnings, fmt.Sprintf(
			"low zScorePearsonR=%.3f — MAD-standardized series shapes disagree", scores.ZScorePearsonR,
		))
	}
	if scores.ResidualZMedianAbs > 1.0 {
		warnings = append(warnings, fmt.Sprintf(
			"residualZMedianAbs=%.2f — typical residual exceeds 1 MAD-z (scale MAD=%.1f W)",
			scores.ResidualZMedianAbs, scores.ResidualMADWatts,
		))
	}
	if scores.OutlierFraction > 0.1 {
		warnings = append(warnings, fmt.Sprintf(
			"outlierFraction=%.1f%% of samples have |residual MAD-z|>%.0f — inspect spikes/descents",
			scores.OutlierFraction*100, residualOutlierFence,
		))
	}

	return warnings
}

// madStandardize returns MAD-based z-scores: (x - median) / (1.4826 * MAD).
// Constant series returns zeros.
func madStandardize(values []float64) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 {
		return out
	}
	med := medianFloat64(values)
	mad := madFloat64(values, med)
	scale := madZScale * mad
	if scale <= 0 {
		return out
	}
	for i, value := range values {
		out[i] = (value - med) / scale
	}

	return out
}

// spearmanRho is Pearson correlation of ranks (average ranks for ties).
func spearmanRho(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}

	return pearsonR(rankAverage(x), rankAverage(y))
}

func rankAverage(values []float64) []float64 {
	n := len(values)
	type pair struct {
		value float64
		index int
	}
	items := make([]pair, n)
	for i, value := range values {
		items[i] = pair{value: value, index: i}
	}
	// Stable sort by value.
	for i := 1; i < n; i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && items[j].value > key.value {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
	ranks := make([]float64, n)
	i := 0
	for i < n {
		j := i
		for j+1 < n && items[j+1].value == items[i].value {
			j++
		}
		// Average rank for ties (1-based ranks).
		avg := float64(i+j+2) / 2.0
		for k := i; k <= j; k++ {
			ranks[items[k].index] = avg
		}
		i = j + 1
	}

	return ranks
}

func backtestMaskStart(mode string, fraction float64, n int) (int, error) {
	switch mode {
	case PowerBacktestMaskSecondHalf:
		return n / 2, nil
	case PowerBacktestMaskAfterFraction:
		if fraction <= 0 || fraction >= 1 {
			return 0, fmt.Errorf("mask_after_fraction requires 0 < fraction < 1, got %v", fraction)
		}
		start := int(float64(n) * fraction)
		if start < 1 {
			start = 1
		}
		if start >= n {
			start = n - 1
		}
		return start, nil
	default:
		return 0, fmt.Errorf("unknown backtest mode %q", mode)
	}
}

// maskStreamsScatter masks a deterministic fraction of samples across the ride
// (every Nth sample after a stride), simulating sparse dropouts with neighbors
// still available — outdoor model fidelity, not PM death.
func maskStreamsScatter(streams NullableStreamData, fraction float64) (NullableStreamData, int, error) {
	if fraction <= 0 || fraction >= 1 {
		return nil, 0, fmt.Errorf("mask_scatter requires 0 < fraction < 1, got %v", fraction)
	}
	watts := NullableStream(streams, "watts")
	n := watts.Len()
	if n < 60 {
		return nil, 0, fmt.Errorf("series too short for scatter mask")
	}
	// Deterministic: mask every stride-th sample.
	stride := int(math.Round(1 / fraction))
	if stride < 2 {
		stride = 2
	}
	out := make(NullableStreamData, len(streams))
	for key, series := range streams {
		values := append([]float64(nil), series.Values...)
		present := append([]bool(nil), series.Present...)
		length := len(values)
		for len(present) < length {
			present = append(present, false)
		}
		out[key] = NullableSeries{Values: values, Present: present}
	}
	// Ensure watts key.
	if _, ok := out["watts"]; !ok {
		if power, ok := out["power"]; ok {
			out["watts"] = power
		}
	}
	w := out["watts"]
	c := out["cadence"]
	masked := 0
	for i := 0; i < n; i += stride {
		if i >= len(w.Values) {
			break
		}
		w.Values[i] = 0
		w.Present[i] = true
		if len(c.Values) > i {
			c.Present[i] = false
			c.Values[i] = 0
		}
		masked++
	}
	out["watts"] = w
	if c.Len() > 0 {
		out["cadence"] = c
	}

	return out, masked, nil
}

// maskStreamsForBacktest clones streams and applies PM-death masking from maskStart:
// watts → present 0, cadence → absent. Other streams unchanged.
func maskStreamsForBacktest(streams NullableStreamData, maskStart int) NullableStreamData {
	out := make(NullableStreamData, len(streams))
	for key, series := range streams {
		values := append([]float64(nil), series.Values...)
		present := append([]bool(nil), series.Present...)
		// Align length safety.
		length := len(values)
		if len(present) < length {
			// pad present
			for len(present) < length {
				present = append(present, false)
			}
		}
		switch key {
		case "watts", "power":
			for i := maskStart; i < length; i++ {
				values[i] = 0
				present[i] = true // device still writes zeros after PM death
			}
		case "cadence":
			for i := maskStart; i < length; i++ {
				present[i] = false // cadence channel dies with the PM
				values[i] = 0
			}
		}
		out[key] = NullableSeries{Values: values, Present: present}
	}
	// Ensure watts exists for classification even if only "power" was present.
	if _, ok := out["watts"]; !ok {
		if power, ok := out["power"]; ok {
			out["watts"] = power
		}
	}

	return out
}

func actualWattsDense(watts NullableSeries, n int) []float64 {
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		if v, ok := watts.At(i); ok {
			out[i] = v
		}
	}

	return out
}

func pearsonR(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}
	meanX := meanFloat64(x)
	meanY := meanFloat64(y)
	var num, denX, denY float64
	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		num += dx * dy
		denX += dx * dx
		denY += dy * dy
	}
	den := math.Sqrt(denX * denY)
	if den == 0 {
		return 0
	}

	return num / den
}

func rmseFloat64(actual, estimated []float64) float64 {
	if len(actual) == 0 || len(actual) != len(estimated) {
		return 0
	}
	var sum float64
	for i := range actual {
		d := estimated[i] - actual[i]
		sum += d * d
	}

	return math.Sqrt(sum / float64(len(actual)))
}

func bestRollingMeanOnSeries(values []float64, timeSeries NullableSeries, start, end, durationSec int) float64 {
	if end > len(values) {
		end = len(values)
	}
	if start < 0 {
		start = 0
	}
	if end-start < 2 || durationSec <= 0 {
		return 0
	}
	left := start
	var sum float64
	best := 0.0
	for right := start; right < end; right++ {
		sum += values[right]
		for left < right {
			span := rollingSpanSeconds(timeSeries, left, right)
			if span <= float64(durationSec) {
				break
			}
			sum -= values[left]
			left++
		}
		span := rollingSpanSeconds(timeSeries, left, right)
		if span >= float64(durationSec)*0.95 {
			avg := sum / float64(right-left+1)
			if avg > best {
				best = avg
			}
		}
	}

	return best
}
