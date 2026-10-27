package icu

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// DefaultMaxSimilarSessionsPerWeek is used when the profile leaves
// MaxSimilarSessionsPerWeek unset: up to two sessions with the same structure
// in one week are allowed; the third is reported. It is a definition of "repeated", not a
// physiological limit, and is overridable in the profile.
const DefaultMaxSimilarSessionsPerWeek = 2

// PlanMonotonyWarnings warns when more than maxSimilar non-cancelled
// sessions in one ISO week share the same workout structure. Structure is
// the parsed step sequence (durations, targets, repeats); descriptions the
// parser does not support fall back to their whitespace-normalised text, and
// sessions without a description are ignored.
func PlanMonotonyWarnings(sessions []PlanResolvedSession, maxSimilar int) []string {
	if maxSimilar <= 0 {
		maxSimilar = DefaultMaxSimilarSessionsPerWeek
	}
	type group struct {
		week  string
		names []string
		dates []string
	}
	groups := map[string]*group{}
	var keys []string
	for index := range sessions {
		session := &sessions[index]
		if session.Action == CalendarActionCancel {
			continue
		}
		signature := WorkoutStructureSignature(session.Desc)
		if signature == "" {
			continue
		}
		week := planWeekStart(session.Date)
		key := week + "|" + signature
		if groups[key] == nil {
			groups[key] = &group{week: week}
			keys = append(keys, key)
		}
		groups[key].names = append(groups[key].names, session.Name)
		groups[key].dates = append(groups[key].dates, session.Date)
	}
	sort.Strings(keys)
	var warnings []string
	for _, key := range keys {
		entry := groups[key]
		if len(entry.names) <= maxSimilar {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"monotony: week %s has %d sessions with the same structure (%s on %s); limit %d",
			entry.week, len(entry.names), strings.Join(entry.names, ", "), strings.Join(entry.dates, ", "), maxSimilar,
		))
	}

	return warnings
}

// WorkoutStructureSignature returns a canonical form of a workout
// description's structure, or "" for an empty description.
func WorkoutStructureSignature(desc string) string {
	if strings.TrimSpace(desc) == "" {
		return ""
	}
	doc, err := ParseWorkoutDescription(desc)
	if err != nil || len(doc.Steps) == 0 {
		return "text:" + strings.Join(strings.Fields(strings.ToLower(desc)), " ")
	}

	return "steps:" + stepsSignature(doc.Steps)
}

func stepsSignature(steps []WorkoutStep) string {
	parts := make([]string, 0, len(steps))
	for index := range steps {
		step := &steps[index]
		if len(step.Steps) > 0 {
			parts = append(parts, strconv.Itoa(step.Reps)+"x("+stepsSignature(step.Steps)+")")

			continue
		}
		target := ""
		if step.Power != nil {
			if step.Ramp {
				target = formatSignatureNumber(step.Power.Start) + "-" + formatSignatureNumber(step.Power.End)
			} else {
				target = formatSignatureNumber(step.Power.Value)
			}
		}
		parts = append(parts, strconv.Itoa(step.Duration)+"s@"+target)
	}

	return strings.Join(parts, ";")
}

func formatSignatureNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
