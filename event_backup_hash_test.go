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

func TestEventBackupHashIgnoresComputedWorkoutDocFields(t *testing.T) {
	t.Parallel()
	doc := func(ftp, watts, stepSecs, stepPct float64) map[string]any {
		return map[string]any{
			"ftp": ftp, "average_watts": watts, "normalized_power": watts + 5, "zoneTimes": []any{watts},
			"strain_score": watts / 2, "duration": 3600.0,
			"steps": []any{map[string]any{
				"duration": stepSecs, "power": map[string]any{"units": "%ftp", "value": stepPct},
				"_power": map[string]any{"value": ftp * 0.9},
			}},
			"description": "4x8",
		}
	}
	hash := func(d map[string]any) string {
		return icu.EventBackupHash(&icu.Event{ID: 1, Name: "SST", WorkoutDoc: d})
	}
	before := hash(doc(280, 230, 600, 90))
	if before != hash(doc(300, 246, 600, 90)) { // FTP changed, nobody edited it
		t.Fatal("FTP-derived workout_doc fields counted as an edit")
	}
	if before == hash(doc(280, 230, 600, 95)) {
		t.Fatal("edited step target not detected")
	}
	if before == hash(doc(280, 230, 900, 90)) {
		t.Fatal("edited step duration not detected")
	}
}

func TestEventBackupHashTreatsEmptyListsAsMissing(t *testing.T) {
	t.Parallel()
	missing := icu.Event{ID: 1}
	empty := icu.Event{ID: 1, Tags: []string{}, CanTrainSports: []string{}}
	if icu.EventBackupHash(&missing) != icu.EventBackupHash(&empty) {
		t.Fatal("tags [] vs null changed the hash")
	}
	tagged := icu.Event{ID: 1, Tags: []string{"a"}}
	if icu.EventBackupHash(&missing) == icu.EventBackupHash(&tagged) {
		t.Fatal("adding a tag not detected")
	}
}
