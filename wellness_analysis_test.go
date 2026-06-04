package icu_test

import (
	"fmt"
	"testing"

	icu "github.com/Thejuampi/icu"
)

const (
	testWellnessStartDate  = "2026-05-01"
	testWellnessSecondDate = "2026-05-02"
)

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

func TestAnalyzeWellnessTrend7DayWithEnoughSamples(t *testing.T) {
	t.Parallel()

	var records []icu.Wellness

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
	first.ID = "2026-05-01"
	first.Lactate = 5.0

	got := icu.AnalyzeWellness([]icu.Wellness{first}, icu.AnalysisOptions{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-01",
	})

	if got.Lactate.State != "high" {
		t.Fatalf("expected high state for lactate 5.0, got %q", got.Lactate.State)
	}
}

func TestAnalyzeWellnessLactateStateBaseline(t *testing.T) {
	t.Parallel()

	var first icu.Wellness
	first.ID = "2026-05-01"
	first.Lactate = 1.0

	got := icu.AnalyzeWellness([]icu.Wellness{first}, icu.AnalysisOptions{
		StartDate: "2026-05-01",
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
	first.ID = "2026-05-01"
	first.Fatigue = 3
	first.Stress = 0
	first.Soreness = 5
	first.Motivation = 4

	got := icu.AnalyzeWellness([]icu.Wellness{first}, icu.AnalysisOptions{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-01",
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
