package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestPlanShowWritesProposalFile(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	cmd, ok := registry.Lookup("plan", "show")
	if !ok {
		t.Fatalf("missing plan show command")
	}
	dir := t.TempDir()
	intentFile := filepath.Join(dir, "intent.json")
	planFile := filepath.Join(dir, "plan.json")
	writePlanTestIntent(t, intentFile, samplePlanIntent())

	err := cmd.Run(nil, map[string]string{
		"file":        planFile,
		"intent-file": intentFile,
		"now-date":    "2026-07-27",
		"type":        "Ride",
	}, client)
	if err != nil {
		t.Fatalf("plan show: %v", err)
	}
	proposal := readPlanTestProposal(t, planFile)

	if proposal.SchemaVersion != icu.PlanSchemaVersion || len(proposal.Operations) == 0 {
		t.Fatalf("proposal = %+v, want plan schema with operations", proposal)
	}
}

func TestPlanPreviewDoesNotMutate(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	show, _ := registry.Lookup("plan", "show")
	preview, ok := registry.Lookup("plan", "preview")
	if !ok {
		t.Fatalf("missing plan preview command")
	}
	dir := t.TempDir()
	intentFile := filepath.Join(dir, "intent.json")
	planFile := filepath.Join(dir, "plan.json")
	writePlanTestIntent(t, intentFile, samplePlanIntent())
	if err := show.Run(nil, map[string]string{
		"file": planFile, "intent-file": intentFile, "now-date": "2026-07-27", "type": "Ride",
	}, client); err != nil {
		t.Fatalf("plan show: %v", err)
	}

	err := preview.Run(nil, map[string]string{"file": planFile, "no-live-check": "true"}, client)
	if err != nil {
		t.Fatalf("plan preview: %v", err)
	}
	proposal := readPlanTestProposal(t, planFile)

	if proposal.Apply != nil {
		t.Fatalf("apply = %+v, want nil after preview", proposal.Apply)
	}
}

func TestPlanAcceptAppliesPendingOperation(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	show, _ := registry.Lookup("plan", "show")
	accept, ok := registry.Lookup("plan", "accept")
	if !ok {
		t.Fatalf("missing plan accept command")
	}
	dir := t.TempDir()
	intentFile := filepath.Join(dir, "intent.json")
	planFile := filepath.Join(dir, "plan.json")
	writePlanTestIntent(t, intentFile, samplePlanIntent())
	if err := show.Run(nil, map[string]string{
		"file": planFile, "intent-file": intentFile, "now-date": "2026-07-27", "type": "Ride",
	}, client); err != nil {
		t.Fatalf("plan show: %v", err)
	}

	err := accept.Run(nil, map[string]string{"file": planFile}, client)
	if err != nil {
		t.Fatalf("plan accept: %v", err)
	}
	proposal := readPlanTestProposal(t, planFile)

	if proposal.Apply == nil || proposal.Apply.Applied == 0 {
		t.Fatalf("apply = %+v, want applied > 0", proposal.Apply)
	}
}

func TestPlanAcceptRejectsBlockingValidation(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	accept, _ := registry.Lookup("plan", "accept")
	file := writePlanTestProposal(t, &icu.PlanProposalFile{
		SchemaVersion: "bad",
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-28"},
	})

	err := accept.Run(nil, map[string]string{"file": file}, client)

	if err == nil {
		t.Fatalf("err = nil, want validation error")
	}
}

func TestRebalancePreviewCommandRegistered(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	preview, ok := registry.Lookup("rebalance", "preview")
	if !ok {
		t.Fatalf("missing rebalance preview command")
	}
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Operations: []icu.RebalanceOperation{{
			ID:     "c1",
			Action: icu.RebalanceActionCreate,
			Status: icu.RebalanceStatusPending,
			Body: icu.EventEx{
				Name:           "Workout",
				Type:           "Ride",
				Target:         "POWER",
				StartDateLocal: "2026-07-28T07:00:00",
				Category:       "WORKOUT",
			},
			ExpectedLoad: 40,
		}},
		Constraints: icu.RebalanceConstraints{
			TargetTolerance:     10,
			Z2IF:                0.65,
			MinSessionMinutes:   20,
			DurationStepMinutes: 5,
			SportType:           "Ride",
			WorkoutTarget:       "POWER",
		},
	})

	err := preview.Run(nil, map[string]string{"file": file, "no-live-check": "true"}, client)
	if err != nil {
		t.Fatalf("rebalance preview: %v", err)
	}
}

func TestReadPlanIntentRequiresFile(t *testing.T) {
	t.Parallel()

	_, err := readPlanIntent(map[string]string{})

	if err == nil {
		t.Fatalf("err = nil, want missing intent-file")
	}
}

func TestLiveCheckCalendarOperationsReportsDrift(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	warnings := liveCheckCalendarOperations(client, []icu.CalendarOperation{{
		ID:         "u1",
		Action:     icu.CalendarActionUpdate,
		EventID:    1,
		SourceHash: "stale-hash",
		Status:     icu.CalendarStatusPending,
	}})

	if len(warnings) == 0 {
		t.Fatalf("warnings = %v, want source hash drift", warnings)
	}
}

func TestPlanPreviewLiveCheckAddsWarnings(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	preview, _ := registry.Lookup("plan", "preview")
	file := writePlanTestProposal(t, &icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-28", NowDate: "2026-07-27"},
		Baseline:      icu.PlanBaseline{CompletedLoad: 999},
		Operations: []icu.CalendarOperation{{
			ID: "c1", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusPending,
			ExpectedLoad: 40,
			Body: icu.EventEx{
				Name: "Z2", Type: "Ride", Category: "WORKOUT",
				StartDateLocal: "2026-07-28T07:00:00",
			},
		}},
	})

	err := preview.Run(nil, map[string]string{"file": file}, client)
	if err != nil {
		t.Fatalf("plan preview: %v", err)
	}
}

func TestPlanAcceptRejectsStoredBlockingWithoutNetwork(t *testing.T) {
	t.Parallel()

	registry, _ := rebalanceTestRegistryClient(t)
	accept, _ := registry.Lookup("plan", "accept")
	file := writePlanTestProposal(t, &icu.PlanProposalFile{
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
	// nil client would panic on network; validation must fail first.
	err := accept.Run(nil, map[string]string{"file": file}, nil)

	if err == nil || !strings.Contains(err.Error(), "validate plan proposal") {
		t.Fatalf("err = %v, want validation error before network", err)
	}
}

func TestPlanPreviewSurfacesBlockingValidation(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	preview, _ := registry.Lookup("plan", "preview")
	file := writePlanTestProposal(t, &icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-30"},
		Validation: icu.PlanValidation{
			Blocking: true,
			Errors:   []string{"strict target miss"},
			Warnings: []string{"soft note"},
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

	err := preview.Run(nil, map[string]string{"file": file, "no-live-check": "true"}, client)

	if err == nil || !strings.Contains(err.Error(), "strict target miss") {
		t.Fatalf("err = %v, want blocking preview error", err)
	}
}

func TestPlanCommandsRejectUnknownFlags(t *testing.T) {
	t.Parallel()

	cmd := planPreviewCommand()

	err := validateCommandInput(cmd, nil, map[string]string{"file": "x.json", "bogus": "1"})

	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("err = %v, want unknown flag", err)
	}
}

func TestRebalanceCommandsRejectUnknownFlags(t *testing.T) {
	t.Parallel()

	cmd := rebalanceDryRunCommand()

	err := validateCommandInput(cmd, nil, map[string]string{
		"file": "x.json", "oldest": "2026-01-01", "newest": "2026-01-07", "bogus": "1",
	})

	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("err = %v, want unknown flag", err)
	}
}

func TestRebalanceShowUsageIncludesMaxCaps(t *testing.T) {
	t.Parallel()

	cmd := rebalanceDryRunCommand()

	if !strings.Contains(cmd.Usage, "--max-intensity") || !strings.Contains(cmd.Usage, "--max-watts") {
		t.Fatalf("usage = %q, want max-intensity and max-watts", cmd.Usage)
	}
}

func TestReadPlanIntentRejectsBadSchema(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "intent.json")
	if err := os.WriteFile(file, []byte(`{"schemaVersion":"nope"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := readPlanIntent(map[string]string{"intent-file": file})

	if err == nil {
		t.Fatalf("err = nil, want schema error")
	}
}

func TestReadPlanProposalRequiresFile(t *testing.T) {
	t.Parallel()

	_, err := readPlanProposal(map[string]string{})

	if err == nil {
		t.Fatalf("err = nil, want missing file")
	}
}

func TestWritePlanProposalRequiresFile(t *testing.T) {
	t.Parallel()

	err := writePlanProposal(map[string]string{}, &icu.PlanProposalFile{})

	if err == nil {
		t.Fatalf("err = nil, want missing file")
	}
}

func TestPlanShowRequiresIntentFile(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	cmd, _ := registry.Lookup("plan", "show")

	err := cmd.Run(nil, map[string]string{"file": filepath.Join(t.TempDir(), "plan.json")}, client)

	if err == nil {
		t.Fatalf("err = nil, want missing intent-file")
	}
}

func TestPlanAcceptDetectsBaselineDrift(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	accept, _ := registry.Lookup("plan", "accept")
	file := writePlanTestProposal(t, &icu.PlanProposalFile{
		SchemaVersion: icu.PlanSchemaVersion,
		Scope:         icu.PlanScope{StartDate: "2026-07-28", EndDate: "2026-07-28", NowDate: "2026-07-27"},
		Baseline:      icu.PlanBaseline{CompletedLoad: 999, LockedPlannedLoad: 999},
		Operations: []icu.CalendarOperation{{
			ID: "c1", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusPending,
			ExpectedLoad: 40,
			Body: icu.EventEx{
				Name: "Z2", Type: "Ride", Category: "WORKOUT",
				StartDateLocal: "2026-07-28T07:00:00",
			},
		}},
	})

	err := accept.Run(nil, map[string]string{"file": file}, client)

	if err == nil {
		t.Fatalf("err = nil, want baseline drift")
	}
}

func TestRebalancePreviewLiveCheck(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	preview, _ := registry.Lookup("rebalance", "preview")
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Scope:         icu.RebalanceScope{StartDate: "2026-06-01", EndDate: "2026-06-07"},
		Baseline:      icu.RebalanceEvaluation{CompletedLoad: 0, LockedPlannedLoad: 0},
		Operations: []icu.RebalanceOperation{{
			ID: "c1", Action: icu.RebalanceActionCreate, Status: icu.RebalanceStatusPending,
			ExpectedLoad: 40,
			Body: icu.EventEx{
				Name: "Workout", Type: "Ride", Target: "POWER", Category: "WORKOUT",
				StartDateLocal: "2026-06-02T07:00:00",
			},
		}},
		Constraints: icu.RebalanceConstraints{
			TargetTolerance: 10, Z2IF: 0.65, MinSessionMinutes: 20, DurationStepMinutes: 5,
			SportType: "Ride", WorkoutTarget: "POWER",
		},
	})

	err := preview.Run(nil, map[string]string{"file": file}, client)
	if err != nil {
		t.Fatalf("rebalance preview: %v", err)
	}
}

func samplePlanIntent() icu.PlanIntent {
	return icu.PlanIntent{
		SchemaVersion: icu.PlanIntentSchemaVersion,
		Oldest:        "2026-07-28",
		Newest:        "2026-07-28",
		FTP:           300,
		Constraints: icu.PlanConstraints{
			AllowCreate:      true,
			AllowUpdate:      true,
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
	}
}

func writePlanTestIntent(t *testing.T, path string, intent icu.PlanIntent) {
	t.Helper()
	data, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatalf("marshal intent: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write intent: %v", err)
	}
}

func writePlanTestProposal(t *testing.T, proposal *icu.PlanProposalFile) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "plan.json")
	data, err := icu.MarshalPlanProposal(proposal)
	if err != nil {
		t.Fatalf("marshal proposal: %v", err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	return file
}

func readPlanTestProposal(t *testing.T, file string) icu.PlanProposalFile {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read proposal: %v", err)
	}
	var proposal icu.PlanProposalFile
	if err := json.Unmarshal(data, &proposal); err != nil {
		t.Fatalf("parse proposal: %v", err)
	}

	return proposal
}
