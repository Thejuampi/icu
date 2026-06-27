package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func stableRegime(samples []float64) icu.RebalanceRegime {
	regime, _ := icu.DetectRebalanceRegime(samples)
	return regime
}

func TestBuildRebalanceEnvelopeBlocksMissingRegime(t *testing.T) {
	t.Parallel()

	if _, ok := icu.BuildRebalanceEnvelope(&icu.RebalanceRegime{}, 300, 0, 1, 1); ok {
		t.Fatalf("missing regime should block envelope")
	}
}

func TestBuildRebalanceEnvelopeWithinFence(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{280, 290, 300, 310, 320, 300, 305, 295, 315, 300})
	report, ok := icu.BuildRebalanceEnvelope(&regime, 300, 0, 6, 6)
	if !ok {
		t.Fatalf("within-fence envelope should build")
	}
	if report.OutsideEnvelope {
		t.Fatalf("current within fence reported outside")
	}
	if report.Envelope.Low.Cmp(report.Envelope.Current) >= 0 {
		t.Fatalf("low should be below current")
	}
	if report.Envelope.High.Cmp(report.Envelope.Current) <= 0 {
		t.Fatalf("high should be above current")
	}
}

func TestBuildRebalanceEnvelopeCurrentAboveHighIsOutside(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{100, 110, 105, 108, 102, 109, 104, 107, 103, 106})
	report, ok := icu.BuildRebalanceEnvelope(&regime, 600, 0, 1, 1)
	if !ok {
		t.Fatalf("envelope should still build when current is outside")
	}
	if !report.OutsideEnvelope {
		t.Fatalf("current far above high should be outside envelope")
	}
}

func TestBuildRebalanceEnvelopeCurrentBelowLowIsOutside(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{200, 210, 205, 208, 202, 209, 204, 207, 203, 206})
	report, ok := icu.BuildRebalanceEnvelope(&regime, 10, 0, 1, 1)
	if !ok {
		t.Fatalf("envelope should still build when current is outside")
	}
	if !report.OutsideEnvelope {
		t.Fatalf("current far below low should be outside envelope")
	}
}

func TestBuildRebalanceEnvelopePhysiologicalCeilingCapsHigh(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{400, 410, 390, 420, 380, 405, 395, 415, 385, 980})
	report, ok := icu.BuildRebalanceEnvelope(&regime, 395, 400, 1, 1)
	if !ok {
		t.Fatalf("ceiling envelope should build")
	}
	if report.Envelope.High.Cmp(report.Envelope.Current) <= 0 {
		t.Fatalf("high should remain above current even when capped")
	}
	ceiling, _ := icu.NewRebalanceRatFromDyadic(400)
	if report.Envelope.High.Cmp(ceiling) > 0 {
		t.Fatalf("high should not exceed physiological ceiling, got %s", report.Envelope.High.DecimalString())
	}
	if report.HighSource == "" || report.HighSource == "data_robust_fence" {
		t.Fatalf("high source should note ceiling cap, got %q", report.HighSource)
	}
}

func TestBuildRebalanceEnvelopeMissingMetricsReduceConfidence(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{280, 290, 300, 310, 320, 300, 305, 295, 315, 300})
	full, _ := icu.BuildRebalanceEnvelope(&regime, 300, 0, 6, 6)
	partial, _ := icu.BuildRebalanceEnvelope(&regime, 300, 0, 2, 6)
	if partial.Confidence >= full.Confidence {
		t.Fatalf("partial metrics confidence %f should be below full %f", partial.Confidence, full.Confidence)
	}
}

func TestBuildRebalanceEnvelopeLowNeverBelowZero(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{50, 55, 50, 52, 50, 53, 50, 51, 50, 54})
	report, ok := icu.BuildRebalanceEnvelope(&regime, 52, 0, 1, 1)
	if !ok {
		t.Fatalf("envelope should build")
	}
	zero := icu.ZeroRebalanceRat()
	if report.Envelope.Low.Cmp(zero) < 0 {
		t.Fatalf("low below zero: %s", report.Envelope.Low.DecimalString())
	}
}
