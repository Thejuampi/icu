package icu_test

import (
	"errors"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func parseOne(t *testing.T, line string) icu.WorkoutStep {
	t.Helper()
	doc, err := icu.ParseWorkoutDescription(line)
	if err != nil {
		t.Fatalf("%q: %v", line, err)
	}

	return doc.Steps[0]
}

func wantParseError(t *testing.T, desc string, fragments ...string) {
	t.Helper()
	_, err := icu.ParseWorkoutDescription(desc)
	if err == nil || !errors.Is(err, icu.ErrWorkoutDescriptionUnsupported) {
		t.Fatalf("%q: err %v", desc, err)
	}
	for _, want := range fragments {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("%q: %q missing %q", desc, err.Error(), want)
		}
	}
}

// Default bands 55/75/90/105/120/150/999: Z1 50 and top 160 fixed (client
// decision), other zones the intervals.icu midpoint (prev+1 .. upper).
func TestParseWorkoutDescriptionDefaultZones(t *testing.T) {
	t.Parallel()
	want := map[string]float64{"Z1": 50, "Z2": 65.5, "Z3": 83, "Z4": 98, "Z5": 113, "Z6": 135.5, "z7": 160}
	for zone, pct := range want {
		step := parseOne(t, "- 10m "+zone+" easy spin")
		if step.Duration != 600 || step.Ramp || step.Power.Units != "%ftp" || step.Power.Value != pct || step.Text != "easy spin" {
			t.Fatalf("%s: step %+v power %+v", zone, step, *step.Power)
		}
	}
	if step := parseOne(t, "- 5m Steady Z2"); step.Text != "Steady" || step.Power.Value != 65.5 {
		t.Fatalf("keyword + zone: %+v", step)
	}
	wantParseError(t, "- 10m Z8", "line 1", "zone Z8 does not exist", "Z1-Z7")
}

// m4: a non-power keyword before the target is rejected too.
func TestParseWorkoutDescriptionRejectsKeywordBeforeTarget(t *testing.T) {
	t.Parallel()
	for _, desc := range []string{"- 10m HR 70%", "- 5m 50%\n- 10m LTHR 90%", "- 10m pace 80%"} {
		wantParseError(t, desc, "HR, LTHR or Pace, not power")
	}
	wantParseError(t, "- 5m 50%\n- 10m HR 70%", "line 2")
}

// M1: every price stays inside its own band.
func TestPowerZoneMidpointsStayInBand(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		bounds []int
		want   []float64
	}{
		"low Z1, open top":         {[]int{45, 60, 80, 999}, []float64{22.5, 53, 70.5, 160}},
		"high bands, open top":     {[]int{55, 75, 90, 105, 120, 200, 999}, []float64{50, 65.5, 83, 98, 113, 160.5, 210}},
		"finite top":               {[]int{55, 75, 90, 105, 120, 150}, []float64{50, 65.5, 83, 98, 113, 135.5}},
		"open top above 160 bound": {[]int{50, 160, 999}, []float64{50, 105.5, 170}},
		"defaults":                 {icu.DefaultPowerZoneBounds(), []float64{50, 65.5, 83, 98, 113, 135.5, 160}},
	} {
		got := icu.PowerZoneMidpoints(tc.bounds)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: %v", name, got)
		}
		for index := range got {
			if got[index] != tc.want[index] {
				t.Fatalf("%s: got %v want %v", name, got, tc.want)
			}
		}
	}
}

func TestParseWorkoutDescriptionCustomZones(t *testing.T) {
	t.Parallel()
	zones := icu.PowerZoneMidpoints([]int{50, 70, 85, 100, 999})
	if got := []float64{50, 60.5, 78, 93, 160}; len(zones) != 5 || zones[1] != got[1] || zones[2] != got[2] || zones[3] != got[3] || zones[4] != 160 {
		t.Fatalf("midpoints %v want %v", zones, got)
	}
	doc, err := icu.ParseWorkoutDescriptionWithZones("- 10m Z2\n- 10m Z5", zones)
	if err != nil || doc.Steps[0].Power.Value != 60.5 || doc.Steps[1].Power.Value != 160 {
		t.Fatalf("custom zones %+v err=%v", doc, err)
	}
	if _, err := icu.ParseWorkoutDescriptionWithZones("- 10m Z2\n- 10m Z6", zones); err == nil ||
		!strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "Z1-Z5") {
		t.Fatalf("Z6 with 5 zones: %v", err)
	}
	if !icu.WorkoutDescriptionUsesPowerZones("intro\n3x\n  - 4m Ramp z4") || icu.WorkoutDescriptionUsesPowerZones("- 10m 70%\n- 5m 200w") {
		t.Fatal("WorkoutDescriptionUsesPowerZones")
	}
}

// Ramp versus range comes only from the "ramp" keyword, for % and w alike.
func TestParseWorkoutDescriptionRampVersusRange(t *testing.T) {
	t.Parallel()
	if s := parseOne(t, "- 10m 65-75%"); s.Ramp || s.Power.Units != "%ftp" || s.Power.Value != 70 {
		t.Fatalf("%% range: %+v %+v", s, *s.Power)
	}
	if s := parseOne(t, "- 10m ramp 65-75%"); !s.Ramp || s.Power.Units != "%ftp" || s.Power.Start != 65 || s.Power.End != 75 || s.Power.Value != 0 {
		t.Fatalf("%% ramp: %+v %+v", s, *s.Power)
	}
	if s := parseOne(t, "- 10m 110-130w easy spin"); s.Ramp || s.Power.Units != "w" || s.Power.Value != 120 || s.Text != "easy spin" {
		t.Fatalf("w range: %+v %+v", s, *s.Power)
	}
	if s := parseOne(t, "- 10m Ramp 100-200W"); !s.Ramp || s.Power.Units != "w" || s.Power.Start != 100 || s.Power.End != 200 {
		t.Fatalf("w ramp: %+v %+v", s, *s.Power)
	}
	for line, watts := range map[string]float64{"- 10m 200w": 200, "- 10m 200 W": 200, "- 2m 300 - 340W hard": 320} {
		if s := parseOne(t, line); s.Power.Units != "w" || s.Power.Value != watts {
			t.Fatalf("%q: %+v", line, *s.Power)
		}
	}
	wantParseError(t, "- 10m ramp Z2", "ramp needs a % or watt range")
	wantParseError(t, "- 10m ramp 70%", "a ramp needs a range")
	wantParseError(t, "- 10m Z3-Z4", "zone ranges")
	wantParseError(t, "- 10m Z3-4", "zone ranges")
	wantParseError(t, "- 10m 200 warmup")
}

func TestParseWorkoutDescriptionRejectsNonPowerTargets(t *testing.T) {
	t.Parallel()
	for _, desc := range []string{"- 5m 50%\n- 30m Z2 HR", "- 5m 50%\n- 30m Z2 Pace", "- 5m 50%\n- 30m 70% HR", "- 5m 50%\n- 30m 70% lthr", "- 5m 50%\n- 30m 200w pace"} {
		wantParseError(t, desc, "line 2", "HR, LTHR or Pace, not power")
	}
}

func TestParseWorkoutDescriptionTargetBounds(t *testing.T) {
	t.Parallel()
	for _, desc := range []string{"- 10m 0%", "- 10m 0w", "- 10m 0-100w", "- 10m ramp 0-50%"} {
		wantParseError(t, desc, "line 1", "above 0")
	}
	wantParseError(t, "- 10m 50%\n- 10m 1001%", "line 2", "above 1000% FTP")
	wantParseError(t, "- 10m 50%\n- 1m ramp 100-3001w", "line 2", "above 3000 W")
	if s := parseOne(t, "- 10m 1000%"); s.Power.Value != 1000 {
		t.Fatalf("1000%% is allowed: %+v", *s.Power)
	}
	if s := parseOne(t, "- 10s 3000w"); s.Power.Value != 3000 {
		t.Fatalf("3000w is allowed: %+v", *s.Power)
	}
	// Above 300% FTP: priced, with a warning naming the line (m1).
	doc, err := icu.ParseWorkoutDescription("- 10m 50%\n- 10s 1000w")
	if err != nil {
		t.Fatal(err)
	}
	if got := icu.ValidateWorkoutTargets(doc.Steps, 266); len(got) != 1 || got[0] != "line 2: 1000w is above 300% of FTP 266 (798w); check for a typo" {
		t.Fatalf("1000w at FTP 266: %q", got)
	}
	doc, _ = icu.ParseWorkoutDescription("intro\n3x\n  - 1m ramp 100-799w\n  - 1m 798w\n- 1m 301%\n- 1m 300%")
	got := icu.ValidateWorkoutTargets(doc.Steps, 266)
	if len(got) != 2 || !strings.HasPrefix(got[0], "line 3: 799w") || got[1] != "line 5: target 301% FTP is above 300% FTP; check for a typo" {
		t.Fatalf("warnings %q", got)
	}
	if got := icu.ValidateWorkoutTargets(doc.Steps, 0); len(got) != 1 {
		t.Fatalf("no FTP: only the %% warning, got %q", got)
	}
}

func TestParseWorkoutDescriptionCadenceNotInLabel(t *testing.T) {
	t.Parallel()
	for line, text := range map[string]string{
		"- 5m 65% 90rpm easy": "easy", "- 5m 65% 90-100 rpm": "", "- 5m Z2 cadence 90 smooth": "smooth", "- 5m ramp 50-60% 85rpm": "",
	} {
		if s := parseOne(t, line); s.Text != text {
			t.Fatalf("%q: text %q want %q", line, s.Text, text)
		}
	}
}

func TestParseWorkoutDescriptionMixedWithRepeat(t *testing.T) {
	t.Parallel()
	doc, err := icu.ParseWorkoutDescription("Synthetic session\n- 10m Z1 easy spin\n\n3x\n- 4m 250-270w\n- 2m Z1\n\n- 10m 110-130w easy spin")
	if err != nil || len(doc.Steps) != 3 || doc.Steps[1].Reps != 3 || doc.Duration != 600+3*360+600 {
		t.Fatalf("doc %+v err=%v", doc, err)
	}
}

func TestParseWorkoutDescriptionActionableError(t *testing.T) {
	t.Parallel()
	forms := []string{"supported step forms", `"- 10m Z2"`, `"- 10m 110-130w"`, `"- 10m ramp 55-75%"`}
	cases := map[string]string{
		"- 10m Z1\n- 10m Z8 too hard":     "line 2",
		"- 10m 70%\n\n- 10m fast":         "line 3",
		"3x\n- 4m 90%\n- 1m recovery":     "line 3",
		"- 10m Z2\n2x\n  - 1m 300w\n  3x": "line 4",
	}
	for desc, line := range cases {
		wantParseError(t, desc, append([]string{line + ":"}, forms...)...)
	}
}
