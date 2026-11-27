package icu

import (
	"testing"
	"time"
)

func TestDecodeSleepAdvancesDayWhenStartGoesBack(t *testing.T) {
	t.Parallel()

	// The first stage ends exactly at midnight (no wrap); the next one
	// starts earlier on the clock, so it is on the next day.
	raw := `{"stage":[{"start":1380,"stop":1439,"mode":4},{"start":10,"stop":100,"mode":5},{"start":100,"stop":200,"mode":4}]}`

	sleep, err := decodeSleep([]byte(raw), "2026-06-01")
	if err != nil {
		t.Fatalf("decodeSleep: %v", err)
	}

	next := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC).Unix()
	if got := sleep.Stages[1].StartEpoch; got != next+10*60 {
		t.Fatalf("stage 1 start = %d, want %d", got, next+10*60)
	}

	if got := sleep.Stages[2].StartEpoch; got != next+100*60 {
		t.Fatalf("stage 2 start = %d, want %d (no double advance)", got, next+100*60)
	}
}

func TestZeppItemLocation(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"":                 "2026-06-01",
		"America/New_York": "2026-05-31",
		"-5":               "2026-05-31",
		"+05:30":           "2026-06-01",
		"-03:00":           "2026-05-31",
		"not/a-zone":       "2026-06-01",
		"99":               "2026-06-01",
	}

	for name, want := range cases {
		if got := at.In(zeppItemLocation(name)).Format(time.DateOnly); got != want {
			t.Fatalf("%q: date = %s, want %s", name, got, want)
		}
	}
}

func TestZeppTrackIDStringDecodesStringNumberAndNull(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{`"17"`: "17", `17`: "17", `null`: "", ``: ""} {
		var got zeppTrackIDString
		if err := got.UnmarshalJSON([]byte(raw)); err != nil || string(got) != want {
			t.Fatalf("%q: got %q, err %v", raw, got, err)
		}
	}

	var bad zeppTrackIDString
	if err := bad.UnmarshalJSON([]byte(`{}`)); err == nil {
		t.Fatalf("expected an error for an object")
	}

	if err := bad.UnmarshalJSON([]byte(`"x`)); err == nil {
		t.Fatalf("expected an error for a broken string")
	}
}
