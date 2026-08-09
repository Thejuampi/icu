package icu

import "fmt"

// Power sample classification kinds.
const (
	PowerSampleMeasured  = "measured"
	PowerSampleTrueZero  = "true_zero"
	PowerSampleMissing   = "missing"
	PowerSampleAmbiguous = "ambiguous"
)

// Speed threshold (m/s) below which the athlete is treated as stopped.
// Protocol constant for movement detection, not a coaching decision rule.
const (
	powerGapMovingSpeedMS     = 1.0
	powerGapMeterDeathMinMiss = 30
	powerGapMeterDeathMinMove = 30
)

// PowerGapInputs are nullable streams used to classify power blanks.
type PowerGapInputs struct {
	Watts   NullableSeries
	Cadence NullableSeries
	// Balance is left_right_balance (percent left, dual-sided PM). Present only while
	// the power meter is alive and reporting L/R; null after mid-ride PM death.
	Balance  NullableSeries
	Speed    NullableSeries // m/s; optional if distance+time available
	Distance NullableSeries // meters
	Time     NullableSeries // seconds since start
}

// PowerGapSegment is a contiguous run of one classification kind.
type PowerGapSegment struct {
	StartIndex  int    `json:"startIndex"`
	EndIndex    int    `json:"endIndex"`
	Kind        string `json:"kind"`
	Reason      string `json:"reason,omitempty"`
	SampleCount int    `json:"sampleCount"`
}

// Power meter death detection sources (hardware signals preferred over labels).
const (
	PowerDeathSourceBalance    = "left_right_balance"
	PowerDeathSourceCadence    = "cadence"
	PowerDeathSourceMissingRun = "missing_run"
)

// PowerGapClassification summarizes per-sample blank detection.
type PowerGapClassification struct {
	Labels           []string          `json:"-"`
	Reasons          []string          `json:"-"`
	MeasuredSeconds  int               `json:"measuredSeconds"`
	TrueZeroSeconds  int               `json:"trueZeroSeconds"`
	MissingSeconds   int               `json:"missingSeconds"`
	AmbiguousSeconds int               `json:"ambiguousSeconds"`
	Segments         []PowerGapSegment `json:"segments,omitempty"`
	MeterDeathIndex  *int              `json:"meterDeathIndex"`
	// DeathSource is how MeterDeathIndex was derived (balance / cadence / missing_run).
	DeathSource string   `json:"deathSource,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// ClassifyPowerSamples labels each sample as measured, true_zero, missing, or ambiguous.
//
// Rules (PM death and coasting):
//   - watts > 0 → measured (unless post-death balance gap override for zeros/nulls)
//   - watts present 0 + cadence present 0 → true_zero (coasting)
//   - watts present 0 + cadence present > 0 → measured (zero torque while spinning)
//   - watts absent/0 + cadence absent + moving → missing
//   - watts absent/0 + cadence absent + stopped → true_zero
//   - watts null + cadence 0 → true_zero
//   - watts null + cadence > 0 → missing
//
// Dual-sided PM edge cases (left_right_balance):
//   - Balance present only while the meter is alive; first half real power usually has L/R.
//   - After last present balance sample, a long null tail is treated as PM death.
//   - Moving zeros/null watts after balance death → missing (not true_zero from cadence alone).
//   - Positive watts after balance death are left as measured (prior fill); use refill mask to reopen.
func ClassifyPowerSamples(inputs *PowerGapInputs) PowerGapClassification {
	if inputs == nil {
		return PowerGapClassification{Warnings: []string{"no samples to classify"}}
	}
	sampleCount := classificationLength(inputs)
	result := PowerGapClassification{
		Labels:  make([]string, sampleCount),
		Reasons: make([]string, sampleCount),
	}
	if sampleCount == 0 {
		result.Warnings = append(result.Warnings, "no samples to classify")
		return result
	}

	speed := deriveSpeedSeries(inputs, sampleCount)
	hasCadenceStream := inputs.Cadence.Len() > 0
	if !hasCadenceStream {
		result.Warnings = append(result.Warnings, "cadence stream absent; missing/true_zero separation is lower confidence")
	}
	hasBalance := countPresent(inputs.Balance) > 0
	if hasBalance {
		result.Warnings = append(
			result.Warnings,
			"left_right_balance present: dual-sided PM death detection prefers L/R null tail over cadence alone",
		)
	}

	for index := range sampleCount {
		kind, reason := classifyPowerSample(index, inputs, speed, hasCadenceStream)
		result.Labels[index] = kind
		result.Reasons[index] = reason
		tallyPowerSample(&result, kind)
	}

	// Prefer hardware death (L/R balance → cadence), then missing-run heuristic.
	if death, src := DetectPowerMeterDeathIndex(inputs.Balance, inputs.Cadence); death >= 0 {
		result.MeterDeathIndex = &death
		result.DeathSource = src
		applyBalanceDeathGaps(&result, inputs, speed, death, src)
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"power meter death at index %d from %s (first half with sensor data preserved as measured)",
			death, src,
		))
	} else if death := detectMeterDeathIndex(result.Labels, speed); death >= 0 {
		result.MeterDeathIndex = &death
		result.DeathSource = PowerDeathSourceMissingRun
		promoteAmbiguousAfterDeath(&result, speed, death)
	}

	// Recount after post-death overrides.
	result.MeasuredSeconds = 0
	result.TrueZeroSeconds = 0
	result.MissingSeconds = 0
	result.AmbiguousSeconds = 0
	for _, kind := range result.Labels {
		tallyPowerSample(&result, kind)
	}

	result.Segments = buildPowerGapSegments(result.Labels, result.Reasons)

	return result
}

func tallyPowerSample(result *PowerGapClassification, kind string) {
	switch kind {
	case PowerSampleMeasured:
		result.MeasuredSeconds++
	case PowerSampleTrueZero:
		result.TrueZeroSeconds++
	case PowerSampleMissing:
		result.MissingSeconds++
	default:
		result.AmbiguousSeconds++
	}
}

func promoteAmbiguousAfterDeath(result *PowerGapClassification, speed []float64, death int) {
	for index := death; index < len(result.Labels); index++ {
		if result.Labels[index] != PowerSampleAmbiguous {
			continue
		}
		if speed[index] <= powerGapMovingSpeedMS {
			continue
		}
		result.Labels[index] = PowerSampleMissing
		result.Reasons[index] = "post_meter_death_moving"
		result.AmbiguousSeconds--
		result.MissingSeconds++
	}
}

func classificationLength(inputs *PowerGapInputs) int {
	minLen := -1
	for _, series := range []NullableSeries{
		inputs.Watts, inputs.Cadence, inputs.Balance, inputs.Speed, inputs.Distance, inputs.Time,
	} {
		if series.Len() == 0 {
			continue
		}
		if minLen < 0 || series.Len() < minLen {
			minLen = series.Len()
		}
	}
	if minLen < 0 {
		return 0
	}

	return minLen
}

func countPresent(series NullableSeries) int {
	var n int
	for index := range series.Len() {
		if _, ok := series.At(index); ok {
			n++
		}
	}

	return n
}

// applyBalanceDeathGaps marks moving zero/null-watt samples after hardware death
// as missing. Leaves positive watts alone (prior fill / real residual power).
func applyBalanceDeathGaps(
	result *PowerGapClassification,
	inputs *PowerGapInputs,
	speed []float64,
	death int,
	source string,
) {
	if result == nil || inputs == nil || death < 0 {
		return
	}
	reason := "post_meter_death_moving"
	if source == PowerDeathSourceBalance {
		reason = "post_balance_death_moving"
	} else if source == PowerDeathSourceCadence {
		reason = "post_cadence_death_moving"
	}
	for index := death; index < len(result.Labels); index++ {
		if index < len(speed) && speed[index] <= powerGapMovingSpeedMS {
			// Stopped: keep true_zero; don't invent missing while coasting still.
			continue
		}
		// Prior accepted fill: positive watts stay measured until explicit refill mask.
		if w, ok := inputs.Watts.At(index); ok && w > 0 {
			continue
		}
		if result.Labels[index] == PowerSampleMissing {
			result.Reasons[index] = reason
			continue
		}
		if result.Labels[index] == PowerSampleMeasured {
			continue
		}
		result.Labels[index] = PowerSampleMissing
		result.Reasons[index] = reason
	}
	promoteAmbiguousAfterDeath(result, speed, death)
}

func deriveSpeedSeries(inputs *PowerGapInputs, sampleCount int) []float64 {
	speed := make([]float64, sampleCount)
	copySpeedStream(speed, inputs.Speed, sampleCount)
	fillSpeedFromDistance(speed, inputs, sampleCount)

	return speed
}

func copySpeedStream(speed []float64, series NullableSeries, sampleCount int) {
	if series.Len() == 0 {
		return
	}
	for index := range sampleCount {
		if value, ok := series.At(index); ok && value > 0 {
			speed[index] = value
		}
	}
}

func fillSpeedFromDistance(speed []float64, inputs *PowerGapInputs, sampleCount int) {
	if inputs.Distance.Len() == 0 || inputs.Time.Len() == 0 {
		return
	}
	for index := 1; index < sampleCount; index++ {
		if speed[index] > 0 {
			continue
		}
		d0, ok0 := inputs.Distance.At(index - 1)
		d1, ok1 := inputs.Distance.At(index)
		t0, okT0 := inputs.Time.At(index - 1)
		t1, okT1 := inputs.Time.At(index)
		if !ok0 || !ok1 || !okT0 || !okT1 {
			continue
		}
		dt := t1 - t0
		if dt <= 0 {
			continue
		}
		if dd := d1 - d0; dd >= 0 {
			speed[index] = dd / dt
		}
	}
}

//nolint:gocritic // kind/reason pair is the natural return shape for classification
func classifyPowerSample(index int, inputs *PowerGapInputs, speed []float64, hasCadence bool) (string, string) {
	moving := speed[index] > powerGapMovingSpeedMS
	watts, wattsOK := inputs.Watts.At(index)
	cadence, cadenceOK := inputs.Cadence.At(index)

	if wattsOK && watts > 0 {
		return PowerSampleMeasured, "positive_watts"
	}
	if kind, reason, ok := classifyZeroWattsWithCadence(wattsOK, watts, cadenceOK, cadence); ok {
		return kind, reason
	}
	if kind, reason, ok := classifyNullWattsWithCadence(wattsOK, cadenceOK, cadence); ok {
		return kind, reason
	}
	if kind, reason, ok := classifyNoCadence(wattsOK, watts, cadenceOK, moving); ok {
		return kind, reason
	}
	if !wattsOK && hasCadence && cadenceOK && cadence > 0 {
		return PowerSampleMissing, "no_watts_positive_cadence"
	}
	if !moving {
		return PowerSampleTrueZero, "stopped_default"
	}

	return PowerSampleAmbiguous, "unclassified_moving"
}

//nolint:gocritic // optional classification triple
func classifyZeroWattsWithCadence(wattsOK bool, watts float64, cadenceOK bool, cadence float64) (string, string, bool) {
	if !wattsOK || watts != 0 || !cadenceOK {
		return "", "", false
	}
	if cadence == 0 {
		return PowerSampleTrueZero, "zero_watts_zero_cadence", true
	}

	return PowerSampleMeasured, "zero_watts_positive_cadence", true
}

//nolint:gocritic // optional classification triple
func classifyNullWattsWithCadence(wattsOK, cadenceOK bool, cadence float64) (string, string, bool) {
	if wattsOK || !cadenceOK {
		return "", "", false
	}
	if cadence == 0 {
		return PowerSampleTrueZero, "null_watts_zero_cadence", true
	}

	return PowerSampleMissing, "null_watts_positive_cadence", true
}

//nolint:gocritic // optional classification triple
func classifyNoCadence(wattsOK bool, watts float64, cadenceOK, moving bool) (string, string, bool) {
	if cadenceOK {
		return "", "", false
	}
	if wattsOK && watts != 0 {
		return "", "", false
	}
	if moving {
		return PowerSampleMissing, "no_cadence_moving", true
	}

	return PowerSampleTrueZero, "no_cadence_stopped", true
}

func detectMeterDeathIndex(labels []string, speed []float64) int {
	lastMeasured := -1
	for index, label := range labels {
		if label == PowerSampleMeasured {
			lastMeasured = index
		}
	}
	if lastMeasured < 0 || lastMeasured >= len(labels)-1 {
		return -1
	}

	missingAfter := 0
	movingAfter := 0
	for index := lastMeasured + 1; index < len(labels); index++ {
		switch labels[index] {
		case PowerSampleMissing:
			missingAfter++
		case PowerSampleTrueZero, PowerSampleAmbiguous:
			// counted for movement only
		default:
			continue
		}
		if speed[index] > powerGapMovingSpeedMS {
			movingAfter++
		}
	}
	if missingAfter >= powerGapMeterDeathMinMiss && movingAfter >= powerGapMeterDeathMinMove {
		return lastMeasured + 1
	}

	return -1
}

// DetectBalanceDeathIndex finds the first sample after the last present
// left_right_balance value when a long null tail remains. Dual-sided PMs report
// L/R only while alive; coasting may null briefly, but last-present + long tail
// is mid-ride battery/sensor death. Returns -1 when balance is unused/absent.
func DetectBalanceDeathIndex(balance NullableSeries) int {
	n := balance.Len()
	if n == 0 {
		return -1
	}
	lastPresent := -1
	presentCount := 0
	for index := range n {
		if _, ok := balance.At(index); ok {
			lastPresent = index
			presentCount++
		}
	}
	// Need a real dual-sided segment before death (not a few glitch samples).
	if presentCount < powerGapMeterDeathMinMiss || lastPresent < 0 {
		return -1
	}
	remaining := n - lastPresent - 1
	if remaining < powerGapMeterDeathMinMiss {
		return -1
	}

	return lastPresent + 1
}

// DetectCadenceDeathIndex finds death via the longest null-cadence run that
// reaches the end of the series (not a mid-ride freewheel). Cadence can null on
// coasting, so only end-anchored runs count.
func DetectCadenceDeathIndex(cadence NullableSeries) int {
	n := cadence.Len()
	if n == 0 {
		return -1
	}
	// Walk backward: death tail is contiguous nulls at the end.
	end := n - 1
	for end >= 0 {
		if _, ok := cadence.At(end); ok {
			break
		}
		end--
	}
	// end is last present cadence; death starts at end+1.
	death := end + 1
	if death >= n {
		return -1
	}
	tailLen := n - death
	if tailLen < powerGapMeterDeathMinMiss {
		return -1
	}
	// Require some measured cadence earlier so this isn't "never had cadence".
	if end < powerGapMeterDeathMinMiss-1 {
		return -1
	}

	return death
}

// DetectPowerMeterDeathIndex prefers L/R balance (dual-sided PM hardware), then
// end-anchored cadence null tail. Returns (index, source) or (-1, "").
func DetectPowerMeterDeathIndex(balance, cadence NullableSeries) (int, string) {
	if death := DetectBalanceDeathIndex(balance); death >= 0 {
		return death, PowerDeathSourceBalance
	}
	if death := DetectCadenceDeathIndex(cadence); death >= 0 {
		return death, PowerDeathSourceCadence
	}

	return -1, ""
}

// MaskStreamsAsPowerMeterDeathFrom clones streams and forces a mid-ride PM death
// pattern from fromIndex: watts present 0, cadence/balance absent. Used to re-fill
// a region that was previously estimated/accepted (otherwise it looks "measured").
func MaskStreamsAsPowerMeterDeathFrom(streams NullableStreamData, fromIndex int) NullableStreamData {
	if streams == nil || fromIndex < 0 {
		return streams
	}
	out := make(NullableStreamData, len(streams))
	for key, series := range streams {
		length := series.Len()
		values := make([]float64, length)
		present := make([]bool, length)
		copy(values, series.Values)
		copy(present, series.Present)
		switch key {
		case "watts", "power":
			for i := fromIndex; i < length; i++ {
				values[i] = 0
				present[i] = true // present zero + null cadence → missing when moving
			}
		case "cadence", "left_right_balance":
			for i := fromIndex; i < length; i++ {
				values[i] = 0
				present[i] = false
			}
		}
		out[key] = NullableSeries{Values: values, Present: present}
	}

	return out
}

func buildPowerGapSegments(labels, reasons []string) []PowerGapSegment {
	if len(labels) == 0 {
		return nil
	}
	var segments []PowerGapSegment
	start := 0
	for index := 1; index <= len(labels); index++ {
		if index < len(labels) && labels[index] == labels[start] && reasons[index] == reasons[start] {
			continue
		}
		segments = append(segments, PowerGapSegment{
			StartIndex:  start,
			EndIndex:    index - 1,
			Kind:        labels[start],
			Reason:      reasons[start],
			SampleCount: index - start,
		})
		start = index
	}

	return segments
}
