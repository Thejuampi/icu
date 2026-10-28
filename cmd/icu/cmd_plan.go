package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	icu "github.com/Thejuampi/icu"
)

func registerPlanCommands(registry *CommandRegistry) {
	registry.Register("plan", "show", planShowCommand())
	registry.Register("plan", "preview", planPreviewCommand())
	registry.Register("plan", "accept", planAcceptCommand())
}

func planShowCommand() *Command {
	return &Command{
		Name: "",
		Usage: "plan show --file PATH --intent-file PATH " +
			"[--allow-today] [--allow-past] [--now-date DATE] [--type SPORT] " +
			"[--session-environment indoor|outdoor]",
		Description: "Build an editable multi-session plan proposal from intent JSON without mutating Intervals.icu.",
		Schema:      planShowSchema(),
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout io.Writer) error {
			input, err := readPlanInput(flags, client)
			if err != nil {
				return err
			}
			proposal := icu.BuildPlanProposal(&input)
			proposal.Validation.Warnings = append(proposal.Validation.Warnings,
				planProfileWarnings(&proposal, input.SportSettings)...)

			return writePlanProposal(flags, &proposal)
		},
	}
}

func planPreviewCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "plan preview --file PATH [--no-live-check] [--max-ramp CTL/WEEK] [--now-date DATE] [--timezone TZ] [--save-learned]",
		Description: "Validate a plan proposal and print a non-mutating operation preview.",
		Schema:      planPreviewSchema(),
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout io.Writer) error {
			proposal, err := readPlanProposal(flags)
			if err != nil {
				return err
			}
			validation := icu.ValidatePlanProposal(&proposal)
			validation.Warnings = append(validation.Warnings, planProfileWarnings(&proposal, nil)...)
			var blockingErr error
			if validation.Blocking {
				blockingErr = fmt.Errorf("validate plan proposal: %s", validation.Errors[0])
			}

			return runCalendarOpsPreviewWith(stdout,
				flags,
				proposal.Operations,
				calendarValidationFromPlan(validation),
				validation.Blocking,
				blockingErr,
				func(preview *icu.CalendarOpsPreview) []string {
					warnings := liveCheckCalendarOperations(client, proposal.Operations)
					if err := checkPlanBaselineDrift(client, &proposal, flags); err != nil {
						warnings = append(warnings, err.Error())
					}
					projection, err := planLoadProjection(client, &proposal, flags, time.Now())
					if err != nil {
						return append(warnings, "projection: "+err.Error())
					}
					preview.Projection = projection

					return append(warnings, projection.Warnings...)
				},
			)
		},
	}
}

func planAcceptCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "plan accept --file PATH",
		Description: "Apply pending create/update/cancel operations from a plan proposal file.",
		Schema:      planAcceptSchema(),
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout io.Writer) error {
			proposal, err := readPlanProposal(flags)
			if err != nil {
				return err
			}
			validation := icu.ValidatePlanProposal(&proposal)
			if validation.Blocking {
				return fmt.Errorf("validate plan proposal: %s", validation.Errors[0])
			}
			if err := checkPlanBaselineDrift(client, &proposal, flags); err != nil {
				return err
			}
			proposal.Apply = applyCalendarOperations(client, proposal.Operations)
			preview := icu.PreviewCalendarOperations(proposal.Operations)
			proposal.Preview = &preview
			if err := writePlanProposal(flags, &proposal); err != nil {
				return err
			}

			return writeJSON(stdout, proposal.Apply)
		},
	}
}

func planShowSchema() *CommandSchema {
	return &CommandSchema{
		RejectPositionals: true,
		Flags: append([]CommandFlag{
			{Name: "file", ValueName: "PATH", Description: "Output proposal JSON path."},
			{Name: "intent-file", ValueName: "PATH", Description: "Intent JSON path."},
			{Name: "allow-today", ValueName: "BOOL", Description: "Allow mutating today's planned sessions.", Kind: commandFlagBoolean},
			{Name: "allow-past", ValueName: "BOOL", Description: "Allow mutating past planned sessions.", Kind: commandFlagBoolean},
			{Name: "now-date", ValueName: "DATE", Description: "Lock boundary date (YYYY-MM-DD)."},
			{Name: "type", ValueName: "SPORT", Description: "Sport type for FTP/settings lookup."},
			{
				Name: "session-environment", ValueName: "indoor|outdoor",
				Description: "Explicit FTP environment override for sessions with no per-record signal (D-10c); " +
					"overrides the intent file's own \"sessionEnvironment\" constraint. Distinct from --indoor on " +
					"events/ftp/sports: those set a record's own flag, this overrides which FTP number a proposal " +
					"prices sessions at when the record leaves it ambiguous.",
			},
		}, commonAuthFlags()...),
	}
}

func planPreviewSchema() *CommandSchema {
	return &CommandSchema{
		RejectPositionals: true,
		Flags: append([]CommandFlag{
			{Name: "file", ValueName: "PATH", Description: proposalFileDescription},
			{Name: "no-live-check", ValueName: "BOOL", Description: "Skip live source-hash, baseline drift and load projection.", Kind: commandFlagBoolean},
			{Name: "max-ramp", ValueName: "CTL/WEEK", Description: "Projected CTL ramp cap; overrides profile maxRampPerWeek."},
			{Name: "now-date", ValueName: "DATE", Description: "Projection 'today' (YYYY-MM-DD); default is today in the athlete timezone."},
			{Name: "timezone", ValueName: "TZ", Description: "IANA timezone for 'today'; default is the athlete profile timezone."},
			{Name: "save-learned", ValueName: "BOOL", Description: "Save the learned CTL/ATL-only activity types in the local profile (default: use them in memory only).", Kind: commandFlagBoolean},
		}, commonAuthFlags()...),
	}
}

func planAcceptSchema() *CommandSchema {
	return &CommandSchema{
		RejectPositionals: true,
		Flags: append([]CommandFlag{
			{Name: "file", ValueName: "PATH", Description: proposalFileDescription},
		}, commonAuthFlags()...),
	}
}

func readPlanInput(flags map[string]string, client *icu.Client) (icu.PlanInput, error) {
	intent, err := readPlanIntent(flags)
	if err != nil {
		return icu.PlanInput{}, err
	}
	if intent.Oldest == "" || intent.Newest == "" {
		// BuildPlanProposal also derives bounds; seed here so fetches have a range.
		seed := icu.BuildPlanProposal(&icu.PlanInput{Intent: intent})
		if intent.Oldest == "" {
			intent.Oldest = seed.Scope.StartDate
		}
		if intent.Newest == "" {
			intent.Newest = seed.Scope.EndDate
		}
	}
	oldest := intent.Oldest
	newest := intent.Newest
	if oldest == "" || newest == "" {
		return icu.PlanInput{}, errors.New("intent oldest and newest are required (or provide session dates)")
	}
	if err := validateAnalysisRange("intent.oldest", oldest, "intent.newest", newest); err != nil {
		return icu.PlanInput{}, err
	}
	nowDate := icu.StringFlag(flags, "now-date", newest)
	// Stage 6 R11: plan's lock decision (dateLockedForPlan -> dateLocked ->
	// rebalanceDateLockedByTime) reaches the same raw string comparison
	// against NowDate that rebalance does; validate here for the same reason.
	if _, err := parseAnalysisDate("--now-date", nowDate); err != nil {
		return icu.PlanInput{}, err
	}
	sessionEnvironment, err := sessionEnvironmentFromFlags(flags)
	if err != nil {
		return icu.PlanInput{}, err
	}
	if sessionEnvironment != "" {
		intent.Constraints.SessionEnvironment = sessionEnvironment
	}
	activities, err := readRebalanceActivities(client, oldest, newest, flags)
	if err != nil {
		return icu.PlanInput{}, err
	}
	events, err := readRebalanceEvents(client, oldest, newest)
	if err != nil {
		return icu.PlanInput{}, err
	}
	sportType := planSettingsSportType(icu.StringFlag(flags, "type", intent.Constraints.DefaultType), &intent)
	sportSettings, ftpWarnings, err := readPlanSportSettings(client, sportType)
	if err != nil {
		return icu.PlanInput{}, err
	}

	return icu.PlanInput{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		NowDate:       nowDate,
		Activities:    activities,
		Events:        events,
		SportSettings: sportSettings,
		Warnings:      ftpWarnings,
		Intent:        intent,
		AllowToday:    BoolFlag(flags, "allow-today"),
		AllowPast:     BoolFlag(flags, "allow-past"),
	}, nil
}

// planSettingsSportType picks the sport whose settings price the plan. Without
// --type or constraints.defaultType it falls back to the first typed session,
// so sport-settings (ftp and indoor_ftp) are still fetched instead of
// leaving every session with no FTP.
func planSettingsSportType(explicit string, intent *icu.PlanIntent) string {
	if explicit != "" {
		return explicit
	}
	for index := range intent.Sessions {
		if intent.Sessions[index].Type != "" {
			return intent.Sessions[index].Type
		}
	}

	return ""
}

func readPlanSportSettings(client *icu.Client, sportType string) (*icu.SportSettings, []string, error) {
	settings := &icu.SportSettings{}
	if sportType != "" {
		if err := client.Get("sport-settings", []string{sportType}, nil, settings); err != nil {
			return nil, nil, wrapCommandError(err)
		}
	}
	profile, err := loadTrainingProfile()
	if err != nil {
		return nil, nil, err
	}
	merged, warnings := icu.MergeProfileFTP(settings, profile)

	return merged, warnings, nil
}

func readPlanIntent(flags map[string]string) (icu.PlanIntent, error) {
	file := icu.StringFlag(flags, "intent-file", "")
	if file == "" {
		return icu.PlanIntent{}, errMissing("intent-file")
	}
	data, err := os.ReadFile(file) //nolint:gosec // G703: the user names this file on the command line; using it is the point.
	if err != nil {
		return icu.PlanIntent{}, fmt.Errorf("read plan intent file: %w", err)
	}
	var intent icu.PlanIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return icu.PlanIntent{}, fmt.Errorf("parse plan intent file: %w", err)
	}
	if intent.SchemaVersion != "" && intent.SchemaVersion != icu.PlanIntentSchemaVersion {
		return icu.PlanIntent{}, fmt.Errorf("unsupported plan intent schemaVersion: %s", intent.SchemaVersion)
	}

	return intent, nil
}

func readPlanProposal(flags map[string]string) (icu.PlanProposalFile, error) {
	file := icu.StringFlag(flags, "file", "")
	if file == "" {
		return icu.PlanProposalFile{}, errMissing("file")
	}
	data, err := os.ReadFile(file) //nolint:gosec // G703: the user names this file on the command line; using it is the point.
	if err != nil {
		return icu.PlanProposalFile{}, fmt.Errorf("read plan file: %w", err)
	}
	var proposal icu.PlanProposalFile
	if err := json.Unmarshal(data, &proposal); err != nil {
		return icu.PlanProposalFile{}, fmt.Errorf("parse plan file: %w", err)
	}

	return proposal, nil
}

func writePlanProposal(flags map[string]string, proposal *icu.PlanProposalFile) error {
	file := icu.StringFlag(flags, "file", "")
	if file == "" {
		return errMissing("file")
	}
	data, err := icu.MarshalPlanProposal(proposal)
	if err != nil {
		return fmt.Errorf("marshal plan file: %w", err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil { //nolint:gosec // G703: the user names this file on the command line; using it is the point.
		return fmt.Errorf("write plan file: %w", err)
	}

	return nil
}

func checkPlanBaselineDrift(client *icu.Client, proposal *icu.PlanProposalFile, flags map[string]string) error {
	currentActivities, err := readRebalanceActivities(client, proposal.Scope.StartDate, proposal.Scope.EndDate, flags)
	if err != nil {
		return err
	}
	currentEvents, err := readRebalanceEvents(client, proposal.Scope.StartDate, proposal.Scope.EndDate)
	if err != nil {
		return err
	}
	input := icu.PlanInput{
		NowDate:    proposal.Scope.NowDate,
		Activities: currentActivities,
		Events:     currentEvents,
		Intent: icu.PlanIntent{
			Oldest:      proposal.Scope.StartDate,
			Newest:      proposal.Scope.EndDate,
			Constraints: proposal.Constraints,
		},
	}
	if input.NowDate == "" {
		input.NowDate = proposal.Scope.EndDate
	}
	current := icu.BuildPlanProposal(&input).Baseline
	if current.CompletedLoad != proposal.Baseline.CompletedLoad ||
		current.LockedPlannedLoad != proposal.Baseline.LockedPlannedLoad {
		return fmt.Errorf(
			"calendar drift detected: completed %d→%d or locked %d→%d; re-run plan show",
			proposal.Baseline.CompletedLoad, current.CompletedLoad,
			proposal.Baseline.LockedPlannedLoad, current.LockedPlannedLoad,
		)
	}

	return nil
}

// planProfileWarnings validates a proposal against the local training
// profile and runs the weekly monotony check. A missing profile skips the
// profile checks; an unreadable config is reported instead of silently
// skipping them.
func planProfileWarnings(proposal *icu.PlanProposalFile, settings *icu.SportSettings) []string {
	profile, err := loadTrainingProfile()
	if err != nil {
		return []string{"profile: " + err.Error()}
	}
	maxSimilar := 0
	if profile != nil {
		maxSimilar = profile.MaxSimilarSessionsPerWeek
	}

	return append(icu.ValidatePlanAgainstProfile(proposal, profile, settings),
		icu.PlanMonotonyWarnings(proposal.Sessions, maxSimilar)...)
}

// projectionLookbackDays bounds the wellness and activity reads: the start
// row, the real CTL of the days before it (rolling ramp), and enough history
// to infer which sports intervals.icu leaves out of CTL.
const projectionLookbackDays = 90

// planLoadProjection projects CTL/ATL/TSB from the last wellness row before
// today through the proposal's end date. Today is --now-date, else today in
// the athlete timezone. Days before today use completed activity load, today
// uses the larger of completed and planned load, later days use existing
// calendar workouts plus pending operations. The ramp cap comes from
// --max-ramp, else the profile; with no cap the ramp is reported but never
// flagged. All reads are GETs.
func planLoadProjection(
	client *icu.Client, proposal *icu.PlanProposalFile, flags map[string]string, now time.Time,
) (*icu.LoadProjection, error) {
	today, err := projectionToday(client, flags, now)
	if err != nil {
		return nil, err
	}
	oldest, err := addDays(today, -projectionLookbackDays)
	if err != nil {
		return nil, err
	}
	var records []icu.Wellness
	if err := client.Get("wellness", nil, dateRangeQuery(oldest, today), &records); err != nil {
		return nil, wrapCommandError(err)
	}
	yesterday, err := addDays(today, -1)
	if err != nil {
		return nil, err
	}
	start, ok := icu.LatestWellnessFitness(records, yesterday)
	if !ok {
		if start, ok = icu.LatestWellnessFitness(records, today); !ok {
			return nil, fmt.Errorf("no wellness CTL/ATL on or before %s", today)
		}
	}
	known := map[string]float64{}
	for index := range records {
		if records[index].CTL != 0 || records[index].ATL != 0 {
			known[records[index].ID] = records[index].CTL
		}
	}
	end := proposal.Scope.EndDate
	if end == "" || end < start.ID {
		end = start.ID
	}
	firstDay, err := addDays(start.ID, 1)
	if err != nil {
		return nil, err
	}
	var activities []icu.Activity
	activityQuery := dateRangeQuery(oldest, today)
	activityQuery["fields"] = analysisActivityFields() + ",paired_event_id"
	if err := client.Get("activities", nil, activityQuery, &activities); err != nil {
		return nil, wrapCommandError(err)
	}
	completed := map[int]bool{}
	for index := range activities {
		if activities[index].PairedEventID != nil {
			completed[*activities[index].PairedEventID] = true
		}
	}
	events, err := readRebalanceEvents(client, firstDay, end)
	if err != nil {
		return nil, err
	}
	profile, err := loadTrainingProfile()
	if err != nil {
		return nil, err
	}
	if profile == nil {
		profile = &icu.TrainingProfile{}
	}
	excluded, typeWarnings := planFitnessLoadTypes(
		profile, records, activities, proposal.Operations, events, firstDay, BoolFlag(flags, "save-learned"),
	)
	actual, actualFitness := projectionActualLoads(records, activities, excluded, firstDay, today)
	maxRamp := floatFlagVal(flags, "max-ramp", profile.MaxRampPerWeek)
	planned, plannedFitness := icu.PlanProjectedDailyLoadsByModel(proposal.Operations, events, excluded, completed)
	projection, err := icu.ProjectLoad(&icu.LoadProjectionInput{
		StartDate: start.ID, StartSource: "wellness", StartCTL: start.CTL, StartATL: start.ATL,
		EndDate: end, Today: today, KnownCTL: known, ActualLoads: actual,
		PlannedLoads: planned, PlannedFitnessLoads: plannedFitness, ActualFitnessLoads: actualFitness,
		CTLDays: profile.CTLDays, ATLDays: profile.ATLDays, MaxRampPerWeek: maxRamp,
	})
	if err != nil {
		return nil, fmt.Errorf("project load: %w", err)
	}
	projection.Warnings = append(projection.Warnings, typeWarnings...)

	return &projection, nil
}

// planFitnessLoadTypes learns which activity types feed CTL from the
// wellness ctlLoad history, merges them in memory with the classes saved in
// the local profile (writing them back only when save is set, i.e.
// --save-learned) and resolves them with the profile's manual overrides. Types the
// projection needs (activities from firstDay on, planned sessions) with no
// class get a warning.
func planFitnessLoadTypes(
	profile *icu.TrainingProfile, records []icu.Wellness, activities []icu.Activity,
	operations []icu.CalendarOperation, events []icu.Event, firstDay string, save bool,
) (map[string]bool, []string) {
	learned, conflicts := icu.ClassifyFitnessLoadTypes(icu.LearnFitnessLoadTypes(records, activities))
	var warnings []string
	merged := map[string]string{}
	for sport, class := range profile.LearnedFitnessLoadTypes {
		merged[sport] = class
	}
	changed := false
	for sport, class := range learned {
		if merged[sport] != class {
			merged[sport], changed = class, true
		}
	}
	// A type now seen both ways drops its stale saved class.
	for _, sport := range conflicts {
		if _, ok := merged[sport]; ok {
			delete(merged, sport)
			changed = true
		}
	}
	if changed && save {
		if err := saveLearnedFitnessLoadTypes(merged); err != nil {
			warnings = append(warnings, "fitness load: could not save learned types: "+err.Error())
		}
	}
	var needed []string
	for index := range activities {
		if activities[index].StartDateLocal >= firstDay {
			needed = append(needed, activities[index].Type)
		}
	}
	for index := range operations {
		if operations[index].Action != icu.CalendarActionCancel {
			needed = append(needed, operations[index].Body.Type)
		}
	}
	for index := range events {
		if events[index].Category == "WORKOUT" {
			needed = append(needed, events[index].Type)
		}
	}
	excluded, resolveWarnings := icu.ResolveFitnessLoadTypes(profile.FitnessLoadTypes, nil, merged, conflicts, needed)

	return excluded, append(warnings, resolveWarnings...)
}

// saveLearnedFitnessLoadTypes writes the learned classes into the local
// config profile (local file only; nothing is sent to Intervals.icu).
func saveLearnedFitnessLoadTypes(learned map[string]string) error {
	cfg, err := icu.LoadConfig()
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &icu.Config{}
	}
	if cfg.Profile == nil {
		cfg.Profile = &icu.TrainingProfile{}
	}
	cfg.Profile.LearnedFitnessLoadTypes = learned

	return icu.SaveConfigWithAction(cfg, "profile_learn_fitness_load")
}

func projectionToday(client *icu.Client, flags map[string]string, now time.Time) (string, error) {
	if nowDate := icu.StringFlag(flags, "now-date", ""); nowDate != "" {
		if _, err := parseAnalysisDate("--now-date", nowDate); err != nil {
			return "", err
		}

		return nowDate, nil
	}

	return clampDateToAthleteToday(client, flags, "9999-12-31", now)
}

// projectionActualLoads returns completed ATL and CTL loads per day from
// oldest through newest. Days before newest use the wellness atlLoad/ctlLoad
// intervals.icu computed (effective loads, including manual overrides and
// fatigue-only sports); otherwise the activities' icu_training_load is summed,
// leaving fitness-excluded types out of the CTL side.
func projectionActualLoads(
	records []icu.Wellness, activities []icu.Activity, excluded map[string]bool, oldest, newest string,
) (map[string]float64, map[string]float64) {
	loads, fitness := map[string]float64{}, map[string]float64{}
	for index := range activities {
		date := activities[index].StartDateLocal
		if len(date) < len("2006-01-02") {
			continue
		}
		date = date[:len("2006-01-02")]
		if date < oldest || date > newest {
			continue
		}
		load := float64(activities[index].TrainingLoad)
		loads[date] += load
		if !excluded[activities[index].Type] {
			fitness[date] += load
		}
	}
	for index := range records {
		row := &records[index]
		if row.ID >= oldest && row.ID < newest && (row.ATLLoad != 0 || row.CTLLoad != 0) {
			loads[row.ID], fitness[row.ID] = row.ATLLoad, row.CTLLoad
		}
	}

	return loads, fitness
}

// dateRangeQuery is the oldest/newest query shared by read-only range GETs.
func dateRangeQuery(oldest, newest string) map[string]string {
	return map[string]string{"oldest": oldest, "newest": newest}
}
