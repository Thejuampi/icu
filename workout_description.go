package icu

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var ErrWorkoutDescriptionUnsupported = errors.New("workout description unsupported")

var (
	repeatLinePattern = regexp.MustCompile(`^(\d+)x$`)
	stepLinePattern   = regexp.MustCompile(`^-\s*(\d+(?:\.\d+)?)([hms])\s+(\d+(?:\.\d+)?)(?:\s*-\s*(\d+(?:\.\d+)?))?%\s*(.*)$`)
)

func ParseWorkoutDescription(description string) (*WorkoutDoc, error) {
	lines := strings.Split(description, "\n")
	var doc WorkoutDoc

	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}

		if matches := repeatLinePattern.FindStringSubmatch(line); matches != nil {
			reps, err := strconv.Atoi(matches[1])
			if err != nil || reps <= 0 {
				return nil, fmt.Errorf("%w: invalid repeat %q", ErrWorkoutDescriptionUnsupported, line)
			}

			block, nextIndex, err := parseRepeatBlock(lines, index+1, reps)
			if err != nil {
				return nil, err
			}
			doc.Steps = append(doc.Steps, block)
			index = nextIndex - 1
			continue
		}

		step, err := parseWorkoutDescriptionStep(line)
		if err != nil {
			return nil, err
		}
		doc.Steps = append(doc.Steps, step)
	}

	if len(doc.Steps) == 0 {
		return nil, fmt.Errorf("%w: no workout steps", ErrWorkoutDescriptionUnsupported)
	}

	doc.Description = description
	doc.Duration = workoutDocDuration(doc.Steps)

	return &doc, nil
}

func parseRepeatBlock(lines []string, startIndex, reps int) (WorkoutStep, int, error) {
	block := WorkoutStep{Reps: reps}
	index := startIndex
	for index < len(lines) {
		raw := lines[index]
		line := strings.TrimSpace(raw)
		if line == "" {
			index++
			continue
		}

		if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") {
			break
		}

		if repeatLinePattern.MatchString(line) {
			return block, index, fmt.Errorf("%w: nested repeats are not supported", ErrWorkoutDescriptionUnsupported)
		}

		step, err := parseWorkoutDescriptionStep(line)
		if err != nil {
			return block, index, err
		}
		block.Steps = append(block.Steps, step)
		index++
	}

	if len(block.Steps) == 0 {
		return block, index, fmt.Errorf("%w: repeat block has no steps", ErrWorkoutDescriptionUnsupported)
	}

	return block, index, nil
}

func parseWorkoutDescriptionStep(line string) (WorkoutStep, error) {
	matches := stepLinePattern.FindStringSubmatch(line)
	if matches == nil {
		return WorkoutStep{}, fmt.Errorf("%w: %q", ErrWorkoutDescriptionUnsupported, line)
	}

	duration, err := parseWorkoutDuration(matches[1], matches[2])
	if err != nil {
		return WorkoutStep{}, err
	}

	start, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return WorkoutStep{}, fmt.Errorf("%w: invalid power target %q", ErrWorkoutDescriptionUnsupported, line)
	}

	step := WorkoutStep{
		Duration: duration,
		Text:     strings.TrimSpace(matches[5]),
		Power:    &WorkoutTarget{Units: "%ftp"},
	}

	if matches[4] != "" {
		end, parseErr := strconv.ParseFloat(matches[4], 64)
		if parseErr != nil {
			return WorkoutStep{}, fmt.Errorf("%w: invalid power target %q", ErrWorkoutDescriptionUnsupported, line)
		}
		step.Ramp = true
		step.Power.Start = start
		step.Power.End = end
		return step, nil
	}

	step.Power.Value = start

	return step, nil
}

func parseWorkoutDuration(value, unit string) (int, error) {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("%w: invalid duration %q%s", ErrWorkoutDescriptionUnsupported, value, unit)
	}

	switch unit {
	case "h":
		return int(math.Round(number * float64(secondsPerHour))), nil
	case "m":
		return int(math.Round(number * float64(secondsPerMinute))), nil
	case "s":
		return int(math.Round(number)), nil
	default:
		return 0, fmt.Errorf("%w: invalid duration unit %q", ErrWorkoutDescriptionUnsupported, unit)
	}
}

func workoutDocDuration(steps []WorkoutStep) int {
	var total int
	for index := range steps {
		step := steps[index]
		reps := step.Reps
		if reps <= 0 {
			reps = 1
		}
		if len(step.Steps) > 0 {
			total += reps * workoutDocDuration(step.Steps)
			continue
		}
		total += reps * step.Duration
	}

	return total
}
