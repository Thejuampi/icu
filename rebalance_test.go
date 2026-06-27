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

	if targets.Source != "missing_intensity_basis" {
		t.Fatalf("source = %q, want missing intensity basis", targets.Source)
	}
}

func TestBuildRebalanceProposalBlocksWithoutIntensityBasis(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.SportSettings = nil
	input.Constraints.Z1IF = 0
	input.Constraints.Z2IF = 0

	proposal := icu.BuildRebalanceProposal(&input)

	if !proposal.Validation.Blocking {
		t.Fatalf("blocking = false, want true")
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

	if proposal.Operations[0].EventID != 2 {
		t.Fatalf("first event id = %d, want 2", proposal.Operations[0].EventID)
	}
}

func TestBuildRebalanceProposalLocksPastEventsByDefaultWithoutCompletedActivity(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Activities = nil
	input.Constraints.AllowCreate = false
	input.Constraints.TargetLoad = 240

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].EventID != 2 {
		t.Fatalf("first event id = %d, want 2", proposal.Operations[0].EventID)
	}
}

func TestBuildRebalanceProposalAllowsPastWhenConfigured(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Activities = nil
	input.Constraints.AllowCreate = false
	input.Constraints.AllowPast = true
	input.Constraints.TargetLoad = 240
	input.NowDate = "2026-06-25"

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

func TestBuildRebalanceProposalCreateUsesExplicitStartTime(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.AllowedDays = []string{"sat"}
	input.Constraints.StartTime = "06:30"

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].Body.StartDateLocal != "2026-06-27T06:30:00" {
		t.Fatalf("start date = %q, want explicit start time", proposal.Operations[0].Body.StartDateLocal)
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

func TestBuildRebalanceProposalUsesMutableEventWeightsWhenCancelDisabled(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.TargetLoad = 87
	input.Constraints.AllowCancel = false

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Options[0].Evaluation.WeeklyLoad != 96 {
		t.Fatalf("weekly load = %d, want 96", proposal.Options[0].Evaluation.WeeklyLoad)
	}
}

func TestBuildRebalanceProposalCreatesOnlyOnAllowedDays(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.AllowedDays = []string{"sat"}

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].Body.StartDateLocal != "2026-06-27T07:00:00" {
		t.Fatalf("start date = %q, want saturday", proposal.Operations[0].Body.StartDateLocal)
	}
}

func TestBuildRebalanceProposalSkipsOffDays(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.OffDays = []string{"wed", "thu", "fri"}

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].Body.StartDateLocal != "2026-06-27T07:00:00" {
		t.Fatalf("start date = %q, want saturday", proposal.Operations[0].Body.StartDateLocal)
	}
}

func TestBuildRebalanceProposalWarnsWhenNoSlotsAreAvailable(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.NowDate = input.Scope.EndDate

	proposal := icu.BuildRebalanceProposal(&input)

	if len(proposal.Options[0].Warnings) != 1 {
		t.Fatalf("warning count = %d, want 1", len(proposal.Options[0].Warnings))
	}
}

func TestBuildRebalanceProposalIgnoresNoteEvents(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Events = append(input.Events, icu.Event{ID: 9, Category: "NOTE", StartDateLocal: "2026-06-25T00:00:00", Name: "Travel"})

	proposal := icu.BuildRebalanceProposal(&input)

	if rebalanceOperationCount(proposal.Operations, icu.RebalanceActionCancel) != 0 {
		t.Fatalf("cancel count = %d, want 0", rebalanceOperationCount(proposal.Operations, icu.RebalanceActionCancel))
	}
}

func TestBuildRebalanceProposalDoesNotEmitFixedScore(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Options[0].Evaluation.Score != 0 {
		t.Fatalf("score = %d, want 0", proposal.Options[0].Evaluation.Score)
	}
}

func TestBuildRebalanceProposalDerivesTargetToleranceFromHistory(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.TargetTolerance = 0
	input.Activities = rebalanceHistoryActivities()

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Constraints.TargetTolerance != 10 {
		t.Fatalf("target tolerance = %d, want 10", proposal.Constraints.TargetTolerance)
	}
	if proposal.Context.DynamicTargets.TargetToleranceSource != "historical_weekly_load_mad" {
		t.Fatalf("target tolerance source = %q, want historical", proposal.Context.DynamicTargets.TargetToleranceSource)
	}
}

func TestBuildRebalanceProposalUsesExplicitTargetToleranceSource(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.TargetTolerance = 12

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Context.DynamicTargets.TargetToleranceSource != "explicit_target_tolerance" {
		t.Fatalf("target tolerance source = %q, want explicit", proposal.Context.DynamicTargets.TargetToleranceSource)
	}
}

func TestBuildRebalanceProposalDerivesStartTimeFromHistory(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.StartTime = ""
	input.Activities = rebalanceHistoryActivities()

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].Body.StartDateLocal != "2026-06-24T06:30:00" {
		t.Fatalf("start date = %q, want historical start time", proposal.Operations[0].Body.StartDateLocal)
	}
}

func TestBuildRebalanceProposalDerivesDurationStepFromHistory(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.DurationStepMinutes = 0
	input.Activities = rebalanceHistoryActivities()

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Constraints.DurationStepMinutes != 60 {
		t.Fatalf("duration step = %d, want 60", proposal.Constraints.DurationStepMinutes)
	}
}

func TestBuildRebalanceProposalUpdatePreservesEventTime(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.AllowCreate = false
	input.Events[1].StartDateLocal = "2026-06-25T18:30:00"

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].Body.StartDateLocal != "2026-06-25T18:30:00" {
		t.Fatalf("start date = %q, want existing event time", proposal.Operations[0].Body.StartDateLocal)
	}
	session := proposal.Options[0].Sessions[0]
	if session.StartTimeSource != "existing_event_local_time" || session.SportTypeSource != "existing_event_type" {
		t.Fatalf("session sources = %+v, want existing event sources", session)
	}
}

func TestBuildRebalanceProposalUsesHistoricalDayWeights(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.AllocationBasis = ""
	input.Activities = rebalanceDayWeightActivities()

	proposal := icu.BuildRebalanceProposal(&input)

	if len(proposal.Operations) != 5 {
		t.Fatalf("operation count = %d, want 5", len(proposal.Operations))
	}
}

func TestBuildRebalanceProposalPreservesEventTarget(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.WorkoutTarget = ""
	input.Constraints.AllowCreate = false
	input.Events[1].Target = "PACE"

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Operations[0].Body.Target != "PACE" {
		t.Fatalf("target = %q, want PACE", proposal.Operations[0].Body.Target)
	}
}

func TestBuildRebalanceProposalBlocksMissingSportTypeForCreate(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.SportType = ""

	proposal := icu.BuildRebalanceProposal(&input)

	if !proposal.Validation.Blocking {
		t.Fatalf("blocking = false, want true")
	}
}

func TestBuildRebalanceProposalBlocksMissingWorkoutTargetForCreate(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	input.Constraints.WorkoutTarget = ""

	proposal := icu.BuildRebalanceProposal(&input)

	if !proposal.Validation.Blocking {
		t.Fatalf("blocking = false, want true")
	}
}

func TestBuildRebalanceProposalCarriesDecisionSourcesInSession(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()

	proposal := icu.BuildRebalanceProposal(&input)
	session := proposal.Options[0].Sessions[0]

	if session.StartTimeSource == "" ||
		session.SportTypeSource == "" ||
		session.WorkoutTargetSource == "" ||
		session.IntensitySource == "" ||
		session.DurationSource == "" ||
		session.AllocationSource == "" ||
		session.ClassificationSource == "" {
		t.Fatalf("session decision sources incomplete: %+v", session)
	}
}

func TestBuildRebalanceProposalUsesWorkoutDocLoad(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Events[1].TrainingLoad = 0
	input.Events[1].WorkoutDoc = icu.WorkoutDoc{Steps: []icu.WorkoutStep{{Duration: 3600, Power: &icu.WorkoutTarget{Value: 65, Units: "%ftp"}}}}

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.Baseline.MutablePlannedLoad != 162 {
		t.Fatalf("mutable planned load = %d, want 162", proposal.Baseline.MutablePlannedLoad)
	}
}

func TestValidateRebalanceProposalRejectsCreateWithoutStartTime(t *testing.T) {
	t.Parallel()

	input := rebalanceCreateFixture()
	proposal := icu.BuildRebalanceProposal(&input)
	proposal.Operations[0].Body.StartDateLocal = proposal.Operations[0].Body.StartDateLocal[:10]

	validation := icu.ValidateRebalanceProposal(&proposal)

	if !validation.Blocking {
		t.Fatalf("blocking = false, want true")
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

func TestValidateRebalanceProposalRejectsUnsupportedStrategy(t *testing.T) {
	t.Parallel()

	proposal := icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Request:       icu.RebalanceRequest{Strategy: "unsupported"},
		Baseline:      icu.RebalanceEvaluation{Source: "test"},
		Constraints:   icu.RebalanceConstraints{TargetTolerance: 10, Z2IF: 0.65, MinSessionMinutes: 20, DurationStepMinutes: 5},
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

	if targets.Source != "missing_intensity_basis" {
		t.Fatalf("source = %q, want missing intensity basis", targets.Source)
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
			TargetLoad:          220,
			TargetTolerance:     20,
			AllowCreate:         true,
			AllowUpdate:         true,
			AllowCancel:         true,
			SportType:           "Ride",
			WorkoutTarget:       "POWER",
			StartTime:           "07:00",
			MinSessionMinutes:   20,
			DurationStepMinutes: 5,
		},
	}
}

func rebalanceCreateFixture() icu.RebalanceInput {
	return icu.RebalanceInput{
		AthleteID:     "i1",
		NowDate:       "2026-06-23",
		SportSettings: &icu.SportSettings{FTP: 285, IndoorFTP: 285, PowerZones: []int{55, 75, 90}},
		Scope: icu.RebalanceScope{
			StartDate: "2026-06-24",
			EndDate:   "2026-06-28",
			Week:      "2026-W26",
		},
		Constraints: icu.RebalanceConstraints{
			TargetLoad:          60,
			TargetTolerance:     10,
			AllowCreate:         true,
			StartTime:           "07:00",
			MinSessionMinutes:   20,
			DurationStepMinutes: 5,
			AllocationBasis:     "explicit_equal",
			SportType:           "Ride",
			WorkoutTarget:       "POWER",
		},
	}
}

func rebalanceHistoryActivities() []icu.Activity {
	return []icu.Activity{
		{Type: "Ride", StartDateLocal: "2026-06-01T06:30:00", TrainingLoad: 100, MovingTime: 60 * 60, Intensity: 0.62},
		{Type: "Ride", StartDateLocal: "2026-06-08T06:30:00", TrainingLoad: 110, MovingTime: 120 * 60, Intensity: 0.64},
		{Type: "Ride", StartDateLocal: "2026-06-15T06:30:00", TrainingLoad: 90, MovingTime: 180 * 60, Intensity: 0.66},
	}
}

func rebalanceDayWeightActivities() []icu.Activity {
	return []icu.Activity{
		{Type: "Ride", StartDateLocal: "2026-06-03T06:30:00", TrainingLoad: 10, MovingTime: 60 * 60, Intensity: 0.62},
		{Type: "Ride", StartDateLocal: "2026-06-04T06:30:00", TrainingLoad: 20, MovingTime: 60 * 60, Intensity: 0.63},
		{Type: "Ride", StartDateLocal: "2026-06-05T06:30:00", TrainingLoad: 30, MovingTime: 60 * 60, Intensity: 0.64},
		{Type: "Ride", StartDateLocal: "2026-06-06T06:30:00", TrainingLoad: 40, MovingTime: 60 * 60, Intensity: 0.65},
		{Type: "Ride", StartDateLocal: "2026-06-07T06:30:00", TrainingLoad: 50, MovingTime: 60 * 60, Intensity: 0.66},
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

func rebalanceOperationCount(operations []icu.RebalanceOperation, action string) int {
	var total int
	for index := range operations {
		if operations[index].Action == action {
			total++
		}
	}

	return total
}
