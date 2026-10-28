package icu_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// synthRide builds 1 Hz streams of the given length in minutes. power(sec)
// and hr(sec) return the sample at second sec.
func synthRide(minutes int, power, hr func(sec int) float64) icu.NullableStreamData {
	n := minutes * 60
	timeS := icu.NullableSeries{Values: make([]float64, n), Present: make([]bool, n)}
	watts := icu.NullableSeries{Values: make([]float64, n), Present: make([]bool, n)}
	heart := icu.NullableSeries{Values: make([]float64, n), Present: make([]bool, n)}
	for sec := range n {
		timeS.Values[sec], timeS.Present[sec] = float64(sec), true
		watts.Values[sec], watts.Present[sec] = power(sec), true
		heart.Values[sec], heart.Present[sec] = hr(sec), true
	}

	return icu.NullableStreamData{"time": timeS, "watts": watts, "heartrate": heart}
}

func constant(value float64) func(int) float64 { return func(int) float64 { return value } }

func mustDecouple(t *testing.T, streams icu.NullableStreamData, options icu.DecouplingOptions) icu.StreamDecoupling {
	t.Helper()
	got, err := icu.ComputeStreamDecoupling(streams, options)
	if err != nil {
		t.Fatalf("ComputeStreamDecoupling: %v", err)
	}

	return got
}

func TestStreamDecouplingSteadyRideIsZero(t *testing.T) {
	t.Parallel()
	got := mustDecouple(t, synthRide(90, constant(150), constant(135)), icu.DefaultDecouplingOptions())
	if got.Value != 0 || got.Basis != icu.DecouplingBasisPowerHR {
		t.Fatalf("got %+v, want 0 power_hr", got)
	}
	if got.RangeStartMinute != 20 || got.RangeEndMinute != 78 || got.MinutesKept != 58 {
		t.Fatalf("range %d-%d kept %d, want 20-78 kept 58", got.RangeStartMinute, got.RangeEndMinute, got.MinutesKept)
	}
}

func TestStreamDecouplingDetectsHRDrift(t *testing.T) {
	t.Parallel()
	// HR 5% higher in the second half at equal power: 1 - 1/1.05 = 4.76%.
	hr := func(sec int) float64 {
		if sec >= 49*60+23 { // second half starts at min 49; HR read 23 s later
			return 140 * 1.05
		}

		return 140
	}
	got := mustDecouple(t, synthRide(90, constant(150), hr), icu.DefaultDecouplingOptions())
	if math.Abs(got.Value-4.76) > 0.01 {
		t.Fatalf("decoupling = %v, want 4.76", got.Value)
	}
}

func TestStreamDecouplingTrimsWarmupAndCooldown(t *testing.T) {
	t.Parallel()
	// Warm-up at a very different ratio must not leak into the first half.
	power := func(sec int) float64 {
		if sec < 20*60 || sec >= 78*60 {
			return 300
		}

		return 150
	}
	got := mustDecouple(t, synthRide(90, power, constant(135)), icu.DefaultDecouplingOptions())
	if got.Value != 0 {
		t.Fatalf("decoupling = %v, want 0 (warm-up/cool-down trimmed)", got.Value)
	}
}

func TestStreamDecouplingExcludesCoastingMinutes(t *testing.T) {
	t.Parallel()
	// Second half has 6 minutes of coasting (0-20 W) with HR still elevated;
	// unfiltered this reads as large drift.
	power := func(sec int) float64 {
		minute := sec / 60
		if minute >= 60 && minute < 66 {
			return 20
		}

		return 150
	}
	options := icu.DefaultDecouplingOptions()
	got := mustDecouple(t, synthRide(90, power, constant(135)), options)
	if got.Value != 0 || got.Excluded[icu.DecouplingExcludedLowPower] != 6 || got.MinutesKept != 52 {
		t.Fatalf("got value %v excluded %v kept %d, want 0 / low_power 6 / 52", got.Value, got.Excluded, got.MinutesKept)
	}

	options.MinPowerWatts = 0
	options.IQRK = 0
	unfiltered := mustDecouple(t, synthRide(90, power, constant(135)), options)
	if unfiltered.Value < 5 {
		t.Fatalf("unfiltered decoupling = %v, want large positive drift from coasting", unfiltered.Value)
	}
}

func TestStreamDecouplingExcludesRatioOutliers(t *testing.T) {
	t.Parallel()
	// Two sprint minutes (600 W) in the second half: IQR-flagged.
	power := func(sec int) float64 {
		minute := sec / 60
		if minute == 55 || minute == 70 {
			return 600
		}

		return 150
	}
	options := icu.DefaultDecouplingOptions()
	got := mustDecouple(t, synthRide(90, power, constant(135)), options)
	if got.Value != 0 || got.Excluded[icu.DecouplingExcludedRatioOutlier] != 2 {
		t.Fatalf("got value %v excluded %v, want 0 / ratio_outlier 2", got.Value, got.Excluded)
	}
	if got.RatioFenceLow == nil || got.RatioFenceHigh == nil {
		t.Fatal("fences missing")
	}

	options.IQRK = 0
	kept := mustDecouple(t, synthRide(90, power, constant(135)), options)
	if kept.Value >= 0 || kept.Excluded[icu.DecouplingExcludedRatioOutlier] != 0 || kept.RatioFenceLow != nil {
		t.Fatalf("k=0: got %+v, want negative value, no outliers, no fences", kept)
	}
}

func TestStreamDecouplingAppliesHRLag(t *testing.T) {
	t.Parallel()
	// Power alternates every 90 s; HR is proportional to power 23 s earlier.
	power := func(sec int) float64 {
		if (sec/90)%2 == 0 {
			return 120
		}

		return 200
	}
	hr := func(sec int) float64 {
		if sec < 23 {
			return power(0) * 0.7
		}

		return power(sec-23) * 0.7
	}
	options := icu.DefaultDecouplingOptions()
	options.IQRK = 0
	got := mustDecouple(t, synthRide(90, power, hr), options)
	if math.Abs(got.Value) > 0.001 {
		t.Fatalf("lag 23: decoupling = %v, want 0", got.Value)
	}
	options.HRLagSeconds = 0
	noLag := mustDecouple(t, synthRide(90, power, hr), options)
	if noLag.Value == got.Value {
		t.Fatalf("lag 0 should differ from lag 23 (both %v)", got.Value)
	}
}

func TestStreamDecouplingDropsMissingSamples(t *testing.T) {
	t.Parallel()
	streams := synthRide(90, constant(150), constant(135))
	heart := streams["heartrate"]
	for sec := 40 * 60; sec < 45*60; sec++ { // 5 min HR dropout
		heart.Present[sec] = false
	}
	got := mustDecouple(t, streams, icu.DefaultDecouplingOptions())
	if got.Value != 0 || got.Excluded[icu.DecouplingExcludedMissingHR] < 4 {
		t.Fatalf("got value %v excluded %v, want 0 and missing_hr minutes", got.Value, got.Excluded)
	}
}

func TestStreamDecouplingPaceBasisWithoutPower(t *testing.T) {
	t.Parallel()
	streams := synthRide(90, constant(0), constant(150))
	delete(streams, "watts")
	speed := icu.NullableSeries{Values: make([]float64, 90*60), Present: make([]bool, 90*60)}
	for sec := range speed.Values {
		speed.Values[sec], speed.Present[sec] = 3.2, true
	}
	streams["velocity_smooth"] = speed
	got := mustDecouple(t, streams, icu.DefaultDecouplingOptions())
	if got.Basis != icu.DecouplingBasisPaceHR || got.Value != 0 {
		t.Fatalf("got %+v, want pace_hr 0", got)
	}
}

func TestStreamDecouplingInsufficientData(t *testing.T) {
	t.Parallel()
	if _, err := icu.ComputeStreamDecoupling(synthRide(40, constant(150), constant(135)), icu.DefaultDecouplingOptions()); !errors.Is(err, icu.ErrDecouplingInsufficientStreams) {
		t.Fatalf("short ride err = %v, want insufficient", err)
	}
	streams := synthRide(90, constant(150), constant(135))
	delete(streams, "heartrate")
	if _, err := icu.ComputeStreamDecoupling(streams, icu.DefaultDecouplingOptions()); !errors.Is(err, icu.ErrDecouplingInsufficientStreams) {
		t.Fatalf("no HR err = %v, want insufficient", err)
	}
}

func TestActivityDecouplingSourceLabels(t *testing.T) {
	t.Parallel()
	activity := icu.Activity{Decoupling: 8.26}
	activity.ApplyStreamDecoupling(&icu.StreamDecoupling{Value: 0, Basis: icu.DecouplingBasisPowerHR})
	if !activity.HasDecoupling() || activity.Decoupling != 0 || activity.DecouplingSource != icu.DecouplingSourceStreams ||
		activity.DecouplingAPI == nil || *activity.DecouplingAPI != 8.26 {
		t.Fatalf("stream apply: %+v", activity)
	}
	fallback := icu.Activity{Decoupling: 4.1}
	fallback.MarkAPIDecouplingFallback("streams fetch failed")
	if fallback.DecouplingSource != icu.DecouplingSourceAPI || fallback.Decoupling != 4.1 || fallback.DecouplingNote == "" {
		t.Fatalf("fallback: %+v", fallback)
	}
}

// TestStreamDecouplingReferenceRideI202545713 replays the recorded streams of
// i202545713 (Z2, 2026-10-27; intervals.icu decoupling field: 8.26%).
// Reference from B15: range min 20-95, HR lag 23 s, <50 W + IQR 1.5 ->
// 71 of 75 minutes kept, ~2.7% (this implementation: 2.89%).
func TestStreamDecouplingReferenceRideI202545713(t *testing.T) {
	t.Parallel()
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

	got := mustDecouple(t, streams, icu.DefaultDecouplingOptions())
	if got.RangeStartMinute != 20 || got.RangeEndMinute != 95 || got.MinutesInRange != 75 || got.MinutesKept != 71 {
		t.Fatalf("range %d-%d in %d kept %d, want 20-95 / 75 / 71",
			got.RangeStartMinute, got.RangeEndMinute, got.MinutesInRange, got.MinutesKept)
	}
	if got.Excluded[icu.DecouplingExcludedLowPower] != 4 {
		t.Fatalf("excluded = %v, want low_power 4", got.Excluded)
	}
	if got.Value < 2.4 || got.Value > 3.2 {
		t.Fatalf("decoupling = %v, want ~2.7 (2.4-3.2)", got.Value)
	}

	// Same range without the coasting filter/lag/IQR reproduces the ~7%
	// that is close to intervals.icu's 8.26%: the formula was never the bug.
	options := icu.DefaultDecouplingOptions()
	options.MinPowerWatts, options.HRLagSeconds, options.IQRK = 0, 0, 0
	unfiltered := mustDecouple(t, streams, options)
	if unfiltered.Value < 6 || unfiltered.Value > 8 {
		t.Fatalf("unfiltered decoupling = %v, want ~7", unfiltered.Value)
	}
}
