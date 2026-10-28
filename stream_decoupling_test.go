package icu_test

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// synthRide builds 1 Hz streams of the given length in minutes. power(sec)
// and hr(sec) return the sample at second sec; a negative HR is a null.
func synthRide(minutes int, power, hr func(sec int) float64) icu.NullableStreamData {
	n := minutes * 60
	timeS := icu.NullableSeries{Values: make([]float64, n), Present: make([]bool, n)}
	watts := icu.NullableSeries{Values: make([]float64, n), Present: make([]bool, n)}
	heart := icu.NullableSeries{Values: make([]float64, n), Present: make([]bool, n)}
	for sec := range n {
		timeS.Values[sec], timeS.Present[sec] = float64(sec), true
		watts.Values[sec], watts.Present[sec] = power(sec), true
		if value := hr(sec); value >= 0 {
			heart.Values[sec], heart.Present[sec] = value, true
		}
	}

	return icu.NullableStreamData{"time": timeS, "watts": watts, "heartrate": heart}
}

func constant(value float64) func(int) float64 { return func(int) float64 { return value } }

// fixed returns options with a manual 0/0 trim and 0 s lag, isolating the
// rule under test from the derived trim and lag.
func fixed(k float64) icu.DecouplingOptions {
	return icu.DecouplingOptions{WarmupSeconds: new(0), CooldownSeconds: new(0), HRLagSeconds: new(0), IQRK: k}
}

func mustDecouple(t *testing.T, streams icu.NullableStreamData, options icu.DecouplingOptions) icu.StreamDecoupling {
	t.Helper()
	got := icu.ComputeStreamDecoupling(streams, options, icu.DecouplingContext{})
	if got.Status != icu.DecouplingStatusMeasured || got.Value == nil {
		t.Fatalf("status %q (%s), want measured", got.Status, got.Reason)
	}

	return got
}

func TestStreamDecouplingSteadyRideIsZero(t *testing.T) {
	t.Parallel()
	got := mustDecouple(t, synthRide(90, constant(150), constant(135)), icu.DefaultDecouplingOptions())
	if *got.Value != 0 || got.Basis != icu.DecouplingBasisPowerHR {
		t.Fatalf("got %+v, want 0 power_hr", got)
	}
	if got.Trim.Method != icu.DecouplingTrimSteadyState || got.Trim.WarmupSeconds != 0 || got.MinutesKept != 90 {
		t.Fatalf("trim %+v kept %d, want steady_state 0 kept 90", got.Trim, got.MinutesKept)
	}
	if got.HRLagMethod != icu.DecouplingLagNone || got.HRLagSeconds != 0 {
		t.Fatalf("lag %d %s, want 0 none (no output variance)", got.HRLagSeconds, got.HRLagMethod)
	}
}

func TestStreamDecouplingDetectsHRDrift(t *testing.T) {
	t.Parallel()
	// HR 5% higher in the second half at equal power: 1 - 1/1.05 = 4.76%.
	hr := func(sec int) float64 {
		if sec >= 45*60 {
			return 140 * 1.05
		}

		return 140
	}
	got := mustDecouple(t, synthRide(90, constant(150), hr), icu.DefaultDecouplingOptions())
	if math.Abs(*got.Value-4.76) > 0.01 {
		t.Fatalf("decoupling = %v, want 4.76", *got.Value)
	}
}

func TestStreamDecouplingManualTrim(t *testing.T) {
	t.Parallel()
	options := fixed(0)
	options.WarmupSeconds, options.CooldownSeconds = new(600), new(300)
	got := mustDecouple(t, synthRide(90, constant(150), constant(135)), options)
	if got.Trim.Method != icu.DecouplingTrimManual || got.MinutesInRange != 75 {
		t.Fatalf("trim %+v in range %d, want manual 75", got.Trim, got.MinutesInRange)
	}
}

func TestStreamDecouplingPlanTrimThenSteadyState(t *testing.T) {
	t.Parallel()
	// Plan says 10 min warm-up / 5 min cool-down; the ride also has 3 easy
	// minutes after the plan's warm-up, which the steady-state rule drops.
	power := func(sec int) float64 {
		if sec >= 10*60 && sec < 13*60 {
			return 100
		}

		return 200
	}
	streams := synthRide(90, power, constant(140))
	context := icu.DecouplingContext{Plan: &icu.DecouplingPlanTrim{WarmupSeconds: 600, CooldownSeconds: 300}}
	got := icu.ComputeStreamDecoupling(streams, icu.DefaultDecouplingOptions(), context)
	if got.Trim.Method != "workout_steps+steady_state" || got.Trim.WarmupSeconds != 780 || got.Trim.CooldownSeconds != 300 {
		t.Fatalf("trim = %+v, want workout_steps+steady_state 780/300", got.Trim)
	}
	if got.MinutesInRange != 72 {
		t.Fatalf("minutes in range = %d, want 72", got.MinutesInRange)
	}
}

func TestStreamDecouplingSteadyStateTrimsEasyEdges(t *testing.T) {
	t.Parallel()
	power := func(sec int) float64 {
		switch {
		case sec < 10*60:
			return 100
		case sec >= 85*60:
			return 90
		default:
			return 200
		}
	}
	got := mustDecouple(t, synthRide(90, power, constant(140)), icu.DefaultDecouplingOptions())
	if got.Trim.WarmupSeconds != 600 || got.Trim.CooldownSeconds != 300 || got.MinutesInRange != 75 {
		t.Fatalf("trim %+v in range %d, want 600/300 and 75", got.Trim, got.MinutesInRange)
	}
}

func TestStreamDecouplingExcludesCoastingMinutes(t *testing.T) {
	t.Parallel()
	// Minutes 30-34 pedal only 20 s of 60 at 210 W (mean 70 W, above the old
	// fixed 50 W floor): coasting by the pedalling-fraction rule.
	power := func(sec int) float64 {
		if minute := sec / 60; minute >= 30 && minute < 35 && sec%60 >= 20 {
			return 0
		}
		if sec/60 >= 30 && sec/60 < 35 {
			return 210
		}

		return 150
	}
	got := mustDecouple(t, synthRide(90, power, constant(135)), fixed(0))
	if got.Excluded[icu.DecouplingExcludedCoasting] != 5 || got.MinutesKept != 85 || *got.Value != 0 {
		t.Fatalf("excluded %v kept %d value %v, want coasting 5, 85 kept, 0", got.Excluded, got.MinutesKept, *got.Value)
	}
	// Pedalling 40 of 60 s is not coasting.
	half := func(sec int) float64 {
		if sec/60 == 30 && sec%60 >= 40 {
			return 0
		}

		return 150
	}
	if got := mustDecouple(t, synthRide(90, half, constant(135)), fixed(0)); got.Excluded[icu.DecouplingExcludedCoasting] != 0 {
		t.Fatalf("excluded %v, want no coasting", got.Excluded)
	}
}

// outlierRide: minute powers cycle 148/150/152/154 W at constant HR, one
// minute at 162 W (between the k=1.5 and k=3 fences), and 30 low-power
// minutes pedalled at 60 W (not coasting) whose HR covers only 20 s, so the
// missing_hr filter drops them. Their 60/140 ratio would widen the fences
// enough to keep the 162 W minute if fences were computed before filtering.
func outlierRide() icu.NullableStreamData {
	cycle := []float64{148, 150, 152, 154}
	lowMinute := func(sec int) bool { return sec/60 >= 20 && sec/60 < 50 }
	power := func(sec int) float64 {
		minute := sec / 60
		switch {
		case minute == 60:
			return 162
		case lowMinute(sec):
			return 60
		default:
			return cycle[minute%len(cycle)]
		}
	}
	hr := func(sec int) float64 {
		if lowMinute(sec) && sec%60 >= 20 {
			return -1
		}

		return 140
	}

	return synthRide(90, power, hr)
}

func TestStreamDecouplingIQRFencesAfterPowerFilter(t *testing.T) {
	t.Parallel()
	got := mustDecouple(t, outlierRide(), fixed(icu.DefaultDecouplingIQRK))
	if got.Excluded[icu.DecouplingExcludedRatioOutlier] != 1 || got.Excluded[icu.DecouplingExcludedMissingHR] != 30 ||
		got.Excluded[icu.DecouplingExcludedCoasting] != 0 {
		t.Fatalf("excluded = %v, want 1 ratio_outlier, 30 missing_hr, no coasting", got.Excluded)
	}
	if got.RatioFenceHigh == nil || *got.RatioFenceHigh >= 162.0/140 {
		t.Fatalf("high fence = %v, want below 162/140", got.RatioFenceHigh)
	}
	// k is honored: at k=3 the 162 W minute is inside the fences.
	if wide := mustDecouple(t, outlierRide(), fixed(3)); wide.Excluded[icu.DecouplingExcludedRatioOutlier] != 0 {
		t.Fatalf("k=3 excluded = %v, want no ratio_outlier", wide.Excluded)
	}
	// k=0 disables screening.
	if off := mustDecouple(t, outlierRide(), fixed(0)); off.RatioFenceLow != nil || off.Excluded[icu.DecouplingExcludedRatioOutlier] != 0 {
		t.Fatalf("k=0: %+v", off)
	}
}

func TestStreamDecouplingCrossCorrelatesHRLag(t *testing.T) {
	t.Parallel()
	// 5-min square wave; HR follows power 30 s later.
	power := func(sec int) float64 {
		if (sec/300)%2 == 0 {
			return 150
		}

		return 250
	}
	hr := func(sec int) float64 { return 100 + 0.2*power(sec-30) }
	options := fixed(0)
	options.HRLagSeconds = nil
	got := mustDecouple(t, synthRide(90, power, hr), options)
	if got.HRLagMethod != icu.DecouplingLagCorrelation || got.HRLagSeconds != 30 {
		t.Fatalf("lag = %d (%s), want 30 cross_correlation", got.HRLagSeconds, got.HRLagMethod)
	}
	options.HRLagSeconds = new(5)
	if manual := mustDecouple(t, synthRide(90, power, hr), options); manual.HRLagSeconds != 5 || manual.HRLagMethod != icu.DecouplingLagManual {
		t.Fatalf("manual lag = %d %s", manual.HRLagSeconds, manual.HRLagMethod)
	}
}

func TestStreamDecouplingMissingHRRule(t *testing.T) {
	t.Parallel()
	// Minute 30 has HR for 20 of 60 s (missing_hr); minute 50 for 40 of 60
	// (kept).
	hr := func(sec int) float64 {
		minute, second := sec/60, sec%60
		if (minute == 30 && second >= 20) || (minute == 50 && second >= 40) {
			return -1
		}

		return 135
	}
	got := mustDecouple(t, synthRide(90, constant(150), hr), fixed(0))
	if got.Excluded[icu.DecouplingExcludedMissingHR] != 1 || len(got.ExcludedMinutes) != 1 || got.ExcludedMinutes[0].Minute != 30 {
		t.Fatalf("excluded = %v %v, want missing_hr at minute 30 only", got.Excluded, got.ExcludedMinutes)
	}
}

func TestStreamDecouplingGAPBasisAndStopSpeed(t *testing.T) {
	t.Parallel()
	streams := synthRide(60, constant(0), constant(150))
	delete(streams, "watts")
	n := 60 * 60
	gap := icu.NullableSeries{Values: make([]float64, n), Present: make([]bool, n)}
	speed := icu.NullableSeries{Values: make([]float64, n), Present: make([]bool, n)}
	for sec := range n {
		value := 3.0
		if sec >= 1800 && sec < 2100 { // 5-min stop
			value = 0.2
		}
		gap.Values[sec], gap.Present[sec] = value*1.02, true
		speed.Values[sec], speed.Present[sec] = value, true
	}
	streams["ga_velocity"], streams["velocity_smooth"] = gap, speed
	got := icu.ComputeStreamDecoupling(streams, fixed(0), icu.DecouplingContext{MovingTimeSeconds: n - 300})
	if got.Basis != icu.DecouplingBasisGAPHR || got.MovingSeconds != n-300 || got.StopSpeed == nil || *got.StopSpeed != 0.2 {
		t.Fatalf("got basis %s moving %d stop %v, want gap_hr %d 0.2", got.Basis, got.MovingSeconds, got.StopSpeed, n-300)
	}
	delete(streams, "ga_velocity")
	if pace := icu.ComputeStreamDecoupling(streams, fixed(0), icu.DecouplingContext{}); pace.Basis != icu.DecouplingBasisPaceHR || pace.StopSpeed != nil {
		t.Fatalf("pace basis = %s stop %v", pace.Basis, pace.StopSpeed)
	}
}

func TestStreamDecouplingNotMeasurable(t *testing.T) {
	t.Parallel()
	short := icu.ComputeStreamDecoupling(synthRide(6, constant(150), constant(135)), fixed(0), icu.DecouplingContext{})
	if short.Status != icu.DecouplingStatusNotMeasurable || short.Value != nil || short.Reason == "" {
		t.Fatalf("short = %+v", short)
	}
	streams := synthRide(90, constant(150), constant(135))
	delete(streams, "heartrate")
	if got := icu.ComputeStreamDecoupling(streams, fixed(0), icu.DecouplingContext{}); got.Status != icu.DecouplingStatusNotMeasurable {
		t.Fatalf("no HR = %+v", got)
	}
	options := fixed(0)
	options.WarmupSeconds = new(90 * 60)
	if got := icu.ComputeStreamDecoupling(synthRide(90, constant(150), constant(135)), options, icu.DecouplingContext{}); got.Value != nil {
		t.Fatalf("over-trimmed = %+v", got)
	}
}

func TestDecouplingVIGate(t *testing.T) {
	t.Parallel()
	if icu.DecouplingVICutoff([]float64{1.02, 1.05, 0}) != nil {
		t.Fatal("cutoff from < 4 rides")
	}
	cutoff := icu.DecouplingVICutoff([]float64{1.00, 1.02, 1.04, 1.06, 1.08})
	// Q1 1.02, Q3 1.06 -> 1.06 + 1.5*0.04 = 1.12.
	if cutoff == nil || math.Abs(*cutoff-1.12) > 1e-9 {
		t.Fatalf("cutoff = %v, want 1.12", cutoff)
	}
	result := mustDecouple(t, synthRide(90, constant(150), constant(135)), fixed(0))
	result.ApplyVIGate(1.10, cutoff, false)
	if result.Status != icu.DecouplingStatusMeasured || result.Value == nil {
		t.Fatalf("VI below cutoff: %s", result.Status)
	}
	result.ApplyVIGate(1.20, cutoff, true)
	if result.Status != icu.DecouplingStatusNotMeasurable || result.Reason == "" || result.Value != nil {
		t.Fatalf("VI above cutoff: %+v", result)
	}
	// No cutoff: measurable only when a paired steady workout vouches for it.
	planned := mustDecouple(t, synthRide(90, constant(150), constant(135)), fixed(0))
	planned.ApplyVIGate(1.6, nil, true)
	if planned.Status != icu.DecouplingStatusMeasured {
		t.Fatalf("planned steady without cutoff: %+v", planned)
	}
	unverified := mustDecouple(t, synthRide(90, constant(150), constant(135)), fixed(0))
	unverified.ApplyVIGate(1.0, nil, false)
	if unverified.Status != icu.DecouplingStatusNotMeasurable || unverified.Value != nil {
		t.Fatalf("unverified: %+v", unverified)
	}
}

func TestPlannedDecouplingTrim(t *testing.T) {
	t.Parallel()
	// i202545713's paired workout: 10' ramp 50-65%, 3x(5' 65, 5' 65,
	// 5' 67), 25' 62, 10' 55. Duration-weighted median target 62%.
	doc := &icu.WorkoutDoc{Steps: []icu.WorkoutStep{
		{Duration: 600, Ramp: true, Power: &icu.WorkoutTarget{Start: 50, End: 65}},
		{Reps: 3, Steps: []icu.WorkoutStep{
			{Duration: 300, Power: &icu.WorkoutTarget{Value: 65}},
			{Duration: 300, Power: &icu.WorkoutTarget{Value: 65}},
			{Duration: 300, Power: &icu.WorkoutTarget{Value: 67}},
		}},
		{Duration: 1500, Power: &icu.WorkoutTarget{Value: 62}},
		{Duration: 600, Power: &icu.WorkoutTarget{Value: 55}},
	}}
	trim, ok := icu.PlannedDecouplingTrim(doc)
	if !ok || trim.WarmupSeconds != 600 || trim.CooldownSeconds != 600 {
		t.Fatalf("trim = %+v %v, want 600/600", trim, ok)
	}
	labeled := &icu.WorkoutDoc{Steps: []icu.WorkoutStep{
		{Duration: 300, Text: "Warmup", Power: &icu.WorkoutTarget{Value: 70}},
		{Duration: 1200, Power: &icu.WorkoutTarget{Value: 70}},
		{Duration: 300, Text: "Cooldown", Power: &icu.WorkoutTarget{Value: 70}},
	}}
	if trim, ok := icu.PlannedDecouplingTrim(labeled); !ok || trim.WarmupSeconds != 300 || trim.CooldownSeconds != 300 {
		t.Fatalf("labeled trim = %+v", trim)
	}
	if _, ok := icu.PlannedDecouplingTrim(nil); ok {
		t.Fatal("nil doc")
	}
	allRamp := &icu.WorkoutDoc{Steps: []icu.WorkoutStep{{Duration: 600, Ramp: true, Power: &icu.WorkoutTarget{Start: 50, End: 70}}}}
	if _, ok := icu.PlannedDecouplingTrim(allRamp); ok {
		t.Fatal("all-ramp doc should not yield a trim")
	}
}

func TestActivityDecouplingSourceLabels(t *testing.T) {
	t.Parallel()
	zero := 0.0
	streamResult := icu.StreamDecoupling{Status: icu.DecouplingStatusMeasured, Value: &zero, Basis: icu.DecouplingBasisPowerHR}

	consumed := icu.Activity{Decoupling: 8.26}
	consumed.AttachStreamDecoupling(&streamResult, true)
	if !consumed.HasDecoupling() || consumed.Decoupling != 0 || consumed.DecouplingSource != icu.DecouplingSourceStreams ||
		consumed.DecouplingAPI == nil || *consumed.DecouplingAPI != 8.26 || consumed.DecouplingStreams == nil {
		t.Fatalf("consumed: %+v", consumed)
	}
	// A not-measured result never shows or consumes a stream value.
	gated := icu.Activity{Decoupling: 8.26}
	unmeasured := icu.StreamDecoupling{Status: icu.DecouplingStatusNotMeasurable, Value: &zero}
	gated.AttachStreamDecoupling(&unmeasured, true)
	if gated.Decoupling != 8.26 || gated.DecouplingSource != icu.DecouplingSourceAPI || gated.DecouplingStreams != nil {
		t.Fatalf("gated: %+v", gated)
	}
	reported := icu.Activity{Decoupling: 8.26}
	reported.AttachStreamDecoupling(&streamResult, false)
	if reported.Decoupling != 8.26 || reported.DecouplingSource != icu.DecouplingSourceAPI || *reported.DecouplingStreams != 0 {
		t.Fatalf("reported: %+v", reported)
	}
	// An API value of 0 is still labeled api (not skipped), with no API value.
	absent := icu.Activity{}
	absent.MarkAPIDecoupling("")
	if absent.DecouplingSource != icu.DecouplingSourceAPI || absent.DecouplingAPI != nil || absent.HasDecoupling() {
		t.Fatalf("absent: %+v", absent)
	}
}

func loadReferenceStreams(t *testing.T) icu.NullableStreamData {
	t.Helper()
	raw, err := os.ReadFile("testdata/streams_i202545713.json")
	if err != nil {
		t.Fatal(err)
	}
	var recorded []icu.ActivityStream
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatal(err)
	}
	streams, err := icu.PreserveNullableStreams(recorded)
	if err != nil {
		t.Fatal(err)
	}

	return streams
}

// TestStreamDecouplingReferenceRideI202545713 replays the recorded streams of
// i202545713 (Z2, 2026-10-27). It asserts structure only: the method is not
// tuned to any target number; the value is logged and reported in the PR.
func TestStreamDecouplingReferenceRideI202545713(t *testing.T) {
	t.Parallel()
	streams := loadReferenceStreams(t)
	plan := &icu.DecouplingPlanTrim{WarmupSeconds: 600, CooldownSeconds: 600}
	got := icu.ComputeStreamDecoupling(streams, icu.DefaultDecouplingOptions(), icu.DecouplingContext{MovingTimeSeconds: 6086, Plan: plan})
	if got.Status != icu.DecouplingStatusMeasured || got.Basis != icu.DecouplingBasisPowerHR {
		t.Fatalf("got %+v", got)
	}
	excluded := 0
	for _, count := range got.Excluded {
		excluded += count
	}
	if got.MinutesKept+excluded != got.MinutesInRange || math.Abs(float64(got.MovingSeconds-6086)) > 0.01*6086 { // ties at the stop speed
		t.Fatalf("kept %d + excluded %d != %d, moving %d", got.MinutesKept, excluded, got.MinutesInRange, got.MovingSeconds)
	}
	t.Logf("i202545713: %.2f%% trim %+v lag %d (%s) kept %d/%d excluded %v",
		*got.Value, got.Trim, got.HRLagSeconds, got.HRLagMethod, got.MinutesKept, got.MinutesInRange, got.Excluded)
}

func TestStreamDecouplingHRLagSearchIsBounded(t *testing.T) {
	t.Parallel()
	// HR follows power 150 s later, beyond the 120 s search bound.
	power := func(sec int) float64 {
		if (sec/600)%2 == 0 {
			return 150
		}

		return 250
	}
	hr := func(sec int) float64 { return 100 + 0.2*power(sec-150) }
	options := fixed(0)
	options.HRLagSeconds = nil
	got := mustDecouple(t, synthRide(90, power, hr), options)
	if got.HRLagSeconds != 120 {
		t.Fatalf("lag = %d, want clamped to the 120 s bound", got.HRLagSeconds)
	}
}

func TestStreamDecouplingHRLagNinetySeconds(t *testing.T) {
	t.Parallel()
	// A realistic ~90 s HR response, inside the search range: found, not clamped.
	power := func(sec int) float64 {
		if (sec/600)%2 == 0 {
			return 150
		}

		return 250
	}
	hr := func(sec int) float64 { return 100 + 0.2*power(sec-90) }
	options := fixed(0)
	options.HRLagSeconds = nil
	got := mustDecouple(t, synthRide(90, power, hr), options)
	if got.HRLagSeconds < 85 || got.HRLagSeconds > 95 || got.HRLagMethod != "cross_correlation" {
		t.Fatalf("lag = %d (%s), want ~90 s by cross-correlation", got.HRLagSeconds, got.HRLagMethod)
	}
}

func TestPlannedDecouplingWorkoutAthleteZonesAndFlatLadder(t *testing.T) {
	t.Parallel()
	step := func(seconds int, watts, cadence float64) map[string]any {
		out := map[string]any{"duration": seconds, "power": map[string]any{"value": watts, "units": "%ftp"}}
		if cadence > 0 {
			out["cadence"] = map[string]any{"value": cadence}
		}

		return out
	}
	ramp := map[string]any{"duration": 600, "ramp": true, "power": map[string]any{"start": 50, "end": 65, "units": "%ftp"}}
	// 62/68/72 % FTP: three raw targets, one athlete zone (Z2 56-75 %).
	sameZone := map[string]any{"steps": []any{ramp, step(1200, 62, 0), step(1200, 68, 0), step(1200, 72, 0), step(600, 50, 0)}}
	zones := []int{55, 75, 90, 105, 120, 150, 999}
	if _, kind, _ := icu.PlannedDecouplingWorkoutZones(sameZone, zones); kind != icu.DecouplingWorkoutSteady {
		t.Fatalf("athlete zones: %q, want steady", kind)
	}
	if _, kind, _ := icu.PlannedDecouplingWorkoutZones(sameZone, nil); kind != icu.DecouplingWorkoutIntervals {
		t.Fatalf("raw targets: %q, want intervals", kind)
	}
	// 62 vs 80 % FTP spans Z2/Z3 with athlete zones.
	twoZones := map[string]any{"steps": []any{ramp, step(1800, 62, 0), step(1800, 80, 0), step(600, 50, 0)}}
	if _, kind, _ := icu.PlannedDecouplingWorkoutZones(twoZones, zones); kind != icu.DecouplingWorkoutIntervals {
		t.Fatalf("two athlete zones: %q, want intervals", kind)
	}
	// Cadence ladder written as flat steps (no repeat block).
	flat := map[string]any{"steps": []any{
		ramp, step(600, 65, 90), step(600, 65, 100), step(600, 65, 70), step(600, 65, 90), step(600, 50, 0),
	}}
	if _, kind, _ := icu.PlannedDecouplingWorkoutZones(flat, zones); kind != icu.DecouplingWorkoutLadder {
		t.Fatalf("flat ladder: %q, want ladder", kind)
	}
}

func workoutRaw(steps ...map[string]any) map[string]any {
	list := make([]any, 0, len(steps))
	for _, step := range steps {
		list = append(list, step)
	}

	return map[string]any{
		"steps":     list,
		"zoneTimes": []any{map[string]any{"id": "Z1", "max": 55}, map[string]any{"id": "Z2", "max": 75}, map[string]any{"id": "SS", "max": 97}},
	}
}

func TestPlannedDecouplingWorkoutKinds(t *testing.T) {
	t.Parallel()
	ramp := map[string]any{"duration": 600, "ramp": true, "power": map[string]any{"start": 50, "end": 65, "units": "%ftp"}}
	cool := map[string]any{"duration": 600, "power": map[string]any{"value": 50, "units": "%ftp"}}
	steady := workoutRaw(ramp, map[string]any{"duration": 3600, "power": map[string]any{"value": 65, "units": "%ftp"}},
		map[string]any{"duration": 1200, "power": map[string]any{"value": 72, "units": "%ftp"}}, cool)
	waves := workoutRaw(ramp, map[string]any{"reps": 6, "steps": []any{
		map[string]any{"duration": 150, "power": map[string]any{"value": 72, "units": "%ftp"}},
		map[string]any{"duration": 35, "power": map[string]any{"value": 54, "units": "%ftp"}},
	}}, cool)
	ladder := workoutRaw(ramp, map[string]any{"reps": 3, "steps": []any{
		map[string]any{"duration": 300, "power": map[string]any{"value": 65, "units": "%ftp"}, "cadence": map[string]any{"value": 90}},
		map[string]any{"duration": 300, "power": map[string]any{"value": 65, "units": "%ftp"}, "cadence": map[string]any{"value": 100}},
	}}, cool)
	zones := map[string]any{"steps": []any{
		map[string]any{"duration": 600, "power": map[string]any{"value": 1, "units": "power_zone"}},
		map[string]any{"duration": 4500, "power": map[string]any{"value": 2, "units": "power_zone"}},
		map[string]any{"duration": 300, "power": map[string]any{"value": 1, "units": "power_zone"}},
	}}
	for name, tc := range map[string]struct {
		raw  any
		want string
	}{
		"steady": {steady, icu.DecouplingWorkoutSteady}, "waves": {waves, icu.DecouplingWorkoutIntervals},
		"ladder": {ladder, icu.DecouplingWorkoutLadder}, "power_zone": {zones, icu.DecouplingWorkoutSteady},
	} {
		if _, kind, ok := icu.PlannedDecouplingWorkout(tc.raw); !ok || kind != tc.want {
			t.Fatalf("%s: kind %q ok %v, want %q", name, kind, ok, tc.want)
		}
	}
	if _, _, ok := icu.PlannedDecouplingWorkout(nil); ok {
		t.Fatal("nil doc")
	}
	result := mustDecouple(t, synthRide(90, constant(150), constant(135)), fixed(0))
	result.ApplyWorkoutGate(icu.DecouplingWorkoutLadder)
	if result.Status != icu.DecouplingStatusNotMeasurable || result.Value != nil || result.WorkoutKind != icu.DecouplingWorkoutLadder {
		t.Fatalf("ladder gate: %+v", result)
	}
}
