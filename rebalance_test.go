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

func TestValidateRebalanceProposalHonorsStoredBlocking(t *testing.T) {
	t.Parallel()

	validation := icu.ValidateRebalanceProposal(&icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Validation: icu.RebalanceValidation{
			Blocking: true,
			Errors:   []string{"stored blocking reason"},
		},
		Constraints: icu.RebalanceConstraints{
			TargetTolerance: 10, Z2IF: 0.65, MinSessionMinutes: 20, DurationStepMinutes: 5,
			SportType: "Ride", WorkoutTarget: "POWER",
		},
	})

	if !validation.Blocking {
		t.Fatalf("validation = %+v, want stored blocking honored", validation)
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

// TestValidateRebalanceProposalRejectsOldSchemaVersionWithMigrationMessage
// is W1b-E02: a proposal file still at the pre-1b schema version is
// rejected loudly, and the message names both the old and expected
// versions plus the remedy (regenerate), not a bare "unsupported" string a
// caller cannot act on.
func TestValidateRebalanceProposalRejectsOldSchemaVersionWithMigrationMessage(t *testing.T) {
	t.Parallel()

	validation := icu.ValidateRebalanceProposal(&icu.RebalanceProposalFile{SchemaVersion: "icu.rebalance.v2"})

	if !validation.Blocking || !strings.Contains(strings.Join(validation.Errors, "\n"), "icu.rebalance.v2") ||
		!strings.Contains(strings.Join(validation.Errors, "\n"), "regenerate") {
		t.Fatalf("validation = %+v, want blocking with a migration message naming icu.rebalance.v2 and regenerate", validation)
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

// TestRebalanceSchemaVersionDidNotDrift pins the literal wire string
// (T1b.7, D-7b), not the constant against itself: per-session FTP changes
// what a proposal's prescribed watts mean, a meaning change that must carry
// a version bump a reader can grep for. Comparing icu.RebalanceSchemaVersion
// against itself would pass regardless of what the constant holds.
func TestRebalanceSchemaVersionDidNotDrift(t *testing.T) {
	t.Parallel()

	if icu.RebalanceSchemaVersion != "icu.rebalance.v3" {
		t.Fatalf("RebalanceSchemaVersion = %q, want icu.rebalance.v3", icu.RebalanceSchemaVersion)
	}
}

// TestRebalanceOutdoorSessionPricedAtOutdoorFTP is W1b-R01: FTP:270,
// IndoorFTP:285, an outdoor (no per-record indoor signal, plain "Ride")
// session must price at the outdoor number, 270 — not the athlete's
// IndoorFTP. Pre-1b's week-level rebalanceFTP unconditionally preferred
// IndoorFTP whenever it was set, mispricing every outdoor session in a week
// where the athlete has both numbers configured (confirmed failing on the
// wave-1a merge SHA 4a7f3caeacdbbfedb4c823b41c6bacba69ce1270 via a
// throwaway worktree test: rebalanceFTP returned 285, not 270).
func TestRebalanceOutdoorSessionPricedAtOutdoorFTP(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.SportSettings = &icu.SportSettings{FTP: 270, IndoorFTP: 285, PowerZones: []int{55, 75, 90}}

	proposal := icu.BuildRebalanceProposal(&input)

	session := rebalanceSessionByEventID(&proposal, 2)
	if session == nil || session.FTPApplied != 270 {
		t.Fatalf("session for event 2 = %+v, want FTPApplied 270 (outdoor default)", session)
	}
}

// TestRebalanceEqualFTPsAreInertNoGateNoWarning is W1b-R02: FTP:285,
// IndoorFTP:285 (equal) is the D-10 inertness guard's trigger — there is no
// real number to discriminate indoor from outdoor, so resolution proceeds
// unchanged from pre-1b with Source: athlete_default, no mixed-week gate,
// and no confidence warning.
func TestRebalanceEqualFTPsAreInertNoGateNoWarning(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture() // FTP:285, IndoorFTP:285 already

	proposal := icu.BuildRebalanceProposal(&input)

	session := rebalanceSessionByEventID(&proposal, 2)
	if session == nil || session.FTPSource != icu.FTPSourceAthleteDefault {
		t.Fatalf("session for event 2 = %+v, want FTPSource %q (inertness guard)", session, icu.FTPSourceAthleteDefault)
	}
}

// TestRebalanceWeekWithIndoorAndOutdoorSessionsYieldsTwoFTPApplied is
// W1b-P01: a week with one indoor (type_inherent VirtualRide) and one
// outdoor (type_default Ride) mutable event must carry two different
// ftpApplied values in the same proposal (D-11) — a single week-level FTP
// field cannot represent this week honestly.
func TestRebalanceWeekWithIndoorAndOutdoorSessionsYieldsTwoFTPApplied(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.SportSettings = &icu.SportSettings{FTP: 270, IndoorFTP: 285, PowerZones: []int{55, 75, 90}}
	input.Events[1].Type = icu.ActivityTypeVirtualRide // event 2: indoor, type_inherent
	input.Events[2].Type = "Ride"                      // event 3: outdoor, type_default

	proposal := icu.BuildRebalanceProposal(&input)

	indoorSession := rebalanceSessionByEventID(&proposal, 2)
	outdoorSession := rebalanceSessionByEventID(&proposal, 3)
	if indoorSession == nil || outdoorSession == nil || indoorSession.FTPApplied == outdoorSession.FTPApplied {
		t.Fatalf("indoor/outdoor FTPApplied = %+v/%+v, want two different values", indoorSession, outdoorSession)
	}
}

// TestRebalanceMixedWeekWithSessionEnvironmentRefuses is W1b-N01: a mixed-
// environment week (one type-inherent indoor session, one type-default
// outdoor session) with no per-record signal and only --session-environment
// given must refuse — a single scalar cannot describe both sessions
// honestly (D-10c). Refusal here means zero operations are produced and the
// mixed week is named in the warnings.
func TestRebalanceMixedWeekWithSessionEnvironmentRefuses(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Constraints.SessionEnvironment = icu.SessionEnvironmentOutdoor
	input.Events[1].Type = icu.ActivityTypeVirtualRide
	input.Events[2].Type = "Ride"

	proposal := icu.BuildRebalanceProposal(&input)

	if len(proposal.Operations) != 0 || !strings.Contains(strings.Join(proposal.Options[0].Warnings, "\n"), "mixed-environment week") {
		t.Fatalf("operations = %d, warnings = %v, want zero operations and a named mixed-week refusal",
			len(proposal.Operations), proposal.Options[0].Warnings)
	}
}

// TestRebalanceClassFourSessionEnumeratedWithNoLoadSource is W1b-E01: a
// class-4 (D-10 not_applicable) session in the week is enumerated in
// timeAccounting with loadSource "none" — recorded so its minutes are
// visible, never priced (D-23), and never compared against a capacity
// bound: F-27 establishes no per-day minutes bound exists before wave-3, so
// this test does not attempt one.
func TestRebalanceClassFourSessionEnumeratedWithNoLoadSource(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Events = append(input.Events, icu.Event{
		ID: 4, Category: "WORKOUT", Type: "Run",
		StartDateLocal: "2026-06-26T00:00:00", Name: "Easy jog", MovingTime: 1800,
	})

	proposal := icu.BuildRebalanceProposal(&input)

	entry := rebalanceTimeAccountingEntryByName(&proposal, "Easy jog")
	if entry == nil || entry.LoadSource != "none" {
		t.Fatalf("timeAccounting entry = %+v, want an entry with loadSource \"none\"", entry)
	}
}

// TestRebalanceWeekWithClassFourSessionIsPartialAccounting is W1b-E03: a
// week containing one class-4 session has loadAccounting "partial" and
// unaccountedSessions 1 — the weekly load total is knowingly incomplete
// until wave-2's hr_load arrives (D-23).
func TestRebalanceWeekWithClassFourSessionIsPartialAccounting(t *testing.T) {
	t.Parallel()

	input := rebalanceFixture()
	input.Events = append(input.Events, icu.Event{
		ID: 4, Category: "WORKOUT", Type: "Run",
		StartDateLocal: "2026-06-26T00:00:00", Name: "Easy jog", MovingTime: 1800,
	})

	proposal := icu.BuildRebalanceProposal(&input)

	if proposal.TimeAccounting == nil || proposal.TimeAccounting.LoadAccounting != "partial" || proposal.TimeAccounting.UnaccountedSessions != 1 {
		t.Fatalf("timeAccounting = %+v, want loadAccounting \"partial\" and unaccountedSessions 1", proposal.TimeAccounting)
	}
}

// TestRebalanceStravaStubActivityConsumesNoTimeOrLoad is W1b-E04: a week
// containing a STRAVA stub (an Activity record with no type and no
// metrics — D-24's not_a_record shape) must not change the week's
// completed load. Sessions are built from Events, never from Activities,
// so a stub activity structurally cannot "appear as a session"; the
// residual risk this test pins is completedRebalanceDates silently
// counting the stub's mere presence on a date as a real completed session.
func TestRebalanceStravaStubActivityConsumesNoTimeOrLoad(t *testing.T) {
	t.Parallel()

	withoutStub := rebalanceFixture()
	proposalWithout := icu.BuildRebalanceProposal(&withoutStub)

	withStub := rebalanceFixture()
	withStub.Activities = append(withStub.Activities, icu.Activity{ID: "stub1", StartDateLocal: "2026-06-26T00:00:00"})
	proposalWith := icu.BuildRebalanceProposal(&withStub)

	if proposalWith.Options[0].Evaluation.CompletedLoad != proposalWithout.Options[0].Evaluation.CompletedLoad {
		t.Fatalf("completedLoad with stub = %d, want unchanged %d (a STRAVA stub carries zero load and consumes no time)",
			proposalWith.Options[0].Evaluation.CompletedLoad, proposalWithout.Options[0].Evaluation.CompletedLoad)
	}
}

// TestRebalanceUpdateBodyPreservesIndoorTrueWithoutSessionEnvironment is
// W1b-E05: an UPDATE on an event with Indoor: true, with no
// --session-environment override intending a change, must carry
// Indoor: true on the operation body — not silently drop back to Go's
// zero value (D-25/T1b.11).
func TestRebalanceUpdateBodyPreservesIndoorTrueWithoutSessionEnvironment(t *testing.T) {
	t.Parallel()

	input := rebalanceUpdateEnvironmentFixture(true, "")

	proposal := icu.BuildRebalanceProposal(&input)

	if len(proposal.Operations) != 1 || !proposal.Operations[0].Body.Indoor {
		t.Fatalf("operations = %+v, want exactly one operation with body.Indoor true", proposal.Operations)
	}
}

// TestRebalanceUpdateBodySessionEnvironmentOverridesIndoorTrue is W1b-E06:
// --session-environment outdoor on an event with Indoor: true still wins —
// an explicit operator assertion overrides the event's own stored flag.
func TestRebalanceUpdateBodySessionEnvironmentOverridesIndoorTrue(t *testing.T) {
	t.Parallel()

	input := rebalanceUpdateEnvironmentFixture(true, icu.SessionEnvironmentOutdoor)

	proposal := icu.BuildRebalanceProposal(&input)

	if len(proposal.Operations) != 1 || proposal.Operations[0].Body.Indoor {
		t.Fatalf("operations = %+v, want exactly one operation with body.Indoor false (explicit outdoor override)", proposal.Operations)
	}
}

// rebalanceUpdateEnvironmentFixture builds a single-event week (no
// synthetic Constraints.SportType injected, so the D-10c mixed-week gate
// never fires here) so W1b-E05/E06 can isolate the UPDATE body's Indoor
// value from the mixed-week refusal path exercised separately by
// TestRebalanceMixedWeekWithSessionEnvironmentRefuses.
func rebalanceUpdateEnvironmentFixture(indoor bool, sessionEnvironment string) icu.RebalanceInput {
	return icu.RebalanceInput{
		AthleteID: "i1",
		NowDate:   "2026-06-24",
		Events: []icu.Event{
			{
				ID: 5, Category: "WORKOUT", Type: "Ride", StartDateLocal: "2026-06-27T00:00:00",
				Name: "Endurance", TrainingLoad: 100, MovingTime: 5400, Intensity: 0.65, Indoor: boolPtr(indoor),
			},
		},
		SportSettings: &icu.SportSettings{FTP: 270, IndoorFTP: 285, PowerZones: []int{55, 75, 90}},
		Scope:         icu.RebalanceScope{StartDate: "2026-06-24", EndDate: "2026-06-28", Week: "2026-W26"},
		Constraints: icu.RebalanceConstraints{
			TargetLoad: 100, TargetTolerance: 20,
			AllowUpdate: true, WorkoutTarget: "POWER", StartTime: "07:00",
			MinSessionMinutes: 20, DurationStepMinutes: 5,
			SessionEnvironment: sessionEnvironment,
		},
	}
}

func rebalanceTimeAccountingEntryByName(proposal *icu.RebalanceProposalFile, name string) *icu.RebalanceTimeAccountingEntry {
	if proposal.TimeAccounting == nil {
		return nil
	}

	for index := range proposal.TimeAccounting.Entries {
		if proposal.TimeAccounting.Entries[index].Name == name {
			return &proposal.TimeAccounting.Entries[index]
		}
	}

	return nil
}

func rebalanceSessionByEventID(proposal *icu.RebalanceProposalFile, eventID int) *icu.RebalanceSession {
	for index := range proposal.Options[0].Sessions {
		if proposal.Options[0].Sessions[index].EventID == eventID {
			return &proposal.Options[0].Sessions[index]
		}
	}

	return nil
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
