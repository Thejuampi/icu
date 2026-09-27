package icu_test

import (
	"encoding/json"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestBuildPlanProposalCreatesSessionFromIntent(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowCreate:      true,
				DefaultStartTime: "07:00",
				DefaultType:      "Ride",
				DefaultCategory:  "WORKOUT",
				DefaultTarget:    "POWER",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date:      "2026-07-28",
				Name:      "Z2 HR-Control Waves",
				Desc:      "- 60m 70% FTP",
				DescLocal: "- 60m 70%",
			}},
		},
	})

	if len(proposal.Operations) != 1 || proposal.Operations[0].Action != icu.CalendarActionCreate || proposal.Operations[0].ExpectedLoad != 49 {
		t.Fatalf("operations = %+v, want one create with load 49", proposal.Operations)
	}
}

// TestBuildPlanProposalIndoorDraftPricedAtIndoorFTP is plan's W1b-R01/P01
// analogue: FTP:270, IndoorFTP:285, one draft marked Indoor: true and one
// left at the outdoor default in the same intent — the indoor draft must
// price at 285 and the outdoor draft at 270, two different ftpApplied in
// one proposal (D-11). Pre-1b's planFTP returned one week-level number for
// every session regardless of Indoor, which would have priced both drafts
// identically.
func TestBuildPlanProposalIndoorDraftPricedAtIndoorFTP(t *testing.T) {
	t.Parallel()

	trueValue := true
	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate:       "2026-07-27",
		SportSettings: &icu.SportSettings{FTP: 270, IndoorFTP: 285},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-29",
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{
				{Date: "2026-07-28", Name: "Indoor session", DescLocal: "- 60m 70%", Desc: "- 60m 70%", Indoor: &trueValue},
				{Date: "2026-07-29", Name: "Outdoor session", DescLocal: "- 60m 70%", Desc: "- 60m 70%"},
			},
		},
	})

	indoor := planResolvedSessionByName(&proposal, "Indoor session")
	outdoor := planResolvedSessionByName(&proposal, "Outdoor session")
	if indoor == nil || outdoor == nil || indoor.FTPApplied != 285 || outdoor.FTPApplied != 270 {
		t.Fatalf("indoor/outdoor sessions = %+v/%+v, want ftpApplied 285/270", indoor, outdoor)
	}
}

// TestPlanAndRebalanceAgreeOnSameSessionExpectedLoad is W1b-P02: for the
// same session (same WorkoutDoc, same date, same Indoor flag, same athlete
// FTP settings), plan and rebalance must compute identical expected load —
// F-1's disagreement (rebalanceFTP always preferring IndoorFTP vs. planFTP's
// own separate week-level pick) is gone because both now resolve the same
// per-session FTPSelection via resolveEventFTP and price it through the
// same EstimatePlannedLoad.
func TestPlanAndRebalanceAgreeOnSameSessionExpectedLoad(t *testing.T) {
	t.Parallel()

	settings := &icu.SportSettings{FTP: 270, IndoorFTP: 285}
	trueValue := true
	doc := icu.WorkoutDoc{Steps: []icu.WorkoutStep{{Duration: 3600, Power: &icu.WorkoutTarget{Value: 70, Units: "%ftp"}}}}
	event := icu.Event{
		ID: 42, Category: "WORKOUT", Type: "Ride", Indoor: &trueValue,
		StartDateLocal: "2026-07-28T06:30:00", Name: "Z2", WorkoutDoc: doc,
	}

	planProposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate:       "2026-07-27",
		SportSettings: settings,
		Events:        []icu.Event{event},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28",
			Constraints: icu.PlanConstraints{AllowUpdate: true, DefaultStartTime: "07:00", DefaultType: "Ride"},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Z2", Desc: "- 60m 70% FTP", DescLocal: "- 60m 70%", Indoor: &trueValue,
			}},
		},
	})

	rebalanceProposal := icu.BuildRebalanceProposal(&icu.RebalanceInput{
		AthleteID: "i1", NowDate: "2026-07-20",
		SportSettings: settings,
		Events:        []icu.Event{event},
		Scope:         icu.RebalanceScope{StartDate: "2026-07-27", EndDate: "2026-08-02", Week: "2026-W31"},
		Constraints: icu.RebalanceConstraints{
			// Wide tolerance so rebalance reports the session's own baseline
			// reading rather than redistributing it toward a new target —
			// what "reading the same session" means for a tool whose job is
			// otherwise to change the number.
			AllowUpdate: true, TargetLoad: 1, TargetTolerance: 1000,
			WorkoutTarget: "POWER", StartTime: "07:00", MinSessionMinutes: 20, DurationStepMinutes: 5,
		},
	})

	if planProposal.Operations[0].ExpectedLoad != rebalanceProposal.Baseline.MutablePlannedLoad {
		t.Fatalf("plan expectedLoad = %d, rebalance baseline mutablePlannedLoad = %d, want equal",
			planProposal.Operations[0].ExpectedLoad, rebalanceProposal.Baseline.MutablePlannedLoad)
	}
}

func planResolvedSessionByName(proposal *icu.PlanProposalFile, name string) *icu.PlanResolvedSession {
	for index := range proposal.Sessions {
		if proposal.Sessions[index].Name == name {
			return &proposal.Sessions[index]
		}
	}

	return nil
}

func TestBuildPlanProposalUpdatesMatchedEventByDateName(t *testing.T) {
	t.Parallel()

	event := icu.Event{
		ID:             42,
		Name:           "Z2 HR-Control Waves",
		Category:       "WORKOUT",
		Type:           "Ride",
		StartDateLocal: "2026-07-28T06:30:00",
		Description:    "old",
		TrainingLoad:   30,
		MovingTime:     1800,
		Target:         "POWER",
	}
	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Events:  []icu.Event{event},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowUpdate:      true,
				DefaultStartTime: "07:00",
				DefaultType:      "Ride",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date:      "2026-07-28",
				Name:      "Z2 HR-Control Waves",
				Desc:      "- 60m 70% FTP",
				DescLocal: "- 60m 70%",
			}},
		},
	})

	if len(proposal.Operations) != 1 || proposal.Operations[0].Action != icu.CalendarActionUpdate || proposal.Operations[0].EventID != 42 {
		t.Fatalf("operations = %+v, want one update for event 42", proposal.Operations)
	}
}

// TestBuildPlanProposalUpdateBodyPreservesIndoorTrue is plan's side of
// W1b-E05: an UPDATE on an event with Indoor: true, with no
// --session-environment override intending a change, must carry
// Indoor: true on the operation body — not silently drop back to Go's
// zero value (D-25/T1b.11).
func TestBuildPlanProposalUpdateBodyPreservesIndoorTrue(t *testing.T) {
	t.Parallel()

	event := icu.Event{
		ID: 42, Name: "Z2 HR-Control Waves", Category: "WORKOUT", Type: "Ride",
		StartDateLocal: "2026-07-28T06:30:00", TrainingLoad: 30, MovingTime: 1800, Target: "POWER",
		Indoor: boolPtr(true),
	}
	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Events:  []icu.Event{event},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowUpdate: true, DefaultStartTime: "07:00", DefaultType: "Ride",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Z2 HR-Control Waves",
				Desc: "- 60m 70% FTP", DescLocal: "- 60m 70%",
			}},
		},
	})

	if len(proposal.Operations) != 1 || !proposal.Operations[0].Body.Indoor {
		t.Fatalf("operations = %+v, want one update with body.Indoor true", proposal.Operations)
	}
}

// TestBuildPlanProposalUpdateBodySessionEnvironmentOverridesIndoorTrue is
// plan's side of W1b-E06: --session-environment outdoor on an event with
// Indoor: true still wins.
func TestBuildPlanProposalUpdateBodySessionEnvironmentOverridesIndoorTrue(t *testing.T) {
	t.Parallel()

	event := icu.Event{
		ID: 42, Name: "Z2 HR-Control Waves", Category: "WORKOUT", Type: "Ride",
		StartDateLocal: "2026-07-28T06:30:00", TrainingLoad: 30, MovingTime: 1800, Target: "POWER",
		Indoor: boolPtr(true),
	}
	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Events:  []icu.Event{event},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowUpdate: true, DefaultStartTime: "07:00", DefaultType: "Ride",
				SessionEnvironment: icu.SessionEnvironmentOutdoor,
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Z2 HR-Control Waves",
				Desc: "- 60m 70% FTP", DescLocal: "- 60m 70%",
			}},
		},
	})

	if len(proposal.Operations) != 1 || proposal.Operations[0].Body.Indoor {
		t.Fatalf("operations = %+v, want one update with body.Indoor false (explicit outdoor override)", proposal.Operations)
	}
}

// TestBuildPlanProposalUpdateBodyWirePreservesIndoorTrue is F1's wire-level
// pin, plan's side of W1b-E05: the struct-field assertion above can pass
// even if the JSON tag regresses. Marshal the actual operation body and
// require the literal `"indoor":true` bytes an UPDATE with no
// --session-environment override sends on the wire.
func TestBuildPlanProposalUpdateBodyWirePreservesIndoorTrue(t *testing.T) {
	t.Parallel()

	event := icu.Event{
		ID: 42, Name: "Z2 HR-Control Waves", Category: "WORKOUT", Type: "Ride",
		StartDateLocal: "2026-07-28T06:30:00", TrainingLoad: 30, MovingTime: 1800, Target: "POWER",
		Indoor: boolPtr(true),
	}
	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Events:  []icu.Event{event},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowUpdate: true, DefaultStartTime: "07:00", DefaultType: "Ride",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Z2 HR-Control Waves",
				Desc: "- 60m 70% FTP", DescLocal: "- 60m 70%",
			}},
		},
	})

	body, err := json.Marshal(proposal.Operations[0].Body)
	if err != nil {
		t.Fatalf("marshal operation body: %v", err)
	}

	if got := string(body); !strings.Contains(got, `"indoor":true`) {
		t.Fatalf("operation body JSON = %s, want it to contain \"indoor\":true", got)
	}
}

// TestBuildPlanProposalUpdateBodyWireOverrideOmitsIndoorKey is F1's
// wire-level pin, plan's side of W1b-E06: EventEx.Indoor is a bare bool with
// `omitempty` (LL-17: it must not become a pointer, which would regress the
// preserve case above), so an override to outdoor marshals to no "indoor"
// key at all rather than an explicit false — a request body byte-identical
// to one that never resolved an environment. See docs/rebalance.md for the
// unverified stored-record caveat this test does not attempt to settle.
func TestBuildPlanProposalUpdateBodyWireOverrideOmitsIndoorKey(t *testing.T) {
	t.Parallel()

	event := icu.Event{
		ID: 42, Name: "Z2 HR-Control Waves", Category: "WORKOUT", Type: "Ride",
		StartDateLocal: "2026-07-28T06:30:00", TrainingLoad: 30, MovingTime: 1800, Target: "POWER",
		Indoor: boolPtr(true),
	}
	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Events:  []icu.Event{event},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowUpdate: true, DefaultStartTime: "07:00", DefaultType: "Ride",
				SessionEnvironment: icu.SessionEnvironmentOutdoor,
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Z2 HR-Control Waves",
				Desc: "- 60m 70% FTP", DescLocal: "- 60m 70%",
			}},
		},
	})

	body, err := json.Marshal(proposal.Operations[0].Body)
	if err != nil {
		t.Fatalf("marshal operation body: %v", err)
	}

	if got := string(body); strings.Contains(got, `"indoor"`) {
		t.Fatalf("operation body JSON = %s, want no \"indoor\" key (explicit outdoor override omits, does not send false)", got)
	}
}

func TestBuildPlanProposalMatchesEventByUID(t *testing.T) {
	t.Parallel()

	event := icu.Event{
		ID:             7,
		UID:            "session-uid-1",
		Name:           "Other Name",
		Category:       "WORKOUT",
		Type:           "Ride",
		StartDateLocal: "2026-07-29T07:00:00",
		Target:         "POWER",
	}
	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Events:  []icu.Event{event},
		Intent: icu.PlanIntent{
			Oldest:      "2026-07-28",
			Newest:      "2026-07-29",
			FTP:         300,
			Constraints: icu.PlanConstraints{AllowUpdate: true, DefaultType: "Ride", DefaultStartTime: "07:00"},
			Sessions: []icu.PlanSessionDraft{{
				Date:      "2026-07-28",
				Name:      "Renamed",
				UID:       "session-uid-1",
				DescLocal: "- 30m 70%",
				Desc:      "- 30m 70% FTP",
			}},
		},
	})

	if len(proposal.Operations) != 1 || proposal.Operations[0].EventID != 7 || proposal.Sessions[0].MatchedBy != "uid" {
		t.Fatalf("match = %+v sessions=%+v, want uid match event 7", proposal.Operations, proposal.Sessions)
	}
}

func TestBuildPlanProposalUsesDescForBodyAndDescLocalForLoad(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date:      "2026-07-28",
				Name:      "Intervals",
				Desc:      "2x\n\n- 5m Ramp 120-120% FTP\n- 5m 50% FTP",
				DescLocal: "2x\n  - 5m 120%\n  - 5m 50%",
			}},
		},
	})

	if proposal.Operations[0].Body.Description != "2x\n\n- 5m Ramp 120-120% FTP\n- 5m 50% FTP" {
		t.Fatalf("body desc = %q, want intervals format", proposal.Operations[0].Body.Description)
	}
	if proposal.Operations[0].ExpectedLoad <= 0 || proposal.Operations[0].Body.TrainingLoad != proposal.Operations[0].ExpectedLoad {
		t.Fatalf("load = %+v, want calculated load on body", proposal.Operations[0])
	}
}

func TestBuildPlanProposalLocksPastByDefault(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-28",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-27",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-27", Name: "Past", DescLocal: "- 30m 70%", Desc: "- 30m 70%",
			}},
		},
	})

	if len(proposal.Operations) != 0 || len(proposal.Validation.Warnings) == 0 {
		t.Fatalf("ops=%d warnings=%v, want locked past warning and no ops", len(proposal.Operations), proposal.Validation.Warnings)
	}
}

func TestBuildPlanProposalAllowsPastWhenConfigured(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate:   "2026-07-28",
		AllowPast: true,
		Intent: icu.PlanIntent{
			Oldest: "2026-07-27",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, AllowPast: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-27", Name: "Past", DescLocal: "- 30m 70%", Desc: "- 30m 70%",
			}},
		},
	})

	if len(proposal.Operations) != 1 || proposal.Operations[0].Action != icu.CalendarActionCreate {
		t.Fatalf("operations = %+v, want create for allowed past", proposal.Operations)
	}
}

func TestBuildPlanProposalCancelsUnmatchedWhenAllowed(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Events: []icu.Event{{
			ID: 9, Name: "Old Spin", Category: "WORKOUT", Type: "Ride",
			StartDateLocal: "2026-07-28T07:00:00", TrainingLoad: 20, MovingTime: 1200,
		}},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, AllowCancel: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "New Z2", DescLocal: "- 60m 70%", Desc: "- 60m 70%",
			}},
		},
	})

	var cancelCount int
	for index := range proposal.Operations {
		if proposal.Operations[index].Action == icu.CalendarActionCancel && proposal.Operations[index].EventID == 9 {
			cancelCount++
		}
	}
	if cancelCount != 1 {
		t.Fatalf("operations = %+v, want cancel for unmatched event 9", proposal.Operations)
	}
}

func TestBuildPlanProposalDoesNotCancelUnmatchedByDefault(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Events: []icu.Event{{
			ID: 9, Name: "Old Spin", Category: "WORKOUT", Type: "Ride",
			StartDateLocal: "2026-07-28T07:00:00", TrainingLoad: 20,
		}},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "New Z2", DescLocal: "- 60m 70%", Desc: "- 60m 70%",
			}},
		},
	})

	for index := range proposal.Operations {
		if proposal.Operations[index].Action == icu.CalendarActionCancel {
			t.Fatalf("operations = %+v, want no cancel by default", proposal.Operations)
		}
	}
}

func TestBuildPlanProposalWeeklyTargetWarningNotBlocking(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-08-02",
			FTP:    300,
			WeeklyTargets: []icu.PlanWeeklyTarget{{
				WeekStart: "2026-07-27", TargetLoad: 200, Tolerance: 10,
			}},
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Z2", DescLocal: "- 60m 70%", Desc: "- 60m 70%",
			}},
		},
	})

	if proposal.Validation.Blocking || len(proposal.Validation.Warnings) == 0 {
		t.Fatalf("validation = %+v, want non-blocking weekly target warning", proposal.Validation)
	}
}

// TestBuildPlanProposalWeeklyLoadExcludesStubActivity is F2: a D-24 STRAVA
// stub is not a session and must not contribute to weeklyLoads.completedLoad,
// matching every sibling aggregator's use of ExcludeStubActivities. The stub
// here carries a deliberately large TrainingLoad to prove exclusion rather
// than relying on real stubs never happening to carry one.
func TestBuildPlanProposalWeeklyLoadExcludesStubActivity(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Activities: []icu.Activity{{
			StartDateLocal: "2026-07-28T00:00:00",
			TrainingLoad:   500,
			StubShape:      true,
		}},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-08-02",
			FTP:    300,
			WeeklyTargets: []icu.PlanWeeklyTarget{{
				WeekStart: "2026-07-27", TargetLoad: 200, Tolerance: 10,
			}},
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
		},
	})

	if len(proposal.WeeklyLoads) != 1 || proposal.WeeklyLoads[0].CompletedLoad != 0 {
		t.Fatalf("weeklyLoads = %+v, want one week with completedLoad 0 (stub excluded despite TrainingLoad 500)", proposal.WeeklyLoads)
	}
}

func TestBuildPlanProposalStrictTargetsBlock(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest:        "2026-07-28",
			Newest:        "2026-08-02",
			FTP:           300,
			StrictTargets: true,
			WeeklyTargets: []icu.PlanWeeklyTarget{{
				WeekStart: "2026-07-27", TargetLoad: 200, Tolerance: 10,
			}},
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Z2", DescLocal: "- 60m 70%", Desc: "- 60m 70%",
			}},
		},
	})

	if !proposal.Validation.Blocking {
		t.Fatalf("validation = %+v, want blocking strict weekly target", proposal.Validation)
	}
}

func TestBuildPlanProposalBaselineCompletedLoad(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-28",
		Activities: []icu.Activity{{
			StartDateLocal: "2026-07-27T08:00:00",
			TrainingLoad:   55,
			Type:           "Ride",
		}},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-27",
			Newest: "2026-08-02",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-29", Name: "Z2", DescLocal: "- 60m 70%", Desc: "- 60m 70%",
			}},
		},
	})

	if proposal.Baseline.CompletedLoad != 55 {
		t.Fatalf("baseline completed = %d, want 55", proposal.Baseline.CompletedLoad)
	}
}

func TestValidatePlanProposalRejectsBadSchema(t *testing.T) {
	t.Parallel()

	validation := icu.ValidatePlanProposal(&icu.PlanProposalFile{
		SchemaVersion: "nope",
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-28"},
	})

	if !validation.Blocking || !strings.Contains(strings.Join(validation.Errors, " "), "schemaVersion") {
		t.Fatalf("validation = %+v, want schema error", validation)
	}
}

func TestMarshalPlanProposalPrettyJSON(t *testing.T) {
	t.Parallel()

	data, err := icu.MarshalPlanProposal(&icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		ProposalID:    "plan-test",
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-28"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(data), "\n  \"schemaVersion\"") {
		t.Fatalf("json = %s, want pretty indented", data)
	}
}

func TestBuildPlanProposalNilInput(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(nil)

	if proposal.SchemaVersion != icu.PlanSchemaVersion {
		t.Fatalf("schema = %q, want %q", proposal.SchemaVersion, icu.PlanSchemaVersion)
	}
}

func TestBuildPlanProposalFTPFromSportSettings(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate:       "2026-07-27",
		SportSettings: &icu.SportSettings{FTP: 300},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-28",
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Z2", DescLocal: "- 60m 70%", Desc: "- 60m 70%",
			}},
		},
	})

	// FTPSource is "athlete_default" (D-10's inertness guard, wave-1b): with
	// no IndoorFTP configured there is no real number to discriminate
	// between, so Source no longer distinguishes "came from intent" vs.
	// "came from sport settings" — Watts (still 300, from SportSettings) and
	// ExpectedLoad are what this test actually protects.
	if proposal.Context.FTP != 300 || proposal.Context.FTPSource != icu.FTPSourceAthleteDefault || proposal.Operations[0].ExpectedLoad != 49 {
		t.Fatalf("context/ops = %+v / %+v, want ftp from sport settings and load 49", proposal.Context, proposal.Operations)
	}
}

// TestBuildPlanProposalNoPhantomFTPWarningWithoutDefaultType is Stage 6 R4:
// context.ftp/ftpSource are a week-level representative only (D-11) that
// requires Constraints.DefaultType to resolve; each session is still priced
// individually from its own Type. Previously, an intent with FTP set but no
// DefaultType (a realistic shape when every session names its own Type) got
// a false "ftp is missing" context warning even though the session itself
// resolved and priced correctly, because the warning did not distinguish
// "the representative type is not applicable" from "the athlete has no FTP".
func TestBuildPlanProposalNoPhantomFTPWarningWithoutDefaultType(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate:       "2026-07-27",
		SportSettings: &icu.SportSettings{FTP: 270},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28",
			FTP: 270,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultCategory: "WORKOUT",
				// DefaultType intentionally left unset: sessions name their own Type.
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Z2", Type: "Ride", DescLocal: "- 60m 70%", Desc: "- 60m 70%",
			}},
		},
	})

	if len(proposal.Context.Warnings) != 0 {
		t.Fatalf("context.warnings = %v, want none (session prices its own type)", proposal.Context.Warnings)
	}
	if len(proposal.Operations) != 1 || proposal.Operations[0].ExpectedLoad != 49 {
		t.Fatalf("operations = %+v, want one create priced from the session's own type", proposal.Operations)
	}
}

func TestBuildPlanProposalDerivesScopeFromSessions(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			FTP: 300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{
				{Date: "2026-07-30", Name: "B", DescLocal: "- 30m 70%", Desc: "- 30m 70%"},
				{Date: "2026-07-28", Name: "A", DescLocal: "- 30m 70%", Desc: "- 30m 70%"},
			},
		},
	})

	if proposal.Scope.StartDate != "2026-07-28" || proposal.Scope.EndDate != "2026-07-30" {
		t.Fatalf("scope = %+v, want derived session bounds", proposal.Scope)
	}
}

func TestBuildPlanProposalBlocksCreateWithoutType(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28", FTP: 300,
			Constraints: icu.PlanConstraints{AllowCreate: true, DefaultStartTime: "07:00"},
			Sessions:    []icu.PlanSessionDraft{{Date: "2026-07-28", Name: "Z2", Desc: "- 30m 70%"}},
		},
	})

	// Stage 6 R4: this used to assert on Warnings, which passed only
	// incidentally — a since-fixed phantom "ftp is missing" context warning
	// (unrelated to this test's actual scenario) always populated Warnings
	// regardless of whether create-without-type genuinely blocked anything.
	// The real signal is Validation.Blocking/Errors: without a type, the
	// session's workout description cannot resolve an FTP to price against,
	// so expectedLoad stays 0 and ValidatePlanProposal blocks on it.
	if len(proposal.Operations) != 0 || !proposal.Validation.Blocking || len(proposal.Validation.Errors) == 0 {
		t.Fatalf("ops=%d blocking=%v errors=%v, want create blocked without type",
			len(proposal.Operations), proposal.Validation.Blocking, proposal.Validation.Errors)
	}
}

func TestBuildPlanProposalBlocksWhenCreateDisabled(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28", FTP: 300,
			Constraints: icu.PlanConstraints{AllowCreate: false, AllowUpdate: true, DefaultStartTime: "07:00", DefaultType: "Ride"},
			Sessions:    []icu.PlanSessionDraft{{Date: "2026-07-28", Name: "Z2", Desc: "- 30m 70%"}},
		},
	})

	if len(proposal.Operations) != 0 {
		t.Fatalf("operations = %+v, want none when create disabled", proposal.Operations)
	}
}

func TestBuildPlanProposalCancelCategoriesCustom(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Events: []icu.Event{
			{ID: 1, Name: "Note", Category: "NOTE", StartDateLocal: "2026-07-28T07:00:00"},
			{ID: 2, Name: "Spin", Category: "WORKOUT", Type: "Ride", StartDateLocal: "2026-07-28T08:00:00"},
		},
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28", FTP: 300,
			Constraints: icu.PlanConstraints{
				AllowCancel: true, AllowCreate: true, CancelCategories: []string{"NOTE"},
				DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{Date: "2026-07-28", Name: "New", Desc: "- 30m 70%", DescLocal: "- 30m 70%"}},
		},
	})

	var noteCancel bool
	var workoutCancel bool
	for index := range proposal.Operations {
		op := proposal.Operations[index]
		if op.Action != icu.CalendarActionCancel {
			continue
		}
		if op.EventID == 1 {
			noteCancel = true
		}
		if op.EventID == 2 {
			workoutCancel = true
		}
	}
	if !noteCancel || workoutCancel {
		t.Fatalf("operations = %+v, want cancel NOTE only", proposal.Operations)
	}
}

func TestMarshalPlanIntentPrettyJSON(t *testing.T) {
	t.Parallel()

	data, err := icu.MarshalPlanIntent(&icu.PlanIntent{SchemaVersion: icu.PlanIntentSchemaVersion})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(data), "schemaVersion") {
		t.Fatalf("json = %s, want schemaVersion", data)
	}
}

func TestBuildPlanProposalSessionOutsideRangeBlocks(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28",
			Newest: "2026-07-30",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-08-05", Name: "Outside", DescLocal: "- 60m 70%", Desc: "- 60m 70%",
			}},
		},
	})

	if !proposal.Validation.Blocking || len(proposal.Operations) != 0 {
		t.Fatalf("validation=%+v ops=%d, want blocking and no ops", proposal.Validation, len(proposal.Operations))
	}
}

func TestBuildPlanProposalInvertedRangeBlocks(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-08-02",
			Newest: "2026-07-28",
			FTP:    300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-29", Name: "Z2", DescLocal: "- 60m 70%", Desc: "- 60m 70%",
			}},
		},
	})

	if !proposal.Validation.Blocking {
		t.Fatalf("validation = %+v, want blocking inverted range", proposal.Validation)
	}
}

func TestBuildPlanProposalMissingDateNameBlocks(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28", FTP: 300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{Name: "NoDate", DescLocal: "- 60m 70%", Desc: "- 60m 70%"}},
		},
	})

	if !proposal.Validation.Blocking {
		t.Fatalf("validation = %+v, want blocking missing date/name", proposal.Validation)
	}
}

func TestBuildPlanProposalUnparseableWorkoutDescBlocks(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28", FTP: 300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Bad", Desc: "not a workout", DescLocal: "not a workout",
			}},
		},
	})

	if !proposal.Validation.Blocking || len(proposal.Operations) != 0 {
		t.Fatalf("validation=%+v ops=%d, want blocking unparseable workout and no ops", proposal.Validation, len(proposal.Operations))
	}
}

func TestBuildPlanProposalZeroLoadWorkoutBlocks(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28", FTP: 300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "WORKOUT",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "EmptyLoad", Desc: "- 0m 70%", DescLocal: "- 0m 70%",
			}},
		},
	})

	if !proposal.Validation.Blocking || len(proposal.Operations) != 0 {
		t.Fatalf("validation=%+v ops=%d, want blocking zero-load workout and no ops", proposal.Validation, len(proposal.Operations))
	}
}

func TestBuildPlanProposalNOTEAllowsNoLoad(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28", FTP: 300,
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "NOTE",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Coach note", Category: "NOTE", Desc: "free text reminder",
			}},
		},
	})

	if proposal.Validation.Blocking || len(proposal.Operations) != 1 {
		t.Fatalf("validation=%+v ops=%d, want non-blocking NOTE create", proposal.Validation, len(proposal.Operations))
	}
}

// TestValidatePlanProposalRejectsOldSchemaVersionWithMigrationMessage
// mirrors W1b-E02 for plan: a proposal file still at the pre-1b schema
// version is rejected with a message naming both the stale version and the
// remedy, not a bare "unsupported" string a caller cannot act on.
func TestValidatePlanProposalRejectsOldSchemaVersionWithMigrationMessage(t *testing.T) {
	t.Parallel()

	validation := icu.ValidatePlanProposal(&icu.PlanProposalFile{
		SchemaVersion: "icu.plan.v1",
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-30"},
	})

	if !validation.Blocking || !strings.Contains(strings.Join(validation.Errors, "\n"), "icu.plan.v1") ||
		!strings.Contains(strings.Join(validation.Errors, "\n"), "regenerate") {
		t.Fatalf("validation = %+v, want blocking with a migration message naming icu.plan.v1 and regenerate", validation)
	}
}

func TestValidatePlanProposalHonorsStoredBlocking(t *testing.T) {
	t.Parallel()

	validation := icu.ValidatePlanProposal(&icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-30"},
		Validation: icu.PlanValidation{
			Blocking: true,
			Errors:   []string{"week 2026-07-27 planned load 49 outside target 200±10 (delta -151)"},
		},
		Operations: []icu.CalendarOperation{{
			ID: "c1", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusPending,
			ExpectedLoad: 49,
			Body: icu.EventEx{
				Name: "Z2", Type: "Ride", Category: "WORKOUT",
				StartDateLocal: "2026-07-28T07:00:00",
			},
		}},
	})

	if !validation.Blocking {
		t.Fatalf("validation = %+v, want stored blocking honored", validation)
	}
}

func TestValidatePlanProposalStrictTargetsFromWeeklyLoads(t *testing.T) {
	t.Parallel()

	validation := icu.ValidatePlanProposal(&icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-08-02"},
		Intent: icu.PlanIntent{
			StrictTargets: true,
			WeeklyTargets: []icu.PlanWeeklyTarget{{
				WeekStart: "2026-07-27", TargetLoad: 200, Tolerance: 10,
			}},
		},
		WeeklyLoads: []icu.PlanWeekLoad{{
			WeekStart: "2026-07-27", TargetLoad: 200, Tolerance: 10,
			PlannedLoad: 49, Delta: -151, WithinTarget: false,
		}},
		Operations: []icu.CalendarOperation{{
			ID: "c1", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusPending,
			ExpectedLoad: 49,
			Body: icu.EventEx{
				Name: "Z2", Type: "Ride", Category: "WORKOUT",
				StartDateLocal: "2026-07-28T07:00:00",
			},
		}},
	})

	if !validation.Blocking {
		t.Fatalf("validation = %+v, want strict weekly targets blocking on accept path", validation)
	}
}

func TestValidatePlanProposalMarkedBlockingWithoutErrors(t *testing.T) {
	t.Parallel()

	validation := icu.ValidatePlanProposal(&icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-28"},
		Validation:    icu.PlanValidation{Blocking: true},
	})

	if !validation.Blocking || len(validation.Errors) == 0 {
		t.Fatalf("validation = %+v, want synthetic blocking error", validation)
	}
}

func TestValidatePlanProposalRejectsOperationOutsideScope(t *testing.T) {
	t.Parallel()

	validation := icu.ValidatePlanProposal(&icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-30"},
		Operations: []icu.CalendarOperation{{
			ID: "c1", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusPending,
			ExpectedLoad: 40,
			Body: icu.EventEx{
				Name: "Z2", Type: "Ride", Category: "WORKOUT",
				StartDateLocal: "2026-08-10T07:00:00",
			},
		}},
	})

	if !validation.Blocking {
		t.Fatalf("validation = %+v, want outside-scope error", validation)
	}
}

func TestValidatePlanProposalIgnoresAppliedZeroLoadWorkout(t *testing.T) {
	t.Parallel()

	validation := icu.ValidatePlanProposal(&icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-28"},
		Operations: []icu.CalendarOperation{{
			ID: "c1", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusApplied,
			Body: icu.EventEx{
				Name: "Z2", Type: "Ride", Category: "WORKOUT",
				StartDateLocal: "2026-07-28T07:00:00",
			},
		}},
	})

	if validation.Blocking {
		t.Fatalf("validation = %+v, want non-blocking for applied ops", validation)
	}
}

func TestBuildPlanProposalNOTEUnparseableDescIsWarning(t *testing.T) {
	t.Parallel()

	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate: "2026-07-27",
		Intent: icu.PlanIntent{
			Oldest: "2026-07-28", Newest: "2026-07-28",
			Constraints: icu.PlanConstraints{
				AllowCreate: true, DefaultStartTime: "07:00", DefaultType: "Ride", DefaultCategory: "NOTE",
			},
			Sessions: []icu.PlanSessionDraft{{
				Date: "2026-07-28", Name: "Note", Category: "NOTE",
				Desc: "totally freeform text that is not intervals syntax !!!",
			}},
		},
	})

	if proposal.Validation.Blocking || len(proposal.Operations) != 1 {
		t.Fatalf("validation=%+v ops=%d, want NOTE create with non-blocking parse warning path", proposal.Validation, len(proposal.Operations))
	}
}

func TestValidatePlanProposalNil(t *testing.T) {
	t.Parallel()

	validation := icu.ValidatePlanProposal(nil)

	if !validation.Blocking {
		t.Fatalf("validation = %+v, want blocking nil proposal", validation)
	}
}

func TestValidatePlanProposalRequiresBodyFields(t *testing.T) {
	t.Parallel()

	validation := icu.ValidatePlanProposal(&icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-28"},
		Operations: []icu.CalendarOperation{{
			ID: "c1", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusPending,
			Body: icu.EventEx{Name: "X", StartDateLocal: "2026-07-28"},
		}},
	})

	if !validation.Blocking {
		t.Fatalf("validation = %+v, want body field errors", validation)
	}
}

func TestPreviewCalendarOperationsCountsAppliedFailed(t *testing.T) {
	t.Parallel()

	preview := icu.PreviewCalendarOperations([]icu.CalendarOperation{
		{ID: "a", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusApplied},
		{ID: "f", Action: icu.CalendarActionUpdate, Status: icu.CalendarStatusFailed, EventID: 1},
		{ID: "p", Action: icu.CalendarActionCreate, Status: ""},
	})

	if preview.Applied != 1 || preview.Failed != 1 || preview.Pending != 1 {
		t.Fatalf("preview = %+v, want applied/failed/pending counts", preview)
	}
}

func TestCalendarOperationErrorsNil(t *testing.T) {
	t.Parallel()

	errors := icu.CalendarOperationErrors(nil)

	if len(errors) != 1 {
		t.Fatalf("errors = %v, want one nil error", errors)
	}
}

// TestPlanSchemaVersionsDidNotDrift pins the literal wire strings (T1b.7,
// D-7b), not the constants against themselves: PlanSchemaVersion moved to
// v2 because per-session FTP changes what a proposal's prescribed watts
// mean, while PlanIntentSchemaVersion stays v1 because the wave-1b addition
// to PlanSessionDraft (Indoor) is additive and does not change the meaning
// of an already-accepted intent. Comparing icu.PlanSchemaVersion against
// itself would pass regardless of what the constant holds; only a literal
// catches an accidental version bump or an accidental non-bump.
func TestPlanSchemaVersionsDidNotDrift(t *testing.T) {
	t.Parallel()

	if icu.PlanSchemaVersion != "icu.plan.v2" {
		t.Fatalf("PlanSchemaVersion = %q, want icu.plan.v2", icu.PlanSchemaVersion)
	}
}

func TestPlanIntentSchemaVersionDidNotDrift(t *testing.T) {
	t.Parallel()

	if icu.PlanIntentSchemaVersion != "icu.plan.intent.v1" {
		t.Fatalf("PlanIntentSchemaVersion = %q, want icu.plan.intent.v1", icu.PlanIntentSchemaVersion)
	}
}
