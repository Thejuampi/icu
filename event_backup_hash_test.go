package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestEventBackupHashDetectsEveryEditableField(t *testing.T) {
	t.Parallel()
	indoor := true
	outdoor := false
	base := func() icu.Event {
		return icu.Event{
			ID: 1, StartDateLocal: "2026-10-28T07:00:00", Category: "WORKOUT", Type: "Ride", Name: "Z2",
			Description: "- 60m 65%", TrainingLoad: 50, MovingTime: 3600, Indoor: &indoor,
			WorkoutDoc: map[string]any{"steps": []any{1.0}},
		}
	}
	reference := base()
	want := icu.EventBackupHash(&reference)
	if icu.EventBackupHash(nil) != "" || want == "" {
		t.Fatal("unexpected empty/nil hash behaviour")
	}
	changes := map[string]func(*icu.Event){
		"type":             func(e *icu.Event) { e.Type = "VirtualRide" },
		"category":         func(e *icu.Event) { e.Category = "NOTE" },
		"workout_doc":      func(e *icu.Event) { e.WorkoutDoc = map[string]any{"steps": []any{2.0}} },
		"indoor":           func(e *icu.Event) { e.Indoor = &outdoor },
		"indoor unset":     func(e *icu.Event) { e.Indoor = nil },
		"name":             func(e *icu.Event) { e.Name = "Z3" },
		"start":            func(e *icu.Event) { e.StartDateLocal = "2026-10-29T07:00:00" },
		"end":              func(e *icu.Event) { e.EndDateLocal = "2026-10-29T08:00:00" },
		"description":      func(e *icu.Event) { e.Description = "- 30m" },
		"load":             func(e *icu.Event) { e.TrainingLoad = 51 },
		"moving time":      func(e *icu.Event) { e.MovingTime = 3601 },
		"distance":         func(e *icu.Event) { e.Distance = 1 },
		"color":            func(e *icu.Event) { e.Color = "red" },
		"target":           func(e *icu.Event) { e.Target = "POWER" },
		"uid":              func(e *icu.Event) { e.UID = "u" },
		"tags":             func(e *icu.Event) { e.Tags = []string{"x"} },
		"external id":      func(e *icu.Event) { e.ExternalID = "x" },
		"hidden":           func(e *icu.Event) { e.HideFromAthlete = true },
		"locked":           func(e *icu.Event) { e.AthleteCannotEdit = true },
		"load target":      func(e *icu.Event) { e.LoadTarget = 1 },
		"time target":      func(e *icu.Event) { e.TimeTarget = 1 },
		"distance target":  func(e *icu.Event) { e.DistanceTarget = 1 },
		"availability":     func(e *icu.Event) { e.TrainingAvailability = "LIMITED" },
		"max time":         func(e *icu.Event) { e.MaxTrainingTime = 1 },
		"can train sports": func(e *icu.Event) { e.CanTrainSports = []string{"Run"} },
	}
	for name, change := range changes {
		event := base()
		change(&event)
		if icu.EventBackupHash(&event) == want {
			t.Errorf("%s change not detected", name)
		}
	}
	// Server-computed fields do not count as edits.
	event := base()
	event.CTL, event.ATL, event.Intensity = 40, 50, 0.7
	if icu.EventBackupHash(&event) != want {
		t.Error("server-computed fields changed the hash")
	}
}
