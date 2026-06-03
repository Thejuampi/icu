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
		CurrentCTL:           0,
		CurrentATL:           0,
		CurrentTSB:           0,
		CurrentDecoupling:    0,
		CurrentHotSessions:   0,
		CurrentStateSource:   "",
		PlannedLoadAlignment: "below_recent_tolerance",
		Weeks: []icu.TrainingPlanCompletedWeek{
			{
				ISOWeek:               "2026-W19",
				StartDate:             "2026-05-04",
				EndDate:               "2026-05-10",
				Load:                  440,
				MovingTimeSecs:        36000,
				MovingTimeHours:       10,
				Sessions:              1,
				HighIntensityDays:     0,
				LongEnduranceSessions: 1,
				MeanIntensity:         0,
				MeanDecoupling:        0,
				Tolerance:             "within_recent_tolerance",
			},
			{
				ISOWeek:               "2026-W20",
				StartDate:             "2026-05-11",
				EndDate:               "2026-05-17",
				Load:                  450,
				MovingTimeSecs:        37000,
				MovingTimeHours:       10.28,
				Sessions:              1,
				HighIntensityDays:     0,
				LongEnduranceSessions: 1,
				MeanIntensity:         0,
				MeanDecoupling:        0,
				Tolerance:             "within_recent_tolerance",
			},
			{
				ISOWeek:               "2026-W21",
				StartDate:             "2026-05-18",
				EndDate:               "2026-05-24",
				Load:                  490,
				MovingTimeSecs:        38500,
				MovingTimeHours:       10.69,
				Sessions:              1,
				HighIntensityDays:     0,
				LongEnduranceSessions: 1,
				MeanIntensity:         0,
				MeanDecoupling:        0,
				Tolerance:             "within_recent_tolerance",
			},
		},
	}

	if !reflect.DeepEqual(got.History, want) {
		t.Fatalf("History = %+v, want %+v", got.History, want)
	}
}

func TestAnalyzeTrainingPlanIncludesCompletedWeekSeries(t *testing.T) {
	t.Parallel()

	var hardRide icu.Activity
	hardRide.Type = testRideType
	hardRide.StartDateLocal = "2026-05-05T08:00:00"
	hardRide.TrainingLoad = 100
	hardRide.MovingTime = 3600
	hardRide.Intensity = 0.9
	hardRide.Decoupling = 3

	var longRide icu.Activity
	longRide.Type = testRideType
	longRide.StartDateLocal = "2026-05-07T08:00:00"
	longRide.TrainingLoad = 80
	longRide.MovingTime = 9000
	longRide.Intensity = 0.68
	longRide.Decoupling = 4

	var recoveryRide icu.Activity
	recoveryRide.Type = testRideType
	recoveryRide.StartDateLocal = "2026-05-12T08:00:00"
	recoveryRide.TrainingLoad = 60
	recoveryRide.MovingTime = 3600
	recoveryRide.Intensity = 0.6

	got := icu.AnalyzeTrainingPlan([]icu.Activity{hardRide, longRide, recoveryRide}, nil, icu.TrainingPlanOptions{
		HistoryStartDate: "2026-05-04",
		HistoryEndDate:   "2026-05-17",
		PlanStartDate:    "",
		PlanEndDate:      "",
	})
	want := []icu.TrainingPlanCompletedWeek{
		{
			ISOWeek:               "2026-W19",
			StartDate:             "2026-05-04",
			EndDate:               "2026-05-10",
			Load:                  180,
			MovingTimeSecs:        12600,
			MovingTimeHours:       3.5,
			Sessions:              2,
			HighIntensityDays:     1,
			LongEnduranceSessions: 1,
			MeanIntensity:         0.79,
			MeanDecoupling:        3.5,
			Tolerance:             "above_recent_average",
		},
		{
			ISOWeek:               "2026-W20",
			StartDate:             "2026-05-11",
			EndDate:               "2026-05-17",
			Load:                  60,
			MovingTimeSecs:        3600,
			MovingTimeHours:       1,
			Sessions:              1,
			HighIntensityDays:     0,
			LongEnduranceSessions: 0,
			MeanIntensity:         0.6,
			MeanDecoupling:        0,
			Tolerance:             "below_recent_tolerance",
		},
	}

	if !reflect.DeepEqual(got.History.Weeks, want) {
		t.Fatalf("History.Weeks = %+v, want %+v", got.History.Weeks, want)
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

func TestAnalyzeTrainingPlanUsesSportAnchorsFromContext(t *testing.T) {
	t.Parallel()

	var settings icu.SportSettings
	settings.FTP = 295
	settings.IndoorFTP = 288
	settings.LTHR = 176
	settings.MaxHR = 194
	settings.WPrime = 23000
	settings.PMax = 1090

	got := icu.AnalyzeTrainingPlanWithContext(nil, nil, icu.TrainingPlanOptions{
		HistoryStartDate: "",
		HistoryEndDate:   "",
		PlanStartDate:    "",
		PlanEndDate:      "",
	}, icu.TrainingPlanContext{
		SportSettings: &settings,
		Wellness:      nil,
		Adaptation:    nil,
	})
	want := icu.TrainingPlanSportAnchors{
		FTP:       295,
		IndoorFTP: 288,
		LTHR:      176,
		MaxHR:     194,
		WPrime:    23000,
		PMax:      1090,
		Source:    "sport_settings",
	}

	if got.Anchors != want {
		t.Fatalf("Anchors = %+v, want %+v", got.Anchors, want)
	}
}

func TestAnalyzeTrainingPlanBuildsTargetEventStatus(t *testing.T) {
	t.Parallel()

	var events []icu.Event
	events = append(
		events,
		plannedWorkout("2026-06-03T08:00:00", "Tempo", 80, 5400, 0.74),
		plannedTarget("2026-06-20T08:00:00", "A Race", "RACE_A"),
		plannedTarget("2026-06-28T08:00:00", "A Event", "TARGET"),
	)

	var rides []icu.Activity
	rides = append(rides, completedRide("2026-05-28T08:00:00", 110, 7200))

	got := icu.AnalyzeTrainingPlan(rides, events, icu.TrainingPlanOptions{
		HistoryStartDate: "2026-05-01",
		HistoryEndDate:   "2026-05-31",
		PlanStartDate:    "2026-06-01",
		PlanEndDate:      "2026-06-30",
	})
	want := icu.TrainingPlanEventTargetStatus{
		TargetEvents:   2,
		RaceEvents:     1,
		UpcomingEvents: 2,
		NextEventDate:  "2026-06-20",
		Readiness:      "on_track",
		Source:         "calendar_event_targets",
		Events:         nil,
	}

	if got.TargetStatus.TargetEvents != want.TargetEvents ||
		got.TargetStatus.RaceEvents != want.RaceEvents ||
		got.TargetStatus.UpcomingEvents != want.UpcomingEvents ||
		got.TargetStatus.NextEventDate != want.NextEventDate ||
		got.TargetStatus.Readiness != want.Readiness ||
		got.TargetStatus.Source != want.Source {
		t.Fatalf("TargetStatus = %+v, want core %+v", got.TargetStatus, want)
	}
}

func TestAnalyzeTrainingPlanBuildsLoadForecast(t *testing.T) {
	t.Parallel()

	var current icu.Activity
	current.Type = testRideType
	current.StartDateLocal = "2026-05-31T08:00:00"
	current.CTL = 70
	current.ATL = 80

	var events []icu.Event
	events = append(
		events,
		plannedWorkout("2026-06-01T08:00:00", "VO2", 100, 3600, 0.9),
		plannedWorkout("2026-06-02T08:00:00", "Easy", 50, 3600, 0.6),
	)

	got := icu.AnalyzeTrainingPlan([]icu.Activity{current}, events, icu.TrainingPlanOptions{
		HistoryStartDate: "2026-05-01",
		HistoryEndDate:   "2026-05-31",
		PlanStartDate:    "2026-06-01",
		PlanEndDate:      "2026-06-02",
	})
	want := icu.TrainingPlanForecast{
		StartCTL:   70,
		StartATL:   80,
		StartTSB:   -10,
		EndCTL:     70.22,
		EndATL:     78.16,
		EndTSB:     -7.94,
		LowestTSB:  -12.14,
		HighestTSB: -7.94,
		Status:     "watch",
		Source:     "planned_load_impulse_model",
		Points:     nil,
	}

	if got.Forecast.StartCTL != want.StartCTL ||
		got.Forecast.StartATL != want.StartATL ||
		got.Forecast.StartTSB != want.StartTSB ||
		got.Forecast.EndCTL != want.EndCTL ||
		got.Forecast.EndATL != want.EndATL ||
		got.Forecast.EndTSB != want.EndTSB ||
		got.Forecast.LowestTSB != want.LowestTSB ||
		got.Forecast.HighestTSB != want.HighestTSB ||
		got.Forecast.Status != want.Status ||
		got.Forecast.Source != want.Source {
		t.Fatalf("Forecast = %+v, want core %+v", got.Forecast, want)
	}
}

func TestAnalyzeTrainingPlanBuildsDayRulesAndDecisionGuidance(t *testing.T) {
	t.Parallel()

	var current icu.Activity
	current.Type = testRideType
	current.StartDateLocal = "2026-05-31T08:00:00"
	current.CTL = 60
	current.ATL = 78
	current.Decoupling = 6.2
	current.AverageTemp = 32

	var events []icu.Event
	events = append(events, plannedWorkout("2026-06-01T08:00:00", "VO2 Key", 110, 3600, 0.9))

	var firstWellness icu.Wellness
	firstWellness.ID = "2026-05-29"
	firstWellness.HRV = 50
	firstWellness.RestingHR = 45
	firstWellness.SleepScore = 85

	var secondWellness icu.Wellness
	secondWellness.ID = "2026-05-30"
	secondWellness.HRV = 43
	secondWellness.RestingHR = 55
	secondWellness.SleepScore = 70

	wellness := icu.AnalyzeWellness([]icu.Wellness{firstWellness, secondWellness}, icu.AnalysisOptions{
		StartDate: "2026-05-29",
		EndDate:   "2026-05-31",
	})

	got := icu.AnalyzeTrainingPlanWithContext([]icu.Activity{current}, events, icu.TrainingPlanOptions{
		HistoryStartDate: "2026-05-01",
		HistoryEndDate:   "2026-05-31",
		PlanStartDate:    "2026-06-01",
		PlanEndDate:      "2026-06-01",
	}, icu.TrainingPlanContext{SportSettings: nil, Wellness: &wellness, Adaptation: nil})

	if len(got.DayAdjustments) == 0 || got.Decision.PrimaryDirective != "recovery_priority" ||
		got.Decision.ADEScore >= 100 {
		t.Fatalf("DayAdjustments/Decision = %+v / %+v, want non-empty rules and reduced ADE", got.DayAdjustments, got.Decision)
	}
}

func TestAnalyzeTrainingPlanAddsExplicitSleepAndRestingHRRules(t *testing.T) {
	t.Parallel()

	event := plannedWorkout("2026-06-01T08:00:00", "VO2 Key", 110, 3600, 0.9)

	var baseline icu.Wellness
	baseline.ID = "2026-05-30"
	baseline.RestingHR = 45
	baseline.SleepScore = 85

	var latest icu.Wellness
	latest.ID = "2026-05-31"
	latest.RestingHR = 55
	latest.SleepScore = 70

	wellness := icu.AnalyzeWellness([]icu.Wellness{baseline, latest}, icu.AnalysisOptions{
		StartDate: "2026-05-30",
		EndDate:   "2026-05-31",
	})

	got := icu.AnalyzeTrainingPlanWithContext(nil, []icu.Event{event}, icu.TrainingPlanOptions{
		HistoryStartDate: "2026-05-01",
		HistoryEndDate:   "2026-05-31",
		PlanStartDate:    "2026-06-01",
		PlanEndDate:      "2026-06-01",
	}, icu.TrainingPlanContext{SportSettings: nil, Wellness: &wellness, Adaptation: nil})

	if !containsAdjustmentCondition(got.DayAdjustments, "sleep_score_watch") ||
		!containsAdjustmentCondition(got.DayAdjustments, "resting_hr_delta_watch") {
		t.Fatalf("DayAdjustments = %+v, want sleep and resting-HR gates", got.DayAdjustments)
	}
}

func TestAnalyzeTrainingPlanUsesAdaptationInDecisionGuidance(t *testing.T) {
	t.Parallel()

	adaptation := icu.CyclingAdaptationAnalysis{
		Scope: icu.AdaptationScope{StartDate: "", EndDate: "", Curves: 2, Activities: 0},
		PowerAnchors: icu.AdaptationPowerAnchors{
			FTP:           285,
			CriticalPower: 292,
			WPrime:        22000,
			PMax:          1050,
			Source:        "mmp_model",
		},
		PowerCurveDeltas: nil,
		SystemStatus: icu.AdaptationSystemStatus{
			Status:        "declining",
			Improved:      0,
			Stable:        1,
			Declined:      2,
			PrimarySignal: "5m_declined",
			Source:        "power_curve_comparison",
		},
		Lactate: icu.WellnessLactateCalibration{
			Mean:            0,
			Latest:          0,
			Trend7Day:       0,
			Samples:         0,
			CoveragePercent: 0,
			State:           "",
			Source:          "",
		},
		PhaseSummary: icu.AdaptationPhaseSummary{
			Weeks:        0,
			RecentLoad:   0,
			PreviousLoad: 0,
			Trend:        "",
			Phase:        "",
			Source:       "",
		},
		Warnings: nil,
	}

	got := icu.AnalyzeTrainingPlanWithContext(nil, nil, icu.TrainingPlanOptions{
		HistoryStartDate: "",
		HistoryEndDate:   "",
		PlanStartDate:    "",
		PlanEndDate:      "",
	}, icu.TrainingPlanContext{SportSettings: nil, Wellness: nil, Adaptation: &adaptation})

	if got.Decision.PrimaryDirective != "protect_target_event" || got.Decision.ADEScore >= 100 {
		t.Fatalf("Decision = %+v, want adaptation-driven target protection and ADE penalty", got.Decision)
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

func plannedTarget(date, name, category string) icu.Event {
	var event icu.Event
	event.StartDateLocal = date
	event.Category = category
	event.Type = testRideType
	event.Name = name

	return event
}

func containsAdjustmentCondition(adjustments []icu.TrainingPlanDayAdjustment, condition string) bool {
	for index := range adjustments {
		if adjustments[index].Condition == condition {
			return true
		}
	}

	return false
}

func completedRide(date string, load, movingTime int) icu.Activity {
	var activity icu.Activity
	activity.Type = testRideType
	activity.StartDateLocal = date
	activity.TrainingLoad = load
	activity.MovingTime = movingTime

	return activity
}
