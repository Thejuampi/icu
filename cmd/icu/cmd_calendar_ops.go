package main

import (
	"fmt"
	"strconv"

	icu "github.com/Thejuampi/icu"
)

func applyCalendarOperations(client *icu.Client, operations []icu.CalendarOperation) *icu.CalendarApplySummary {
	summary := &icu.CalendarApplySummary{}
	for index := range operations {
		operation := &operations[index]
		result := applyCalendarOperation(client, operation)
		summary.Results = append(summary.Results, result)
		countCalendarApplyResult(summary, result)
	}

	return summary
}

func applyCalendarOperation(client *icu.Client, operation *icu.CalendarOperation) icu.CalendarApplyResult {
	result := icu.CalendarApplyResult{OperationID: operation.ID, Action: operation.Action, EventID: operation.EventID}
	if operation.Status != "" && operation.Status != icu.CalendarStatusPending {
		operation.Status = icu.CalendarStatusSkipped
		result.Status = icu.CalendarStatusSkipped

		return result
	}
	if err := verifyCalendarSourceHash(client, operation); err != nil {
		operation.Status = icu.CalendarStatusFailed
		operation.Error = err.Error()
		result.Status = icu.CalendarStatusFailed
		result.Error = err.Error()

		return result
	}
	var event icu.Event
	err := applyCalendarMutation(client, operation, &event)
	if err != nil {
		operation.Status = icu.CalendarStatusFailed
		operation.Error = err.Error()
		result.Status = icu.CalendarStatusFailed
		result.Error = err.Error()

		return result
	}
	operation.Status = icu.CalendarStatusApplied
	operation.AppliedEventID = appliedCalendarEventID(operation, &event)
	result.Status = icu.CalendarStatusApplied
	result.EventID = operation.AppliedEventID

	return result
}

func verifyCalendarSourceHash(client *icu.Client, operation *icu.CalendarOperation) error {
	if operation.Action == icu.CalendarActionCreate || operation.SourceHash == "" {
		return nil
	}
	var current icu.Event
	if err := client.Get("events", []string{strconv.Itoa(operation.EventID)}, nil, &current); err != nil {
		return wrapCommandError(err)
	}
	if got := icu.EventSourceHash(&current); got != operation.SourceHash {
		return fmt.Errorf("event %d changed since dry-run", operation.EventID)
	}

	return nil
}

func applyCalendarMutation(client *icu.Client, operation *icu.CalendarOperation, event *icu.Event) error {
	switch operation.Action {
	case icu.CalendarActionCreate:
		return wrapCommandError(client.Post("events", nil, nil, operation.Body, event))
	case icu.CalendarActionUpdate:
		return wrapCommandError(client.Put("events", []string{strconv.Itoa(operation.EventID)}, nil, operation.Body, event))
	case icu.CalendarActionCancel:
		return wrapCommandError(client.Delete("events", []string{strconv.Itoa(operation.EventID)}, nil, event))
	default:
		return fmt.Errorf("unsupported calendar action %q", operation.Action)
	}
}

func appliedCalendarEventID(operation *icu.CalendarOperation, event *icu.Event) int {
	if event.ID != 0 {
		return event.ID
	}

	return operation.EventID
}

func countCalendarApplyResult(summary *icu.CalendarApplySummary, result icu.CalendarApplyResult) {
	switch result.Status {
	case icu.CalendarStatusApplied:
		summary.Applied++
	case icu.CalendarStatusSkipped:
		summary.Skipped++
	default:
		summary.Failed++
	}
}

func liveCheckCalendarOperations(client *icu.Client, operations []icu.CalendarOperation) []string {
	var warnings []string
	for index := range operations {
		operation := &operations[index]
		if operation.Action == icu.CalendarActionCreate || operation.SourceHash == "" {
			continue
		}
		if operation.Status != "" && operation.Status != icu.CalendarStatusPending {
			continue
		}
		if err := verifyCalendarSourceHash(client, operation); err != nil {
			warnings = append(warnings, err.Error())
		}
	}

	return warnings
}

func runCalendarOpsPreview(
	flags map[string]string,
	operations []icu.CalendarOperation,
	validation *icu.CalendarOpsValidation,
	blocking bool,
	blockingErr error,
	liveCheck func() []string,
) error {
	preview := icu.PreviewCalendarOperations(operations)
	preview.Validation = validation
	if validation != nil {
		preview.Warnings = appendUniquePreviewWarnings(preview.Warnings, validation.Warnings...)
	}
	if !blocking && !BoolFlag(flags, "no-live-check") && liveCheck != nil {
		preview.Warnings = append(preview.Warnings, liveCheck()...)
	}
	if err := writeJSON(preview); err != nil {
		return err
	}
	if blocking {
		return blockingErr
	}

	return nil
}

func calendarValidationFromPlan(validation icu.PlanValidation) *icu.CalendarOpsValidation {
	return &icu.CalendarOpsValidation{
		Feasible: validation.Feasible,
		Blocking: validation.Blocking,
		Warnings: append([]string{}, validation.Warnings...),
		Errors:   append([]string{}, validation.Errors...),
	}
}

func calendarValidationFromRebalance(validation icu.RebalanceValidation) *icu.CalendarOpsValidation {
	return &icu.CalendarOpsValidation{
		Feasible: validation.Feasible,
		Blocking: validation.Blocking,
		Warnings: append([]string{}, validation.Warnings...),
		Errors:   append([]string{}, validation.Errors...),
	}
}

func appendUniquePreviewWarnings(dst []string, values ...string) []string {
	seen := map[string]bool{}
	for _, value := range dst {
		seen[value] = true
	}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		dst = append(dst, value)
	}

	return dst
}

func commonAuthFlags() []CommandFlag {
	return []CommandFlag{
		{Name: "api-key", ValueName: "KEY", Description: "Intervals.icu API key."},
		{Name: "athlete-id", ValueName: "ID", Description: "Intervals.icu athlete ID."},
	}
}
