package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func newEnvelopeRat() icu.RebalanceEnvelope {
	l, _ := icu.ParseRebalanceRat("200")
	c, _ := icu.ParseRebalanceRat("300")
	h, _ := icu.ParseRebalanceRat("420")
	return icu.RebalanceEnvelope{Low: l, Current: c, High: h}
}

func ratLevel(level string) icu.RebalanceRat {
	r, _ := icu.ParseRebalanceRat(level)
	return r
}

func TestRebalancePCHIPAnchorLow(t *testing.T) {
	t.Parallel()

	curve := icu.NewRebalancePCHIP(newEnvelopeRat())
	if !curve.Evaluate(ratLevel("0")).Equal(ratLevel("200")) {
		t.Fatalf("level 0 should equal low anchor")
	}
}

func TestRebalancePCHIPAnchorCurrentExact(t *testing.T) {
	t.Parallel()

	curve := icu.NewRebalancePCHIP(newEnvelopeRat())
	if !curve.Evaluate(ratLevel("0.5")).Equal(ratLevel("300")) {
		t.Fatalf("level 0.5 should equal current anchor exactly")
	}
}

func TestRebalancePCHIPAnchorHigh(t *testing.T) {
	t.Parallel()

	curve := icu.NewRebalancePCHIP(newEnvelopeRat())
	if !curve.Evaluate(ratLevel("1")).Equal(ratLevel("420")) {
		t.Fatalf("level 1 should equal high anchor")
	}
}

func TestRebalancePCHAINoShootLow(t *testing.T) {
	t.Parallel()

	curve := icu.NewRebalancePCHIP(newEnvelopeRat())
	out := curve.Evaluate(ratLevel("-0.25"))
	if out.Cmp(ratLevel("200")) < 0 {
		t.Fatalf("level below range should not undershoot low: %s", out.DecimalString())
	}
}

func TestRebalancePCHIPNoOvershootIncreasing(t *testing.T) {
	t.Parallel()

	curve := icu.NewRebalancePCHIP(newEnvelopeRat())
	for _, level := range []string{"0", "0.1", "0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8", "0.9", "1"} {
		out := curve.Evaluate(ratLevel(level))
		if out.Cmp(ratLevel("200")) < 0 || out.Cmp(ratLevel("420")) > 0 {
			t.Fatalf("level %s overshoots envelope: %s", level, out.DecimalString())
		}
	}
}

func TestRebalancePCHIPMonotonicIncreasing(t *testing.T) {
	t.Parallel()

	curve := icu.NewRebalancePCHIP(newEnvelopeRat())
	levels := []string{"0", "0.1", "0.25", "0.4", "0.5", "0.65", "0.8", "1"}
	var prev icu.RebalanceRat
	for index, level := range levels {
		out := curve.Evaluate(ratLevel(level))
		if index > 0 && out.Cmp(prev) <= 0 {
			t.Fatalf("not strictly increasing at %s: prev %s out %s", level, prev.DecimalString(), out.DecimalString())
		}
		prev = out
	}
}

func TestRebalancePCHIPCurrentIsNoOpForSliderHalf(t *testing.T) {
	t.Parallel()

	envelope := newEnvelopeRat()
	curve := icu.NewRebalancePCHIP(envelope)
	delta := curve.Evaluate(ratLevel("0.5")).Sub(envelope.Current)
	if !delta.IsZero() {
		t.Fatalf("level 0.5 must be exact no-op, delta = %s", delta.DecimalString())
	}
}

func TestRebalancePCHIPDerivativeContinuousAtHalf(t *testing.T) {
	t.Parallel()

	curve := icu.NewRebalancePCHIP(newEnvelopeRat())
	eps, _ := icu.ParseRebalanceRat("0.0001")
	half := ratLevel("0.5")
	left := curve.Evaluate(half.Sub(eps)).Float64()
	right := curve.Evaluate(half.Add(eps)).Float64()
	center := curve.Evaluate(half).Float64()
	leftSlope := (center - left) / eps.Float64()
	rightSlope := (right - center) / eps.Float64()
	avg := (absFloat(leftSlope) + absFloat(rightSlope)) / 2
	if avg > 0 && absFloat(leftSlope-rightSlope)/avg > 1e-3 {
		t.Fatalf("derivative discontinuous at 0.5: left %f right %f", leftSlope, rightSlope)
	}
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
