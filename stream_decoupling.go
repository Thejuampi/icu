package icu

import (
	"errors"
	"math"
	"sort"
)

// Decoupling sources, emitted as Activity.DecouplingSource (B15).
//
// intervals.icu's `decoupling` activity field is not reliable: on
// i202545713 (Z2, 2026-10-27) it reports 8.26% while the raw streams show
// ~0% drift. The CLI therefore recomputes aerobic decoupling from the
// activity streams and only falls back to the API field, labeled, when the
// streams cannot be read or measured.
const (
	DecouplingSourceStreams = "streams"
	DecouplingSourceAPI     = "api_fallback"

	DecouplingBasisPowerHR = "power_hr"
	DecouplingBasisPaceHR  = "pace_hr"

	// DecouplingAlgorithmVersion is bumped whenever the stream algorithm
	// changes so cached results are recomputed.
	DecouplingAlgorithmVersion = 1
)

// Default stream-decoupling parameters (B15). On the reference ride
// i202545713 (107 min) the defaults reproduce intervals.icu's own analysis
// range (minutes 20-95).
const (
	DefaultDecouplingWarmupSeconds   = 1200
	DefaultDecouplingCooldownSeconds = 720
	// DefaultDecouplingMinPowerWatts: minutes whose average power is below
	// this are coasting/stops and are excluded (low_power).
	DefaultDecouplingMinPowerWatts = 50
	// DefaultDecouplingHRLagSeconds: HR responds to power with a delay, so
	// each power sample is paired with the HR sample this much later.
	DefaultDecouplingHRLagSeconds = 23
	// DefaultDecouplingIQRK: minutes whose power/HR ratio falls outside
	// [Q1 - k*IQR, Q3 + k*IQR] are excluded (ratio_outlier).
	DefaultDecouplingIQRK = 1.5
	// DefaultDecouplingMinSteadyMinutes: fewer kept minutes means the ride is
	// too short to measure drift.
	DefaultDecouplingMinSteadyMinutes = 20
	// decouplingStoppedSpeed: pace basis, a minute with mean speed at or
	// below this is a stop (works for m/s and km/h).
	decouplingStoppedSpeed = 0.5
	secondsPerMinuteF      = 60.0
)

// Exclusion reasons reported in StreamDecoupling.Excluded.
const (
	DecouplingExcludedLowPower     = "low_power"
	DecouplingExcludedMissingHR    = "missing_hr"
	DecouplingExcludedRatioOutlier = "ratio_outlier"
)

// ErrDecouplingInsufficientStreams means the streams cannot support a
// stream-based decoupling value (missing HR, no power and no speed, or too
// few steady minutes after trimming/filtering).
var ErrDecouplingInsufficientStreams = errors.New("insufficient stream data for decoupling")

// DecouplingOptions configures ComputeStreamDecoupling.
type DecouplingOptions struct {
	WarmupSeconds    int     `json:"warmupSeconds"`
	CooldownSeconds  int     `json:"cooldownSeconds"`
	MinPowerWatts    float64 `json:"minPowerWatts"`
	HRLagSeconds     int     `json:"hrLagSeconds"`
	IQRK             float64 `json:"iqrK"`
	MinSteadyMinutes int     `json:"minSteadyMinutes"`
}

// DefaultDecouplingOptions returns the documented defaults.
func DefaultDecouplingOptions() DecouplingOptions {
	return DecouplingOptions{
		WarmupSeconds:    DefaultDecouplingWarmupSeconds,
		CooldownSeconds:  DefaultDecouplingCooldownSeconds,
		MinPowerWatts:    DefaultDecouplingMinPowerWatts,
		HRLagSeconds:     DefaultDecouplingHRLagSeconds,
		IQRK:             DefaultDecouplingIQRK,
		MinSteadyMinutes: DefaultDecouplingMinSteadyMinutes,
	}
}

// DecouplingExcludedMinute is one excluded minute (minute index from the
// activity start) and why.
type DecouplingExcludedMinute struct {
	Minute int    `json:"minute"`
	Reason string `json:"reason"`
}

// StreamDecoupling is the result of a stream-based decoupling computation.
type StreamDecoupling struct {
	Value            float64                    `json:"value"`
	Basis            string                     `json:"basis"`
	Method           string                     `json:"method"`
	Options          DecouplingOptions          `json:"options"`
	RangeStartMinute int                        `json:"rangeStartMinute"`
	RangeEndMinute   int                        `json:"rangeEndMinute"`
	MinutesInRange   int                        `json:"minutesInRange"`
	MinutesKept      int                        `json:"minutesKept"`
	Excluded         map[string]int             `json:"excluded,omitempty"`
	ExcludedMinutes  []DecouplingExcludedMinute `json:"excludedMinutes,omitempty"`
	// RatioFenceLow/High are the Tukey fences; nil when IQR screening is
	// disabled (k=0) or there are too few minutes.
	RatioFenceLow    *float64 `json:"ratioFenceLow,omitempty"`
	RatioFenceHigh   *float64 `json:"ratioFenceHigh,omitempty"`
	FirstHalfOutput  float64  `json:"firstHalfOutput"`
	FirstHalfHR      float64  `json:"firstHalfHr"`
	SecondHalfOutput float64  `json:"secondHalfOutput"`
	SecondHalfHR     float64  `json:"secondHalfHr"`
}

// DecouplingStreamTypes is the stream request used by the decoupling fetch.
const DecouplingStreamTypes = "time,watts,heartrate,velocity_smooth"

type decouplingMinute struct {
	index  int
	output float64
	hr     float64
	reason string
}

// ComputeStreamDecoupling computes Friel aerobic decoupling (Pa:HR, or
// pace:HR when there is no power stream) from activity streams, on
// per-minute averages:
//
//  1. trim: only whole minutes in [warm-up, total - cool-down) are used;
//  2. each minute averages output (null samples skipped; zeros kept, so a
//     coasting minute shows low power) and HR shifted by HRLagSeconds;
//  3. minutes with HR coverage < 50% are missing_hr; minutes with mean power
//     < MinPowerWatts (pace basis: mean speed <= stop speed) are low_power;
//  4. the per-minute output/HR ratio is screened with Tukey fences
//     (k*IQR, computed over all trimmed minutes with HR): ratio_outlier;
//  5. kept minutes are split at the time midpoint of the range and
//     decoupling = (1 - (out2/hr2)/(out1/hr1)) * 100, each half's out and
//     hr being the mean of its kept minutes (filtered ratio of means).
func ComputeStreamDecoupling(streams NullableStreamData, options DecouplingOptions) (StreamDecoupling, error) {
	result := StreamDecoupling{Options: options, Method: "filtered_ratio_per_minute"}
	hr, hasHR := streams["heartrate"]
	if !hasHR || hr.Len() == 0 {
		return result, ErrDecouplingInsufficientStreams
	}
	output, hasOutput := streams["watts"]
	result.Basis = DecouplingBasisPowerHR
	if !hasOutput || !seriesHasPositive(output) {
		output, hasOutput = streams["velocity_smooth"]
		result.Basis = DecouplingBasisPaceHR
	}
	if !hasOutput || !seriesHasPositive(output) {
		return result, ErrDecouplingInsufficientStreams
	}

	count := minInt(hr.Len(), output.Len())
	elapsed := decouplingElapsed(streams["time"], count)
	totalMinutes := int(math.Round(elapsed[count-1] / secondsPerMinuteF))
	result.RangeStartMinute = options.WarmupSeconds / int(secondsPerMinuteF)
	result.RangeEndMinute = totalMinutes - options.CooldownSeconds/int(secondsPerMinuteF)
	if result.RangeEndMinute <= result.RangeStartMinute {
		return result, ErrDecouplingInsufficientStreams
	}

	minutes := decouplingMinutes(hr, output, elapsed, count, &result, options)
	result.MinutesInRange = len(minutes)
	fenceLow, fenceHigh := decouplingRatioFences(minutes, options.IQRK)
	if !math.IsInf(fenceLow, 0) {
		low, high := round4(fenceLow), round4(fenceHigh)
		result.RatioFenceLow, result.RatioFenceHigh = &low, &high
	}

	mid := float64(result.RangeStartMinute+result.RangeEndMinute) / 2
	var first, second []decouplingMinute
	result.Excluded = map[string]int{}
	for _, minute := range minutes {
		if minute.reason == "" {
			ratio := minute.output / minute.hr
			if ratio < fenceLow || ratio > fenceHigh {
				minute.reason = DecouplingExcludedRatioOutlier
			}
		}
		if minute.reason != "" {
			result.Excluded[minute.reason]++
			result.ExcludedMinutes = append(result.ExcludedMinutes, DecouplingExcludedMinute{minute.index, minute.reason})

			continue
		}
		result.MinutesKept++
		if float64(minute.index) < mid {
			first = append(first, minute)
		} else {
			second = append(second, minute)
		}
	}
	if result.MinutesKept < options.MinSteadyMinutes || len(first) == 0 || len(second) == 0 {
		return result, ErrDecouplingInsufficientStreams
	}

	result.FirstHalfOutput, result.FirstHalfHR = decouplingMeans(first)
	result.SecondHalfOutput, result.SecondHalfHR = decouplingMeans(second)
	if result.FirstHalfOutput <= 0 {
		return result, ErrDecouplingInsufficientStreams
	}
	ratio1 := result.FirstHalfOutput / result.FirstHalfHR
	ratio2 := result.SecondHalfOutput / result.SecondHalfHR
	result.Value = round2((1 - ratio2/ratio1) * percentScale)
	result.FirstHalfOutput = round2(result.FirstHalfOutput)
	result.FirstHalfHR = round2(result.FirstHalfHR)
	result.SecondHalfOutput = round2(result.SecondHalfOutput)
	result.SecondHalfHR = round2(result.SecondHalfHR)

	return result, nil
}

func decouplingMinutes(
	hr, output NullableSeries,
	elapsed []float64,
	count int,
	result *StreamDecoupling,
	options DecouplingOptions,
) []decouplingMinute {
	hrAt := make(map[int]float64, count)
	for index := range count {
		if value, ok := hr.At(index); ok && value > 0 {
			hrAt[int(elapsed[index])] = value
		}
	}

	type bucket struct {
		samples, outputN, hrN int
		output, hr            float64
	}
	buckets := map[int]*bucket{}
	for index := range count {
		minute := int(elapsed[index] / secondsPerMinuteF)
		if minute < result.RangeStartMinute || minute >= result.RangeEndMinute {
			continue
		}
		b := buckets[minute]
		if b == nil {
			b = &bucket{}
			buckets[minute] = b
		}
		b.samples++
		if value, ok := output.At(index); ok {
			b.output += value
			b.outputN++
		}
		if value, ok := hrAt[int(elapsed[index])+options.HRLagSeconds]; ok {
			b.hr += value
			b.hrN++
		}
	}

	minutes := make([]decouplingMinute, 0, len(buckets))
	for minute := result.RangeStartMinute; minute < result.RangeEndMinute; minute++ {
		b := buckets[minute]
		if b == nil {
			continue // no samples recorded (paused): not part of the ride
		}
		entry := decouplingMinute{index: minute}
		if b.outputN > 0 {
			entry.output = b.output / float64(b.outputN)
		}
		if b.hrN > 0 {
			entry.hr = b.hr / float64(b.hrN)
		}
		switch {
		case decouplingLowOutput(entry.output, result.Basis, options):
			entry.reason = DecouplingExcludedLowPower
		case b.hrN*2 < b.samples:
			entry.reason = DecouplingExcludedMissingHR
		}
		minutes = append(minutes, entry)
	}

	return minutes
}

func decouplingLowOutput(value float64, basis string, options DecouplingOptions) bool {
	if basis == DecouplingBasisPowerHR {
		return value < options.MinPowerWatts
	}

	return value <= decouplingStoppedSpeed
}

// decouplingRatioFences computes Tukey fences on the per-minute output/HR
// ratio over every trimmed minute with HR (low-power minutes included, so
// coasting widens nothing it would not already exclude).
func decouplingRatioFences(minutes []decouplingMinute, k float64) (float64, float64) {
	var ratios []float64
	for _, minute := range minutes {
		if minute.hr > 0 && minute.reason != DecouplingExcludedMissingHR {
			ratios = append(ratios, minute.output/minute.hr)
		}
	}
	if len(ratios) < 4 || k <= 0 {
		return math.Inf(-1), math.Inf(1)
	}
	sort.Float64s(ratios)
	q1, q3 := linearQuantile(ratios, 0.25), linearQuantile(ratios, 0.75)
	iqr := q3 - q1

	return q1 - k*iqr, q3 + k*iqr
}

func linearQuantile(sorted []float64, q float64) float64 {
	position := float64(len(sorted)-1) * q
	lower := int(position)
	upper := minInt(lower+1, len(sorted)-1)

	return sorted[lower] + (position-float64(lower))*(sorted[upper]-sorted[lower])
}

func seriesHasPositive(series NullableSeries) bool {
	for index := range series.Values {
		if value, ok := series.At(index); ok && value > 0 {
			return true
		}
	}

	return false
}

func decouplingElapsed(timeSeries NullableSeries, count int) []float64 {
	elapsed := make([]float64, count)
	last := -1.0
	for index := range count {
		value, ok := timeSeries.At(index)
		if !ok || value <= last {
			value = last + 1
		}
		elapsed[index] = value
		last = value
	}

	return elapsed
}

func decouplingMeans(minutes []decouplingMinute) (float64, float64) {
	sumOutput, sumHR := 0.0, 0.0
	for _, minute := range minutes {
		sumOutput += minute.output
		sumHR += minute.hr
	}
	n := float64(len(minutes))

	return sumOutput / n, sumHR / n
}

// HasDecoupling reports whether the activity carries a decoupling value.
// A stream-computed value of exactly 0 is a real measurement (no drift), so
// it counts; an API value of 0 historically meant "absent".
func (activity *Activity) HasDecoupling() bool {
	return activity.DecouplingSource == DecouplingSourceStreams || activity.Decoupling != 0
}

// ApplyStreamDecoupling replaces the API decoupling with the stream value,
// keeping the API value for comparison.
func (activity *Activity) ApplyStreamDecoupling(result *StreamDecoupling) {
	if activity.Decoupling != 0 {
		api := activity.Decoupling
		activity.DecouplingAPI = &api
	}
	activity.Decoupling = result.Value
	activity.DecouplingSource = DecouplingSourceStreams
	activity.DecouplingBasis = result.Basis
	activity.DecouplingNote = ""
}

// MarkAPIDecouplingFallback labels a retained API decoupling value.
func (activity *Activity) MarkAPIDecouplingFallback(reason string) {
	if activity.Decoupling == 0 {
		return
	}
	api := activity.Decoupling
	activity.DecouplingAPI = &api
	activity.DecouplingSource = DecouplingSourceAPI
	activity.DecouplingNote = reason
}
