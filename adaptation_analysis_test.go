package icu_test

import (
	"reflect"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestAnalyzeCyclingAdaptationComparesPowerCurves(t *testing.T) {
	t.Parallel()

	current := icu.DataCurve{
		ID:                "42d",
		Label:             "42d",
		StartDate:         "2026-04-21",
		EndDate:           "2026-06-01",
		Days:              42,
		MovingTime:        7200,
		TrainingLoad:      180,
		Weight:            80,
		InputPointIndexes: []int{0, 1, 2},
		Secs:              []int{60, 300, 1200},
		Values:            []int{520, 410, 330},
		Distance:          nil,
	}
	baseline := icu.DataCurve{
		ID:                "365d",
		Label:             "1y",
		StartDate:         "2025-06-02",
		EndDate:           "2026-06-01",
		Days:              365,
		MovingTime:        54000,
		TrainingLoad:      1200,
		Weight:            80,
		InputPointIndexes: []int{3, 4, 5},
		Secs:              []int{60, 300, 1200},
		Values:            []int{500, 420, 320},
		Distance:          nil,
	}
	model := icu.PowerModel{Type: testRideType, CriticalPower: 296, WPrime: 24000, PMax: 1120, FTP: 290}

	got := icu.AnalyzeCyclingAdaptation(
		[]icu.DataCurve{baseline, current},
		model,
		nil,
		nil,
		nil,
		icu.AnalysisOptions{StartDate: "2026-04-21", EndDate: "2026-06-01"},
	)
	wantAnchors := icu.AdaptationPowerAnchors{
		FTP:           290,
		CriticalPower: 296,
		WPrime:        24000,
		PMax:          1120,
		Source:        "mmp_model",
	}
	wantDeltas := []icu.PowerCurveDelta{
		{
			DurationSecs:  60,
			CurrentWatts:  520,
			BaselineWatts: 500,
			DeltaWatts:    20,
			DeltaPercent:  4,
			Status:        "improved",
			CurrentCurve:  "42d",
			BaselineCurve: "1y",
		},
		{
			DurationSecs:  300,
			CurrentWatts:  410,
			BaselineWatts: 420,
			DeltaWatts:    -10,
			DeltaPercent:  -2.38,
			Status:        "stable",
			CurrentCurve:  "42d",
			BaselineCurve: "1y",
		},
		{
			DurationSecs:  1200,
			CurrentWatts:  330,
			BaselineWatts: 320,
			DeltaWatts:    10,
			DeltaPercent:  3.13,
			Status:        "improved",
			CurrentCurve:  "42d",
			BaselineCurve: "1y",
		},
	}

	if got.PowerAnchors != wantAnchors || !reflect.DeepEqual(got.PowerCurveDeltas, wantDeltas) ||
		got.SystemStatus.Status != "improving" {
		t.Fatalf("Adaptation = %+v, want anchors %+v deltas %+v improving", got, wantAnchors, wantDeltas)
	}
}

func TestAnalyzeCyclingAdaptationSummarizesPhaseAndLactate(t *testing.T) {
	t.Parallel()

	var firstWeek icu.Activity
	firstWeek.Type = testRideType
	firstWeek.StartDateLocal = "2026-05-04T08:00:00"
	firstWeek.TrainingLoad = 200

	var secondWeek icu.Activity
	secondWeek.Type = testRideType
	secondWeek.StartDateLocal = "2026-05-11T08:00:00"
	secondWeek.TrainingLoad = 240

	var thirdWeek icu.Activity
	thirdWeek.Type = testRideType
	thirdWeek.StartDateLocal = "2026-05-18T08:00:00"
	thirdWeek.TrainingLoad = 280

	wellness := icu.WellnessAnalysis{
		Scope: icu.WellnessScope{
			Records:   2,
			TotalDays: 4,
			StartDate: "2026-05-01",
			EndDate:   "2026-05-04",
		},
		Coverage:  icu.WellnessCoverage{HRV: 0, RestingHR: 0, Sleep: 0, Subjective: 0},
		HRV:       icu.WellnessSignal{Mean: 0, Latest: 0, Ratio: 0, Delta: 0, Trend7Day: 0, Samples: 0, CoveragePercent: 0},
		RestingHR: icu.WellnessSignal{Mean: 0, Latest: 0, Ratio: 0, Delta: 0, Trend7Day: 0, Samples: 0, CoveragePercent: 0},
		Sleep:     icu.WellnessSignal{Mean: 0, Latest: 0, Ratio: 0, Delta: 0, Trend7Day: 0, Samples: 0, CoveragePercent: 0},
		Lactate: icu.WellnessLactateCalibration{
			Mean:            2.1,
			Latest:          2.8,
			Trend7Day:       0,
			Samples:         2,
			CoveragePercent: 50,
			State:           "watch",
			Source:          "wellness_lactate",
		},
		Subjective: icu.SubjectiveWellness{Samples: 0, CoveragePercent: 0, MeanFatigue: 0, MeanStress: 0, MeanSoreness: 0, MeanMotivation: 0},
		Load:       icu.WellnessLoadState{CTL: 0, ATL: 0, TSB: 0},
		State:      icu.PhysiologyState{State: "", Confidence: "", Source: "", Reasons: nil},
		Warnings:   nil,
	}

	settings := icu.SportSettings{
		ID:            0,
		AthleteID:     "",
		Types:         nil,
		FTP:           285,
		IndoorFTP:     0,
		WPrime:        22000,
		PMax:          1050,
		LTHR:          0,
		MaxHR:         0,
		PowerZones:    nil,
		HRZones:       nil,
		PaceZones:     nil,
		ThresholdPace: 0,
		PaceUnits:     "",
		HRLoadType:    "",
		PaceLoadType:  "",
		GapModel:      "",
	}

	got := icu.AnalyzeCyclingAdaptation(
		nil,
		icu.PowerModel{Type: "", CriticalPower: 0, WPrime: 0, PMax: 0, FTP: 0},
		&settings,
		[]icu.Activity{firstWeek, secondWeek, thirdWeek},
		&wellness,
		icu.AnalysisOptions{StartDate: "2026-05-04", EndDate: "2026-05-24"},
	)
	wantPhase := icu.AdaptationPhaseSummary{
		Weeks:        3,
		RecentLoad:   280,
		PreviousLoad: 240,
		Trend:        "increasing",
		Phase:        "build",
		Source:       "activity_weekly_load_segments",
	}

	if got.PhaseSummary != wantPhase || got.Lactate != wellness.Lactate || got.PowerAnchors.Source != "sport_settings" {
		t.Fatalf("Adaptation = %+v, want phase %+v lactate passthrough and sport anchors", got, wantPhase)
	}
}

func TestAnalyzeCyclingAdaptationClassifiesDecliningCurveAndRecoveryPhase(t *testing.T) {
	t.Parallel()

	current := icu.DataCurve{
		ID:                "current",
		Label:             "",
		StartDate:         "2026-05-01",
		EndDate:           "2026-06-01",
		Days:              0,
		MovingTime:        3600,
		TrainingLoad:      120,
		Weight:            80,
		InputPointIndexes: nil,
		Secs:              []int{45, 3600},
		Values:            []int{100, 250},
		Distance:          nil,
	}
	baseline := icu.DataCurve{
		ID:                "baseline",
		Label:             "",
		StartDate:         "2025-06-02",
		EndDate:           "2026-06-01",
		Days:              0,
		MovingTime:        7200,
		TrainingLoad:      240,
		Weight:            80,
		InputPointIndexes: nil,
		Secs:              []int{45, 3600},
		Values:            []int{120, 300},
		Distance:          nil,
	}

	var peakWeek icu.Activity
	peakWeek.Type = testRideType
	peakWeek.StartDateLocal = "2026-05-04T08:00:00"
	peakWeek.TrainingLoad = 300

	var recoveryWeek icu.Activity
	recoveryWeek.Type = testRideType
	recoveryWeek.StartDateLocal = "2026-05-11T08:00:00"
	recoveryWeek.TrainingLoad = 200

	got := icu.AnalyzeCyclingAdaptation(
		[]icu.DataCurve{baseline, current},
		icu.PowerModel{Type: "", CriticalPower: 0, WPrime: 0, PMax: 0, FTP: 0},
		nil,
		[]icu.Activity{peakWeek, recoveryWeek},
		nil,
		icu.AnalysisOptions{StartDate: "2026-05-04", EndDate: "2026-05-17"},
	)

	if got.SystemStatus.Status != "declining" || got.SystemStatus.PrimarySignal != "45s_declined" ||
		got.PowerCurveDeltas[1].CurrentCurve != "current" || got.PowerCurveDeltas[1].BaselineCurve != "baseline" ||
		got.PowerCurveDeltas[1].Status != "declined" || got.PhaseSummary.Phase != "recovery" {
		t.Fatalf("Adaptation = %+v, want declining curve and recovery phase", got)
	}
}

func TestAnalyzeCyclingAdaptationWarnsWithoutCurveComparison(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeCyclingAdaptation(
		nil,
		icu.PowerModel{Type: "", CriticalPower: 0, WPrime: 0, PMax: 0, FTP: 0},
		nil,
		nil,
		nil,
		icu.AnalysisOptions{StartDate: "", EndDate: ""},
	)

	if got.PowerAnchors.Source != "" || got.SystemStatus.Status != "unknown" || len(got.Warnings) != 1 {
		t.Fatalf("Adaptation = %+v, want empty anchors, unknown status, and warning", got)
	}
}

func TestAnalyzeCyclingAdaptationIncludesTimezoneMetadata(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeCyclingAdaptation(
		nil,
		icu.PowerModel{Type: "", CriticalPower: 0, WPrime: 0, PMax: 0, FTP: 0},
		nil,
		nil,
		nil,
		icu.AnalysisOptions{
			StartDate:      "2026-04-21",
			EndDate:        "2026-06-01",
			Timezone:       icu.DefaultAnalysisTimezone,
			TimezoneSource: icu.DefaultAnalysisTimezoneSource,
		},
	)

	if got.Scope.Timezone != icu.DefaultAnalysisTimezone {
		t.Fatalf("Scope.Timezone = %q, want %s", got.Scope.Timezone, icu.DefaultAnalysisTimezone)
	}

	if got.Scope.TimezoneSource != icu.DefaultAnalysisTimezoneSource {
		t.Fatalf("Scope.TimezoneSource = %q, want %s", got.Scope.TimezoneSource, icu.DefaultAnalysisTimezoneSource)
	}
}
