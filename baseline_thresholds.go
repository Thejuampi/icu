package icu

import (
	"math"
	"sort"
)

// Readiness and analysis thresholds are not fixed numbers: each one is
// derived from the athlete's own history with a robust baseline (median and
// MAD), and can be pinned per athlete in the training profile.
//
// Method (documented statistical parameters, not physiological limits):
//
//   - baseline = median of the athlete's own samples in the analysis window,
//     excluding the value being judged;
//   - scale = MAD × 1.4826 (consistent with one standard deviation for
//     normally distributed data), falling back to the standard deviation
//     when the MAD is zero;
//   - robust z = (value − baseline) / scale;
//   - WATCH when the robust z is past WatchZ in the "bad" direction, RED when
//     it is past RedZ;
//   - with fewer than MinSamples baseline samples, or no spread, the level is
//     "" (insufficient history) rather than a guess.
const (
	// madToSigma converts a MAD into a normal-consistent standard deviation.
	madToSigma = 1.4826
	// DefaultBaselineWatchZ is the robust z that marks a WATCH deviation
	// (≈ the outer 7% of a normal one-sided tail).
	DefaultBaselineWatchZ = 1.5
	// DefaultBaselineRedZ is the robust z that marks a RED deviation
	// (≈ the outer 0.6% of a normal one-sided tail).
	DefaultBaselineRedZ = 2.5
	// DefaultBaselineMinSamples is the minimum baseline size for a robust
	// median/MAD (two weeks of daily samples).
	DefaultBaselineMinSamples = 14
)

// thresholdSourceProfile labels a bound pinned in the training profile.
const thresholdSourceProfile = "profile"

// Baseline levels.
const (
	BaselineLevelOK    = "OK"
	BaselineLevelWatch = "WATCH"
	BaselineLevelRed   = "RED"
)

// Bad-direction for a metric.
const (
	baselineLowIsBad  = -1
	baselineHighIsBad = 1
)

// TrainingThresholds are per-athlete overrides stored in the training
// profile. WatchZ/RedZ/MinSamples tune the robust method; every other field
// pins an absolute bound that replaces the derived one. Nil means "derive".
type TrainingThresholds struct {
	WatchZ     float64 `json:"watchZ,omitempty"`
	RedZ       float64 `json:"redZ,omitempty"`
	MinSamples int     `json:"minSamples,omitempty"`

	HRVWatch            *float64 `json:"hrvWatch,omitempty"`
	HRVRed              *float64 `json:"hrvRed,omitempty"`
	RestingHRWatch      *float64 `json:"restingHrWatch,omitempty"`
	RestingHRRed        *float64 `json:"restingHrRed,omitempty"`
	SleepWatch          *float64 `json:"sleepWatch,omitempty"`
	SleepRed            *float64 `json:"sleepRed,omitempty"`
	LactateWatch        *float64 `json:"lactateWatch,omitempty"`
	LactateHigh         *float64 `json:"lactateHigh,omitempty"`
	TSBWatch            *float64 `json:"tsbWatch,omitempty"`
	TSBRed              *float64 `json:"tsbRed,omitempty"`
	DecouplingWatchPct  *float64 `json:"decouplingWatchPct,omitempty"`
	AdaptationChangePct *float64 `json:"adaptationChangePct,omitempty"`
}

// DerivedThreshold is a WATCH/RED bound pair in the metric's own units, with
// where it came from.
type DerivedThreshold struct {
	Watch    float64 `json:"watch"`
	Red      float64 `json:"red"`
	Baseline float64 `json:"baseline,omitempty"`
	Scale    float64 `json:"scale,omitempty"`
	Samples  int     `json:"samples,omitempty"`
	// Source is "baseline_mad", "baseline_stddev" or "profile".
	Source string `json:"source"`
	// WatchOnly is set when only the WATCH bound is pinned and there is no
	// history for RED: the signal can reach WATCH but never RED.
	WatchOnly bool `json:"watchOnly,omitempty"`
}

type baselineMethod struct {
	watchZ     float64
	redZ       float64
	minSamples int
}

func thresholdMethod(overrides *TrainingThresholds) baselineMethod {
	method := baselineMethod{
		watchZ: DefaultBaselineWatchZ, redZ: DefaultBaselineRedZ, minSamples: DefaultBaselineMinSamples,
	}
	if overrides == nil {
		return method
	}
	if overrides.WatchZ > 0 {
		method.watchZ = overrides.WatchZ
	}
	if overrides.RedZ > 0 {
		method.redZ = overrides.RedZ
	}
	if overrides.MinSamples > 0 {
		method.minSamples = overrides.MinSamples
	}

	return method
}

// robustScale returns median and a normal-consistent spread (MAD, else
// standard deviation) of values, and which estimator was used.
func robustScale(values []float64) (float64, float64, string) {
	if len(values) == 0 {
		return 0, 0, ""
	}
	median := medianOf(values)
	deviations := make([]float64, 0, len(values))
	for _, value := range values {
		deviations = append(deviations, math.Abs(value-median))
	}
	if mad := medianOf(deviations); mad > 0 {
		return median, mad * madToSigma, "baseline_mad"
	}
	var sum, squares float64
	for _, value := range values {
		sum += value
	}
	mean := sum / float64(len(values))
	for _, value := range values {
		squares += (value - mean) * (value - mean)
	}
	if spread := math.Sqrt(squares / float64(len(values))); spread > 0 {
		return median, spread, "baseline_stddev"
	}

	return median, 0, ""
}

func medianOf(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	midpoint := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[midpoint]
	}

	return (sorted[midpoint-1] + sorted[midpoint]) / 2
}

// deriveThreshold builds WATCH/RED bounds from baseline values. direction is
// baselineLowIsBad or baselineHighIsBad. Profile bounds (watch/red) win when
// set; a nil result means there is not enough history and no override.
func deriveThreshold(
	values []float64, direction int, method baselineMethod, watch, red *float64,
) *DerivedThreshold {
	if watch != nil && red != nil {
		return &DerivedThreshold{Watch: *watch, Red: *red, Source: thresholdSourceProfile}
	}
	if len(values) < method.minSamples {
		return profilePartialThreshold(watch, red)
	}
	median, scale, source := robustScale(values)
	if scale == 0 {
		return profilePartialThreshold(watch, red)
	}
	sign := float64(direction)
	derived := &DerivedThreshold{
		Watch:    round2(median + sign*method.watchZ*scale),
		Red:      round2(median + sign*method.redZ*scale),
		Baseline: round2(median),
		Scale:    round2(scale),
		Samples:  len(values),
		Source:   source,
	}
	if watch != nil {
		derived.Watch, derived.Source = *watch, thresholdSourceProfile
	}
	if red != nil {
		derived.Red, derived.Source = *red, thresholdSourceProfile
	}

	return derived
}

// profilePartialThreshold covers a single pinned bound with no history. A
// pinned WATCH alone never escalates to RED (WatchOnly); a pinned RED alone
// is also the WATCH bound, so crossing it is RED.
func profilePartialThreshold(watch, red *float64) *DerivedThreshold {
	switch {
	case watch != nil:
		return &DerivedThreshold{Watch: *watch, Red: *watch, Source: thresholdSourceProfile, WatchOnly: true}
	case red != nil:
		return &DerivedThreshold{Watch: *red, Red: *red, Source: thresholdSourceProfile}
	default:
		return nil
	}
}

// thresholdLevel classifies value against bounds in the bad direction.
func thresholdLevel(value float64, threshold *DerivedThreshold, direction int) string {
	if threshold == nil {
		return ""
	}
	past := func(bound float64) bool {
		if direction == baselineLowIsBad {
			return value <= bound
		}

		return value >= bound
	}
	switch {
	case !threshold.WatchOnly && past(threshold.Red):
		return BaselineLevelRed
	case past(threshold.Watch):
		return BaselineLevelWatch
	default:
		return BaselineLevelOK
	}
}

func overrideOf(overrides *TrainingThresholds, pick func(*TrainingThresholds) *float64) *float64 {
	if overrides == nil {
		return nil
	}

	return pick(overrides)
}
