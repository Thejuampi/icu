package icu_test

import (
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
