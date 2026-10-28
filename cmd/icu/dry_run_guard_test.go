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
	if code := run([]string{"icu", "events", "update", "3", "--name", "x", "--dry-run"}, &stdout, &stderr); code != 0 || *writes != 0 {
		t.Fatalf("code=%d writes=%d stderr=%q", code, *writes, stderr.String())
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
