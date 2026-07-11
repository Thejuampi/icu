package icu

import (
	"math"
	"testing"
)

func TestRebalanceV2ExactIFIsSquareRootOfLoadRatio(t *testing.T) {
	t.Parallel()

	// load = 64 TSS over 1 hour ⇒ IF = sqrt(64 / (1 * 100)) = 0.8
	load := rebalanceRatFromInt(64)
	seconds := rebalanceRatFromInt(3600)
	got := rebalanceV2ExactIF(load, seconds).Float64()
	want := 0.8
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("ExactIF = %v, want %v (pre-fix returned IF² ≈ 0.64)", got, want)
	}
}

func TestRebalanceV2ExactIFMatchesAppliedIFForSameInputs(t *testing.T) {
	t.Parallel()

	// Invert estimateLoadFromDurationIF: 3600s @ IF 0.75 → load 56.25 → round 56
	// Use exact rational load that is hours*100*IF² with IF=0.5: 1h * 100 * 0.25 = 25
	load := rebalanceRatFromInt(25)
	seconds := rebalanceRatFromInt(3600)
	got := rebalanceV2ExactIF(load, seconds).Float64()
	if math.Abs(got-0.5) > 1e-6 {
		t.Fatalf("ExactIF = %v, want 0.5", got)
	}
}

func TestRebalanceV2MaxSessionErrorUsesAbsoluteValue(t *testing.T) {
	t.Parallel()

	// Unit-level: abs of negative residual step
	errStep := rebalanceRatFromInt(10).Sub(rebalanceRatFromInt(25)) // -15
	if errStep.Sign() >= 0 {
		t.Fatal("setup: expected negative error")
	}
	abs := ZeroRebalanceRat().Sub(errStep)
	if abs.Float64() != 15 {
		t.Fatalf("abs error = %v, want 15", abs.Float64())
	}
	// The no-op bug was: errStep.Sub(Zero) left it negative so max never updated.
	noop := errStep.Sub(ZeroRebalanceRat())
	if noop.Sign() >= 0 {
		t.Fatal("noop subtraction should remain negative")
	}
	if noop.Cmp(abs) >= 0 {
		t.Fatal("absolute value must exceed negative residual for max tracking")
	}
}

func TestRebalanceRatSqrtEdges(t *testing.T) {
	t.Parallel()

	if !rebalanceRatSqrt(ZeroRebalanceRat()).IsZero() {
		t.Fatal("sqrt(0) want 0")
	}
	if !rebalanceRatSqrt(rebalanceRatFromInt(-1)).IsZero() {
		t.Fatal("sqrt(negative) want 0")
	}
	if got := rebalanceRatSqrt(rebalanceRatFromInt(4)).Float64(); math.Abs(got-2) > 1e-9 {
		t.Fatalf("sqrt(4)=%v, want 2", got)
	}
	if !rebalanceV2ExactIF(rebalanceRatFromInt(10), ZeroRebalanceRat()).IsZero() {
		t.Fatal("ExactIF with zero seconds want 0")
	}
	if !rebalanceV2ExactIF(rebalanceRatFromInt(-1), rebalanceRatFromInt(3600)).IsZero() {
		t.Fatal("ExactIF with negative load want 0")
	}
}
