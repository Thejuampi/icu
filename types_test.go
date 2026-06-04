package icu_test

import (
	"encoding/json"
	"reflect"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestAthleteJSONRoundtrip(t *testing.T) {
	t.Parallel()

	var a icu.Athlete
	a.ID = testAthleteID
	a.Name = "Juan"
	a.Weight = 81
	a.Height = 1.81

	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}

	var got icu.Athlete
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if got.ID != a.ID || got.Name != a.Name || got.Weight != a.Weight {
		t.Errorf("Athlete roundtrip: got %+v, want %+v", got, a)
	}
}

func TestActivityJSONRoundtrip(t *testing.T) {
	t.Parallel()

	var a icu.Activity
	a.ID = "i151187748"
	a.Name = "Morning Ride"
	a.Type = "Ride"
	a.MovingTime = 5300
	a.Distance = 46889.29
	a.TrainingLoad = 85

	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}

	var got icu.Activity
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if got.ID != a.ID || got.Name != a.Name || got.TrainingLoad != a.TrainingLoad {
		t.Errorf("Activity roundtrip: got %+v, want %+v", got, a)
	}
}

func TestWellnessJSONRoundtrip(t *testing.T) {
	t.Parallel()

	var w icu.Wellness
	w.ID = "2026-05-24"
	w.Weight = 81
	w.RestingHR = 50
	w.HRV = 72.5
	w.SleepSecs = 28800
	w.SleepScore = 85
	w.CTL = 65.2
	w.ATL = 55.1

	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}

	var got icu.Wellness
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if got.ID != w.ID || got.Weight != w.Weight || got.HRV != w.HRV {
		t.Errorf("Wellness roundtrip: got %+v, want %+v", got, w)
	}
}

func TestEventJSONRoundtrip(t *testing.T) {
	t.Parallel()

	var e icu.Event
	e.Category = testWorkoutType
	e.Type = testRideType
	e.Name = "Intervals"
	e.StartDateLocal = "2026-05-25T07:00:00"
	e.TrainingLoad = 90
	e.MovingTime = 3600

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	var got icu.Event
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if got.Category != e.Category || got.Name != e.Name || got.TrainingLoad != e.TrainingLoad {
		t.Errorf("Event roundtrip: got %+v, want %+v", got, e)
	}
}

func TestEventExPartialUpdateOmitsCreateOnlyFields(t *testing.T) {
	t.Parallel()

	var event icu.EventEx
	event.Name = "3x4 VO2Max - Re-Entry"
	event.Description = "- Warmup 10m 55%"

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	got := string(data)
	want := `{"name":"3x4 VO2Max - Re-Entry","description":"- Warmup 10m 55%"}`

	if got != want {
		t.Fatalf("EventEx partial JSON = %s, want %s", got, want)
	}
}

func TestSportSettingsJSONRoundtrip(t *testing.T) {
	t.Parallel()

	var settings icu.SportSettings
	settings.Types = []string{"Ride", "VirtualRide"}
	settings.FTP = 290
	settings.LTHR = 178
	settings.MaxHR = 198

	b, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}

	var got icu.SportSettings
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if got.FTP != settings.FTP || got.LTHR != settings.LTHR {
		t.Errorf("SportSettings roundtrip: got %+v, want %+v", got, settings)
	}
}

func TestSportSettingsUnmarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var settings icu.SportSettings

	data := []byte(`{"athlete_id":"` + testAthleteID + `","indoor_ftp":280,"w_prime":21000,` +
		`"p_max":1120,"max_hr":196,"power_zones":[130,190,230],` +
		`"hr_zones":[120,145,170],"pace_zones":[3.2,3.6],` +
		`"threshold_pace":3.8,"pace_units":"min/km","hr_load_type":"TRIMP",` +
		`"pace_load_type":"PACE","gap_model":"hilly"}`)

	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	got := struct {
		AthleteID     string
		IndoorFTP     int
		WPrime        int
		PMax          int
		MaxHR         int
		PowerZones    []int
		HRZones       []int
		PaceZones     []float64
		ThresholdPace float64
		PaceUnits     string
		HRLoadType    string
		PaceLoadType  string
		GapModel      string
	}{
		settings.AthleteID,
		settings.IndoorFTP,
		settings.WPrime,
		settings.PMax,
		settings.MaxHR,
		settings.PowerZones,
		settings.HRZones,
		settings.PaceZones,
		settings.ThresholdPace,
		settings.PaceUnits,
		settings.HRLoadType,
		settings.PaceLoadType,
		settings.GapModel,
	}
	want := struct {
		AthleteID     string
		IndoorFTP     int
		WPrime        int
		PMax          int
		MaxHR         int
		PowerZones    []int
		HRZones       []int
		PaceZones     []float64
		ThresholdPace float64
		PaceUnits     string
		HRLoadType    string
		PaceLoadType  string
		GapModel      string
	}{
		testAthleteID,
		280,
		21000,
		1120,
		196,
		[]int{130, 190, 230},
		[]int{120, 145, 170},
		[]float64{3.2, 3.6},
		3.8,
		"min/km",
		"TRIMP",
		"PACE",
		"hilly",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SportSettings snake fields = %+v, want %+v", got, want)
	}
}

func TestDataCurveUnmarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var curve icu.DataCurve

	data := []byte(`{"id":"42d","label":"42d","start_date_local":"2026-05-01",` +
		`"end_date_local":"2026-06-01","moving_time":7200,"training_load":180,` +
		`"input_point_indexes":[0,3,5],"secs":[60,300],"values":[510,390]}`)

	if err := json.Unmarshal(data, &curve); err != nil {
		t.Fatal(err)
	}

	got := struct {
		StartDate         string
		EndDate           string
		MovingTime        int
		TrainingLoad      int
		InputPointIndexes []int
	}{curve.StartDate, curve.EndDate, curve.MovingTime, curve.TrainingLoad, curve.InputPointIndexes}
	want := struct {
		StartDate         string
		EndDate           string
		MovingTime        int
		TrainingLoad      int
		InputPointIndexes []int
	}{"2026-05-01", "2026-06-01", 7200, 180, []int{0, 3, 5}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DataCurve snake fields = %+v, want %+v", got, want)
	}
}

func TestWeatherSummaryUnmarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var summary icu.WeatherSummary

	data := []byte(`{"average_temp":29.4,"min_temp":18.2,"max_temp":34.1,` +
		`"average_feels_like":31.7,"average_wind_speed":16.8,` +
		`"average_wind_gust":29.1,"headwind_percent":38.5,` +
		`"tailwind_percent":24.4,"description":"hot crosswind"}`)

	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}

	got := struct {
		AvgTemp      float64
		MinTemp      float64
		MaxTemp      float64
		AvgFeelsLike float64
		AvgWindSpeed float64
		AvgWindGust  float64
		HeadwindPct  float64
		TailwindPct  float64
		Description  string
	}{
		summary.AvgTemp,
		summary.MinTemp,
		summary.MaxTemp,
		summary.AvgFeelsLike,
		summary.AvgWindSpeed,
		summary.AvgWindGust,
		summary.HeadwindPct,
		summary.TailwindPct,
		summary.Description,
	}
	want := struct {
		AvgTemp      float64
		MinTemp      float64
		MaxTemp      float64
		AvgFeelsLike float64
		AvgWindSpeed float64
		AvgWindGust  float64
		HeadwindPct  float64
		TailwindPct  float64
		Description  string
	}{29.4, 18.2, 34.1, 31.7, 16.8, 29.1, 38.5, 24.4, "hot crosswind"}

	if got != want {
		t.Fatalf("WeatherSummary snake fields = %+v, want %+v", got, want)
	}
}

func TestForecastUnmarshalLocation(t *testing.T) {
	t.Parallel()

	var forecast icu.Forecast

	data := []byte(`{"id":7,"label":"Home","location":"Medellin","lat":6.2,"lon":-75.6,"enabled":true}`)

	if err := json.Unmarshal(data, &forecast); err != nil {
		t.Fatal(err)
	}

	got := forecast.Location
	want := "Medellin"

	if got != want {
		t.Fatalf("Forecast.Location = %q, want %q", got, want)
	}
}

func TestCustomItemUnmarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var item icu.CustomItem

	data := []byte(`{"id":5,"athlete_id":"` + testAthleteID + `","name":"Lactate","type":"NUMBER"}`)

	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatal(err)
	}

	got := item.AthleteID
	want := testAthleteID

	if got != want {
		t.Fatalf("CustomItem.AthleteID = %q, want %q", got, want)
	}
}

func TestPowerHRCurveUnmarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var curve icu.PowerHRCurve

	data := []byte(`{"athlete_id":"` + testAthleteID + `","min_watts":50,"max_watts":600,"bucket_size":5,"max_hr":198}`)

	if err := json.Unmarshal(data, &curve); err != nil {
		t.Fatal(err)
	}

	got := struct {
		AthleteID  string
		MinWatts   int
		MaxWatts   int
		BucketSize int
		MaxHR      int
	}{curve.AthleteID, curve.MinWatts, curve.MaxWatts, curve.BucketSize, curve.MaxHR}
	want := struct {
		AthleteID  string
		MinWatts   int
		MaxWatts   int
		BucketSize int
		MaxHR      int
	}{testAthleteID, 50, 600, 5, 198}

	if got != want {
		t.Fatalf("PowerHRCurve snake fields = %+v, want %+v", got, want)
	}
}

func TestGearJSONRoundtrip(t *testing.T) {
	t.Parallel()

	var g icu.Gear
	g.Name = "Carmelita"
	g.Type = "Bike"
	g.Distance = 22214500
	g.Activities = 500

	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}

	var got icu.Gear
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if got.Name != g.Name || got.Type != g.Type || got.Distance != g.Distance {
		t.Errorf("Gear roundtrip: got %+v, want %+v", got, g)
	}
}
