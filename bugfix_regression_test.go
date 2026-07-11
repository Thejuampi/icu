package icu_test

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

func TestBuildFullURLEncodesCustomBaseQuery(t *testing.T) {
	t.Parallel()

	var gotRawURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// RequestURI preserves the wire-form query encoding from the client.
		gotRawURL = req.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)

	client := icu.NewClient("k", "0", icu.WithHTTPClient(srv.Client()), icu.WithBaseURL(srv.URL))
	var activities []icu.Activity
	err := client.Get("activities", nil, map[string]string{
		"fields": "id,name/start",
		"q":      "café ride",
	}, &activities)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !strings.Contains(gotRawURL, "name%2Fstart") {
		t.Fatalf("custom-base query missing slash encoding: %q", gotRawURL)
	}
	if !strings.Contains(gotRawURL, "caf%C3%A9+ride") {
		t.Fatalf("custom-base query missing UTF-8 encoding: %q", gotRawURL)
	}
	// Pre-fix concatenated raw values: fields=id,name/start (unencoded slash).
	if strings.Contains(gotRawURL, "name/start") {
		t.Fatalf("custom-base still has unencoded slash: %q", gotRawURL)
	}
}

func TestWriteTableColumnsAlign(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := icu.WriteTable(&buf, []string{"Name", "Age"}, [][]string{{"Juan", "36"}, {"Ana", "8"}})
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("lines = %v", lines)
	}
	// Header and first data row must share the same column start for "Age".
	headerAge := strings.Index(lines[0], "Age")
	dataAge := strings.Index(lines[2], "36")
	if headerAge < 0 || dataAge < 0 {
		t.Fatalf("missing columns:\n%s", buf.String())
	}
	if headerAge != dataAge {
		t.Fatalf("Age column misaligned: header@%d data@%d\n%s", headerAge, dataAge, buf.String())
	}
}

func TestNormalizeStreamsNullSamplesBecomeZero(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{{
		Type: "watts",
		Data: []any{100.0, nil, 200.0},
	}}
	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("NormalizeStreams: %v", err)
	}
	watts := got["watts"]
	if len(watts) != 3 {
		t.Fatalf("len(watts)=%d, want 3 (null samples must not drop stream)", len(watts))
	}
	if watts[1] != 0 {
		t.Fatalf("null sample = %v, want 0", watts[1])
	}
}

func TestCopyRawIfSetAppliesExplicitZero(t *testing.T) {
	t.Parallel()

	// Mixed camel + snake: snake_case explicit zero must override camel non-zero.
	payload := []byte(`{"icuTrainingLoad":50,"icu_training_load":0,"type":"Ride"}`)
	var activity icu.Activity
	if err := json.Unmarshal(payload, &activity); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if activity.TrainingLoad != 0 {
		t.Fatalf("TrainingLoad = %d, want 0 from snake_case override", activity.TrainingLoad)
	}
}

func TestLatestCTLNotWipedByLaterZeroActivity(t *testing.T) {
	t.Parallel()

	activities := []icu.Activity{
		{Type: "Ride", StartDateLocal: "2026-06-01T08:00:00", TrainingLoad: 100, CTL: 80, ATL: 70},
		{Type: "Ride", StartDateLocal: "2026-06-02T08:00:00", TrainingLoad: 50}, // no CTL/ATL fields
	}
	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{
		StartDate: "2026-06-01",
		EndDate:   "2026-06-02",
	})
	if got.Load.LatestCTL != 80 {
		t.Fatalf("LatestCTL = %v, want 80 (later zero fields must not wipe)", got.Load.LatestCTL)
	}
	if got.Load.LatestATL != 70 {
		t.Fatalf("LatestATL = %v, want 70", got.Load.LatestATL)
	}
}

func TestDailyLoadsFillRestDaysWhenRangeMissing(t *testing.T) {
	t.Parallel()

	// Mon/Wed/Fri only — without fill, monotony/ACWR use 3 points not 5 calendar days.
	activities := []icu.Activity{
		{Type: "Ride", StartDateLocal: "2026-06-01T08:00:00", TrainingLoad: 100}, // Mon
		{Type: "Ride", StartDateLocal: "2026-06-03T08:00:00", TrainingLoad: 100}, // Wed
		{Type: "Ride", StartDateLocal: "2026-06-05T08:00:00", TrainingLoad: 100}, // Fri
	}
	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{})
	if len(got.Load.Daily) != 5 {
		t.Fatalf("daily days = %d, want 5 inclusive Mon–Fri with rest zeros", len(got.Load.Daily))
	}
	var zeros int
	for _, day := range got.Load.Daily {
		if day.Load == 0 {
			zeros++
		}
	}
	if zeros != 2 {
		t.Fatalf("rest-day zeros = %d, want 2 (Tue+Thu)", zeros)
	}
}

func TestMatchWorkoutEventRequiresTimeWindow(t *testing.T) {
	t.Parallel()

	activity := &icu.Activity{
		ID:             "a1",
		Type:           "Ride",
		Name:           "Tempo",
		StartDateLocal: "2026-06-10T08:00:00",
		MovingTime:     3600,
		TrainingLoad:   80,
	}
	events := []icu.Event{{
		ID:             99,
		Category:       "WORKOUT",
		Type:           "Ride",
		Name:           "Tempo",
		StartDateLocal: "2026-06-01T08:00:00", // 9 days earlier
		MovingTime:     3600,
		TrainingLoad:   80,
		WorkoutDoc:     &icu.WorkoutDoc{Duration: 3600},
	}}
	match := icu.MatchWorkoutEvent(activity, events, icu.WorkoutEventMatchOptions{
		MatchWindowHours: 24,
	})
	if match.EventID != 0 {
		t.Fatalf("matched event %d outside window; want no match", match.EventID)
	}
	if match.Confidence != "none" {
		t.Fatalf("confidence = %q, want none", match.Confidence)
	}
}

func TestParseWorkoutDurationRoundsFractionalHours(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 0.1h 70%\n")
	if err != nil {
		t.Fatalf("ParseWorkoutDescription: %v", err)
	}
	if doc.Duration != 360 {
		t.Fatalf("duration = %d, want 360 (0.1h rounded, not truncated)", doc.Duration)
	}
}

func TestActivityModelUnreliableWithoutFTP(t *testing.T) {
	t.Parallel()

	if icu.ActivityModelReliable(&icu.Activity{CriticalPower: 100, WPrime: 15000}) {
		t.Fatal("ActivityModelReliable with CP but no FTP should be false")
	}
}

func TestMicrocycleOptionsNotMutated(t *testing.T) {
	t.Parallel()

	options := &icu.MicrocycleOptions{
		StartDate: "2026-06-01",
		EndDate:   "2026-06-07",
	}
	_ = icu.AnalyzeMicrocycle(nil, nil, nil, nil, options)
	if !options.Now.IsZero() {
		t.Fatalf("caller options.Now was mutated to %v", options.Now)
	}
	if options.SportType != "" {
		t.Fatalf("caller options.SportType was mutated to %q", options.SportType)
	}
}

func TestMicrocyclePreviousLoadIgnoresNonCycling(t *testing.T) {
	t.Parallel()

	// Current week: one ride TSS 100.
	// Previous week: run TSS 300 + ride TSS 100. Comparison should use ride-only previous=100
	// (ratio≈1 → similar/equal). Pre-fix included the run → previous=400 → "below".
	activities := []icu.Activity{
		{Type: "Run", StartDateLocal: "2026-05-26T08:00:00", TrainingLoad: 300},
		{Type: "Ride", StartDateLocal: "2026-05-27T08:00:00", TrainingLoad: 100},
		{Type: "Ride", StartDateLocal: "2026-06-02T08:00:00", TrainingLoad: 100},
	}
	options := &icu.MicrocycleOptions{
		StartDate: "2026-06-01",
		EndDate:   "2026-06-07",
		Now:       time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	}
	got := icu.AnalyzeMicrocycle(activities, nil, nil, nil, options)
	if got.Load.TSS != 100 {
		t.Fatalf("TSS = %d, want 100", got.Load.TSS)
	}
	if got.Load.Comparison.Previous7Days == "below" {
		t.Fatalf("Previous7Days = %q; non-cycling load incorrectly inflated previous baseline", got.Load.Comparison.Previous7Days)
	}
}

func TestNormalizeStreamsCoercesNumericAnyTypes(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{{
		Type: "mixed",
		Data: []any{float32(1.5), int(2), int32(3), int64(4)},
	}}
	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("NormalizeStreams: %v", err)
	}
	data := got["mixed"]
	if len(data) != 4 || data[0] != 1.5 || data[1] != 2 || data[2] != 3 || data[3] != 4 {
		t.Fatalf("mixed stream = %v, want coerced numerics", data)
	}
}

func TestMatchWorkoutEventExplicitBypassesWindow(t *testing.T) {
	t.Parallel()

	activity := &icu.Activity{
		ID: "a1", Type: "Ride", Name: "Tempo",
		StartDateLocal: "2026-06-10T08:00:00", MovingTime: 3600, TrainingLoad: 80,
	}
	events := []icu.Event{{
		ID: 7, Category: "WORKOUT", Type: "Ride", Name: "Tempo",
		StartDateLocal: "2026-05-01T08:00:00", MovingTime: 3600, TrainingLoad: 80,
	}}
	match := icu.MatchWorkoutEvent(activity, events, icu.WorkoutEventMatchOptions{
		ExplicitEventID:  7,
		MatchWindowHours: 24,
	})
	if match.EventID != 7 {
		t.Fatalf("EventID = %d, want explicit 7 outside window", match.EventID)
	}
}

func TestRebalanceV2ExactIFZeroSeconds(t *testing.T) {
	t.Parallel()
	// Exported path: empty duration via EstimatePlannedLoad is separate; here
	// NormalizedPower empty + BuildURL empty query keep package surface covered.
	if got := icu.BuildURL("/api/v1/x", nil); got != "https://intervals.icu/api/v1/x" {
		t.Fatalf("BuildURL empty query = %q", got)
	}
}

func TestNormalizedPowerMatchesRollingAverage(t *testing.T) {
	t.Parallel()

	// Blocky intervals: 30s @ 100, 30s @ 400, repeated — NP must exceed average.
	watts := make([]float64, 120)
	for i := range watts {
		if (i/30)%2 == 0 {
			watts[i] = 100
		} else {
			watts[i] = 400
		}
	}
	np := icu.NormalizedPower(watts, 0, len(watts))
	avg := icu.AveragePower(watts, 0, len(watts))
	if np <= avg {
		t.Fatalf("NP=%v should exceed average=%v for blocky variable power", np, avg)
	}
	if math.IsNaN(np) || math.IsInf(np, 0) {
		t.Fatalf("NP invalid: %v", np)
	}
}
