package icu

import (
	"fmt"
	"math"
	"time"
)

const daysPerWeek = 7

// LoadProjection is the projected CTL/ATL/TSB through the end of a plan,
// starting from the latest daily wellness values (not the last activity).
type LoadProjection struct {
	StartDate      string               `json:"startDate"`
	StartSource    string               `json:"startSource"`
	StartCTL       float64              `json:"startCtl"`
	StartATL       float64              `json:"startAtl"`
	StartTSB       float64              `json:"startTsb"`
	EndDate        string               `json:"endDate"`
	EndCTL         float64              `json:"endCtl"`
	EndATL         float64              `json:"endAtl"`
	EndTSB         float64              `json:"endTsb"`
	CTLDays        int                  `json:"ctlDays"`
	ATLDays        int                  `json:"atlDays"`
	MaxRampPerWeek float64              `json:"maxRampPerWeek,omitempty"`
	Weeks          []LoadProjectionWeek `json:"weeks,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
}

// LoadProjectionWeek is one ISO week of the projection. Ramp is the raw CTL
// change across the projected days of the week. RampPerWeek is the largest
// rolling 7-day ramp, CTL[d] - CTL[d-7], ending on a day of this week, so a
// partial week is never extrapolated. Days before the projection start use
// the athlete's real wellness CTL (LoadProjectionInput.KnownCTL).
type LoadProjectionWeek struct {
	WeekStart   string  `json:"weekStart"`
	Days        int     `json:"days"`
	Load        float64 `json:"load"`
	StartCTL    float64 `json:"startCtl"`
	EndCTL      float64 `json:"endCtl"`
	Ramp        float64 `json:"ramp"`
	RampPerWeek float64 `json:"rampPerWeek"`
	// RampKnown is false when no day of the week has a CTL value 7 days back.
	RampKnown bool `json:"rampKnown"`
	OverCap   bool `json:"overCap,omitempty"`
}

// LoadProjectionInput feeds ProjectLoad. Loads are keyed by YYYY-MM-DD; only
// days after StartDate (whose values already include that day) through
// EndDate are projected. Days before Today use ActualLoads (completed
// activities); Today uses the larger of the completed and planned load, so a
// workout that is still planned counts and a completed one is not counted
// twice; later days use PlannedLoads. An empty Today treats every day as
// planned.
//
// intervals.icu feeds CTL and ATL separate loads (wellness ctlLoad/atlLoad):
// some sports count toward fatigue only. The *FitnessLoads maps carry the CTL
// side; nil means "same as the ATL map".
type LoadProjectionInput struct {
	StartDate    string
	StartSource  string
	StartCTL     float64
	StartATL     float64
	EndDate      string
	Today        string
	PlannedLoads map[string]float64
	ActualLoads  map[string]float64
	// PlannedFitnessLoads/ActualFitnessLoads are the CTL loads (nil: same
	// as PlannedLoads/ActualLoads).
	PlannedFitnessLoads map[string]float64
	ActualFitnessLoads  map[string]float64
	KnownCTL            map[string]float64
	CTLDays             int
	ATLDays             int
	MaxRampPerWeek      float64
}

// emaFactor is the daily smoothing factor intervals.icu uses for CTL/ATL:
// k = 1 - exp(-1/tau). The linear 1/tau approximation drifts (ATL 45.84 vs
// 45.02 live); see the replay test against real wellness rows.
func emaFactor(days int) float64 {
	return 1 - math.Exp(-1/float64(days))
}

// loadOn returns the day's (CTL load, ATL load).
func (input *LoadProjectionInput) loadOn(date string) (float64, float64) {
	plannedFitness := input.PlannedFitnessLoads
	if plannedFitness == nil {
		plannedFitness = input.PlannedLoads
	}
	actualFitness := input.ActualFitnessLoads
	if actualFitness == nil {
		actualFitness = input.ActualLoads
	}
	switch {
	case input.Today == "" || date > input.Today:
		return plannedFitness[date], input.PlannedLoads[date]
	case date == input.Today:
		// Completed load plus what is still planned. Callers leave sessions
		// already paired with a completed activity out of PlannedLoads.
		return actualFitness[date] + plannedFitness[date], input.ActualLoads[date] + input.PlannedLoads[date]
	default:
		return actualFitness[date], input.ActualLoads[date]
	}
}

// ProjectLoad runs the exponentially weighted CTL/ATL model day by day:
// x += (load - x) * (1 - exp(-1/days)). TSB is CTL - ATL, as intervals.icu
// reports form.
func ProjectLoad(input *LoadProjectionInput) (LoadProjection, error) {
	start, err := time.Parse(analysisDateLayout, input.StartDate)
	if err != nil {
		return LoadProjection{}, fmt.Errorf("projection start date %q: %w", input.StartDate, err)
	}
	end, err := time.Parse(analysisDateLayout, input.EndDate)
	if err != nil {
		return LoadProjection{}, fmt.Errorf("projection end date %q: %w", input.EndDate, err)
	}
	ctlDays := positiveOr(input.CTLDays, DefaultCTLDays)
	atlDays := positiveOr(input.ATLDays, DefaultATLDays)
	projection := LoadProjection{
		StartDate: input.StartDate, StartSource: input.StartSource,
		StartCTL: round2(input.StartCTL), StartATL: round2(input.StartATL),
		StartTSB: round2(input.StartCTL - input.StartATL),
		EndDate:  input.StartDate, CTLDays: ctlDays, ATLDays: atlDays, MaxRampPerWeek: input.MaxRampPerWeek,
	}
	ctlByDate := map[string]float64{}
	for date, value := range input.KnownCTL {
		if date <= input.StartDate {
			ctlByDate[date] = value
		}
	}
	ctlByDate[input.StartDate] = input.StartCTL
	ctlFactor, atlFactor := emaFactor(ctlDays), emaFactor(atlDays)
	ctl, atl := input.StartCTL, input.StartATL
	var week *LoadProjectionWeek
	for day := start.AddDate(0, 0, 1); !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format(analysisDateLayout)
		weekStart := planWeekStart(date)
		if week == nil || week.WeekStart != weekStart {
			projection.appendWeek(week)
			week = &LoadProjectionWeek{WeekStart: weekStart, StartCTL: round2(ctl)}
		}
		fitnessLoad, load := input.loadOn(date)
		ctl += (fitnessLoad - ctl) * ctlFactor
		atl += (load - atl) * atlFactor
		ctlByDate[date] = ctl
		if prior, ok := ctlByDate[day.AddDate(0, 0, -daysPerWeek).Format(analysisDateLayout)]; ok {
			ramp := ctl - prior
			if !week.RampKnown || ramp > week.RampPerWeek {
				week.RampPerWeek = ramp
			}
			week.RampKnown = true
		}
		week.Days++
		week.Load += load
		week.EndCTL = round2(ctl)
		projection.EndDate = date
	}
	projection.appendWeek(week)
	projection.EndCTL, projection.EndATL, projection.EndTSB = round2(ctl), round2(atl), round2(ctl-atl)

	return projection, nil
}

func (projection *LoadProjection) appendWeek(week *LoadProjectionWeek) {
	if week == nil {
		return
	}
	week.Ramp = round2(week.EndCTL - week.StartCTL)
	week.RampPerWeek = round2(week.RampPerWeek)
	if projection.MaxRampPerWeek > 0 && week.RampKnown && week.RampPerWeek > projection.MaxRampPerWeek {
		week.OverCap = true
		projection.Warnings = append(projection.Warnings, fmt.Sprintf(
			"projection: week %s rolling 7-day CTL ramp %.2f exceeds the %.2f/week cap (CTL %.1f -> %.1f)",
			week.WeekStart, week.RampPerWeek, projection.MaxRampPerWeek, week.StartCTL, week.EndCTL,
		))
	}
	projection.Weeks = append(projection.Weeks, *week)
}

func positiveOr(value, fallback int) int {
	if value > 0 {
		return value
	}

	return fallback
}

// PlanProjectedDailyLoads sums the load expected on each day: existing
// calendar workouts not touched by an update/cancel operation, plus every
// pending create/update operation's expected load.
func PlanProjectedDailyLoads(operations []CalendarOperation, events []Event) map[string]float64 {
	loads, _ := PlanProjectedDailyLoadsByModel(operations, events, nil, nil)

	return loads
}

// PlanProjectedDailyLoadsByModel is PlanProjectedDailyLoads split into the
// ATL loads and the CTL (fitness) loads, which leave out sessions whose type
// is in fitnessExcluded (see FitnessExcludedTypes). Events in completed
// (paired with a completed activity) and operations on them are skipped:
// their load is already in the actual load.
func PlanProjectedDailyLoadsByModel(
	operations []CalendarOperation, events []Event, fitnessExcluded map[string]bool, completed map[int]bool,
) (map[string]float64, map[string]float64) {
	replaced := map[int]bool{}
	loads := map[string]float64{}
	fitness := map[string]float64{}
	add := func(date, sport string, load float64) {
		if date == "" {
			return
		}
		loads[date] += load
		if !fitnessExcluded[sport] {
			fitness[date] += load
		}
	}
	for index := range operations {
		operation := &operations[index]
		if operation.Status != "" && operation.Status != CalendarStatusPending {
			continue
		}
		if operation.EventID != 0 {
			replaced[operation.EventID] = true
			if completed[operation.EventID] {
				continue
			}
		}
		if operation.Action == CalendarActionCancel {
			continue
		}
		add(dateOnly(operation.Body.StartDateLocal), operation.Body.Type, float64(operation.ExpectedLoad))
	}
	for index := range events {
		event := &events[index]
		if replaced[event.ID] || completed[event.ID] || event.Category != planWorkoutCategory {
			continue
		}
		load := event.TrainingLoad
		if load == 0 {
			load = event.LoadTarget
		}
		add(dateOnly(event.StartDateLocal), event.Type, float64(load))
	}

	return loads, fitness
}

// FitnessExcludedTypes infers which activity types intervals.icu leaves out
// of CTL for this athlete, from history: a type with load is excluded when it
// never appears on a day whose wellness ctlLoad equals atlLoad (everything
// counted) and appears on at least one day where ctlLoad < atlLoad.
// Example (live, 2026): WeightTraining and Workout count toward ATL only.
func FitnessExcludedTypes(wellness []Wellness, activities []Activity) map[string]bool {
	const tolerance = 0.5
	byDate := map[string]*Wellness{}
	for index := range wellness {
		byDate[wellness[index].ID] = &wellness[index]
	}
	included, short := map[string]bool{}, map[string]bool{}
	for index := range activities {
		activity := &activities[index]
		row := byDate[activityDate(activity)]
		if row == nil || activity.TrainingLoad <= 0 || activity.Type == "" {
			continue
		}
		if math.Abs(row.CTLLoad-row.ATLLoad) <= tolerance {
			included[activity.Type] = true
		} else if row.CTLLoad < row.ATLLoad {
			short[activity.Type] = true
		}
	}
	excluded := map[string]bool{}
	for sport := range short {
		if !included[sport] {
			excluded[sport] = true
		}
	}

	return excluded
}

func dateOnly(value string) string {
	if len(value) < len(analysisDateLayout) {
		return ""
	}

	return value[:len(analysisDateLayout)]
}

// LatestWellnessFitness returns the most recent wellness row on or before
// asOf that carries CTL/ATL; future projected rows are ignored.
func LatestWellnessFitness(records []Wellness, asOf string) (Wellness, bool) {
	best := -1
	for index := range records {
		record := &records[index]
		if record.ID == "" || record.ID > asOf || (record.CTL == 0 && record.ATL == 0) {
			continue
		}
		if best < 0 || record.ID > records[best].ID {
			best = index
		}
	}
	if best < 0 {
		return Wellness{}, false
	}

	return records[best], true
}
