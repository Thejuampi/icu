package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestDetectRebalanceRegimeBlocksSparseHistory(t *testing.T) {
	t.Parallel()

	if _, ok := icu.DetectRebalanceRegime([]float64{100, 120, 90}); ok {
		t.Fatalf("sparse history should block regime detection")
	}
}

func TestDetectRebalanceRegimeStableSingleRegime(t *testing.T) {
	t.Parallel()

	samples := []float64{50, 52, 49, 51, 50, 48, 51, 49, 50, 52}
	regime, ok := icu.DetectRebalanceRegime(samples)
	if !ok {
		t.Fatalf("stable history should detect a regime")
	}
	if len(regime.ChangePoints) != 0 {
		t.Fatalf("stable history should not split, got %v", regime.ChangePoints)
	}
	if regime.Coverage != 1.0 {
		t.Fatalf("coverage = %f, want 1.0", regime.Coverage)
	}
	if regime.Confidence < 0.8 {
		t.Fatalf("confidence = %f, want >= 0.8", regime.Confidence)
	}
}

func TestDetectRebalanceRegimeDetectsBaselineShift(t *testing.T) {
	t.Parallel()

	samples := []float64{100, 98, 101, 99, 100, 300, 298, 302, 299, 301}
	regime, ok := icu.DetectRebalanceRegime(samples)
	if !ok {
		t.Fatalf("shifted history should detect a regime")
	}
	if len(regime.ChangePoints) != 1 || regime.ChangePoints[0] != 5 {
		t.Fatalf("change points = %v, want [5]", regime.ChangePoints)
	}
	if regime.Median < 298 || regime.Median > 301 {
		t.Fatalf("current regime median = %f, want ~300", regime.Median)
	}
	if regime.Coverage != 0.5 {
		t.Fatalf("coverage = %f, want 0.5", regime.Coverage)
	}
}

func TestDetectRebalanceRegimeUsesMostRecentRegime(t *testing.T) {
	t.Parallel()

	samples := []float64{100, 101, 99, 100, 200, 199, 201, 200, 300, 299, 301, 300}
	regime, ok := icu.DetectRebalanceRegime(samples)
	if !ok {
		t.Fatalf("should detect a regime")
	}
	if regime.Median < 298 || regime.Median > 302 {
		t.Fatalf("most recent regime median = %f, want ~300", regime.Median)
	}
	if regime.StartIndex != 8 {
		t.Fatalf("start index = %d, want 8", regime.StartIndex)
	}
}

func TestDetectRebalanceRegimeRampDoesNotFragmentIntoTinySegments(t *testing.T) {
	t.Parallel()

	samples := []float64{100, 120, 140, 160, 180, 200, 220, 240, 260, 280}
	regime, ok := icu.DetectRebalanceRegime(samples)
	if !ok {
		t.Fatalf("ramp history should detect a regime")
	}
	for index := 1; index < len(regime.ChangePoints); index++ {
		if regime.ChangePoints[index]-regime.ChangePoints[index-1] < 2 {
			t.Fatalf("ramp fragmented into tiny segments: %v", regime.ChangePoints)
		}
	}
	if regime.Confidence < 0 || regime.Confidence > 1 {
		t.Fatalf("confidence out of range: %f", regime.Confidence)
	}
}

func TestDetectRebalanceRegimeIgnoresOldOutlierInCurrentRegime(t *testing.T) {
	t.Parallel()

	samples := []float64{9000, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	regime, ok := icu.DetectRebalanceRegime(samples)
	if !ok {
		t.Fatalf("should detect a regime")
	}
	if regime.Median > 200 {
		t.Fatalf("current regime median polluted by old outlier: %f", regime.Median)
	}
}
