package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
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
	newCommandClient = func(string, string) *icu.Client { return client }
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
func dryRunCommands() map[string]bool {
	return map[string]bool{
		"events update": true, "events delete": true, "events revert": true, "sports update": true,
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

//nolint:paralleltest // replaces newCommandClient and sets env.
func TestEveryCommandRejectsUnsupportedFlagsWithoutWrites(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	t.Setenv("INTERVALS_ICU_API_KEY", "k")
	t.Setenv("INTERVALS_ICU_ATHLETE_ID", "i1")
	var mu sync.Mutex
	writes := map[string]int{}
	current := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != http.MethodGet {
			writes[current]++
			w.WriteHeader(http.StatusTeapot)

			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"name":"Z2","type":"Ride","start_date_local":"2026-10-28T07:00:00"}`))
	}))
	t.Cleanup(server.Close)
	withMockCommandClient(t, icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL)))

	registry := NewCommandRegistry()
	registerAllCommands(registry)
	accepted := map[string]bool{}
	for _, resource := range registry.Resources() {
		for _, action := range registry.Actions(resource) {
			key := resource + " " + action
			cmd, _ := registry.Lookup(resource, action)

			var stdout, stderr bytes.Buffer
			if code := run(commandArgs(resource, action, "--not-a-flag", "1"), &stdout, &stderr); code == 0 ||
				!strings.Contains(stderr.String(), "unknown flag --not-a-flag") {
				t.Errorf("%s --not-a-flag: code=%d stderr=%q", key, code, stderr.String())
			}

			if validateCommandInput(cmd, nil, map[string]string{"dry-run": ""}) != nil {
				continue
			}
			accepted[key] = true
			if readOnlyCommands()[key] {
				t.Errorf("%s is listed read-only but accepts --dry-run", key)
			}
			mu.Lock()
			current = key
			mu.Unlock()
			stdout.Reset()
			stderr.Reset()
			_ = run(commandArgs(resource, action, "1", "--name", "X", "--dry-run"), &stdout, &stderr)
		}
	}
	for key := range readOnlyCommands() {
		parts := strings.SplitN(key, " ", 2)
		if _, ok := registry.Lookup(parts[0], parts[1]); !ok {
			t.Errorf("readOnlyCommands lists unknown command %s", key)
		}
		// Plain runs against the write-failing mock confirm the
		// classification. Zepp, config and estimate-power reach other
		// services or local files, so they are only classified.
		if parts[0] == "zepp" || parts[0] == "config" || strings.HasPrefix(parts[1], "estimate-power") {
			continue
		}
		mu.Lock()
		current = key + " (plain)"
		mu.Unlock()
		var stdout, stderr bytes.Buffer
		_ = run(commandArgs(parts[0], parts[1], "1"), &stdout, &stderr)
	}
	for key := range accepted {
		if !dryRunCommands()[key] {
			t.Errorf("%s accepts --dry-run but is not in dryRunCommands", key)
		}
	}
	for key := range dryRunCommands() {
		if !accepted[key] {
			t.Errorf("%s is in dryRunCommands but rejects --dry-run", key)
		}
	}
	for key, n := range writes {
		t.Errorf("%s --dry-run sent %d write request(s)", key, n)
	}
}
