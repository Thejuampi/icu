package main

import (
	"encoding/json"
	"errors"
	"io"
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

	err := cmd.Run(nil, rebalanceDryRunFlags(file), client, io.Discard, io.Discard)
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
	if err := dryRun.Run(nil, rebalanceDryRunFlags(file), client, io.Discard, io.Discard); err != nil {
		t.Fatalf("rebalance dry-run: %v", err)
	}

	err := accept.Run(nil, map[string]string{"file": file}, client, io.Discard, io.Discard)
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

	err := accept.Run(nil, map[string]string{"file": file}, client, io.Discard, io.Discard)
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

	err := accept.Run(nil, map[string]string{"file": file}, client, io.Discard, io.Discard)
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

	err := accept.Run(nil, map[string]string{"file": file}, client, io.Discard, io.Discard)
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

// TestReadRebalanceActivitiesIgnoresFieldsOverride is W2-N02/D-8: the
// choke point must fetch its own fixed field list regardless of a
// caller-supplied --activity-fields, so wave-2's validity/pairing fields are
// never dropped by a narrower request.
func TestReadRebalanceActivitiesIgnoresFieldsOverride(t *testing.T) {
	t.Parallel()

	var gotFields string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		gotFields = request.URL.Query().Get("fields")
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	_, err := readRebalanceActivities(client, "2026-06-22", "2026-06-28", map[string]string{"activity-fields": "id,name"})
	if err != nil {
		t.Fatalf("read activities: %v", err)
	}

	if gotFields != defaultAnalysisFields {
		t.Fatalf("fields = %q, want the unmodified defaultAnalysisFields regardless of the activity-fields override", gotFields)
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
	// The 14-day lookback is the analysis scope; the read still reaches back
	// the full wellness baseline (49 days) so thresholds are not truncated.
	if gotOldest != "2026-05-10" {
		t.Fatalf("oldest = %q, want 2026-05-10", gotOldest)
	}
}

func TestRebalanceDryRunPrefersHybridChargeForWellnessContext(t *testing.T) {
	withHybridChargeZeppTestServer(t, `{"items":[{"timestamp":1780272000000,"value":90},{"timestamp":1780358400000,"value":88}]}`)

	registry := NewCommandRegistry()
	registerAllCommands(registry)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if strings.Contains(request.URL.Path, "/wellness") {
			_, _ = response.Write([]byte(`[{"id":"2026-05-31","sleepScore":55,"restingHr":50,"icuCtl":50,"icuAtl":50},{"id":"2026-06-01","sleepScore":60,"restingHr":49,"icuCtl":50,"icuAtl":50}]`))

			return
		}

		_, _ = response.Write([]byte(functionalJSONResponse(request.Method, request.URL.Path))) //nolint:gosec // G705: httptest server answering the test client, not a browser.
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	cmd, ok := registry.Lookup("rebalance", "show")
	if !ok {
		t.Fatalf("missing rebalance command")
	}
	file := filepath.Join(t.TempDir(), "rebalance.json")
	err := cmd.Run(nil, rebalanceDryRunFlags(file), client, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("rebalance dry-run: %v", err)
	}

	proposal := readRebalanceTestProposal(t, file)
	// Two days of history judge nothing.
	if proposal.Context.WellnessState != icu.PhysiologyInsufficient {
		t.Fatalf("wellness state = %q, want INSUFFICIENT", proposal.Context.WellnessState)
	}
}

func TestRebalanceDryRunRejectsMaxHRFlag(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	flags := rebalanceDryRunFlags(filepath.Join(t.TempDir(), "rebalance.json"))
	flags["max-hr"] = "140"

	_, err := readRebalanceInput(flags, client)

	if err == nil || !strings.Contains(err.Error(), "max-hr is not supported") {
		t.Fatalf("err = %v, want the specific max-hr rejection", err)
	}
}

// TestRebalanceShowMaxHRFlagReachesSpecificRejection is Stage 6 R9:
// rebalanceShowSchema previously never declared --max-hr, so
// validateCommandInput's generic "unknown flag" check always fired first and
// readRebalanceInput's specific rejection message (below) was unreachable
// dead code — the same failure never surfaced because both checks reject.
// This test pins both halves of the fix: the schema no longer treats
// --max-hr as unknown, and the specific message still fires once past it.
func TestRebalanceShowSchemaAcceptsMaxHRFlagAsDeclared(t *testing.T) {
	t.Parallel()

	cmd := rebalanceDryRunCommand()
	schemaFlags := map[string]string{"oldest": "2026-06-01", "newest": "2026-06-07", "max-hr": "140"}

	err := validateCommandInput(cmd, nil, schemaFlags)
	if err != nil {
		t.Fatalf("validateCommandInput() = %v, want --max-hr accepted as a declared flag", err)
	}
}

func TestRebalanceShowMaxHRFlagReachesSpecificRejection(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	flags := rebalanceDryRunFlags(filepath.Join(t.TempDir(), "rebalance.json"))
	flags["max-hr"] = "140"

	err := rebalanceDryRunCommand().Run(nil, flags, client, io.Discard, io.Discard)

	if err == nil || !strings.Contains(err.Error(), "max-hr is not supported") {
		t.Fatalf("err = %v, want the specific max-hr rejection", err)
	}
}

func TestRebalanceDryRunRejectsInvalidSessionEnvironmentFlag(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	flags := rebalanceDryRunFlags(filepath.Join(t.TempDir(), "rebalance.json"))
	flags["session-environment"] = "sideways"

	_, err := readRebalanceInput(flags, client)

	if err == nil {
		t.Fatalf("err = nil, want rejection of invalid session-environment value")
	}
}

// TestRebalanceDryRunRejectsInvertedDateRange is Stage 6 R3: rebalance had no
// local date validation at all, unlike its `analysis` siblings — an inverted
// window silently wrote a proposal with scope.startDate > scope.endDate
// instead of failing fast. Reuses validateAnalysisRange (DRY), which already
// serves four analysis call sites.
func TestRebalanceDryRunRejectsInvertedDateRange(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	flags := rebalanceDryRunFlags(filepath.Join(t.TempDir(), "rebalance.json"))
	flags["oldest"] = "2026-06-07"
	flags["newest"] = "2026-06-01"

	_, err := readRebalanceInput(flags, client)

	if !errors.Is(err, errInvalidValue) {
		t.Fatalf("err = %v, want errInvalidValue for an inverted date range", err)
	}
}

// TestRebalanceDryRunRejectsMalformedDate is Stage 6 R6: a malformed date
// value is a wrong-shape input, not an absent one — routing it through
// errMissingRequired mislabelled the category while the explanation text
// stayed accurate.
func TestRebalanceDryRunRejectsMalformedDate(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	flags := rebalanceDryRunFlags(filepath.Join(t.TempDir(), "rebalance.json"))
	flags["oldest"] = "2026-13-45"

	_, err := readRebalanceInput(flags, client)

	if !errors.Is(err, errInvalidValue) {
		t.Fatalf("err = %v, want errInvalidValue for a malformed date", err)
	}
}

// TestRebalanceDryRunRejectsMalformedNowDate is Stage 6 R11:
// rebalanceDateLockedByTime (rebalance.go) compares NowDate against session
// dates as a raw string (date < input.NowDate). An unvalidated NowDate that
// sorts below digit characters (e.g. "!") makes every real date compare as
// "after now", silently defeating keepCompleted/AllowPast for a genuinely
// past week and letting create/update operations reach the output file
// that `rebalance accept` would apply next.
func TestRebalanceDryRunRejectsMalformedNowDate(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	flags := rebalanceDryRunFlags(filepath.Join(t.TempDir(), "rebalance.json"))
	flags["now-date"] = "!"

	err := rebalanceDryRunCommand().Run(nil, flags, client, io.Discard, io.Discard)

	if !errors.Is(err, errInvalidValue) {
		t.Fatalf("err = %v, want errInvalidValue for a malformed --now-date", err)
	}
}

// TestRebalanceDryRunNeverWritesProposalFileForMalformedNowDate is Stage 6
// R11's observable-consequence half: pin that the proposal file is never
// written at all when --now-date is malformed, not just that an error is
// returned, so there is nothing left for a caller to accidentally accept.
func TestRebalanceDryRunNeverWritesProposalFileForMalformedNowDate(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	file := filepath.Join(t.TempDir(), "rebalance.json")
	flags := rebalanceDryRunFlags(file)
	flags["now-date"] = "!"

	_ = rebalanceDryRunCommand().Run(nil, flags, client, io.Discard, io.Discard)
	_, statErr := os.Stat(file)

	if !os.IsNotExist(statErr) {
		t.Fatalf("proposal file exists after a rejected --now-date; want no file written (no operations ever produced)")
	}
}

func TestRebalanceConstraintsFromFlagsCarriesSessionEnvironment(t *testing.T) {
	t.Parallel()

	constraints := rebalanceConstraintsFromFlags(map[string]string{"session-environment": "outdoor"})

	if constraints.SessionEnvironment != "outdoor" {
		t.Fatalf("session environment = %q, want outdoor", constraints.SessionEnvironment)
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

	registry, client := rebalanceTestRegistryClient(t)
	cmd, ok := registry.Lookup("rebalance", "show")
	if !ok {
		t.Fatalf("missing rebalance show command")
	}

	err := cmd.Run(nil, map[string]string{}, client, io.Discard, io.Discard)

	if err == nil {
		t.Fatalf("err = nil, want missing oldest")
	}
}

func TestRebalanceDryRunRequiresOldestFlag(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)

	_, err := readRebalanceInput(map[string]string{}, client)

	if err == nil {
		t.Fatalf("err = nil, want missing oldest")
	}
}

func TestRebalanceDryRunRejectsUnsupportedTarget(t *testing.T) {
	t.Parallel()

	_, client := rebalanceTestRegistryClient(t)
	flags := rebalanceDryRunFlags(filepath.Join(t.TempDir(), "rebalance.json"))
	flags["target"] = "HR"

	_, err := readRebalanceInput(flags, client)
	if err == nil {
		t.Fatalf("err = nil, want unsupported target")
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

	result := applyRebalanceOperation(io.Discard, client, &operation)

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
		_, _ = response.Write([]byte(functionalJSONResponse(request.Method, request.URL.Path))) //nolint:gosec // G705: httptest server answering the test client, not a browser.
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

	err := approve.Run(nil, map[string]string{"file": file}, nil, io.Discard, io.Discard)
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

	err := approve.Run(nil, map[string]string{"file": file, "reason": "ramp-up"}, nil, io.Discard, io.Discard)
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
	if proposal.Approve.TargetLoad != 0 || proposal.Approve.Level != "" || proposal.Approve.Mode != "" {
		t.Fatalf("approve limits = %+v, want inherited proposal values only when provided", proposal.Approve)
	}
}

func TestRebalanceApproveStoresExplicitLimitsWithoutMutatingConstraints(t *testing.T) {
	t.Parallel()

	registry, _ := rebalanceTestRegistryClient(t)
	approve, _ := registry.Lookup("rebalance", "approve")
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Constraints:   icu.RebalanceConstraints{TargetLoad: 354, Level: "0.5", Mode: icu.RebalanceModePreserveStructure},
		Policy:        &icu.RebalancePolicy{Strategy: icu.RebalanceStrategyAdaptiveBidirectional, Level: "0.5", Mode: icu.RebalanceModePreserveStructure, CurveMethod: icu.RebalanceCurveMethodPCHIP},
		Envelope:      &icu.RebalanceEnvelopeReport{Envelope: icu.RebalanceEnvelope{Low: fRat(200), Current: fRat(300), High: fRat(400)}, LowSource: "data_robust_fence", HighSource: "data_robust_fence"},
		Baseline:      icu.RebalanceEvaluation{TargetLoad: 354},
		Options:       []icu.RebalanceOption{{Evaluation: icu.RebalanceEvaluation{TargetLoad: 354}}},
	})

	err := approve.Run(nil, map[string]string{"file": file, "reason": "override", "target-load": "380", "level": "0.7"}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	proposal := readRebalanceTestProposal(t, file)
	if proposal.Constraints.TargetLoad != 354 {
		t.Fatalf("constraints target load = %d, want unchanged 354", proposal.Constraints.TargetLoad)
	}
	if proposal.Approve == nil || proposal.Approve.TargetLoad != 380 || proposal.Approve.Level != "0.7" {
		t.Fatalf("approve = %+v, want explicit stored limits", proposal.Approve)
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

	err := approve.Run(nil, map[string]string{"file": file, "reason": "override"}, nil, io.Discard, io.Discard)
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

	err := accept.Run(nil, map[string]string{"file": file}, nil, io.Discard, io.Discard)
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

	err := accept.Run(nil, map[string]string{"file": file}, nil, io.Discard, io.Discard)
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

	err := accept.Run(nil, map[string]string{"file": file}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatalf("err = nil, want approval hash mismatch")
	}
}

func TestRebalanceAcceptRejectsApprovalLimitsMismatch(t *testing.T) {
	t.Parallel()

	registry, _ := rebalanceTestRegistryClient(t)
	accept, _ := registry.Lookup("rebalance", "accept")
	approvedProposal := icu.RebalanceProposalFile{
		Constraints: icu.RebalanceConstraints{
			TargetLoad: 380,
			Level:      "0.7",
			Mode:       icu.RebalanceModePreserveStructure,
		},
		Envelope: &icu.RebalanceEnvelopeReport{
			Envelope: icu.RebalanceEnvelope{Low: fRat(200), Current: fRat(300), High: fRat(320)},
		},
	}
	approvedHash := icu.ComputeRebalanceApprove(&approvedProposal, "override", true).RecalcHash
	file := writeRebalanceTestProposal(t, &icu.RebalanceProposalFile{
		SchemaVersion: icu.RebalanceSchemaVersion,
		Constraints:   icu.RebalanceConstraints{TargetLoad: 354, Level: "0.5", Mode: icu.RebalanceModePreserveStructure},
		Envelope: &icu.RebalanceEnvelopeReport{
			Envelope: icu.RebalanceEnvelope{Low: fRat(200), Current: fRat(300), High: fRat(320)},
		},
		Approve: &icu.RebalanceApprove{
			Reason:         "override",
			ProvidedLimits: true,
			Verified:       true,
			TargetLoad:     380,
			Level:          "0.7",
			Mode:           icu.RebalanceModePreserveStructure,
			RecalcHash:     approvedHash,
			LimitsHash:     "stale",
		},
	})

	err := accept.Run(nil, map[string]string{"file": file}, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatalf("err = nil, want approval limits mismatch")
	}
}

func fRat(v float64) icu.RebalanceRat { r, _ := icu.NewRebalanceRatFromDyadic(v); return r }
