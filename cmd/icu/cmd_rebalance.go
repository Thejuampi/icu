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
		Usage: "rebalance --dry-run --file PATH --oldest DATE --newest DATE " +
			"[--target-load N] [--target-tolerance N] [--type SPORT] [--target TARGET] " +
			"[--start-time HH:MM] [--min-session-minutes N] [--duration-step-minutes N] " +
			"[--allocation-basis explicit_equal] [--allow-today] [--allow-past] " +
			"[--wellness-lookback-days N] [--now-date DATE]",
		Description: "Write an editable weekly load rebalance proposal without mutating Intervals.icu.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
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
				check := icu.ComputeRebalanceApprove(&proposal, proposal.Approve.Reason, proposal.Approve.ProvidedLimits)
				if check.RecalcHash != "" && check.RecalcHash != proposal.Approve.RecalcHash {
					return errors.New("rebalance approval hash mismatch: proposal changed after approve")
				}
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
	if !BoolFlag(flags, "dry-run") {
		return icu.RebalanceInput{}, errMissing("dry-run")
	}
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
		Description: "Bind a rebalance proposal to an approval reason and explicit limits, recording hashes for accept verification.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			proposal, err := readRebalanceProposal(flags)
			if err != nil {
				return err
			}
			reason := icu.StringFlag(flags, "reason", "")
			if reason == "" {
				return errMissing("reason")
			}
			if flags["target-load"] != "" {
				if load, err := strconv.Atoi(flags["target-load"]); err == nil {
					proposal.Constraints.TargetLoad = load
				}
			}
			if flags["level"] != "" {
				proposal.Constraints.Level = flags["level"]
			}
			if flags["mode"] != "" {
				proposal.Constraints.Mode = flags["mode"]
			}
			providedLimits := flags["target-load"] != "" && flags["level"] != ""
			if proposal.Envelope != nil && proposal.Envelope.OutsideEnvelope && !providedLimits {
				return errors.New("baseline outside envelope: approve requires explicit --target-load and --level limits")
			}
			approve := icu.ComputeRebalanceApprove(&proposal, reason, providedLimits)
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
