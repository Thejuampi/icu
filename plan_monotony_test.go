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

	got := icu.PlanMonotonyWarnings(sessions, 1)
	joined := strings.Join(got, "\n")
	if len(got) != 2 || !strings.Contains(joined, "2 sessions with the same structure (A, B on 2026-11-02, 2026-11-04)") ||
		!strings.Contains(joined, "(E, F on") || strings.Contains(joined, "D") {
		t.Fatalf("warnings = %v", got)
	}
	if relaxed := icu.PlanMonotonyWarnings(sessions, 2); len(relaxed) != 0 {
		t.Fatalf("relaxed warnings = %v", relaxed)
	}
}

func TestPlanMonotonyDefaultAllowsTwoRepeats(t *testing.T) {
	t.Parallel()

	session := func(date, name string) icu.PlanResolvedSession {
		return icu.PlanResolvedSession{Date: date, Name: name, Action: icu.CalendarActionCreate, Desc: "- 20m 90%"}
	}
	two := []icu.PlanResolvedSession{session("2026-11-02", "A"), session("2026-11-04", "B")}
	if got := icu.PlanMonotonyWarnings(two, 0); len(got) != 0 {
		t.Fatalf("two repeats warned by default: %v", got)
	}
	three := make([]icu.PlanResolvedSession, 0, len(two)+1)
	three = append(three, two...)
	three = append(three, session("2026-11-06", "C"))
	got := icu.PlanMonotonyWarnings(three, 0)
	if len(got) != 1 || !strings.Contains(got[0], "3 sessions") || !strings.Contains(got[0], "limit 2") {
		t.Fatalf("third repeat warnings = %v", got)
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
