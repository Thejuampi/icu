package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	icu "github.com/Thejuampi/icu"
)

// defaultAnalysisFields is D-8's single choke point: every analysis/rebalance
// command that fetches activities uses exactly this field list, never a
// caller-supplied override (see analysisActivityFields below), so wave-2's
// validity/pairing/compliance/tri-load fields (F-19, D-2, D-21, D-23) are
// always present regardless of --fields/--activity-fields.
const defaultAnalysisFields = "id,name,start_date_local,type,moving_time,distance,total_elevation_gain," +
	"average_heartrate,max_heartrate,icu_weighted_avg_watts,icu_training_load,icu_intensity," +
	"icu_ftp,icu_pm_cp,icu_pm_w_prime,icu_pm_p_max,icu_pm_ftp,icu_rolling_ftp," +
	"icu_joules_above_ftp,icu_max_wbal_depletion,decoupling," +
	"icu_efficiency_factor,icu_variability_index,icu_zone_times,icu_hr_zone_times," +
	"average_temp,min_temp,max_temp,average_weather_temp,min_weather_temp,max_weather_temp," +
	"average_feels_like,average_wind_speed,average_wind_gust,prevailing_wind_deg," +
	"headwind_percent,tailwind_percent,average_altitude,min_altitude,max_altitude," +
	"average_gradient,average_lactate,min_lactate,max_lactate,average_yaw,route_id,strain_score," +
	"icu_ctl,icu_atl,source,paired_event_id,compliance,hr_load,hr_load_type," +
	"power_load,pace_load,pace_load_type,icu_ignore_power,icu_ignore_hr,icu_ignore_time,analyzed"

// analysisActivityFields is D-8's choke point function: the wave's own
// evidence (F-19) is that --fields/--activity-fields have never once
// changed what a live request needed, so this ignores any CLI-supplied
// value outright rather than unioning it in. CyclingValidity.FieldsGuaranteed
// records that this guarantee held for the analysis that consumed it.
func analysisActivityFields() string {
	return defaultAnalysisFields
}

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
		source = timezoneSourceExplicit
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
	registry.Register("analysis", "coaching", analysisCoachingCommand())
	registry.Register("analysis", "wellness", analysisWellnessCommand())
	registry.Register("analysis", "plan", analysisPlanCommand())
	registry.Register("analysis", "adaptation", analysisAdaptationCommand())
	registry.Register("analysis", "microcycle", analysisMicrocycleCommand())
	registry.Register("analysis", "micro", analysisMicroCommand())
	registry.Register("analysis", "workout", analysisWorkoutCommand())
}

func analysisCoachingCommand() *Command {
	return &Command{
		Name: "",
		Usage: "analysis coaching [--history-oldest DATE --history-newest DATE] " +
			"[--plan-oldest DATE --plan-newest DATE] [--history-days N] [--plan-days N] " +
			"[--sport-type TYPE] [--calendar-id ID] [--resolve BOOL] [--activity-fields CSV] [--limit N] " +
			"[--include-adaptation BOOL] [--include-private BOOL] [--adaptation-curves CSV]",
		Description: "Build one JSON coaching context from athlete, sport settings, activities, wellness, plan events, " +
			"and optional adaptation analysis.",
		Schema: analysisCoachingSchema(),
		Validate: func(flags map[string]string) error {
			_, _, err := trainingPlanDateRanges(flags, time.Now())

			return err
		},
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			dateRanges, explicit, err := trainingPlanDateRanges(flags, time.Now())
			if err != nil {
				return err
			}

			inputs, err := readCoachingContextInputs(client, flags, dateRanges, explicit)
			if err != nil {
				return err
			}

			tzInfo := analysisTimezone(explicit)
			context := icu.BuildCoachingContext(inputs, &icu.CoachingContextOptions{
				SportType:         icu.StringFlag(flags, "sport-type", "Ride"),
				HistoryStartDate:  dateRanges.History.Oldest,
				HistoryEndDate:    dateRanges.History.Newest,
				PlanStartDate:     dateRanges.Plan.Oldest,
				PlanEndDate:       dateRanges.Plan.Newest,
				Timezone:          tzInfo.timezone,
				TimezoneSource:    tzInfo.source,
				IncludeAdaptation: BoolFlag(flags, "include-adaptation"),
			})
			if context.Athlete != nil {
				athlete := *context.Athlete
				redactAthletePrivate(&athlete, flags)
				context.Athlete = &athlete
			}

			return writeJSON(stdout, context)
		},
	}
}

func analysisCoachingSchema() *CommandSchema {
	return &CommandSchema{
		RejectPositionals: true,
		Flags: append([]CommandFlag{
			{Name: "history-oldest", ValueName: "DATE", Description: "History range start (YYYY-MM-DD)."},
			{Name: "history-newest", ValueName: "DATE", Description: "History range end (YYYY-MM-DD)."},
			{Name: "plan-oldest", ValueName: "DATE", Description: "Plan range start (YYYY-MM-DD)."},
			{Name: "plan-newest", ValueName: "DATE", Description: "Plan range end (YYYY-MM-DD)."},
			{Name: "history-days", ValueName: "N", Description: "History window length.", Default: "84", Kind: commandFlagPositiveInteger},
			{Name: "plan-days", ValueName: "N", Description: "Plan window length.", Default: "28", Kind: commandFlagPositiveInteger},
			{Name: "sport-type", ValueName: "TYPE", Description: "Intervals.icu sport type.", Default: "Ride"},
			{Name: "calendar-id", ValueName: "ID", Description: "Calendar ID."},
			{Name: "resolve", ValueName: "BOOL", Description: "Resolve calendar event references.", Kind: commandFlagBoolean},
			{Name: "activity-fields", ValueName: "CSV", Description: "Deprecated, ignored (D-8): the analysis fetch always " +
				"uses its own fixed field list so wave-2's validity/pairing/compliance fields are never dropped."},
			{Name: "limit", ValueName: "N", Description: "Maximum history activities fetched.", Kind: commandFlagPositiveInteger},
			{Name: "include-adaptation", ValueName: "BOOL", Description: "Include adaptation analysis.", Kind: commandFlagBoolean},
			{Name: "include-private", ValueName: "BOOL", Description: "Keep athlete date of birth and Strava id in the output.", Kind: commandFlagBoolean},
			{Name: "adaptation-curves", ValueName: "CSV", Description: "Power curves used by adaptation.", Default: "42d,365d"},
			{Name: "api-key", ValueName: "KEY", Description: "Intervals.icu API key."},
			{Name: "athlete-id", ValueName: "ID", Description: "Intervals.icu athlete ID."},
		}, decouplingSchemaFlags()...),
	}
}

func readCoachingContextInputs(
	client *icu.Client,
	flags map[string]string,
	dateRanges trainingPlanRanges,
	explicit bool,
) (icu.CoachingContextInputs, error) {
	var inputs icu.CoachingContextInputs

	var athlete icu.Athlete
	if err := client.Get("athlete", nil, nil, &athlete); err != nil {
		return inputs, wrapCommandError(err)
	}
	inputs.Athlete = &athlete

	sportType := icu.StringFlag(flags, "sport-type", "Ride")
	var sportSettings icu.SportSettings
	if err := client.Get("sport-settings", []string{sportType}, nil, &sportSettings); err != nil {
		if !isHTTPNotFound(err) {
			return inputs, wrapCommandError(err)
		}
	} else {
		inputs.SportSettings = &sportSettings
	}

	activities, err := readCoachingActivities(client, flags, dateRanges.History)
	if err != nil {
		return inputs, err
	}

	wellnessRecords, wellnessWarnings, err := readCoachingWellnessRecords(client, dateRanges.History)
	if err != nil {
		return inputs, err
	}

	events, err := readCoachingEvents(client, flags, dateRanges.Plan)
	if err != nil {
		return inputs, err
	}
	inputs.Events = events

	tzInfo := analysisTimezone(explicit)
	cycling := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{
		Thresholds:     profileThresholds(),
		StartDate:      dateRanges.History.Oldest,
		EndDate:        dateRanges.History.Newest,
		Timezone:       tzInfo.timezone,
		TimezoneSource: tzInfo.source,
	})
	inputs.Cycling = &cycling

	wellness := analyzeWellness(wellnessRecords, wellnessWarnings, icu.AnalysisOptions{
		Thresholds:     profileThresholds(),
		StartDate:      dateRanges.History.Oldest,
		EndDate:        dateRanges.History.Newest,
		Timezone:       tzInfo.timezone,
		TimezoneSource: tzInfo.source,
	})
	inputs.Wellness = &wellness

	if BoolFlag(flags, "include-adaptation") {
		adaptation, err := readCoachingAdaptation(client, flags, sportType, dateRanges.History, activities, &wellness, inputs.SportSettings, tzInfo)
		if err != nil {
			return inputs, err
		}
		inputs.Adaptation = adaptation
	}

	plan := icu.AnalyzeTrainingPlanWithContext(activities, events, icu.TrainingPlanOptions{
		Thresholds:       profileThresholds(),
		HistoryStartDate: dateRanges.History.Oldest,
		HistoryEndDate:   dateRanges.History.Newest,
		PlanStartDate:    dateRanges.Plan.Oldest,
		PlanEndDate:      dateRanges.Plan.Newest,
	}, icu.TrainingPlanContext{
		SportSettings: inputs.SportSettings,
		Wellness:      &wellness,
		Adaptation:    inputs.Adaptation,
	})
	plan.Scope.Timezone = tzInfo.timezone
	plan.Scope.TimezoneSource = tzInfo.source
	inputs.Plan = &plan

	return inputs, nil
}

func readCoachingActivities(client *icu.Client, flags map[string]string, dateRange analysisRange) ([]icu.Activity, error) {
	query := queryFromFlags(flags, "limit")
	query["oldest"] = dateRange.Oldest
	query["newest"] = dateRange.Newest
	query["fields"] = analysisActivityFields()

	var activities []icu.Activity
	if err := client.Get("activities", nil, query, &activities); err != nil {
		return nil, wrapCommandError(err)
	}

	if err := enrichActivitiesDecoupling(client, activities, flags); err != nil {
		return nil, err
	}

	return activities, nil
}

func readCoachingWellnessRecords(client *icu.Client, dateRange analysisRange) ([]icu.Wellness, []string, error) {
	return readWellnessRecordsForAnalysis(client, dateRange.Oldest, dateRange.Newest, nil)
}

func readCoachingEvents(client *icu.Client, flags map[string]string, dateRange analysisRange) ([]icu.Event, error) {
	query := queryFromFlags(flags, "calendar-id")
	query["oldest"] = dateRange.Oldest
	query["newest"] = dateRange.Newest
	if BoolFlag(flags, "resolve") {
		query["resolve"] = strTrue
	}

	var events []icu.Event
	if err := client.Get("events", nil, query, &events); err != nil {
		return nil, wrapCommandError(err)
	}

	return events, nil
}

func readCoachingAdaptation(
	client *icu.Client,
	flags map[string]string,
	sportType string,
	dateRange analysisRange,
	activities []icu.Activity,
	wellness *icu.WellnessAnalysis,
	sportSettings *icu.SportSettings,
	tzInfo analysisTimezoneInfo,
) (*icu.CyclingAdaptationAnalysis, error) {
	curveQuery := map[string]string{
		"type":   sportType,
		"curves": icu.StringFlag(flags, "adaptation-curves", ""),
	}
	if curveQuery["curves"] == "" {
		windows, err := icu.AdaptationCurveWindows(dateRange.Oldest, dateRange.Newest)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidValue, err)
		}
		curveQuery["curves"] = windows
	}

	var curveResponse struct {
		List []icu.DataCurve `json:"list"`
	}
	if err := client.Get("power-curves", nil, curveQuery, &curveResponse); err != nil {
		return nil, wrapCommandError(err)
	}

	var model icu.PowerModel
	if err := client.Get("mmp-model", nil, map[string]string{"type": sportType}, &model); err != nil {
		return nil, wrapCommandError(err)
	}

	analysis := icu.AnalyzeCyclingAdaptation(
		curveResponse.List,
		model,
		sportSettings,
		activities,
		wellness,
		icu.AnalysisOptions{
			Thresholds:     profileThresholds(),
			StartDate:      dateRange.Oldest,
			EndDate:        dateRange.Newest,
			Timezone:       tzInfo.timezone,
			TimezoneSource: tzInfo.source,
		},
	)

	return &analysis, nil
}

func analysisCyclingCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "analysis cycling [--oldest DATE --newest DATE | --days N] [--min-included-activities N]",
		Description: "Compute numeric cycling analysis from completed activities. " +
			"Default ranges use UTC; pass explicit --oldest/--newest dates for athlete-local daily boundaries. " +
			"--min-included-activities has no default (D-4): when set and validity.included falls below it, " +
			"load/intensity/durability refuse with a named warning instead of reporting on too few sessions.",
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			dateRange, explicit, err := analysisDateRange(flags, time.Now())
			if err != nil {
				return err
			}

			fitnessRecords, fitnessNewest, fitnessWarning, err := readCurrentWellnessFitness(client, flags, dateRange)
			if err != nil {
				return err
			}

			query := queryFromFlags(flags, "limit")
			query["oldest"] = dateRange.Oldest
			query["newest"] = dateRange.Newest
			query["fields"] = analysisActivityFields()

			var activities []icu.Activity
			if err := client.Get("activities", nil, query, &activities); err != nil {
				return wrapCommandError(err)
			}
			if err := enrichActivitiesDecoupling(client, activities, flags); err != nil {
				return err
			}

			tzInfo := analysisTimezone(explicit)
			analysis := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{
				Thresholds:            profileThresholds(),
				StartDate:             dateRange.Oldest,
				EndDate:               dateRange.Newest,
				Timezone:              tzInfo.timezone,
				TimezoneSource:        tzInfo.source,
				MinIncludedActivities: IntFlag(flags, "min-included-activities", 0),
			})
			icu.ApplyWellnessFitness(&analysis.Load, fitnessRecords, fitnessNewest)
			if fitnessWarning != "" {
				analysis.Warnings = append(analysis.Warnings, fitnessWarning)
			}

			return writeJSON(stdout, analysis)
		},
	}
}

// currentFitnessLookbackDays bounds the wellness read used to find the newest
// daily CTL/ATL row at or before the analysis end date.
const currentFitnessLookbackDays = 7

// readCurrentWellnessFitness reads the daily wellness rows used to replace
// activity-frozen CTL/ATL with the state at the range end, clamped to today
// in the athlete's timezone. Wellness is optional context here: a failed read
// returns no rows plus a warning for the JSON output, so the activity-derived
// values stay, labeled latestLoadSource "last_activity".
func readCurrentWellnessFitness(client *icu.Client, flags map[string]string, dateRange analysisRange) ([]icu.Wellness, string, string, error) {
	newest, err := clampDateToAthleteToday(client, flags, dateRange.Newest, time.Now())
	if err != nil {
		return nil, "", "", err
	}
	oldest, err := addDays(newest, -currentFitnessLookbackDays)
	if err != nil {
		return nil, newest, "", err
	}

	var records []icu.Wellness
	if err := client.Get("wellness", nil, dateRangeQuery(oldest, newest), &records); err != nil {
		return nil, newest, "wellness unavailable, CTL/ATL/TSB are from the last activity: " + err.Error(), nil //nolint:nilerr // wellness is optional context; the failure is reported as a warning
	}

	return records, newest, "", nil
}

// clampDateToAthleteToday clamps date to today in the athlete's timezone
// (--timezone, else the athlete profile, else the system zone).
func clampDateToAthleteToday(client *icu.Client, flags map[string]string, date string, now time.Time) (string, error) {
	location, _, err := resolveMicrocycleLocation(client, flags)
	if err != nil {
		return "", err
	}

	return clampDateToToday(date, now.In(location)), nil
}

func analysisPlanCommand() *Command {
	return &Command{
		Name: "",
		Usage: "analysis plan [--history-oldest DATE --history-newest DATE] " +
			"[--plan-oldest DATE --plan-newest DATE]",
		Description: "Analyze a planned training block from recent history and future calendar events. " +
			"Default ranges use UTC; pass explicit dates for athlete-local daily boundaries.",
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			dateRanges, explicit, err := trainingPlanDateRanges(flags, time.Now())
			if err != nil {
				return err
			}

			activityQuery := queryFromFlags(flags, "limit")
			activityQuery["oldest"] = dateRanges.History.Oldest
			activityQuery["newest"] = dateRanges.History.Newest
			activityQuery["fields"] = analysisActivityFields()

			var activities []icu.Activity
			if err := client.Get("activities", nil, activityQuery, &activities); err != nil {
				return wrapCommandError(err)
			}
			if err := enrichActivitiesDecoupling(client, activities, flags); err != nil {
				return err
			}

			sportType := icu.StringFlag(flags, "sport-type", "Ride")
			var sportSettings icu.SportSettings
			if err := client.Get("sport-settings", []string{sportType}, nil, &sportSettings); err != nil {
				return wrapCommandError(err)
			}

			wellnessNewest, err := clampDateToAthleteToday(client, flags, dateRanges.History.Newest, time.Now())
			if err != nil {
				return err
			}
			wellnessRecords, wellnessWarnings, err := readWellnessRecordsForAnalysis(client, dateRanges.History.Oldest, wellnessNewest, nil)
			if err != nil {
				return err
			}

			tzInfo := analysisTimezone(explicit)
			wellnessAnalysis := analyzeWellness(wellnessRecords, wellnessWarnings, icu.AnalysisOptions{
				Thresholds:     profileThresholds(),
				StartDate:      dateRanges.History.Oldest,
				EndDate:        wellnessNewest,
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
				Thresholds:       profileThresholds(),
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

			return writeJSON(stdout, analysis)
		},
	}
}

func analysisWellnessCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "analysis wellness [--oldest DATE --newest DATE | --days N]",
		Description: "Compute wellness and physiology analysis from wellness records. " +
			"Default ranges use UTC; pass explicit --oldest/--newest dates for athlete-local daily boundaries.",
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			dateRange, explicit, err := analysisDateRange(flags, time.Now())
			if err != nil {
				return err
			}

			records, warnings, err := readWellnessRecordsForAnalysis(client, dateRange.Oldest, dateRange.Newest, queryFromFlags(flags, "fields"))
			if err != nil {
				return err
			}

			tzInfo := analysisTimezone(explicit)
			analysis := analyzeWellness(records, warnings, icu.AnalysisOptions{
				Thresholds:     profileThresholds(),
				StartDate:      dateRange.Oldest,
				EndDate:        dateRange.Newest,
				Timezone:       tzInfo.timezone,
				TimezoneSource: tzInfo.source,
			})

			return writeJSON(stdout, analysis)
		},
	}
}

func analysisWorkoutCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "analysis workout <activity-id> [--event-id ID] [--calendar-id ID] [--sport-type TYPE] [--match-window-hours N]",
		Description: "Analyze one completed workout against its planned workout event, intervals, and streams.",
		Run: func(args []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			activityID := args[0]
			inputs, options, err := readWorkoutExecutionInputs(client, flags, activityID)
			if err != nil {
				return err
			}

			analysis := icu.AnalyzeWorkoutExecution(inputs, options)

			return writeJSON(stdout, analysis)
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
	if err := client.Get("activity", []string{activityID, activityStreamsPath}, streamQuery, &rawStreams); err != nil {
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
		if !isHTTPNotFound(err) {
			return inputs, options, wrapCommandError(err)
		}
	}
	inputs.SportSettings = &sportSettings

	var powerModel icu.PowerModel
	if err := client.Get("mmp-model", nil, map[string]string{"type": sportType}, &powerModel); err != nil {
		if !isHTTPNotFound(err) {
			return inputs, options, wrapCommandError(err)
		}
	}
	if powerModel.CriticalPower > 0 || powerModel.WPrime > 0 {
		inputs.PowerModel = &powerModel
	}

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
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			dateRange, explicit, err := analysisDateRange(flags, time.Now())
			if err != nil {
				return err
			}

			sportType := icu.StringFlag(flags, "type", "Ride")

			activityQuery := queryFromFlags(flags, "limit")
			activityQuery["oldest"] = dateRange.Oldest
			activityQuery["newest"] = dateRange.Newest
			activityQuery["fields"] = analysisActivityFields()

			var activities []icu.Activity
			if err := client.Get("activities", nil, activityQuery, &activities); err != nil {
				return wrapCommandError(err)
			}

			curveQuery := map[string]string{
				"type":   sportType,
				"curves": icu.StringFlag(flags, "curves", ""),
			}
			if curveQuery["curves"] == "" {
				windows, err := icu.AdaptationCurveWindows(dateRange.Oldest, dateRange.Newest)
				if err != nil {
					return fmt.Errorf("%w: %w", errInvalidValue, err)
				}
				curveQuery["curves"] = windows
			}

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

			wellnessRecords, wellnessWarnings, err := readWellnessRecordsForAnalysis(client, dateRange.Oldest, dateRange.Newest, nil)
			if err != nil {
				return err
			}

			tzInfo := analysisTimezone(explicit)
			wellnessAnalysis := analyzeWellness(wellnessRecords, wellnessWarnings, icu.AnalysisOptions{
				Thresholds:     profileThresholds(),
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
					Thresholds:     profileThresholds(),
					StartDate:      dateRange.Oldest,
					EndDate:        dateRange.Newest,
					Timezone:       tzInfo.timezone,
					TimezoneSource: tzInfo.source,
				},
			)

			return writeJSON(stdout, analysis)
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
	_, hasOldest := flags["history-oldest"]
	_, hasNewest := flags["history-newest"]
	_, hasDays := flags["history-days"]

	if hasOldest || hasNewest {
		if hasDays {
			return analysisRange{}, false, fmt.Errorf("%w: explicit history dates cannot be combined with --history-days", errMissingRequired)
		}
		if oldest == "" || newest == "" {
			return analysisRange{}, false, errMissing("--history-oldest and --history-newest")
		}
		if err := validateAnalysisRange("--history-oldest", oldest, "--history-newest", newest); err != nil {
			return analysisRange{}, false, err
		}

		return analysisRange{Oldest: oldest, Newest: newest}, true, nil
	}

	days, err := positiveIntFlag(flags, "history-days", defaultPlanHistoryDays)
	if err != nil {
		return analysisRange{}, false, err
	}

	end := now.UTC()
	start := end.AddDate(0, 0, -days+1)

	return analysisRange{Oldest: start.Format("2006-01-02"), Newest: end.Format("2006-01-02")}, false, nil
}

func trainingPlanFutureRange(flags map[string]string, now time.Time) (analysisRange, bool, error) {
	oldest := icu.StringFlag(flags, "plan-oldest", "")
	newest := icu.StringFlag(flags, "plan-newest", "")
	_, hasOldest := flags["plan-oldest"]
	_, hasNewest := flags["plan-newest"]
	_, hasDays := flags["plan-days"]

	if hasOldest || hasNewest {
		if hasDays {
			return analysisRange{}, false, fmt.Errorf("%w: explicit plan dates cannot be combined with --plan-days", errMissingRequired)
		}
		if oldest == "" || newest == "" {
			return analysisRange{}, false, errMissing("--plan-oldest and --plan-newest")
		}
		if err := validateAnalysisRange("--plan-oldest", oldest, "--plan-newest", newest); err != nil {
			return analysisRange{}, false, err
		}

		alignedOldest := alignToISOWeekStart(oldest)
		alignedNewest := alignToISOWeekEnd(newest)
		if err := validateAnalysisRange("aligned --plan-oldest", alignedOldest, "aligned --plan-newest", alignedNewest); err != nil {
			return analysisRange{}, false, err
		}

		return analysisRange{
			Oldest: alignedOldest,
			Newest: alignedNewest,
		}, true, nil
	}

	days, err := positiveIntFlag(flags, "plan-days", defaultPlanDays)
	if err != nil {
		return analysisRange{}, false, err
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

func alignToISOWeekStart(date string) string {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}

	weekday := int(parsed.Weekday())
	if weekday == 0 {
		weekday = calendarWeekDays
	}

	return parsed.AddDate(0, 0, -(weekday - 1)).Format("2006-01-02")
}

func alignToISOWeekEnd(date string) string {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}

	weekday := int(parsed.Weekday())
	if weekday == 0 {
		return date
	}

	return parsed.AddDate(0, 0, calendarWeekDays-weekday).Format("2006-01-02")
}

func analysisDateRange(flags map[string]string, now time.Time) (analysisRange, bool, error) {
	oldest := icu.StringFlag(flags, "oldest", "")
	newest := icu.StringFlag(flags, "newest", "")
	_, hasOldest := flags["oldest"]
	_, hasNewest := flags["newest"]
	_, hasDays := flags["days"]

	if hasOldest || hasNewest {
		if hasDays {
			return analysisRange{}, false, fmt.Errorf("%w: explicit dates cannot be combined with --days", errMissingRequired)
		}
		if oldest == "" || newest == "" {
			return analysisRange{}, false, errMissing("--oldest and --newest")
		}
		if err := validateAnalysisRange("--oldest", oldest, "--newest", newest); err != nil {
			return analysisRange{}, false, err
		}

		return analysisRange{Oldest: oldest, Newest: newest}, true, nil
	}

	days, err := positiveIntFlag(flags, "days", defaultAnalysisDays)
	if err != nil {
		return analysisRange{}, false, err
	}

	normalizedNow := now.UTC()
	end := normalizedNow.Format("2006-01-02")
	start := normalizedNow.AddDate(0, 0, -days+1).Format("2006-01-02")

	return analysisRange{Oldest: start, Newest: end}, false, nil
}

func positiveIntFlag(flags map[string]string, name string, defaultValue int) (int, error) {
	value, ok := flags[name]
	if !ok {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: --%s must be an integer", errInvalidValue, name)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%w: --%s must be greater than 0", errInvalidValue, name)
	}

	return parsed, nil
}

func validateAnalysisRange(oldestName, oldest, newestName, newest string) error {
	oldestDate, err := parseAnalysisDate(oldestName, oldest)
	if err != nil {
		return err
	}
	newestDate, err := parseAnalysisDate(newestName, newest)
	if err != nil {
		return err
	}
	if oldestDate.After(newestDate) {
		return fmt.Errorf("%w: %s must be on or before %s", errInvalidValue, oldestName, newestName)
	}

	return nil
}

func parseAnalysisDate(name, value string) (time.Time, error) {
	const layout = "2006-01-02"

	parsed, err := time.Parse(layout, value)
	if err != nil || parsed.Format(layout) != value {
		return time.Time{}, fmt.Errorf("%w: %s must use YYYY-MM-DD", errInvalidValue, name)
	}

	return parsed, nil
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
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			now := time.Now()
			location, timezoneSource, err := resolveMicrocycleLocation(client, flags)
			if err != nil {
				return err
			}
			dateRange, location, err := microcycleDateRange(flags, now, location)
			if err != nil {
				return err
			}
			wellnessNewest := clampDateToToday(dateRange.Newest, now.In(location))

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
				timezoneSource,
				wellnessNewest,
			)
			if err != nil {
				return err
			}

			sportType := icu.StringFlag(flags, "sport-type", "Ride")
			analysis := icu.AnalyzeMicrocycle(inputs.Activities, inputs.Events, inputs.Wellness, &inputs.SportSettings, &icu.MicrocycleOptions{
				StartDate:        dateRange.Oldest,
				EndDate:          dateRange.Newest,
				Timezone:         location.String(),
				TimezoneSource:   timezoneSource,
				Now:              now.In(location),
				PlanIncluded:     includePlan,
				WellnessIncluded: includeWellness,
				SportType:        sportType,
				Full:             BoolFlag(flags, "full"),
			})

			if BoolFlag(flags, "json") || icu.ResolveOutputFormat(flags) == icu.FormatJSON {
				return writeJSON(stdout, analysis)
			}

			return writeMicrocycleHuman(stdout, &analysis)
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
	wellnessNewest string,
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
		wellness, err := readMicrocycleWellness(client, wellnessNewest, lookbackStart, timezone, timezoneSource)
		if err != nil {
			return inputs, err
		}
		inputs.Wellness = wellness
	}

	sportType := icu.StringFlag(flags, "sport-type", "Ride")
	if err := client.Get("sport-settings", []string{sportType}, nil, &inputs.SportSettings); err != nil {
		if isHTTPNotFound(err) {
			return inputs, nil
		}

		return inputs, wrapCommandError(err)
	}

	return inputs, nil
}

const httpStatusNotFound = 404

func isHTTPNotFound(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), fmt.Sprintf("HTTP status error %d", httpStatusNotFound))
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
	activityQuery["fields"] = analysisActivityFields()

	var activities []icu.Activity
	if err := client.Get("activities", nil, activityQuery, &activities); err != nil {
		return nil, wrapCommandError(err)
	}

	if err := enrichActivitiesDecoupling(client, activities, flags); err != nil {
		return nil, err
	}

	return activities, nil
}

func readMicrocycleEvents(client *icu.Client, flags map[string]string, dateRange analysisRange) ([]icu.Event, error) {
	// Flag parser normalizes to hyphen keys ("calendar-id"); queryFromFlags
	// then maps them to snake_case API params ("calendar_id").
	eventQuery := queryFromFlags(flags, "calendar-id")
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

// readMicrocycleWellness reads wellness only up to newest, which callers clamp
// to today: Intervals.icu returns projected CTL/ATL rows for future dates, and
// the latest row drives readiness state.
func readMicrocycleWellness(
	client *icu.Client,
	newest string,
	lookbackStart string,
	timezone string,
	timezoneSource string,
) (*icu.WellnessAnalysis, error) {
	wellnessRecords, warnings, err := readWellnessRecordsForAnalysis(client, lookbackStart, newest, nil)
	if err != nil {
		return nil, err
	}

	analysis := analyzeWellness(wellnessRecords, warnings, icu.AnalysisOptions{
		Thresholds:     profileThresholds(),
		StartDate:      lookbackStart,
		EndDate:        newest,
		Timezone:       timezone,
		TimezoneSource: timezoneSource,
	})

	return &analysis, nil
}

func microcycleDateRange(flags map[string]string, now time.Time, location *time.Location) (analysisRange, *time.Location, error) {
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

// resolveMicrocycleLocation applies timezone precedence: --timezone flag,
// then the athlete profile timezone, then the system timezone. The athlete
// profile is only fetched when no flag is given; a failed or empty lookup
// falls back to the system timezone so the source label stays truthful.
func resolveMicrocycleLocation(client *icu.Client, flags map[string]string) (*time.Location, string, error) {
	flagName := icu.StringFlag(flags, "timezone", "")
	athleteName := ""
	if flagName == "" {
		var athlete icu.Athlete
		if err := client.Get("athlete", nil, nil, &athlete); err == nil {
			athleteName = athlete.Timezone
		}
	}

	return microcycleLocation(flagName, athleteName, time.Now().Location())
}

func microcycleLocation(flagName, athleteName string, system *time.Location) (*time.Location, string, error) {
	if flagName != "" {
		location, err := time.LoadLocation(flagName)
		if err != nil {
			return nil, "", fmt.Errorf("%w: invalid --timezone %s", errInvalidValue, flagName)
		}

		return location, "flag", nil
	}
	if athleteName != "" {
		if location, err := time.LoadLocation(athleteName); err == nil {
			return location, "athlete", nil
		}
	}

	return system, "system", nil
}

// clampDateToToday returns the earlier of date and today's local date.
func clampDateToToday(date string, localNow time.Time) string {
	today := localNow.Format("2006-01-02")
	if date > today {
		return today
	}

	return date
}

func isoWeekRange(date string, location *time.Location) (analysisRange, *time.Location, error) {
	parsed, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		return analysisRange{}, nil, fmt.Errorf("%w: invalid date %s", errInvalidValue, date)
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
		return fmt.Errorf("%w: invalid --from %s", errInvalidValue, from)
	}
	if endErr != nil {
		return fmt.Errorf("%w: invalid --to %s", errInvalidValue, to)
	}
	if end.Before(start) {
		return fmt.Errorf("%w: --from must be on or before --to", errInvalidValue)
	}

	return nil
}

func addDays(date string, days int) (string, error) {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("%w: invalid date %s", errInvalidValue, date)
	}

	return parsed.AddDate(0, 0, days).Format("2006-01-02"), nil
}

func writeMicrocycleHuman(stdout io.Writer, analysis *icu.MicrocycleAnalysis) error {
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

	return writeOutput(stdout, []byte(output.String()))
}
