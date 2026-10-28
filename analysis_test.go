package icu_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

const testAnalysisSecondRideDateTime = "2026-05-30T08:00:00"

func TestAnalyzeCyclingActivitiesFiltersCycling(t *testing.T) {
	t.Parallel()

	var ride icu.Activity
	ride.Type = testRideType
	ride.TrainingLoad = 50

	var run icu.Activity
	run.Type = "Run"
	run.TrainingLoad = 80

	activities := []icu.Activity{ride, run}

	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{StartDate: "", EndDate: ""})

	if got.Scope.CyclingActivities != 1 {
		t.Fatalf("CyclingActivities = %d, want 1", got.Scope.CyclingActivities)
	}
}

func TestAnalyzeCyclingActivitiesVolume(t *testing.T) {
	t.Parallel()

	var ride icu.Activity
	ride.Type = testRideType
	ride.MovingTime = 3600
	ride.Distance = 25000
	ride.TotalElevationGain = 200

	var virtualRide icu.Activity
	virtualRide.Type = "VirtualRide"
	virtualRide.MovingTime = 5400
	virtualRide.Distance = 40000
	virtualRide.TotalElevationGain = 300

	activities := []icu.Activity{ride, virtualRide}

	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{StartDate: "", EndDate: ""})
	want := icu.CyclingVolume{
		MovingTimeSecs:      9000,
		MovingTimeHours:     2.5,
		DistanceMeters:      65000,
		DistanceKilometers:  65,
		ElevationGainMeters: 500,
	}

	if !reflect.DeepEqual(got.Volume, want) {
		t.Fatalf("Volume = %+v, want %+v", got.Volume, want)
	}
}

func TestAnalyzeCyclingActivitiesDailyLoadUsesRange(t *testing.T) {
	t.Parallel()

	var firstRide icu.Activity
	firstRide.Type = testRideType
	firstRide.StartDateLocal = "2026-05-01T08:00:00"
	firstRide.TrainingLoad = 70

	var secondRide icu.Activity
	secondRide.Type = testRideType
	secondRide.StartDateLocal = "2026-05-03T08:00:00"
	secondRide.TrainingLoad = 30

	activities := []icu.Activity{firstRide, secondRide}

	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-03",
	})
	want := []icu.DailyTrainingLoad{
		{Date: "2026-05-01", Load: 70},
		{Date: "2026-05-02", Load: 0},
		{Date: "2026-05-03", Load: 30},
	}

	if !reflect.DeepEqual(got.Load.Daily, want) {
		t.Fatalf("Daily = %+v, want %+v", got.Load.Daily, want)
	}
}

func TestAnalyzeCyclingActivitiesZoneTotals(t *testing.T) {
	t.Parallel()

	var firstRide icu.Activity
	firstRide.Type = testRideType
	firstRide.ZoneTimes = []icu.ZoneTime{{ID: "Z2", Secs: 3000}, {ID: "Z5", Secs: 600}}

	var secondRide icu.Activity
	secondRide.Type = testRideType
	secondRide.ZoneTimes = []icu.ZoneTime{{ID: "Z2", Secs: 1800}}

	activities := []icu.Activity{firstRide, secondRide}

	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{StartDate: "", EndDate: ""})
	want := map[string]int{"Z2": 4800, "Z5": 600}

	if !reflect.DeepEqual(got.Intensity.ZoneSeconds, want) {
		t.Fatalf("ZoneSeconds = %+v, want %+v", got.Intensity.ZoneSeconds, want)
	}
}

func TestAnalyzeCyclingActivitiesSessionSummary(t *testing.T) {
	t.Parallel()

	var ride icu.Activity
	ride.ID = "i152759915"
	ride.Name = "Indoor-Friendly Adventure"
	ride.Type = testRideType
	ride.StartDateLocal = testAnalysisSecondRideDateTime
	ride.MovingTime = 8370
	ride.Distance = 59500
	ride.TrainingLoad = 124
	ride.Intensity = 0.73
	ride.WeightedAvgPower = 208
	ride.AverageHeartRate = 145
	ride.Decoupling = 3.1
	ride.EfficiencyFactor = 1.44
	ride.VariabilityIndex = 1.29
	ride.JoulesAboveFTP = 12000
	ride.MaxWbalDepletion = 6500
	ride.MaxHeartRate = 185
	ride.ZoneTimes = []icu.ZoneTime{{ID: "Z1", Secs: 2000}, {ID: "Z2", Secs: 1500}}
	ride.HRZoneTimes = []int{1000, 1500, 800, 600, 0, 0, 0}

	got := icu.AnalyzeCyclingActivities([]icu.Activity{ride}, icu.AnalysisOptions{StartDate: "", EndDate: ""})
	want := []icu.CyclingSession{
		{
			ID:                 "i152759915",
			Date:               "2026-05-30",
			Name:               "Indoor-Friendly Adventure",
			Type:               testRideType,
			MovingTimeSecs:     8370,
			MovingTimeMinutes:  139.5,
			DistanceMeters:     59500,
			DistanceKilometers: 59.5,
			TrainingLoad:       124,
			Intensity:          0.73,
			WeightedAvgPower:   208,
			AverageHeartRate:   145,
			Decoupling:         new(3.1),
			EfficiencyFactor:   1.44,
			VariabilityIndex:   1.29,
			JoulesAboveFTP:     12000,
			MaxWBalDepletion:   6500,
			WPrime:             0,
			WBalDepletionPct:   0,
			CriticalPower:      0,
			PMax:               0,
			FTP:                0,
			RollingFTP:         0,
			AverageTemp:        0,
			AverageFeelsLike:   0,
			HeadwindPercent:    0,
			StrainScore:        0,
			ZoneTimes: []icu.ZoneTime{
				{ID: "Z1", Secs: 2000},
				{ID: "Z2", Secs: 1500},
			},
			HRZoneTimes:  []int{1000, 1500, 800, 600, 0, 0, 0},
			MaxHeartRate: 185,
		},
	}

	if !reflect.DeepEqual(got.Sessions, want) {
		t.Fatalf("Sessions = %+v, want %+v", got.Sessions, want)
	}
}

func TestAnalyzeCyclingActivitiesEnvironmentalContext(t *testing.T) {
	t.Parallel()

	var hotRide icu.Activity
	hotRide.Type = testRideType
	hotRide.StartDateLocal = "2026-05-29T08:00:00"
	hotRide.MovingTime = 5400
	hotRide.AverageTemp = 31
	hotRide.MaxTemp = 36
	hotRide.AverageWeatherTemp = 30
	hotRide.AverageFeelsLike = 34
	hotRide.AverageWindSpeed = 18
	hotRide.AverageWindGust = 34
	hotRide.HeadwindPercent = 44
	hotRide.TailwindPercent = 20
	hotRide.AverageAltitude = 520
	hotRide.MaxAltitude = 780
	hotRide.AverageGradient = 3.8
	hotRide.AverageYaw = 8
	hotRide.StrainScore = 72

	var mildRide icu.Activity
	mildRide.Type = testRideType
	mildRide.StartDateLocal = testAnalysisSecondRideDateTime
	mildRide.MovingTime = 3600
	mildRide.AverageTemp = 24
	mildRide.AverageWeatherTemp = 23
	mildRide.AverageFeelsLike = 25
	mildRide.AverageWindSpeed = 8
	mildRide.HeadwindPercent = 12
	mildRide.AverageAltitude = 300
	mildRide.AverageGradient = 1.5
	mildRide.StrainScore = 40

	got := icu.AnalyzeCyclingActivities([]icu.Activity{hotRide, mildRide}, icu.AnalysisOptions{
		StartDate: "",
		EndDate:   "",
	})
	want := icu.CyclingEnvironmentContext{
		Samples:              2,
		AverageTemp:          27.5,
		MaxTemp:              36,
		AverageWeatherTemp:   26.5,
		AverageFeelsLike:     29.5,
		AverageWindSpeed:     13,
		MaxWindGust:          34,
		HeadwindPercent:      28,
		TailwindPercent:      20,
		AverageAltitude:      410,
		MaxAltitude:          780,
		AverageGradient:      2.65,
		AverageYaw:           8,
		AverageStrainScore:   56,
		HotSessions:          1,
		WindAffectedSessions: 1,
		ClimbingSessions:     1,
		Source:               "activity_environment_fields",
	}

	if !reflect.DeepEqual(got.Environment, want) {
		t.Fatalf("Environment = %+v, want %+v", got.Environment, want)
	}
}

func TestAnalyzeCyclingActivitiesWPrimeAndEfficiencySignals(t *testing.T) {
	t.Parallel()

	var firstRide icu.Activity
	firstRide.Type = testRideType
	firstRide.MovingTime = 3600
	firstRide.FTP = 285
	firstRide.CriticalPower = 292
	firstRide.WPrime = 22000
	firstRide.PMax = 1100
	firstRide.RollingFTP = 288
	firstRide.JoulesAboveFTP = 11000
	firstRide.MaxWbalDepletion = 8800
	firstRide.EfficiencyFactor = 1.46
	firstRide.VariabilityIndex = 1.1

	var secondRide icu.Activity
	secondRide.Type = testRideType
	secondRide.MovingTime = 5400
	secondRide.FTP = 290
	secondRide.CriticalPower = 296
	secondRide.WPrime = 24000
	secondRide.PMax = 1120
	secondRide.RollingFTP = 291
	secondRide.JoulesAboveFTP = 9000
	secondRide.MaxWbalDepletion = 12000
	secondRide.EfficiencyFactor = 1.5
	secondRide.VariabilityIndex = 1.16

	got := icu.AnalyzeCyclingActivities([]icu.Activity{firstRide, secondRide}, icu.AnalysisOptions{
		StartDate: "",
		EndDate:   "",
	})
	want := icu.CyclingPowerAnchors{
		FTP:           290,
		CriticalPower: 296,
		WPrime:        24000,
		PMax:          1120,
		RollingFTP:    291,
		Source:        "latest_activity_model_fields",
	}

	if got.PowerAnchors != want {
		t.Fatalf("PowerAnchors = %+v, want %+v", got.PowerAnchors, want)
	}

	if got.Performance.Repeatability.Pattern != "repeatable_w_prime_depletion" ||
		got.Performance.Repeatability.MaxWBalDepletionPercent != 50 ||
		got.Performance.Efficiency.State != "efficient_stable" {
		t.Fatalf("Performance = %+v, want W prime pattern and efficient_stable", got.Performance)
	}
}

func TestAnalyzeCyclingActivitiesStateAndPerformance(t *testing.T) {
	t.Parallel()

	var firstRide icu.Activity
	firstRide.Type = testRideType
	firstRide.StartDateLocal = "2026-05-23T08:00:00"
	firstRide.MovingTime = 8400
	firstRide.TrainingLoad = 120
	firstRide.Intensity = 0.7
	firstRide.Decoupling = 3
	firstRide.EfficiencyFactor = 1.4
	firstRide.VariabilityIndex = 1.2
	firstRide.JoulesAboveFTP = 10000
	firstRide.MaxWbalDepletion = 5000
	firstRide.CTL = 53
	firstRide.ATL = 60

	var secondRide icu.Activity
	secondRide.Type = testRideType
	secondRide.StartDateLocal = testAnalysisSecondRideDateTime
	secondRide.MovingTime = 3600
	secondRide.TrainingLoad = 80
	secondRide.Intensity = 0.9
	secondRide.Decoupling = 8
	secondRide.EfficiencyFactor = 1.5
	secondRide.VariabilityIndex = 1.4
	secondRide.JoulesAboveFTP = 20000
	secondRide.MaxWbalDepletion = 7000
	secondRide.CTL = 53.7
	secondRide.ATL = 67.19

	got := icu.AnalyzeCyclingActivities(
		[]icu.Activity{firstRide, secondRide},
		icu.AnalysisOptions{StartDate: "2026-05-23", EndDate: "2026-05-30"},
	)
	want := icu.CyclingPerformanceIntelligence{
		Repeatability: icu.CyclingRepeatability{
			TotalWorkAboveFTP:        30000,
			MaxWBalDepletion:         7000,
			MeanMaxWBalDepletion:     6000,
			MaxWBalDepletionPercent:  0,
			MeanWBalDepletionPercent: 0,
			WPrimeCapacity:           0,
			WorkAboveFTPPerHour:      9000,
			SessionsWithWBalData:     2,
			Pattern:                  "",
			Classification:           "informational",
			ClassificationContext:    "local_activity_fields",
		},
		Durability: icu.CyclingDurabilitySignal{
			State:                 "watch",
			MeanDecoupling:        5.5,
			MaxDecoupling:         8,
			HighDriftSessions:     1,
			LongEnduranceSessions: 1,
			ISDMState:             "durability_limiter",
			ISDMScore:             50,
			Classification:        "local_heuristic",
		},
		NeuralDensity: icu.CyclingNeuralDensity{
			RollingWorkAboveFTP:  30000,
			HighIntensityDays:    1,
			MeanIntensity:        0.8,
			MeanEfficiencyFactor: 1.45,
			MeanVariabilityIndex: 1.3,
			Classification:       "local_heuristic",
		},
		Efficiency: icu.CyclingEfficiencySignal{
			State:                  "efficient_variable",
			MeanEfficiencyFactor:   1.45,
			MeanVariabilityIndex:   1.3,
			SessionsWithEfficiency: 2,
			Classification:         "local_efficiency_heuristic",
		},
	}

	if !reflect.DeepEqual(got.Performance, want) {
		t.Fatalf("Performance = %+v, want %+v", got.Performance, want)
	}

	if got.State.OperationalState != "recovery_priority" || got.State.LoadPressure != 13.49 {
		t.Fatalf("State = %+v, want recovery_priority with load pressure 13.49", got.State)
	}
}

func TestAnalyzeCyclingActivitiesIncludesTimezoneMetadata(t *testing.T) {
	t.Parallel()

	var ride icu.Activity
	ride.Type = testRideType
	ride.TrainingLoad = 50

	got := icu.AnalyzeCyclingActivities([]icu.Activity{ride}, icu.AnalysisOptions{
		StartDate:      "2026-06-01",
		EndDate:        "2026-06-07",
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

// TestCyclingAnalysisSchemaVersionDidNotDrift pins the literal wire string
// (D-7), not the constant against itself: a consumer parsing analysis
// output needs a version marker to detect a future meaning change.
// Comparing icu.CyclingAnalysisSchemaVersion against itself would pass
// regardless of what the constant holds.
func TestCyclingAnalysisSchemaVersionDidNotDrift(t *testing.T) {
	t.Parallel()

	if icu.CyclingAnalysisSchemaVersion != "icu.analysis.cycling.v1" {
		t.Fatalf("CyclingAnalysisSchemaVersion = %q, want icu.analysis.cycling.v1", icu.CyclingAnalysisSchemaVersion)
	}
}

func TestAnalyzeCyclingActivitiesEmitsSchemaVersion(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeCyclingActivities(nil, icu.AnalysisOptions{})

	if got.SchemaVersion != "icu.analysis.cycling.v1" {
		t.Fatalf("SchemaVersion = %q, want icu.analysis.cycling.v1", got.SchemaVersion)
	}
}

// TestAnalyzeCyclingActivitiesExcludesStubsFromScopeActivities is W2-P01:
// stubs never occupy a count anywhere (F-19's live defect), so
// Scope.Activities must equal the real-activity count, not the raw fetch
// length, and Validity.Unavailable.StravaStubs must report the excluded
// count separately.
func TestAnalyzeCyclingActivitiesExcludesStubsFromScopeActivities(t *testing.T) {
	t.Parallel()

	activities := make([]icu.Activity, 0, 61)
	for range 41 {
		activities = append(activities, icu.Activity{ID: "real", Type: "Walk"})
	}

	for range 20 {
		activities = append(activities, icu.Activity{ID: "stub", StubShape: true})
	}

	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{})

	if got.Scope.Activities != 41 || got.Validity.Unavailable.StravaStubs != 20 {
		t.Fatalf("Scope.Activities/Unavailable.StravaStubs = %d/%d, want 41/20",
			got.Scope.Activities, got.Validity.Unavailable.StravaStubs)
	}
}

// TestAnalyzeCyclingActivitiesFTPDiscriminatorConflictsAbsentFromWire is R1:
// ftpDiscriminatorConflicts must be top-level only (never a per-session key,
// so the wave-2 golden's Sessions rows stay untouched) and omitted, not a
// stray zero, when there is nothing to report. Activity.Trainer is not yet
// an exposed Go field, so even an ambiguous cycling type with no
// discriminating flag can never populate this counter today — this test
// pins that absence at the JSON level and will fail loudly the day Trainer
// is wired and this needs a real positive-case test alongside it.
func TestAnalyzeCyclingActivitiesFTPDiscriminatorConflictsAbsentFromWire(t *testing.T) {
	t.Parallel()

	ambiguous := icu.Activity{Type: "GravelRide", TrainingLoad: 50, MovingTime: 3600}

	got := icu.AnalyzeCyclingActivities([]icu.Activity{ambiguous}, icu.AnalysisOptions{})

	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal analysis: %v", err)
	}

	if got.FTPDiscriminatorConflicts != nil || strings.Contains(string(data), "ftpDiscriminatorConflicts") {
		t.Fatalf("FTPDiscriminatorConflicts = %+v, JSON contains key = %v, want nil/absent",
			got.FTPDiscriminatorConflicts, strings.Contains(string(data), "ftpDiscriminatorConflicts"))
	}
}

// TestAnalyzeCyclingActivitiesIgnorePowerExcludesLoadRetainsVolume is
// W2-P02: icu_ignore_power drops an activity from load (power-derived) but
// retains it in volume, and its validity scope is reported as "power".
func TestAnalyzeCyclingActivitiesIgnorePowerExcludesLoadRetainsVolume(t *testing.T) {
	t.Parallel()

	included := icu.Activity{Type: testRideType, TrainingLoad: 50, MovingTime: 3600}
	excluded := icu.Activity{Type: testRideType, TrainingLoad: 80, MovingTime: 3600, IgnorePower: true}

	got := icu.AnalyzeCyclingActivities([]icu.Activity{included, excluded}, icu.AnalysisOptions{})

	if got.Load.Total != 50 || got.Volume.MovingTimeSecs != 7200 || got.Validity.ExcludedByScope[icu.ValidityScopePower] != 1 {
		t.Fatalf("Load.Total/Volume.MovingTimeSecs/excludedByScope[power] = %d/%d/%d, want 50/7200/1",
			got.Load.Total, got.Volume.MovingTimeSecs, got.Validity.ExcludedByScope[icu.ValidityScopePower])
	}
}

// TestAnalyzeCyclingActivitiesIgnoreTimeExcludesVolumeToo is W2-P03:
// icu_ignore_time is the one flag that also excludes volume, since duration
// is the denominator every other aggregate divides by (D-2's "all" scope).
func TestAnalyzeCyclingActivitiesIgnoreTimeExcludesVolumeToo(t *testing.T) {
	t.Parallel()

	included := icu.Activity{Type: testRideType, TrainingLoad: 50, MovingTime: 3600}
	excluded := icu.Activity{Type: testRideType, TrainingLoad: 80, MovingTime: 3600, IgnoreTime: true}

	got := icu.AnalyzeCyclingActivities([]icu.Activity{included, excluded}, icu.AnalysisOptions{})

	if got.Volume.MovingTimeSecs != 3600 || got.Validity.ExcludedByScope[icu.ValidityScopeAll] != 1 {
		t.Fatalf("Volume.MovingTimeSecs/excludedByScope[all] = %d/%d, want 3600/1",
			got.Volume.MovingTimeSecs, got.Validity.ExcludedByScope[icu.ValidityScopeAll])
	}
}

// TestAnalyzeCyclingActivitiesRefusesLoadSectionWhenAllFlagged is W2-E01:
// when every cycling activity in scope is flag-excluded, the load section
// (0 included) must carry a named warning rather than silently presenting
// its zero-valued fields as a measured result.
func TestAnalyzeCyclingActivitiesRefusesLoadSectionWhenAllFlagged(t *testing.T) {
	t.Parallel()

	activities := []icu.Activity{
		{Type: testRideType, TrainingLoad: 50, MovingTime: 3600, IgnorePower: true},
		{Type: testRideType, TrainingLoad: 80, MovingTime: 3600, IgnorePower: true},
	}

	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{})

	wantWarning := "load section refused: 0 included activities (all excluded by icu_ignore_power/icu_ignore_time)"
	found := false

	for _, warning := range got.Warnings {
		if warning == wantWarning {
			found = true
		}
	}

	if !found {
		t.Fatalf("Warnings = %v, want to contain %q", got.Warnings, wantWarning)
	}
}

// TestAnalyzeCyclingActivitiesWarnsBelowMinIncludedActivities is D-4's
// explicit user constraint: --min-included-activities has no default and
// never blocks output by itself, but when Included falls below the
// caller-supplied threshold it must add a named warning identifying both
// counts and the flag that set the threshold.
func TestAnalyzeCyclingActivitiesWarnsBelowMinIncludedActivities(t *testing.T) {
	t.Parallel()

	activities := []icu.Activity{
		{Type: testRideType, TrainingLoad: 50, MovingTime: 3600},
		{Type: testRideType, TrainingLoad: 80, MovingTime: 3600},
	}

	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{MinIncludedActivities: 5})

	wantWarning := "included activity count 2 is below the configured minimum 5 (--min-included-activities)"
	found := false

	for _, warning := range got.Warnings {
		if warning == wantWarning {
			found = true
		}
	}

	if !found {
		t.Fatalf("Warnings = %v, want to contain %q", got.Warnings, wantWarning)
	}
}

func TestApplyWellnessFitness(t *testing.T) {
	t.Parallel()

	records := []icu.Wellness{
		{ID: "2026-10-24", CTL: 29.2, ATL: 44.0},
		{ID: "2026-10-26", CTL: 28.88, ATL: 39.03},
		{ID: "2026-11-01", CTL: 25.03, ATL: 16.56},
		{ID: "2026-10-25"},
	}
	cases := []struct {
		name     string
		records  []icu.Wellness
		end      string
		wantTSB  float64
		wantDate string
	}{
		{"uses end-date row, ignores projected future row", records, "2026-10-26", -10.15, "2026-10-26"},
		{"skips rows without fitness fields", records, "2026-10-25", -14.8, "2026-10-24"},
		{"leaves activity values when no rows", nil, "2026-10-26", -15.45, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			load := icu.CyclingLoad{LatestCTL: 29.57, LatestATL: 45.02, LatestForm: -15.45}
			icu.ApplyWellnessFitness(&load, tc.records, tc.end)
			if load.LatestForm != tc.wantTSB || load.LatestLoadDate != tc.wantDate {
				t.Fatalf("form=%v date=%q, want %v %q", load.LatestForm, load.LatestLoadDate, tc.wantTSB, tc.wantDate)
			}
		})
	}
}
