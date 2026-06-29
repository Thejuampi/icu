package icu

import (
	"math"
	"sort"
	"strconv"
)

const (
	rebalanceEnvelopeFenceMAD  = 3.0
	rebalanceEnvelopeMetrics10 = 10
)

type RebalanceEnvelopeReport struct {
	Envelope        RebalanceEnvelope `json:"envelope"`
	CurrentLoad     float64           `json:"currentLoad"`
	OutsideEnvelope bool              `json:"outsideEnvelope"`
	Confidence      float64           `json:"confidence"`
	Completeness    float64           `json:"completeness"`
	LowSource       string            `json:"lowSource,omitempty"`
	HighSource      string            `json:"highSource,omitempty"`
	Sources         []string          `json:"sources,omitempty"`
}

// BuildRebalanceEnvelope derives the data-backed low/current/high weekly-load
// envelope from the vigent regime, the current calendar load, and up to 10
// continuous context metrics from the full RebalanceInput. Each metric produces
// a modifier in [0.5, 1.5] and the average drives dynamic fence scaling:
//
//	highScalar = median + 3.0 * MAD * highModifier
//	lowScalar = max(0, median - 3.0 * MAD * lowModifier)
//	lowModifier = clamp(2.0 - highModifier, 0.5, 1.5)
//
// Missing metrics contribute a neutral 1.0 and reduce the available count,
// lowering completeness and confidence but never blocking the envelope.
func BuildRebalanceEnvelope(regime *RebalanceRegime, input *RebalanceInput, currentLoad float64) (RebalanceEnvelopeReport, bool) {
	if len(regime.Samples) == 0 {
		return RebalanceEnvelopeReport{}, false
	}
	median := medianFloat64(regime.Samples)
	mad := madFloat64(regime.Samples, median)

	highModifier, metricsAvailable := rebalanceComputeModifiers(regime, input, currentLoad)
	lowModifier := clampFloat(2.0-highModifier, 0.5, 1.5)

	highScalar := math.Round(median + rebalanceEnvelopeFenceMAD*mad*highModifier)
	lowScalar := math.Round(math.Max(0, median-rebalanceEnvelopeFenceMAD*mad*lowModifier))

	highSource := "data_robust_fence_dynamic_10_metrics"
	baseHigh := math.Round(median + rebalanceEnvelopeFenceMAD*mad)
	if highModifier < 1.0 && highScalar < baseHigh {
		highSource = "data_robust_fence_contracted_by_context"
	}
	if highModifier > 1.0 && highScalar > baseHigh {
		highSource = "data_robust_fence_expanded_by_context"
	}

	low := NewRebalanceRatFromInt(int(lowScalar), 1)
	current := NewRebalanceRatFromInt(int(currentLoad), 1)
	high := NewRebalanceRatFromInt(int(highScalar), 1)
	if low.Cmp(current) > 0 {
		low = current.clone()
	}
	if high.Cmp(current) < 0 {
		high = current.clone()
	}

	outside := int(currentLoad) < int(lowScalar) || int(currentLoad) > int(highScalar)
	completeness := 1.0
	if rebalanceEnvelopeMetrics10 > 0 {
		completeness = clampFloat01(float64(metricsAvailable) / float64(rebalanceEnvelopeMetrics10))
	}
	confidence := clampFloat01(regime.Confidence * completeness)

	return RebalanceEnvelopeReport{
		Envelope: RebalanceEnvelope{
			Low:     low,
			Current: current,
			High:    high,
		},
		CurrentLoad:     currentLoad,
		OutsideEnvelope: outside,
		Confidence:      round3(confidence),
		Completeness:    round3(completeness),
		LowSource:       "data_robust_fence_dynamic_10_metrics",
		HighSource:      highSource,
		Sources:         envelopeSources(metricsAvailable, rebalanceEnvelopeMetrics10),
	}, true
}

// rebalanceComputeModifiers evaluates up to 10 context metrics from the input
// and returns the average modifier and the count of metrics that had data.
// Each metric's contribution is clamped to [0.5, 1.5] and missing metrics
// default to 1.0 without incrementing the available count.
func rebalanceComputeModifiers(regime *RebalanceRegime, input *RebalanceInput, currentLoad float64) (avg float64, available int) {
	var sum float64
	available = 0

	type metricFn func() (modifier float64, hasData bool)
	metrics := []metricFn{
		func() (float64, bool) { return rebalanceMetricLoad(regime, currentLoad), true }, // Load always computable with regime
		func() (float64, bool) { return rebalanceMetricDurationHasData(input) },
		func() (float64, bool) { return rebalanceMetricIntensityHasData(input) },
		func() (float64, bool) { return rebalanceMetricZoneDistributionHasData(input) },
		func() (float64, bool) { return rebalanceMetricComplianceHasData(input) },
		func() (float64, bool) { return rebalanceMetricRPEHasData(input) },
		func() (float64, bool) { return rebalanceMetricHRVHasData(input) },
		func() (float64, bool) { return rebalanceMetricSleepHasData(input) },
		func() (float64, bool) { return rebalanceMetricRestingHRHasData(input) },
		func() (float64, bool) { return rebalanceMetricAdaptationHasData(input) },
	}

	for _, fn := range metrics {
		m, hasData := fn()
		sum += m
		if hasData {
			available++
		}
	}

	if available == 0 {
		return 1.0, 0
	}
	modifier := sum / float64(rebalanceEnvelopeMetrics10)
	return clampFloat(modifier, 0.5, 1.5), available
}

// hasData helpers for each metric

func rebalanceMetricDurationHasData(input *RebalanceInput) (float64, bool) {
	if input == nil || len(input.Activities) == 0 {
		return 1.0, false
	}
	for i := range input.Activities {
		if input.Activities[i].MovingTime > 0 {
			return rebalanceMetricDuration(input), true
		}
	}
	return 1.0, false
}

func rebalanceMetricIntensityHasData(input *RebalanceInput) (float64, bool) {
	if input == nil {
		return 1.0, false
	}
	for i := range input.Activities {
		if input.Activities[i].Intensity > 0 && IsCyclingActivityType(input.Activities[i].Type) {
			return rebalanceMetricIntensity(input), true
		}
	}
	return 1.0, false
}

func rebalanceMetricZoneDistributionHasData(input *RebalanceInput) (float64, bool) {
	if input == nil || rebalanceFTP(input) == 0 {
		return 1.0, false
	}
	for i := range input.Activities {
		if IsCyclingActivityType(input.Activities[i].Type) && input.Activities[i].Intensity > 0 && input.Activities[i].MovingTime > 0 {
			return rebalanceMetricZoneDistribution(input), true
		}
	}
	return 1.0, false
}

func rebalanceMetricComplianceHasData(input *RebalanceInput) (float64, bool) {
	if input == nil || input.Plan == nil || input.Plan.History.PlannedLoadAlignment == "" {
		return 1.0, false
	}
	return rebalanceMetricCompliance(input), true
}

func rebalanceMetricRPEHasData(input *RebalanceInput) (float64, bool) {
	if input == nil || len(input.Activities) == 0 {
		return 1.0, false
	}
	for i := range input.Activities {
		if input.Activities[i].RPE > 0 && IsCyclingActivityType(input.Activities[i].Type) {
			return rebalanceMetricRPE(input), true
		}
	}
	return 1.0, false
}

func rebalanceMetricHRVHasData(input *RebalanceInput) (float64, bool) {
	if input == nil || input.Wellness == nil || input.Wellness.Scope.Records == 0 {
		return 1.0, false
	}
	return rebalanceMetricHRV(input), true
}

func rebalanceMetricSleepHasData(input *RebalanceInput) (float64, bool) {
	if input == nil || input.Wellness == nil || input.Wellness.Scope.Records == 0 {
		return 1.0, false
	}
	return rebalanceMetricSleep(input), true
}

func rebalanceMetricRestingHRHasData(input *RebalanceInput) (float64, bool) {
	if input == nil || input.Wellness == nil || input.Wellness.Scope.Records == 0 {
		return 1.0, false
	}
	return rebalanceMetricRestingHR(input), true
}

func rebalanceMetricAdaptationHasData(input *RebalanceInput) (float64, bool) {
	if input == nil || input.Plan == nil {
		return 1.0, false
	}
	return rebalanceMetricAdaptation(input), true
}

// rebalanceMetricLoad evaluates the load metric modifier.
// Current load Z-score relative to regime median/MAD; high load contracts.
func rebalanceMetricLoad(regime *RebalanceRegime, currentLoad float64) float64 {
	if mad := regime.MAD; mad > 0 && regime.Median > 0 {
		z := rebalanceMadScale * (currentLoad - regime.Median) / mad
		m := 1.0 - clampFloat(z/3.0, -0.5, 0.5)
		return clampFloat(m, 0.5, 1.5)
	}
	return 1.0
}

// rebalanceMetricDuration evaluates the duration metric modifier.
// Uses RebalanceWeeklyDurationSeries to compute a Z-score-like modifier.
func rebalanceMetricDuration(input *RebalanceInput) float64 {
	if input == nil || len(input.Activities) == 0 {
		return 1.0
	}
	durations := RebalanceWeeklyDurationSeries(input.Activities, input.Scope.StartDate)
	if len(durations) < 3 {
		return 1.0
	}
	median := medianFloat64(durations)
	mad := madFloat64(durations, median)
	if mad <= 0 || median <= 0 {
		return 1.0
	}
	// Compute current week's moving time
	var currentWeekSecs float64
	for index := range input.Activities {
		date := activityDate(&input.Activities[index])
		if date >= input.Scope.StartDate && date <= input.Scope.EndDate {
			currentWeekSecs += float64(input.Activities[index].MovingTime)
		}
	}
	z := rebalanceMadScale * (currentWeekSecs - median) / mad
	m := 1.0 - clampFloat(z/3.0, -0.5, 0.5)
	return clampFloat(m, 0.5, 1.5)
}

// rebalanceMetricIntensity evaluates the intensity metric modifier.
// Higher activity IF relative to planned event IF contracts the envelope.
func rebalanceMetricIntensity(input *RebalanceInput) float64 {
	if input == nil {
		return 1.0
	}
	var activityIFs []float64
	for index := range input.Activities {
		if input.Activities[index].Intensity > 0 && IsCyclingActivityType(input.Activities[index].Type) {
			activityIFs = append(activityIFs, input.Activities[index].Intensity)
		}
	}
	var eventIFs []float64
	for index := range input.Events {
		if input.Events[index].Intensity > 0 && isWorkoutEvent(&input.Events[index]) {
			eventIFs = append(eventIFs, input.Events[index].Intensity)
		}
	}
	if len(activityIFs) == 0 || len(eventIFs) == 0 {
		return 1.0
	}
	meanActivityIF := meanFloat64(activityIFs)
	meanEventIF := meanFloat64(eventIFs)
	if meanEventIF <= 0 {
		return 1.0
	}
	ratio := meanActivityIF / meanEventIF
	// ratio > 1 means activities are harder than planned → stress → < 1
	m := 1.0 - clampFloat((ratio-1.0)/0.3, -0.5, 0.5)
	return clampFloat(m, 0.5, 1.5)
}

// rebalanceMetricZoneDistribution evaluates the zone distribution modifier.
// Higher proportion of time above Z2 (>75% FTP) contracts the envelope.
func rebalanceMetricZoneDistribution(input *RebalanceInput) float64 {
	if input == nil || rebalanceFTP(input) == 0 {
		return 1.0
	}
	var totalSecs float64
	var aboveZ2Secs float64
	for index := range input.Activities {
		activity := &input.Activities[index]
		if !IsCyclingActivityType(activity.Type) || activity.Intensity <= 0 || activity.MovingTime <= 0 {
			continue
		}
		totalSecs += float64(activity.MovingTime)
		if activity.Intensity > 0.75 {
			aboveZ2Secs += float64(activity.MovingTime)
		}
	}
	if totalSecs == 0 {
		return 1.0
	}
	proportion := aboveZ2Secs / totalSecs
	// proportion 0 → 1.5 (no high-intensity), proportion 1 → 0.5 (all high-intensity)
	m := 1.5 - proportion
	return clampFloat(m, 0.5, 1.5)
}

// rebalanceMetricCompliance evaluates the compliance metric modifier.
// PlannedLoadAlignment states map to continuous modifier weights.
func rebalanceMetricCompliance(input *RebalanceInput) float64 {
	if input == nil || input.Plan == nil || input.Plan.History.PlannedLoadAlignment == "" {
		return 1.0
	}
	switch input.Plan.History.PlannedLoadAlignment {
	case planAlignmentAbovePeak:
		return 0.7
	case planAlignmentBelow:
		return 0.8
	case planAlignmentAboveAverage:
		return 0.9
	case planAlignmentWithin:
		return 1.0
	default:
		return 1.0
	}
}

// rebalanceMetricRPE evaluates the RPE metric modifier.
// Higher mean RPE contracts the envelope.
func rebalanceMetricRPE(input *RebalanceInput) float64 {
	if input == nil || len(input.Activities) == 0 {
		return 1.0
	}
	var sum float64
	var count int
	for index := range input.Activities {
		if input.Activities[index].RPE > 0 && IsCyclingActivityType(input.Activities[index].Type) {
			sum += float64(input.Activities[index].RPE)
			count++
		}
	}
	if count == 0 {
		return 1.0
	}
	meanRPE := sum / float64(count)
	// Scale: RPE 5 → 1.0 (neutral), RPE 10 → 0.5, RPE 0 → 1.5
	m := 1.0 - clampFloat((meanRPE-5.0)/5.0, -0.5, 0.5)
	return clampFloat(m, 0.5, 1.5)
}

// rebalanceMetricHRV evaluates the HRV metric modifier.
// Positive HRV z-score (recovered) expands; negative (stressed) contracts.
func rebalanceMetricHRV(input *RebalanceInput) float64 {
	if input == nil || input.Wellness == nil || input.Wellness.Scope.Records == 0 {
		return 1.0
	}
	z := input.Wellness.HRV.ZScore
	// Positive Z → recovered → expand (m > 1)
	m := 1.0 + clampFloat(z/3.0, -0.5, 0.5)
	return clampFloat(m, 0.5, 1.5)
}

// rebalanceMetricSleep evaluates the sleep metric modifier.
// Higher sleep score ratio expands the envelope.
func rebalanceMetricSleep(input *RebalanceInput) float64 {
	if input == nil || input.Wellness == nil || input.Wellness.Scope.Records == 0 {
		return 1.0
	}
	ratio := input.Wellness.Sleep.Ratio
	// ratio 0.5 → 1.0, ratio 1.0 → 1.5, ratio 0.0 → 0.5
	m := 0.5 + clampFloat(ratio, 0.0, 1.0)
	return clampFloat(m, 0.5, 1.5)
}

// rebalanceMetricRestingHR evaluates the resting HR metric modifier.
// Positive delta (elevated RHR) contracts the envelope.
func rebalanceMetricRestingHR(input *RebalanceInput) float64 {
	if input == nil || input.Wellness == nil || input.Wellness.Scope.Records == 0 {
		return 1.0
	}
	delta := input.Wellness.RestingHR.Delta
	// delta near 0 → 1.0, delta 10+ bpm → 0.5, delta -10 → 1.5
	m := 1.0 - clampFloat(delta/10.0, -0.5, 0.5)
	return clampFloat(m, 0.5, 1.5)
}

// rebalanceMetricAdaptation evaluates the adaptation metric modifier.
// Higher ADEScore (more readiness) expands the envelope.
func rebalanceMetricAdaptation(input *RebalanceInput) float64 {
	if input == nil || input.Plan == nil {
		return 1.0
	}
	score := input.Plan.Decision.ADEScore
	// score 0 → 0.5, score 50 → 1.0, score 100 → 1.5
	m := 0.5 + (float64(score)/100.0)*1.0
	return clampFloat(m, 0.5, 1.5)
}

// RebalanceWeeklyDurationSeries aggregates cycling activities before the scope
// week into per-week moving-time totals, ordered oldest to newest.
func RebalanceWeeklyDurationSeries(activities []Activity, scopeStart string) []float64 {
	durations := map[string]float64{}
	scopeWeek := isoWeekKeyFromDate(scopeStart)
	for index := range activities {
		activity := &activities[index]
		if !IsCyclingActivityType(activity.Type) || activity.MovingTime <= 0 {
			continue
		}
		key := isoWeekKeyFromDate(activityDate(activity))
		if key == "" || key >= scopeWeek {
			continue
		}
		durations[key] += float64(activity.MovingTime)
	}
	keys := make([]string, 0, len(durations))
	for key := range durations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]float64, 0, len(keys))
	for _, key := range keys {
		out = append(out, durations[key])
	}
	return out
}

func envelopeSources(available, total int) []string {
	if total <= 0 {
		return nil
	}
	return []string{pluralizeMetricCount(available, total)}
}

func pluralizeMetricCount(available, total int) string {
	return "metrics_available:" + strconv.Itoa(available) + "/" + strconv.Itoa(total)
}

func clampFloat(value, minVal, maxVal float64) float64 {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

func meanFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
