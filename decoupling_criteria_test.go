package icu_test

import (
	"math"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// criteriaContext: FTP 200 W and the standard zones, so Z2 tops out at
// 150 W (75%) and 105% FTP is 210 W.
func criteriaContext() icu.DecouplingContext {
	return icu.DecouplingContext{FTP: 200, PowerZones: []int{55, 75, 90, 105, 120, 150, 999}}
}

func criterion(t *testing.T, result *icu.StreamDecoupling, name string) icu.DecouplingCriterion {
	t.Helper()
	for _, item := range result.Criteria {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("criterion %s missing in %+v", name, result.Criteria)

	return icu.DecouplingCriterion{}
}

// evaluate wraps the given minutes in a 15 min warm-up and a 10 min
// cool-down at 140 W (steady HR, so the coach's trim is exactly 15/10) and
// runs the full pipeline, so the analyzed range is the given minutes.
func evaluate(t *testing.T, minutes int, power func(int) float64) icu.StreamDecoupling {
	t.Helper()
	padded := func(sec int) float64 {
		if sec < icu.CoachWarmupMinSeconds || sec >= icu.CoachWarmupMinSeconds+minutes*60 {
			return 140
		}

		return power(sec - icu.CoachWarmupMinSeconds)
	}
	options := icu.DefaultDecouplingOptions()
	options.HRLagSeconds = new(0)
	result := icu.ComputeStreamDecoupling(synthRide(minutes+25, padded, constant(140)), options, criteriaContext())
	result.ApplyCoachCriteria()

	return result
}

func TestCoachCriterionDurationBoundary(t *testing.T) {
	t.Parallel()
	if got := evaluate(t, 60, constant(140)); !criterion(t, &got, icu.CriterionDuration).Pass || got.Validity != icu.DecouplingValid {
		t.Fatalf("60 min: %+v", got)
	}
	got := evaluate(t, 59, constant(140))
	if criterion(t, &got, icu.CriterionDuration).Pass || got.Value != nil || got.Validity != icu.DecouplingNotValid ||
		len(got.FailedCriteriaNames) != 1 || got.FailedCriteriaNames[0] != icu.CriterionDuration {
		t.Fatalf("59 min: %+v", got)
	}
	// The raw stream number is kept next to the not-valid label.
	if got.RawValue == nil || *got.RawValue != 0 {
		t.Fatalf("59 min raw value: %v", got.RawValue)
	}
	if got.Reason != "not valid: failed coaching criteria duration" {
		t.Fatalf("reason %q", got.Reason)
	}
}

func TestCoachCriterionZ1Z2Boundary(t *testing.T) {
	t.Parallel()
	above := func(aboveMinutes int) func(int) float64 {
		return func(sec int) float64 {
			if sec/60 < aboveMinutes {
				return 160
			}

			return 140
		}
	}
	// 9 of 60 min above Z2: exactly 85% in Z1-Z2 passes; 10 min (83.3%) fails.
	if got := evaluate(t, 60, above(9)); !criterion(t, &got, icu.CriterionZ1Z2).Pass {
		t.Fatalf("85%%: %+v", criterion(t, &got, icu.CriterionZ1Z2))
	}
	if got := evaluate(t, 60, above(10)); criterion(t, &got, icu.CriterionZ1Z2).Pass || got.Validity != icu.DecouplingNotValid {
		t.Fatalf("83%%: %+v", criterion(t, &got, icu.CriterionZ1Z2))
	}
	// The Z2 upper bound itself (150 W) counts as Z2.
	if got := evaluate(t, 60, constant(150)); criterion(t, &got, icu.CriterionZ1Z2).Value != 1 {
		t.Fatalf("150 W: %+v", criterion(t, &got, icu.CriterionZ1Z2))
	}
}

func TestCoachCriterionMaxMinuteBoundary(t *testing.T) {
	t.Parallel()
	spike := func(watts float64) func(int) float64 {
		return func(sec int) float64 {
			if sec/60 == 30 {
				return watts
			}

			return 140
		}
	}
	if got := evaluate(t, 60, spike(210)); !criterion(t, &got, icu.CriterionMaxMinute).Pass {
		t.Fatalf("105%%: %+v", criterion(t, &got, icu.CriterionMaxMinute))
	}
	if got := evaluate(t, 60, spike(211)); criterion(t, &got, icu.CriterionMaxMinute).Pass || got.Validity != icu.DecouplingNotValid {
		t.Fatalf("105.5%%: %+v", criterion(t, &got, icu.CriterionMaxMinute))
	}
}

func TestCoachCriterionCoastingBoundary(t *testing.T) {
	t.Parallel()
	coast := func(seconds int) func(int) float64 {
		return func(sec int) float64 {
			// one zero every 10 s from the start, so no minute is a
			// coasting minute and the VI stays low
			if sec%10 == 0 && sec/10 < seconds {
				return 0
			}

			return 140
		}
	}
	// 3600 s analyzed: 359 s coasting (9.97%) passes; 360 s (10%) fails.
	if got := evaluate(t, 60, coast(359)); !criterion(t, &got, icu.CriterionCoasting).Pass {
		t.Fatalf("9.97%%: %+v", criterion(t, &got, icu.CriterionCoasting))
	}
	if got := evaluate(t, 60, coast(360)); criterion(t, &got, icu.CriterionCoasting).Pass || got.Validity != icu.DecouplingNotValid {
		t.Fatalf("10%%: %+v", criterion(t, &got, icu.CriterionCoasting))
	}
}

func TestCoachVIValidityBoundaries(t *testing.T) {
	t.Parallel()
	for vi, want := range map[float64]string{
		1.0: icu.DecouplingValid, 1.05: icu.DecouplingValid, 1.0501: icu.DecouplingLowConfidence,
		1.10: icu.DecouplingLowConfidence, 1.1001: icu.DecouplingNotValid, 0: icu.DecouplingNotValid,
	} {
		if got := icu.CoachVIValidity(vi); got != want {
			t.Fatalf("VI %v: %s, want %s", vi, got, want)
		}
	}
	// A constant ride has VI 1: valid, the number kept and interpreted.
	got := evaluate(t, 60, constant(140))
	if math.Abs(got.VariabilityIndex-1) > 1e-9 || got.Validity != icu.DecouplingValid || got.Value == nil || got.Interpretation != icu.DecouplingInterpretationGood {
		t.Fatalf("constant: %+v", got)
	}
	// Alternating 30 s at 60/220 W: VI well above 1.10, not valid, no number.
	surges := evaluate(t, 60, func(sec int) float64 {
		if (sec/30)%2 == 0 {
			return 60
		}

		return 220
	})
	if surges.VariabilityIndex <= icu.CoachVILowConfidence || surges.Validity != icu.DecouplingNotValid || surges.Value != nil {
		t.Fatalf("surges: VI %v %+v", surges.VariabilityIndex, surges.FailedCriteriaNames)
	}
}

func TestCoachApplyCriteriaStates(t *testing.T) {
	t.Parallel()
	low := icu.StreamDecoupling{
		Status: icu.DecouplingStatusMeasured, Value: new(6.0), VariabilityIndex: 1.08,
		Criteria: []icu.DecouplingCriterion{{Name: icu.CriterionVI, Value: 1.08, Pass: true}},
	}
	low.ApplyCoachCriteria()
	if low.RawValue == nil || *low.RawValue != 6 {
		t.Fatalf("low raw: %v", low.RawValue)
	}
	if low.Validity != icu.DecouplingLowConfidence || low.Value == nil || low.Interpretation != icu.DecouplingInterpretationModerate {
		t.Fatalf("low confidence: %+v", low)
	}
	fatigue := icu.StreamDecoupling{
		Status: icu.DecouplingStatusMeasured, Value: new(-6.0), VariabilityIndex: 1.0, HRStabilized: true,
		Criteria: []icu.DecouplingCriterion{{Name: icu.CriterionVI, Value: 1, Pass: true}},
	}
	fatigue.ApplyCoachCriteria()
	if fatigue.Validity != icu.DecouplingValid || !fatigue.PossibleFatigue {
		t.Fatalf("fatigue: %+v", fatigue)
	}
	empty := icu.StreamDecoupling{Status: icu.DecouplingStatusNotMeasurable}
	empty.ApplyCoachCriteria()
	if empty.Validity != icu.DecouplingNotValid {
		t.Fatalf("no criteria: %+v", empty)
	}
	unmeasured := icu.StreamDecoupling{Status: icu.DecouplingStatusNotMeasurable, Criteria: []icu.DecouplingCriterion{{Pass: true}}}
	unmeasured.ApplyCoachCriteria()
	if unmeasured.Validity != icu.DecouplingNotValid {
		t.Fatalf("unmeasured: %+v", unmeasured)
	}
}

func TestCoachCriteriaNeedPowerAndZones(t *testing.T) {
	t.Parallel()
	noZones := icu.ComputeStreamDecoupling(synthRide(60, constant(140), constant(140)), fixed(0), icu.DecouplingContext{})
	noZones.ApplyCoachCriteria()
	if criterion(t, &noZones, icu.CriterionAthleteZones).Pass || noZones.Value != nil {
		t.Fatalf("no zones: %+v", noZones)
	}
	ride := synthRide(60, constant(3), constant(150))
	run := icu.NullableStreamData{"time": ride["time"], "velocity_smooth": ride["watts"], "heartrate": ride["heartrate"]}
	paced := icu.ComputeStreamDecoupling(run, fixed(0), criteriaContext())
	paced.ApplyCoachCriteria()
	if criterion(t, &paced, icu.CriterionPowerStream).Pass || paced.Validity != icu.DecouplingNotValid {
		t.Fatalf("no power: %+v", paced)
	}
}

func TestDecouplingInterpretationBoundaries(t *testing.T) {
	t.Parallel()
	for value, want := range map[float64]struct {
		band    string
		fatigue bool
	}{
		4.99: {icu.DecouplingInterpretationGood, false}, 5: {icu.DecouplingInterpretationModerate, false},
		8: {icu.DecouplingInterpretationModerate, false}, 8.01: {icu.DecouplingInterpretationHigh, false},
		-5: {icu.DecouplingInterpretationGood, false}, -5.01: {icu.DecouplingInterpretationGood, true},
	} {
		if band, fatigue := icu.DecouplingInterpretation(value); band != want.band || fatigue != want.fatigue {
			t.Fatalf("%v: %s %v, want %s %v", value, band, fatigue, want.band, want.fatigue)
		}
	}
}

func TestCoachFixedTrimWarmupFollowsHRStabilization(t *testing.T) {
	t.Parallel()
	// HR climbs 2 bpm/min for 15 min, then is flat: consecutive 5 min windows
	// first agree within 3 bpm at minute 17, so warm-up passes 15 min.
	climb := func(sec int) float64 { return 110 + 2*float64(min(sec/60, 15)) }
	got := icu.ComputeStreamDecoupling(synthRide(100, constant(140), climb), icu.DefaultDecouplingOptions(), criteriaContext())
	if !got.HRStabilized || got.Trim.WarmupSeconds != 17*60 || got.Trim.CooldownSeconds != icu.CoachCooldownSeconds {
		t.Fatalf("climb trim: %+v stabilized %v", got.Trim, got.HRStabilized)
	}
	// HR never settles (1.5 bpm/min throughout): warm-up cut at 20 min, flagged.
	drift := func(sec int) float64 { return 100 + 1.5*float64(sec/60) }
	never := icu.ComputeStreamDecoupling(synthRide(100, constant(140), drift), icu.DefaultDecouplingOptions(), criteriaContext())
	if never.HRStabilized || never.Trim.WarmupSeconds != icu.CoachWarmupMaxSeconds {
		t.Fatalf("never: %+v stabilized %v", never.Trim, never.HRStabilized)
	}
	// A manual 0/0 override cannot shorten the coach's trim.
	manual := icu.ComputeStreamDecoupling(synthRide(100, constant(140), climb), fixed(0), criteriaContext())
	if manual.Trim.Method != icu.DecouplingTrimCoachFixed || manual.Trim.WarmupSeconds != 17*60 || manual.Trim.CooldownSeconds != icu.CoachCooldownSeconds {
		t.Fatalf("manual: %+v", manual.Trim)
	}
}

// coachRide runs a constant 140 W / given-HR ride run with the coach's
// default trim and 0 s lag, with the criteria applied.
func coachRide(t *testing.T, minutes int, hr func(int) float64, options icu.DecouplingOptions) icu.StreamDecoupling {
	t.Helper()
	options.HRLagSeconds = new(0)
	result := icu.ComputeStreamDecoupling(synthRide(minutes, constant(140), hr), options, criteriaContext())
	result.ApplyCoachCriteria()

	return result
}

// M1: a 0/0 manual trim cannot shorten the coach's trim. A 75 min ride is
// 75 min with no trim (would pass duration) but 50 min after 15/10: not valid.
func TestCoachTrimZeroOverrideIsNotValid(t *testing.T) {
	t.Parallel()
	got := coachRide(t, 75, constant(140), fixed(0))
	if got.Validity != icu.DecouplingNotValid || got.Value != nil || got.Trim.WarmupSeconds != icu.CoachWarmupMinSeconds ||
		got.Trim.CooldownSeconds != icu.CoachCooldownSeconds || len(got.FailedCriteriaNames) != 1 || got.FailedCriteriaNames[0] != icu.CriterionDuration {
		t.Fatalf("0/0 trim: validity %s trim %+v failed %v", got.Validity, got.Trim, got.FailedCriteriaNames)
	}
	if criterion(t, &got, icu.CriterionDuration).Value != 50 {
		t.Fatalf("analyzed minutes %+v, want 50", criterion(t, &got, icu.CriterionDuration))
	}
}

// The 60 min minimum is counted after the trims: 85 min raw (15 + 60 + 10)
// passes, 84 min fails.
func TestCoachMinimumDurationAfterTrims(t *testing.T) {
	t.Parallel()
	if got := coachRide(t, 85, constant(140), icu.DefaultDecouplingOptions()); got.Validity != icu.DecouplingValid {
		t.Fatalf("85 min: %s %v", got.Validity, got.FailedCriteriaNames)
	}
	if got := coachRide(t, 84, constant(140), icu.DefaultDecouplingOptions()); got.Validity != icu.DecouplingNotValid ||
		got.FailedCriteriaNames[0] != icu.CriterionDuration {
		t.Fatalf("84 min: %s %v", got.Validity, got.FailedCriteriaNames)
	}
}

// Warm-up cap: HR stabilizing at minute 20 keeps a 20 min warm-up and stays
// valid; at minute 21 it is past the cap, so the warm-up is cut at 20 min
// and the result is low confidence. Cool-down stays 10 min.
func TestCoachWarmupCapAt20Minutes(t *testing.T) {
	t.Parallel()
	climb := func(top int) func(int) float64 {
		return func(sec int) float64 { return 110 + 2*float64(min(sec/60, top)) }
	}
	at20 := coachRide(t, 100, climb(18), icu.DefaultDecouplingOptions())
	if !at20.HRStabilized || at20.Trim.WarmupSeconds != icu.CoachWarmupMaxSeconds || at20.Validity != icu.DecouplingValid {
		t.Fatalf("stabilized at 20: %+v %v %s", at20.Trim, at20.HRStabilized, at20.Validity)
	}
	past := coachRide(t, 100, climb(19), icu.DefaultDecouplingOptions())
	if past.HRStabilized || past.Trim.WarmupSeconds != icu.CoachWarmupMaxSeconds ||
		past.Trim.CooldownSeconds != icu.CoachCooldownSeconds || past.Validity != icu.DecouplingLowConfidence || past.Value == nil {
		t.Fatalf("past the cap: %+v %v %s", past.Trim, past.HRStabilized, past.Validity)
	}
	// Not stabilized never upgrades a not-valid result.
	short := coachRide(t, 60, climb(19), icu.DefaultDecouplingOptions())
	if short.Validity != icu.DecouplingNotValid {
		t.Fatalf("short unstable: %s", short.Validity)
	}
}

// HR stability edge: a staircase rising d bpm every 5 min makes every pair
// of consecutive 5 min windows differ by exactly d. d = 3 bpm stabilizes
// (at minute 5, so the 15 min floor applies); d = 3.01 never does.
func TestCoachHRStabilityEdge(t *testing.T) {
	t.Parallel()
	stairs := func(step float64) func(int) float64 {
		return func(sec int) float64 { return 120 + step*float64(sec/300) }
	}
	if got := coachRide(t, 100, stairs(3), icu.DefaultDecouplingOptions()); !got.HRStabilized || got.Trim.WarmupSeconds != icu.CoachWarmupMinSeconds {
		t.Fatalf("3 bpm: %+v %v", got.Trim, got.HRStabilized)
	}
	if got := coachRide(t, 100, stairs(3.01), icu.DefaultDecouplingOptions()); got.HRStabilized || got.Trim.WarmupSeconds != icu.CoachWarmupMaxSeconds {
		t.Fatalf("3.01 bpm: %+v %v", got.Trim, got.HRStabilized)
	}
}

// A measured-looking value with a non-measured status is cleared (raw kept).
func TestCoachApplyCriteriaClearsValueWhenNotMeasured(t *testing.T) {
	t.Parallel()
	result := icu.StreamDecoupling{
		Status: icu.DecouplingStatusNotMeasurable, Value: new(4.2), VariabilityIndex: 1, HRStabilized: true,
		Criteria: []icu.DecouplingCriterion{{Name: icu.CriterionVI, Value: 1, Pass: true}},
	}
	result.ApplyCoachCriteria()
	if result.Value != nil || result.Validity != icu.DecouplingNotValid || result.RawValue == nil || *result.RawValue != 4.2 {
		t.Fatalf("not measured: value %v validity %s raw %v", result.Value, result.Validity, result.RawValue)
	}
}

// VI bands compare the exact VI: 1.05004 would round to 1.05 (valid) but is
// low confidence.
func TestCoachVIValidityIsNotRounded(t *testing.T) {
	t.Parallel()
	if got := icu.CoachVIValidity(1.05004); got != icu.DecouplingLowConfidence {
		t.Fatalf("1.05004: %s", got)
	}
	if got := icu.CoachVIValidity(1.10004); got != icu.DecouplingNotValid {
		t.Fatalf("1.10004: %s", got)
	}
}

// The 30 s NP window matters: 15 s alternations of 60/220 W average out in
// the window (VI 1, valid); 60 s blocks of the same powers do not (VI above
// 1.10, not valid). A 1 s window would give both the same VI.
func TestCoachNPWindowChangesVI(t *testing.T) {
	t.Parallel()
	alternate := func(period int) func(int) float64 {
		return func(sec int) float64 {
			if (sec/period)%2 == 0 {
				return 60
			}

			return 220
		}
	}
	fast := evaluate(t, 60, alternate(15))
	slow := evaluate(t, 60, alternate(60))
	if math.Abs(fast.VariabilityIndex-1) > 0.01 || criterion(t, &fast, icu.CriterionVI).Pass != true {
		t.Fatalf("15 s: VI %v", fast.VariabilityIndex)
	}
	if slow.VariabilityIndex <= icu.CoachVILowConfidence || criterion(t, &slow, icu.CriterionVI).Pass {
		t.Fatalf("60 s: VI %v", slow.VariabilityIndex)
	}
}
