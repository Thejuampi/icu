package icu

import (
	"math"
	"strconv"
)

// Coaching criteria for stream decoupling (B15), as specified by the client
// coach (Hugo). They are fixed constants, deliberately not derived from or
// fitted to the athlete's data, and replace the earlier data-derived gates
// (history VI cutoff, workout-kind gate). All are evaluated on the analyzed
// range: the moving samples left after the fixed warm-up/cool-down trim.
const (
	// CoachVIOK / CoachVILowConfidence: VI = NP / mean power. <= 1.05 is
	// valid, (1.05, 1.10] low confidence, > 1.10 not valid (no number).
	CoachVIOK            = 1.05
	CoachVILowConfidence = 1.10
	// CoachMinAnalyzedMinutes: the analyzed range must last at least 60 min.
	CoachMinAnalyzedMinutes = 60
	// CoachMinZ1Z2Share: at least 85% of the analyzed time at or below the
	// athlete's Z2 upper bound (sport-settings power zones, % FTP).
	CoachMinZ1Z2Share = 0.85
	// CoachMaxMinutePctFTP: no moving minute may average above 105% FTP.
	CoachMaxMinutePctFTP = 105.0
	// CoachMaxCoastingShare: coasting (watts = 0) under 10% of the time.
	CoachMaxCoastingShare = 0.10
	// CoachWarmupMinSeconds / CoachWarmupMaxSeconds / CoachCooldownSeconds:
	// the fixed trim. Warm-up is max(15 min, HR stabilization); when HR does
	// not stabilize by 20 min the warm-up is cut at 20 min and the result is
	// at most low confidence. Cool-down is always 10 min. Manual overrides
	// can only lengthen these, never shorten them.
	CoachWarmupMinSeconds = 15 * 60
	CoachWarmupMaxSeconds = 20 * 60
	CoachCooldownSeconds  = 10 * 60
	// HR stabilization: the first moving minute m >= CoachHRStableWindow at
	// which the mean HR of the window [m, m+5) is within CoachHRStableBPM of
	// the mean HR of the preceding window [m-5, m).
	CoachHRStableWindow = 5
	CoachHRStableBPM    = 3.0
	// Interpretation bands of the decoupling value (%): < 5 good, 5-8
	// moderate, > 8 high; < -5 flags possible fatigue.
	CoachDecouplingGood     = 5.0
	CoachDecouplingModerate = 8.0
	CoachPossibleFatigue    = -5.0
	// npWindowSeconds is the standard 30 s rolling window of normalized power.
	npWindowSeconds = 30
)

// Validity, interpretation and criterion names.
const (
	DecouplingValid         = "valid"
	DecouplingLowConfidence = "low_confidence"
	DecouplingNotValid      = "not_valid"

	DecouplingInterpretationGood     = "good"
	DecouplingInterpretationModerate = "moderate"
	DecouplingInterpretationHigh     = "high"

	CriterionVI           = "variability_index"
	CriterionDuration     = "duration"
	CriterionZ1Z2         = "z1_z2_share"
	CriterionMaxMinute    = "max_minute_pct_ftp"
	CriterionCoasting     = "coasting_share"
	CriterionPowerStream  = "power_stream"
	CriterionAthleteZones = "athlete_ftp_and_zones"

	DecouplingTrimCoachFixed = "coach_fixed"
)

// DecouplingCriterion is one evaluated coaching criterion.
type DecouplingCriterion struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Threshold string  `json:"threshold"`
	Pass      bool    `json:"pass"`
}

// coachWarmupSamples returns the fixed warm-up in moving samples:
// max(15 min, HR stabilization) and whether HR stabilized. Stabilization
// is only searched up to 20 min; without it the warm-up is 20 min.
func (ride *decouplingRide) coachWarmupSamples() (int, bool) {
	perMinute := ride.samplesPerMinute()
	minimum := int(math.Round(CoachWarmupMinSeconds / ride.step))
	minuteHR := func(minute int) (float64, bool) {
		sum, n := 0.0, 0
		for _, index := range ride.moving[minute*perMinute : (minute+1)*perMinute] {
			if value, ok := ride.hrAt[int(ride.elapsed[index])]; ok {
				sum += value
				n++
			}
		}
		if n == 0 {
			return 0, false
		}

		return sum / float64(n), true
	}
	windowHR := func(from int) (float64, bool) {
		sum := 0.0
		for minute := from; minute < from+CoachHRStableWindow; minute++ {
			value, ok := minuteHR(minute)
			if !ok {
				return 0, false
			}
			sum += value
		}

		return sum / CoachHRStableWindow, true
	}
	minutes := len(ride.moving) / perMinute
	maxMinute := CoachWarmupMaxSeconds / secondsPerMinute
	for minute := CoachHRStableWindow; minute <= maxMinute && minute+CoachHRStableWindow <= minutes; minute++ {
		before, okBefore := windowHR(minute - CoachHRStableWindow)
		after, okAfter := windowHR(minute)
		if okBefore && okAfter && math.Abs(after-before) <= CoachHRStableBPM {
			return maxInt(minimum, minute*perMinute), true
		}
	}

	return int(math.Round(CoachWarmupMaxSeconds / ride.step)), false
}

// evaluateCoachCriteria checks the coaching criteria on moving samples
// [start, end) and returns them with the VI.
func (ride *decouplingRide) evaluateCoachCriteria(start, end int, context DecouplingContext) ([]DecouplingCriterion, float64) {
	analyzedMinutes := float64(end-start) * ride.step / secondsPerMinuteF
	criteria := []DecouplingCriterion{{
		Name: CriterionDuration, Value: round2(analyzedMinutes),
		Threshold: ">= " + strconv.Itoa(CoachMinAnalyzedMinutes) + " min", Pass: analyzedMinutes >= CoachMinAnalyzedMinutes,
	}}
	if !ride.power {
		return append(criteria, DecouplingCriterion{Name: CriterionPowerStream, Threshold: "power stream required"}), 0
	}
	var sum, coasting float64
	values := make([]float64, 0, end-start)
	for _, index := range ride.moving[start:end] {
		value, _ := ride.output.At(index)
		values = append(values, value)
		sum += value
		if value <= 0 {
			coasting++
		}
	}
	n := float64(len(values))
	vi := 0.0
	if mean := sum / n; mean > 0 {
		// Unrounded: the validity bands compare the exact VI.
		vi = normalizedPower(values, maxInt(1, int(math.Round(npWindowSeconds/ride.step)))) / mean
	}
	criteria = append(criteria,
		DecouplingCriterion{Name: CriterionVI, Value: round4(vi), Threshold: "<= 1.10 (<= 1.05 ok)", Pass: CoachVIValidity(vi) != DecouplingNotValid},
		DecouplingCriterion{
			Name: CriterionCoasting, Value: round4(coasting / n), Threshold: "< 0.10",
			Pass: coasting/n < CoachMaxCoastingShare,
		})
	if context.FTP <= 0 || len(context.PowerZones) < 2 {
		return append(criteria, DecouplingCriterion{Name: CriterionAthleteZones, Threshold: "FTP and power zones required"}), vi
	}
	ftp := float64(context.FTP)
	z2Upper := float64(context.PowerZones[1]) * ftp / percentScale
	inZ1Z2, maxMinute := 0.0, 0.0
	for _, value := range values {
		if value <= z2Upper {
			inZ1Z2++
		}
	}
	perMinute := ride.samplesPerMinute()
	for from := 0; from+perMinute <= len(values); from += perMinute {
		minuteSum := 0.0
		for _, value := range values[from : from+perMinute] {
			minuteSum += value
		}
		maxMinute = math.Max(maxMinute, minuteSum/float64(perMinute)*percentScale/ftp)
	}

	return append(criteria,
		DecouplingCriterion{Name: CriterionZ1Z2, Value: round4(inZ1Z2 / n), Threshold: ">= 0.85", Pass: inZ1Z2/n >= CoachMinZ1Z2Share},
		DecouplingCriterion{
			Name: CriterionMaxMinute, Value: round2(maxMinute), Threshold: "<= 105 % FTP",
			Pass: maxMinute <= CoachMaxMinutePctFTP,
		}), vi
}

// normalizedPower: fourth root of the mean fourth power of the rolling mean.
func normalizedPower(values []float64, window int) float64 {
	if len(values) < window {
		window = len(values)
	}
	rolling, sum4 := 0.0, 0.0
	count := 0
	for index, value := range values {
		rolling += value
		if index >= window {
			rolling -= values[index-window]
		}
		if index >= window-1 {
			mean := rolling / float64(window)
			sum4 += mean * mean * mean * mean
			count++
		}
	}
	if count == 0 {
		return 0
	}

	return math.Pow(sum4/float64(count), 0.25)
}

// ApplyCoachCriteria applies the evaluated result.Criteria: any failure
// makes the result not valid and drops the number (the reason names the
// failed criteria; RawValue keeps the pipeline's number for display, it is
// never consumed); otherwise the VI decides valid or low confidence and a
// measured value gets its interpretation. A result without criteria
// (unusable streams) or not measured by the stream pipeline is not valid.
func (result *StreamDecoupling) ApplyCoachCriteria() {
	if result.Value != nil {
		result.RawValue = new(*result.Value)
	}
	result.FailedCriteriaNames = nil
	for _, criterion := range result.Criteria {
		if !criterion.Pass {
			result.FailedCriteriaNames = append(result.FailedCriteriaNames, criterion.Name)
		}
	}
	switch {
	case len(result.FailedCriteriaNames) > 0:
		result.Validity = DecouplingNotValid
		if result.Status == DecouplingStatusMeasured {
			result.Status = DecouplingStatusNotMeasurable
		}
		result.Value = nil
		result.Reason = "not valid: failed coaching criteria"
		for index, name := range result.FailedCriteriaNames {
			sep := ", "
			if index == 0 {
				sep = " "
			}
			result.Reason += sep + name
		}
	case len(result.Criteria) == 0 || result.Status != DecouplingStatusMeasured:
		result.Validity = DecouplingNotValid
		result.Value = nil
	default:
		result.Validity = CoachVIValidity(result.VariabilityIndex)
		if result.Validity == DecouplingValid && !result.HRStabilized {
			// HR never stabilized: warm-up cut at 20 min, low confidence.
			result.Validity = DecouplingLowConfidence
		}
		if result.Value != nil {
			result.Interpretation, result.PossibleFatigue = DecouplingInterpretation(*result.Value)
		}
	}
}

// CoachVIValidity: VI <= 1.05 valid, (1.05, 1.10] low confidence, above
// 1.10 (or no VI) not valid.
func CoachVIValidity(vi float64) string {
	switch {
	case vi <= 0 || vi > CoachVILowConfidence:
		return DecouplingNotValid
	case vi > CoachVIOK:
		return DecouplingLowConfidence
	default:
		return DecouplingValid
	}
}

// DecouplingInterpretation maps a decoupling value to the coach's bands:
// < 5 good, 5..8 moderate, > 8 high; below -5 flags possible fatigue.
func DecouplingInterpretation(value float64) (string, bool) {
	band := DecouplingInterpretationModerate
	switch {
	case value < CoachDecouplingGood:
		band = DecouplingInterpretationGood
	case value > CoachDecouplingModerate:
		band = DecouplingInterpretationHigh
	}

	return band, value < CoachPossibleFatigue
}
