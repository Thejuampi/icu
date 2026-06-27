package icu_test

import (
	"reflect"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func v2PriorActivities() []icu.Activity {
	loads := []int{280, 290, 300, 310, 300, 295, 315, 300}
	dates := []string{
		"2026-04-27T08:00:00",
		"2026-05-04T08:00:00",
		"2026-05-11T08:00:00",
		"2026-05-18T08:00:00",
		"2026-05-25T08:00:00",
		"2026-06-01T08:00:00",
		"2026-06-08T08:00:00",
		"2026-06-15T08:00:00",
	}
	out := make([]icu.Activity, len(loads))
	for index := range loads {
		out[index] = icu.Activity{Type: "Ride", StartDateLocal: dates[index], TrainingLoad: loads[index], Intensity: 0.66, MovingTime: 12000}
	}
	return out
}

func v2Fixture(level, mode string) icu.RebalanceInput {
	return icu.RebalanceInput{
		AthleteID:  "i1",
		Activities: v2PriorActivities(),
		Events: []icu.Event{
			{ID: 1, Category: "WORKOUT", Type: "Ride", Target: "POWER", StartDateLocal: "2026-06-24T07:00:00", Name: "Tempo", TrainingLoad: 150, MovingTime: 12384, Intensity: 0.66},
			{ID: 2, Category: "WORKOUT", Type: "Ride", Target: "POWER", StartDateLocal: "2026-06-27T07:00:00", Name: "Long", TrainingLoad: 150, MovingTime: 12384, Intensity: 0.66},
		},
		SportSettings: &icu.SportSettings{FTP: 285, IndoorFTP: 285, PowerZones: []int{55, 75, 90}},
		Scope: icu.RebalanceScope{
			StartDate: "2026-06-22",
			EndDate:   "2026-06-28",
			Week:      "2026-W26",
		},
		Request: icu.RebalanceRequest{
			Strategy: icu.RebalanceStrategyAdaptiveBidirectional,
			DryRun:   true,
		},
		Constraints: icu.RebalanceConstraints{
			TargetLoad:          0,
			TargetTolerance:     20,
			AllowCreate:         true,
			AllowUpdate:         true,
			AllowCancel:         true,
			SportType:           "Ride",
			WorkoutTarget:       "POWER",
			StartTime:           "07:00",
			MinSessionMinutes:   20,
			DurationStepMinutes: 5,
			Level:               level,
			Mode:                mode,
		},
	}
}

func TestRebalanceWeeklyLoadSeriesExcludesScopeWeekAndOrders(t *testing.T) {
	t.Parallel()

	activities := []icu.Activity{
		{Type: "Ride", StartDateLocal: "2026-05-25T08:00:00", TrainingLoad: 100}, // w21
		{Type: "Ride", StartDateLocal: "2026-06-01T08:00:00", TrainingLoad: 120}, // w22
		{Type: "Ride", StartDateLocal: "2026-06-02T08:00:00", TrainingLoad: 30},  // w22
		{Type: "Ride", StartDateLocal: "2026-06-22T08:00:00", TrainingLoad: 999}, // scope week (excluded)
	}
	got := icu.RebalanceWeeklyLoadSeries(activities, "2026-06-22")
	want := []float64{100, 150}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("series = %v, want %v", got, want)
	}
}

func TestRebalanceWeeklyLoadSeriesIgnoresZeroLoadAndNonCycling(t *testing.T) {
	t.Parallel()

	activities := []icu.Activity{
		{Type: "Ride", StartDateLocal: "2026-05-04T08:00:00", TrainingLoad: 90},
		{Type: "Run", StartDateLocal: "2026-05-11T08:00:00", TrainingLoad: 80},
		{Type: "Ride", StartDateLocal: "2026-05-11T09:00:00", TrainingLoad: 0},
		{Type: "Ride", StartDateLocal: "2026-05-18T08:00:00", TrainingLoad: 110},
	}
	got := icu.RebalanceWeeklyLoadSeries(activities, "2026-06-22")
	want := []float64{90, 110}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("series = %v, want %v", got, want)
	}
}

func TestBuildRebalanceV2RequiresLevel(t *testing.T) {
	t.Parallel()

	input := v2Fixture("", "")
	input.Constraints.Level = ""
	proposal := icu.BuildRebalanceProposal(&input)
	if !proposal.Validation.Blocking {
		t.Fatalf("missing level should block")
	}
}

func TestBuildRebalanceV2RejectsInvalidLevel(t *testing.T) {
	t.Parallel()

	input := v2Fixture("", "")
	input.Constraints.Level = "1.5"
	proposal := icu.BuildRebalanceProposal(&input)
	if !proposal.Validation.Blocking {
		t.Fatalf("level 1.5 should block")
	}
}

func TestBuildRebalanceV2BlocksInsufficientHistory(t *testing.T) {
	t.Parallel()

	input := v2Fixture("0.5", "")
	input.Activities = input.Activities[:2]
	proposal := icu.BuildRebalanceProposal(&input)
	if !proposal.Validation.Blocking {
		t.Fatalf("sparse prior history should block")
	}
}

func TestBuildRebalanceV2HalfIsNoOp(t *testing.T) {
	t.Parallel()

	input := v2Fixture("0.5", "preserve-structure")
	proposal := icu.BuildRebalanceProposal(&input)
	if proposal.Validation.Blocking {
		t.Fatalf("half should not block: %v", proposal.Validation.Errors)
	}
	if len(proposal.Operations) != 0 {
		t.Fatalf("level 0.5 should produce no operations, got %d", len(proposal.Operations))
	}
}

func TestBuildRebalanceV2ReducesAtLevelZero(t *testing.T) {
	t.Parallel()

	input := v2Fixture("0", "preserve-structure")
	proposal := icu.BuildRebalanceProposal(&input)
	if proposal.Validation.Blocking {
		t.Fatalf("level 0 should not block: %v", proposal.Validation.Errors)
	}
	if len(proposal.Operations) == 0 {
		t.Fatalf("level 0 should reduce load via operations")
	}
	z1 := proposal.Options[0].Sessions[0].IntensityFactor
	if z1 > proposal.Constraints.Z1IF+0.001 {
		t.Fatalf("level 0 session IF = %.3f should be within Z1 %.3f", z1, proposal.Constraints.Z1IF)
	}
	total := 0
	for _, session := range proposal.Options[0].Sessions {
		total += session.TargetLoad
	}
	if total >= 300 {
		t.Fatalf("level 0 total load %d should reduce below current 300", total)
	}
}

func TestBuildRebalanceV2IncreasesAtLevelOneWithHeadroom(t *testing.T) {
	t.Parallel()

	input := v2Fixture("1", "preserve-structure")
	proposal := icu.BuildRebalanceProposal(&input)
	if proposal.Validation.Blocking {
		t.Fatalf("level 1 should not block: %v", proposal.Validation.Errors)
	}
	total := 0
	for _, session := range proposal.Options[0].Sessions {
		if session.IntensityFactor > proposal.Constraints.MaxIntensity+0.001 && session.IntensityFactor > 0 {
			t.Fatalf("level 1 IF %.3f exceeds max %.3f", session.IntensityFactor, proposal.Constraints.MaxIntensity)
		}
		total += session.TargetLoad
	}
	if total <= 300 {
		t.Fatalf("level 1 total %d should increase above current 300", total)
	}
}

func TestBuildRebalanceV2TargetLoadDirectionalFloor(t *testing.T) {
	t.Parallel()

	input := v2Fixture("0.3", "redistribute")
	input.Constraints.TargetLoad = 290
	proposal := icu.BuildRebalanceProposal(&input)
	target := proposal.Options[0].Evaluation.TargetLoad
	if target < 290 {
		t.Fatalf("below 0.5 floor should clamp target to 290, got %d", target)
	}
}

func TestBuildRebalanceV2TargetLoadDirectionalCeiling(t *testing.T) {
	t.Parallel()

	input := v2Fixture("0.7", "redistribute")
	input.Constraints.TargetLoad = 305
	proposal := icu.BuildRebalanceProposal(&input)
	target := proposal.Options[0].Evaluation.TargetLoad
	if target > 305 {
		t.Fatalf("above 0.5 ceiling should clamp target to 305, got %d", target)
	}
}

func TestBuildRebalanceV2BaselineOutsideEnvelopeIsDiagnostic(t *testing.T) {
	t.Parallel()

	input := v2Fixture("0.5", "preserve-structure")
	input.Events[0].TrainingLoad = 4000
	input.Events[1].TrainingLoad = 4000
	proposal := icu.BuildRebalanceProposal(&input)
	if len(proposal.Operations) != 0 {
		t.Fatalf("outside envelope should produce no operations, got %d", len(proposal.Operations))
	}
}

func TestBuildRebalanceV2RejectsV1Schema(t *testing.T) {
	t.Parallel()

	proposal := icu.RebalanceProposalFile{SchemaVersion: "icu.rebalance.v1"}
	validation := icu.ValidateRebalanceProposal(&proposal)
	if !validation.Blocking {
		t.Fatalf("v1 schema file should be rejected")
	}
}

func TestBuildRebalanceV2ModeTopologyIndependentOfLevel(t *testing.T) {
	t.Parallel()

	low := v2Fixture("0", "preserve-structure")
	high := v2Fixture("1", "preserve-structure")
	proposalLow := icu.BuildRebalanceProposal(&low)
	proposalHigh := icu.BuildRebalanceProposal(&high)
	if len(proposalLow.Operations) != len(proposalHigh.Operations) {
		t.Fatalf("preserve-structure operation count depends on level: %d vs %d", len(proposalLow.Operations), len(proposalHigh.Operations))
	}
}

func TestValidRebalanceLevelAcceptsBounds(t *testing.T) {
	t.Parallel()

	cases := []string{"0", "0.5", "1", "0.25", "0.000001"}
	for _, level := range cases {
		if !icu.ValidRebalanceLevel(level) {
			t.Fatalf("level %q should be valid", level)
		}
	}
}

func TestComputeRebalanceApproveRequiresExplicitLimitsOutsideEnvelope(t *testing.T) {
	t.Parallel()

	proposal := icu.RebalanceProposalFile{
		Constraints: icu.RebalanceConstraints{Level: "0.8", TargetLoad: 340, Mode: icu.RebalanceModePreserveStructure},
		Policy: &icu.RebalancePolicy{
			Strategy:    icu.RebalanceStrategyAdaptiveBidirectional,
			Level:       "0.8",
			Mode:        icu.RebalanceModePreserveStructure,
			CurveMethod: icu.RebalanceCurveMethodPCHIP,
		},
		Envelope: &icu.RebalanceEnvelopeReport{
			OutsideEnvelope: true,
			Envelope:        newEnvelopeRat(),
			LowSource:       "data_robust_fence",
			HighSource:      "data_robust_fence",
		},
	}

	approve := icu.ComputeRebalanceApprove(&proposal, "override", false)
	if approve.Verified {
		t.Fatalf("approve should require explicit limits outside envelope")
	}
	if approve.RecalcHash == "" || approve.LimitsHash == "" {
		t.Fatalf("approve hashes should be populated")
	}
}

func TestComputeRebalanceApproveVerifiesInsideEnvelope(t *testing.T) {
	t.Parallel()

	proposal := icu.RebalanceProposalFile{
		Constraints: icu.RebalanceConstraints{Level: "0.5", TargetLoad: 300, Mode: icu.RebalanceModePreserveStructure},
		Envelope: &icu.RebalanceEnvelopeReport{
			OutsideEnvelope: false,
			Envelope:        newEnvelopeRat(),
		},
	}

	approve := icu.ComputeRebalanceApprove(&proposal, "no-op", false)
	if !approve.Verified {
		t.Fatalf("approve should verify inside envelope")
	}
}
