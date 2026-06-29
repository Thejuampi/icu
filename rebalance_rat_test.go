package icu_test

import (
	"encoding/json"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestParseRebalanceRatAcceptsHalf(t *testing.T) {
	t.Parallel()

	got, err := icu.ParseRebalanceRat("0.5")
	if err != nil {
		t.Fatalf("parse 0.5: %v", err)
	}
	if !got.Equal(icu.NewRebalanceRatFromInt(1, 2)) {
		t.Fatalf("0.5 != 1/2")
	}
}

func TestParseRebalanceRatRejectsFractionSyntax(t *testing.T) {
	t.Parallel()

	if _, err := icu.ParseRebalanceRat("1/2"); err == nil {
		t.Fatalf("fraction syntax should be rejected, decimal-only input")
	}
}

func TestParseRebalanceRatRejectsNonDecimal(t *testing.T) {
	t.Parallel()

	cases := []string{"", "abc", "1.2.3", "1e3", "  ", "0.5x", "--1"}
	for _, in := range cases {
		if _, err := icu.ParseRebalanceRat(in); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}

func TestRebalanceRatAddExactDecimal(t *testing.T) {
	t.Parallel()

	one, _ := icu.ParseRebalanceRat("0.1")
	two, _ := icu.ParseRebalanceRat("0.2")
	three, _ := icu.ParseRebalanceRat("0.3")
	if !one.Add(two).Equal(three) {
		t.Fatalf("0.1 + 0.2 != 0.3 (exact)")
	}
}

func TestRebalanceRatMulExact(t *testing.T) {
	t.Parallel()

	a, _ := icu.ParseRebalanceRat("1.5")
	b, _ := icu.ParseRebalanceRat("2")
	if !a.Mul(b).Equal(icu.NewRebalanceRatFromInt(3, 1)) {
		t.Fatalf("1.5 * 2 != 3")
	}
}

func TestRebalanceRatQuoExact(t *testing.T) {
	t.Parallel()

	a, _ := icu.ParseRebalanceRat("0.6")
	b, _ := icu.ParseRebalanceRat("2")
	if !a.Quo(b).Equal(icu.NewRebalanceRatFromInt(3, 10)) {
		t.Fatalf("0.6 / 2 != 0.3")
	}
}

func TestRebalanceRatCmpOrdering(t *testing.T) {
	t.Parallel()

	half, _ := icu.ParseRebalanceRat("0.5")
	quarter, _ := icu.ParseRebalanceRat("0.25")
	if got := half.Cmp(quarter); got != 1 {
		t.Fatalf("0.5 cmp 0.25 = %d, want 1", got)
	}
}

func TestRebalanceRatIsZero(t *testing.T) {
	t.Parallel()

	if !icu.ZeroRebalanceRat().IsZero() {
		t.Fatalf("zero rat should be zero")
	}
	half, _ := icu.ParseRebalanceRat("0.5")
	if half.IsZero() {
		t.Fatalf("0.5 should not be zero")
	}
}

func TestRebalanceRatDecimalStringTrimsTrailingZeros(t *testing.T) {
	t.Parallel()

	v, _ := icu.ParseRebalanceRat("1.500000")
	if got := v.DecimalString(); got != "1.5" {
		t.Fatalf("decimal string = %q, want 1.5", got)
	}
}

func TestRebalanceRatDecimalStringExactTerminating(t *testing.T) {
	t.Parallel()

	if got := icu.NewRebalanceRatFromInt(1, 4).DecimalString(); got != "0.25" {
		t.Fatalf("1/4 decimal = %q, want 0.25", got)
	}
}

func TestRebalanceRatDecimalStringHighPrecisionNonTerminating(t *testing.T) {
	t.Parallel()

	got := icu.NewRebalanceRatFromInt(1, 3).DecimalString()
	if len(got) < 35 || !startsWith(got, "0.3333") {
		t.Fatalf("1/3 decimal = %q, want >= 35 digits starting 0.3333", got)
	}
}

func TestRebalanceRatStringMatchesDecimalString(t *testing.T) {
	t.Parallel()

	value := icu.NewRebalanceRatFromInt(7, 4)
	if value.String() != value.DecimalString() {
		t.Fatalf("string = %q, decimal = %q", value.String(), value.DecimalString())
	}
}

func TestRebalanceRatJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := icu.NewRebalanceRatFromInt(7, 4)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `"1.75"` {
		t.Fatalf("marshal = %s, want %q", data, `"1.75"`)
	}
	var decoded icu.RebalanceRat
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !decoded.Equal(original) {
		t.Fatalf("decoded = %s, want %s", decoded.DecimalString(), original.DecimalString())
	}
}

func startsWith(value, prefix string) bool {
	return len(value) >= len(prefix) && value[:len(prefix)] == prefix
}
