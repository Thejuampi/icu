package icu_test

import (
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

const microcycleTestConfidenceHigh = "high"

func TestAnalyzeMicrocycleHappyPath(t *testing.T) {
	t.Parallel()

	settings := icu.SportSettings{
		FTP:        285,
		PowerZones: []int{55, 75, 90, 105, 120},
		HRZones:    []int{120, 140, 155, 170},
	}
	wellness := icu.AnalyzeWellness([]icu.Wellness{
		{ID: "2026-06-08", HRV: 42, RestingHR: 50, SleepScore: 82},
		{ID: "2026-06-09", HRV: 44, RestingHR: 49, SleepScore: 85},
		{ID: "2026-06-10", HRV: 43, RestingHR: 51, SleepScore: 80},
	}, icu.AnalysisOptions{StartDate: "2026-06-08", EndDate: "2026-06-14"})

	got := icu.AnalyzeMicrocycle(
		[]icu.Activity{
			microcycleActivity("i1", "2026-06-08T07:00:00", 60, 0.72),
			microcycleActivity("i2", "2026-06-10T07:00:00", 95, 0.91),
		},
		[]icu.Event{
			microcycleWorkout("2026-06-08T07:00:00", 60),
			microcycleWorkout("2026-06-10T07:00:00", 95),
		},
		&wellness,
		&settings,
		&icu.MicrocycleOptions{
			StartDate:        "2026-06-08",
			EndDate:          "2026-06-14",
			Timezone:         "America/New_York",
			TimezoneSource:   "flag",
			Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
			PlanIncluded:     true,
			WellnessIncluded: true,
			SportType:        "Ride",
		},
	)

	if got.Status != "success" || got.PlannedVsActual.Available != true || got.Load.TSS != 155 {
		t.Fatalf("AnalyzeMicrocycle happy path = status %s planned %v tss %d", got.Status, got.PlannedVsActual.Available, got.Load.TSS)
	}
}

// TestMicrocycleAnalysisSchemaVersionDidNotDrift pins the literal wire
// string (D-7), not the constant against itself: a consumer parsing
// analysis output needs a version marker to detect a future meaning
// change. Comparing icu.MicrocycleAnalysisSchemaVersion against itself
// would pass regardless of what the constant holds.
func TestMicrocycleAnalysisSchemaVersionDidNotDrift(t *testing.T) {
	t.Parallel()

	if icu.MicrocycleAnalysisSchemaVersion != "icu.analysis.microcycle.v1" {
		t.Fatalf("MicrocycleAnalysisSchemaVersion = %q, want icu.analysis.microcycle.v1", icu.MicrocycleAnalysisSchemaVersion)
	}
}

func TestAnalyzeMicrocycleEmitsSchemaVersion(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeMicrocycle(nil, nil, nil, nil, nil)

	if got.SchemaVersion != "icu.analysis.microcycle.v1" {
		t.Fatalf("SchemaVersion = %q, want icu.analysis.microcycle.v1", got.SchemaVersion)
	}
}

func TestAnalyzeMicrocyclePartialWeek(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeMicrocycle(nil, nil, nil, nil, &icu.MicrocycleOptions{
		StartDate:        "2026-06-08",
		EndDate:          "2026-06-14",
		Timezone:         "UTC",
		TimezoneSource:   "system",
		Now:              time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC),
		PlanIncluded:     false,
		WellnessIncluded: false,
		SportType:        "Ride",
	})

	if !got.Microcycle.IsPartial || got.Microcycle.ElapsedDays != 3 {
		t.Fatalf("partial = %v elapsed = %d, want true 3", got.Microcycle.IsPartial, got.Microcycle.ElapsedDays)
	}
}

func TestAnalyzeMicrocycleNoActivitiesIsDataLimited(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeMicrocycle(nil, nil, nil, nil, &icu.MicrocycleOptions{
		StartDate:        "2026-06-08",
		EndDate:          "2026-06-14",
		Timezone:         "UTC",
		TimezoneSource:   "system",
		Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
		PlanIncluded:     false,
		WellnessIncluded: false,
		SportType:        "Ride",
	})

	if got.Classification.Value != "data_limited" {
		t.Fatalf("classification = %s, want data_limited", got.Classification.Value)
	}
}

func TestAnalyzeMicrocycleMissingPlanAndWellnessReduceConfidence(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeMicrocycle(
		[]icu.Activity{microcycleActivity("i1", "2026-06-08T07:00:00", 50, 0.7)},
		nil,
		nil,
		&icu.SportSettings{FTP: 285, PowerZones: []int{55, 75, 90, 105}},
		&icu.MicrocycleOptions{
			StartDate:        "2026-06-08",
			EndDate:          "2026-06-14",
			Timezone:         "UTC",
			TimezoneSource:   "system",
			Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
			PlanIncluded:     false,
			WellnessIncluded: false,
			SportType:        "Ride",
		},
	)

	if got.Confidence != "low" {
		t.Fatalf("confidence = %s, want low", got.Confidence)
	}
}

func TestAnalyzeMicrocycleMissingZonesWarns(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeMicrocycle(
		[]icu.Activity{microcycleActivity("i1", "2026-06-08T07:00:00", 50, 0.7)},
		nil,
		nil,
		&icu.SportSettings{FTP: 285},
		&icu.MicrocycleOptions{
			StartDate:        "2026-06-08",
			EndDate:          "2026-06-14",
			Timezone:         "UTC",
			TimezoneSource:   "system",
			Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
			PlanIncluded:     false,
			WellnessIncluded: false,
			SportType:        "Ride",
		},
	)

	if got.DataQuality.Zones != "missing" {
		t.Fatalf("zones = %s, want missing", got.DataQuality.Zones)
	}
}

func TestAnalyzeMicrocycleExcessZ4PlusRisk(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeMicrocycle(
		[]icu.Activity{
			microcycleActivity("i1", "2026-06-08T07:00:00", 90, 0.9),
			microcycleActivity("i2", "2026-06-10T07:00:00", 90, 0.91),
			microcycleActivity("i3", "2026-06-12T07:00:00", 90, 0.92),
		},
		nil,
		nil,
		&icu.SportSettings{FTP: 285, PowerZones: []int{55, 75, 90, 105}},
		&icu.MicrocycleOptions{
			StartDate:        "2026-06-08",
			EndDate:          "2026-06-14",
			Timezone:         "UTC",
			TimezoneSource:   "system",
			Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
			PlanIncluded:     false,
			WellnessIncluded: false,
			SportType:        "Ride",
		},
	)

	if len(got.Risks) == 0 || got.Risks[0].Type != "intensity_density" {
		t.Fatalf("risks = %+v, want intensity_density", got.Risks)
	}
}

func TestAnalyzeMicrocycleLoadComparisons(t *testing.T) {
	t.Parallel()

	activities := []icu.Activity{
		microcycleActivity("old1", "2026-06-03T07:00:00", 20, 0.7),
		microcycleActivity("old2", "2026-06-05T07:00:00", 20, 0.7),
		microcycleActivity("new1", "2026-06-08T07:00:00", 100, 0.7),
	}

	got := icu.AnalyzeMicrocycle(activities, nil, nil, &icu.SportSettings{FTP: 285, PowerZones: []int{55}}, &icu.MicrocycleOptions{
		StartDate:        "2026-06-08",
		EndDate:          "2026-06-14",
		Timezone:         "UTC",
		TimezoneSource:   "flag",
		Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
		PlanIncluded:     false,
		WellnessIncluded: false,
		SportType:        "Ride",
	})

	if got.Load.Comparison.Previous7Days != "above" {
		t.Fatalf("previous7Days = %s, want above", got.Load.Comparison.Previous7Days)
	}
}

func TestAnalyzeMicrocycleUnderloadedWithZeroLoadActivity(t *testing.T) {
	t.Parallel()

	activity := microcycleActivity("i1", "2026-06-08T07:00:00", 0, 0.5)
	got := icu.AnalyzeMicrocycle([]icu.Activity{activity}, nil, nil, &icu.SportSettings{FTP: 285, PowerZones: []int{55}}, &icu.MicrocycleOptions{
		StartDate:        "2026-06-08",
		EndDate:          "2026-06-14",
		Timezone:         "UTC",
		TimezoneSource:   "flag",
		Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
		PlanIncluded:     false,
		WellnessIncluded: false,
		SportType:        "Ride",
	})

	if got.Classification.Value != "data_limited" {
		t.Fatalf("classification = %s, want data_limited from low confidence", got.Classification.Value)
	}
}

func TestAnalyzeMicrocycleLoadPressureRisk(t *testing.T) {
	t.Parallel()

	activity := microcycleActivity("i1", "2026-06-08T07:00:00", 80, 0.7)
	activity.CTL = 50
	activity.ATL = 70
	// Today's form (-20) is judged against the athlete's own TSB baseline
	// (-4..-6), not a fixed TSB.
	tsb := append(repeatPattern([]float64{-4, -5, -6}, 20), -20)
	records := make([]icu.Wellness, 0, len(tsb))
	for index, value := range tsb {
		day := time.Date(2026, time.May, 20, 0, 0, 0, 0, time.UTC).AddDate(0, 0, index)
		records = append(records, icu.Wellness{ID: day.Format("2006-01-02"), CTL: 50, ATL: 50 - value})
	}
	wellness := icu.AnalyzeWellness(records, icu.AnalysisOptions{})

	got := icu.AnalyzeMicrocycle([]icu.Activity{activity}, nil, &wellness, &icu.SportSettings{FTP: 285, PowerZones: []int{55}}, &icu.MicrocycleOptions{
		StartDate:        "2026-06-08",
		EndDate:          "2026-06-14",
		Timezone:         "UTC",
		TimezoneSource:   "flag",
		Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
		PlanIncluded:     false,
		WellnessIncluded: false,
		SportType:        "Ride",
	})

	if got.FatigueDurability.State != "load_pressure" {
		t.Fatalf("fatigue state = %s, want load_pressure", got.FatigueDurability.State)
	}
}

func TestAnalyzeMicrocycleFutureWeekHasNoElapsedDays(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeMicrocycle(nil, nil, nil, nil, &icu.MicrocycleOptions{
		StartDate:        "2026-06-15",
		EndDate:          "2026-06-21",
		Timezone:         "UTC",
		TimezoneSource:   "flag",
		Now:              time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC),
		PlanIncluded:     false,
		WellnessIncluded: false,
		SportType:        "Ride",
	})

	if got.Microcycle.ElapsedDays != 0 || got.Microcycle.RemainingDays != 7 {
		t.Fatalf("elapsed/remaining = %d/%d, want 0/7", got.Microcycle.ElapsedDays, got.Microcycle.RemainingDays)
	}
}

func TestAnalyzeMicrocycleHighConfidenceOnTrack(t *testing.T) {
	t.Parallel()

	records := make([]icu.Wellness, 0, 7)
	for day := 8; day <= 14; day++ {
		records = append(records, icu.Wellness{
			ID:         time.Date(2026, time.June, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
			HRV:        50,
			RestingHR:  48,
			SleepScore: 85,
		})
	}
	wellness := icu.AnalyzeWellness(records, icu.AnalysisOptions{StartDate: "2026-06-08", EndDate: "2026-06-14"})
	settings := icu.SportSettings{FTP: 285, PowerZones: []int{55, 75, 90, 105}, HRZones: []int{120, 140, 155, 170}}

	got := icu.AnalyzeMicrocycle(
		[]icu.Activity{microcycleActivity("i1", "2026-06-08T07:00:00", 60, 0.7)},
		[]icu.Event{microcycleWorkout("2026-06-08T07:00:00", 60)},
		&wellness,
		&settings,
		&icu.MicrocycleOptions{
			StartDate:        "2026-06-08",
			EndDate:          "2026-06-14",
			Timezone:         "UTC",
			TimezoneSource:   "flag",
			Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
			PlanIncluded:     true,
			WellnessIncluded: true,
			SportType:        "Ride",
		},
	)

	if got.Confidence != microcycleTestConfidenceHigh || got.Classification.Value != "on_track" {
		t.Fatalf("confidence/classification = %s/%s, want high/on_track", got.Confidence, got.Classification.Value)
	}
}

func TestAnalyzeMicrocyclePrefersNamedRecoveryScoreOverSleepScore(t *testing.T) {
	t.Parallel()

	wellness := icu.AnalyzeWellness([]icu.Wellness{
		{
			ID:         "2026-06-08",
			HRV:        42,
			RestingHR:  50,
			SleepScore: 60,
			PreferredScore: icu.NamedWellnessScore{
				Name:  "zepp_hybridcharge",
				Value: 90,
			},
		},
		{
			ID:         "2026-06-09",
			HRV:        44,
			RestingHR:  49,
			SleepScore: 55,
			PreferredScore: icu.NamedWellnessScore{
				Name:  "zepp_hybridcharge",
				Value: 88,
			},
		},
	}, icu.AnalysisOptions{
		Thresholds: pinnedThresholds(), StartDate: "2026-06-08", EndDate: "2026-06-14",
	})

	got := icu.AnalyzeMicrocycle(
		[]icu.Activity{microcycleActivity("i1", "2026-06-08T07:00:00", 60, 0.7)},
		[]icu.Event{microcycleWorkout("2026-06-08T07:00:00", 60)},
		&wellness,
		&icu.SportSettings{FTP: 285, PowerZones: []int{55, 75, 90, 105}},
		&icu.MicrocycleOptions{
			StartDate:        "2026-06-08",
			EndDate:          "2026-06-14",
			Timezone:         "UTC",
			TimezoneSource:   "system",
			Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
			PlanIncluded:     true,
			WellnessIncluded: true,
			SportType:        "Ride",
		},
	)

	if got.Wellness.State.State != "OK" || !microcycleContainsString(got.Wellness.PositiveSignals, "sleep_score_ok") {
		t.Fatalf("wellness = %+v, want OK state with positive sleep signal from preferred score", got.Wellness)
	}
}

func TestAnalyzeMicrocyclePartialHeartRateAvailability(t *testing.T) {
	t.Parallel()

	first := microcycleActivity("i1", "2026-06-08T07:00:00", 60, 0.7)
	second := microcycleActivity("i2", "2026-06-09T07:00:00", 60, 0.7)
	second.AverageHeartRate = 0
	second.MaxHeartRate = 0
	second.HRZoneTimes = nil

	got := icu.AnalyzeMicrocycle([]icu.Activity{first, second}, nil, nil, &icu.SportSettings{FTP: 285, PowerZones: []int{55}}, &icu.MicrocycleOptions{
		StartDate:        "2026-06-08",
		EndDate:          "2026-06-14",
		Timezone:         "UTC",
		TimezoneSource:   "flag",
		Now:              time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
		PlanIncluded:     false,
		WellnessIncluded: false,
		SportType:        "Ride",
	})

	if got.DataQuality.HeartRate != "partial" {
		t.Fatalf("heart rate quality = %s, want partial", got.DataQuality.HeartRate)
	}
}

func TestAnalyzeMicrocycleNilOptions(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeMicrocycle(nil, nil, nil, nil, nil)

	if got.Command != "analysis microcycle" {
		t.Fatalf("command = %s, want analysis microcycle", got.Command)
	}
}

// TestAnalyzeMicrocycleExcludesStubActivitiesFromCompletedSessions is
// F-19/T2.3: CompletedSessions used to be a bare len(activities), so a
// D-24 STRAVA stub landing in the window would inflate it as a fake
// completed session; ExcludeStubActivities must strip it before any
// activity-count denominator in this payload sees it.
func TestAnalyzeMicrocycleExcludesStubActivitiesFromCompletedSessions(t *testing.T) {
	t.Parallel()

	settings := icu.SportSettings{FTP: 285}
	activities := []icu.Activity{
		microcycleActivity("i1", "2026-06-08T07:00:00", 60, 0.72),
		{ID: "stub1", StartDateLocal: "2026-06-09T00:00:00", StubShape: true},
	}

	got := icu.AnalyzeMicrocycle(activities, nil, nil, &settings, &icu.MicrocycleOptions{
		StartDate: "2026-06-08", EndDate: "2026-06-14",
		Now: time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
	})

	if got.PlannedVsActual.CompletedSessions != 1 {
		t.Fatalf("CompletedSessions = %d, want 1 (the stub must not count as a completed session)", got.PlannedVsActual.CompletedSessions)
	}
}

func microcycleActivity(id, date string, load int, intensity float64) icu.Activity {
	return icu.Activity{
		ID:               id,
		Name:             "Ride " + id,
		Type:             "Ride",
		StartDateLocal:   date,
		MovingTime:       3600,
		Distance:         25000,
		TrainingLoad:     load,
		Intensity:        intensity,
		AverageHeartRate: 145,
		WeightedAvgPower: 210,
		ZoneTimes: []icu.ZoneTime{
			{ID: "Z1", Secs: 600},
			{ID: "Z2", Secs: 1800},
			{ID: "Z4", Secs: 1200},
		},
		HRZoneTimes: []int{600, 1800, 1200},
		FTP:         285,
	}
}

func microcycleWorkout(date string, load int) icu.Event {
	return icu.Event{
		StartDateLocal: date,
		Category:       "WORKOUT",
		Type:           "Ride",
		Name:           "Planned Ride",
		MovingTime:     3600,
		TrainingLoad:   load,
	}
}

func microcycleContainsString(values []string, want string) bool {
	for index := range values {
		if values[index] == want {
			return true
		}
	}

	return false
}
