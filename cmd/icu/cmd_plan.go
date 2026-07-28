package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
			"[--allow-today] [--allow-past] [--now-date DATE] [--type SPORT]",
		Description: "Build an editable multi-session plan proposal from intent JSON without mutating Intervals.icu.",
		Schema:      planShowSchema(),
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			input, err := readPlanInput(flags, client)
			if err != nil {
				return err
			}
			proposal := icu.BuildPlanProposal(&input)

			return writePlanProposal(flags, &proposal)
		},
	}
}

func planPreviewCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "plan preview --file PATH [--no-live-check]",
		Description: "Validate a plan proposal and print a non-mutating operation preview.",
		Schema:      planPreviewSchema(),
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			proposal, err := readPlanProposal(flags)
			if err != nil {
				return err
			}
			validation := icu.ValidatePlanProposal(&proposal)
			var blockingErr error
			if validation.Blocking {
				blockingErr = fmt.Errorf("validate plan proposal: %s", validation.Errors[0])
			}

			return runCalendarOpsPreview(
				flags,
				proposal.Operations,
				calendarValidationFromPlan(validation),
				validation.Blocking,
				blockingErr,
				func() []string {
					warnings := liveCheckCalendarOperations(client, proposal.Operations)
					if err := checkPlanBaselineDrift(client, &proposal, flags); err != nil {
						warnings = append(warnings, err.Error())
					}

					return warnings
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
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
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

			return writeJSON(proposal.Apply)
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
		}, commonAuthFlags()...),
	}
}

func planPreviewSchema() *CommandSchema {
	return &CommandSchema{
		RejectPositionals: true,
		Flags: append([]CommandFlag{
			{Name: "file", ValueName: "PATH", Description: "Proposal JSON path."},
			{Name: "no-live-check", ValueName: "BOOL", Description: "Skip live source-hash and baseline drift checks.", Kind: commandFlagBoolean},
		}, commonAuthFlags()...),
	}
}

func planAcceptSchema() *CommandSchema {
	return &CommandSchema{
		RejectPositionals: true,
		Flags: append([]CommandFlag{
			{Name: "file", ValueName: "PATH", Description: "Proposal JSON path."},
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
	activities, err := readRebalanceActivities(client, oldest, newest, flags)
	if err != nil {
		return icu.PlanInput{}, err
	}
	events, err := readRebalanceEvents(client, oldest, newest)
	if err != nil {
		return icu.PlanInput{}, err
	}
	sportType := icu.StringFlag(flags, "type", intent.Constraints.DefaultType)
	sportSettings, err := readPlanSportSettings(client, sportType)
	if err != nil {
		return icu.PlanInput{}, err
	}

	return icu.PlanInput{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		NowDate:       icu.StringFlag(flags, "now-date", newest),
		Activities:    activities,
		Events:        events,
		SportSettings: sportSettings,
		Intent:        intent,
		AllowToday:    BoolFlag(flags, "allow-today"),
		AllowPast:     BoolFlag(flags, "allow-past"),
	}, nil
}

func readPlanSportSettings(client *icu.Client, sportType string) (*icu.SportSettings, error) {
	if sportType == "" {
		return &icu.SportSettings{}, nil
	}
	var settings icu.SportSettings
	if err := client.Get("sport-settings", []string{sportType}, nil, &settings); err != nil {
		return nil, wrapCommandError(err)
	}

	return &settings, nil
}

func readPlanIntent(flags map[string]string) (icu.PlanIntent, error) {
	file := icu.StringFlag(flags, "intent-file", "")
	if file == "" {
		return icu.PlanIntent{}, errMissing("intent-file")
	}
	data, err := os.ReadFile(file)
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
	data, err := os.ReadFile(file)
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
	if err := os.WriteFile(file, data, 0o600); err != nil {
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
