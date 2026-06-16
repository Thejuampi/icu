package icu

import (
	"encoding/json"
	"testing"
)

func TestFlexIntUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    flexInt
		wantErr bool
	}{
		{"number", `42`, 42, false},
		{"string_number", `"42"`, 42, false},
		{"empty", ``, 0, false},
		{"string_invalid", `"abc"`, 0, false},
		{"invalid_json", `{`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got flexInt

			err := got.UnmarshalJSON([]byte(tt.input))

			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFlexFloatUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    flexFloat
		wantErr bool
	}{
		{"number", `3.14`, 3.14, false},
		{"string_number", `"3.14"`, 3.14, false},
		{"empty", ``, 0, false},
		{"string_invalid", `"abc"`, 0, false},
		{"invalid_json", `{`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got flexFloat

			err := got.UnmarshalJSON([]byte(tt.input))

			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAtoiOrZero(t *testing.T) {
	t.Parallel()

	if got := atoiOrZero("42"); got != 42 {
		t.Fatalf("atoiOrZero(42) = %d, want 42", got)
	}

	if got := atoiOrZero("abc"); got != 0 {
		t.Fatalf("atoiOrZero(abc) = %d, want 0", got)
	}

	if got := atoiOrZero(""); got != 0 {
		t.Fatalf("atoiOrZero() = %d, want 0", got)
	}
}

func TestParseDateToSecondsUTC(t *testing.T) {
	t.Parallel()

	sec, err := parseDateToSecondsUTC("2026-06-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sec != 1780272000 {
		t.Fatalf("seconds = %d, want 1780272000", sec)
	}

	if _, err := parseDateToSecondsUTC("invalid"); err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestParseDateEndOfDaySecondsUTC(t *testing.T) {
	t.Parallel()

	sec, err := parseDateEndOfDaySecondsUTC("2026-06-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const want = 1780272000 + 24*60*60 - 1
	if sec != want {
		t.Fatalf("seconds = %d, want %d", sec, want)
	}

	if _, err := parseDateEndOfDaySecondsUTC("invalid"); err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestDecodeSleepWithInvalidDate(t *testing.T) {
	t.Parallel()

	_, err := decodeSleep(json.RawMessage(`{"st":1,"ed":2}`), "invalid-date")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestDecodeStepsWithInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := decodeSteps(json.RawMessage(`{`))
	if err == nil {
		t.Fatalf("expected error for invalid json")
	}
}

func TestZeppAnyToFloat(t *testing.T) {
	t.Parallel()

	if got := zeppAnyToFloat(float64(42.5)); got != 42.5 {
		t.Fatalf("number = %f, want 42.5", got)
	}

	if got := zeppAnyToFloat("3.14"); got != 3.14 {
		t.Fatalf("string number = %f, want 3.14", got)
	}

	if got := zeppAnyToFloat("not-a-number"); got != 0 {
		t.Fatalf("invalid string = %f, want 0", got)
	}
}
