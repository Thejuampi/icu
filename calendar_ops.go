package icu

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// Calendar operation types are aliases of the rebalance operation shape so
// plan and rebalance proposal JSON stay identical for apply/preview.
type (
	CalendarOperation    = RebalanceOperation
	CalendarApplySummary = RebalanceApplySummary
	CalendarApplyResult  = RebalanceApplyResult
)

const (
	CalendarActionCreate = RebalanceActionCreate
	CalendarActionUpdate = RebalanceActionUpdate
	CalendarActionCancel = RebalanceActionCancel

	CalendarStatusPending = RebalanceStatusPending
	CalendarStatusSkipped = RebalanceStatusSkipped
	CalendarStatusApplied = RebalanceStatusApplied
	CalendarStatusFailed  = RebalanceStatusFailed
)

type CalendarOpsPreview struct {
	PendingCreates         int                      `json:"pendingCreates"`
	PendingUpdates         int                      `json:"pendingUpdates"`
	PendingCancels         int                      `json:"pendingCancels"`
	Pending                int                      `json:"pending"`
	Skipped                int                      `json:"skipped"`
	Applied                int                      `json:"applied"`
	Failed                 int                      `json:"failed"`
	ExpectedLoad           int                      `json:"expectedLoad"`
	ExpectedMovingTimeSecs int                      `json:"expectedMovingTimeSecs"`
	Operations             []CalendarOpsPreviewItem `json:"operations,omitempty"`
	Warnings               []string                 `json:"warnings,omitempty"`
	Validation             *CalendarOpsValidation   `json:"validation,omitempty"`
	// Projection is filled by plan preview's live check only.
	Projection *LoadProjection `json:"projection,omitempty"`
}

type CalendarOpsValidation struct {
	Feasible bool     `json:"feasible"`
	Blocking bool     `json:"blocking"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

type CalendarOpsPreviewItem struct {
	ID           string `json:"id"`
	Action       string `json:"action"`
	Status       string `json:"status,omitempty"`
	EventID      int    `json:"eventId,omitempty"`
	Name         string `json:"name,omitempty"`
	Date         string `json:"date,omitempty"`
	ExpectedLoad int    `json:"expectedLoad,omitempty"`
	// Description is the event description as it will render, one line per
	// entry, so the preview shows exactly what will be written.
	Description []string `json:"description,omitempty"`
}

func EventSourceHash(event *Event) string {
	return RebalanceEventHash(event)
}

// EventBackupHash fingerprints every user-editable field of an event (the
// fields a person can change in Intervals.icu), so the event-backup
// third-party-edit check notices any such edit. Server-computed fields
// (CTL, ATL, intensity, strain, created-by) are left out: they can change
// without anyone editing the event.
func EventBackupHash(event *Event) string {
	if event == nil {
		return ""
	}
	snapshot := struct {
		ID                   int      `json:"id"`
		StartDateLocal       string   `json:"startDateLocal"`
		EndDateLocal         string   `json:"endDateLocal"`
		Category             string   `json:"category"`
		Type                 string   `json:"type"`
		Name                 string   `json:"name"`
		Description          string   `json:"description"`
		TrainingLoad         int      `json:"trainingLoad"`
		MovingTime           int      `json:"movingTime"`
		Distance             float64  `json:"distance"`
		Color                string   `json:"color"`
		Indoor               *bool    `json:"indoor"`
		Target               string   `json:"target"`
		UID                  string   `json:"uid"`
		Tags                 []string `json:"tags"`
		ExternalID           string   `json:"externalId"`
		HideFromAthlete      bool     `json:"hideFromAthlete"`
		AthleteCannotEdit    bool     `json:"athleteCannotEdit"`
		LoadTarget           int      `json:"loadTarget"`
		TimeTarget           int      `json:"timeTarget"`
		DistanceTarget       float64  `json:"distanceTarget"`
		TrainingAvailability string   `json:"trainingAvailability"`
		MaxTrainingTime      int      `json:"maxTrainingTime"`
		CanTrainSports       []string `json:"canTrainSports"`
		WorkoutDoc           any      `json:"workoutDoc"`
	}{
		event.ID, event.StartDateLocal, event.EndDateLocal, event.Category, event.Type, event.Name,
		event.Description, event.TrainingLoad, event.MovingTime, event.Distance, event.Color, event.Indoor,
		event.Target, event.UID, event.Tags, event.ExternalID, event.HideFromAthlete, event.AthleteCannotEdit,
		event.LoadTarget, event.TimeTarget, event.DistanceTarget, event.TrainingAvailability,
		event.MaxTrainingTime, event.CanTrainSports, event.WorkoutDoc,
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

func PreviewCalendarOperations(operations []CalendarOperation) CalendarOpsPreview {
	var preview CalendarOpsPreview
	for index := range operations {
		operation := &operations[index]
		item := CalendarOpsPreviewItem{
			ID:           operation.ID,
			Action:       operation.Action,
			Status:       operation.Status,
			EventID:      operation.EventID,
			Name:         operation.Body.Name,
			Date:         eventDateOnly(operation.Body.StartDateLocal),
			ExpectedLoad: operation.ExpectedLoad,
			Description:  RenderDescriptionLines(operation.Body.Description),
		}
		preview.Operations = append(preview.Operations, item)
		status := operation.Status
		if status == "" {
			status = CalendarStatusPending
		}
		switch status {
		case CalendarStatusPending:
			preview.Pending++
			switch operation.Action {
			case CalendarActionCreate:
				preview.PendingCreates++
			case CalendarActionUpdate:
				preview.PendingUpdates++
			case CalendarActionCancel:
				preview.PendingCancels++
			}
			if operation.Action != CalendarActionCancel {
				preview.ExpectedLoad += operation.ExpectedLoad
				preview.ExpectedMovingTimeSecs += operation.ExpectedMovingTimeSecs
			}
		case CalendarStatusSkipped:
			preview.Skipped++
		case CalendarStatusApplied:
			preview.Applied++
		case CalendarStatusFailed:
			preview.Failed++
		}
	}

	return preview
}

func CalendarOperationErrors(operation *CalendarOperation) []string {
	if operation == nil {
		return []string{"operation is required"}
	}
	var errors []string
	if operation.ID == "" {
		errors = append(errors, "operation id is required")
	}
	if !validRebalanceAction(operation.Action) {
		errors = append(errors, "operation action is unsupported: "+operation.Action)
	}
	if !validRebalanceStatus(operation.Status) {
		errors = append(errors, "operation status is unsupported: "+operation.Status)
	}
	if operation.Action != CalendarActionCreate && operation.EventID == 0 {
		errors = append(errors, "eventId is required for "+operation.Action)
	}
	if err := ValidateDescriptionText(operation.Body.Description); err != nil && operation.Action != CalendarActionCancel {
		errors = append(errors, operation.ID+": "+err.Error())
	}
	if operation.Action != CalendarActionCancel && operation.Body.Name == "" {
		errors = append(errors, "body.name is required for "+operation.Action)
	}

	return errors
}

func ValidateCalendarOperations(operations []CalendarOperation) []string {
	var errors []string
	for index := range operations {
		errors = append(errors, CalendarOperationErrors(&operations[index])...)
	}

	return errors
}

func cancelCategoriesAllowed(categories []string) map[string]bool {
	allowed := map[string]bool{}
	if len(categories) == 0 {
		allowed[strings.ToUpper(planWorkoutCategory)] = true

		return allowed
	}
	for _, category := range categories {
		if category == "" {
			continue
		}
		allowed[strings.ToUpper(category)] = true
	}

	return allowed
}
