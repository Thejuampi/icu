package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestFindLastWriteSkipsPendingBackups(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	written := &eventBackup{EventID: 7, Reason: "events update", Event: json.RawMessage(`{}`), PostWriteHash: "h1"}
	if _, err := writeEventBackup(written, base); err != nil {
		t.Fatal(err)
	}
	pending := &eventBackup{EventID: 7, Reason: "events update", Event: json.RawMessage(`{}`), WritePending: true}
	if _, err := writeEventBackup(pending, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	last, err := findLastWrite(7)
	if err != nil || last.hash != "h1" || last.legacy {
		t.Fatalf("last = %+v err = %v, want the older written backup", last, err)
	}

	// Only pending backups: no write recorded, so no --force demanded.
	if _, err := writeEventBackup(&eventBackup{EventID: 8, Reason: "events update", Event: json.RawMessage(`{}`), WritePending: true}, base); err != nil {
		t.Fatal(err)
	}
	if last, err := findLastWrite(8); err != nil || last != (lastWrite{}) {
		t.Fatalf("pending-only last = %+v err = %v", last, err)
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestEventsUpdateValidatesBeforeBackup(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	client, _, puts := editableEventServer(t)
	_, err := captureStdout(t, func(stdout io.Writer) error {
		return eventsUpdateCommand().Run([]string{"3"}, map[string]string{
			"type": "Run", "desc": "- 30m 70%", "calculate-load": "true", "ftp": "300",
		}, client, stdout, io.Discard)
	})
	if err == nil || !strings.Contains(err.Error(), "not FTP-priced") || *puts != 0 {
		t.Fatalf("err = %v puts = %d", err, *puts)
	}
	if paths, _ := listEventBackups(3); len(paths) != 0 {
		t.Fatalf("rejected update left backups %v", paths)
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestEventsUpdateClearsWritePendingAfterWrite(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	client, _, puts := editableEventServer(t)
	if _, err := captureStdout(t, func(stdout io.Writer) error {
		return eventsUpdateCommand().Run([]string{"3"}, map[string]string{"name": "N"}, client, stdout, io.Discard)
	}); err != nil || *puts != 1 {
		t.Fatalf("err = %v puts = %d", err, *puts)
	}
	paths, _ := listEventBackups(3)
	backup, err := readEventBackup(paths[0])
	if err != nil || backup.WritePending || backup.PostWriteHash == "" {
		t.Fatalf("backup = %+v err = %v", backup, err)
	}
	if want := icu.EventBackupHash(&icu.Event{}); backup.PostWriteHash == want {
		t.Fatalf("unexpected empty-event hash")
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestEventsUpdateFailedPutLeavesPendingBackupIgnored(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	client, _, puts := editableEventServer(t)
	if _, err := captureStdout(t, func(stdout io.Writer) error {
		return eventsUpdateCommand().Run([]string{"3"}, map[string]string{"name": "N"}, client, stdout, io.Discard)
	}); err != nil || *puts != 1 {
		t.Fatalf("first: %v", err)
	}
	first, _ := findLastWrite(3)
	// Simulate a later update whose PUT failed: a pending backup on top.
	if _, err := writeEventBackup(&eventBackup{EventID: 3, Reason: "events update", Event: json.RawMessage(`{}`), WritePending: true},
		time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if last, _ := findLastWrite(3); last != first {
		t.Fatalf("last = %+v, want %+v", last, first)
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestEventsUpdateServerErrorLeavesPendingBackup(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts++
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":3,"name":"old","type":"Ride","category":"WORKOUT"}`))
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	_, err := captureStdout(t, func(stdout io.Writer) error {
		return eventsUpdateCommand().Run([]string{"3"}, map[string]string{"name": "N"}, client, stdout, io.Discard)
	})
	if err == nil || puts != 1 {
		t.Fatalf("err = %v puts = %d, want PUT failure", err, puts)
	}
	paths, _ := listEventBackups(3)
	if len(paths) != 1 {
		t.Fatalf("backups = %v", paths)
	}
	backup, err := readEventBackup(paths[0])
	if err != nil || !backup.WritePending || backup.PostWriteHash != "" {
		t.Fatalf("backup = %+v err = %v, want pending without hash", backup, err)
	}
	if last, _ := findLastWrite(3); last != (lastWrite{}) {
		t.Fatalf("failed write counted as last write: %+v", last)
	}
	if _, err := resolveRevertBackup(map[string]string{"id": "3"}); err == nil {
		t.Fatal("revert picked a pending backup")
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestResolveRevertBackupSkipsPending(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	older, err := writeEventBackup(&eventBackup{EventID: 9, Reason: "events update", Event: json.RawMessage(`{}`), PostWriteHash: "h"}, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeEventBackup(&eventBackup{EventID: 9, Reason: "events update", Event: json.RawMessage(`{}`), WritePending: true},
		base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveRevertBackup(map[string]string{"id": "9"}); err != nil || got != older {
		t.Fatalf("got %q err %v, want %q", got, err, older)
	}
}
