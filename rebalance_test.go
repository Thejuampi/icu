package icu_test

import (
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestDynamicRebalanceTargetsUseSportPowerZones(t *testing.T) {
	t.Parallel()

	input := icu.RebalanceInput{SportSettings: &icu.SportSettings{PowerZones: []int{55, 75, 90}}}

	targets := icu.DynamicRebalanceTargets(&input)

	if targets.Z1IF != 0.55 || targets.Z2IF != 0.65 {
		t.Fatalf("targets = %.3f/%.3f, want 0.550/0.650", targets.Z1IF, targets.Z2IF)
	}
}

func TestDynamicRebalanceTargetsIgnoreIntensityOutlier(t *testing.T) {
	t.Parallel()

	input := icu.RebalanceInput{Activities: []icu.Activity{
		{Type: "Ride", Intensity: 0.62},
		{Type: "Ride", Intensity: 0.64},
		{Type: "Ride", Intensity: 0.65},
		{Type: "Ride", Intensity: 0.66},
		{Type: "Ride", Intensity: 2.00},
	}}

	targets := icu.DynamicRebalanceTargets(&input)

	if targets.MaxIntensity != 0.655 {
		t.Fatalf("max intensity = %.3f, want 0.655", targets.MaxIntensity)
	}
}

func TestDynamicRebalanceTargetsDeriveLongRideFromHistory(t *testing.T) {
	t.Parallel()

	input := icu.RebalanceInput{Activities: []icu.Activity{
		{Type: "Ride", MovingTime: 70 * 60},
		{Type: "Ride", MovingTime: 80 * 60},
		{Type: "Ride", MovingTime: 90 * 60},
		{Type: "Ride", MovingTime: 100 * 60},
		{Type: "Ride", MovingTime: 300 * 60},
	}}

	targets := icu.DynamicRebalanceTargets(&input)

	if targets.LongRideMinutes != 100 {
		t.Fatalf("long ride minutes = %d, want 100", targets.LongRideMinutes)
	}
}

func TestDynamicRebalanceTargetsDeriveDailyLoadLimitFromHistory(t *testing.T) {
	t.Parallel()

	input := icu.RebalanceInput{Activities: []icu.Activity{
		{Type: "Ride", StartDateLocal: "2026-06-01T08:00:00", TrainingLoad: 50},
		{Type: "Ride", StartDateLocal: "2026-06-02T08:00:00", TrainingLoad: 60},
		{Type: "Ride", StartDateLocal: "2026-06-03T08:00:00", TrainingLoad: 70},
		{Type: "Ride", StartDateLocal: "2026-06-04T08:00:00", TrainingLoad: 80},
		{Type: "Ride", StartDateLocal: "2026-06-05T08:00:00", TrainingLoad: 1000},
	}}

	targets := icu.DynamicRebalanceTargets(&input)

	if targets.DailyLoadLimit != 75 {
		t.Fatalf("daily load limit = %d, want 75", targets.DailyLoadLimit)
	}
}

func TestDynamicRebalanceTargetsHandlesNilInput(t *testing.T) {
	t.Parallel()

	targets := icu.DynamicRebalanceTargets(nil)

	if targets.Source != "fallback_no_sport_zones_or_history" {
		t.Fatalf("source = %q, want fallback", targets.Source)
	}
}

func TestBuildRebalanceProposalBuildsFeasibleZ2Updates(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.TargetTolerance = 20

	proposal := icu.BuildRebalanceProposal(&input)

	if !proposal.Options[0].Evaluation.Feasible {
		t.Fatalf("feasible = false, target delta = %d", proposal.Options[0].Evaluation.TargetDelta)
	}
}

func TestBuildRebalanceProposalEvaluationMatchesOperationLoads(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()

	proposal := icu.BuildRebalanceProposal(&input)

	operationLoad := rebalanceEvaluatedOperationLoad(&proposal)
	if proposal.Options[0].Evaluation.WeeklyLoad != operationLoad {
		t.Fatalf("weekly load = %d, want operation total %d", proposal.Options[0].Evaluation.WeeklyLoad, operationLoad)
	}
}

func TestBuildRebalanceProposalLocksCompletedEvents(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].EventID == 1 {
		t.Fatalf("first operation updated completed event %d", proposal.Operations[0].EventID)
	}
}

func TestBuildRebalanceProposalDoesNotDoubleCountCompletedEventDate(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Baseline.LockedPlannedLoad != 0 {
		t.Fatalf("locked planned load = %d, want 0", proposal.Baseline.LockedPlannedLoad)
	}
}

func TestValidateRebalanceProposalRejectsMissingUpdateEventID(t *testing.T) {
	t.Parallel()

	proposal := icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Operations: []icu.RebalanceOperation{{
			ID:     "bad-update",
			Action: icu.RebalanceActionUpdate,
			Status: icu.RebalanceStatusPending,
			Body:   icu.EventEx{Name: "Workout"},
		}},
	}

	validation := icu.ValidateRebalanceProposal(&proposal)

	if !validation.Blocking {
		t.Fatalf("validation blocking = false, want true")
	}
}

func TestBuildRebalanceProposalCancelsMutableEventsWhenTargetAlreadyMet(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.TargetLoad = input.Activities[0].TrainingLoad

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].Action != icu.RebalanceActionCancel {
		t.Fatalf("first action = %q, want cancel", proposal.Operations[0].Action)
	}
}

func TestBuildRebalanceProposalAllowsTodayWhenConfigured(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.AllowToday = true

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].EventID != 1 {
		t.Fatalf("first event id = %d, want 1", proposal.Operations[0].EventID)
	}
}

func TestBuildRebalanceProposalHonorsMaxSessionMinutes(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.TargetLoad = 500
	input.Constraints.MaxSessionMinutes = 30

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].ExpectedMovingTimeSecs != 1800 {
		t.Fatalf("moving time = %d, want 1800", proposal.Operations[0].ExpectedMovingTimeSecs)
	}
}

func TestBuildRebalanceProposalHonorsMaxIntensity(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.MaxIntensity = 0.60

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].ExpectedIF != 0.60 {
		t.Fatalf("expected IF = %.2f, want 0.60", proposal.Operations[0].ExpectedIF)
	}
}

func TestBuildRebalanceProposalHonorsMaxWatts(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.MaxWatts = 171

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].ExpectedIF != 0.60 {
		t.Fatalf("expected IF = %.2f, want 0.60", proposal.Operations[0].ExpectedIF)
	}
}

func TestBuildRebalanceProposalCountsMutableEventsWhenTheyCannotChange(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.AllowUpdate = false
	input.Constraints.AllowCancel = false

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Options[0].Evaluation.WeeklyLoad != 231 {
		t.Fatalf("weekly load = %d, want 231", proposal.Options[0].Evaluation.WeeklyLoad)
	}
}

func TestBuildRebalanceProposalPreservesUnusedMutableWhenCancelDisabled(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.TargetLoad = 87
	input.Constraints.AllowCancel = false

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Options[0].Evaluation.WeeklyLoad != 118 {
		t.Fatalf("weekly load = %d, want 118", proposal.Options[0].Evaluation.WeeklyLoad)
	}
}

func TestBuildRebalanceProposalHandlesNilInput(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildRebalanceProposal(nil)

	if proposal.SchemaVersion != icu.RebalanceSchemaVersion {
		t.Fatalf("schema = %q, want %q", proposal.SchemaVersion, icu.RebalanceSchemaVersion)
	}
}

func TestMarshalRebalanceProposalWritesPrettyJSON(t *testing.T) {
	t.Parallel()

	proposal := icu.RebalanceProposalFile{SchemaVersion: icu.RebalanceSchemaVersion}

	data, err := icu.MarshalRebalanceProposal(&proposal)
	if err != nil || !strings.Contains(string(data), "\n") {
		t.Fatalf("marshal err = %v, data = %q", err, string(data))
	}
}

func TestRebalanceEventHashHandlesNil(t *testing.T) {
	t.Parallel()

	hash := icu.RebalanceEventHash(nil)

	if hash != "" {
		t.Fatalf("hash = %q, want empty", hash)
	}
}

func TestRebalanceEventHashHandlesEvent(t *testing.T) {
	t.Parallel()

	hash := icu.RebalanceEventHash(&icu.Event{ID: 7, Name: "Ride", StartDateLocal: "2026-06-25T00:00:00"})

	if len(hash) != 12 {
		t.Fatalf("hash length = %d, want 12", len(hash))
	}
}

func TestValidateRebalanceProposalRejectsNil(t *testing.T) {
	t.Parallel()

	validation := icu.ValidateRebalanceProposal(nil)

	if !validation.Blocking {
		t.Fatalf("validation blocking = false, want true")
	}
}

func TestValidateRebalanceProposalRejectsUnsupportedAction(t *testing.T) {
	t.Parallel()

	proposal := icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Operations: []icu.RebalanceOperation{{
			ID:     "bad-action",
			Action: "move",
			Status: icu.RebalanceStatusPending,
		}},
	}

	validation := icu.ValidateRebalanceProposal(&proposal)

	if !validation.Blocking {
		t.Fatalf("validation blocking = false, want true")
	}
}

func TestValidateRebalanceProposalRejectsMissingCreateBodyName(t *testing.T) {
	t.Parallel()

	proposal := icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Operations: []icu.RebalanceOperation{{
			ID:     "bad-create",
			Action: icu.RebalanceActionCreate,
			Status: icu.RebalanceStatusPending,
		}},
	}

	validation := icu.ValidateRebalanceProposal(&proposal)

	if !validation.Blocking {
		t.Fatalf("validation blocking = false, want true")
	}
}

func TestDynamicRebalanceTargetsFallbackIsLabeled(t *testing.T) {
	t.Parallel()

	targets := icu.DynamicRebalanceTargets(&icu.RebalanceInput{})

	if targets.Source != "fallback_no_sport_zones_or_history" {
		t.Fatalf("source = %q, want fallback", targets.Source)
	}
}

func rebalanceFixture() icu.RebalanceInput {
	return icu.RebalanceInput{
		AthleteID: "i1",
		NowDate:   "2026-06-24",
		Activities: []icu.Activity{
			{Type: "Ride", StartDateLocal: "2026-06-24T18:00:00", TrainingLoad: 46, Intensity: 0.55, MovingTime: 5400},
		},
		Events: []icu.Event{
			{ID: 1, Category: "WORKOUT", Type: "Ride", StartDateLocal: "2026-06-24T00:00:00", Name: "Done", TrainingLoad: 40, MovingTime: 3600},
			{ID: 2, Category: "WORKOUT", Type: "Ride", StartDateLocal: "2026-06-25T00:00:00", Name: "Tempo", TrainingLoad: 65, MovingTime: 4320, Intensity: 0.76},
			{ID: 3, Category: "WORKOUT", Type: "Ride", StartDateLocal: "2026-06-27T00:00:00", Name: "Long", TrainingLoad: 120, MovingTime: 9000, Intensity: 0.68},
		},
		SportSettings: &icu.SportSettings{FTP: 285, IndoorFTP: 285, PowerZones: []int{55, 75, 90}},
		Scope: icu.RebalanceScope{
			StartDate: "2026-06-24",
			EndDate:   "2026-06-28",
			Week:      "2026-W26",
		},
		Constraints: icu.RebalanceConstraints{
			TargetLoad:  220,
			AllowCreate: true,
			AllowUpdate: true,
			AllowCancel: true,
		},
	}
}

func rebalanceEvaluatedOperationLoad(proposal *icu.RebalanceProposalFile) int {
	total := proposal.Options[0].Evaluation.CompletedLoad + proposal.Options[0].Evaluation.LockedPlannedLoad
	for index := range proposal.Operations {
		if proposal.Operations[index].Action != icu.RebalanceActionCancel {
			total += proposal.Operations[index].ExpectedLoad
		}
	}

	return total
}
