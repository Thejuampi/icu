package icu_test

import (
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

const testWorkoutConfidenceNone = "none"

func TestMatchWorkoutEventPrefersSameDayWorkout(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{
		ID: "a1", Name: "3x18 Sweet Spot", Type: "Ride", StartDateLocal: "2026-06-18T18:09:46",
		MovingTime: 6743, TrainingLoad: 106,
	}
	events := []icu.Event{
		{ID: 1, Category: "NOTE", Name: "Block note", StartDateLocal: "2026-06-18T00:00:00"},
		{
			ID: 2, Category: "WORKOUT", Type: "Ride", Name: "W3 Thu 2026-06-18 ALT 3x18 Sweet Spot",
			StartDateLocal: "2026-06-18T00:00:00", MovingTime: 6420, TrainingLoad: 118,
		},
	}

	got := icu.MatchWorkoutEvent(&activity, events, icu.WorkoutEventMatchOptions{})

	if got.EventID != 2 || got.Confidence == testWorkoutConfidenceNone {
		t.Fatalf("MatchWorkoutEvent = %+v, want event 2 with confidence", got)
	}
}

func TestMatchWorkoutEventPrefersSameDateOverShorterNextDayEvent(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{
		ID: "i173884602", Name: "W32 Sat Z2 Durable + Z3 Spikes", Type: "Ride",
		StartDateLocal: "2026-08-08T18:04:50", MovingTime: 5637, TrainingLoad: 55,
	}
	events := []icu.Event{
		{
			ID: 125663242, Category: "WORKOUT", Type: "Ride", Name: "W32 Sat Z2 Durable + Z3 Spikes",
			StartDateLocal: "2026-08-08T07:00:00", MovingTime: 9495, TrainingLoad: 96,
		},
		{
			ID: 125663243, Category: "WORKOUT", Type: "Ride", Name: "W32 Sun Z2 Cruise + Z3 Spikes",
			StartDateLocal: "2026-08-09T07:00:00", MovingTime: 4455, TrainingLoad: 44,
		},
	}

	got := icu.MatchWorkoutEvent(&activity, events, icu.WorkoutEventMatchOptions{})

	if got.EventID != 125663242 {
		t.Fatalf("MatchWorkoutEvent = %+v, want same-date event 125663242", got)
	}
}

func TestAnalyzeWorkoutExecutionComparesSessionAndReps(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{
		ID: "a1", Name: "3x18 Sweet Spot", Type: "Ride", StartDateLocal: "2026-06-18T18:09:46",
		MovingTime: 6743, TrainingLoad: 106, Intensity: 0.751, WeightedAvgPower: 214, AverageHeartRate: 136,
	}
	event := icu.Event{
		ID: 2, Category: "WORKOUT", Type: "Ride", Name: "W3 Thu 2026-06-18 ALT 3x18 Sweet Spot",
		StartDateLocal: "2026-06-18T00:00:00", MovingTime: 6420, TrainingLoad: 118, Intensity: 0.813, WorkoutDoc: map[string]any{
			"steps": []any{map[string]any{"reps": float64(3), "steps": []any{
				map[string]any{"duration": float64(1080), "power": map[string]any{"start": float64(90), "end": float64(94), "units": "%ftp"}},
				map[string]any{"duration": float64(360), "power": map[string]any{"start": float64(60), "end": float64(65), "units": "%ftp"}},
			}}},
		},
	}
	intervals := icu.IntervalsDTO{Intervals: []icu.Interval{
		{StartIndex: 0, EndIndex: 1080, Type: "WORK", MovingTime: 1080, AvgPower: 258, AvgHR: 158},
		{StartIndex: 1080, EndIndex: 1440, Type: "RECOVERY", MovingTime: 360, AvgPower: 178, AvgHR: 142},
		{StartIndex: 1440, EndIndex: 2520, Type: "WORK", MovingTime: 1080, AvgPower: 255, AvgHR: 161},
	}}
	streams := icu.StreamData{"watts": repeatValue(250, 2520), "heartrate": repeatValue(155, 2520)}

	got := icu.AnalyzeWorkoutExecution(icu.WorkoutExecutionInputs{
		Activity:      &activity,
		Streams:       streams,
		Intervals:     &intervals,
		Events:        []icu.Event{event},
		SportSettings: &icu.SportSettings{FTP: 285, LTHR: 178},
	}, icu.WorkoutExecutionOptions{})

	if got.Comparison.RepCount.Planned != 3 || got.Comparison.RepCount.Executed != 2 {
		t.Fatalf("rep count = %+v, want planned 3 executed 2", got.Comparison.RepCount)
	}
}

// TestAnalyzeWorkoutExecutionEmitsFTPSourceAndConfidence is R1: the read
// path must surface ResolveFTP's provenance next to Activity.FTP instead of
// discarding it. A VirtualRide is type_inherent/high, so no
// ftp_confidence_low warning should appear either.
func TestAnalyzeWorkoutExecutionEmitsFTPSourceAndConfidence(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{ID: "a1", Type: "VirtualRide", StartDateLocal: "2026-06-18T18:09:46", MovingTime: 3600}

	got := icu.AnalyzeWorkoutExecution(icu.WorkoutExecutionInputs{
		Activity:      &activity,
		SportSettings: &icu.SportSettings{FTP: 270, IndoorFTP: 285},
	}, icu.WorkoutExecutionOptions{})

	if got.FTPSource != "type_inherent" || got.FTPConfidence != "high" ||
		strings.Contains(strings.Join(got.Warnings, "\n"), "ftp_confidence_low") {
		t.Fatalf("ftpSource/ftpConfidence = %q/%q, warnings = %v, want type_inherent/high and no ftp_confidence_low",
			got.FTPSource, got.FTPConfidence, got.Warnings)
	}
}

// TestAnalyzeWorkoutExecutionAmbiguousTypeWarnsFTPConfidenceLow is R1's
// warning half: D-10c requires the read path to proceed and warn on `low`
// confidence, never refuse. GravelRide with no discriminating flag (Activity
// exposes no Trainer field yet) resolves unknown/low.
func TestAnalyzeWorkoutExecutionAmbiguousTypeWarnsFTPConfidenceLow(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{ID: "a1", Type: "GravelRide", StartDateLocal: "2026-06-18T18:09:46", MovingTime: 3600}

	got := icu.AnalyzeWorkoutExecution(icu.WorkoutExecutionInputs{
		Activity:      &activity,
		SportSettings: &icu.SportSettings{FTP: 270, IndoorFTP: 285},
	}, icu.WorkoutExecutionOptions{})

	if got.FTPConfidence != "low" || !strings.Contains(strings.Join(got.Warnings, "\n"), "ftp_confidence_low") {
		t.Fatalf("ftpConfidence = %q, warnings = %v, want low with ftp_confidence_low warning", got.FTPConfidence, got.Warnings)
	}
}

func TestAnalyzeWorkoutExecutionMissingActivityWarns(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeWorkoutExecution(icu.WorkoutExecutionInputs{}, icu.WorkoutExecutionOptions{})

	if got.Comparison.Completion != "plan_missing" || len(got.Warnings) == 0 {
		t.Fatalf("AnalyzeWorkoutExecution missing = %+v, want plan_missing warning", got)
	}
}

// TestWorkoutExecutionAnalysisSchemaVersionDidNotDrift pins the literal
// wire string (D-7), not the constant against itself: a consumer parsing
// analysis output needs a version marker to detect a future meaning
// change. Comparing icu.WorkoutExecutionAnalysisSchemaVersion against
// itself would pass regardless of what the constant holds.
func TestWorkoutExecutionAnalysisSchemaVersionDidNotDrift(t *testing.T) {
	t.Parallel()

	if icu.WorkoutExecutionAnalysisSchemaVersion != "icu.analysis.workout.v1" {
		t.Fatalf("WorkoutExecutionAnalysisSchemaVersion = %q, want icu.analysis.workout.v1", icu.WorkoutExecutionAnalysisSchemaVersion)
	}
}

func TestAnalyzeWorkoutExecutionEmitsSchemaVersion(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeWorkoutExecution(icu.WorkoutExecutionInputs{}, icu.WorkoutExecutionOptions{})

	if got.SchemaVersion != "icu.analysis.workout.v1" {
		t.Fatalf("SchemaVersion = %q, want icu.analysis.workout.v1", got.SchemaVersion)
	}
}

func TestAnalyzeWorkoutExecutionNoMatchReportsWarnings(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{ID: "a1", Name: "Free Ride", Type: "Ride", StartDateLocal: "2026-06-18T08:00:00"}

	got := icu.AnalyzeWorkoutExecution(icu.WorkoutExecutionInputs{Activity: &activity}, icu.WorkoutExecutionOptions{})

	if got.Match.Confidence != testWorkoutConfidenceNone || len(got.Warnings) < 4 {
		t.Fatalf("AnalyzeWorkoutExecution no match = %+v, want none and warnings", got)
	}
}

func TestMatchWorkoutEventExplicitEventID(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{ID: "a1", Name: "Easy", Type: "Ride", StartDateLocal: "2026-06-18T08:00:00"}
	events := []icu.Event{
		{ID: 1, Category: "WORKOUT", Type: "Ride", Name: "Easy", StartDateLocal: "2026-06-18T08:00:00"},
		{ID: 2, Category: "NOTE", Name: "Manual match", StartDateLocal: "2026-06-20T08:00:00"},
	}

	got := icu.MatchWorkoutEvent(&activity, events, icu.WorkoutEventMatchOptions{ExplicitEventID: 2})

	if got.EventID != 2 {
		t.Fatalf("MatchWorkoutEvent explicit = %+v, want event 2", got)
	}
}

func TestMatchWorkoutEventRejectsOutsideWindow(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{ID: "a1", Name: "Easy", Type: "Ride", StartDateLocal: "2026-06-18T08:00:00"}
	events := []icu.Event{{ID: 1, Category: "NOTE", Name: "Other", StartDateLocal: "2026-06-20T08:00:00"}}

	got := icu.MatchWorkoutEvent(&activity, events, icu.WorkoutEventMatchOptions{MatchWindowHours: 1})

	if got.Confidence != testWorkoutConfidenceNone {
		t.Fatalf("MatchWorkoutEvent outside = %+v, want none", got)
	}
}

// TestMatchWorkoutEventServerPairingMatchesOutsideHeuristicWindow is W2-P04:
// paired_event_id is authoritative (D-21) and matches even when the event
// falls outside MatchWorkoutEvent's own time window.
func TestMatchWorkoutEventServerPairingMatchesOutsideHeuristicWindow(t *testing.T) {
	t.Parallel()

	pairedEventID := 999
	activity := icu.Activity{
		ID: "a1", Name: "Long Ride", Type: "Ride", StartDateLocal: "2026-06-18T08:00:00",
		PairedEventID: &pairedEventID,
	}
	events := []icu.Event{
		{ID: 999, Category: "WORKOUT", Type: "Ride", Name: "Planned Long Ride", StartDateLocal: "2026-05-01T08:00:00"},
	}

	got := icu.MatchWorkoutEvent(&activity, events, icu.WorkoutEventMatchOptions{MatchWindowHours: 1})

	if got.EventID != 999 || got.Source != "server_paired_event_id" {
		t.Fatalf("MatchWorkoutEvent server-paired = %+v, want event 999 from server_paired_event_id", got)
	}
}

// TestMatchWorkoutEventUnpairedComplianceOmitted is W2-N01: an unpaired
// activity's compliance (0.0 on all 36 live occurrences, F-18) must never
// surface, but match.source is still emitted so the absence is explicable.
func TestMatchWorkoutEventUnpairedComplianceOmitted(t *testing.T) {
	t.Parallel()

	compliance := 0.0
	activity := icu.Activity{ID: "a1", Name: "Free Ride", Type: "Ride", StartDateLocal: "2026-06-18T08:00:00", Compliance: &compliance}

	got := icu.MatchWorkoutEvent(&activity, nil, icu.WorkoutEventMatchOptions{})

	if got.Compliance != nil || got.Source != "unmatched" {
		t.Fatalf("MatchWorkoutEvent unpaired = %+v, want nil compliance and source unmatched", got)
	}
}

// TestMatchWorkoutEventStubNeverPaired is W2-N03: a STRAVA stub on the same
// date as a planned event must not be paired on either the server or the
// heuristic path, and must not carry a compliance value.
func TestMatchWorkoutEventStubNeverPaired(t *testing.T) {
	t.Parallel()

	pairedEventID := 42
	compliance := 0.8
	stub := icu.Activity{
		ID: "s1", StartDateLocal: "2026-06-18T08:00:00", Source: "STRAVA",
		StubShape: true, PairedEventID: &pairedEventID, Compliance: &compliance,
	}
	events := []icu.Event{
		{ID: 42, Category: "WORKOUT", Type: "Ride", Name: "Planned Ride", StartDateLocal: "2026-06-18T08:00:00"},
	}

	got := icu.MatchWorkoutEvent(&stub, events, icu.WorkoutEventMatchOptions{})

	if got.EventID != 0 || got.Source != "not_a_record" || got.Compliance != nil {
		t.Fatalf("MatchWorkoutEvent stub = %+v, want no pairing, source not_a_record, no compliance", got)
	}
}

// TestMatchWorkoutEventPairedComplianceKeyLackingIsNil is W2-E02: a paired
// activity whose payload never carried compliance at all must report a nil
// pointer (key omitted on marshal), not an invented zero.
func TestMatchWorkoutEventPairedComplianceKeyLackingIsNil(t *testing.T) {
	t.Parallel()

	pairedEventID := 7
	activity := icu.Activity{ID: "a1", StartDateLocal: "2026-06-18T08:00:00", PairedEventID: &pairedEventID}
	events := []icu.Event{{ID: 7, Category: "WORKOUT", Name: "Planned", StartDateLocal: "2026-06-18T08:00:00"}}

	got := icu.MatchWorkoutEvent(&activity, events, icu.WorkoutEventMatchOptions{})

	if got.Compliance != nil {
		t.Fatalf("MatchWorkoutEvent = %+v, want nil compliance when the activity never carried the field", got)
	}
}

// TestMatchWorkoutEventPairedZeroComplianceIsPresent is W2-E03: a paired
// activity with a genuine 0.0 compliance must surface the key present with
// 0, distinct from W2-E02's absent case.
func TestMatchWorkoutEventPairedZeroComplianceIsPresent(t *testing.T) {
	t.Parallel()

	pairedEventID := 7
	compliance := 0.0
	activity := icu.Activity{ID: "a1", StartDateLocal: "2026-06-18T08:00:00", PairedEventID: &pairedEventID, Compliance: &compliance}
	events := []icu.Event{{ID: 7, Category: "WORKOUT", Name: "Planned", StartDateLocal: "2026-06-18T08:00:00"}}

	got := icu.MatchWorkoutEvent(&activity, events, icu.WorkoutEventMatchOptions{})

	if got.Compliance == nil || *got.Compliance != 0 {
		t.Fatalf("MatchWorkoutEvent = %+v, want compliance present with 0", got)
	}
}

func repeatValue(value float64, count int) []float64 {
	result := make([]float64, count)
	for i := range result {
		result[i] = value
	}
	return result
}
