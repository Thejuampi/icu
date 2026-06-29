package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestFormatWorkoutDescriptionSteadyStep(t *testing.T) {
	t.Parallel()

	doc := &icu.WorkoutDoc{
		Steps: []icu.WorkoutStep{
			{Duration: 3600, Power: &icu.WorkoutTarget{Value: 70, Units: "%ftp"}},
		},
	}
	got := icu.FormatWorkoutDescription(doc)
	want := "- 60m 70% "
	if got != want {
		t.Fatalf("FormatWorkoutDescription steady = %q, want %q", got, want)
	}
}

func TestFormatWorkoutDescriptionRampStep(t *testing.T) {
	t.Parallel()

	doc := &icu.WorkoutDoc{
		Steps: []icu.WorkoutStep{
			{Duration: 600, Ramp: true, Power: &icu.WorkoutTarget{Start: 55, End: 75, Units: "%ftp"}},
		},
	}
	got := icu.FormatWorkoutDescription(doc)
	want := "- 10m 55-75% "
	if got != want {
		t.Fatalf("FormatWorkoutDescription ramp = %q, want %q", got, want)
	}
}

func TestFormatWorkoutDescriptionRepeatBlock(t *testing.T) {
	t.Parallel()

	doc := &icu.WorkoutDoc{
		Steps: []icu.WorkoutStep{
			{Reps: 3, Steps: []icu.WorkoutStep{
				{Duration: 300, Power: &icu.WorkoutTarget{Value: 120, Units: "%ftp"}},
				{Duration: 180, Power: &icu.WorkoutTarget{Value: 50, Units: "%ftp"}},
			}},
		},
	}
	got := icu.FormatWorkoutDescription(doc)
	want := "3x\n  - 5m 120% \n  - 3m 50% "
	if got != want {
		t.Fatalf("FormatWorkoutDescription repeat =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatWorkoutDescriptionEmptyDoc(t *testing.T) {
	t.Parallel()

	if got := icu.FormatWorkoutDescription(nil); got != "" {
		t.Fatalf("FormatWorkoutDescription nil = %q, want empty", got)
	}
	if got := icu.FormatWorkoutDescription(&icu.WorkoutDoc{}); got != "" {
		t.Fatalf("FormatWorkoutDescription empty = %q, want empty", got)
	}
}

func TestRebalanceScaleWorkoutDescriptionRoundTrip(t *testing.T) {
	t.Parallel()

	origDesc := "- 60m 70% "
	event := icu.Event{
		Description: origDesc,
	}
	// ratio=1.0 should return original description
	got := icu.RebalanceScaleWorkoutDescription(&event, 0.70, 0.70)
	if got != origDesc {
		t.Fatalf("description unchanged = %q, want %q", got, origDesc)
	}
}

func TestRebalanceScaleWorkoutDescriptionFromWorkoutDoc(t *testing.T) {
	t.Parallel()

	doc := icu.WorkoutDoc{
		Steps: []icu.WorkoutStep{
			{Duration: 3600, Power: &icu.WorkoutTarget{Value: 70, Units: "%ftp"}},
		},
	}
	event := icu.Event{
		WorkoutDoc: doc,
	}
	// Scale from 0.70 IF to 0.85 IF → ratio ≈ 1.214
	got := icu.RebalanceScaleWorkoutDescription(&event, 0.70, 0.85)
	if got == "" {
		t.Fatalf("expected non-empty scaled description")
	}
	// Should have a higher power target
	want := "- 60m 85% "
	if got != want {
		t.Fatalf("scaled description = %q, want %q", got, want)
	}
}

func TestRebalanceScaleWorkoutDescriptionFromParsedDescription(t *testing.T) {
	t.Parallel()

	event := icu.Event{
		Description: "- 60m 70% ",
	}
	// Scale from 0.70 IF to 0.56 IF → ratio = 0.8
	got := icu.RebalanceScaleWorkoutDescription(&event, 0.70, 0.56)
	if got == "" {
		t.Fatalf("expected non-empty scaled description from parsed")
	}
	want := "- 60m 56% "
	if got != want {
		t.Fatalf("scaled description from parsed = %q, want %q", got, want)
	}
}

func TestRebalanceScaleWorkoutDescriptionNoStructure(t *testing.T) {
	t.Parallel()

	event := icu.Event{}
	got := icu.RebalanceScaleWorkoutDescription(&event, 0.70, 0.85)
	if got != "" {
		t.Fatalf("no-structure event should return empty, got %q", got)
	}
}

func TestFormatWorkoutDescriptionWithHoursAndMinutes(t *testing.T) {
	t.Parallel()

	doc := icu.WorkoutDoc{
		Steps: []icu.WorkoutStep{
			{Duration: 10800, Power: &icu.WorkoutTarget{Value: 70, Units: "%ftp"}},
		},
	}
	got := icu.FormatWorkoutDescription(&doc)
	if got != "- 180m 70% " {
		t.Fatalf("expected - 180m 70%%, got %q", got)
	}
}

func TestFormatWorkoutDescriptionNestedBlock(t *testing.T) {
	t.Parallel()

	doc := icu.WorkoutDoc{
		Steps: []icu.WorkoutStep{
			{
				Duration: 1800,
				Steps: []icu.WorkoutStep{
					{Duration: 1800, Power: &icu.WorkoutTarget{Value: 80, Units: "%ftp"}},
				},
			},
		},
	}
	got := icu.FormatWorkoutDescription(&doc)
	if got == "" {
		t.Fatalf("nested block should produce non-empty description, got empty")
	}
}

func TestRebalanceScaleWorkoutNonPowerStep(t *testing.T) {
	t.Parallel()

	event := icu.Event{WorkoutDoc: icu.WorkoutDoc{
		Steps: []icu.WorkoutStep{
			{Duration: 3000, Text: "Free ride", Power: nil},
		},
	}}
	got := icu.RebalanceScaleWorkoutDescription(&event, 0.70, 0.85)
	if got == "" {
		t.Fatalf("non-power step should produce non-empty description, got empty")
	}
}

func TestFormatWorkoutDescriptionMinuteRemainder(t *testing.T) {
	t.Parallel()

	doc := icu.WorkoutDoc{
		Steps: []icu.WorkoutStep{
			{Duration: 3900, Power: &icu.WorkoutTarget{Value: 75, Units: "%ftp"}},
		},
	}
	got := icu.FormatWorkoutDescription(&doc)
	if got != "- 65m 75% " {
		t.Fatalf("expected - 65m 75%%, got %q", got)
	}
}

func TestRebalanceScaleWorkoutDescriptionNilEvent(t *testing.T) {
	t.Parallel()

	got := icu.RebalanceScaleWorkoutDescription(nil, 0.70, 0.85)
	if got != "" {
		t.Fatalf("nil event should return empty, got %q", got)
	}
}

func TestRebalanceScaleWorkoutDescriptionWorkoutDocWithNestedSteps(t *testing.T) {
	t.Parallel()

	doc := icu.WorkoutDoc{
		Steps: []icu.WorkoutStep{
			{
				Steps: []icu.WorkoutStep{
					{Duration: 1800, Power: &icu.WorkoutTarget{Value: 80, Units: "%ftp"}},
				},
			},
		},
	}
	event := icu.Event{WorkoutDoc: doc}
	got := icu.RebalanceScaleWorkoutDescription(&event, 0.80, 0.90)
	if got == "" {
		t.Fatalf("expected scaled description for nested steps")
	}
}

func TestRebalanceScaleWorkoutDescriptionDescOnlyNoNumeric(t *testing.T) {
	t.Parallel()

	event := icu.Event{Description: "Free ride"}
	got := icu.RebalanceScaleWorkoutDescription(&event, 0.70, 0.85)
	if got != "Free ride" {
		t.Fatalf("unparseable description should return original, got %q", got)
	}
}
