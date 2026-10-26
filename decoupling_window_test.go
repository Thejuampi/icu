package icu

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func decouplingRides(start time.Time, days int, values ...float64) []Activity {
	rides := make([]Activity, 0, days)
	for day := range days {
		rides = append(rides, Activity{
			Type: "Ride", MovingTime: 3600, TrainingLoad: 50,
			StartDateLocal: start.AddDate(0, 0, day).Format(analysisDateLayout) + "T08:00:00",
			Decoupling:     values[day%len(values)],
		})
	}

	return rides
}

func TestRecentDecouplingUsesFixedPriorWindow(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := "2026-10-05"
	// 28 baseline days at 2-4%, then the last 7 days (09-29..10-05) at 9%.
	activities := append(decouplingRides(start, 28, 2, 3, 4), decouplingRides(start.AddDate(0, 0, 28), 7, 9, 9.5)...)
	options := TrainingPlanOptions{HistoryStartDate: "2026-09-01", HistoryEndDate: end}

	history := summarizeTrainingPlanHistory(activities, options, nil)
	threshold := history.DecouplingThreshold
	if threshold == nil || threshold.Samples != 28 || threshold.Baseline != 3 {
		t.Fatalf("threshold = %+v: baseline must be the 28 rides before the last 7 days", threshold)
	}
	if history.RecentDecoupling < 9 ||
		thresholdLevel(history.RecentDecoupling, threshold, baselineHighIsBad) != BaselineLevelRed {
		t.Fatalf("recent = %.2f threshold = %+v, want RED", history.RecentDecoupling, threshold)
	}

	// The high rides are two weeks old and the last 7 days are normal: not RED.
	old := append(decouplingRides(start, 21, 2, 3, 4), decouplingRides(start.AddDate(0, 0, 21), 7, 9, 9.5)...)
	old = append(old, decouplingRides(start.AddDate(0, 0, 28), 7, 3)...)
	history = summarizeTrainingPlanHistory(old, options, nil)
	if level := thresholdLevel(history.RecentDecoupling, history.DecouplingThreshold, baselineHighIsBad); level == BaselineLevelRed {
		t.Fatalf("recent = %.2f level = %s: an old week must not be judged as recent", history.RecentDecoupling, level)
	}

	// No rides in the last 7 days: no judgement.
	history = summarizeTrainingPlanHistory(decouplingRides(start, 28, 2, 3, 4), options, nil)
	if history.RecentDecoupling != 0 || history.DecouplingThreshold != nil {
		t.Fatalf("history = %+v", history)
	}
}

func TestWatchOnlyOverrideNeverEscalatesToRed(t *testing.T) {
	t.Parallel()
	watch := 5.0
	threshold := deriveThreshold(nil, baselineHighIsBad, thresholdMethod(nil), &watch, nil)
	if level := thresholdLevel(50, threshold, baselineHighIsBad); level != BaselineLevelWatch {
		t.Fatalf("level = %s, want WATCH", level)
	}
	red := 5.0
	threshold = deriveThreshold(nil, baselineHighIsBad, thresholdMethod(nil), nil, &red)
	if level := thresholdLevel(6, threshold, baselineHighIsBad); level != BaselineLevelRed {
		t.Fatalf("level = %s, want RED", level)
	}
}

func TestValidateThresholdsRejectsWatchZAtOrAboveRedZ(t *testing.T) {
	t.Parallel()
	for _, thresholds := range []TrainingThresholds{{WatchZ: 3}, {WatchZ: 2, RedZ: 2}, {RedZ: 1}} {
		if err := validateTrainingThresholds(&thresholds); err == nil {
			t.Fatalf("%+v: expected error", thresholds)
		}
	}
	if err := validateTrainingThresholds(&TrainingThresholds{WatchZ: 1, RedZ: 2}); err != nil {
		t.Fatal(err)
	}
}

func TestWellnessBaselinesUseFixedPriorWindow(t *testing.T) {
	t.Parallel()
	end := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	records := make([]Wellness, 0, 120)
	for day := 119; day >= 0; day-- {
		date := end.AddDate(0, 0, -day)
		hr := 50 + day%3
		if day >= 49 {
			hr = 70 // older than the window: must not shift the baseline
		}
		if day < 7 {
			hr = 58 // recent week: must not sit in its own baseline
		}
		records = append(records, Wellness{ID: date.Format(analysisDateLayout), RestingHR: hr})
	}
	analyze := func(oldest int) *DerivedThreshold {
		window := records[len(records)-oldest:]
		analysis := AnalyzeWellness(window, AnalysisOptions{
			StartDate: window[0].ID, EndDate: window[len(window)-1].ID,
		})

		return analysis.RestingHR.Threshold
	}
	short, long := analyze(49), analyze(120)
	if short == nil || long == nil || *short != *long || short.Baseline != 51 || short.Samples != 42 {
		t.Fatalf("thresholds depend on --oldest or include the recent week: %+v vs %+v", short, long)
	}
}

func TestWellnessStateInsufficientWithoutBaselines(t *testing.T) {
	t.Parallel()
	var records []Wellness
	for day := 20; day <= 26; day++ {
		records = append(records, Wellness{
			ID: fmt.Sprintf("2026-10-%02d", day), HRV: 50, RestingHR: 50, SleepScore: 80, CTL: 40, ATL: 45,
		})
	}
	got := AnalyzeWellness(records, AnalysisOptions{StartDate: "2026-10-20", EndDate: "2026-10-26"})
	if got.State.State != PhysiologyInsufficient || got.State.Confidence != "low" ||
		!strings.Contains(strings.Join(got.Warnings, "|"), "INSUFFICIENT") ||
		!strings.Contains(strings.Join(got.Warnings, "|"), "data starts 2026-10-20") {
		t.Fatalf("state %+v warnings %v", got.State, got.Warnings)
	}
}

func TestWellnessBaselineUsesHistoryBeforeShortScope(t *testing.T) {
	t.Parallel()
	end := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	var records []Wellness
	for day := 60; day >= 0; day-- {
		records = append(records, Wellness{ID: end.AddDate(0, 0, -day).Format(analysisDateLayout), RestingHR: 50 + day%3})
	}
	full := AnalyzeWellness(records, AnalysisOptions{StartDate: records[0].ID, EndDate: "2026-10-26"})
	short := AnalyzeWellness(records, AnalysisOptions{StartDate: "2026-10-20", EndDate: "2026-10-26"})
	if short.RestingHR.Threshold == nil || full.RestingHR.Threshold == nil ||
		*short.RestingHR.Threshold != *full.RestingHR.Threshold || short.Scope.Records != 7 ||
		short.RestingHR.Samples != 7 || len(short.Warnings) != 0 {
		t.Fatalf("short %+v (%v) vs full %+v", short.RestingHR, short.Warnings, full.RestingHR)
	}
}
