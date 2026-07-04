package icu_test

import (
	"fmt"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

const (
	testWellnessStartDate  = "2026-05-01"
	testWellnessSecondDate = "2026-05-02"
)

func wellnessRecordWithHRV(dayOffset int, hrv float64) icu.Wellness {
	date := time.Date(2026, time.May, dayOffset, 0, 0, 0, 0, time.UTC)

	return icu.Wellness{
		ID:         date.Format("2006-01-02"),
		HRV:        hrv,
		RestingHR:  50,
		SleepScore: 85,
	}
}

func wellnessRecordWithPreferredScore(date string, preferredName string, preferredValue float64, sleepScore float64) icu.Wellness {
	return icu.Wellness{
		ID:         date,
		RestingHR:  50,
		SleepScore: sleepScore,
		PreferredScore: icu.NamedWellnessScore{
			Name:  preferredName,
			Value: preferredValue,
		},
	}
}

func TestAnalyzeWellnessComputesHRV(t *testing.T) {
	t.Parallel()

	var first icu.Wellness
	first.ID = testWellnessStartDate
	first.HRV = 50

	var second icu.Wellness
	second.ID = testWellnessSecondDate
	second.HRV = 40

	got := icu.AnalyzeWellness([]icu.Wellness{first, second}, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   testWellnessSecondDate,
	})

	if got.HRV.Mean != 45 || got.HRV.Latest != 40 || got.HRV.Ratio != 0.89 || got.HRV.CoveragePercent != 100 {
		t.Fatalf("HRV = %+v, want mean 45 latest 40 ratio 0.89 coverage 100", got.HRV)
	}
}

func TestAnalyzeWellnessComputesCoverage(t *testing.T) {
	t.Parallel()

	var first icu.Wellness
	first.ID = testWellnessStartDate
	first.HRV = 50
	first.RestingHR = 48
	first.SleepScore = 80

	var second icu.Wellness
	second.ID = "2026-05-03"
	second.HRV = 42

	got := icu.AnalyzeWellness([]icu.Wellness{first, second}, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   "2026-05-03",
	})

	if got.Coverage.HRV != 66.67 || got.Coverage.RestingHR != 33.33 || got.Coverage.Sleep != 33.33 {
		t.Fatalf("Coverage = %+v, want HRV 66.67 RHR 33.33 Sleep 33.33", got.Coverage)
	}
}

func TestAnalyzeWellnessComputesPhysiologyState(t *testing.T) {
	t.Parallel()

	var first icu.Wellness
	first.ID = testWellnessStartDate
	first.HRV = 50
	first.RestingHR = 45
	first.SleepScore = 85

	var second icu.Wellness
	second.ID = testWellnessSecondDate
	second.HRV = 43
	second.RestingHR = 55
	second.SleepScore = 70
	second.CTL = 53
	second.ATL = 67

	got := icu.AnalyzeWellness([]icu.Wellness{first, second}, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   testWellnessSecondDate,
	})

	if got.State.State != "WATCH" || got.State.Confidence != "high" || got.Load.TSB != -14 {
		t.Fatalf("State = %+v Load = %+v, want WATCH high TSB -14", got.State, got.Load)
	}
}

func TestAnalyzeWellnessPrefersNamedScoreOverSleepScore(t *testing.T) {
	t.Parallel()

	records := []icu.Wellness{
		wellnessRecordWithPreferredScore(testWellnessStartDate, "zepp_hybridcharge", 92, 60),
		wellnessRecordWithPreferredScore(testWellnessSecondDate, "zepp_hybridcharge", 88, 55),
	}

	got := icu.AnalyzeWellness(records, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   testWellnessSecondDate,
	})

	if got.Sleep.Latest != 88 || got.Sleep.Mean != 90 || got.Sleep.ScoreName != "zepp_hybridcharge" || got.State.State != "OK" {
		t.Fatalf("Sleep = %+v State = %+v, want latest 88 mean 90 score zepp_hybridcharge state OK", got.Sleep, got.State)
	}
}

func TestAnalyzeWellnessFallsBackToSleepScoreWhenNamedScoreMissing(t *testing.T) {
	t.Parallel()

	records := []icu.Wellness{
		{ID: testWellnessStartDate, RestingHR: 50, SleepScore: 82},
		{ID: testWellnessSecondDate, RestingHR: 49, SleepScore: 78},
	}

	got := icu.AnalyzeWellness(records, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   testWellnessSecondDate,
	})

	if got.Sleep.Latest != 78 || got.Sleep.ScoreName != "sleepScore" || got.Sleep.FallbackScoreName != "" {
		t.Fatalf("Sleep = %+v, want latest 78 primary sleepScore with no fallback", got.Sleep)
	}
}

func TestAnalyzeWellnessReportsNamedScoreFallbackUsage(t *testing.T) {
	t.Parallel()

	records := []icu.Wellness{
		wellnessRecordWithPreferredScore(testWellnessStartDate, "zepp_hybridcharge", 90, 75),
		{ID: testWellnessSecondDate, RestingHR: 49, SleepScore: 72},
	}

	got := icu.AnalyzeWellness(records, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   testWellnessSecondDate,
	})

	if got.Sleep.ScoreName != "zepp_hybridcharge" || got.Sleep.FallbackScoreName != "sleepScore" || len(got.Warnings) == 0 {
		t.Fatalf("Sleep = %+v Warnings = %v, want zepp_hybridcharge with sleepScore fallback warning", got.Sleep, got.Warnings)
	}
}

func TestAnalyzeWellnessKeepsHRVOKWhenLatestRatioDropsButRecentMeanIsNormal(t *testing.T) {
	t.Parallel()

	var records []icu.Wellness
	var day int
	for day = 1; day <= 35; day++ {
		records = append(records, wellnessRecordWithHRV(day, 44))
	}
	for day = 36; day <= 41; day++ {
		records = append(records, wellnessRecordWithHRV(day, 44))
	}
	records = append(records, wellnessRecordWithHRV(42, 42))

	got := icu.AnalyzeWellness(records, icu.AnalysisOptions{
		StartDate: "2026-05-01",
		EndDate:   "2026-06-11",
	})

	if got.State.State != "OK" {
		t.Fatalf("State = %+v, want OK", got.State)
	}
}

func TestAnalyzeWellnessUsesRobustHRVZScoreWithBaselineOutlier(t *testing.T) {
	t.Parallel()

	var records []icu.Wellness
	var day int
	for day = 1; day <= 17; day++ {
		records = append(records, wellnessRecordWithHRV(day, 49))
	}
	records = append(records, wellnessRecordWithHRV(18, 10))
	for day = 19; day <= 35; day++ {
		records = append(records, wellnessRecordWithHRV(day, 51))
	}
	for day = 36; day <= 42; day++ {
		records = append(records, wellnessRecordWithHRV(day, 45))
	}

	got := icu.AnalyzeWellness(records, icu.AnalysisOptions{
		StartDate: "2026-05-01",
		EndDate:   "2026-06-11",
	})

	if got.HRV.ZScore >= -1 {
		t.Fatalf("HRV z-score = %f, want robust negative deviation", got.HRV.ZScore)
	}
}

func TestAnalyzeWellnessTrend7DayWithEnoughSamples(t *testing.T) {
	t.Parallel()

	records := make([]icu.Wellness, 0, 21)

	for i := range 21 {
		records = append(records, icu.Wellness{
			ID:      fmt.Sprintf("2026-05-%02d", i+1),
			Lactate: 1.0 + float64(i)*0.1,
		})
	}

	got := icu.AnalyzeWellness(records, icu.AnalysisOptions{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-21",
	})

	if got.Lactate.Trend7Day == 0 {
		t.Fatalf("expected non-zero trend with 21 samples, got %f", got.Lactate.Trend7Day)
	}
}

func TestAnalyzeWellnessLactateStateHigh(t *testing.T) {
	t.Parallel()

	var first icu.Wellness
	first.ID = testWellnessStartDate
	first.Lactate = 5.0

	got := icu.AnalyzeWellness([]icu.Wellness{first}, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   "2026-05-01",
	})

	if got.Lactate.State != "high" {
		t.Fatalf("expected high state for lactate 5.0, got %q", got.Lactate.State)
	}
}

func TestAnalyzeWellnessLactateStateBaseline(t *testing.T) {
	t.Parallel()

	var first icu.Wellness
	first.ID = testWellnessStartDate
	first.Lactate = 1.0

	got := icu.AnalyzeWellness([]icu.Wellness{first}, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   "2026-05-01",
	})

	if got.Lactate.State != "baseline" {
		t.Fatalf("expected baseline state for lactate 1.0, got %q", got.Lactate.State)
	}
}

func TestAnalyzeWellnessTotalDaysFallsBackToUniqueDays(t *testing.T) {
	t.Parallel()

	records := []icu.Wellness{
		{ID: "2026-05-01", HRV: 50},
		{ID: "2026-05-01", HRV: 52},
		{ID: "2026-05-02", HRV: 48},
	}

	got := icu.AnalyzeWellness(records, icu.AnalysisOptions{
		StartDate: "9999-99-99",
		EndDate:   "9999-99-99",
	})

	if got.Scope.TotalDays <= 0 {
		t.Fatalf("expected >0 total days, got %d", got.Scope.TotalDays)
	}
}

func TestCoveragePercentReturnsZeroForZeroTotalDays(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeWellness(nil, icu.AnalysisOptions{
		StartDate: "",
		EndDate:   "",
	})

	if got.Scope.TotalDays != 0 {
		t.Fatalf("expected 0 total days with empty range, got %d", got.Scope.TotalDays)
	}
}

func TestAnalyzeWellnessAddSubjectiveMetricTracksValues(t *testing.T) {
	t.Parallel()

	var first icu.Wellness
	first.ID = testWellnessStartDate
	first.Fatigue = 3
	first.Stress = 0
	first.Soreness = 5
	first.Motivation = 4

	got := icu.AnalyzeWellness([]icu.Wellness{first}, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   testWellnessStartDate,
	})

	if got.Subjective.MeanFatigue == 0 {
		t.Fatalf("expected non-zero fatigue, got %+v", got.Subjective)
	}
}

func TestDaysBetweenReturnsZeroForInvalidDates(t *testing.T) {
	t.Parallel()

	records := []icu.Wellness{
		{ID: "2026-05-01", HRV: 50},
	}

	got := icu.AnalyzeWellness(records, icu.AnalysisOptions{
		StartDate: "9999-99-99",
		EndDate:   "9999-98-98",
	})

	if got.Scope.TotalDays != 1 {
		t.Fatalf("expected 1 total day (unique), got %d", got.Scope.TotalDays)
	}
}

func TestAnalyzeWellnessComputesLactateCalibration(t *testing.T) {
	t.Parallel()

	var first icu.Wellness
	first.ID = testWellnessStartDate
	first.Lactate = 1.2

	var second icu.Wellness
	second.ID = testWellnessSecondDate
	second.Lactate = 3.1

	got := icu.AnalyzeWellness([]icu.Wellness{first, second}, icu.AnalysisOptions{
		StartDate: testWellnessStartDate,
		EndDate:   "2026-05-04",
	})
	want := icu.WellnessLactateCalibration{
		Mean:            2.15,
		Latest:          3.1,
		Trend7Day:       0,
		Samples:         2,
		CoveragePercent: 50,
		State:           "watch",
		Source:          "wellness_lactate",
	}

	if got.Lactate != want {
		t.Fatalf("Lactate = %+v, want %+v", got.Lactate, want)
	}
}

func TestAnalyzeWellnessIncludesTimezoneMetadata(t *testing.T) {
	t.Parallel()

	var record icu.Wellness
	record.ID = testWellnessStartDate
	record.HRV = 50

	got := icu.AnalyzeWellness([]icu.Wellness{record}, icu.AnalysisOptions{
		StartDate:      testWellnessStartDate,
		EndDate:        testWellnessSecondDate,
		Timezone:       icu.DefaultAnalysisTimezone,
		TimezoneSource: icu.DefaultAnalysisTimezoneSource,
	})

	if got.Scope.Timezone != icu.DefaultAnalysisTimezone {
		t.Fatalf("Scope.Timezone = %q, want %s", got.Scope.Timezone, icu.DefaultAnalysisTimezone)
	}

	if got.Scope.TimezoneSource != icu.DefaultAnalysisTimezoneSource {
		t.Fatalf("Scope.TimezoneSource = %q, want %s", got.Scope.TimezoneSource, icu.DefaultAnalysisTimezoneSource)
	}
}
