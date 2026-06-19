package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	icu "github.com/Thejuampi/icu"
)

const defaultAnalysisFields = "id,name,start_date_local,type,moving_time,distance,total_elevation_gain," +
	"average_heartrate,max_heartrate,icu_weighted_avg_watts,icu_training_load,icu_intensity," +
	"icu_ftp,icu_pm_cp,icu_pm_w_prime,icu_pm_p_max,icu_pm_ftp,icu_rolling_ftp," +
	"icu_joules_above_ftp,icu_max_wbal_depletion,decoupling," +
	"icu_efficiency_factor,icu_variability_index,icu_zone_times,icu_hr_zone_times," +
	"average_temp,min_temp,max_temp,average_weather_temp,min_weather_temp,max_weather_temp," +
	"average_feels_like,average_wind_speed,average_wind_gust,prevailing_wind_deg," +
	"headwind_percent,tailwind_percent,average_altitude,min_altitude,max_altitude," +
	"average_gradient,average_lactate,min_lactate,max_lactate,average_yaw,route_id,strain_score," +
	"icu_ctl,icu_atl"

const (
	defaultAnalysisDays      = 28
	defaultPlanHistoryDays   = 84
	defaultPlanDays          = 28
	calendarWeekDays         = 7
	nextWeekdayModuloBase    = 8
	defaultWorkoutMatchHours = 24
)

type analysisTimezoneInfo struct {
	timezone string
	source   string
}

func analysisTimezone(explicit bool) analysisTimezoneInfo {
	source := icu.DefaultAnalysisTimezoneSource
	if explicit {
		source = "explicit"
	}

	return analysisTimezoneInfo{
		timezone: icu.DefaultAnalysisTimezone,
		source:   source,
	}
}

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
	registry.Register("analysis", "adaptation", analysisAdaptationCommand())
	registry.Register("analysis", "microcycle", analysisMicrocycleCommand())
	registry.Register("analysis", "micro", analysisMicroCommand())
	registry.Register("analysis", "workout", analysisWorkoutCommand())
}

func analysisCyclingCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "analysis cycling [--oldest DATE --newest DATE | --days N]",
		Description: "Compute numeric cycling analysis from completed activities. " +
			"Default ranges use UTC; pass explicit --oldest/--newest dates for athlete-local daily boundaries.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			dateRange, explicit, err := analysisDateRange(flags, time.Now())
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

			tzInfo := analysisTimezone(explicit)
			analysis := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{
				StartDate:      dateRange.Oldest,
				EndDate:        dateRange.Newest,
				Timezone:       tzInfo.timezone,
				TimezoneSource: tzInfo.source,
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
		Description: "Analyze a planned training block from recent history and future calendar events. " +
			"Default ranges use UTC; pass explicit dates for athlete-local daily boundaries.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			dateRanges, explicit, err := trainingPlanDateRanges(flags, time.Now())
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

			sportType := icu.StringFlag(flags, "sport-type", "Ride")
			var sportSettings icu.SportSettings
			if err := client.Get("sport-settings", []string{sportType}, nil, &sportSettings); err != nil {
				return wrapCommandError(err)
			}

			wellnessQuery := map[string]string{
				"oldest": dateRanges.History.Oldest,
				"newest": dateRanges.History.Newest,
			}

			var wellnessRecords []icu.Wellness
			if err := client.Get("wellness", nil, wellnessQuery, &wellnessRecords); err != nil {
				return wrapCommandError(err)
			}

			tzInfo := analysisTimezone(explicit)
			wellnessAnalysis := icu.AnalyzeWellness(wellnessRecords, icu.AnalysisOptions{
				StartDate:      dateRanges.History.Oldest,
				EndDate:        dateRanges.History.Newest,
				Timezone:       tzInfo.timezone,
				TimezoneSource: tzInfo.source,
			})

			eventQuery := queryFromFlags(flags, "calendar-id")
			eventQuery["oldest"] = dateRanges.Plan.Oldest
			eventQuery["newest"] = dateRanges.Plan.Newest
			if BoolFlag(flags, "resolve") {
				eventQuery["resolve"] = strTrue
			}

			var events []icu.Event
			if err := client.Get("events", nil, eventQuery, &events); err != nil {
				return wrapCommandError(err)
			}

			analysis := icu.AnalyzeTrainingPlanWithContext(activities, events, icu.TrainingPlanOptions{
				HistoryStartDate: dateRanges.History.Oldest,
				HistoryEndDate:   dateRanges.History.Newest,
				PlanStartDate:    dateRanges.Plan.Oldest,
				PlanEndDate:      dateRanges.Plan.Newest,
			}, icu.TrainingPlanContext{
				SportSettings: &sportSettings,
				Wellness:      &wellnessAnalysis,
				Adaptation:    nil,
			})
			analysis.Scope.Timezone = tzInfo.timezone
			analysis.Scope.TimezoneSource = tzInfo.source

			return writeJSON(analysis)
		},
	}
}

func analysisWellnessCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "analysis wellness [--oldest DATE --newest DATE | --days N]",
		Description: "Compute wellness and physiology analysis from wellness records. " +
			"Default ranges use UTC; pass explicit --oldest/--newest dates for athlete-local daily boundaries.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			dateRange, explicit, err := analysisDateRange(flags, time.Now())
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

			tzInfo := analysisTimezone(explicit)
			analysis := icu.AnalyzeWellness(records, icu.AnalysisOptions{
				StartDate:      dateRange.Oldest,
				EndDate:        dateRange.Newest,
				Timezone:       tzInfo.timezone,
				TimezoneSource: tzInfo.source,
			})

			return writeJSON(analysis)
		},
	}
}

func analysisWorkoutCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "analysis workout <activity-id> [--event-id ID] [--calendar-id ID] [--sport-type TYPE] [--match-window-hours N]",
		Description: "Analyze one completed workout against its planned workout event, intervals, and streams.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			activityID := args[0]
			inputs, options, err := readWorkoutExecutionInputs(client, flags, activityID)
			if err != nil {
				return err
			}

			analysis := icu.AnalyzeWorkoutExecution(inputs, options)

			return writeJSON(analysis)
		},
	}
}

func readWorkoutExecutionInputs(
	client *icu.Client,
	flags map[string]string,
	activityID string,
) (icu.WorkoutExecutionInputs, icu.WorkoutExecutionOptions, error) {
	var inputs icu.WorkoutExecutionInputs
	options := icu.WorkoutExecutionOptions{
		ExplicitEventID:  IntFlag(flags, "event-id", 0),
		MatchWindowHours: IntFlag(flags, "match-window-hours", defaultWorkoutMatchHours),
	}

	var activity icu.Activity
	if err := client.Get("activity", []string{activityID}, nil, &activity); err != nil {
		return inputs, options, wrapCommandError(err)
	}
	inputs.Activity = &activity

	var intervals icu.IntervalsDTO
	if err := client.Get("activity", []string{activityID, "intervals"}, nil, &intervals); err != nil {
		return inputs, options, wrapCommandError(err)
	}
	inputs.Intervals = &intervals

	streamQuery := map[string]string{"types": icu.StringFlag(flags, "stream-types", "watts,heartrate,cadence")}
	var rawStreams []icu.ActivityStream
	if err := client.Get("activity", []string{activityID, "streams"}, streamQuery, &rawStreams); err != nil {
		return inputs, options, wrapCommandError(err)
	}
	streams, err := icu.NormalizeStreams(rawStreams)
	if err != nil {
		return inputs, options, fmt.Errorf("normalize streams: %w", err)
	}
	inputs.Streams = streams

	events, err := readWorkoutExecutionEvents(client, flags, &activity, options.ExplicitEventID)
	if err != nil {
		return inputs, options, err
	}
	inputs.Events = events

	sportType := icu.StringFlag(flags, "sport-type", activity.Type)
	if sportType == "" {
		sportType = "Ride"
	}
	var sportSettings icu.SportSettings
	if err := client.Get("sport-settings", []string{sportType}, nil, &sportSettings); err != nil {
		if !isHTTPStatus(err, httpStatusNotFound) {
			return inputs, options, wrapCommandError(err)
		}
	}
	inputs.SportSettings = &sportSettings

	return inputs, options, nil
}

func readWorkoutExecutionEvents(
	client *icu.Client,
	flags map[string]string,
	activity *icu.Activity,
	explicitEventID int,
) ([]icu.Event, error) {
	if explicitEventID > 0 {
		var event icu.Event
		if err := client.Get("events", []string{strconv.Itoa(explicitEventID)}, nil, &event); err != nil {
			return nil, wrapCommandError(err)
		}

		return []icu.Event{event}, nil
	}

	activityDate := workoutActivityDate(activity)
	if activityDate == "" {
		return nil, nil
	}
	oldest, err := addDays(activityDate, -1)
	if err != nil {
		return nil, err
	}
	newest, err := addDays(activityDate, 1)
	if err != nil {
		return nil, err
	}

	eventQuery := queryFromFlags(flags, "calendar-id")
	eventQuery["oldest"] = oldest
	eventQuery["newest"] = newest
	eventQuery["resolve"] = strTrue

	var events []icu.Event
	if err := client.Get("events", nil, eventQuery, &events); err != nil {
		return nil, wrapCommandError(err)
	}

	return events, nil
}

func workoutActivityDate(activity *icu.Activity) string {
	if activity == nil || len(activity.StartDateLocal) < len("2006-01-02") {
		return ""
	}

	return activity.StartDateLocal[:len("2006-01-02")]
}

func analysisAdaptationCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "analysis adaptation [--oldest DATE --newest DATE | --days N] [--type Ride]",
		Description: "Analyze cycling adaptation from power curves, MMP model, sport anchors, activities, and wellness. " +
			"Default ranges use UTC; pass explicit --oldest/--newest dates for athlete-local daily boundaries.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			dateRange, explicit, err := analysisDateRange(flags, time.Now())
			if err != nil {
				return err
			}

			sportType := icu.StringFlag(flags, "type", "Ride")

			activityQuery := queryFromFlags(flags, "limit")
			activityQuery["oldest"] = dateRange.Oldest
			activityQuery["newest"] = dateRange.Newest
			activityQuery["fields"] = icu.StringFlag(flags, "activity-fields", defaultAnalysisFields)

			var activities []icu.Activity
			if err := client.Get("activities", nil, activityQuery, &activities); err != nil {
				return wrapCommandError(err)
			}

			curveQuery := queryFromFlags(flags, "newest", "filters")
			curveQuery["type"] = sportType
			curveQuery["curves"] = icu.StringFlag(flags, "curves", "42d,365d")

			var curveResponse struct {
				List []icu.DataCurve `json:"list"`
			}
			if err := client.Get("power-curves", nil, curveQuery, &curveResponse); err != nil {
				return wrapCommandError(err)
			}
			curves := curveResponse.List

			modelQuery := map[string]string{"type": sportType}
			var model icu.PowerModel
			if err := client.Get("mmp-model", nil, modelQuery, &model); err != nil {
				return wrapCommandError(err)
			}

			var sportSettings icu.SportSettings
			if err := client.Get("sport-settings", []string{sportType}, nil, &sportSettings); err != nil {
				return wrapCommandError(err)
			}

			wellnessQuery := map[string]string{
				"oldest": dateRange.Oldest,
				"newest": dateRange.Newest,
			}

			var wellnessRecords []icu.Wellness
			if err := client.Get("wellness", nil, wellnessQuery, &wellnessRecords); err != nil {
				return wrapCommandError(err)
			}

			tzInfo := analysisTimezone(explicit)
			wellnessAnalysis := icu.AnalyzeWellness(wellnessRecords, icu.AnalysisOptions{
				StartDate:      dateRange.Oldest,
				EndDate:        dateRange.Newest,
				Timezone:       tzInfo.timezone,
				TimezoneSource: tzInfo.source,
			})

			analysis := icu.AnalyzeCyclingAdaptation(
				curves,
				model,
				&sportSettings,
				activities,
				&wellnessAnalysis,
				icu.AnalysisOptions{
					StartDate:      dateRange.Oldest,
					EndDate:        dateRange.Newest,
					Timezone:       tzInfo.timezone,
					TimezoneSource: tzInfo.source,
				},
			)

			return writeJSON(analysis)
		},
	}
}

func trainingPlanDateRanges(flags map[string]string, now time.Time) (trainingPlanRanges, bool, error) {
	history, historyExplicit, err := trainingPlanHistoryRange(flags, now)
	if err != nil {
		return trainingPlanRanges{}, false, err
	}

	plan, planExplicit, err := trainingPlanFutureRange(flags, now)
	if err != nil {
		return trainingPlanRanges{}, false, err
	}

	return trainingPlanRanges{History: history, Plan: plan}, historyExplicit || planExplicit, nil
}

func trainingPlanHistoryRange(flags map[string]string, now time.Time) (analysisRange, bool, error) {
	oldest := icu.StringFlag(flags, "history-oldest", "")
	newest := icu.StringFlag(flags, "history-newest", "")

	if oldest != "" || newest != "" {
		if oldest == "" || newest == "" {
			return analysisRange{}, false, errMissing("--history-oldest and --history-newest")
		}

		return analysisRange{Oldest: oldest, Newest: newest}, true, nil
	}

	days := IntFlag(flags, "history-days", defaultPlanHistoryDays)
	if days <= 0 {
		return analysisRange{}, false, fmt.Errorf("%w: --history-days must be greater than 0", errMissingRequired)
	}

	end := now.UTC()
	start := end.AddDate(0, 0, -days+1)

	return analysisRange{Oldest: start.Format("2006-01-02"), Newest: end.Format("2006-01-02")}, false, nil
}

func trainingPlanFutureRange(flags map[string]string, now time.Time) (analysisRange, bool, error) {
	oldest := icu.StringFlag(flags, "plan-oldest", "")
	newest := icu.StringFlag(flags, "plan-newest", "")

	if oldest != "" || newest != "" {
		if oldest == "" || newest == "" {
			return analysisRange{}, false, errMissing("--plan-oldest and --plan-newest")
		}

		return analysisRange{Oldest: oldest, Newest: newest}, true, nil
	}

	days := IntFlag(flags, "plan-days", defaultPlanDays)
	if days <= 0 {
		return analysisRange{}, false, fmt.Errorf("%w: --plan-days must be greater than 0", errMissingRequired)
	}

	start := nextISOBlockStart(now.UTC())
	end := start.AddDate(0, 0, days-1)

	return analysisRange{Oldest: start.Format("2006-01-02"), Newest: end.Format("2006-01-02")}, false, nil
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

func analysisDateRange(flags map[string]string, now time.Time) (analysisRange, bool, error) {
	oldest := icu.StringFlag(flags, "oldest", "")
	newest := icu.StringFlag(flags, "newest", "")

	if oldest != "" || newest != "" {
		if oldest == "" || newest == "" {
			return analysisRange{}, false, errMissing("--oldest and --newest")
		}

		return analysisRange{Oldest: oldest, Newest: newest}, true, nil
	}

	days := IntFlag(flags, "days", defaultAnalysisDays)
	if days <= 0 {
		return analysisRange{}, false, fmt.Errorf("%w: --days must be greater than 0", errMissingRequired)
	}

	normalizedNow := now.UTC()
	end := normalizedNow.Format("2006-01-02")
	start := normalizedNow.AddDate(0, 0, -days+1).Format("2006-01-02")

	return analysisRange{Oldest: start, Newest: end}, false, nil
}

func analysisMicroCommand() *Command {
	cmd := analysisMicrocycleCommand()
	cmd.Usage = "analysis micro [--date DATE | --week DATE | --from DATE --to DATE] [--json] [--full] [--no-plan] [--no-wellness] [--sport-type TYPE] [--timezone TZ]"
	cmd.Description = "[experimental alias] Analyze the current or selected training microcycle for LLM-ready diagnostics."

	return cmd
}

func analysisMicrocycleCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "analysis microcycle [--date DATE | --week DATE | --from DATE --to DATE] [--json] [--full] [--no-plan] [--no-wellness] [--sport-type TYPE] [--timezone TZ]",
		Description: "[experimental] Analyze the selected training microcycle as a read-only, LLM-ready diagnostic contract.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			now := time.Now()
			dateRange, location, err := microcycleDateRange(flags, now)
			if err != nil {
				return err
			}

			lookbackStart, err := addDays(dateRange.Oldest, -90)
			if err != nil {
				return err
			}

			includePlan := !BoolFlag(flags, "no-plan")
			includeWellness := !BoolFlag(flags, "no-wellness")

			inputs, err := readMicrocycleInputs(
				client,
				flags,
				dateRange,
				lookbackStart,
				includePlan,
				includeWellness,
				location.String(),
				microcycleTimezoneSource(flags),
			)
			if err != nil {
				return err
			}

			sportType := icu.StringFlag(flags, "sport-type", "Ride")
			analysis := icu.AnalyzeMicrocycle(inputs.Activities, inputs.Events, inputs.Wellness, &inputs.SportSettings, &icu.MicrocycleOptions{
				StartDate:        dateRange.Oldest,
				EndDate:          dateRange.Newest,
				Timezone:         location.String(),
				TimezoneSource:   microcycleTimezoneSource(flags),
				Now:              now.In(location),
				PlanIncluded:     includePlan,
				WellnessIncluded: includeWellness,
				SportType:        sportType,
				Full:             BoolFlag(flags, "full"),
			})

			if BoolFlag(flags, "json") || icu.ResolveOutputFormat(flags) == icu.FormatJSON {
				return writeJSON(analysis)
			}

			return writeMicrocycleHuman(&analysis)
		},
	}
}

type microcycleInputs struct {
	Activities    []icu.Activity
	Events        []icu.Event
	Wellness      *icu.WellnessAnalysis
	SportSettings icu.SportSettings
}

func readMicrocycleInputs(
	client *icu.Client,
	flags map[string]string,
	dateRange analysisRange,
	lookbackStart string,
	includePlan bool,
	includeWellness bool,
	timezone string,
	timezoneSource string,
) (microcycleInputs, error) {
	var inputs microcycleInputs

	activities, err := readMicrocycleActivities(client, flags, dateRange, lookbackStart)
	if err != nil {
		return inputs, err
	}
	inputs.Activities = activities

	if includePlan {
		events, err := readMicrocycleEvents(client, flags, dateRange)
		if err != nil {
			return inputs, err
		}
		inputs.Events = events
	}

	if includeWellness {
		wellness, err := readMicrocycleWellness(client, dateRange, lookbackStart, timezone, timezoneSource)
		if err != nil {
			return inputs, err
		}
		inputs.Wellness = wellness
	}

	sportType := icu.StringFlag(flags, "sport-type", "Ride")
	if err := client.Get("sport-settings", []string{sportType}, nil, &inputs.SportSettings); err != nil {
		if isHTTPStatus(err, httpStatusNotFound) {
			return inputs, nil
		}

		return inputs, wrapCommandError(err)
	}

	return inputs, nil
}

const httpStatusNotFound = 404

func isHTTPStatus(err error, status int) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), fmt.Sprintf("HTTP status error %d", status))
}

func readMicrocycleActivities(
	client *icu.Client,
	flags map[string]string,
	dateRange analysisRange,
	lookbackStart string,
) ([]icu.Activity, error) {
	activityQuery := queryFromFlags(flags, "limit")
	activityQuery["oldest"] = lookbackStart
	activityQuery["newest"] = dateRange.Newest
	activityQuery["fields"] = icu.StringFlag(flags, "activity-fields", defaultAnalysisFields)

	var activities []icu.Activity
	if err := client.Get("activities", nil, activityQuery, &activities); err != nil {
		return nil, wrapCommandError(err)
	}

	return activities, nil
}

func readMicrocycleEvents(client *icu.Client, flags map[string]string, dateRange analysisRange) ([]icu.Event, error) {
	eventQuery := queryFromFlags(flags, "calendar_id")
	eventQuery["oldest"] = dateRange.Oldest
	eventQuery["newest"] = dateRange.Newest
	if BoolFlag(flags, "resolve") {
		eventQuery["resolve"] = strTrue
	}

	var events []icu.Event
	if err := client.Get("events", nil, eventQuery, &events); err != nil {
		return nil, wrapCommandError(err)
	}

	return events, nil
}

func readMicrocycleWellness(
	client *icu.Client,
	dateRange analysisRange,
	lookbackStart string,
	timezone string,
	timezoneSource string,
) (*icu.WellnessAnalysis, error) {
	wellnessQuery := map[string]string{
		"oldest": lookbackStart,
		"newest": dateRange.Newest,
	}
	var wellnessRecords []icu.Wellness
	if err := client.Get("wellness", nil, wellnessQuery, &wellnessRecords); err != nil {
		return nil, wrapCommandError(err)
	}

	analysis := icu.AnalyzeWellness(wellnessRecords, icu.AnalysisOptions{
		StartDate:      lookbackStart,
		EndDate:        dateRange.Newest,
		Timezone:       timezone,
		TimezoneSource: timezoneSource,
	})

	return &analysis, nil
}

func microcycleDateRange(flags map[string]string, now time.Time) (analysisRange, *time.Location, error) {
	location, err := microcycleLocation(flags)
	if err != nil {
		return analysisRange{}, nil, err
	}
	localNow := now.In(location)

	from := icu.StringFlag(flags, "from", "")
	to := icu.StringFlag(flags, "to", "")
	week := icu.StringFlag(flags, "week", "")
	date := icu.StringFlag(flags, "date", "")

	if from != "" || to != "" {
		if from == "" || to == "" {
			return analysisRange{}, nil, errMissing("--from and --to")
		}
		if week != "" || date != "" {
			return analysisRange{}, nil, fmt.Errorf("%w: --from/--to cannot be combined with --week or --date", errMissingRequired)
		}
		if err := validateDateRange(from, to); err != nil {
			return analysisRange{}, nil, err
		}

		return analysisRange{Oldest: from, Newest: to}, location, nil
	}
	if week != "" && date != "" {
		return analysisRange{}, nil, fmt.Errorf("%w: --week cannot be combined with --date", errMissingRequired)
	}
	if week != "" {
		return isoWeekRange(week, location)
	}
	if date != "" {
		return isoWeekRange(date, location)
	}

	return isoWeekRange(localNow.Format("2006-01-02"), location)
}

func microcycleLocation(flags map[string]string) (*time.Location, error) {
	name := icu.StringFlag(flags, "timezone", "")
	if name == "" {
		return time.Now().Location(), nil
	}

	location, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid --timezone %s", errMissingRequired, name)
	}

	return location, nil
}

func microcycleTimezoneSource(flags map[string]string) string {
	if icu.StringFlag(flags, "timezone", "") != "" {
		return "flag"
	}

	return "system"
}

func isoWeekRange(date string, location *time.Location) (analysisRange, *time.Location, error) {
	parsed, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		return analysisRange{}, nil, fmt.Errorf("%w: invalid date %s", errMissingRequired, date)
	}

	weekday := int(parsed.Weekday())
	if weekday == 0 {
		weekday = calendarWeekDays
	}
	start := parsed.AddDate(0, 0, -(weekday - 1))
	end := start.AddDate(0, 0, calendarWeekDays-1)

	return analysisRange{Oldest: start.Format("2006-01-02"), Newest: end.Format("2006-01-02")}, location, nil
}

func validateDateRange(from, to string) error {
	start, startErr := time.Parse("2006-01-02", from)
	end, endErr := time.Parse("2006-01-02", to)
	if startErr != nil {
		return fmt.Errorf("%w: invalid --from %s", errMissingRequired, from)
	}
	if endErr != nil {
		return fmt.Errorf("%w: invalid --to %s", errMissingRequired, to)
	}
	if end.Before(start) {
		return fmt.Errorf("%w: --from must be on or before --to", errMissingRequired)
	}

	return nil
}

func addDays(date string, days int) (string, error) {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("%w: invalid date %s", errMissingRequired, date)
	}

	return parsed.AddDate(0, 0, days).Format("2006-01-02"), nil
}

func writeMicrocycleHuman(analysis *icu.MicrocycleAnalysis) error {
	var output strings.Builder
	fmt.Fprintf(&output, "Microcycle: %s to %s\n", analysis.Microcycle.StartDate, analysis.Microcycle.EndDate)
	fmt.Fprintf(
		&output, "Status: partial=%t, elapsed=%d, remaining=%d\n",
		analysis.Microcycle.IsPartial,
		analysis.Microcycle.ElapsedDays,
		analysis.Microcycle.RemainingDays,
	)
	fmt.Fprintf(&output, "Timezone: %s (%s)\n", analysis.Microcycle.Timezone, analysis.Microcycle.TimezoneSource)
	fmt.Fprintf(&output, "Classification: %s\n", analysis.Classification.Value)
	fmt.Fprintf(&output, "Confidence: %s\n", analysis.Confidence)
	fmt.Fprintf(
		&output, "Activities: %d, Load: %d, Z4+ sessions: %d\n",
		analysis.Load.ActivityCount,
		analysis.Load.TSS,
		analysis.Intensity.Z4PlusSessions,
	)
	fmt.Fprintf(&output, "Data quality warnings: %d\n", len(analysis.DataQuality.Warnings))
	if analysis.Classification.MainPositiveSignal != "" {
		fmt.Fprintf(&output, "Main positive signal: %s\n", analysis.Classification.MainPositiveSignal)
	}
	if analysis.Classification.MainRisk != "" {
		fmt.Fprintf(&output, "Main risk: %s\n", analysis.Classification.MainRisk)
	}
	fmt.Fprintln(&output, "Final note: diagnostic only; no plan, config, or external sync changes were made.")

	return writeOutput([]byte(output.String()))
}
