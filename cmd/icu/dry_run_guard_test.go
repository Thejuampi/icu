package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

// readOnlyServer answers GETs with an event and fails the test on any write.
func readOnlyServer(t *testing.T) (*icu.Client, *int) {
	t.Helper()
	var mu sync.Mutex
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != http.MethodGet {
			writes++
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)

			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":3,"name":"Z2","type":"Ride","start_date_local":"2026-10-28T07:00:00","description":"a"}`))
	}))
	t.Cleanup(server.Close)

	return icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL)), &writes
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestEventsUpdateDryRunNeverWrites(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	client, writes := readOnlyServer(t)
	out, err := captureStdout(t, func(stdout io.Writer) error {
		return eventsUpdateCommand().Run([]string{"3"}, map[string]string{"name": "x", "dry-run": ""}, client, stdout, io.Discard)
	})
	if err != nil || *writes != 0 || !strings.Contains(out, `"dryRun": true`) || !strings.Contains(out, `"name": "x"`) {
		t.Fatalf("err=%v writes=%d out=%s", err, *writes, out)
	}
	if paths, _ := listEventBackups(3); len(paths) != 0 {
		t.Fatalf("dry-run left backups %v", paths)
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestEventsDeleteAndRevertDryRunNeverWrite(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	client, writes := readOnlyServer(t)
	out, err := captureStdout(t, func(stdout io.Writer) error {
		return eventsDeleteCommand().Run([]string{"3"}, map[string]string{"dry-run": ""}, client, stdout, io.Discard)
	})
	if err != nil || *writes != 0 || !strings.Contains(out, `"wouldDelete": 3`) {
		t.Fatalf("delete err=%v writes=%d out=%s", err, *writes, out)
	}
	if _, err := backupEvent(client, 3, eventBackupReasonDelete); err != nil {
		t.Fatal(err)
	}
	paths, _ := listEventBackups(3)
	if len(paths) != 1 {
		t.Fatalf("paths = %v", paths)
	}
	if err := markWriteDone(paths[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func(stdout io.Writer) error {
		return eventsRevertCommand().Run(nil, map[string]string{"id": "3", "dry-run": ""}, client, stdout, io.Discard)
	}); err != nil || *writes != 0 {
		t.Fatalf("revert err=%v writes=%d", err, *writes)
	}
}

// withMockCommandClient routes executeCommand at client for the test.
func withMockCommandClient(t *testing.T, client *icu.Client) {
	t.Helper()
	previous := newCommandClient
	newCommandClient = func(string, string, string) *icu.Client { return client }
	t.Cleanup(func() { newCommandClient = previous })
}

//nolint:paralleltest // replaces newCommandClient and sets env.
func TestUnknownFlagRejectedBeforeAnyRequest(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	t.Setenv("INTERVALS_ICU_API_KEY", "k")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)
	withMockCommandClient(t, icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL)))

	for _, args := range [][]string{
		{"icu", "events", "update", "3", "--name", "x", "--dryrun"},
		{"icu", "events", "delete", "3", "--yes-really"},
		{"icu", "wellness", "bulk", "--file", "/nonexistent.json", "--dry-run"},
		{"icu", "workouts", "update", "3", "--name", "x", "--bogus", "1"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "unknown flag") {
			t.Fatalf("%v: code=%d stderr=%q", args, code, stderr.String())
		}
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
}

//nolint:paralleltest // replaces newCommandClient and sets env.
func TestEventsUpdateDryRunEndToEndNeverWrites(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	t.Setenv("INTERVALS_ICU_API_KEY", "k")
	client, writes := readOnlyServer(t)
	withMockCommandClient(t, client)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"icu", "events", "update", "1", "--name", "X", "--dry-run"}, &stdout, &stderr); code != 0 || *writes != 0 {
		t.Fatalf("code=%d writes=%d stderr=%q", code, *writes, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"field": "name"`) || !strings.Contains(out, `"before": "Z2"`) || !strings.Contains(out, `"after": "X"`) {
		t.Fatalf("dry-run diff missing: %s", out)
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestCalendarCancelClearsWritePending(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":5,"name":"Z2","type":"Ride","start_date_local":"2026-10-28T07:00:00"}`))
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	operation := &icu.CalendarOperation{ID: "c", Action: icu.CalendarActionCancel, EventID: 5}
	result := applyCalendarOperation(io.Discard, client, operation)
	if result.Status != icu.CalendarStatusApplied || result.BackupPath == "" {
		t.Fatalf("result = %+v", result)
	}
	backup, err := readEventBackup(result.BackupPath)
	if err != nil || backup.WritePending {
		t.Fatalf("backup pending=%v err=%v, want cleared", backup.WritePending, err)
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestResolveRevertBackupOnlyPendingPointsToBackupFlag(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	path, err := writeEventBackup(&eventBackup{EventID: 9, Reason: "events update", Event: []byte(`{}`), WritePending: true}, timeNowForTest())
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolveRevertBackup(map[string]string{"id": "9"})
	if err == nil || !strings.Contains(err.Error(), "1 pending backup(s)") || !strings.Contains(err.Error(), "--backup "+path) {
		t.Fatalf("err = %v", err)
	}
}

func timeNowForTest() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }

// readOnlyCommands never write. Every other registered command is treated as
// a write command by TestEveryCommandRejectsUnsupportedFlagsWithoutWrites, so
// a new command is guarded until it is classified here.
func readOnlyCommands() map[string]bool {
	return map[string]bool{
		"activities around":                true,
		"activities csv":                   true,
		"activities get":                   true,
		"activities interval-search":       true,
		"activities list":                  true,
		"activities search":                true,
		"activities search-full":           true,
		"activity best-efforts":            true,
		"activity decoupling":              true,
		"activity estimate-power":          true,
		"activity estimate-power-backtest": true,
		"activity file":                    true,
		"activity fit-file":                true,
		"activity gpx-file":                true,
		"activity hr-curve":                true,
		"activity intervals":               true,
		"activity map":                     true,
		"activity messages":                true,
		"activity pace-curve":              true,
		"activity power-curve":             true,
		"activity power-vs-hr":             true,
		"activity segments":                true,
		"activity show":                    true,
		"activity streams":                 true,
		"activity weather":                 true,
		"activity weather-summary":         true,
		"analysis adaptation":              true,
		"analysis coaching":                true,
		"analysis cycling":                 true,
		"analysis micro":                   true,
		"analysis microcycle":              true,
		"analysis plan":                    true,
		"analysis wellness":                true,
		"analysis workout":                 true,
		"athlete profile":                  true,
		"athlete settings":                 true,
		"athlete show":                     true,
		"athlete summary":                  true,
		"chats get":                        true,
		"chats list":                       true,
		"chats messages":                   true,
		"config diagnose":                  true,
		"config path":                      true,
		"config show":                      true,
		"curves hr":                        true,
		"curves mmp":                       true,
		"curves pace":                      true,
		"curves power":                     true,
		"curves power-hr":                  true,
		"custom get":                       true,
		"custom list":                      true,
		"events backups":                   true,
		"events download":                  true,
		"events get":                       true,
		"events list":                      true,
		"events tags":                      true,
		"fitness-events list":              true,
		"folders list":                     true,
		"ftp show":                         true,
		"gear list":                        true,
		"plan show":                        true,
		"profile show":                     true,
		"rebalance preview":                true,
		"rebalance show":                   true,
		"routes get":                       true,
		"routes list":                      true,
		"shared-event get":                 true,
		"sports get":                       true,
		"sports list":                      true,
		"tags activities":                  true,
		"tags events":                      true,
		"tags workouts":                    true,
		"weather forecast":                 true,
		"wellness get":                     true,
		"wellness list":                    true,
		"workouts calculate":               true,
		"workouts get":                     true,
		"workouts list":                    true,
		"workouts tags":                    true,
		"zepp biocharge":                   true,
		"zepp blood-pressure":              true,
		"zepp body-battery":                true,
		"zepp events":                      true,
		"zepp health-summary":              true,
		"zepp heart-rate":                  true,
		"zepp hrv":                         true,
		"zepp hybridcharge":                true,
		"zepp manual-data":                 true,
		"zepp mood":                        true,
		"zepp pai":                         true,
		"zepp profile":                     true,
		"zepp readiness":                   true,
		"zepp respiratory-rate":            true,
		"zepp second-heart-rate":           true,
		"zepp skin-temp":                   true,
		"zepp sleep":                       true,
		"zepp spo2":                        true,
		"zepp spo2-windows":                true,
		"zepp sport-load":                  true,
		"zepp status":                      true,
		"zepp stress":                      true,
		"zepp stress-minute":               true,
		"zepp summary":                     true,
		"zepp vo2":                         true,
		"zepp weight":                      true,
		"zepp workout":                     true,
		"zepp workouts":                    true,
	}
}

// dryRunCommands implement --dry-run and must make no write with it.
// writeCommands may write, remotely or to local config. Every registered
// command must be in exactly one of readOnlyCommands and writeCommands.
func writeCommands() map[string]bool {
	keys := []string{
		"activities manual", "activities upload", "activity delete", "activity estimate-power-accept",
		"activity update", "athlete plan", "athlete update", "chats send", "config set",
		"custom create", "custom delete", "custom update",
		"events create", "events delete", "events revert", "events update",
		"folders create", "folders delete", "folders update", "ftp update",
		"gear create", "gear delete", "gear update", "plan accept", "plan preview", "profile set",
		"rebalance accept", "rebalance approve", "sports delete", "sports update", "weather config",
		"wellness bulk", "wellness sync", "wellness update", "wellness upload",
		"workouts create", "workouts delete", "workouts update",
		"zepp login", "zepp logout", "zepp token",
	}
	out := make(map[string]bool, len(keys))
	for _, key := range keys {
		out[key] = true
	}

	return out
}

// dryRunCommands implement --dry-run; the value is the arguments (after the
// command) of a dry run that must exit 0 without any write.
func dryRunCommands() map[string][]string {
	return map[string][]string{
		"events update":  {"1", "--name", "X", "--dry-run"},
		"events delete":  {"1", "--dry-run"},
		"events revert":  {"--id", "1", "--dry-run"},
		"sports update":  {"Ride", "--ftp", "250", "--dry-run"},
		"athlete update": {"--weight", "70", "--dry-run"},
		"ftp update":     {"--value", "250", "--dry-run"},
	}
}

func commandArgs(resource, action string, extra ...string) []string {
	args := make([]string, 0, 4+len(extra))
	args = append(args, "icu", resource)
	if isIDFirstResource(resource) {
		args = append(args, "1")
	}
	args = append(args, action)

	return append(args, extra...)
}

// guardMock is a mock intervals.icu that counts requests per phase and, unless
// writes are allowed, fails every non-GET.
type guardMock struct {
	mu          sync.Mutex
	current     string
	allowWrites bool
	requests    map[string]int
	writes      map[string]int
}

func (m *guardMock) phase(key string, allowWrites bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current, m.allowWrites = key, allowWrites
}

func (m *guardMock) counts(key string) (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.requests[key], m.writes[key]
}

func newGuardMock(t *testing.T) *guardMock {
	t.Helper()
	mock := &guardMock{requests: map[string]int{}, writes: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.mu.Lock()
		defer mock.mu.Unlock()
		mock.requests[mock.current]++
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			mock.writes[mock.current]++
			if !mock.allowWrites {
				w.WriteHeader(http.StatusTeapot)

				return
			}
		}
		if r.URL.Path == "/api/v1/athlete/i1" {
			_, _ = w.Write([]byte(`{"id":"i1","name":"A","weight":71,"sportSettings":[{"id":1,"types":["Ride"],"ftp":200}]}`))

			return
		}
		_, _ = w.Write([]byte(`{"id":1,"name":"Z2","type":"Ride","types":["Ride"],"ftp":200,` +
			`"category":"WORKOUT","start_date_local":"2026-10-28T07:00:00"}`))
	}))
	t.Cleanup(server.Close)
	withMockCommandClient(t, icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL)))

	return mock
}

func guardEnv(t *testing.T) {
	t.Helper()
	t.Setenv(eventBackupDirEnv, t.TempDir())
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	t.Setenv("INTERVALS_ICU_API_KEY", "k")
	t.Setenv("INTERVALS_ICU_ATHLETE_ID", "i1")
}

// skipLiveRun: commands that reach services other than intervals.icu
// (Zepp, weather/estimate-power) are classified and flag-checked but never run.
func skipLiveRun(resource, action string) bool {
	return resource == "zepp" || action == "estimate-power" || action == "estimate-power-backtest"
}

//nolint:paralleltest // replaces newCommandClient and sets env.
func TestEveryCommandRejectsUnsupportedFlagsWithoutWrites(t *testing.T) {
	guardEnv(t)
	mock := newGuardMock(t)
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	readOnly, writers, dryRuns := readOnlyCommands(), writeCommands(), dryRunCommands()
	seen := map[string]bool{}
	for _, resource := range registry.Resources() {
		for _, action := range registry.Actions(resource) {
			key := resource + " " + action
			seen[key] = true
			if readOnly[key] == writers[key] {
				t.Errorf("%s must be in exactly one of readOnlyCommands and writeCommands", key)
			}

			mock.phase(key+" --not-a-flag", false)
			var stdout, stderr bytes.Buffer
			if code := run(commandArgs(resource, action, "--not-a-flag", "1"), &stdout, &stderr); code == 0 ||
				!strings.Contains(stderr.String(), "unknown flag --not-a-flag") {
				t.Errorf("%s --not-a-flag: code=%d stderr=%q", key, code, stderr.String())
			}
			if requests, _ := mock.counts(key + " --not-a-flag"); requests != 0 {
				t.Errorf("%s --not-a-flag sent %d request(s)", key, requests)
			}

			dryArgs, supportsDryRun := dryRuns[key]
			if !supportsDryRun {
				mock.phase(key+" --dry-run", false)
				stdout.Reset()
				stderr.Reset()
				if code := run(commandArgs(resource, action, "--dry-run"), &stdout, &stderr); code == 0 ||
					!strings.Contains(stderr.String(), "unknown flag --dry-run") {
					t.Errorf("%s --dry-run must be rejected: code=%d stderr=%q", key, code, stderr.String())
				}
				if requests, _ := mock.counts(key + " --dry-run"); requests != 0 {
					t.Errorf("%s --dry-run sent %d request(s)", key, requests)
				}

				continue
			}
			if !writers[key] {
				t.Errorf("%s supports --dry-run but is not a write command", key)
			}
			if key == "events revert" {
				seedRevertBackup(t)
			}
			mock.phase(key+" --dry-run", false)
			stdout.Reset()
			stderr.Reset()
			if code := run(commandArgs(resource, action, dryArgs...), &stdout, &stderr); code != 0 {
				t.Errorf("%s dry run exit=%d stderr=%q", key, code, stderr.String())
			}
			if _, writes := mock.counts(key + " --dry-run"); writes != 0 {
				t.Errorf("%s --dry-run sent %d write request(s)", key, writes)
			}
		}
	}
	for key := range dryRuns {
		if !seen[key] {
			t.Errorf("dryRunCommands lists unknown command %s", key)
		}
	}
	for key := range readOnly {
		if !seen[key] {
			t.Errorf("readOnlyCommands lists unknown command %s", key)
		}
	}
	for key := range writers {
		if !seen[key] {
			t.Errorf("writeCommands lists unknown command %s", key)
		}
	}
}

// seedRevertBackup leaves one completed backup of event 1 so events revert
// --id 1 --dry-run has something to preview.
func seedRevertBackup(t *testing.T) {
	t.Helper()
	// newGuardMock has already swapped newCommandClient (withMockCommandClient),
	// so the base URL is ignored and the backup GET goes to the guard mock. The
	// dead loopback port only matters if that swap is ever missing: the GET then
	// fails fast (connection refused) instead of reaching the real API.
	client := newCommandClient("k", "i1", "http://127.0.0.1:1")
	if _, err := backupEvent(client, 1, eventBackupReasonDelete); err != nil {
		t.Fatal(err)
	}
	paths, err := listEventBackups(1)
	if err != nil || len(paths) == 0 {
		t.Fatalf("backups = %v, %v", paths, err)
	}
	if err := markWriteDone(paths[0]); err != nil {
		t.Fatal(err)
	}
}

// TestCommandsRunAgainstMock runs every command without --dry-run against the
// mock: read-only commands must not write; write commands may.
//
//nolint:paralleltest // replaces newCommandClient and sets env.
func TestCommandsRunAgainstMock(t *testing.T) {
	guardEnv(t)
	mock := newGuardMock(t)
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	totalWrites := 0
	sawAccept := false
	for _, resource := range registry.Resources() {
		for _, action := range registry.Actions(resource) {
			key := resource + " " + action
			if skipLiveRun(resource, action) {
				continue
			}
			write := writeCommands()[key]
			mock.phase(key, write)
			var stdout, stderr bytes.Buffer
			extra := []string{"1"}
			if key == "activity estimate-power-accept" {
				path := filepath.Join(t.TempDir(), "estimate.json")
				if err := os.WriteFile(path, []byte(`{"activityId":"1","filledWatts":[150,160,170]}`), 0o600); err != nil {
					t.Fatal(err)
				}
				extra = []string{"--file", path}
			}
			_ = run(commandArgs(resource, action, extra...), &stdout, &stderr)
			_, writes := mock.counts(key)
			if key == "activity estimate-power-accept" {
				sawAccept = true
			}
			if key == "activity estimate-power-accept" && writes < 1 {
				t.Errorf("estimate-power-accept with filledWatts sent %d write(s), want >= 1", writes)
			}
			if !write && writes != 0 {
				t.Errorf("read-only %s sent %d write request(s)", key, writes)
			}
			totalWrites += writes
		}
	}
	if !sawAccept {
		t.Fatal("activity estimate-power-accept was never run against the mock")
	}
	if totalWrites == 0 {
		t.Fatal("no write command reached the mock; the plain-run pass is not exercising writes")
	}
}

//nolint:paralleltest // replaces newCommandClient and sets env.
func TestWellnessWriteDryRunRejectedWithoutWrites(t *testing.T) {
	for _, args := range [][]string{
		{"icu", "wellness", "bulk", "--dry-run"},
		{"icu", "wellness", "update", "--dry-run"},
		{"icu", "wellness", "update", "2026-10-01", "--weight", "70", "--dry-run"},
	} {
		guardEnv(t)
		mock := newGuardMock(t)
		key := args[1] + " " + args[2]
		mock.phase(key, false)
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code == 0 || !strings.Contains(stderr.String(), "unknown flag --dry-run") {
			t.Fatalf("%v: code=%d stderr=%q", args, code, stderr.String())
		}
		if requests, writes := mock.counts(key); requests != 0 || writes != 0 {
			t.Fatalf("%v sent %d request(s), %d write(s)", args, requests, writes)
		}
	}
}

//nolint:paralleltest // replaces newCommandClient and sets env.
func TestWarnRealAPIFiresForDefaultBaseURL(t *testing.T) {
	guardEnv(t)
	t.Setenv(icu.BaseURLEnv, "")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"i1","name":"Mock"}`))
	}))
	t.Cleanup(server.Close)
	// The seam points at the loopback server; the resolved base URL stays the default real host.
	withMockCommandClient(t, icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL)))
	var stdout, stderr bytes.Buffer
	if code := run([]string{"icu", "athlete", "show"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Warning: using the real intervals.icu API") {
		t.Fatalf("no real-API warning without --base-url: %q", stderr.String())
	}
	if requests == 0 {
		t.Fatal("athlete show did not reach the loopback mock")
	}
}

func TestActivityDetailUsageText(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	_ = run([]string{"icu", "activity", "--help"}, &stdout, &stderr)
	help := stdout.String() + stderr.String()
	if strings.Contains(help, "icu.Activity") || strings.Contains(help, strings.Repeat("icu ", 2)) {
		t.Fatalf("bad activity usage text:\n%s", help)
	}
	for _, want := range []string{"icu activity <id> best-efforts", "icu activity <id> fit-file", "icu activity <id> power-curve"} {
		if !strings.Contains(help, want) {
			t.Fatalf("activity help missing %q", want)
		}
	}
}

// --no-weather belongs to activity estimate-power(-backtest) only; analysis
// commands never fetch weather and must reject it before any request.
//
//nolint:paralleltest // replaces newCommandClient and sets env.
func TestNoWeatherOnlyOnEstimatePower(t *testing.T) {
	guardEnv(t)
	mock := newGuardMock(t)
	mock.phase("analysis coaching", false)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"icu", "analysis", "coaching", "--no-weather"}, &stdout, &stderr); code == 0 ||
		!strings.Contains(stderr.String(), "unknown flag --no-weather") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if requests, _ := mock.counts("analysis coaching"); requests != 0 {
		t.Fatalf("analysis coaching --no-weather sent %d request(s)", requests)
	}
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	for resource, actions := range registry.commands {
		for action, command := range actions {
			has := usageFlags(command.Usage)["no-weather"]
			want := resource == "activity" && (action == "estimate-power" || action == "estimate-power-backtest")
			if has != want {
				t.Errorf("%s %s: usage lists --no-weather = %v, want %v", resource, action, has, want)
			}
		}
	}
}
