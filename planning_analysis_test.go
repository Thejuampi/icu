package icu_test

import (
	"reflect"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestAnalyzeTrainingPlanClassifiesBuildWithDeload(t *testing.T) {
	t.Parallel()

	var events []icu.Event
	events = append(
		events,
		plannedWorkout("2026-06-02T08:00:00", "VO2 Re-Entry", 80, 4300, 0.82),
		plannedWorkout("2026-06-04T08:00:00", "Tempo Durability", 100, 5700, 0.74),
		plannedWorkout("2026-06-06T08:00:00", "Long Durability I", 140, 10380, 0.68),
		plannedWorkout("2026-06-09T08:00:00", "VO2 Stabilize", 90, 4300, 0.86),
		plannedWorkout("2026-06-11T08:00:00", "Threshold Builder", 120, 6700, 0.78),
		plannedWorkout("2026-06-13T08:00:00", "Long Durability II", 160, 11520, 0.69),
		plannedWorkout("2026-06-16T08:00:00", "VO2 Peak", 100, 4600, 0.88),
		plannedWorkout("2026-06-18T08:00:00", "Threshold Durability", 130, 7500, 0.79),
		plannedWorkout("2026-06-20T08:00:00", "Long Durability III", 180, 12540, 0.69),
		plannedWorkout("2026-06-23T08:00:00", "VO2 Openers", 65, 4140, 0.76),
		plannedWorkout("2026-06-25T08:00:00", "Validation Tempo", 80, 5700, 0.71),
		plannedWorkout("2026-06-27T08:00:00", "Endurance Validation", 110, 8400, 0.68),
	)

	got := icu.AnalyzeTrainingPlan(nil, events, icu.TrainingPlanOptions{
		HistoryStartDate: "",
		HistoryEndDate:   "",
		PlanStartDate:    "2026-06-01",
		PlanEndDate:      "2026-06-28",
	})
	want := icu.TrainingPlanPhase{
		Label:      "build",
		Pattern:    "build_with_deload",
		Intent:     "progressive build with planned absorption week",
		Confidence: "moderate",
		Source:     "planned_event_load_heuristic",
	}

	if !reflect.DeepEqual(got.Phase, want) {
		t.Fatalf("Phase = %+v, want %+v", got.Phase, want)
	}
}

func TestAnalyzeTrainingPlanSummarizesSessionStructure(t *testing.T) {
	t.Parallel()

	var events []icu.Event
	events = append(
		events,
		plannedNote("2026-06-01T08:00:00", "OFF"),
		plannedWorkout("2026-06-02T08:00:00", "VO2 Re-Entry", 80, 4300, 0.88),
		plannedWorkout("2026-06-04T08:00:00", "Tempo Durability", 100, 5700, 0.74),
		plannedWorkout("2026-06-06T08:00:00", "Long Durability I", 140, 10380, 0.68),
		plannedWorkout("2026-06-07T08:00:00", "Easy Recovery", 45, 4200, 0.60),
	)

	got := icu.AnalyzeTrainingPlan(nil, events, icu.TrainingPlanOptions{
		HistoryStartDate: "",
		HistoryEndDate:   "",
		PlanStartDate:    "2026-06-01",
		PlanEndDate:      "2026-06-07",
	})
	want := icu.TrainingPlanSessionSummary{
		Events:                 5,
		Workouts:               4,
		Notes:                  1,
		RestDays:               1,
		HighIntensitySessions:  1,
		TempoThresholdSessions: 1,
		LongEnduranceSessions:  1,
		RecoverySessions:       1,
		AerobicSessions:        1,
		OpenerSessions:         0,
	}

	if !reflect.DeepEqual(got.Sessions, want) {
		t.Fatalf("Sessions = %+v, want %+v", got.Sessions, want)
	}
}

func TestAnalyzeTrainingPlanComparesAgainstCompletedHistory(t *testing.T) {
	t.Parallel()

	var activities []icu.Activity
	activities = append(
		activities,
		completedRide("2026-05-05T08:00:00", 440, 36000),
		completedRide("2026-05-12T08:00:00", 450, 37000),
		completedRide("2026-05-19T08:00:00", 490, 38500),
	)

	var events []icu.Event
	events = append(
		events,
		plannedWorkout("2026-06-02T08:00:00", "VO2 Re-Entry", 80, 4300, 0.82),
		plannedWorkout("2026-06-06T08:00:00", "Long Durability I", 140, 10380, 0.68),
		plannedWorkout("2026-06-16T08:00:00", "VO2 Peak", 100, 4600, 0.88),
		plannedWorkout("2026-06-20T08:00:00", "Long Durability III", 180, 12540, 0.69),
	)

	got := icu.AnalyzeTrainingPlan(activities, events, icu.TrainingPlanOptions{
		HistoryStartDate: "2026-05-04",
		HistoryEndDate:   "2026-05-24",
		PlanStartDate:    "2026-06-01",
		PlanEndDate:      "2026-06-28",
	})
	want := icu.TrainingPlanHistory{
		CompletedWeeks:       3,
		AverageWeeklyLoad:    460,
		PeakWeeklyLoad:       490,
		RecentWeeklyLoad:     490,
		AverageWeeklyHours:   10.32,
		CurrentStateLabel:    "",
		CurrentLoadPressure:  0,
		CurrentStateSource:   "",
		PlannedLoadAlignment: "below_recent_tolerance",
	}

	if !reflect.DeepEqual(got.History, want) {
		t.Fatalf("History = %+v, want %+v", got.History, want)
	}
}

func TestAnalyzeTrainingPlanAddsExecutionGuidanceForKeyWorkout(t *testing.T) {
	t.Parallel()

	var events []icu.Event
	events = append(events, plannedWorkout("2026-06-16T08:00:00", "Ride - VO2 Peak 4x5 Week 3", 91, 4620, 0.842))

	got := icu.AnalyzeTrainingPlan(nil, events, icu.TrainingPlanOptions{
		HistoryStartDate: "",
		HistoryEndDate:   "",
		PlanStartDate:    "2026-06-16",
		PlanEndDate:      "2026-06-16",
	})
	want := "4x5 VO2Max"

	if got.PlannedSessions[0].Execution.RecommendedTitle != want {
		t.Fatalf("RecommendedTitle = %q, want %q", got.PlannedSessions[0].Execution.RecommendedTitle, want)
	}
}

func TestAnalyzeTrainingPlanUsesEncouragementOnlyWhenAppropriate(t *testing.T) {
	t.Parallel()

	var events []icu.Event
	events = append(events, plannedWorkout("2026-06-07T08:00:00", "Ride - Easy Recovery", 46, 4200, 0.628))

	got := icu.AnalyzeTrainingPlan(nil, events, icu.TrainingPlanOptions{
		HistoryStartDate: "",
		HistoryEndDate:   "",
		PlanStartDate:    "2026-06-07",
		PlanEndDate:      "2026-06-07",
	})

	for _, cue := range got.PlannedSessions[0].Execution.Cues {
		if cue.Tone == "encouragement" {
			t.Fatalf("Recovery cue = %+v, want no encouragement tone", cue)
		}
	}
}

func TestAnalyzeTrainingPlanRoundsLongEnduranceTitle(t *testing.T) {
	t.Parallel()

	var events []icu.Event
	events = append(events, plannedWorkout("2026-06-06T08:00:00", "Ride - Long Durability I", 135, 10380, 0.684))

	got := icu.AnalyzeTrainingPlan(nil, events, icu.TrainingPlanOptions{
		HistoryStartDate: "",
		HistoryEndDate:   "",
		PlanStartDate:    "2026-06-06",
		PlanEndDate:      "2026-06-06",
	})
	want := "3h Z2"

	if got.PlannedSessions[0].Execution.RecommendedTitle != want {
		t.Fatalf("RecommendedTitle = %q, want %q", got.PlannedSessions[0].Execution.RecommendedTitle, want)
	}
}

func TestAnalyzeTrainingPlanAddsIndoorZ2MicroVariation(t *testing.T) {
	t.Parallel()

	event := plannedWorkout("2026-06-06T08:00:00", "Ride - Long Durability Indoor", 135, 10380, 0.684)
	event.Indoor = true

	got := icu.AnalyzeTrainingPlan([]icu.Activity{}, []icu.Event{event}, icu.TrainingPlanOptions{
		HistoryStartDate: "",
		HistoryEndDate:   "",
		PlanStartDate:    "2026-06-06",
		PlanEndDate:      "2026-06-06",
	})
	want := "indoor_z2_micro_variation"

	if got.PlannedSessions[0].Execution.WorkoutProfile.Name != want {
		t.Fatalf("WorkoutProfile.Name = %q, want %q", got.PlannedSessions[0].Execution.WorkoutProfile.Name, want)
	}
}

func TestAnalyzeTrainingPlanIndoorZ2UsesFourMinuteMicroIntervals(t *testing.T) {
	t.Parallel()

	event := plannedWorkout("2026-06-06T08:00:00", "Ride - Long Durability Indoor", 135, 10380, 0.684)
	event.Indoor = true

	got := icu.AnalyzeTrainingPlan([]icu.Activity{}, []icu.Event{event}, icu.TrainingPlanOptions{
		HistoryStartDate: "",
		HistoryEndDate:   "",
		PlanStartDate:    "2026-06-06",
		PlanEndDate:      "2026-06-06",
	})
	want := icu.TrainingPlanWorkoutBlock{
		Name:            "Z2 wave micro-intervals",
		Repeat:          20,
		WorkSeconds:     240,
		RecoverySeconds: 40,
		WorkTarget:      "rotate low Z2, mid Z2 shadow, high Z2, cap at max Z2",
		RecoveryTarget:  "low Z2 or high Z1 until HR settles",
		Cue:             "Use the 40s valleys to control HR drift, not to coast fully.",
	}

	if !reflect.DeepEqual(got.PlannedSessions[0].Execution.WorkoutProfile.Blocks[1], want) {
		t.Fatalf("WorkoutProfile.Blocks[1] = %+v, want %+v", got.PlannedSessions[0].Execution.WorkoutProfile.Blocks[1], want)
	}
}

func plannedWorkout(date, name string, load, movingTime int, intensity float64) icu.Event {
	var event icu.Event
	event.StartDateLocal = date
	event.Category = testWorkoutType
	event.Type = testRideType
	event.Name = name
	event.TrainingLoad = load
	event.MovingTime = movingTime
	event.Intensity = intensity

	return event
}

func plannedNote(date, name string) icu.Event {
	var event icu.Event
	event.StartDateLocal = date
	event.Category = "NOTE"
	event.Name = name

	return event
}

func completedRide(date string, load, movingTime int) icu.Activity {
	var activity icu.Activity
	activity.Type = testRideType
	activity.StartDateLocal = date
	activity.TrainingLoad = load
	activity.MovingTime = movingTime

	return activity
}
