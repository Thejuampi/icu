package icu_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestBuildCoachingContextExtractsNotes(t *testing.T) {
	t.Parallel()

	context := icu.BuildCoachingContext(icu.CoachingContextInputs{
		Athlete:       &icu.Athlete{ID: "i123", Name: "Rider"},
		SportSettings: &icu.SportSettings{FTP: 285},
		Events: []icu.Event{
			{ID: 1, Category: "WORKOUT", Name: "Endurance", StartDateLocal: "2026-06-29T08:00:00"},
			{ID: 2, Category: "NOTE", Name: "Travel", Description: "late flight", StartDateLocal: "2026-06-30T00:00:00"},
		},
	}, &icu.CoachingContextOptions{
		SportType:         "Ride",
		HistoryStartDate:  "2026-04-06",
		HistoryEndDate:    "2026-06-28",
		PlanStartDate:     "2026-06-29",
		PlanEndDate:       "2026-07-26",
		Timezone:          "UTC",
		TimezoneSource:    "explicit",
		IncludeAdaptation: false,
	})

	if len(context.Calendar.Notes) != 1 || context.Calendar.Notes[0].Description != "late flight" {
		t.Fatalf("notes = %+v, want one extracted NOTE with description", context.Calendar.Notes)
	}
}

func TestBuildCoachingContextReportsMissingSources(t *testing.T) {
	t.Parallel()

	context := icu.BuildCoachingContext(icu.CoachingContextInputs{}, &icu.CoachingContextOptions{})

	if len(context.DataQuality.Missing) != 5 {
		t.Fatalf("missing = %v, want five required missing sections", context.DataQuality.Missing)
	}
}

func TestBuildCoachingContextSupportsNilOptionsAndUnavailableAdaptation(t *testing.T) {
	t.Parallel()

	withoutOptions := icu.BuildCoachingContext(icu.CoachingContextInputs{}, nil)
	if withoutOptions.Scope.Command != "analysis coaching" {
		t.Fatalf("command = %q, want analysis coaching", withoutOptions.Scope.Command)
	}

	withAdaptation := icu.BuildCoachingContext(icu.CoachingContextInputs{}, &icu.CoachingContextOptions{IncludeAdaptation: true})
	if !strings.Contains(strings.Join(withAdaptation.DataQuality.Warnings, "\n"), "adaptation requested but unavailable") {
		t.Fatalf("warnings = %v, want unavailable adaptation", withAdaptation.DataQuality.Warnings)
	}
}

func TestBuildCoachingContextIncludesSideEffectDeclaration(t *testing.T) {
	t.Parallel()

	context := icu.BuildCoachingContext(icu.CoachingContextInputs{}, &icu.CoachingContextOptions{})

	if context.SideEffects.MutatedPlan || context.SideEffects.MutatedConfig || context.SideEffects.SyncedExternalData {
		t.Fatalf("sideEffects = %+v, want read-only false values", context.SideEffects)
	}
}

func TestBuildCoachingContextKeepsEvents(t *testing.T) {
	t.Parallel()

	context := icu.BuildCoachingContext(icu.CoachingContextInputs{
		Events: []icu.Event{{ID: 1, Category: "WORKOUT"}, {ID: 2, Category: "NOTE"}},
	}, &icu.CoachingContextOptions{})

	if len(context.Calendar.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(context.Calendar.Events))
	}
}

func TestBuildCoachingContextSerializesEmptyCalendarArrays(t *testing.T) {
	t.Parallel()

	context := icu.BuildCoachingContext(icu.CoachingContextInputs{}, &icu.CoachingContextOptions{})
	encoded, err := json.Marshal(context.Calendar)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	if string(encoded) != `{"events":[],"notes":[]}` {
		t.Fatalf("calendar JSON = %s, want empty arrays", encoded)
	}
}

func TestBuildCoachingContextDoesNotMutateInputEvents(t *testing.T) {
	t.Parallel()

	events := []icu.Event{{ID: 1, Category: "WORKOUT", Name: "Original"}}
	context := icu.BuildCoachingContext(icu.CoachingContextInputs{Events: events}, &icu.CoachingContextOptions{})
	context.Calendar.Events[0].Name = "Changed"

	if events[0].Name != "Original" {
		t.Fatalf("input event name = %q, want Original", events[0].Name)
	}
}

func TestBuildCoachingContextAggregatesNestedWarnings(t *testing.T) {
	t.Parallel()

	context := icu.BuildCoachingContext(icu.CoachingContextInputs{
		Cycling:    &icu.CyclingAnalysis{Warnings: []string{"empty history", "empty history"}},
		Wellness:   &icu.WellnessAnalysis{Warnings: []string{"sparse records"}},
		Plan:       &icu.TrainingPlanAnalysis{Warnings: []string{"zero planned load"}},
		Adaptation: &icu.CyclingAdaptationAnalysis{Warnings: []string{"missing curves"}},
	}, &icu.CoachingContextOptions{IncludeAdaptation: true})

	want := []string{
		"calendar events unavailable or empty",
		"cycling: empty history",
		"wellness: sparse records",
		"plan: zero planned load",
		"adaptation: missing curves",
	}
	if !reflect.DeepEqual(context.DataQuality.Warnings, want) {
		t.Fatalf("warnings = %v, want %v", context.DataQuality.Warnings, want)
	}
}

func TestBuildCoachingContextOmitsAdaptationWarningsWhenNotRequested(t *testing.T) {
	t.Parallel()

	context := icu.BuildCoachingContext(icu.CoachingContextInputs{
		Events:     []icu.Event{{ID: 1}},
		Adaptation: &icu.CyclingAdaptationAnalysis{Warnings: []string{"missing curves"}},
	}, &icu.CoachingContextOptions{})

	if strings.Contains(strings.Join(context.DataQuality.Warnings, "\n"), "adaptation:") {
		t.Fatalf("warnings = %v, want no adaptation warning", context.DataQuality.Warnings)
	}
}
