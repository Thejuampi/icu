package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func seriesFrom(values []float64, present []bool) icu.NullableSeries {
	if present == nil {
		present = make([]bool, len(values))
		for index := range present {
			present[index] = true
		}
	}

	return icu.NullableSeries{Values: append([]float64(nil), values...), Present: append([]bool(nil), present...)}
}

func TestClassifyPowerSamplesCoastingIsTrueZero(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom([]float64{0, 0, 0}, nil),
		Cadence: seriesFrom([]float64{0, 0, 0}, nil),
		Speed:   seriesFrom([]float64{5, 5, 5}, nil),
	})
	if got.TrueZeroSeconds != 3 {
		t.Fatalf("trueZero=%d, want 3; labels=%v", got.TrueZeroSeconds, got.Labels)
	}
	if got.MissingSeconds != 0 {
		t.Fatalf("missing=%d, want 0", got.MissingSeconds)
	}
}

func TestPromoteAmbiguousAfterDeathViaClassify(t *testing.T) {
	t.Parallel()

	// Meter death: measured first, then zeros without cadence while moving —
	// ambiguous post-death samples must promote to missing.
	const n = 80
	watts := make([]float64, n)
	wp := make([]bool, n)
	cad := make([]float64, n)
	cp := make([]bool, n)
	speed := make([]float64, n)
	for i := range n {
		speed[i] = 8
		if i < 40 {
			watts[i] = 200
			wp[i] = true
			cad[i] = 90
			cp[i] = true
			continue
		}
		// Device zeros after death without cadence: missing or ambiguous→missing.
		watts[i] = 0
		wp[i] = true
		cp[i] = false
	}
	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom(watts, wp),
		Cadence: seriesFrom(cad, cp),
		Speed:   seriesFrom(speed, nil),
	})
	if got.MeterDeathIndex == nil || *got.MeterDeathIndex < 0 {
		t.Fatalf("expected meter death, got %+v", got)
	}
	if got.MissingSeconds < 20 {
		t.Fatalf("missing=%d want post-death promotion", got.MissingSeconds)
	}
}

func TestClassifyPowerSamplesNullCadenceMovingIsMissing(t *testing.T) {
	t.Parallel()

	cadPresent := []bool{false, false, false}
	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom([]float64{0, 0, 0}, nil),
		Cadence: seriesFrom([]float64{0, 0, 0}, cadPresent),
		Speed:   seriesFrom([]float64{8, 8, 8}, nil),
	})
	if got.MissingSeconds != 3 {
		t.Fatalf("missing=%d, want 3; labels=%v reasons=%v", got.MissingSeconds, got.Labels, got.Reasons)
	}
}

func TestClassifyPowerSamplesPositiveWattsMeasured(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom([]float64{200, 180}, nil),
		Cadence: seriesFrom([]float64{90, 85}, nil),
		Speed:   seriesFrom([]float64{8, 8}, nil),
	})
	if got.MeasuredSeconds != 2 {
		t.Fatalf("measured=%d, want 2", got.MeasuredSeconds)
	}
}

func TestClassifyPowerSamplesStoppedNullCadenceIsTrueZero(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom([]float64{0}, nil),
		Cadence: seriesFrom([]float64{0}, []bool{false}),
		Speed:   seriesFrom([]float64{0.2}, nil),
	})
	if got.TrueZeroSeconds != 1 {
		t.Fatalf("trueZero=%d labels=%v, want 1 true_zero", got.TrueZeroSeconds, got.Labels)
	}
}

func TestClassifyPowerSamplesMeterDeathFixture(t *testing.T) {
	t.Parallel()

	const sampleCount = 200
	watts, cadValues, cadPresent, speed := meterDeathFixture(sampleCount)

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom(watts, nil),
		Cadence: seriesFrom(cadValues, cadPresent),
		Speed:   seriesFrom(speed, nil),
	})
	assertMeterDeathShape(t, &got, watts, cadValues, cadPresent, sampleCount)
}

//nolint:gocritic // fixture tuple
func meterDeathFixture(sampleCount int) ([]float64, []float64, []bool, []float64) {
	watts := make([]float64, sampleCount)
	cadValues := make([]float64, sampleCount)
	cadPresent := make([]bool, sampleCount)
	speed := make([]float64, sampleCount)
	half := sampleCount / 2
	for index := range sampleCount {
		speed[index] = 7
		if index < half {
			watts[index] = 200
			cadValues[index] = 80
			cadPresent[index] = true
			if index%20 == 0 {
				watts[index] = 0
				cadValues[index] = 0
			}
			continue
		}
		watts[index] = 0
		cadPresent[index] = false
	}

	return watts, cadValues, cadPresent, speed
}

func assertMeterDeathShape(t *testing.T, got *icu.PowerGapClassification, watts, cadValues []float64, cadPresent []bool, sampleCount int) {
	t.Helper()
	if got.MeterDeathIndex == nil {
		t.Fatal("expected meterDeathIndex")
	}
	half := sampleCount / 2
	if *got.MeterDeathIndex < half-10 || *got.MeterDeathIndex > half+5 {
		t.Fatalf("meterDeathIndex=%d, want near %d", *got.MeterDeathIndex, half)
	}
	if got.MissingSeconds < half-10 {
		t.Fatalf("missing=%d, want >=%d in dead half", got.MissingSeconds, half-10)
	}
	if got.TrueZeroSeconds < 5 {
		t.Fatalf("trueZero=%d, want some coasting zeros preserved", got.TrueZeroSeconds)
	}
	for index := range half {
		if watts[index] == 0 && cadPresent[index] && cadValues[index] == 0 && got.Labels[index] != icu.PowerSampleTrueZero {
			t.Fatalf("index %d coasting labeled %s, want true_zero", index, got.Labels[index])
		}
	}
	for index := half; index < sampleCount; index++ {
		if got.Labels[index] != icu.PowerSampleMissing {
			t.Fatalf("index %d dead half labeled %s, want missing", index, got.Labels[index])
		}
	}
}

func TestClassifyPowerSamplesNullWattsPositiveCadenceIsMissing(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom([]float64{0}, []bool{false}),
		Cadence: seriesFrom([]float64{90}, nil),
		Speed:   seriesFrom([]float64{6}, nil),
	})
	if got.MissingSeconds != 1 {
		t.Fatalf("missing=%d labels=%v, want 1", got.MissingSeconds, got.Labels)
	}
}

func TestClassifyPowerSamplesNilInputs(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(nil)
	if len(got.Warnings) == 0 {
		t.Fatal("expected warning for nil inputs")
	}
}

func TestClassifyPowerSamplesSpeedFromDistance(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:    seriesFrom([]float64{0, 0}, nil),
		Cadence:  seriesFrom([]float64{0, 0}, []bool{false, false}),
		Distance: seriesFrom([]float64{0, 10}, nil),
		Time:     seriesFrom([]float64{0, 1}, nil),
	})
	if got.MissingSeconds != 1 {
		// index 0 may be stopped; index 1 should be moving at 10 m/s
		t.Fatalf("missing=%d labels=%v", got.MissingSeconds, got.Labels)
	}
}

func TestClassifyPowerSamplesZeroWattsPositiveCadenceMeasured(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom([]float64{0}, nil),
		Cadence: seriesFrom([]float64{90}, nil),
		Speed:   seriesFrom([]float64{5}, nil),
	})
	if got.MeasuredSeconds != 1 {
		t.Fatalf("measured=%d labels=%v", got.MeasuredSeconds, got.Labels)
	}
}

func TestClassifyPowerSamplesNoCadenceStreamWarning(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts: seriesFrom([]float64{0, 100}, nil),
		Speed: seriesFrom([]float64{5, 5}, nil),
	})
	if len(got.Warnings) == 0 {
		t.Fatal("expected cadence absent warning")
	}
}

func TestClassifyPowerSamplesNullWattsZeroCadenceTrueZero(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom([]float64{0}, []bool{false}),
		Cadence: seriesFrom([]float64{0}, nil),
		Speed:   seriesFrom([]float64{5}, nil),
	})
	if got.TrueZeroSeconds != 1 {
		t.Fatalf("trueZero=%d labels=%v", got.TrueZeroSeconds, got.Labels)
	}
}

func TestClassifyPowerSamplesEmptyInputs(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{})
	if len(got.Warnings) == 0 {
		t.Fatal("expected empty warning")
	}
}

func TestClassifyPowerSamplesSegmentsPresent(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom([]float64{100, 100, 0, 0}, nil),
		Cadence: seriesFrom([]float64{90, 90, 0, 0}, nil),
		Speed:   seriesFrom([]float64{5, 5, 5, 5}, nil),
	})
	if len(got.Segments) < 2 {
		t.Fatalf("segments=%d", len(got.Segments))
	}
}

func TestNoCadenceMovingIsMissing(t *testing.T) {
	t.Parallel()

	// No cadence stream + null watts while moving → missing (PM-death style).
	wattsPresent := []bool{true, true, false, false, false}
	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts: seriesFrom([]float64{200, 180, 0, 0, 0}, wattsPresent),
		Speed: seriesFrom([]float64{8, 8, 8, 8, 8}, nil),
	})
	if got.MissingSeconds < 3 {
		t.Fatalf("missing=%d labels=%v", got.MissingSeconds, got.Labels)
	}
}

func TestDetectCadenceDeathIndexEndAnchoredTail(t *testing.T) {
	t.Parallel()

	// 50 present, then 80 null cadence to end → death at 50.
	n := 130
	cad := make([]float64, n)
	present := make([]bool, n)
	for i := range 50 {
		cad[i] = 90
		present[i] = true
	}
	got := icu.DetectCadenceDeathIndex(seriesFrom(cad, present))
	if got != 50 {
		t.Fatalf("death=%d want 50", got)
	}
}

func TestDetectCadenceDeathIgnoresMidRideFreewheel(t *testing.T) {
	t.Parallel()

	// Null cadence in the middle that returns is not death.
	n := 120
	cad := make([]float64, n)
	present := make([]bool, n)
	for i := range n {
		cad[i] = 90
		present[i] = true
	}
	for i := 40; i < 80; i++ {
		present[i] = false
	}
	if got := icu.DetectCadenceDeathIndex(seriesFrom(cad, present)); got != -1 {
		t.Fatalf("mid freewheel should not be death, got %d", got)
	}
}

func TestDetectBalanceDeathIndexLastPresentPlusTail(t *testing.T) {
	t.Parallel()

	// Real dual-sided first half, null forever after (classic PM battery death).
	n := 200
	bal := make([]float64, n)
	present := make([]bool, n)
	for i := range 100 {
		bal[i] = 48 + float64(i%5) // ~50% left
		present[i] = true
	}
	// brief mid null then return is not the last present
	// last present is 99 → death 100
	got := icu.DetectBalanceDeathIndex(seriesFrom(bal, present))
	if got != 100 {
		t.Fatalf("death=%d want 100", got)
	}
}

func TestDetectBalanceDeathIgnoresStartNullsAndBriefCoasts(t *testing.T) {
	t.Parallel()

	n := 150
	bal := make([]float64, n)
	present := make([]bool, n)
	// Start null (warmup)
	for i := 10; i < 140; i++ {
		bal[i] = 50
		present[i] = true
	}
	// Brief coast 60-70 but balance returns → last present near end
	for i := 60; i < 70; i++ {
		present[i] = false
	}
	// Only 10 null at end (< 30) → no death
	got := icu.DetectBalanceDeathIndex(seriesFrom(bal, present))
	if got != -1 {
		t.Fatalf("short end null should not be death, got %d", got)
	}
}

func TestDetectPowerMeterDeathPrefersBalanceOverCadence(t *testing.T) {
	t.Parallel()

	n := 120
	bal := make([]float64, n)
	balP := make([]bool, n)
	cad := make([]float64, n)
	cadP := make([]bool, n)
	for i := range 50 {
		bal[i] = 50
		balP[i] = true
		cad[i] = 90
		cadP[i] = true
	}
	// Cadence dies later than balance
	for i := 50; i < 70; i++ {
		cad[i] = 90
		cadP[i] = true
	}
	death, src := icu.DetectPowerMeterDeathIndex(seriesFrom(bal, balP), seriesFrom(cad, cadP))
	if death != 50 || src != icu.PowerDeathSourceBalance {
		t.Fatalf("death=%d src=%s want 50/%s", death, src, icu.PowerDeathSourceBalance)
	}
}

func TestClassifyUsesBalanceDeathForMovingZeros(t *testing.T) {
	t.Parallel()

	n := 100
	watts := make([]float64, n)
	cad := make([]float64, n)
	bal := make([]float64, n)
	speed := make([]float64, n)
	wP := make([]bool, n)
	cP := make([]bool, n)
	bP := make([]bool, n)
	for i := range n {
		speed[i] = 8
		if i < 40 {
			watts[i] = 200
			cad[i] = 90
			bal[i] = 50
			wP[i], cP[i], bP[i] = true, true, true
			continue
		}
		// After balance death: zero watts, null cad, null balance → missing
		watts[i] = 0
		wP[i] = true
		cP[i] = false
		bP[i] = false
	}
	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom(watts, wP),
		Cadence: seriesFrom(cad, cP),
		Balance: seriesFrom(bal, bP),
		Speed:   seriesFrom(speed, nil),
	})
	if got.MeterDeathIndex == nil || *got.MeterDeathIndex != 40 {
		t.Fatalf("death=%v want 40", got.MeterDeathIndex)
	}
	if got.DeathSource != icu.PowerDeathSourceBalance {
		t.Fatalf("src=%s", got.DeathSource)
	}
	if got.MissingSeconds < 50 {
		t.Fatalf("missing=%d labels last=%v", got.MissingSeconds, got.Labels[90:])
	}
	// First half remains measured.
	if got.Labels[0] != icu.PowerSampleMeasured {
		t.Fatalf("first=%s", got.Labels[0])
	}
}

func TestClassifyPriorFillAfterBalanceDeathStaysMeasured(t *testing.T) {
	t.Parallel()

	// After death, positive watts (prior fill) stay measured; need refill mask to reopen.
	n := 80
	watts := make([]float64, n)
	bal := make([]float64, n)
	speed := make([]float64, n)
	wP := make([]bool, n)
	bP := make([]bool, n)
	for i := range n {
		speed[i] = 7
		watts[i] = 180
		wP[i] = true
		if i < 35 {
			bal[i] = 51
			bP[i] = true
		}
	}
	got := icu.ClassifyPowerSamples(&icu.PowerGapInputs{
		Watts:   seriesFrom(watts, wP),
		Balance: seriesFrom(bal, bP),
		Speed:   seriesFrom(speed, nil),
	})
	if got.MeterDeathIndex == nil {
		t.Fatal("expected balance death")
	}
	// Sample after death with positive watts still measured.
	if got.Labels[50] != icu.PowerSampleMeasured {
		t.Fatalf("prior fill should stay measured, got %s", got.Labels[50])
	}
}

func TestMaskStreamsAsPowerMeterDeathFrom(t *testing.T) {
	t.Parallel()

	streams := icu.NullableStreamData{
		"watts":              seriesFrom([]float64{200, 180, 160, 150}, nil),
		"cadence":            seriesFrom([]float64{90, 90, 90, 90}, nil),
		"left_right_balance": seriesFrom([]float64{50, 50, 50, 50}, nil),
		"time":               seriesFrom([]float64{0, 1, 2, 3}, nil),
	}
	masked := icu.MaskStreamsAsPowerMeterDeathFrom(streams, 2)
	// First half preserved.
	if w, ok := icu.NullableStream(masked, "watts").At(0); !ok || w != 200 {
		t.Fatalf("watts0 changed")
	}
	// From index 2: watts present 0, cadence/balance absent.
	if w, ok := icu.NullableStream(masked, "watts").At(2); !ok || w != 0 {
		t.Fatalf("watts2=%v ok=%v", w, ok)
	}
	if _, ok := icu.NullableStream(masked, "cadence").At(2); ok {
		t.Fatal("cadence2 should be absent")
	}
	if _, ok := icu.NullableStream(masked, "left_right_balance").At(2); ok {
		t.Fatal("balance2 should be absent")
	}
	// Original not mutated.
	if w, _ := icu.NullableStream(streams, "watts").At(2); w != 160 {
		t.Fatal("original mutated")
	}
}
