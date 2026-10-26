package icu_test

import (
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestPlanMonotonyWarnings(t *testing.T) {
	t.Parallel()

	threshold := "- 10m 60%\n\n3x\n- 8m 95%\n- 4m 55%\n\n- 10m 50%"
	sessions := []icu.PlanResolvedSession{
		{Date: "2026-11-02", Name: "A", Action: icu.CalendarActionCreate, Desc: threshold},
		// Same structure written with indentation and different spacing.
		{
			Date: "2026-11-04", Name: "B", Action: icu.CalendarActionCreate,
			Desc: "- 10m 60%\n3x\n  - 8m 95%\n  - 4m 55%\n- 10m 50%",
		},
		{Date: "2026-11-06", Name: "C", Action: icu.CalendarActionCancel, Desc: threshold},
		{Date: "2026-11-09", Name: "D", Action: icu.CalendarActionCreate, Desc: threshold},
		{Date: "2026-11-03", Name: "E", Action: icu.CalendarActionCreate, Desc: "Free ride   with friends"},
		{Date: "2026-11-05", Name: "F", Action: icu.CalendarActionUpdate, Desc: "free ride with FRIENDS"},
		{Date: "2026-11-07", Name: "G", Action: icu.CalendarActionCreate},
		{Date: "2026-11-08", Name: "H", Action: icu.CalendarActionCreate},
	}

	got := icu.PlanMonotonyWarnings(sessions, 0)
	joined := strings.Join(got, "\n")
	if len(got) != 2 || !strings.Contains(joined, "2 sessions with the same structure (A, B on 2026-11-02, 2026-11-04)") ||
		!strings.Contains(joined, "(E, F on") || strings.Contains(joined, "D") {
		t.Fatalf("warnings = %v", got)
	}
	if relaxed := icu.PlanMonotonyWarnings(sessions, 2); len(relaxed) != 0 {
		t.Fatalf("relaxed warnings = %v", relaxed)
	}
}

func TestWorkoutStructureSignatureDistinguishesTargets(t *testing.T) {
	t.Parallel()

	if icu.WorkoutStructureSignature("- 20m 90%") == icu.WorkoutStructureSignature("- 20m 91%") {
		t.Fatal("different targets share a signature")
	}
	if icu.WorkoutStructureSignature("- 10m 50-70%") != icu.WorkoutStructureSignature("-  10m  50-70%") {
		t.Fatal("ramp spacing changed the signature")
	}
	if icu.WorkoutStructureSignature("   ") != "" {
		t.Fatal("blank description has a signature")
	}
}
