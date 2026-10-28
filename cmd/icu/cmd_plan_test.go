package main

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	}, client, io.Discard)
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
	}, client, io.Discard); err != nil {
		t.Fatalf("plan show: %v", err)
	}

	err := preview.Run(nil, map[string]string{"file": planFile, "no-live-check": "true"}, client, io.Discard)
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
	}, client, io.Discard); err != nil {
		t.Fatalf("plan show: %v", err)
	}

	err := accept.Run(nil, map[string]string{"file": planFile}, client, io.Discard)
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

	err := accept.Run(nil, map[string]string{"file": file}, client, io.Discard)

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

	err := preview.Run(nil, map[string]string{"file": file, "no-live-check": "true"}, client, io.Discard)
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

	err := preview.Run(nil, map[string]string{"file": file}, client, io.Discard)
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
	err := accept.Run(nil, map[string]string{"file": file}, nil, io.Discard)

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

	err := preview.Run(nil, map[string]string{"file": file, "no-live-check": "true"}, client, io.Discard)

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

	err := cmd.Run(nil, map[string]string{"file": filepath.Join(t.TempDir(), "plan.json")}, client, io.Discard)

	if err == nil {
		t.Fatalf("err = nil, want missing intent-file")
	}
}

func TestReadPlanInputRejectsInvalidSessionEnvironmentFlag(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	intentFile := filepath.Join(t.TempDir(), "intent.json")
	writePlanTestIntent(t, intentFile, samplePlanIntent())

	_, err := readPlanInput(map[string]string{
		"intent-file":         intentFile,
		"session-environment": "sideways",
	}, client)

	if err == nil {
		t.Fatalf("err = nil, want rejection of invalid session-environment value")
	}
}

// TestReadPlanInputRejectsInvertedIntentDateRange is Stage 6 R3: the CLI-level
// check that rebalance now has (validateAnalysisRange) was also missing on
// plan's read path; only the proposal-validation layer caught an inverted
// window there, a third, inconsistent answer across three commands.
func TestReadPlanInputRejectsInvertedIntentDateRange(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	intent := samplePlanIntent()
	intent.Oldest = "2026-07-28"
	intent.Newest = "2026-07-20"
	intentFile := filepath.Join(t.TempDir(), "intent.json")
	writePlanTestIntent(t, intentFile, intent)

	_, err := readPlanInput(map[string]string{"intent-file": intentFile}, client)

	if !errors.Is(err, errInvalidValue) {
		t.Fatalf("err = %v, want errInvalidValue for an inverted intent date range", err)
	}
}

// TestReadPlanInputRejectsMalformedNowDate is Stage 6 R11: plan reaches the
// same NowDate-as-raw-string lock decision rebalance does (dateLockedForPlan
// -> dateLocked -> rebalanceDateLockedByTime), through --now-date defaulting
// unvalidated straight into PlanInput.NowDate. Pin the rejection at the same
// boundary rebalance now has, so plan cannot construct an input carrying an
// unparseable lock boundary either.
func TestReadPlanInputRejectsMalformedNowDate(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	intentFile := filepath.Join(t.TempDir(), "intent.json")
	writePlanTestIntent(t, intentFile, samplePlanIntent())

	_, err := readPlanInput(map[string]string{"intent-file": intentFile, "now-date": "!"}, client)

	if !errors.Is(err, errInvalidValue) {
		t.Fatalf("err = %v, want errInvalidValue for a malformed --now-date", err)
	}
}

func TestReadPlanInputSessionEnvironmentFlagOverridesIntentFile(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	intent := samplePlanIntent()
	intent.Constraints.SessionEnvironment = icu.SessionEnvironmentIndoor
	intentFile := filepath.Join(t.TempDir(), "intent.json")
	writePlanTestIntent(t, intentFile, intent)

	input, err := readPlanInput(map[string]string{
		"intent-file":         intentFile,
		"session-environment": "outdoor",
	}, client)
	if err != nil {
		t.Fatalf("read plan input: %v", err)
	}

	if input.Intent.Constraints.SessionEnvironment != icu.SessionEnvironmentOutdoor {
		t.Fatalf("session environment = %q, want outdoor (flag overrides intent file)",
			input.Intent.Constraints.SessionEnvironment)
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

	err := accept.Run(nil, map[string]string{"file": file}, client, io.Discard)

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

	err := preview.Run(nil, map[string]string{"file": file}, client, io.Discard)
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

//nolint:paralleltest // swaps the process-wide config store.
func TestPlanLoadProjectionStartsFromWellness(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)
	if err := icu.SaveConfig(&icu.Config{Profile: &icu.TrainingProfile{MaxRampPerWeek: 50}}); err != nil {
		t.Fatal(err)
	}

	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		methods = append(methods, request.Method)
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[{"id":"2026-10-25","ctl":40,"atl":45},` +
				`{"id":"2026-10-26","ctl":41,"atl":47},{"id":"2026-11-01","ctl":30,"atl":10}]`))
		case strings.HasSuffix(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[{"id":9,"category":"WORKOUT","start_date_local":"2026-10-28T07:00:00",` +
				`"icu_training_load":70}]`))
		default:
			_, _ = response.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	proposal := &icu.PlanProposalFile{
		Scope: icu.PlanScope{EndDate: "2026-11-01"},
		Operations: []icu.CalendarOperation{{
			Action: icu.CalendarActionCreate, ExpectedLoad: 300,
			Body: icu.EventEx{StartDateLocal: "2026-10-27T07:00:00"},
		}},
	}
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

	got, err := planLoadProjection(client, proposal, map[string]string{"timezone": "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	// Starts from yesterday's row so today's load is projected too.
	if got.StartDate != "2026-10-25" || got.StartCTL != 40 || got.StartSource != "wellness" ||
		got.EndDate != "2026-11-01" || got.MaxRampPerWeek != 50 || len(got.Warnings) != 0 {
		t.Fatalf("projection = %+v", got)
	}
	capped, err := planLoadProjection(client, proposal, map[string]string{"max-ramp": "1", "timezone": "UTC"}, now)
	if err != nil || len(capped.Warnings) != 1 || !strings.Contains(capped.Warnings[0], "exceeds the 1.00/week cap") {
		t.Fatalf("capped = %+v, %v", capped, err)
	}
	for _, method := range methods {
		if method != http.MethodGet {
			t.Fatalf("projection issued %s", method)
		}
	}

	empty := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`[]`))
	}))
	t.Cleanup(empty.Close)
	emptyClient := icu.NewClient("k", "i1", icu.WithHTTPClient(empty.Client()), icu.WithBaseURL(empty.URL))
	if _, err := planLoadProjection(emptyClient, proposal, nil, now); err == nil {
		t.Fatal("projection without wellness succeeded")
	}
}

func TestPlanLoadProjectionUsesNowDateAndActualLoads(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)

	var activityQuery string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("unexpected %s", request.Method)
		}
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[{"id":"2026-10-20","ctl":40,"atl":45}]`))
		case strings.HasSuffix(request.URL.Path, "/activities"):
			activityQuery = request.URL.Query().Get("oldest") + ".." + request.URL.Query().Get("newest")
			_, _ = response.Write([]byte(`[{"id":"a1","start_date_local":"2026-10-21T08:00:00","icu_training_load":60},` +
				`{"id":"a2","start_date_local":"2026-10-21T18:00:00","icu_training_load":40}]`))
		default:
			_, _ = response.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	proposal := &icu.PlanProposalFile{Scope: icu.PlanScope{EndDate: "2026-10-23"}}

	got, err := planLoadProjection(client, proposal, map[string]string{"now-date": "2026-10-22"},
		time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if activityQuery != "2026-07-24..2026-10-22" || got.StartDate != "2026-10-20" || got.Weeks[0].Load != 100 {
		t.Fatalf("query=%s projection=%+v", activityQuery, got)
	}
	if _, err := planLoadProjection(client, proposal, map[string]string{"now-date": "bad"}, time.Now()); err == nil {
		t.Fatal("expected --now-date validation error")
	}
}

func TestPlanLoadProjectionSplitsFitnessAndFatigueLoads(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(request.URL.Path, "/wellness"):
			// 09-11: strength only, CTL load 0, ATL load 42 (live shape).
			_, _ = response.Write([]byte(`[{"id":"2026-09-10","ctl":40,"atl":40},` +
				`{"id":"2026-09-11","ctl":39,"atl":44,"ctlLoad":0,"atlLoad":42}]`))
		case strings.HasSuffix(request.URL.Path, "/activities"):
			if !strings.Contains(request.URL.Query().Get("fields"), "paired_event_id") {
				t.Error("activities read must request paired_event_id")
			}
			_, _ = response.Write([]byte(`[{"id":"a","type":"WeightTraining","start_date_local":"2026-09-11T17:00:00",` +
				`"icu_training_load":42},{"id":"b","type":"Ride","start_date_local":"2026-09-12T07:00:00",` +
				`"icu_training_load":50,"paired_event_id":7}]`))
		case strings.HasSuffix(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[{"id":7,"category":"WORKOUT","type":"Ride","start_date_local":"2026-09-12T07:00:00",` +
				`"icu_training_load":50},{"id":8,"category":"WORKOUT","type":"WeightTraining",` +
				`"start_date_local":"2026-09-13T17:00:00","icu_training_load":30}]`))
		default:
			_, _ = response.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	proposal := &icu.PlanProposalFile{Scope: icu.PlanScope{EndDate: "2026-09-13"}}

	got, err := planLoadProjection(client, proposal, map[string]string{"now-date": "2026-09-12"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Start 09-11 (last row before today). Today: completed ride 50, its
	// paired event not added again. 09-13: strength 30 is ATL-only.
	k42, k7 := 1-math.Exp(-1.0/42), 1-math.Exp(-1.0/7)
	ctl, atl := 39.0, 44.0
	ctl += (50 - ctl) * k42
	atl += (50 - atl) * k7
	ctl += (0 - ctl) * k42
	atl += (30 - atl) * k7
	if got.StartDate != "2026-09-11" || math.Abs(got.EndCTL-ctl) > 0.01 || math.Abs(got.EndATL-atl) > 0.01 {
		t.Fatalf("projection = %+v, want CTL %.2f ATL %.2f", got, ctl, atl)
	}
}

func TestPlanLoadProjectionPersistsLearnedTypesAndHonorsOverrides(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)
	if err := icu.SaveConfigWithAction(&icu.Config{AthleteID: "i1", Profile: &icu.TrainingProfile{
		FitnessLoadTypes: map[string]string{"Yoga": "atl-only"},
	}}, "config_set"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[{"id":"2026-09-11","ctl":39,"atl":44,"ctlLoad":0,"atlLoad":42}]`))
		case strings.HasSuffix(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[{"id":"a","type":"WeightTraining","start_date_local":"2026-09-11T17:00:00",` +
				`"icu_training_load":42}]`))
		case strings.HasSuffix(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[{"id":8,"category":"WORKOUT","type":"Yoga",` +
				`"start_date_local":"2026-09-13T07:00:00","icu_training_load":30},` +
				`{"id":9,"category":"WORKOUT","type":"Rowing","start_date_local":"2026-09-13T17:00:00","icu_training_load":0}]`))
		default:
			_, _ = response.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	proposal := &icu.PlanProposalFile{Scope: icu.PlanScope{EndDate: "2026-09-13"}}

	// Without --save-learned the learned types are used in memory only.
	if _, err := planLoadProjection(client, proposal, map[string]string{"now-date": "2026-09-12"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := icu.LoadConfig(); cfg.Profile.LearnedFitnessLoadTypes != nil {
		t.Fatalf("preview wrote config without --save-learned: %+v", cfg.Profile)
	}
	got, err := planLoadProjection(client, proposal,
		map[string]string{"now-date": "2026-09-12", "save-learned": "true"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	k42 := 1 - math.Exp(-1.0/42)
	ctl := 39 * (1 - k42) * (1 - k42) // Yoga is manually ATL-only
	if math.Abs(got.EndCTL-ctl) > 0.01 {
		t.Fatalf("CTL %.3f, want %.3f", got.EndCTL, ctl)
	}
	if !strings.Contains(strings.Join(got.Warnings, "|"), "whether Rowing counts toward CTL") {
		t.Fatalf("warnings = %v", got.Warnings)
	}
	cfg, err := icu.LoadConfig()
	if err != nil || cfg.Profile.LearnedFitnessLoadTypes["WeightTraining"] != "atl-only" ||
		cfg.Profile.FitnessLoadTypes["Yoga"] != "atl-only" {
		t.Fatalf("persisted profile = %+v, %v", cfg.Profile, err)
	}
}

func TestApplyFitnessLoadTypesFlag(t *testing.T) {
	t.Parallel()
	profile := icu.TrainingProfile{FitnessLoadTypes: map[string]string{"Yoga": "ctl"}}
	if err := applyFitnessLoadTypesFlag(&profile, map[string]string{
		"fitness-load-types": "WeightTraining=atl-only, Yoga=-",
	}); err != nil || len(profile.FitnessLoadTypes) != 1 || profile.FitnessLoadTypes["WeightTraining"] != "atl-only" {
		t.Fatalf("%v %v", profile.FitnessLoadTypes, err)
	}
	for _, bad := range []string{"Yoga", "=ctl", "Yoga=maybe"} {
		if err := applyFitnessLoadTypesFlag(&profile, map[string]string{"fitness-load-types": bad}); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
	_ = applyFitnessLoadTypesFlag(&profile, map[string]string{"fitness-load-types": "WeightTraining=-"})
	if profile.FitnessLoadTypes != nil {
		t.Fatal("empty overrides not cleared")
	}
	_ = applyFitnessLoadTypesFlag(&profile, map[string]string{"fitness-load-types": "Yoga=ctl"})
	if err := applyFitnessLoadTypesFlag(&profile, map[string]string{"fitness-load-types": "-"}); err != nil ||
		profile.FitnessLoadTypes != nil {
		t.Fatal("- did not clear")
	}
	if err := icu.ValidateTrainingProfile(&icu.TrainingProfile{
		LearnedFitnessLoadTypes: map[string]string{"Ride": "x"},
	}); err == nil {
		t.Fatal("invalid class validated")
	}
}

func TestSaveLearnedFitnessLoadTypesKeepsOtherConfig(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)
	if err := saveLearnedFitnessLoadTypes(map[string]string{"Ride": "ctl"}); err != nil {
		t.Fatal(err)
	}
	if err := icu.SaveConfigWithAction(&icu.Config{AthleteID: "i1", Profile: &icu.TrainingProfile{
		FitnessLoadTypes: map[string]string{"Yoga": "atl-only"},
	}}, "config_set"); err != nil {
		t.Fatal(err)
	}
	if err := saveLearnedFitnessLoadTypes(map[string]string{"Ride": "ctl"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := icu.LoadConfig()
	if cfg.AthleteID != "i1" || cfg.Profile.LearnedFitnessLoadTypes["Ride"] != "ctl" ||
		cfg.Profile.FitnessLoadTypes["Yoga"] != "atl-only" {
		t.Fatalf("config = %+v profile %+v", cfg, cfg.Profile)
	}
}

func TestPlanFitnessLoadTypesClearsConflictedSavedClass(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)
	profile := &icu.TrainingProfile{LearnedFitnessLoadTypes: map[string]string{"Swim": "ctl", "Ride": "ctl"}}
	if err := icu.SaveConfigWithAction(&icu.Config{AthleteID: "i1", Profile: profile}, "config_set"); err != nil {
		t.Fatal(err)
	}
	records := []icu.Wellness{
		{ID: "2026-09-01", CTLLoad: 20, ATLLoad: 20},
		{ID: "2026-09-02", CTLLoad: 0, ATLLoad: 20},
	}
	activities := []icu.Activity{
		{Type: "Swim", StartDateLocal: "2026-09-01T07:00:00", TrainingLoad: 20},
		{Type: "Swim", StartDateLocal: "2026-09-02T07:00:00", TrainingLoad: 20},
	}
	_, warnings := planFitnessLoadTypes(profile, records, activities, nil, nil, "2026-09-03", true)
	if !strings.Contains(strings.Join(warnings, "|"), "Swim counted toward CTL on some days") {
		t.Fatalf("warnings = %v", warnings)
	}
	cfg, _ := icu.LoadConfig()
	if _, ok := cfg.Profile.LearnedFitnessLoadTypes["Swim"]; ok || cfg.Profile.LearnedFitnessLoadTypes["Ride"] != "ctl" {
		t.Fatalf("saved = %v", cfg.Profile.LearnedFitnessLoadTypes)
	}
}
