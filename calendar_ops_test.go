package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestPreviewCalendarOperationsCountsPendingActions(t *testing.T) {
	t.Parallel()

	preview := icu.PreviewCalendarOperations([]icu.CalendarOperation{
		{ID: "c1", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusPending, ExpectedLoad: 40, ExpectedMovingTimeSecs: 3600, Body: icu.EventEx{Name: "A", StartDateLocal: "2026-07-28T07:00:00"}},
		{ID: "u1", Action: icu.CalendarActionUpdate, Status: icu.CalendarStatusPending, EventID: 2, ExpectedLoad: 50, ExpectedMovingTimeSecs: 1800, Body: icu.EventEx{Name: "B", StartDateLocal: "2026-07-29T07:00:00"}},
		{ID: "x1", Action: icu.CalendarActionCancel, Status: icu.CalendarStatusPending, EventID: 3, ExpectedLoad: 20},
		{ID: "s1", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusSkipped, ExpectedLoad: 10},
	})

	if preview.PendingCreates != 1 || preview.PendingUpdates != 1 || preview.PendingCancels != 1 || preview.ExpectedLoad != 90 || preview.Skipped != 1 {
		t.Fatalf("preview = %+v, want creates=1 updates=1 cancels=1 load=90 skipped=1", preview)
	}
}

func TestCalendarOperationErrorsRequiresEventIDForUpdate(t *testing.T) {
	t.Parallel()

	errors := icu.CalendarOperationErrors(&icu.CalendarOperation{
		ID:     "bad",
		Action: icu.CalendarActionUpdate,
		Status: icu.CalendarStatusPending,
		Body:   icu.EventEx{Name: "Workout"},
	})

	if len(errors) != 1 || errors[0] != "eventId is required for update" {
		t.Fatalf("errors = %v, want eventId required", errors)
	}
}

func TestEventSourceHashMatchesRebalanceEventHash(t *testing.T) {
	t.Parallel()

	event := &icu.Event{ID: 9, Name: "Z2", StartDateLocal: "2026-07-28T07:00:00", Description: "- 60m 70%", TrainingLoad: 49, MovingTime: 3600}

	if icu.EventSourceHash(event) != icu.RebalanceEventHash(event) {
		t.Fatalf("EventSourceHash != RebalanceEventHash")
	}
}

func TestValidateCalendarOperationsCollectsErrors(t *testing.T) {
	t.Parallel()

	errors := icu.ValidateCalendarOperations([]icu.CalendarOperation{
		{Action: icu.CalendarActionCreate, Status: icu.CalendarStatusPending},
	})

	if len(errors) < 2 {
		t.Fatalf("errors = %v, want id and name errors", errors)
	}
}
