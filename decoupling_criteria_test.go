package icu_test

import (
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

// evaluate runs the full pipeline with no trim, so the analyzed range is
// the whole synthetic ride, and applies the criteria.
func evaluate(t *testing.T, minutes int, power func(int) float64) icu.StreamDecoupling {
	t.Helper()
	result := icu.ComputeStreamDecoupling(synthRide(minutes, power, constant(140)), fixed(0), criteriaContext())
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
		len(got.FailedCriteria) != 1 || got.FailedCriteria[0] != icu.CriterionDuration {
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
	if got.VariabilityIndex != 1 || got.Validity != icu.DecouplingValid || got.Value == nil || got.Interpretation != icu.DecouplingInterpretationGood {
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
		t.Fatalf("surges: VI %v %+v", surges.VariabilityIndex, surges.FailedCriteria)
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
		Status: icu.DecouplingStatusMeasured, Value: new(-6.0), VariabilityIndex: 1.0,
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
	// HR climbs 2 bpm/min for 25 min, then is flat: consecutive 5 min windows
	// agree within 3 bpm only after the climb, so warm-up passes 15 min.
	climb := func(sec int) float64 { return 110 + 2*float64(min(sec/60, 25)) }
	got := icu.ComputeStreamDecoupling(synthRide(100, constant(140), climb), icu.DefaultDecouplingOptions(), criteriaContext())
	if !got.HRStabilized || got.Trim.WarmupSeconds <= icu.CoachWarmupMinSeconds || got.Trim.CooldownSeconds != icu.CoachCooldownSeconds {
		t.Fatalf("climb trim: %+v stabilized %v", got.Trim, got.HRStabilized)
	}
	// HR never settles (1.5 bpm/min throughout): the 15 min floor, flagged.
	drift := func(sec int) float64 { return 100 + 1.5*float64(sec/60) }
	never := icu.ComputeStreamDecoupling(synthRide(100, constant(140), drift), icu.DefaultDecouplingOptions(), criteriaContext())
	if never.HRStabilized || never.Trim.WarmupSeconds != icu.CoachWarmupMinSeconds {
		t.Fatalf("never: %+v stabilized %v", never.Trim, never.HRStabilized)
	}
	// A manual override still wins over the coach's trim.
	manual := icu.ComputeStreamDecoupling(synthRide(100, constant(140), climb), fixed(0), criteriaContext())
	if manual.Trim.Method != icu.DecouplingTrimManual {
		t.Fatalf("manual: %+v", manual.Trim)
	}
}
