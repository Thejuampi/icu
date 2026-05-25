package icu_test

import (
	"encoding/json"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestAthleteJSONRoundtrip(t *testing.T) {
	t.Parallel()

	var a icu.Athlete
	a.ID = "i123"
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
	e.Category = "WORKOUT"
	e.Type = "Ride"
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

func TestSportSettingsJSONRoundtrip(t *testing.T) {
	t.Parallel()

	var s icu.SportSettings
	s.Types = []string{"Ride", "VirtualRide"}
	s.FTP = 290
	s.LTHR = 178
	s.MaxHR = 198

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	var got icu.SportSettings
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if got.FTP != s.FTP || got.LTHR != s.LTHR {
		t.Errorf("SportSettings roundtrip: got %+v, want %+v", got, s)
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
