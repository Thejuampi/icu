package icu

import "strings"

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
}

func EventSourceHash(event *Event) string {
	return RebalanceEventHash(event)
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
