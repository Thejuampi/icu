package main

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultAnalysisFieldsIncludeReportContractInputs(t *testing.T) {
	t.Parallel()

	required := []string{
		"icu_pm_cp",
		"icu_pm_w_prime",
		"icu_pm_p_max",
		"icu_pm_ftp",
		"icu_rolling_ftp",
		"average_temp",
		"min_temp",
		"max_temp",
		"average_weather_temp",
		"average_feels_like",
		"average_wind_speed",
		"headwind_percent",
		"tailwind_percent",
		"average_altitude",
		"average_gradient",
		"average_lactate",
		"strain_score",
	}

	var missing []string

	for _, field := range required {
		if !strings.Contains(defaultAnalysisFields, field) {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("defaultAnalysisFields missing %v", missing)
	}
}

func TestAnalysisDateRangeUsesExplicitDates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 29, 12, 0, 0, 0, time.UTC)
	got, err := analysisDateRange(map[string]string{
		"oldest": "2026-05-01",
		"newest": "2026-05-29",
	}, now)

	if err != nil || got.Oldest != "2026-05-01" || got.Newest != "2026-05-29" {
		t.Fatalf("analysisDateRange explicit = %+v %v, want 2026-05-01 2026-05-29 nil", got, err)
	}
}

func TestAnalysisDateRangeDefaultsToDays(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 29, 12, 0, 0, 0, time.UTC)
	got, err := analysisDateRange(map[string]string{"days": "7"}, now)

	if err != nil || got.Oldest != "2026-05-23" || got.Newest != "2026-05-29" {
		t.Fatalf("analysisDateRange days = %+v %v, want 2026-05-23 2026-05-29 nil", got, err)
	}
}

func TestTrainingPlanDateRangesDefaultToTwelveWeekHistoryAndNextISOBlock(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 30, 12, 0, 0, 0, time.UTC)
	got, err := trainingPlanDateRanges(map[string]string{}, now)
	want := trainingPlanRanges{
		History: analysisRange{Oldest: "2026-03-08", Newest: "2026-05-30"},
		Plan:    analysisRange{Oldest: "2026-06-01", Newest: "2026-06-28"},
	}

	if err != nil || got != want {
		t.Fatalf("trainingPlanDateRanges default = %+v %v, want %+v nil", got, err, want)
	}
}

func TestTrainingPlanDateRangesUseExplicitDates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 30, 12, 0, 0, 0, time.UTC)
	got, err := trainingPlanDateRanges(map[string]string{
		"history-oldest": "2026-03-01",
		"history-newest": "2026-05-30",
		"plan-oldest":    "2026-06-01",
		"plan-newest":    "2026-06-28",
	}, now)
	want := trainingPlanRanges{
		History: analysisRange{Oldest: "2026-03-01", Newest: "2026-05-30"},
		Plan:    analysisRange{Oldest: "2026-06-01", Newest: "2026-06-28"},
	}

	if err != nil || got != want {
		t.Fatalf("trainingPlanDateRanges explicit = %+v %v, want %+v nil", got, err, want)
	}
}
