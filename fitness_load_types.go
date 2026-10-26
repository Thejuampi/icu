package icu

import (
	"fmt"
	"math"
	"sort"
)

// Fitness-load classes of an activity type: whether intervals.icu feeds its
// load into CTL (fitness) as well as ATL (fatigue), or into ATL only.
const (
	FitnessLoadCTL     = "ctl"
	FitnessLoadATLOnly = "atl-only"
)

const (
	fitnessLoadTolerance = 0.5
	// maxSubsetActivities bounds the per-day subset search (2^n masks).
	maxSubsetActivities = 16
)

// FitnessLoadEvidence counts the days that prove a type's class.
type FitnessLoadEvidence struct {
	CTLDays     int `json:"ctlDays"`
	ATLOnlyDays int `json:"atlOnlyDays"`
}

// LearnFitnessLoadTypes finds, for each wellness day, the subsets of that
// day's activities whose loads sum exactly to the day's ctlLoad (all
// activities of one type in or out together). A type is evidenced as CTL or
// ATL-only on a day only when every matching subset agrees; ambiguous days
// and days whose activity loads do not add up to atlLoad prove nothing.
func LearnFitnessLoadTypes(wellness []Wellness, activities []Activity) map[string]FitnessLoadEvidence {
	byDate := map[string][]*Activity{}
	for index := range activities {
		activity := &activities[index]
		if activity.TrainingLoad > 0 && activity.Type != "" {
			date := activityDate(activity)
			byDate[date] = append(byDate[date], activity)
		}
	}
	evidence := map[string]FitnessLoadEvidence{}
	for index := range wellness {
		row := &wellness[index]
		for sport, counts := range dayFitnessClasses(row, byDate[row.ID]) {
			item := evidence[sport]
			if counts {
				item.CTLDays++
			} else {
				item.ATLOnlyDays++
			}
			evidence[sport] = item
		}
	}

	return evidence
}

// dayFitnessClasses returns the types whose class is forced on one day.
func dayFitnessClasses(row *Wellness, day []*Activity) map[string]bool {
	types := []string{}
	loads := map[string]float64{}
	total := 0.0
	for _, activity := range day {
		if _, ok := loads[activity.Type]; !ok {
			types = append(types, activity.Type)
		}
		loads[activity.Type] += float64(activity.TrainingLoad)
		total += float64(activity.TrainingLoad)
	}
	if len(types) == 0 || len(types) > maxSubsetActivities ||
		math.Abs(total-row.ATLLoad) > fitnessLoadTolerance {
		return nil
	}
	sort.Strings(types)
	always, never := (1<<len(types))-1, (1<<len(types))-1
	matches := 0
	for mask := range 1 << len(types) {
		sum := 0.0
		for bit, sport := range types {
			if mask&(1<<bit) != 0 {
				sum += loads[sport]
			}
		}
		if math.Abs(sum-row.CTLLoad) <= fitnessLoadTolerance {
			matches++
			always &= mask
			never &^= mask
		}
	}
	if matches == 0 {
		return nil
	}
	out := map[string]bool{}
	for bit, sport := range types {
		switch {
		case always&(1<<bit) != 0:
			out[sport] = true
		case never&(1<<bit) != 0:
			out[sport] = false
		}
	}

	return out
}

// ClassifyFitnessLoadTypes turns evidence into classes; a type with evidence
// both ways is left out and reported.
func ClassifyFitnessLoadTypes(evidence map[string]FitnessLoadEvidence) (map[string]string, []string) {
	classes := map[string]string{}
	var conflicts []string
	for sport, item := range evidence {
		switch {
		case item.CTLDays > 0 && item.ATLOnlyDays > 0:
			conflicts = append(conflicts, sport)
		case item.CTLDays > 0:
			classes[sport] = FitnessLoadCTL
		case item.ATLOnlyDays > 0:
			classes[sport] = FitnessLoadATLOnly
		}
	}
	sort.Strings(conflicts)

	return classes, conflicts
}

// ResolveFitnessLoadTypes merges classes by precedence: manual overrides,
// then classes learned from this window, then classes persisted from earlier
// runs. It returns the ATL-only set, plus a warning for every type in
// needed with no class (it is assumed to count toward CTL, the intervals.icu
// default) and for every conflicting type.
func ResolveFitnessLoadTypes(
	manual, learned, persisted map[string]string, conflicts, needed []string,
) (map[string]bool, []string) {
	resolved := map[string]string{}
	for _, layer := range []map[string]string{persisted, learned, manual} {
		for sport, class := range layer {
			resolved[sport] = class
		}
	}
	excluded := map[string]bool{}
	for sport, class := range resolved {
		if class == FitnessLoadATLOnly {
			excluded[sport] = true
		}
	}
	var warnings []string
	for _, sport := range conflicts {
		if _, ok := manual[sport]; !ok {
			warnings = append(warnings, fmt.Sprintf(
				"fitness load: %s counted toward CTL on some days and not on others; set it with "+
					"profile set --fitness-load-types", sport,
			))
		}
	}
	seen := map[string]bool{}
	for _, sport := range needed {
		if sport == "" || seen[sport] {
			continue
		}
		seen[sport] = true
		if _, ok := resolved[sport]; !ok {
			warnings = append(warnings, fmt.Sprintf(
				"fitness load: no history shows whether %s counts toward CTL; assumed it does", sport,
			))
		}
	}
	sort.Strings(warnings)

	return excluded, warnings
}

// ValidFitnessLoadClass reports whether class is a known fitness-load class.
func ValidFitnessLoadClass(class string) bool {
	return class == FitnessLoadCTL || class == FitnessLoadATLOnly
}
