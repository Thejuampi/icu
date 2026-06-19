package icu

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	plannedStepKindWarmup   = "warmup"
	plannedStepKindWork     = "work"
	plannedStepKindRecovery = "recovery"
	plannedStepKindCooldown = "cooldown"
	plannedStepKindOther    = "other"
	workoutWorkTargetMin    = 80
)

var ErrWorkoutDocMissing = errors.New("workout doc missing")

type WorkoutDoc struct {
	Description     string        `json:"description,omitempty"`
	Duration        int           `json:"duration,omitempty"`
	AverageWatts    int           `json:"average_watts,omitempty"`
	NormalizedPower int           `json:"normalized_power,omitempty"`
	Steps           []WorkoutStep `json:"steps,omitempty"`
}

type WorkoutStep struct {
	Duration int            `json:"duration,omitempty"`
	Distance float64        `json:"distance,omitempty"`
	Reps     int            `json:"reps,omitempty"`
	Text     string         `json:"text,omitempty"`
	Ramp     bool           `json:"ramp,omitempty"`
	Freeride bool           `json:"freeride,omitempty"`
	Power    *WorkoutTarget `json:"power,omitempty"`
	HR       *WorkoutTarget `json:"hr,omitempty"`
	Pace     *WorkoutTarget `json:"pace,omitempty"`
	Cadence  *WorkoutTarget `json:"cadence,omitempty"`
	Steps    []WorkoutStep  `json:"steps,omitempty"`
}

type WorkoutTarget struct {
	Start float64 `json:"start,omitempty"`
	End   float64 `json:"end,omitempty"`
	Value float64 `json:"value,omitempty"`
	Units string  `json:"units,omitempty"`
}

type PlannedWorkoutStep struct {
	Index           int            `json:"index"`
	ParentIndex     int            `json:"parentIndex,omitempty"`
	RepeatIndex     int            `json:"repeatIndex,omitempty"`
	Kind            string         `json:"kind"`
	Text            string         `json:"text,omitempty"`
	DurationSeconds int            `json:"durationSeconds,omitempty"`
	StartOffset     int            `json:"startOffsetSeconds,omitempty"`
	EndOffset       int            `json:"endOffsetSeconds,omitempty"`
	Ramp            bool           `json:"ramp,omitempty"`
	Freeride        bool           `json:"freeride,omitempty"`
	Power           *WorkoutTarget `json:"power,omitempty"`
	HR              *WorkoutTarget `json:"hr,omitempty"`
	Pace            *WorkoutTarget `json:"pace,omitempty"`
	Cadence         *WorkoutTarget `json:"cadence,omitempty"`
}

func DecodeWorkoutDoc(raw any) (*WorkoutDoc, error) {
	if raw == nil {
		return nil, ErrWorkoutDocMissing
	}

	if doc, ok := raw.(*WorkoutDoc); ok {
		return doc, nil
	}
	if doc, ok := raw.(WorkoutDoc); ok {
		return &doc, nil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal workout doc: %w", err)
	}

	var doc WorkoutDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal workout doc: %w", err)
	}

	return &doc, nil
}

func DecodeWorkoutFileBase64(value, filename string) (*WorkoutDoc, error) {
	if value == "" {
		return nil, ErrWorkoutDocMissing
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode workout file base64: %w", err)
	}

	if strings.HasSuffix(strings.ToLower(filename), ".json") {
		var doc WorkoutDoc
		if err := json.Unmarshal(decoded, &doc); err != nil {
			return nil, fmt.Errorf("unmarshal workout json file: %w", err)
		}
		return &doc, nil
	}

	return nil, fmt.Errorf("unsupported workout file format %q", filename)
}

func ExpandWorkoutSteps(doc *WorkoutDoc) []PlannedWorkoutStep {
	if doc == nil {
		return nil
	}

	var result []PlannedWorkoutStep
	var offset int
	expandWorkoutSteps(doc.Steps, &result, &offset, 0, 0)
	return result
}

func expandWorkoutSteps(steps []WorkoutStep, result *[]PlannedWorkoutStep, offset *int, parentIndex, repeatIndex int) {
	for i := range steps {
		step := steps[i]
		reps := step.Reps
		if reps <= 0 {
			reps = 1
		}

		if len(step.Steps) > 0 {
			for rep := 1; rep <= reps; rep++ {
				expandWorkoutSteps(step.Steps, result, offset, i+1, rep)
			}
			continue
		}

		for rep := 1; rep <= reps; rep++ {
			planned := PlannedWorkoutStep{
				Index:           len(*result) + 1,
				ParentIndex:     parentIndex,
				RepeatIndex:     repeatIndex,
				Kind:            classifyWorkoutStep(&step),
				Text:            step.Text,
				DurationSeconds: step.Duration,
				StartOffset:     *offset,
				EndOffset:       *offset + step.Duration,
				Ramp:            step.Ramp,
				Freeride:        step.Freeride,
				Power:           step.Power,
				HR:              step.HR,
				Pace:            step.Pace,
				Cadence:         step.Cadence,
			}
			*result = append(*result, planned)
			*offset += step.Duration
		}
	}
}

func classifyWorkoutStep(step *WorkoutStep) string {
	text := strings.ToLower(step.Text)
	if strings.Contains(text, "warm") {
		return plannedStepKindWarmup
	}
	if strings.Contains(text, "cool") {
		return plannedStepKindCooldown
	}
	if step.Power == nil {
		return plannedStepKindOther
	}

	target := targetReference(step.Power)
	switch {
	case target >= workoutWorkTargetMin:
		return plannedStepKindWork
	case target > 0:
		return plannedStepKindRecovery
	default:
		return plannedStepKindOther
	}
}

func targetReference(target *WorkoutTarget) float64 {
	if target == nil {
		return 0
	}
	if target.Value > 0 {
		return target.Value
	}
	if target.Start > 0 && target.End > 0 {
		return (target.Start + target.End) / 2
	}
	if target.End > 0 {
		return target.End
	}
	return target.Start
}
