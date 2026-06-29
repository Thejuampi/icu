package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	icu "github.com/Thejuampi/icu"
)

const rebalanceWellnessLookbackDays = 42

func registerRebalanceCommands(registry *CommandRegistry) {
	registry.Register("rebalance", "show", rebalanceDryRunCommand())
	registry.Register("rebalance", "accept", rebalanceAcceptCommand())
	registry.Register("rebalance", "approve", rebalanceApproveCommand())
}

func rebalanceDryRunCommand() *Command {
	return &Command{
		Name: "",
		Usage: "rebalance show --file PATH --oldest DATE --newest DATE " +
			"[--target-load N] [--target-tolerance N] [--type SPORT] [--target TARGET] " +
			"[--start-time HH:MM] [--min-session-minutes N] [--duration-step-minutes N] " +
			"[--allocation-basis explicit_equal] [--allow-today] [--allow-past] " +
			"[--wellness-lookback-days N] [--now-date DATE]",
		Description: "Write an editable weekly load rebalance proposal without mutating Intervals.icu.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			flags["dry-run"] = "true"
			input, err := readRebalanceInput(flags, client)
			if err != nil {
				return err
			}
			proposal := icu.BuildRebalanceProposal(&input)

			return writeRebalanceProposal(flags, &proposal)
		},
	}
}

func rebalanceAcceptCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "rebalance accept --file PATH",
		Description: "Apply pending create/update/cancel operations from a rebalance proposal file.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			proposal, err := readRebalanceProposal(flags)
			if err != nil {
				return err
			}
			validation := icu.ValidateRebalanceProposal(&proposal)
			if validation.Blocking {
				return fmt.Errorf("validate rebalance proposal: %s", validation.Errors[0])
			}
			if proposal.Envelope != nil && proposal.Envelope.OutsideEnvelope && (proposal.Approve == nil || !proposal.Approve.Verified) {
				return errors.New("outside-envelope proposal requires `icu rebalance approve --file --reason --target-load --level` before accept")
			}
			if proposal.Approve != nil {
				checkProposal := proposal
				applyApprovedLimits(&checkProposal)
				check := icu.ComputeRebalanceApprove(&checkProposal, proposal.Approve.Reason, proposal.Approve.ProvidedLimits)
				if check.RecalcHash != "" && check.RecalcHash != proposal.Approve.RecalcHash {
					return errors.New("rebalance approval hash mismatch: proposal changed after approve")
				}
				if check.LimitsHash != "" && check.LimitsHash != proposal.Approve.LimitsHash {
					return errors.New("rebalance approval limits mismatch: approved limits changed after approve")
				}
			}

			// Drift detection: fetch current activities and events for the scope week
			currentActivities, err := readRebalanceActivities(client, proposal.Scope.StartDate, proposal.Scope.EndDate, flags)
			if err != nil {
				return err
			}
			currentEvents, err := readRebalanceEvents(client, proposal.Scope.StartDate, proposal.Scope.EndDate)
			if err != nil {
				return err
			}
			driftInput := icu.RebalanceInput{
				Activities:  currentActivities,
				Events:      currentEvents,
				Constraints: proposal.Constraints,
				Scope:       proposal.Scope,
				NowDate:     proposal.Scope.EndDate,
			}
			currentBaseline := icu.RebalanceBaseline(&driftInput)
			if currentBaseline.CompletedLoad != proposal.Baseline.CompletedLoad ||
				currentBaseline.LockedPlannedLoad != proposal.Baseline.LockedPlannedLoad {
				return fmt.Errorf(
					"calendar drift detected: completed %d→%d or locked %d→%d; re-run rebalance show and approve",
					proposal.Baseline.CompletedLoad, currentBaseline.CompletedLoad,
					proposal.Baseline.LockedPlannedLoad, currentBaseline.LockedPlannedLoad,
				)
			}

			proposal.Apply = applyRebalanceProposal(client, &proposal)
			if err := writeRebalanceProposal(flags, &proposal); err != nil {
				return err
			}

			return writeJSON(proposal.Apply)
		},
	}
}

func readRebalanceInput(flags map[string]string, client *icu.Client) (icu.RebalanceInput, error) {
	oldest := icu.StringFlag(flags, "oldest", "")
	newest := icu.StringFlag(flags, "newest", "")
	if oldest == "" {
		return icu.RebalanceInput{}, errMissing("oldest")
	}
	if newest == "" {
		return icu.RebalanceInput{}, errMissing("newest")
	}
	if _, ok := flags["max-hr"]; ok {
		return icu.RebalanceInput{}, errors.New("max-hr is not supported for power rebalance sessions")
	}
	if target := icu.StringFlag(flags, "target", ""); target != "" && target != "POWER" {
		return icu.RebalanceInput{}, fmt.Errorf("target %q is not supported for rebalance sessions; use POWER", target)
	}
	activities, err := readRebalanceActivities(client, oldest, newest, flags)
	if err != nil {
		return icu.RebalanceInput{}, err
	}
	events, err := readRebalanceEvents(client, oldest, newest)
	if err != nil {
		return icu.RebalanceInput{}, err
	}
	sportSettings, err := readRebalanceSportSettings(client, flags)
	if err != nil {
		return icu.RebalanceInput{}, err
	}
	wellness, err := readRebalanceWellness(client, oldest, newest, flags)
	if err != nil {
		return icu.RebalanceInput{}, err
	}

	return icu.RebalanceInput{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		NowDate:       icu.StringFlag(flags, "now-date", newest),
		Activities:    activities,
		Events:        events,
		SportSettings: sportSettings,
		Wellness:      wellness,
		Scope: icu.RebalanceScope{
			StartDate:      oldest,
			EndDate:        newest,
			Week:           icu.StringFlag(flags, "week", ""),
			Timezone:       icu.DefaultAnalysisTimezone,
			TimezoneSource: "explicit",
		},
		Request: icu.RebalanceRequest{
			Strategy: icu.StringFlag(flags, "strategy", "target-preserving-z2"),
			DryRun:   true,
			File:     icu.StringFlag(flags, "file", ""),
		},
		Constraints: rebalanceConstraintsFromFlags(flags),
	}, nil
}

func rebalanceApproveCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "rebalance approve --file PATH --reason TEXT [--target-load N] [--level X] [--mode MODE]",
		Description: "Recalculate the proposal under explicit limits and approved constraints, then bind approval hashes for accept verification.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			proposal, err := readRebalanceProposal(flags)
			if err != nil {
				return err
			}
			reason := icu.StringFlag(flags, "reason", "")
			if reason == "" {
				return errMissing("reason")
			}
			providedLimits := flags["target-load"] != "" && flags["level"] != ""
			if proposal.Envelope != nil && proposal.Envelope.OutsideEnvelope && !providedLimits {
				return errors.New("baseline outside envelope: approve requires explicit --target-load and --level limits")
			}

			approvedTargetLoad := proposal.Constraints.TargetLoad
			if flags["target-load"] != "" {
				if load, err := strconv.Atoi(flags["target-load"]); err == nil {
					approvedTargetLoad = load
				}
			}
			approvedLevel := proposal.Constraints.Level
			if flags["level"] != "" {
				approvedLevel = flags["level"]
			}
			approvedMode := proposal.Constraints.Mode
			if flags["mode"] != "" {
				approvedMode = flags["mode"]
			}

			// When a real client is available and the proposal needs recalculation
			// (outside-envelope or explicit overrides), re-fetch data and regenerate.
			if client != nil && (providedLimits || (proposal.Envelope != nil && proposal.Envelope.OutsideEnvelope)) {
				regenInput, err := readRebalanceInputFromScope(client, proposal.Scope.StartDate, proposal.Scope.EndDate, approvedTargetLoad, approvedLevel, approvedMode)
				if err != nil {
					return err
				}
				regenInput.Request.Strategy = proposal.Request.Strategy
				if regenInput.Request.Strategy == "" {
					regenInput.Request.Strategy = icu.RebalanceStrategyAdaptiveBidirectional
				}
				regenInput.Request.DryRun = proposal.Request.DryRun
				regenInput.Constraints.Approved = true

				regen := icu.BuildRebalanceProposal(&regenInput)

				proposal.Operations = regen.Operations
				proposal.Options = regen.Options
				proposal.Projection = regen.Projection
				proposal.Envelope = regen.Envelope
				proposal.Baseline = regen.Baseline
				proposal.Context = regen.Context
				proposal.Validation = regen.Validation
				proposal.Notes = regen.Notes
				proposal.Policy = regen.Policy
			}

			approve := icu.ComputeRebalanceApprove(&proposal, reason, providedLimits)
			approve.TargetLoad = approvedTargetLoad
			approve.Level = approvedLevel
			approve.Mode = approvedMode
			proposal.Approve = &approve

			if err := writeRebalanceProposal(flags, &proposal); err != nil {
				return err
			}
			return writeJSON(approve)
		},
	}
}

func readRebalanceActivities(client *icu.Client, oldest, newest string, flags map[string]string) ([]icu.Activity, error) {
	query := queryFromFlags(flags, "limit")
	query["oldest"] = oldest
	query["newest"] = newest
	query["fields"] = icu.StringFlag(flags, "activity-fields", defaultAnalysisFields)
	var activities []icu.Activity
	if err := client.Get("activities", nil, query, &activities); err != nil {
		return nil, wrapCommandError(err)
	}

	return activities, nil
}

func readRebalanceEvents(client *icu.Client, oldest, newest string) ([]icu.Event, error) {
	query := map[string]string{"oldest": oldest, "newest": newest}
	var events []icu.Event
	if err := client.Get("events", nil, query, &events); err != nil {
		return nil, wrapCommandError(err)
	}

	return events, nil
}

func readRebalanceSportSettings(client *icu.Client, flags map[string]string) (*icu.SportSettings, error) {
	sport := icu.StringFlag(flags, "type", "")
	if sport == "" {
		return &icu.SportSettings{}, nil
	}
	var settings icu.SportSettings
	if err := client.Get("sport-settings", []string{sport}, nil, &settings); err != nil {
		return nil, wrapCommandError(err)
	}

	return &settings, nil
}

func readRebalanceWellness(client *icu.Client, oldest, newest string, flags map[string]string) (*icu.WellnessAnalysis, error) {
	wellnessOldest := rebalanceWellnessStart(oldest, IntFlag(flags, "wellness-lookback-days", rebalanceWellnessLookbackDays))
	query := map[string]string{"oldest": wellnessOldest, "newest": newest}
	var records []icu.Wellness
	if err := client.Get("wellness", nil, query, &records); err != nil {
		return nil, wrapCommandError(err)
	}
	analysis := icu.AnalyzeWellness(records, icu.AnalysisOptions{
		StartDate:      wellnessOldest,
		EndDate:        newest,
		Timezone:       icu.DefaultAnalysisTimezone,
		TimezoneSource: "explicit",
	})

	return &analysis, nil
}

func rebalanceConstraintsFromFlags(flags map[string]string) icu.RebalanceConstraints {
	constraints := icu.DefaultRebalanceConstraints()
	constraints.AllowToday = BoolFlag(flags, "allow-today")
	constraints.AllowPast = BoolFlag(flags, "allow-past")
	constraints.TargetLoad = IntFlag(flags, "target-load", 0)
	constraints.TargetTolerance = IntFlag(flags, "target-tolerance", 0)
	constraints.SportType = icu.StringFlag(flags, "type", "")
	constraints.WorkoutTarget = icu.StringFlag(flags, "target", "")
	constraints.StartTime = icu.StringFlag(flags, "start-time", "")
	constraints.MinSessionMinutes = IntFlag(flags, "min-session-minutes", 0)
	constraints.DurationStepMinutes = IntFlag(flags, "duration-step-minutes", 0)
	constraints.AllocationBasis = icu.StringFlag(flags, "allocation-basis", "")
	constraints.MaxSessionMinutes = IntFlag(flags, "max-session-minutes", 0)
	constraints.MaxWatts = IntFlag(flags, "max-watts", 0)
	constraints.Z1IF = floatFlagVal(flags, "z1-if", 0)
	constraints.Z2IF = floatFlagVal(flags, "z2-if", 0)
	constraints.MaxIntensity = floatFlagVal(flags, "max-intensity", 0)
	constraints.Note = icu.StringFlag(flags, "note", "")
	constraints.Level = icu.StringFlag(flags, "level", "")
	constraints.Mode = icu.StringFlag(flags, "mode", "")

	return constraints
}

func readRebalanceProposal(flags map[string]string) (icu.RebalanceProposalFile, error) {
	file := icu.StringFlag(flags, "file", "")
	if file == "" {
		return icu.RebalanceProposalFile{}, errMissing("file")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return icu.RebalanceProposalFile{}, fmt.Errorf("read rebalance file: %w", err)
	}
	var proposal icu.RebalanceProposalFile
	if err := unmarshalRebalanceProposal(data, &proposal); err != nil {
		return icu.RebalanceProposalFile{}, fmt.Errorf("parse rebalance file: %w", err)
	}

	return proposal, nil
}

func unmarshalRebalanceProposal(data []byte, proposal *icu.RebalanceProposalFile) error {
	if err := json.Unmarshal(data, proposal); err != nil {
		return fmt.Errorf("unmarshal rebalance proposal: %w", err)
	}

	return nil
}

func writeRebalanceProposal(flags map[string]string, proposal *icu.RebalanceProposalFile) error {
	file := icu.StringFlag(flags, "file", "")
	if file == "" {
		return errMissing("file")
	}
	data, err := icu.MarshalRebalanceProposal(proposal)
	if err != nil {
		return fmt.Errorf("marshal rebalance file: %w", err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return fmt.Errorf("write rebalance file: %w", err)
	}

	return nil
}

func applyRebalanceProposal(client *icu.Client, proposal *icu.RebalanceProposalFile) *icu.RebalanceApplySummary {
	summary := &icu.RebalanceApplySummary{}
	for index := range proposal.Operations {
		operation := &proposal.Operations[index]
		result := applyRebalanceOperation(client, operation)
		summary.Results = append(summary.Results, result)
		countRebalanceApplyResult(summary, result)
	}

	return summary
}

func applyRebalanceOperation(client *icu.Client, operation *icu.RebalanceOperation) icu.RebalanceApplyResult {
	result := icu.RebalanceApplyResult{OperationID: operation.ID, Action: operation.Action, EventID: operation.EventID}
	if operation.Status != "" && operation.Status != icu.RebalanceStatusPending {
		operation.Status = icu.RebalanceStatusSkipped
		result.Status = icu.RebalanceStatusSkipped

		return result
	}
	if err := verifyRebalanceSourceHash(client, operation); err != nil {
		operation.Status = icu.RebalanceStatusFailed
		operation.Error = err.Error()
		result.Status = icu.RebalanceStatusFailed
		result.Error = err.Error()

		return result
	}
	var event icu.Event
	err := applyRebalanceMutation(client, operation, &event)
	if err != nil {
		operation.Status = icu.RebalanceStatusFailed
		operation.Error = err.Error()
		result.Status = icu.RebalanceStatusFailed
		result.Error = err.Error()

		return result
	}
	operation.Status = icu.RebalanceStatusApplied
	operation.AppliedEventID = appliedRebalanceEventID(operation, &event)
	result.Status = icu.RebalanceStatusApplied
	result.EventID = operation.AppliedEventID

	return result
}

func verifyRebalanceSourceHash(client *icu.Client, operation *icu.RebalanceOperation) error {
	if operation.Action == icu.RebalanceActionCreate || operation.SourceHash == "" {
		return nil
	}
	var current icu.Event
	if err := client.Get("events", []string{strconv.Itoa(operation.EventID)}, nil, &current); err != nil {
		return wrapCommandError(err)
	}
	if got := icu.RebalanceEventHash(&current); got != operation.SourceHash {
		return fmt.Errorf("event %d changed since dry-run", operation.EventID)
	}

	return nil
}

func applyRebalanceMutation(client *icu.Client, operation *icu.RebalanceOperation, event *icu.Event) error {
	switch operation.Action {
	case icu.RebalanceActionCreate:
		return wrapCommandError(client.Post("events", nil, nil, operation.Body, event))
	case icu.RebalanceActionUpdate:
		return wrapCommandError(client.Put("events", []string{strconv.Itoa(operation.EventID)}, nil, operation.Body, event))
	case icu.RebalanceActionCancel:
		return wrapCommandError(client.Delete("events", []string{strconv.Itoa(operation.EventID)}, nil, event))
	default:
		return fmt.Errorf("unsupported rebalance action %q", operation.Action)
	}
}

func appliedRebalanceEventID(operation *icu.RebalanceOperation, event *icu.Event) int {
	if event.ID != 0 {
		return event.ID
	}

	return operation.EventID
}

func countRebalanceApplyResult(summary *icu.RebalanceApplySummary, result icu.RebalanceApplyResult) {
	switch result.Status {
	case icu.RebalanceStatusApplied:
		summary.Applied++
	case icu.RebalanceStatusSkipped:
		summary.Skipped++
	default:
		summary.Failed++
	}
}

func applyApprovedLimits(proposal *icu.RebalanceProposalFile) {
	if proposal == nil || proposal.Approve == nil {
		return
	}
	if proposal.Approve.TargetLoad > 0 {
		proposal.Constraints.TargetLoad = proposal.Approve.TargetLoad
	}
	if proposal.Approve.Level != "" {
		proposal.Constraints.Level = proposal.Approve.Level
	}
	if proposal.Approve.Mode != "" {
		proposal.Constraints.Mode = proposal.Approve.Mode
	}
}

func readRebalanceInputFromScope(client *icu.Client, startDate, endDate string, targetLoad int, level, mode string) (icu.RebalanceInput, error) {
	activities, err := readRebalanceActivities(client, startDate, endDate, map[string]string{"activity-fields": defaultAnalysisFields, "limit": "100"})
	if err != nil {
		return icu.RebalanceInput{}, err
	}
	events, err := readRebalanceEvents(client, startDate, endDate)
	if err != nil {
		return icu.RebalanceInput{}, err
	}
	// Derive sport type from events
	sportType := ""
	for index := range events {
		if events[index].Category == "WORKOUT" && events[index].Type != "" {
			sportType = events[index].Type
			break
		}
	}
	var sportSettings *icu.SportSettings
	if sportType != "" {
		var settings icu.SportSettings
		if err := client.Get("sport-settings", []string{sportType}, nil, &settings); err == nil {
			sportSettings = &settings
		}
	}
	var wellnessAnalysis *icu.WellnessAnalysis
	wellnessOldest := rebalanceWellnessStart(startDate, rebalanceWellnessLookbackDays)
	var wellnessRecords []icu.Wellness
	if err := client.Get("wellness", nil, map[string]string{"oldest": wellnessOldest, "newest": endDate}, &wellnessRecords); err == nil {
		analysis := icu.AnalyzeWellness(wellnessRecords, icu.AnalysisOptions{
			StartDate:      wellnessOldest,
			EndDate:        endDate,
			Timezone:       icu.DefaultAnalysisTimezone,
			TimezoneSource: "explicit",
		})
		wellnessAnalysis = &analysis
	}

	return icu.RebalanceInput{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		NowDate:       endDate,
		Activities:    activities,
		Events:        events,
		SportSettings: sportSettings,
		Wellness:      wellnessAnalysis,
		Scope: icu.RebalanceScope{
			StartDate:      startDate,
			EndDate:        endDate,
			Week:           "",
			Timezone:       icu.DefaultAnalysisTimezone,
			TimezoneSource: "explicit",
		},
		Request: icu.RebalanceRequest{
			Strategy: icu.RebalanceStrategyAdaptiveBidirectional,
			DryRun:   true,
		},
		Constraints: icu.RebalanceConstraints{
			TargetLoad:          targetLoad,
			Level:               level,
			Mode:                mode,
			AllowCreate:         true,
			AllowUpdate:         true,
			AllowCancel:         true,
			TargetTolerance:     0,
			MinSessionMinutes:   0,
			DurationStepMinutes: 0,
		},
	}, nil
}

func rebalanceWellnessStart(oldest string, lookbackDays int) string {
	parsed, err := time.Parse("2006-01-02", oldest)
	if err != nil {
		return oldest
	}
	if lookbackDays <= 0 {
		lookbackDays = rebalanceWellnessLookbackDays
	}

	return parsed.AddDate(0, 0, -lookbackDays).Format("2006-01-02")
}
