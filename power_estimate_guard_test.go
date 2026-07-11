package icu

import "testing"

func TestPercentileSortedEdges(t *testing.T) {
	t.Parallel()

	if percentileSorted(nil, 0.5) != 0 {
		t.Fatal("empty")
	}
	values := []float64{10, 20, 30, 40, 50}
	if percentileSorted(values, 0) != 10 {
		t.Fatalf("p0=%v", percentileSorted(values, 0))
	}
	if percentileSorted(values, 1) != 50 {
		t.Fatalf("p1=%v", percentileSorted(values, 1))
	}
	if percentileSorted(values, 0.5) != 30 {
		t.Fatalf("p50=%v", percentileSorted(values, 0.5))
	}
}

func TestSmoothSpeedSeriesEmpty(t *testing.T) {
	t.Parallel()

	if smoothSpeedSeries(nil, 2) != nil {
		t.Fatal("nil in")
	}
	in := []float64{1, 2, 3}
	if got := smoothSpeedSeries(in, 0); got[1] != 2 {
		t.Fatalf("zero window should copy, got %v", got)
	}
}

func TestApplyLinearCalibrationAndCap(t *testing.T) {
	t.Parallel()

	filled := []float64{100, 500, 200, 0, 400}
	sources := []string{"measured", "estimated_physics", "estimated_physics", "estimated_physics", "estimated"}
	applyLinearCalibrationAndCap(filled, sources, 0.5, 10, 150)
	if filled[0] != 100 {
		t.Fatalf("measured changed %v", filled[0])
	}
	// 0.5*500+10=260 → cap 150
	if filled[1] != 150 {
		t.Fatalf("cap failed %v", filled[1])
	}
	// 0.5*200+10=110
	if filled[2] != 110 {
		t.Fatalf("linear failed %v", filled[2])
	}
	// freewheel zero preserved
	if filled[3] != 0 {
		t.Fatalf("zero overwritten %v", filled[3])
	}
	// estimated (non-physics) only capped
	if filled[4] != 150 {
		t.Fatalf("estimated cap %v", filled[4])
	}
	// gain <= 0 resets to 1
	filled2 := []float64{100}
	sources2 := []string{"estimated_physics"}
	applyLinearCalibrationAndCap(filled2, sources2, -1, 5, 0)
	if filled2[0] != 105 { // 1*100+5
		t.Fatalf("gain reset %v", filled2[0])
	}
	// negative result clamped
	filled3 := []float64{10}
	sources3 := []string{"estimated_physics"}
	applyLinearCalibrationAndCap(filled3, sources3, 1, -50, 0)
	if filled3[0] != 0 {
		t.Fatalf("neg clamp %v", filled3[0])
	}
}

func TestScaleRollingWindowsMajorityEstimated(t *testing.T) {
	t.Parallel()

	// 100 samples, first 20 measured 100W, rest estimated 400W.
	n := 100
	filled := make([]float64, n)
	sources := make([]string, n)
	times := make([]float64, n)
	for i := range n {
		times[i] = float64(i)
		if i < 20 {
			filled[i] = 100
			sources[i] = PowerSampleMeasured
			continue
		}
		filled[i] = 400
		sources[i] = "estimated"
	}
	timeSeries := denseNullable(times)
	// 30s window mean 400 must scale toward 120.
	if !scaleRollingWindows(filled, sources, timeSeries, 30, 120) {
		t.Fatal("expected scaling")
	}
	// Estimated region should be reduced.
	if filled[50] >= 400 {
		t.Fatalf("estimated not scaled: %v", filled[50])
	}
	if filled[0] != 100 {
		t.Fatalf("measured changed: %v", filled[0])
	}
}
