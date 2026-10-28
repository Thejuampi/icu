package main

import (
	"encoding/json"
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
	_, err := captureStdout(t, func() error {
		return eventsUpdateCommand().Run([]string{"3"}, map[string]string{
			"type": "Run", "desc": "- 30m 70%", "calculate-load": "true", "ftp": "300",
		}, client)
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
	if _, err := captureStdout(t, func() error {
		return eventsUpdateCommand().Run([]string{"3"}, map[string]string{"name": "N"}, client)
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
	if _, err := captureStdout(t, func() error {
		return eventsUpdateCommand().Run([]string{"3"}, map[string]string{"name": "N"}, client)
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
