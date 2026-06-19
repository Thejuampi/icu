package icu_test

import (
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

func TestAnalyzeWorkoutExecutionMissingActivityWarns(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeWorkoutExecution(icu.WorkoutExecutionInputs{}, icu.WorkoutExecutionOptions{})

	if got.Comparison.Completion != "plan_missing" || len(got.Warnings) == 0 {
		t.Fatalf("AnalyzeWorkoutExecution missing = %+v, want plan_missing warning", got)
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

func repeatValue(value float64, count int) []float64 {
	result := make([]float64, count)
	for i := range result {
		result[i] = value
	}
	return result
}
