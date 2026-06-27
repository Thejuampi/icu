package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestRebalanceDryRunWritesProposalFile(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	cmd, ok := registry.Lookup("rebalance", "show")
	if !ok {
		t.Fatalf("missing rebalance command")
	}
	file := filepath.Join(t.TempDir(), "rebalance.json")

	err := cmd.Run(nil, rebalanceDryRunFlags(file), client)
	if err != nil {
		t.Fatalf("rebalance dry-run: %v", err)
	}
	proposal := readRebalanceTestProposal(t, file)

	if proposal.SchemaVersion != icu.RebalanceSchemaVersion {
		t.Fatalf("schema version = %q, want %q", proposal.SchemaVersion, icu.RebalanceSchemaVersion)
	}
}

func TestRebalanceAcceptAppliesPendingOperation(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	dryRun, ok := registry.Lookup("rebalance", "show")
	if !ok {
		t.Fatalf("missing rebalance command")
	}
	accept, ok := registry.Lookup("rebalance", "accept")
	if !ok {
		t.Fatalf("missing rebalance accept command")
	}
	file := filepath.Join(t.TempDir(), "rebalance.json")
	if err := dryRun.Run(nil, rebalanceDryRunFlags(file), client); err != nil {
		t.Fatalf("rebalance dry-run: %v", err)
	}

	err := accept.Run(nil, map[string]string{"file": file}, client)
	if err != nil {
		t.Fatalf("rebalance accept: %v", err)
	}
	proposal := readRebalanceTestProposal(t, file)

	if proposal.Apply.Applied == 0 {
		t.Fatalf("applied = %d, want non-zero", proposal.Apply.Applied)
	}
}

func TestRebalanceAcceptSkipsNonPendingOperation(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	accept, ok := registry.Lookup("rebalance", "accept")
	if !ok {
		t.Fatalf("missing rebalance accept command")
	}
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Operations: []icu.RebalanceOperation{{
			ID:     "already-applied",
			Action: icu.RebalanceActionCreate,
			Status: icu.RebalanceStatusApplied,
			Body:   icu.EventEx{Name: "Workout"},
		}},
	})

	err := accept.Run(nil, map[string]string{"file": file}, client)
	if err != nil {
		t.Fatalf("rebalance accept: %v", err)
	}
	proposal := readRebalanceTestProposal(t, file)

	if proposal.Apply.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", proposal.Apply.Skipped)
	}
}

func TestRebalanceAcceptFailsOnSourceHashDrift(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	accept, ok := registry.Lookup("rebalance", "accept")
	if !ok {
		t.Fatalf("missing rebalance accept command")
	}
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Operations: []icu.RebalanceOperation{{
			ID:         "drift",
			Action:     icu.RebalanceActionUpdate,
			EventID:    1,
			SourceHash: "stale",
			Status:     icu.RebalanceStatusPending,
			Body:       icu.EventEx{Name: "Workout"},
		}},
	})

	err := accept.Run(nil, map[string]string{"file": file}, client)
	if err != nil {
		t.Fatalf("rebalance accept: %v", err)
	}
	proposal := readRebalanceTestProposal(t, file)

	if proposal.Apply.Failed != 1 {
		t.Fatalf("failed = %d, want 1", proposal.Apply.Failed)
	}
}

func TestRebalanceAcceptAppliesCancelOperation(t *testing.T) {
	t.Parallel()

	registry, client := rebalanceTestRegistryClient(t)
	accept, ok := registry.Lookup("rebalance", "accept")
	if !ok {
		t.Fatalf("missing rebalance accept command")
	}
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Operations: []icu.RebalanceOperation{{
			ID:      "cancel",
			Action:  icu.RebalanceActionCancel,
			EventID: 1,
			Status:  icu.RebalanceStatusPending,
		}},
	})

	err := accept.Run(nil, map[string]string{"file": file}, client)
	if err != nil {
		t.Fatalf("rebalance accept: %v", err)
	}
	proposal := readRebalanceTestProposal(t, file)

	if proposal.Apply.Failed != 1 {
		t.Fatalf("failed = %d, want 1", proposal.Apply.Failed)
	}
}

func TestReadRebalanceSportSettingsUsesAthleteEndpoint(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"type":"Ride","ftp":285}`))
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	_, err := readRebalanceSportSettings(client, map[string]string{"type": "Ride"})
	if err != nil {
		t.Fatalf("read sport settings: %v", err)
	}

	if !strings.Contains(gotPath, "/athlete/0/sport-settings/Ride") {
		t.Fatalf("path = %q, want athlete sport-settings endpoint", gotPath)
	}
}

func TestReadRebalanceSportSettingsSkipsWhenTypeMissing(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)

	settings, err := readRebalanceSportSettings(client, map[string]string{})
	if err != nil {
		t.Fatalf("read sport settings: %v", err)
	}
	if settings == nil {
		t.Fatalf("settings = nil, want empty settings value")
	}
	if settings.FTP != 0 || len(settings.PowerZones) != 0 {
		t.Fatalf("settings = %#v, want zero-value settings", settings)
	}
}

func TestReadRebalanceWellnessUsesExplicitLookback(t *testing.T) {
	t.Parallel()

	var gotOldest string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		gotOldest = request.URL.Query().Get("oldest")
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	_, err := readRebalanceWellness(client, "2026-06-22", "2026-06-28", map[string]string{"wellness-lookback-days": "14"})
	if err != nil {
		t.Fatalf("read wellness: %v", err)
	}
	if gotOldest != "2026-06-08" {
		t.Fatalf("oldest = %q, want 2026-06-08", gotOldest)
	}
}

func TestRebalanceDryRunRejectsMaxHRFlag(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	flags := rebalanceDryRunFlags(filepath.Join(t.TempDir(), "rebalance.json"))
	flags["max-hr"] = "140"

	_, err := readRebalanceInput(flags, client)

	if err == nil {
		t.Fatalf("err = nil, want unsupported max-hr")
	}
}

func TestRebalanceConstraintsFromFlagsLeavesTypeAndTargetEmptyWithoutFlags(t *testing.T) {
	t.Parallel()

	constraints := rebalanceConstraintsFromFlags(map[string]string{
		"allow-past":  "true",
		"allow-today": "true",
	})

	if !constraints.AllowPast || !constraints.AllowToday {
		t.Fatalf("allow flags = %+v, want both true", constraints)
	}
	if constraints.SportType != "" || constraints.WorkoutTarget != "" {
		t.Fatalf("type/target = %q/%q, want empty", constraints.SportType, constraints.WorkoutTarget)
	}
}

func TestRebalanceDryRunRequiresDryRunFlag(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)

	_, err := readRebalanceInput(map[string]string{}, client)

	if err == nil {
		t.Fatalf("err = nil, want missing dry-run")
	}
}

func TestRebalanceDryRunRequiresOldestFlag(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)

	_, err := readRebalanceInput(map[string]string{"dry-run": "true"}, client)

	if err == nil {
		t.Fatalf("err = nil, want missing oldest")
	}
}

func TestRebalanceAcceptRequiresFileFlag(t *testing.T) {
	t.Parallel()

	_, err := readRebalanceProposal(map[string]string{})

	if err == nil {
		t.Fatalf("err = nil, want missing file")
	}
}

func TestRebalanceAcceptRejectsInvalidJSONFile(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "rebalance.json")
	if err := os.WriteFile(file, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid proposal: %v", err)
	}

	_, err := readRebalanceProposal(map[string]string{"file": file})

	if err == nil {
		t.Fatalf("err = nil, want parse error")
	}
}

func TestWriteRebalanceProposalRequiresFileFlag(t *testing.T) {
	t.Parallel()

	err := writeRebalanceProposal(map[string]string{}, &icu.RebalanceProposalFile{})

	if err == nil {
		t.Fatalf("err = nil, want missing file")
	}
}

func TestApplyRebalanceOperationRejectsUnsupportedAction(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	operation := icu.RebalanceOperation{ID: "bad", Action: "move", Status: icu.RebalanceStatusPending}

	result := applyRebalanceOperation(client, &operation)

	if result.Status != icu.RebalanceStatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestRebalanceWellnessStartKeepsInvalidDate(t *testing.T) {
	t.Parallel()

	start := rebalanceWellnessStart("bad-date", rebalanceWellnessLookbackDays)

	if start != "bad-date" {
		t.Fatalf("start = %q, want bad-date", start)
	}
}

func TestRebalanceWellnessStartUsesDefaultWhenLookbackIsZero(t *testing.T) {
	t.Parallel()

	start := rebalanceWellnessStart("2026-06-22", 0)

	if start != "2026-05-11" {
		t.Fatalf("start = %q, want 2026-05-11", start)
	}
}

func rebalanceTestRegistryClient(t *testing.T) (*CommandRegistry, *icu.Client) {
	t.Helper()

	registry := NewCommandRegistry()
	registerAllCommands(registry)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(functionalJSONResponse(request.Method, request.URL.Path)))
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	return registry, client
}

func rebalanceDryRunFlags(file string) map[string]string {
	return map[string]string{
		"allocation-basis":      "explicit_equal",
		"dry-run":               "true",
		"duration-step-minutes": "5",
		"file":                  file,
		"min-session-minutes":   "20",
		"newest":                "2026-06-07",
		"now-date":              "2026-05-31",
		"oldest":                "2026-06-01",
		"start-time":            "07:00",
		"target":                "POWER",
		"target-load":           "60",
		"target-tolerance":      "10",
		"type":                  "Ride",
		"z1-if":                 "0.55",
		"z2-if":                 "0.65",
	}
}

func readRebalanceTestProposal(t *testing.T, file string) icu.RebalanceProposalFile {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read proposal: %v", err)
	}
	var proposal icu.RebalanceProposalFile
	if err := json.Unmarshal(data, &proposal); err != nil {
		t.Fatalf("parse proposal: %v", err)
	}

	return proposal
}

func writeRebalanceTestProposal(t *testing.T, proposal *icu.RebalanceProposalFile) string {
	t.Helper()

	file := filepath.Join(t.TempDir(), "rebalance.json")
	data, err := icu.MarshalRebalanceProposal(proposal)
	if err != nil {
		t.Fatalf("marshal proposal: %v", err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	return file
}

func TestRebalanceApproveRequiresReason(t *testing.T) {
	t.Parallel()

	registry, _ := rebalanceTestRegistryClient(t)
	approve, ok := registry.Lookup("rebalance", "approve")
	if !ok {
		t.Fatalf("missing rebalance approve command")
	}
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
	})

	err := approve.Run(nil, map[string]string{"file": file}, nil)
	if err == nil {
		t.Fatalf("err = nil, want missing reason")
	}
}

func TestRebalanceApproveBindsHashAndVerifies(t *testing.T) {
	t.Parallel()

	registry, _ := rebalanceTestRegistryClient(t)
	approve, ok := registry.Lookup("rebalance", "approve")
	if !ok {
		t.Fatalf("missing rebalance approve command")
	}
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Policy:        &icu.RebalancePolicy{Strategy: icu.RebalanceStrategyAdaptiveBidirectional, Level: "0", Mode: "preserve-structure", CurveMethod: icu.RebalanceCurveMethodPCHIP},
		Envelope:      &icu.RebalanceEnvelopeReport{Envelope: icu.RebalanceEnvelope{Low: fRat(200), Current: fRat(300), High: fRat(400)}, LowSource: "data_robust_fence", HighSource: "data_robust_fence"},
	})

	err := approve.Run(nil, map[string]string{"file": file, "reason": "ramp-up"}, nil)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	proposal := readRebalanceTestProposal(t, file)
	if proposal.Approve == nil || !proposal.Approve.Verified {
		t.Fatalf("approve not verified: %+v", proposal.Approve)
	}
	if proposal.Approve.RecalcHash == "" {
		t.Fatalf("missing recalcHash")
	}
}

func TestRebalanceApproveOutsideEnvelopeRequiresLimits(t *testing.T) {
	t.Parallel()

	registry, _ := rebalanceTestRegistryClient(t)
	approve, _ := registry.Lookup("rebalance", "approve")
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Envelope:      &icu.RebalanceEnvelopeReport{OutsideEnvelope: true, Envelope: icu.RebalanceEnvelope{Low: fRat(200), Current: fRat(300), High: fRat(310)}},
	})

	err := approve.Run(nil, map[string]string{"file": file, "reason": "override"}, nil)
	if err == nil {
		t.Fatalf("err = nil, want explicit limits required for outside-envelope")
	}
}

func TestRebalanceAcceptRejectsV1SchemaFile(t *testing.T) {
	t.Parallel()

	registry, _ := rebalanceTestRegistryClient(t)
	accept, _ := registry.Lookup("rebalance", "accept")
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: "icu.rebalance.v1",
	})

	err := accept.Run(nil, map[string]string{"file": file}, nil)
	if err == nil {
		t.Fatalf("err = nil, want v1 schema rejected")
	}
}

func TestRebalanceAcceptRejectsUnapprovedOutsideEnvelope(t *testing.T) {
	t.Parallel()

	registry, _ := rebalanceTestRegistryClient(t)
	accept, _ := registry.Lookup("rebalance", "accept")
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Envelope:      &icu.RebalanceEnvelopeReport{OutsideEnvelope: true, Envelope: icu.RebalanceEnvelope{Low: fRat(200), Current: fRat(300), High: fRat(310)}},
	})

	err := accept.Run(nil, map[string]string{"file": file}, nil)
	if err == nil {
		t.Fatalf("err = nil, want approve required for outside-envelope")
	}
}

func TestRebalanceAcceptRejectsApprovalHashMismatch(t *testing.T) {
	t.Parallel()

	registry, _ := rebalanceTestRegistryClient(t)
	accept, _ := registry.Lookup("rebalance", "accept")
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Envelope: &icu.RebalanceEnvelopeReport{
			Envelope: icu.RebalanceEnvelope{Low: fRat(200), Current: fRat(300), High: fRat(320)},
		},
		Approve: &icu.RebalanceApprove{
			Reason:         "override",
			ProvidedLimits: true,
			Verified:       true,
			RecalcHash:     "stale",
		},
	})

	err := accept.Run(nil, map[string]string{"file": file}, nil)
	if err == nil {
		t.Fatalf("err = nil, want approval hash mismatch")
	}
}

func fRat(v float64) icu.RebalanceRat { r, _ := icu.NewRebalanceRatFromDyadic(v); return r }
