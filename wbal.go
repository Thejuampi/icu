package icu

import "math"

const (
	wbalReliableCPRatio    = 0.85
	wbalStreamStep         = 1.0
	flywheelMinPowerRatio  = 1.2
	flywheelMinDuration    = 3
	flywheelCadenceDropPct = 50
	flywheelLowCadenceRPM  = 30
	flywheelMaxHRRise      = 15
)

// FlywheelArtifact describes a segment of sustained high power that is
// produced by a smart-trainer flywheel coast-down rather than real
// pedalling. The signature is: power above CP while cadence drops
// sharply and heart rate fails to respond.
type FlywheelArtifact struct {
	StartIndex   int     `json:"startIndex"`
	EndIndex     int     `json:"endIndex"`
	StartTimeSec int     `json:"startTimeSec"`
	DurationSec  int     `json:"durationSec"`
	PeakWatts    float64 `json:"peakWatts"`
	StartCadence float64 `json:"startCadence,omitempty"`
	EndCadence   float64 `json:"endCadence,omitempty"`
	StartHR      float64 `json:"startHr,omitempty"`
	EndHR        float64 `json:"endHr,omitempty"`
}

// ArtifactDetectionResult holds detected flywheel artifacts and the
// count of supra-CP seconds that were removed from the power stream.
type ArtifactDetectionResult struct {
	Artifacts      []FlywheelArtifact `json:"artifacts"`
	SupraCPSeconds int                `json:"supraCPSeconds"`
	CleanedSeconds int                `json:"cleanedSeconds"`
	HasCadence     bool               `json:"hasCadence"`
	HasHR          bool               `json:"hasHr"`
}

// DetectFlywheelArtifacts scans a power stream for flywheel coast-down
// artifacts. An artifact is a sustained segment where power exceeds CP
// (or 120% of CP as threshold), cadence drops by more than 50% from the
// segment start or falls below 30 rpm, and heart rate does not rise by
// more than 15 bpm during the segment (when HR data is available).
//
// Short supra-CP bursts of fewer than 3 seconds are treated as power
// meter spikes and are not classified as flywheel artifacts.
func DetectFlywheelArtifacts(watts, cadence, hr []float64, cp int) ArtifactDetectionResult {
	result := ArtifactDetectionResult{
		HasCadence: len(cadence) > 0,
		HasHR:      len(hr) > 0,
	}

	if len(watts) == 0 || cp <= 0 {
		return result
	}

	threshold := float64(cp) * flywheelMinPowerRatio
	minLen := streamMinLength(watts, cadence, hr, result.HasCadence, result.HasHR)

	result.SupraCPSeconds = countSupraCP(watts[:minLen], float64(cp))
	result.Artifacts = detectArtifactSegments(watts, cadence, hr, threshold, minLen, result.HasCadence, result.HasHR)

	for _, a := range result.Artifacts {
		result.CleanedSeconds += a.DurationSec
	}

	return result
}

// CleanPowerStream returns a copy of the power stream with flywheel
// artifact segments zeroed out.
func CleanPowerStream(watts []float64, artifacts []FlywheelArtifact) []float64 {
	if len(watts) == 0 {
		return nil
	}

	cleaned := make([]float64, len(watts))
	copy(cleaned, watts)

	for _, a := range artifacts {
		for i := a.StartIndex; i <= a.EndIndex && i < len(cleaned); i++ {
			cleaned[i] = 0
		}
	}

	return cleaned
}

// ComputeWBalDepletion calculates the maximum W'bal depletion from a
// second-by-second power stream using the Skiba recovery model.
//
// When power exceeds CP, W' is consumed linearly. When power drops
// below CP, W' recovers asymptotically towards W' with a time constant
// of tau = W' / (CP - P).
//
// Returns the maximum depletion in joules and as a percentage of W'.
// Returns zeros when the inputs are insufficient for a valid
// computation.
func ComputeWBalDepletion(watts []float64, cp, wprime int) (int, float64) { //nolint:gocritic // unnamed results are clearer here
	if len(watts) == 0 || cp <= 0 || wprime <= 0 {
		return 0, 0
	}

	cpF := float64(cp)
	wprimeF := float64(wprime)

	wbal := wprimeF
	minWbal := wprimeF

	for i := range watts {
		p := watts[i]
		switch {
		case p > cpF:
			wbal -= (p - cpF) * wbalStreamStep
		case p < cpF:
			tau := wprimeF / (cpF - p)
			wbal += (wprimeF - wbal) * (1 - math.Exp(-wbalStreamStep/tau))
		}
		if wbal < minWbal {
			minWbal = wbal
		}
		if wbal < 0 {
			wbal = 0
		}
	}

	depletion := wprimeF - minWbal
	if depletion < 0 {
		depletion = 0
	}

	return int(math.Round(depletion)), round2(depletion * percentScale / wprimeF)
}

// ActivityModelReliable reports whether the per-activity power model
// (icu_pm_cp, icu_pm_w_prime) can be trusted for W'bal depletion.
//
// Intervals.icu estimates a CP/W'/PMax per activity from that session's
// power curve. For low-intensity rides with no supra-CP efforts, the
// estimate collapses to absurdly low CP values, which in turn inflates
// W'bal depletion. We treat the model as reliable only when the
// activity CP is at least 85% of the athlete FTP, which matches the
// physiological expectation that CP is approximately equal to FTP.
func ActivityModelReliable(activity *Activity) bool {
	if activity == nil {
		return false
	}
	if activity.CriticalPower > 0 && activity.FTP > 0 {
		return float64(activity.CriticalPower) >= float64(activity.FTP)*wbalReliableCPRatio
	}
	return true
}

// RecomputeWBalDepletion combines artifact detection, stream cleaning,
// and W'bal recalculation into a single operation. It uses the provided
// power model (typically the athlete's global MMP model) instead of the
// per-activity model, which can be unreliable for low-intensity
// sessions.
//
// Returns the recomputed depletion, the detection result, and a list of
// human-readable warnings suitable for CLI output.
func RecomputeWBalDepletion( //nolint:gocritic // unnamed results are clearer here
	watts, cadence, hr []float64,
	globalModel PowerModel,
	activityModel *Activity,
) (int, float64, ArtifactDetectionResult, []string) {
	if len(watts) == 0 {
		return 0, 0, ArtifactDetectionResult{}, nil
	}

	cp, wprime := resolveModelParams(globalModel, activityModel)
	if cp <= 0 || wprime <= 0 {
		return 0, 0, ArtifactDetectionResult{}, []string{"insufficient power model for W'bal recalculation"}
	}

	detection := DetectFlywheelArtifacts(watts, cadence, hr, cp)

	cleaned := watts
	var warnings []string
	if len(detection.Artifacts) > 0 {
		cleaned = CleanPowerStream(watts, detection.Artifacts)
		warnings = append(warnings, flywheelWarning(detection))
	}

	if !ActivityModelReliable(activityModel) && activityModel != nil {
		warnings = append(warnings, unreliableActivityModelWarning(activityModel))
	}

	depletionJoules, depletionPct := ComputeWBalDepletion(cleaned, cp, wprime)
	return depletionJoules, depletionPct, detection, warnings
}

func resolveModelParams(globalModel PowerModel, activityModel *Activity) (int, int) { //nolint:gocritic // unnamed results are clearer here
	if globalModel.CriticalPower > 0 && globalModel.WPrime > 0 {
		return globalModel.CriticalPower, globalModel.WPrime
	}
	if activityModel != nil {
		return activityModel.CriticalPower, activityModel.WPrime
	}
	return 0, 0
}

func detectArtifactSegments(
	watts, cadence, hr []float64,
	threshold float64,
	minLen int,
	hasCadence, hasHR bool,
) []FlywheelArtifact {
	var artifacts []FlywheelArtifact

	i := 0
	for i < minLen {
		if watts[i] <= threshold {
			i++
			continue
		}

		start := i
		for i < minLen && watts[i] > threshold {
			i++
		}
		end := i
		duration := end - start

		if duration < flywheelMinDuration {
			continue
		}

		artifact := buildArtifact(watts, cadence, hr, start, end, hasCadence, hasHR)
		if isFlywheelArtifact(artifact, hasCadence, hasHR) {
			artifacts = append(artifacts, artifact)
		}
	}

	return artifacts
}

func buildArtifact(
	watts, cadence, hr []float64,
	start, end int,
	hasCadence, hasHR bool,
) FlywheelArtifact {
	artifact := FlywheelArtifact{
		StartIndex:   start,
		EndIndex:     end - 1,
		StartTimeSec: start,
		DurationSec:  end - start,
		PeakWatts:    peakFloatSlice(watts[start:end]),
	}

	if hasCadence && end <= len(cadence) {
		artifact.StartCadence = cadence[start]
		artifact.EndCadence = cadence[end-1]
	}
	if hasHR && end <= len(hr) {
		artifact.StartHR = hr[start]
		artifact.EndHR = hr[end-1]
	}

	return artifact
}

func isFlywheelArtifact(a FlywheelArtifact, hasCadence, hasHR bool) bool {
	if !hasCadence {
		return false
	}

	cadenceDropped := a.StartCadence > 0 &&
		a.EndCadence < a.StartCadence*(1-flywheelCadenceDropPct/percentScale)

	cadenceTooLow := a.EndCadence > 0 && a.EndCadence < flywheelLowCadenceRPM

	if !cadenceDropped && !cadenceTooLow {
		return false
	}

	if hasHR {
		hrRise := a.EndHR - a.StartHR
		if hrRise > flywheelMaxHRRise {
			return false
		}
	}

	return true
}

func streamMinLength(watts, cadence, hr []float64, hasCadence, hasHR bool) int {
	minLen := len(watts)
	if hasCadence && len(cadence) < minLen {
		minLen = len(cadence)
	}
	if hasHR && len(hr) < minLen {
		minLen = len(hr)
	}
	return minLen
}

func countSupraCP(watts []float64, cpF float64) int {
	count := 0
	for i := range watts {
		if watts[i] > cpF {
			count++
		}
	}
	return count
}

func peakFloatSlice(values []float64) float64 {
	peak := 0.0
	for _, v := range values {
		if v > peak {
			peak = v
		}
	}
	return peak
}

func flywheelWarning(detection ArtifactDetectionResult) string {
	totalSecs := 0
	for _, a := range detection.Artifacts {
		totalSecs += a.DurationSec
	}
	return "flywheel artifact detected: " + intToStr(totalSecs) +
		"s of supra-CP power removed from W'bal calculation across " +
		intToStr(len(detection.Artifacts)) + " segment(s)"
}

func unreliableActivityModelWarning(activity *Activity) string {
	return "activity power model unreliable (CP=" + intToStr(activity.CriticalPower) +
		"W < 85% of FTP=" + intToStr(activity.FTP) + "W); using global model for W'bal"
}

func intToStr(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
	pos := len(digits)
	for value > 0 {
		pos--
		digits[pos] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		pos--
		digits[pos] = '-'
	}
	return string(digits[pos:])
}
