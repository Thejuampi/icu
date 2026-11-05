package icu_test

import (
	"errors"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestParseWorkoutDescriptionZones(t *testing.T) {
	t.Parallel()
	want := map[string]float64{"Z1": 50, "Z2": 65, "Z3": 82.5, "Z4": 97.5, "Z5": 112.5, "Z6": 135, "z7": 160}
	for zone, pct := range want {
		doc, err := icu.ParseWorkoutDescription("- 10m " + zone + " easy spin")
		if err != nil {
			t.Fatalf("%s: %v", zone, err)
		}
		step := doc.Steps[0]
		if step.Duration != 600 || step.Ramp || step.Power.Units != "%ftp" || step.Power.Value != pct || step.Text != "easy spin" {
			t.Fatalf("%s: step %+v power %+v", zone, step, *step.Power)
		}
	}
	doc, err := icu.ParseWorkoutDescription("- 5m Steady Z2")
	if err != nil || doc.Steps[0].Text != "Steady" || doc.Steps[0].Power.Value != 65 {
		t.Fatalf("keyword + zone: %+v err=%v", doc, err)
	}
}

func TestParseWorkoutDescriptionWatts(t *testing.T) {
	t.Parallel()
	cases := map[string]float64{
		"- 10m 110-130w easy spin": 120, "- 10m 200w": 200, "- 10m 200 W": 200, "- 2m 300 - 340W hard": 320,
	}
	for line, watts := range cases {
		doc, err := icu.ParseWorkoutDescription(line)
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		p := doc.Steps[0].Power
		if p.Units != "w" || p.Value != watts || p.Start != 0 || p.End != 0 || doc.Steps[0].Ramp {
			t.Fatalf("%q: power %+v", line, *p)
		}
	}
	if doc, _ := icu.ParseWorkoutDescription("- 10m 110-130w easy spin"); doc.Steps[0].Text != "easy spin" {
		t.Fatalf("text %q", doc.Steps[0].Text)
	}
	// "w" must be a unit, not the start of a word.
	if _, err := icu.ParseWorkoutDescription("- 10m 200 warmup"); err == nil {
		t.Fatal(`"200 warmup" must not parse as watts`)
	}
}

func TestParseWorkoutDescriptionMixedWithRepeat(t *testing.T) {
	t.Parallel()
	doc, err := icu.ParseWorkoutDescription("Synthetic session\n- 10m Z1 easy spin\n\n3x\n- 4m 250-270w\n- 2m Z1\n\n- 10m 110-130w easy spin")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Steps) != 3 || doc.Steps[1].Reps != 3 || doc.Duration != 600+3*360+600 {
		t.Fatalf("doc %+v", doc)
	}
}

func TestParseWorkoutDescriptionActionableError(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"- 10m Z1\n- 10m Z8 too hard":     "line 2",
		"- 10m 70%\n\n- 10m fast":         "line 3",
		"3x\n- 4m 90%\n- 1m recovery":     "line 3",
		"- 10m Z2\n2x\n  - 1m 300w\n  3x": "line 4",
	}
	for desc, line := range cases {
		_, err := icu.ParseWorkoutDescription(desc)
		if err == nil || !errors.Is(err, icu.ErrWorkoutDescriptionUnsupported) {
			t.Fatalf("%q: err %v", desc, err)
		}
		msg := err.Error()
		for _, want := range []string{line + ":", "supported step forms", `"- 10m Z2"`, `"- 10m 110-130w"`, `"- 10m 55-75%"`} {
			if !strings.Contains(msg, want) {
				t.Fatalf("%q: %q missing %q", desc, msg, want)
			}
		}
	}
}
