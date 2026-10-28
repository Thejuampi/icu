package icu

import (
	"errors"
	"math"
	"sort"
	"strings"
)

// Decoupling sources, emitted as Activity.DecouplingSource (B15).
//
// intervals.icu's `decoupling` field stays the default consumer value: the
// stream method below is reported next to it and becomes the consumed value
// only with --decoupling-source streams, until it has been validated against
// more rides.
const (
	DecouplingSourceStreams = "streams"
	DecouplingSourceAPI     = "api"

	DecouplingBasisPowerHR = "power_hr"
	DecouplingBasisGAPHR   = "gap_hr"
	DecouplingBasisPaceHR  = "pace_hr"

	// DecouplingAlgorithmVersion is bumped whenever the stream algorithm
	// changes so cached results are recomputed.
	DecouplingAlgorithmVersion = 2
)

// Stream decoupling status values.
const (
	DecouplingStatusMeasured      = "measured"
	DecouplingStatusNotMeasurable = "not_measurable"
	DecouplingStatusUnavailable   = "unavailable"
)

// Method parameters. None of these is fitted to a ride: they are textbook
// conventions or search bounds, documented where used.
const (
	// DefaultDecouplingIQRK is Tukey's conventional outlier fence multiplier.
	DefaultDecouplingIQRK = 1.5
	// decouplingMaxHRLagSeconds bounds the HR-lag cross-correlation search
	// (HR kinetics settle within ~2 min); the lag itself is measured per ride.
	decouplingMaxHRLagSeconds = 120
	// decouplingMinMinutesPerHalf: each half needs at least the four points a
	// quartile/IQR needs to be meaningful.
	decouplingMinMinutesPerHalf = 4
	secondsPerMinuteF           = 60.0
	decouplingHalf              = 0.5
	quartileLow                 = 0.25
	quartileHigh                = 0.75
)

// Trim and HR-lag provenance.
const (
	DecouplingTrimManual       = "manual"
	DecouplingTrimWorkoutSteps = "workout_steps"
	DecouplingTrimSteadyState  = "steady_state"
	DecouplingLagManual        = "manual"
	DecouplingLagCorrelation   = "cross_correlation"
	DecouplingLagNone          = "none"
)

// Exclusion reasons reported in StreamDecoupling.Excluded.
const (
	DecouplingExcludedCoasting     = "coasting"
	DecouplingExcludedMissingHR    = "missing_hr"
	DecouplingExcludedRatioOutlier = "ratio_outlier"
)

// ErrDecouplingInsufficientStreams means the streams cannot support a
// stream-based decoupling value (missing HR, no output stream, or too few
// steady minutes after trimming/filtering).
var ErrDecouplingInsufficientStreams = errors.New("insufficient stream data for decoupling")

// DecouplingOptions are user overrides; nil means "derive from the ride".
type DecouplingOptions struct {
	WarmupSeconds   *int    `json:"warmupSeconds,omitempty"`
	CooldownSeconds *int    `json:"cooldownSeconds,omitempty"`
	HRLagSeconds    *int    `json:"hrLagSeconds,omitempty"`
	IQRK            float64 `json:"iqrK"`
}

// DefaultDecouplingOptions derives trims and lag and uses Tukey's k=1.5.
func DefaultDecouplingOptions() DecouplingOptions {
	return DecouplingOptions{IQRK: DefaultDecouplingIQRK}
}

// DecouplingContext is per-activity metadata the stream method uses.
type DecouplingContext struct {
	// MovingTimeSeconds (intervals.icu moving_time) derives the stop speed.
	MovingTimeSeconds int `json:"movingTimeSeconds,omitempty"`
	// Plan is the paired workout's warm-up/cool-down, when there is one.
	Plan *DecouplingPlanTrim `json:"plan,omitempty"`
}

// DecouplingPlanTrim is the warm-up/cool-down read from a workout doc.
type DecouplingPlanTrim struct {
	WarmupSeconds   int `json:"warmupSeconds"`
	CooldownSeconds int `json:"cooldownSeconds"`
}

// DecouplingTrim reports the applied trim, in moving seconds.
type DecouplingTrim struct {
	Method          string `json:"method"`
	WarmupSeconds   int    `json:"warmupSeconds"`
	CooldownSeconds int    `json:"cooldownSeconds"`
}

// DecouplingExcludedMinute is one excluded minute (moving-time minute index
// counted from the trim start) and why.
type DecouplingExcludedMinute struct {
	Minute int    `json:"minute"`
	Reason string `json:"reason"`
}

// StreamDecoupling is the result of a stream-based decoupling computation.
type StreamDecoupling struct {
	Status string `json:"status"`
	// Value is nil unless the streams yielded a number; a real 0.0 is kept.
	Value            *float64                   `json:"value,omitempty"`
	Reason           string                     `json:"reason,omitempty"`
	Basis            string                     `json:"basis,omitempty"`
	Method           string                     `json:"method"`
	Options          DecouplingOptions          `json:"options"`
	Trim             DecouplingTrim             `json:"trim"`
	HRLagSeconds     int                        `json:"hrLagSeconds"`
	HRLagMethod      string                     `json:"hrLagMethod"`
	HRLagCorrelation *float64                   `json:"hrLagCorrelation,omitempty"`
	StopSpeed        *float64                   `json:"stopSpeed,omitempty"`
	MovingSeconds    int                        `json:"movingSeconds"`
	MinutesInRange   int                        `json:"minutesInRange"`
	MinutesKept      int                        `json:"minutesKept"`
	Excluded         map[string]int             `json:"excluded,omitempty"`
	ExcludedMinutes  []DecouplingExcludedMinute `json:"excludedMinutes,omitempty"`
	RatioFenceLow    *float64                   `json:"ratioFenceLow,omitempty"`
	RatioFenceHigh   *float64                   `json:"ratioFenceHigh,omitempty"`
	FirstHalfOutput  float64                    `json:"firstHalfOutput,omitempty"`
	FirstHalfHR      float64                    `json:"firstHalfHr,omitempty"`
	SecondHalfOutput float64                    `json:"secondHalfOutput,omitempty"`
	SecondHalfHR     float64                    `json:"secondHalfHr,omitempty"`
	VariabilityIndex float64                    `json:"variabilityIndex,omitempty"`
	VICutoff         *float64                   `json:"viCutoff,omitempty"`
}

// DecouplingStreamTypes is the stream request used by the decoupling fetch
// (all speeds are m/s in the intervals.icu API).
const DecouplingStreamTypes = "time,watts,heartrate,velocity_smooth,ga_velocity"

type decouplingMinute struct {
	index  int
	output float64
	hr     float64
	reason string
}

// decouplingRide is the moving-time view of the streams.
type decouplingRide struct {
	hr, output NullableSeries
	elapsed    []float64
	moving     []int // sample indices of moving samples, in order
	step       float64
	hrAt       map[int]float64
	power      bool
}

// ComputeStreamDecoupling computes Friel aerobic decoupling (Pa:HR; GAP:HR
// or pace:HR without power) on moving-time minutes:
//
//  1. moving samples: speed above a stop speed derived so the moving sample
//     time matches intervals.icu's moving_time (all samples without speed);
//  2. trim (moving seconds): user override, else the paired workout's
//     warm-up/cool-down steps, then steady-state edges (leading/trailing
//     minutes whose output is below the ride's own lower quartile);
//  3. HR lag: user override, else the 0-120 s lag maximizing the
//     output/HR cross-correlation (0 when no positive correlation);
//  4. 60-moving-second buckets from the trim start; a minute is missing_hr
//     when lagged HR covers under half of it and coasting (power basis) when
//     the rider pedalled (watts > 0) under half of it;
//  5. Tukey fences (k*IQR) on the per-minute output/HR ratio of the
//     minutes that survived step 4: ratio_outlier;
//  6. kept minutes split at the range midpoint; decoupling =
//     (1 - (out2/hr2)/(out1/hr1)) * 100 on each half's means.
func ComputeStreamDecoupling(streams NullableStreamData, options DecouplingOptions, context DecouplingContext) StreamDecoupling {
	result := StreamDecoupling{Options: options, Method: "moving_minutes_filtered_ratio", Status: DecouplingStatusNotMeasurable}
	ride, ok := newDecouplingRide(streams, context, &result)
	if !ok {
		result.Reason = ErrDecouplingInsufficientStreams.Error()

		return result
	}

	start, end := ride.trim(options, context, &result)
	if end-start <= 0 {
		result.Reason = "nothing left after warm-up/cool-down trim"

		return result
	}
	ride.hrLag(start, end, options, &result)
	minutes := ride.minutes(start, end, result.HRLagSeconds)
	result.MinutesInRange = len(minutes)
	fenceLow, fenceHigh := decouplingRatioFences(minutes, options.IQRK)
	if !math.IsInf(fenceLow, 0) {
		low, high := round4(fenceLow), round4(fenceHigh)
		result.RatioFenceLow, result.RatioFenceHigh = &low, &high
	}

	mid := float64(len(minutes)) / 2
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
	if len(first) < decouplingMinMinutesPerHalf || len(second) < decouplingMinMinutesPerHalf {
		result.Reason = "too few steady minutes in a half"

		return result
	}

	result.FirstHalfOutput, result.FirstHalfHR = decouplingMeans(first)
	result.SecondHalfOutput, result.SecondHalfHR = decouplingMeans(second)
	if result.FirstHalfOutput <= 0 {
		result.Reason = "no output in the first half"

		return result
	}
	ratio1 := result.FirstHalfOutput / result.FirstHalfHR
	ratio2 := result.SecondHalfOutput / result.SecondHalfHR
	value := round2((1 - ratio2/ratio1) * percentScale)
	result.Value = &value
	result.Status = DecouplingStatusMeasured
	result.FirstHalfOutput = round2(result.FirstHalfOutput)
	result.FirstHalfHR = round2(result.FirstHalfHR)
	result.SecondHalfOutput = round2(result.SecondHalfOutput)
	result.SecondHalfHR = round2(result.SecondHalfHR)

	return result
}

func newDecouplingRide(streams NullableStreamData, context DecouplingContext, result *StreamDecoupling) (*decouplingRide, bool) {
	hr, hasHR := streams["heartrate"]
	if !hasHR || !seriesHasPositive(hr) {
		return nil, false
	}
	ride := &decouplingRide{hr: hr}
	speed := streams["velocity_smooth"]
	switch {
	case seriesHasPositive(streams["watts"]):
		ride.output, result.Basis, ride.power = streams["watts"], DecouplingBasisPowerHR, true
	case seriesHasPositive(streams["ga_velocity"]):
		ride.output, result.Basis = streams["ga_velocity"], DecouplingBasisGAPHR
	case seriesHasPositive(speed):
		ride.output, result.Basis = speed, DecouplingBasisPaceHR
	default:
		return nil, false
	}
	count := minInt(hr.Len(), ride.output.Len())
	ride.elapsed = decouplingElapsed(streams["time"], count)
	ride.step = decouplingSampleStep(ride.elapsed)
	ride.hrAt = make(map[int]float64, count)
	for index := range count {
		if value, ok := hr.At(index); ok && value > 0 {
			ride.hrAt[int(ride.elapsed[index])] = value
		}
	}

	stop, hasStop := decouplingStopSpeed(speed, count, context.MovingTimeSeconds, ride.step)
	if !hasStop && !ride.power {
		stop, hasStop = decouplingStopSpeed(ride.output, count, context.MovingTimeSeconds, ride.step)
	}
	if hasStop {
		rounded := round4(stop)
		result.StopSpeed = &rounded
	}
	for index := range count {
		if hasStop {
			value, ok := speed.At(index)
			if !seriesHasPositive(speed) {
				value, ok = ride.output.At(index)
			}
			if !ok || value <= stop {
				continue
			}
		}
		ride.moving = append(ride.moving, index)
	}
	result.MovingSeconds = int(math.Round(float64(len(ride.moving)) * ride.step))

	return ride, len(ride.moving) > 0
}

// decouplingStopSpeed returns the speed at or below which a sample is a
// stop, chosen so that the samples above it add up to movingTime. Unit-free:
// it is a rank in the ride's own speed distribution.
func decouplingStopSpeed(speed NullableSeries, count, movingTime int, step float64) (float64, bool) {
	if movingTime <= 0 || !seriesHasPositive(speed) {
		return 0, false
	}
	values := make([]float64, 0, count)
	for index := range count {
		value, ok := speed.At(index)
		if !ok {
			value = 0
		}
		values = append(values, value)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(values)))
	rank := int(math.Round(float64(movingTime) / step))
	if rank >= len(values) {
		return 0, true
	}

	return values[rank], true
}

func decouplingSampleStep(elapsed []float64) float64 {
	if len(elapsed) < 2 {
		return 1
	}
	deltas := make([]float64, 0, len(elapsed)-1)
	for index := 1; index < len(elapsed); index++ {
		deltas = append(deltas, elapsed[index]-elapsed[index-1])
	}
	sort.Float64s(deltas)

	return deltas[len(deltas)/2]
}

// samplesPerMinute is the number of moving samples in one moving minute.
func (ride *decouplingRide) samplesPerMinute() int {
	return maxInt(1, int(math.Round(secondsPerMinuteF/ride.step)))
}

// trim returns [start, end) positions into ride.moving.
func (ride *decouplingRide) trim(options DecouplingOptions, context DecouplingContext, result *StreamDecoupling) (int, int) {
	toSamples := func(seconds int) int { return int(math.Round(float64(seconds) / ride.step)) }
	if options.WarmupSeconds != nil || options.CooldownSeconds != nil {
		result.Trim.Method = DecouplingTrimManual
		if options.WarmupSeconds != nil {
			result.Trim.WarmupSeconds = *options.WarmupSeconds
		}
		if options.CooldownSeconds != nil {
			result.Trim.CooldownSeconds = *options.CooldownSeconds
		}

		return toSamples(result.Trim.WarmupSeconds), len(ride.moving) - toSamples(result.Trim.CooldownSeconds)
	}

	start, end := 0, len(ride.moving)
	methods := []string{}
	if context.Plan != nil {
		start, end = toSamples(context.Plan.WarmupSeconds), end-toSamples(context.Plan.CooldownSeconds)
		methods = append(methods, DecouplingTrimWorkoutSteps)
	}
	if end > start {
		start, end = ride.steadyState(start, end)
	}
	methods = append(methods, DecouplingTrimSteadyState)
	result.Trim.Method = strings.Join(methods, "+")
	result.Trim.WarmupSeconds = int(math.Round(float64(start) * ride.step))
	result.Trim.CooldownSeconds = int(math.Round(float64(len(ride.moving)-end) * ride.step))

	return start, end
}

// steadyState drops leading and trailing whole moving minutes whose mean
// output is below the lower quartile of the range's minutes.
func (ride *decouplingRide) steadyState(start, end int) (int, int) {
	perMinute := ride.samplesPerMinute()
	var means []float64
	for from := start; from+perMinute <= end; from += perMinute {
		sum := 0.0
		for _, index := range ride.moving[from : from+perMinute] {
			value, _ := ride.output.At(index)
			sum += value
		}
		means = append(means, sum/float64(perMinute))
	}
	if len(means) < 2*decouplingMinMinutesPerHalf {
		return start, end
	}
	sorted := append([]float64(nil), means...)
	sort.Float64s(sorted)
	q1 := linearQuantile(sorted, quartileLow)
	lead := 0
	for lead < len(means) && means[lead] < q1 {
		lead++
	}
	tail := 0
	for tail < len(means)-lead && means[len(means)-1-tail] < q1 {
		tail++
	}

	return start + lead*perMinute, start + (len(means)-tail)*perMinute
}

// hrLag sets result.HRLagSeconds from the override or cross-correlation.
func (ride *decouplingRide) hrLag(start, end int, options DecouplingOptions, result *StreamDecoupling) {
	if options.HRLagSeconds != nil {
		result.HRLagSeconds, result.HRLagMethod = *options.HRLagSeconds, DecouplingLagManual

		return
	}
	result.HRLagMethod = DecouplingLagNone
	best, bestLag := 0.0, 0
	for lag := 0; lag <= decouplingMaxHRLagSeconds; lag++ {
		correlation, ok := ride.correlation(start, end, lag)
		if ok && correlation > best {
			best, bestLag = correlation, lag
		}
	}
	if best > 0 {
		rounded := round4(best)
		result.HRLagSeconds, result.HRLagMethod, result.HRLagCorrelation = bestLag, DecouplingLagCorrelation, &rounded
	}
}

func (ride *decouplingRide) correlation(start, end, lag int) (float64, bool) {
	var n, sumX, sumY, sumXX, sumYY, sumXY float64
	for _, index := range ride.moving[start:end] {
		x, okX := ride.output.At(index)
		y, okY := ride.hrAt[int(ride.elapsed[index])+lag]
		if !okX || !okY {
			continue
		}
		n++
		sumX += x
		sumY += y
		sumXX += x * x
		sumYY += y * y
		sumXY += x * y
	}
	if n < 2 {
		return 0, false
	}
	cov := sumXY - sumX*sumY/n
	varX := sumXX - sumX*sumX/n
	varY := sumYY - sumY*sumY/n
	if varX <= 0 || varY <= 0 {
		return 0, false
	}

	return cov / math.Sqrt(varX*varY), true
}

// minutes buckets moving samples [start, end) into moving minutes; a final
// partial minute shorter than half a minute is dropped.
func (ride *decouplingRide) minutes(start, end, lag int) []decouplingMinute {
	perMinute := ride.samplesPerMinute()
	var minutes []decouplingMinute
	for from, minute := start, 0; from < end; from, minute = from+perMinute, minute+1 {
		to := minInt(from+perMinute, end)
		samples := to - from
		if float64(samples) < decouplingHalf*float64(perMinute) {
			break
		}
		var output, hr float64
		var outputN, hrN, pedalling int
		for _, index := range ride.moving[from:to] {
			if value, ok := ride.output.At(index); ok {
				output += value
				outputN++
				if value > 0 {
					pedalling++
				}
			}
			if value, ok := ride.hrAt[int(ride.elapsed[index])+lag]; ok {
				hr += value
				hrN++
			}
		}
		entry := decouplingMinute{index: minute}
		if outputN > 0 {
			entry.output = output / float64(outputN)
		}
		if hrN > 0 {
			entry.hr = hr / float64(hrN)
		}
		switch {
		case float64(hrN) < decouplingHalf*float64(samples):
			entry.reason = DecouplingExcludedMissingHR
		case ride.power && float64(pedalling) < decouplingHalf*float64(samples):
			entry.reason = DecouplingExcludedCoasting
		}
		minutes = append(minutes, entry)
	}

	return minutes
}

// decouplingRatioFences computes Tukey fences on the per-minute output/HR
// ratio of the minutes not already excluded (after the coasting and
// missing-HR rules, so stops cannot widen the fences).
func decouplingRatioFences(minutes []decouplingMinute, k float64) (float64, float64) {
	var ratios []float64
	for _, minute := range minutes {
		if minute.reason == "" && minute.hr > 0 {
			ratios = append(ratios, minute.output/minute.hr)
		}
	}
	if len(ratios) < 4 || k <= 0 {
		return math.Inf(-1), math.Inf(1)
	}
	sort.Float64s(ratios)
	q1, q3 := linearQuantile(ratios, quartileLow), linearQuantile(ratios, quartileHigh)
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

// PlannedDecouplingTrim reads warm-up/cool-down from a workout doc: leading
// and trailing steps that are ramps, labeled warm-up/cool-down, or targeted
// below the workout's duration-weighted median target.
func PlannedDecouplingTrim(doc *WorkoutDoc) (*DecouplingPlanTrim, bool) {
	steps := ExpandWorkoutSteps(doc)
	if len(steps) == 0 {
		return nil, false
	}
	median := plannedMedianTarget(steps)
	easy := func(step *PlannedWorkoutStep, label string) bool {
		target := targetReference(step.Power)
		return step.Ramp || step.Kind == label || (target > 0 && target < median)
	}
	trim := &DecouplingPlanTrim{}
	lead := 0
	for lead < len(steps) && easy(&steps[lead], plannedStepKindWarmup) {
		trim.WarmupSeconds += steps[lead].DurationSeconds
		lead++
	}
	for tail := len(steps) - 1; tail >= lead && easy(&steps[tail], plannedStepKindCooldown); tail-- {
		trim.CooldownSeconds += steps[tail].DurationSeconds
	}
	if lead == len(steps) {
		return nil, false
	}

	return trim, true
}

func plannedMedianTarget(steps []PlannedWorkoutStep) float64 {
	type weighted struct{ target, seconds float64 }
	var values []weighted
	total := 0.0
	for index := range steps {
		if target := targetReference(steps[index].Power); target > 0 && steps[index].DurationSeconds > 0 {
			values = append(values, weighted{target, float64(steps[index].DurationSeconds)})
			total += float64(steps[index].DurationSeconds)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].target < values[j].target })
	cumulative := 0.0
	for _, value := range values {
		cumulative += value.seconds
		if cumulative >= total/2 {
			return value.target
		}
	}

	return 0
}

// DecouplingVICutoff is the high Tukey fence (Q3 + 1.5*IQR) of the
// athlete's own ride variability indexes: above it a ride is too unsteady
// for decoupling. Needs at least four rides.
func DecouplingVICutoff(values []float64) *float64 {
	var positive []float64
	for _, value := range values {
		if value > 0 {
			positive = append(positive, value)
		}
	}
	if len(positive) < 4 {
		return nil
	}
	sort.Float64s(positive)
	q1, q3 := linearQuantile(positive, quartileLow), linearQuantile(positive, quartileHigh)
	cutoff := round4(q3 + DefaultDecouplingIQRK*(q3-q1))

	return &cutoff
}

// ApplyVIGate marks a measured result not measurable when the ride's VI
// exceeds the athlete-derived cutoff.
func (result *StreamDecoupling) ApplyVIGate(vi float64, cutoff *float64) {
	result.VariabilityIndex = vi
	result.VICutoff = cutoff
	if cutoff != nil && vi > *cutoff && result.Status == DecouplingStatusMeasured {
		result.Status = DecouplingStatusNotMeasurable
		result.Reason = "variability index above the athlete's cutoff"
	}
}

// HasDecoupling reports whether the activity carries a decoupling value.
// A stream-computed value of exactly 0 is a real measurement (no drift), so
// it counts; an API value of 0 means "absent".
func (activity *Activity) HasDecoupling() bool {
	return activity.DecouplingSource == DecouplingSourceStreams || activity.Decoupling != 0
}

// MarkAPIDecoupling labels the API decoupling as the consumed value. An API
// value of 0 is intervals.icu's "absent" and leaves DecouplingAPI nil.
func (activity *Activity) MarkAPIDecoupling(note string) {
	activity.DecouplingSource = DecouplingSourceAPI
	activity.DecouplingNote = note
	if activity.Decoupling != 0 {
		api := activity.Decoupling
		activity.DecouplingAPI = &api
	}
}

// AttachStreamDecoupling records the stream result next to the API value
// and, when consume is set and the result is measured, makes it the
// consumed Decoupling.
func (activity *Activity) AttachStreamDecoupling(result *StreamDecoupling, consume bool) {
	activity.MarkAPIDecoupling(result.Reason)
	activity.DecouplingStreamsStatus = result.Status
	activity.DecouplingBasis = result.Basis
	if result.Value == nil {
		return
	}
	value := *result.Value
	activity.DecouplingStreams = &value
	if consume && result.Status == DecouplingStatusMeasured {
		activity.Decoupling = value
		activity.DecouplingSource = DecouplingSourceStreams
	}
}
