package icu_test

import (
	"reflect"
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
			Decoupling:         3.1,
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
