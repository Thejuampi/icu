package icu_test

import (
	"encoding/base64"
	"errors"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestDecodeWorkoutDocMissingReturnsSentinel(t *testing.T) {
	t.Parallel()

	_, err := icu.DecodeWorkoutDoc(nil)

	if !errors.Is(err, icu.ErrWorkoutDocMissing) {
		t.Fatalf("DecodeWorkoutDoc nil error = %v, want ErrWorkoutDocMissing", err)
	}
}

func TestDecodeWorkoutDocAcceptsTypedValue(t *testing.T) {
	t.Parallel()

	got, err := icu.DecodeWorkoutDoc(icu.WorkoutDoc{Duration: 120})

	if err != nil || got.Duration != 120 {
		t.Fatalf("DecodeWorkoutDoc typed = %+v err=%v, want duration 120", got, err)
	}
}

func TestDecodeWorkoutFileBase64JSON(t *testing.T) {
	t.Parallel()

	encoded := base64.StdEncoding.EncodeToString([]byte(`{"duration":300}`))
	got, err := icu.DecodeWorkoutFileBase64(encoded, "workout.json")

	if err != nil || got.Duration != 300 {
		t.Fatalf("DecodeWorkoutFileBase64 = %+v err=%v, want duration 300", got, err)
	}
}

func TestDecodeWorkoutFileBase64Unsupported(t *testing.T) {
	t.Parallel()

	encoded := base64.StdEncoding.EncodeToString([]byte(`zwo`))
	_, err := icu.DecodeWorkoutFileBase64(encoded, "workout.zwo")

	if err == nil {
		t.Fatal("DecodeWorkoutFileBase64 unsupported error = nil, want error")
	}
}

func TestDecodeWorkoutDocExpandsRepeatSteps(t *testing.T) {
	t.Parallel()

	raw := map[string]any{
		"duration": float64(1860),
		"steps": []any{
			map[string]any{"duration": float64(600), "power": map[string]any{"start": float64(55), "end": float64(75), "units": "%ftp"}, "ramp": true},
			map[string]any{"reps": float64(2), "steps": []any{
				map[string]any{"duration": float64(300), "power": map[string]any{"start": float64(90), "end": float64(94), "units": "%ftp"}},
				map[string]any{"duration": float64(180), "power": map[string]any{"start": float64(60), "end": float64(65), "units": "%ftp"}},
			}},
			map[string]any{"duration": float64(300), "power": map[string]any{"value": float64(55), "units": "%ftp"}},
		},
	}

	doc, err := icu.DecodeWorkoutDoc(raw)
	if err != nil {
		t.Fatalf("DecodeWorkoutDoc error = %v", err)
	}
	got := icu.ExpandWorkoutSteps(doc)

	if len(got) != 6 {
		t.Fatalf("expanded steps = %d, want 6", len(got))
	}
}

func TestExpandWorkoutStepsClassifiesWorkAndRecovery(t *testing.T) {
	t.Parallel()

	doc := &icu.WorkoutDoc{Steps: []icu.WorkoutStep{{Reps: 1, Steps: []icu.WorkoutStep{
		{Duration: 1080, Power: &icu.WorkoutTarget{Start: 90, End: 94, Units: "%ftp"}},
		{Duration: 360, Power: &icu.WorkoutTarget{Start: 60, End: 65, Units: "%ftp"}},
	}}}}

	got := icu.ExpandWorkoutSteps(doc)

	if got[0].Kind != "work" || got[1].Kind != "recovery" {
		t.Fatalf("step kinds = %q/%q, want work/recovery", got[0].Kind, got[1].Kind)
	}
}

func TestExpandWorkoutStepsClassifiesWarmupCooldownAndOther(t *testing.T) {
	t.Parallel()

	doc := &icu.WorkoutDoc{Steps: []icu.WorkoutStep{
		{Text: "Warmup", Duration: 60},
		{Text: "Cooldown", Duration: 60},
		{Duration: 60},
	}}

	got := icu.ExpandWorkoutSteps(doc)

	if got[0].Kind != "warmup" || got[1].Kind != "cooldown" || got[2].Kind != "other" {
		t.Fatalf("step kinds = %q/%q/%q, want warmup/cooldown/other", got[0].Kind, got[1].Kind, got[2].Kind)
	}
}
