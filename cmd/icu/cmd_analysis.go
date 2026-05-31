package main

import (
	"fmt"
	"time"

	icu "github.com/Thejuampi/icu"
)

const defaultAnalysisFields = "id,name,start_date_local,type,moving_time,distance,total_elevation_gain," +
	"average_heartrate,max_heartrate,icu_weighted_avg_watts,icu_training_load,icu_intensity," +
	"icu_ftp,icu_joules_above_ftp,icu_max_wbal_depletion,decoupling," +
	"icu_efficiency_factor,icu_variability_index,icu_zone_times,icu_hr_zone_times,icu_ctl,icu_atl"

const (
	defaultAnalysisDays    = 28
	defaultPlanHistoryDays = 84
	defaultPlanDays        = 28
	calendarWeekDays       = 7
	nextWeekdayModuloBase  = 8
)

type analysisRange struct {
	Oldest string
	Newest string
}

type trainingPlanRanges struct {
	History analysisRange
	Plan    analysisRange
}

func registerAnalysisCommands(registry *CommandRegistry) {
	registry.Register("analysis", "cycling", analysisCyclingCommand())
	registry.Register("analysis", "wellness", analysisWellnessCommand())
	registry.Register("analysis", "plan", analysisPlanCommand())
}

func analysisCyclingCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "analysis cycling [--oldest DATE --newest DATE | --days N]",
		Description: "Compute numeric cycling analysis from completed activities.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			dateRange, err := analysisDateRange(flags, time.Now())
			if err != nil {
				return err
			}

			query := queryFromFlags(flags, "limit")
			query["oldest"] = dateRange.Oldest
			query["newest"] = dateRange.Newest
			query["fields"] = icu.StringFlag(flags, "fields", defaultAnalysisFields)

			var activities []icu.Activity
			if err := client.Get("activities", nil, query, &activities); err != nil {
				return wrapCommandError(err)
			}

			analysis := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{
				StartDate: dateRange.Oldest,
				EndDate:   dateRange.Newest,
			})

			return writeJSON(analysis)
		},
	}
}

func analysisPlanCommand() *Command {
	return &Command{
		Name: "",
		Usage: "analysis plan [--history-oldest DATE --history-newest DATE] " +
			"[--plan-oldest DATE --plan-newest DATE]",
		Description: "Analyze a planned training block from recent history and future calendar events.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			dateRanges, err := trainingPlanDateRanges(flags, time.Now())
			if err != nil {
				return err
			}

			activityQuery := queryFromFlags(flags, "limit")
			activityQuery["oldest"] = dateRanges.History.Oldest
			activityQuery["newest"] = dateRanges.History.Newest
			activityQuery["fields"] = icu.StringFlag(flags, "activity-fields", defaultAnalysisFields)

			var activities []icu.Activity
			if err := client.Get("activities", nil, activityQuery, &activities); err != nil {
				return wrapCommandError(err)
			}

			eventQuery := queryFromFlags(flags, "calendar_id")
			eventQuery["oldest"] = dateRanges.Plan.Oldest
			eventQuery["newest"] = dateRanges.Plan.Newest
			if BoolFlag(flags, "resolve") {
				eventQuery["resolve"] = strTrue
			}

			var events []icu.Event
			if err := client.Get("events", nil, eventQuery, &events); err != nil {
				return wrapCommandError(err)
			}

			analysis := icu.AnalyzeTrainingPlan(activities, events, icu.TrainingPlanOptions{
				HistoryStartDate: dateRanges.History.Oldest,
				HistoryEndDate:   dateRanges.History.Newest,
				PlanStartDate:    dateRanges.Plan.Oldest,
				PlanEndDate:      dateRanges.Plan.Newest,
			})

			return writeJSON(analysis)
		},
	}
}

func analysisWellnessCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "analysis wellness [--oldest DATE --newest DATE | --days N]",
		Description: "Compute wellness and physiology analysis from wellness records.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			dateRange, err := analysisDateRange(flags, time.Now())
			if err != nil {
				return err
			}

			query := queryFromFlags(flags, "fields")
			query["oldest"] = dateRange.Oldest
			query["newest"] = dateRange.Newest

			var records []icu.Wellness
			if err := client.Get("wellness", nil, query, &records); err != nil {
				return wrapCommandError(err)
			}

			analysis := icu.AnalyzeWellness(records, icu.AnalysisOptions{
				StartDate: dateRange.Oldest,
				EndDate:   dateRange.Newest,
			})

			return writeJSON(analysis)
		},
	}
}

func trainingPlanDateRanges(flags map[string]string, now time.Time) (trainingPlanRanges, error) {
	history, err := trainingPlanHistoryRange(flags, now)
	if err != nil {
		return trainingPlanRanges{}, err
	}

	plan, err := trainingPlanFutureRange(flags, now)
	if err != nil {
		return trainingPlanRanges{}, err
	}

	return trainingPlanRanges{History: history, Plan: plan}, nil
}

func trainingPlanHistoryRange(flags map[string]string, now time.Time) (analysisRange, error) {
	oldest := icu.StringFlag(flags, "history-oldest", "")
	newest := icu.StringFlag(flags, "history-newest", "")

	if oldest != "" || newest != "" {
		if oldest == "" || newest == "" {
			return analysisRange{}, errMissing("--history-oldest and --history-newest")
		}

		return analysisRange{Oldest: oldest, Newest: newest}, nil
	}

	days := IntFlag(flags, "history-days", defaultPlanHistoryDays)
	if days <= 0 {
		return analysisRange{}, fmt.Errorf("%w: --history-days must be greater than 0", errMissingRequired)
	}

	end := now.UTC()
	start := end.AddDate(0, 0, -days+1)

	return analysisRange{Oldest: start.Format("2006-01-02"), Newest: end.Format("2006-01-02")}, nil
}

func trainingPlanFutureRange(flags map[string]string, now time.Time) (analysisRange, error) {
	oldest := icu.StringFlag(flags, "plan-oldest", "")
	newest := icu.StringFlag(flags, "plan-newest", "")

	if oldest != "" || newest != "" {
		if oldest == "" || newest == "" {
			return analysisRange{}, errMissing("--plan-oldest and --plan-newest")
		}

		return analysisRange{Oldest: oldest, Newest: newest}, nil
	}

	days := IntFlag(flags, "plan-days", defaultPlanDays)
	if days <= 0 {
		return analysisRange{}, fmt.Errorf("%w: --plan-days must be greater than 0", errMissingRequired)
	}

	start := nextISOBlockStart(now.UTC())
	end := start.AddDate(0, 0, days-1)

	return analysisRange{Oldest: start.Format("2006-01-02"), Newest: end.Format("2006-01-02")}, nil
}

func nextISOBlockStart(now time.Time) time.Time {
	var weekday int
	weekday = int(now.Weekday())

	if weekday == 0 {
		weekday = calendarWeekDays
	}

	daysUntilMonday := (nextWeekdayModuloBase - weekday) % calendarWeekDays

	return now.AddDate(0, 0, daysUntilMonday)
}

func analysisDateRange(flags map[string]string, now time.Time) (analysisRange, error) {
	oldest := icu.StringFlag(flags, "oldest", "")
	newest := icu.StringFlag(flags, "newest", "")

	if oldest != "" || newest != "" {
		if oldest == "" || newest == "" {
			return analysisRange{}, errMissing("--oldest and --newest")
		}

		return analysisRange{Oldest: oldest, Newest: newest}, nil
	}

	days := IntFlag(flags, "days", defaultAnalysisDays)
	if days <= 0 {
		return analysisRange{}, fmt.Errorf("%w: --days must be greater than 0", errMissingRequired)
	}

	normalizedNow := now.UTC()
	end := normalizedNow.Format("2006-01-02")
	start := normalizedNow.AddDate(0, 0, -days+1).Format("2006-01-02")

	return analysisRange{Oldest: start, Newest: end}, nil
}
