package icu

import (
	"fmt"
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

// LoadProjectionWeek is one ISO week of the projection. RampPerWeek is the
// CTL change scaled to 7 days so partial weeks compare against the cap.
type LoadProjectionWeek struct {
	WeekStart   string  `json:"weekStart"`
	Days        int     `json:"days"`
	Load        float64 `json:"load"`
	StartCTL    float64 `json:"startCtl"`
	EndCTL      float64 `json:"endCtl"`
	Ramp        float64 `json:"ramp"`
	RampPerWeek float64 `json:"rampPerWeek"`
	OverCap     bool    `json:"overCap,omitempty"`
}

// LoadProjectionInput feeds ProjectLoad. DailyLoads are keyed by
// YYYY-MM-DD; only days after StartDate (whose values already include that
// day) through EndDate are projected.
type LoadProjectionInput struct {
	StartDate      string
	StartSource    string
	StartCTL       float64
	StartATL       float64
	EndDate        string
	DailyLoads     map[string]float64
	CTLDays        int
	ATLDays        int
	MaxRampPerWeek float64
}

// ProjectLoad runs the exponentially weighted CTL/ATL model day by day:
// x += (load - x) / days. TSB is CTL - ATL, as intervals.icu reports form.
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
	ctl, atl := input.StartCTL, input.StartATL
	var week *LoadProjectionWeek
	for day := start.AddDate(0, 0, 1); !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format(analysisDateLayout)
		weekStart := planWeekStart(date)
		if week == nil || week.WeekStart != weekStart {
			projection.appendWeek(week)
			week = &LoadProjectionWeek{WeekStart: weekStart, StartCTL: round2(ctl)}
		}
		load := input.DailyLoads[date]
		ctl += (load - ctl) / float64(ctlDays)
		atl += (load - atl) / float64(atlDays)
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
	week.RampPerWeek = round2(week.Ramp * daysPerWeek / float64(week.Days))
	if projection.MaxRampPerWeek > 0 && week.RampPerWeek > projection.MaxRampPerWeek {
		week.OverCap = true
		projection.Warnings = append(projection.Warnings, fmt.Sprintf(
			"projection: week %s CTL ramp %.2f/week exceeds the %.2f/week cap (CTL %.1f -> %.1f)",
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
	replaced := map[int]bool{}
	loads := map[string]float64{}
	for index := range operations {
		operation := &operations[index]
		if operation.Status != "" && operation.Status != CalendarStatusPending {
			continue
		}
		if operation.EventID != 0 {
			replaced[operation.EventID] = true
		}
		if operation.Action == CalendarActionCancel {
			continue
		}
		if date := dateOnly(operation.Body.StartDateLocal); date != "" {
			loads[date] += float64(operation.ExpectedLoad)
		}
	}
	for index := range events {
		event := &events[index]
		if replaced[event.ID] || event.Category != planWorkoutCategory {
			continue
		}
		load := event.TrainingLoad
		if load == 0 {
			load = event.LoadTarget
		}
		if date := dateOnly(event.StartDateLocal); date != "" {
			loads[date] += float64(load)
		}
	}

	return loads
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
