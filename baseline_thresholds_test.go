package icu_test

import (
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

func ptr(value float64) *float64 { return &value }

// pinnedThresholds pins every bound through the profile override path, so
// rule tests with little history stay deterministic.
func pinnedThresholds() *icu.TrainingThresholds {
	return &icu.TrainingThresholds{
		HRVWatch: ptr(45), HRVRed: ptr(40),
		RestingHRWatch: ptr(55), RestingHRRed: ptr(58),
		SleepWatch: ptr(75), SleepRed: ptr(65),
		LactateWatch: ptr(2.5), LactateHigh: ptr(4),
		TSBWatch: ptr(-10), TSBRed: ptr(-25),
		DecouplingWatchPct: ptr(5), AdaptationChangePct: ptr(3),
	}
}

func dailyWellness(values []float64, set func(*icu.Wellness, float64)) []icu.Wellness {
	records := make([]icu.Wellness, 0, len(values))
	for index, value := range values {
		record := icu.Wellness{ID: dayID(1 + index)}
		set(&record, value)
		records = append(records, record)
	}

	return records
}

func dayID(day int) string {
	return time.Date(2026, time.May, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func repeatPattern(pattern []float64, count int) []float64 {
	values := make([]float64, 0, count)
	for len(values) < count {
		values = append(values, pattern...)
	}

	return values[:count]
}

func TestRestingHRLevelDerivesFromAthleteBaseline(t *testing.T) {
	t.Parallel()

	// Baseline 50±1 bpm: median 50, MAD 1 → scale 1.48; watch ≈ 52.22, red ≈ 53.71.
	base := repeatPattern([]float64{49, 50, 51}, 20)
	cases := map[float64]string{50: icu.BaselineLevelOK, 53: icu.BaselineLevelWatch, 55: icu.BaselineLevelRed}
	for latest, want := range cases {
		records := dailyWellness(append(append([]float64(nil), base...), latest), func(w *icu.Wellness, v float64) {
			w.RestingHR = int(v)
		})
		got := icu.AnalyzeWellness(records, icu.AnalysisOptions{})
		if got.RestingHR.Level != want || got.RestingHR.Threshold == nil || got.RestingHR.Threshold.Source != "baseline_mad" {
			t.Fatalf("latest %v: level = %q threshold = %+v, want %q", latest, got.RestingHR.Level, got.RestingHR.Threshold, want)
		}
	}
	// The same 53 bpm is OK for an athlete whose resting HR normally swings ±4.
	noisy := dailyWellness(append(repeatPattern([]float64{46, 50, 54}, 20), 53), func(w *icu.Wellness, v float64) {
		w.RestingHR = int(v)
	})
	if got := icu.AnalyzeWellness(noisy, icu.AnalysisOptions{}); got.RestingHR.Level != icu.BaselineLevelOK {
		t.Fatalf("noisy athlete level = %q, threshold %+v", got.RestingHR.Level, got.RestingHR.Threshold)
	}
}

func TestSleepAndTSBLevelsUseLowSideAndInsufficientHistory(t *testing.T) {
	t.Parallel()

	sleep := dailyWellness(append(repeatPattern([]float64{80, 82, 84}, 20), 70), func(w *icu.Wellness, v float64) {
		w.SleepScore = v
	})
	if got := icu.AnalyzeWellness(sleep, icu.AnalysisOptions{}); got.Sleep.Level != icu.BaselineLevelRed ||
		got.State.State != "RED" {
		t.Fatalf("sleep = %+v state = %+v", got.Sleep, got.State)
	}
	short := dailyWellness([]float64{80, 60}, func(w *icu.Wellness, v float64) { w.SleepScore = v })
	if got := icu.AnalyzeWellness(short, icu.AnalysisOptions{}); got.Sleep.Level != "" || got.Sleep.Threshold != nil {
		t.Fatalf("two samples judged: %+v", got.Sleep)
	}
	tsb := dailyWellness(append(repeatPattern([]float64{-4, -5, -6}, 20), -9), func(w *icu.Wellness, v float64) {
		w.CTL, w.ATL = 50, 50-v
	})
	if got := icu.AnalyzeWellness(tsb, icu.AnalysisOptions{}); got.Load.Level != icu.BaselineLevelRed {
		t.Fatalf("tsb load = %+v", got.Load)
	}
}

func TestThresholdMethodAndProfileOverrides(t *testing.T) {
	t.Parallel()

	records := dailyWellness(append(repeatPattern([]float64{49, 50, 51}, 20), 53), func(w *icu.Wellness, v float64) {
		w.RestingHR = int(v)
	})
	// Wider method parameters relax the derived bound.
	relaxed := icu.AnalyzeWellness(records, icu.AnalysisOptions{Thresholds: &icu.TrainingThresholds{WatchZ: 3, RedZ: 4}})
	if relaxed.RestingHR.Level != icu.BaselineLevelOK {
		t.Fatalf("relaxed = %+v", relaxed.RestingHR)
	}
	// A pinned bound wins over the derived one and is labelled "profile".
	pinned := icu.AnalyzeWellness(records, icu.AnalysisOptions{Thresholds: &icu.TrainingThresholds{RestingHRWatch: ptr(60)}})
	if pinned.RestingHR.Level != icu.BaselineLevelOK || pinned.RestingHR.Threshold.Source != "profile" ||
		pinned.RestingHR.Threshold.Watch != 60 {
		t.Fatalf("pinned = %+v", pinned.RestingHR.Threshold)
	}
	// MinSamples gates judging.
	strict := icu.AnalyzeWellness(records, icu.AnalysisOptions{Thresholds: &icu.TrainingThresholds{MinSamples: 100}})
	if strict.RestingHR.Level != "" {
		t.Fatalf("strict = %+v", strict.RestingHR)
	}
}

func TestHRVLevelJudgesRecentMeanAgainstEarlierBaseline(t *testing.T) {
	t.Parallel()

	values := append(repeatPattern([]float64{58, 60, 62}, 21), repeatPattern([]float64{50}, 7)...)
	records := dailyWellness(values, func(w *icu.Wellness, v float64) { w.HRV = v })
	got := icu.AnalyzeWellness(records, icu.AnalysisOptions{})
	if got.HRV.Level != icu.BaselineLevelRed || got.HRV.Threshold.Baseline != 60 {
		t.Fatalf("hrv = %+v threshold %+v", got.HRV, got.HRV.Threshold)
	}
}

func TestAdaptationChangeThresholdFromAthleteDeltaSpread(t *testing.T) {
	t.Parallel()

	secs := []int{5, 60, 300, 1200, 3600}
	curve := func(id, end string, days int, values []int) icu.DataCurve {
		return icu.DataCurve{ID: id, Label: id, EndDate: end, Days: days, Secs: secs, Values: values}
	}
	curves := []icu.DataCurve{
		curve("current", "2026-06-01", 42, []int{1010, 500, 396, 323, 330}),
		curve("baseline", "2026-04-20", 42, []int{1000, 500, 400, 320, 300}),
	}
	got := icu.AnalyzeCyclingAdaptation(curves, icu.PowerModel{}, nil, nil, nil, icu.AnalysisOptions{})
	if got.PowerCurveChangeThreshold == nil || got.PowerCurveChangeThreshold.Source != "baseline_mad" {
		t.Fatalf("threshold = %+v", got.PowerCurveChangeThreshold)
	}
	status := map[int]string{}
	for _, delta := range got.PowerCurveDeltas {
		status[delta.DurationSecs] = delta.Status
	}
	// ±1% is noise for this athlete; +10% at 60 min clears the derived bound.
	if status[3600] != "improved" || status[5] != "stable" || status[300] != "stable" {
		t.Fatalf("status = %v threshold %+v", status, got.PowerCurveChangeThreshold)
	}
	few := icu.AnalyzeCyclingAdaptation(
		[]icu.DataCurve{curves[0], curves[1]}, icu.PowerModel{}, nil, nil, nil,
		icu.AnalysisOptions{Thresholds: &icu.TrainingThresholds{AdaptationChangePct: ptr(20)}},
	)
	for _, delta := range few.PowerCurveDeltas {
		if delta.Status != "stable" {
			t.Fatalf("pinned 20%% still flagged %+v", delta)
		}
	}
}

func TestForecastStatusUsesWellnessTSBBaseline(t *testing.T) {
	t.Parallel()

	current := icu.Activity{Type: "Ride", StartDateLocal: "2026-05-31T08:00:00", CTL: 70, ATL: 80}
	events := []icu.Event{plannedWorkout("2026-06-01T08:00:00", "VO2", 100, 3600, 0.9)}
	options := icu.TrainingPlanOptions{
		HistoryStartDate: "2026-05-01", HistoryEndDate: "2026-05-31",
		PlanStartDate: "2026-06-01", PlanEndDate: "2026-06-01",
	}
	if got := icu.AnalyzeTrainingPlan([]icu.Activity{current}, events, options); got.Forecast.Status != "no_baseline" {
		t.Fatalf("status without baseline = %q", got.Forecast.Status)
	}
	tsb := dailyWellness(repeatPattern([]float64{-4, -5, -6}, 21), func(w *icu.Wellness, v float64) {
		w.CTL, w.ATL = 70, 70-v
	})
	wellness := icu.AnalyzeWellness(tsb, icu.AnalysisOptions{})
	got := icu.AnalyzeTrainingPlanWithContext([]icu.Activity{current}, events, options,
		icu.TrainingPlanContext{Wellness: &wellness})
	if got.Forecast.Status != "red" || got.Forecast.Threshold == nil || got.Forecast.Threshold.Baseline != -5 {
		t.Fatalf("forecast = %q threshold %+v lowest %v", got.Forecast.Status, got.Forecast.Threshold, got.Forecast.LowestTSB)
	}
}

func TestRecentDecouplingJudgedAgainstEarlierRides(t *testing.T) {
	t.Parallel()

	var activities []icu.Activity
	start := time.Date(2026, time.April, 6, 8, 0, 0, 0, time.UTC)
	pattern := []float64{3, 4, 5, 4}
	for week := range 5 {
		for ride, value := range pattern {
			if week == 4 {
				value = 9
			}
			activities = append(activities, icu.Activity{
				Type: "Ride", StartDateLocal: start.AddDate(0, 0, week*7+ride).Format("2006-01-02T15:04:05"),
				MovingTime: 3600, TrainingLoad: 50, Decoupling: value,
			})
		}
	}
	got := icu.AnalyzeTrainingPlan(activities, nil, icu.TrainingPlanOptions{
		HistoryStartDate: "2026-04-06", HistoryEndDate: "2026-05-10",
	})
	threshold := got.History.DecouplingThreshold
	if got.History.RecentDecoupling != 9 || threshold == nil || threshold.Baseline != 4 || threshold.Watch >= 9 {
		t.Fatalf("recent = %v threshold = %+v", got.History.RecentDecoupling, threshold)
	}
}
