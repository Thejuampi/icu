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
			`(?:(\d+(?:\.\d+)?)(?:\s*-\s*(\d+(?:\.\d+)?))?(%|\s*[wW]\b)|[zZ](\d+)(?:\s*-\s*[zZ]?(\d+))?\b)\s*(.*)$`,
	)
	// nonPowerTargetPattern: HR, LTHR or Pace written after (or as the keyword
	// before) a target means the step is not a power target.
	nonPowerTargetPattern = regexp.MustCompile(`(?i)^(hr|lthr|pace)\b`)
	// cadenceTextPattern strips cadence hints ("85rpm", "90-100 rpm",
	// "cadence 90") from the step label.
	cadenceTextPattern = regexp.MustCompile(`(?i)\b(?:\d+(?:-\d+)?\s*rpm|cadence\s*\d+(?:-\d+)?)\b`)
)

// MaxPercentFTPTarget caps a %FTP step target (and, via MaxWattsTarget, a
// watt target at that share of FTP): above 300% FTP a step is a typo, not a
// workout, and would inflate the load.
const MaxPercentFTPTarget = 300

// supportedStepForms is quoted in parse errors so the fix is obvious.
const supportedStepForms = `supported step forms (power only): "- 10m 70%", "- 10m 65-75%" (range, midpoint), ` +
	`"- 10m ramp 55-75%" (ramp), "- 10m 200w", "- 10m 110-130w" (midpoint), "- 10m ramp 100-200w", "- 10m Z2" ` +
	`(sport-settings zones); targets above 0 and at most 300% FTP; durations like 30s, 10m, 1h30m; repeats as "3x" before the steps`

// Power zone pricing. "Zn" is priced at the midpoint of its band in the
// athlete's Ride sport-settings power_zones (%FTP upper bounds, the last one
// open-ended, intervals.icu writes 999). Z1 (0..first bound) is fixed at 50%
// and the open-ended top zone at 160%, since a 0% floor or an infinite cap has
// no meaningful midpoint. DefaultPowerZoneBounds (intervals.icu's default
// Coggan bands) is used only when sport-settings cannot supply zones.
const (
	powerZoneFirstPercent = 50
	powerZoneTopPercent   = 160
)

// DefaultPowerZoneBounds are intervals.icu's default Ride power zones.
func DefaultPowerZoneBounds() []int { return []int{55, 75, 90, 105, 120, 150, 999} }

// PowerZoneMidpoints returns the %FTP price of Z1..Zn for the given upper
// bounds: Z1 50%, the top zone 160%, every other zone its band midpoint.
func PowerZoneMidpoints(bounds []int) []float64 {
	midpoints := make([]float64, len(bounds))
	for index := range bounds {
		switch {
		case index == 0:
			midpoints[index] = powerZoneFirstPercent
		case index == len(bounds)-1:
			midpoints[index] = powerZoneTopPercent
		default:
			midpoints[index] = float64(bounds[index-1]+1+bounds[index]) / 2 //nolint:mnd // band (prev+1 .. upper) midpoint
		}
	}

	return midpoints
}

var zoneStepPattern = regexp.MustCompile(`^-\s*(?:\d+(?:\.\d+)?[hms])+\s+(?:[A-Za-z]+\s+)?[zZ]\d+\b`)

// WorkoutDescriptionUsesPowerZones reports whether any step targets "Zn", so
// callers fetch sport-settings zones only when they matter.
func WorkoutDescriptionUsesPowerZones(description string) bool {
	for _, line := range strings.Split(description, "\n") {
		if zoneStepPattern.MatchString(strings.TrimSpace(line)) {
			return true
		}
	}

	return false
}

// ParseWorkoutDescription parses with the default power zones.
func ParseWorkoutDescription(description string) (*WorkoutDoc, error) {
	return ParseWorkoutDescriptionWithZones(description, PowerZoneMidpoints(DefaultPowerZoneBounds()))
}

// ParseWorkoutDescriptionWithZones prices "Zn" steps with zones[n-1] (%FTP),
// see PowerZoneMidpoints.
func ParseWorkoutDescriptionWithZones(description string, zones []float64) (*WorkoutDoc, error) {
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

			block, nextIndex, err := parseRepeatBlock(lines, index+1, reps, zones)
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

		step, err := parseWorkoutDescriptionStep(line, zones)
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
func parseRepeatBlock(lines []string, startIndex, reps int, zones []float64) (WorkoutStep, int, error) {
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

		step, err := parseWorkoutDescriptionStep(line, zones)
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

func parseWorkoutDescriptionStep(line string, zones []float64) (WorkoutStep, error) {
	matches := stepLinePattern.FindStringSubmatch(line)
	if matches == nil {
		return WorkoutStep{}, fmt.Errorf("%w: %q", ErrWorkoutDescriptionUnsupported, line)
	}
	keyword, trailing := matches[2], strings.TrimSpace(matches[8])
	if nonPowerTargetPattern.MatchString(keyword) || nonPowerTargetPattern.MatchString(trailing) {
		return WorkoutStep{}, fmt.Errorf("%w: %q targets HR, LTHR or Pace, not power; only power steps can be priced",
			ErrWorkoutDescriptionUnsupported, line)
	}

	duration, err := parseWorkoutDuration(matches[1])
	if err != nil {
		return WorkoutStep{}, err
	}

	// Only the "ramp" keyword makes a ramp; a range without it is priced at
	// its midpoint (%FTP or watts alike).
	ramp := strings.EqualFold(keyword, "ramp")
	if ramp {
		keyword = ""
	}
	step := WorkoutStep{
		Duration: duration,
		Text:     workoutStepText(keyword, trailing),
		Power:    &WorkoutTarget{Units: "%ftp"},
	}

	if matches[6] != "" {
		return zoneStep(step, line, matches[6], matches[7], ramp, zones)
	}

	start, end, err := stepTargetRange(line, matches[3], matches[4])
	if err != nil {
		return WorkoutStep{}, err
	}
	if matches[5] != "%" {
		step.Power.Units = "w"
	} else if end > MaxPercentFTPTarget {
		return WorkoutStep{}, fmt.Errorf("%w: target above %d%% FTP in %q", ErrWorkoutDescriptionUnsupported, MaxPercentFTPTarget, line)
	}

	switch {
	case ramp && matches[4] == "":
		return WorkoutStep{}, fmt.Errorf("%w: a ramp needs a range like \"ramp 55-75%%\" in %q", ErrWorkoutDescriptionUnsupported, line)
	case ramp:
		step.Ramp = true
		step.Power.Start, step.Power.End = start, end
	default:
		step.Power.Value = (start + end) / 2 //nolint:mnd // range midpoint (start == end for a single value)
	}

	return step, nil
}

func zoneStep(step WorkoutStep, line, first, last string, ramp bool, zones []float64) (WorkoutStep, error) {
	if last != "" {
		return WorkoutStep{}, fmt.Errorf("%w: zone ranges like \"Z3-Z4\" are not supported in %q; use a %% or watt range",
			ErrWorkoutDescriptionUnsupported, line)
	}
	if ramp {
		return WorkoutStep{}, fmt.Errorf("%w: a ramp needs a %% or watt range, not a zone, in %q", ErrWorkoutDescriptionUnsupported, line)
	}
	zone, err := strconv.Atoi(first)
	if err != nil || zone < 1 || zone > len(zones) {
		return WorkoutStep{}, fmt.Errorf("%w: zone Z%s does not exist (power zones define Z1-Z%d) in %q",
			ErrWorkoutDescriptionUnsupported, first, len(zones), line)
	}
	step.Power.Value = zones[zone-1]

	return step, nil
}

func stepTargetRange(line, first, second string) (float64, float64, error) {
	start, err := strconv.ParseFloat(first, 64)
	end := start
	if err == nil && second != "" {
		end, err = strconv.ParseFloat(second, 64)
	}
	if err != nil || start <= 0 || end <= 0 {
		return 0, 0, fmt.Errorf("%w: power target must be above 0 in %q", ErrWorkoutDescriptionUnsupported, line)
	}

	return start, end, nil
}

// ValidateWorkoutTargets rejects watt targets above MaxPercentFTPTarget % of
// ftp (percent targets are checked while parsing).
func ValidateWorkoutTargets(steps []WorkoutStep, ftp int) error {
	limit := float64(ftp) * MaxPercentFTPTarget / percentScale
	for index := range steps {
		step := &steps[index]
		if err := ValidateWorkoutTargets(step.Steps, ftp); err != nil {
			return err
		}
		if step.Power == nil || step.Power.Units != "w" || ftp <= 0 {
			continue
		}
		if peak := math.Max(step.Power.Value, math.Max(step.Power.Start, step.Power.End)); peak > limit {
			return fmt.Errorf("%w: %.0fw is above %d%% of FTP %d (%.0fw)",
				ErrWorkoutDescriptionUnsupported, peak, MaxPercentFTPTarget, ftp, limit)
		}
	}

	return nil
}

// workoutStepText joins the keyword written before the target with any text
// written after it, dropping a repeated power unit such as the "FTP" in
// "- 12m Ramp 50-58% FTP" so it does not leak into the step label.
func workoutStepText(keyword, trailing string) string {
	parts := make([]string, 0, 2)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		parts = append(parts, keyword)
	}
	trailing = strings.Join(strings.Fields(cadenceTextPattern.ReplaceAllString(trailing, "")), " ")
	if trailing != "" && !strings.EqualFold(trailing, "ftp") {
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
