package icu

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FormatWorkoutDescription serializes a WorkoutDoc back into the text
// description format consumed by ParseWorkoutDescription.
func FormatWorkoutDescription(doc *WorkoutDoc) string {
	if doc == nil || len(doc.Steps) == 0 {
		return ""
	}
	var lines []string
	for i := range doc.Steps {
		formatWorkoutStep(&lines, &doc.Steps[i], "")
	}
	return strings.Join(lines, "\n")
}

func formatWorkoutStep(lines *[]string, step *WorkoutStep, indent string) {
	if step.Reps > 0 && len(step.Steps) > 0 {
		*lines = append(*lines, strconv.Itoa(step.Reps)+"x")
		for i := range step.Steps {
			formatWorkoutStep(lines, &step.Steps[i], "  ")
		}
		return
	}
	dur := formatWorkoutDuration(step.Duration)
	if step.Ramp && step.Power != nil {
		*lines = append(*lines, fmt.Sprintf("%s- %s %.0f-%.0f%% %s", indent, dur, step.Power.Start, step.Power.End, step.Text))
		return
	}
	if step.Power != nil {
		*lines = append(*lines, fmt.Sprintf("%s- %s %.0f%% %s", indent, dur, step.Power.Value, step.Text))
		return
	}
	*lines = append(*lines, fmt.Sprintf("%s- %s %s", indent, dur, step.Text))
}

func formatWorkoutDuration(seconds int) string {
	if seconds <= 0 {
		return "0s"
	}
	minutes := seconds / 60
	rem := seconds % 60
	if rem == 0 {
		return strconv.Itoa(minutes) + "m"
	}
	if minutes == 0 {
		return strconv.Itoa(seconds) + "s"
	}
	return strconv.Itoa(minutes) + "m" + strconv.Itoa(rem) + "s"
}

// rescaleWorkoutDocPower scales all power targets in a WorkoutDoc by the given
// ratio. Only steps with %ftp unit power targets are scaled. The original
// WorkoutDoc is not modified; a shallow copy with adjusted steps is returned.
func rescaleWorkoutDocPower(doc *WorkoutDoc, ratio float64) *WorkoutDoc {
	if doc == nil || math.Abs(ratio-1.0) < 0.0001 {
		return doc
	}
	scaled := *doc
	scaled.Steps = make([]WorkoutStep, len(doc.Steps))
	for i := range doc.Steps {
		scaled.Steps[i] = rescaleWorkoutStepPower(doc.Steps[i], ratio)
	}
	return &scaled
}

func rescaleWorkoutStepPower(step WorkoutStep, ratio float64) WorkoutStep {
	if len(step.Steps) > 0 {
		scaled := step
		scaled.Steps = make([]WorkoutStep, len(step.Steps))
		for i := range step.Steps {
			scaled.Steps[i] = rescaleWorkoutStepPower(step.Steps[i], ratio)
		}
		return scaled
	}
	if step.Power == nil || !strings.EqualFold(step.Power.Units, "%ftp") {
		return step
	}
	scaled := step
	pow := *step.Power
	if pow.Value > 0 {
		pow.Value = math.Round(pow.Value*ratio*10) / 10
	}
	if pow.Start > 0 {
		pow.Start = math.Round(pow.Start*ratio*10) / 10
	}
	if pow.End > 0 {
		pow.End = math.Round(pow.End*ratio*10) / 10
	}
	scaled.Power = &pow
	return scaled
}

// RebalanceScaleWorkoutDescription takes an event, extracts or parses its
// workout structure, scales power targets by the ratio of targetIF to
// originalIF, and returns the reformatted description. Returns empty string
// when the event's workout structure cannot be determined.
func RebalanceScaleWorkoutDescription(event *Event, originalIF, targetIF float64) string {
	if event == nil || originalIF <= 0 || targetIF <= 0 {
		return ""
	}
	ratio := targetIF / originalIF
	if math.Abs(ratio-1.0) < 0.0001 {
		return event.Description
	}
	var doc *WorkoutDoc
	if event.WorkoutDoc != nil {
		decoded, err := DecodeWorkoutDoc(event.WorkoutDoc)
		if err == nil && decoded != nil && len(decoded.Steps) > 0 {
			doc = decoded
		}
	}
	if doc == nil && event.Description != "" {
		parsed, err := ParseWorkoutDescription(event.Description)
		if err == nil && parsed != nil && len(parsed.Steps) > 0 {
			doc = parsed
		}
	}
	if doc == nil {
		return event.Description
	}
	scaled := rescaleWorkoutDocPower(doc, ratio)
	return FormatWorkoutDescription(scaled)
}
