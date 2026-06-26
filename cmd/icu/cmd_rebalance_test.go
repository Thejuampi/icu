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

	start := rebalanceWellnessStart("bad-date")

	if start != "bad-date" {
		t.Fatalf("start = %q, want bad-date", start)
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
		"dry-run":     "true",
		"file":        file,
		"oldest":      "2026-06-01",
		"newest":      "2026-06-07",
		"now-date":    "2026-05-31",
		"target-load": "60",
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
