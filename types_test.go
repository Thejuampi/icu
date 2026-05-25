package main

import (
	"encoding/json"
	"testing"
)

func TestAthleteJSONRoundtrip(t *testing.T) {
	a := Athlete{ID: "i123", Name: "Juan", Weight: 81, Height: 1.81}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var got Athlete
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != a.ID || got.Name != a.Name || got.Weight != a.Weight {
		t.Errorf("Athlete roundtrip: got %+v, want %+v", got, a)
	}
}

func TestActivityJSONRoundtrip(t *testing.T) {
	a := Activity{
		ID: "i151187748", Name: "Morning Ride", Type: "Ride",
		MovingTime: 5300, Distance: 46889.29, TrainingLoad: 85,
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var got Activity
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != a.ID || got.Name != a.Name || got.TrainingLoad != a.TrainingLoad {
		t.Errorf("Activity roundtrip: got %+v, want %+v", got, a)
	}
}

func TestWellnessJSONRoundtrip(t *testing.T) {
	w := Wellness{
		ID: "2026-05-24", Weight: 81, RestingHR: 50, HRV: 72.5,
		SleepSecs: 28800, SleepScore: 85, CTL: 65.2, ATL: 55.1,
	}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var got Wellness
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != w.ID || got.Weight != w.Weight || got.HRV != w.HRV {
		t.Errorf("Wellness roundtrip: got %+v, want %+v", got, w)
	}
}

func TestEventJSONRoundtrip(t *testing.T) {
	e := Event{
		Category: "WORKOUT", Type: "Ride", Name: "Intervals",
		StartDateLocal: "2026-05-25T07:00:00", TrainingLoad: 90, MovingTime: 3600,
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var got Event
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Category != e.Category || got.Name != e.Name || got.TrainingLoad != e.TrainingLoad {
		t.Errorf("Event roundtrip: got %+v, want %+v", got, e)
	}
}

func TestSportSettingsJSONRoundtrip(t *testing.T) {
	s := SportSettings{
		Types: []string{"Ride", "VirtualRide"}, FTP: 290, LTHR: 178, MaxHR: 198,
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var got SportSettings
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.FTP != s.FTP || got.LTHR != s.LTHR {
		t.Errorf("SportSettings roundtrip: got %+v, want %+v", got, s)
	}
}

func TestGearJSONRoundtrip(t *testing.T) {
	g := Gear{
		Name: "Carmelita", Type: "Bike", Distance: 22214500, Activities: 500,
	}
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var got Gear
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != g.Name || got.Type != g.Type || got.Distance != g.Distance {
		t.Errorf("Gear roundtrip: got %+v, want %+v", got, g)
	}
}
