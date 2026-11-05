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

const stepLinePrefix = "-"

var (
	repeatLinePattern = regexp.MustCompile(`^(\d+)x$`)
	// Intervals.icu writes steps as "- 12m Ramp 50-58% FTP": an optional
	// keyword sits between the duration and the target, and the unit may be
	// repeated after the percent sign. Durations may be compound ("1h30m",
	// "2m30s"), as intervals.icu accepts them.
	durationPartPattern = regexp.MustCompile(`(\d+(?:\.\d+)?)([hms])`)
	//
	// The target is a percent of FTP ("70%", ramp "55-75%"), absolute watts
	// ("200w", range "110-130w" priced at its midpoint) or a power zone
	// ("Z1".."Z7", priced at a fixed %FTP, see powerZoneMidpointPercent).
	stepLinePattern = regexp.MustCompile(
		`^-\s*((?:\d+(?:\.\d+)?[hms])+)\s+(?:([A-Za-z]+)\s+)?` +
			`(?:(\d+(?:\.\d+)?)(?:\s*-\s*(\d+(?:\.\d+)?))?(%|\s*[wW]\b)|[zZ]([1-7])\b)\s*(.*)$`,
	)
)

// supportedStepForms is quoted in parse errors so the fix is obvious.
const supportedStepForms = `supported step forms: "- 10m 70%", "- 10m 55-75%" (ramp), "- 10m 200w", ` +
	`"- 10m 110-130w" (midpoint), "- 10m Z2" (Z1-Z7); durations like 30s, 10m, 1h30m; repeats as "3x" before the steps`

// powerZoneMidpointPercent prices "Z1".."Z7" as %FTP. Bands follow the
// intervals.icu default (Coggan) power zones, upper bounds 55/75/90/105/120/150%
// FTP, and each zone is the midpoint of its band. Z1 (0-55%) uses 50% and the
// open-ended Z7 (>150%) uses 160%, since a 0% floor or an infinite cap has no
// meaningful midpoint.
func powerZoneMidpointPercent(zone int) float64 {
	//nolint:mnd // the documented zone table itself
	return [...]float64{1: 50, 2: 65, 3: 82.5, 4: 97.5, 5: 112.5, 6: 135, 7: 160}[zone]
}

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
				return nil, lineError(nextIndex, err)
			}
			doc.Steps = append(doc.Steps, block)
			index = nextIndex - 1
			continue
		}

		// Intervals.icu lets a description open with free-text coaching notes
		// before the structure. Only lines that claim to be steps must parse,
		// so a typo in a step line is still an error rather than silent prose.
		if !strings.HasPrefix(line, stepLinePrefix) {
			continue
		}

		step, err := parseWorkoutDescriptionStep(line)
		if err != nil {
			return nil, lineError(index, err)
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

// parseRepeatBlock collects the steps that follow an "Nx" header. Two
// layouts are accepted: the indented form, where every child line starts with
// whitespace, and the intervals.icu native form, where un-indented "-" steps
// follow the header and a blank line (or any non-step line) closes the block.
func parseRepeatBlock(lines []string, startIndex, reps int) (WorkoutStep, int, error) {
	block := WorkoutStep{Reps: reps}
	index := startIndex
	indentedBlock := nextNonBlankIsIndented(lines, startIndex)
	for index < len(lines) {
		raw := lines[index]
		line := strings.TrimSpace(raw)
		if line == "" {
			if len(block.Steps) > 0 && !indentedBlock {
				break
			}
			index++
			continue
		}

		indented := strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t")
		if indented != indentedBlock {
			break
		}
		if !indented && !strings.HasPrefix(line, stepLinePrefix) {
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

// lineError names the 1-based description line and the supported forms.
func lineError(index int, err error) error {
	return fmt.Errorf("line %d: %w; %s", index+1, err, supportedStepForms)
}

func nextNonBlankIsIndented(lines []string, index int) bool {
	for _, raw := range lines[index:] {
		if strings.TrimSpace(raw) == "" {
			continue
		}

		return strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t")
	}

	return false
}

func parseWorkoutDescriptionStep(line string) (WorkoutStep, error) {
	matches := stepLinePattern.FindStringSubmatch(line)
	if matches == nil {
		return WorkoutStep{}, fmt.Errorf("%w: %q", ErrWorkoutDescriptionUnsupported, line)
	}

	duration, err := parseWorkoutDuration(matches[1])
	if err != nil {
		return WorkoutStep{}, err
	}

	step := WorkoutStep{
		Duration: duration,
		Text:     workoutStepText(matches[2], matches[7]),
		Power:    &WorkoutTarget{Units: "%ftp"},
	}

	if matches[6] != "" {
		zone, _ := strconv.Atoi(matches[6]) // the pattern only admits 1-7
		step.Power.Value = powerZoneMidpointPercent(zone)

		return step, nil
	}

	start, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return WorkoutStep{}, fmt.Errorf("%w: invalid power target %q", ErrWorkoutDescriptionUnsupported, line)
	}
	end := start
	if matches[4] != "" {
		if end, err = strconv.ParseFloat(matches[4], 64); err != nil {
			return WorkoutStep{}, fmt.Errorf("%w: invalid power target %q", ErrWorkoutDescriptionUnsupported, line)
		}
	}

	if matches[5] != "%" {
		// Absolute watts; a range is priced at its midpoint, not as a ramp.
		step.Power.Units = "w"
		step.Power.Value = (start + end) / 2

		return step, nil
	}

	if matches[4] != "" {
		step.Ramp = true
		step.Power.Start = start
		step.Power.End = end

		return step, nil
	}

	step.Power.Value = start

	return step, nil
}

// workoutStepText joins the keyword written before the target with any text
// written after it, dropping a repeated power unit such as the "FTP" in
// "- 12m Ramp 50-58% FTP" so it does not leak into the step label.
func workoutStepText(keyword, trailing string) string {
	parts := make([]string, 0, 2)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		parts = append(parts, keyword)
	}
	if trailing = strings.TrimSpace(trailing); trailing != "" && !strings.EqualFold(trailing, "ftp") {
		parts = append(parts, trailing)
	}

	return strings.Join(parts, " ")
}

// parseWorkoutDuration sums a duration token made of one or more
// number+unit parts ("10m", "1h30m", "2m30s"). Each unit may appear once and
// units must run from larger to smaller, matching intervals.icu.
func parseWorkoutDuration(token string) (int, error) {
	parts := durationPartPattern.FindAllStringSubmatch(token, -1)
	if len(parts) == 0 {
		return 0, fmt.Errorf("%w: invalid duration %q", ErrWorkoutDescriptionUnsupported, token)
	}

	total := 0
	lastRank := 0
	for _, part := range parts {
		rank, seconds, err := workoutDurationPart(part[1], part[2])
		if err != nil {
			return 0, fmt.Errorf("%w (in %q)", err, token)
		}
		if rank <= lastRank {
			return 0, fmt.Errorf("%w: invalid duration order %q", ErrWorkoutDescriptionUnsupported, token)
		}
		lastRank = rank
		total += seconds
	}
	if total <= 0 {
		return 0, fmt.Errorf("%w: invalid duration %q", ErrWorkoutDescriptionUnsupported, token)
	}

	return total, nil
}

func workoutDurationPart(value, unit string) (int, int, error) {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 {
		return 0, 0, fmt.Errorf("%w: invalid duration %q%s", ErrWorkoutDescriptionUnsupported, value, unit)
	}

	switch unit {
	case "h":
		return 1, int(math.Round(number * float64(secondsPerHour))), nil
	case "m":
		return 2, int(math.Round(number * float64(secondsPerMinute))), nil
	default:
		return 3, int(math.Round(number)), nil
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
