package icu_test

import (
	"math"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestAveragePower(t *testing.T) {
	t.Parallel()

	watts := []float64{100, 200, 300, 400, 500}
	got := icu.AveragePower(watts, 0, len(watts))

	var want float64 = 300
	if got != want {
		t.Fatalf("AveragePower = %v, want %v", got, want)
	}
}

func TestAveragePowerSubrange(t *testing.T) {
	t.Parallel()

	watts := []float64{100, 200, 300, 400, 500}
	got := icu.AveragePower(watts, 1, 3)

	var want float64 = 250
	if got != want {
		t.Fatalf("AveragePower subrange = %v, want %v", got, want)
	}
}

func TestAveragePowerEmptySlice(t *testing.T) {
	t.Parallel()

	var watts []float64
	got := icu.AveragePower(watts, 0, 0)

	if got != 0 {
		t.Fatalf("AveragePower empty = %v, want 0", got)
	}
}

func TestAverageHeartRate(t *testing.T) {
	t.Parallel()

	hr := []float64{120, 130, 140, 150, 160}
	got := icu.AverageHeartRate(hr, 0, len(hr))

	var want float64 = 140
	if got != want {
		t.Fatalf("AverageHeartRate = %v, want %v", got, want)
	}
}

func TestNormalizedPower(t *testing.T) {
	t.Parallel()

	watts := []float64{200, 200, 200, 200, 200}
	got := icu.NormalizedPower(watts, 0, len(watts))

	var want float64 = 200
	if math.Abs(got-want) > 1 {
		t.Fatalf("NormalizedPower constant = %v, want %v", got, want)
	}
}

func TestNormalizedPowerVariable(t *testing.T) {
	t.Parallel()

	watts := make([]float64, 60)
	for i := range watts {
		if i < 30 {
			watts[i] = 100
		} else {
			watts[i] = 300
		}
	}
	got := icu.NormalizedPower(watts, 0, len(watts))

	if got <= 200 {
		t.Fatalf("NormalizedPower variable = %v, want > 200 (variable power raises NP)", got)
	}
}

func TestNormalizedPowerEmpty(t *testing.T) {
	t.Parallel()

	var watts []float64
	got := icu.NormalizedPower(watts, 0, 0)

	if got != 0 {
		t.Fatalf("NormalizedPower empty = %v, want 0", got)
	}
}

func TestMaxValue(t *testing.T) {
	t.Parallel()

	data := []float64{1.5, 3.2, 0.7, 4.9, 2.1}
	got := icu.MaxValue(data)

	want := 4.9
	if got != want {
		t.Fatalf("MaxValue = %v, want %v", got, want)
	}
}

func TestMaxValueEmpty(t *testing.T) {
	t.Parallel()

	var data []float64
	got := icu.MaxValue(data)

	if got != 0 {
		t.Fatalf("MaxValue empty = %v, want 0", got)
	}
}

func TestHeartRateDriftIncreasing(t *testing.T) {
	t.Parallel()

	hr := make([]float64, 120)
	for i := range hr {
		hr[i] = 130 + float64(i)
	}
	got := icu.HeartRateDrift(hr, 60)

	if got <= 0 {
		t.Fatalf("HeartRateDrift increasing = %v, want positive", got)
	}
}

func TestHeartRateDriftFlat(t *testing.T) {
	t.Parallel()

	hr := make([]float64, 120)
	for i := range hr {
		hr[i] = 140
	}
	got := icu.HeartRateDrift(hr, 60)

	if math.Abs(got) > 1 {
		t.Fatalf("HeartRateDrift flat = %v, want ~0", got)
	}
}

func TestHeartRateDriftDecreasing(t *testing.T) {
	t.Parallel()

	hr := make([]float64, 120)
	for i := range hr {
		hr[i] = 150 - float64(i)
	}
	got := icu.HeartRateDrift(hr, 60)

	if got >= 0 {
		t.Fatalf("HeartRateDrift decreasing = %v, want negative", got)
	}
}

func TestHeartRateDriftShortSlice(t *testing.T) {
	t.Parallel()

	hr := []float64{130, 140}
	got := icu.HeartRateDrift(hr, 1)

	if got <= 0 {
		t.Fatalf("HeartRateDrift short = %v, want positive (slope of 10 bpm per step * 60)", got)
	}
}

func TestPowerHRRatio(t *testing.T) {
	t.Parallel()

	watts := []float64{200, 200, 200}
	hr := []float64{100, 120, 140}
	got := icu.PowerHRRatio(watts, hr, 0, 3)

	want := 200.0 / 120.0
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("PowerHRRatio = %v, want %v", got, want)
	}
}

func TestPowerHRRatioEmpty(t *testing.T) {
	t.Parallel()

	var watts, hr []float64
	got := icu.PowerHRRatio(watts, hr, 0, 0)

	if got != 0 {
		t.Fatalf("PowerHRRatio empty = %v, want 0", got)
	}
}

func TestDecoupling(t *testing.T) {
	t.Parallel()

	watts := []float64{200, 200, 200, 200}
	hr := []float64{120, 130, 140, 150}
	got := icu.Decoupling(watts, hr)

	if got <= 0 {
		t.Fatalf("Decoupling rising HR = %v, want positive", got)
	}
}

func TestDecouplingUsesFirstHalfReference(t *testing.T) {
	t.Parallel()

	// Constant power, two equal halves of HR so EF1 and EF2 are exact.
	// watts=200 for all; half1 HR avg=125 → EF1=1.6; half2 HR avg=145 → EF2≈1.379
	watts := []float64{200, 200, 200, 200}
	hr := []float64{120, 130, 140, 150}
	got := icu.Decoupling(watts, hr)

	firstEF := 200.0 / 125.0
	secondEF := 200.0 / 145.0
	want := (1 - secondEF/firstEF) * 100
	if math.Abs(got-want) > 0.05 {
		t.Fatalf("Decoupling = %v, want Friel formula %v", got, want)
	}
}

func TestDecouplingFlat(t *testing.T) {
	t.Parallel()

	watts := []float64{200, 200, 200, 200}
	hr := []float64{140, 140, 140, 140}
	got := icu.Decoupling(watts, hr)

	if math.Abs(got) > 0.05 {
		t.Fatalf("Decoupling flat = %v, want ~0", got)
	}
}

func TestDecouplingShort(t *testing.T) {
	t.Parallel()

	watts := []float64{200}
	hr := []float64{140}
	got := icu.Decoupling(watts, hr)

	if got != 0 {
		t.Fatalf("Decoupling short = %v, want 0", got)
	}
}

func TestTimeInZone(t *testing.T) {
	t.Parallel()

	watts := []float64{100, 150, 200, 250, 300}
	ftp := 250

	gotZ2 := icu.TimeInZone(watts, ftp, 56, 75)
	if gotZ2 != 1 {
		t.Fatalf("Z2 count = %d, want 1 (only 150 is 56-75%% of %d, 200=80%%)", gotZ2, ftp)
	}
}

func TestTimeInZoneEmpty(t *testing.T) {
	t.Parallel()

	var watts []float64
	got := icu.TimeInZone(watts, 250, 56, 75)

	if got != 0 {
		t.Fatalf("TimeInZone empty = %d, want 0", got)
	}
}

func TestHeartRateRecovery(t *testing.T) {
	t.Parallel()

	hr := []float64{120, 130, 140, 150, 160, 170, 180, 175, 170, 165, 160, 155}

	// Peak 180 at index 6; 5 samples later is 155 → total drop 25 bpm (not rate 5).
	got := icu.HeartRateRecovery(hr, 6, 5)
	if math.Abs(got-25) > 0.01 {
		t.Fatalf("HeartRateRecovery = %v, want total drop 25", got)
	}
}

func TestHeartRateRecoveryShortWindow(t *testing.T) {
	t.Parallel()

	hr := []float64{180, 170}
	got := icu.HeartRateRecovery(hr, 0, 1)

	if math.Abs(got-10) > 1 {
		t.Fatalf("HeartRateRecovery = %v, want ~10 (drops from 180 to 170 in 1 step)", got)
	}
}

func TestHeartRateRecoveryOneMinuteTotalDrop(t *testing.T) {
	t.Parallel()

	hr := make([]float64, 61)
	hr[0] = 180
	for i := 1; i < len(hr); i++ {
		hr[i] = 180 - float64(i) // linear 1 bpm/s drop
	}
	got := icu.HeartRateRecovery(hr, 0, 60)
	if math.Abs(got-60) > 0.01 {
		t.Fatalf("HeartRateRecovery 60s = %v, want total drop 60 (not 1.0 rate)", got)
	}
}

func TestSlope(t *testing.T) {
	t.Parallel()

	data := []float64{1, 2, 3, 4, 5}
	got := icu.Slope(data)

	var want float64 = 1
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("Slope = %v, want %v", got, want)
	}
}

func TestSlopeFlat(t *testing.T) {
	t.Parallel()

	data := []float64{5, 5, 5, 5, 5}
	got := icu.Slope(data)

	if math.Abs(got) > 0.01 {
		t.Fatalf("Slope flat = %v, want 0", got)
	}
}

func TestSlopeShort(t *testing.T) {
	t.Parallel()

	data := []float64{42}
	got := icu.Slope(data)

	if got != 0 {
		t.Fatalf("Slope single = %v, want 0", got)
	}
}

func TestVariance(t *testing.T) {
	t.Parallel()

	data := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	got := icu.Variance(data)

	if got <= 0 {
		t.Fatalf("Variance = %v, want positive", got)
	}
}

func TestVarianceConstant(t *testing.T) {
	t.Parallel()

	data := []float64{5, 5, 5, 5}
	got := icu.Variance(data)

	if got != 0 {
		t.Fatalf("Variance constant = %v, want 0", got)
	}
}

func TestHeartRateToLTHR(t *testing.T) {
	t.Parallel()

	got := icu.HeartRateToLTHR(140, 200)

	var want float64 = 70
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("HeartRateToLTHR = %v, want %v", got, want)
	}
}

func TestHeartRateToLTHRZero(t *testing.T) {
	t.Parallel()

	got := icu.HeartRateToLTHR(140, 0)

	if got != 0 {
		t.Fatalf("HeartRateToLTHR zero LTHR = %v, want 0", got)
	}
}

func TestHeartRateDriftEmpty(t *testing.T) {
	t.Parallel()

	var hr []float64
	got := icu.HeartRateDrift(hr, 60)

	if got != 0 {
		t.Fatalf("HeartRateDrift empty = %v, want 0", got)
	}
}
