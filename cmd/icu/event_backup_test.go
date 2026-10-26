package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	icu "github.com/Thejuampi/icu"
)

type recordedRequest struct {
	method, path, body string
}

func newBackupTestServer(t *testing.T) (*icu.Client, *[]recordedRequest) {
	t.Helper()

	var mu sync.Mutex
	var requests []recordedRequest
	event7 := `{"id":7,"name":"Original","description":"- 60m 65%","custom_field":"kept"}`
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		mu.Lock()
		requests = append(requests, recordedRequest{request.Method, request.URL.Path, string(body)})
		mu.Unlock()
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/events/7"):
			mu.Lock()
			_, _ = response.Write([]byte(event7))
			mu.Unlock()
		case request.Method == http.MethodPut && strings.HasSuffix(request.URL.Path, "/events/7"):
			mu.Lock()
			event7 = string(body)
			mu.Unlock()
			_, _ = response.Write([]byte(`{"id":7}`))
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/events/8"):
			_, _ = response.Write([]byte(`{"id":8,"name":"Dropped"}`))
		case request.Method == http.MethodPost:
			_, _ = response.Write([]byte(`{"id":99}`))
		default:
			_, _ = response.Write([]byte(`{"id":7}`))
		}
	}))
	t.Cleanup(server.Close)

	return icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL)), &requests
}

//nolint:paralleltest // sets ICU_BACKUP_DIR and captures stdout.
func TestApplyCalendarOperationsBacksUpAndRevertRestores(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(eventBackupDirEnv, dir)
	client, requests := newBackupTestServer(t)

	operations := []icu.CalendarOperation{
		{
			ID: "u", Action: icu.CalendarActionUpdate, EventID: 7, Status: icu.CalendarStatusPending,
			Body: icu.EventEx{Name: "Changed"},
		},
		{ID: "c", Action: icu.CalendarActionCancel, EventID: 8, Status: icu.CalendarStatusPending},
		{ID: "n", Action: icu.CalendarActionCreate, Status: icu.CalendarStatusPending, Body: icu.EventEx{Name: "New"}},
	}
	summary := applyCalendarOperations(client, operations)
	if summary.Applied != 3 || summary.Results[0].BackupPath == "" || summary.Results[1].BackupPath == "" ||
		summary.Results[2].BackupPath != "" {
		t.Fatalf("summary = %+v", summary)
	}
	if (*requests)[0].method != http.MethodGet || (*requests)[1].method != http.MethodPut {
		t.Fatalf("backup must precede the update: %+v", *requests)
	}

	listed, err := captureStdout(t, func() error { return eventsBackupsCommand().Run(nil, map[string]string{}, nil) })
	if err != nil || strings.Count(listed, `"path"`) != 2 {
		t.Fatalf("backups = %s, %v", listed, err)
	}

	dry, err := captureStdout(t, func() error {
		return eventsRevertCommand().Run(nil, map[string]string{"id": "7", "dry-run": "true"}, client)
	})
	if err != nil || !strings.Contains(dry, `"dryRun": true`) {
		t.Fatalf("dry run = %s, %v", dry, err)
	}

	*requests = nil
	out, err := captureStdout(t, func() error {
		return eventsRevertCommand().Run(nil, map[string]string{"id": "7"}, client)
	})
	if err != nil {
		t.Fatalf("revert error = %v", err)
	}
	var result eventRevertResult
	if err := json.Unmarshal([]byte(out), &result); err != nil || result.Action != "update" ||
		result.RestoredEventID != 7 || result.PreRevertBackup == "" {
		t.Fatalf("revert = %s, %v", out, err)
	}
	put := (*requests)[len(*requests)-1]
	if put.method != http.MethodPut || !strings.Contains(put.body, `"name":"Original"`) ||
		!strings.Contains(put.body, `"custom_field":"kept"`) {
		t.Fatalf("revert PUT = %+v", put)
	}

	// The event now matches the original backup: a second revert by id skips
	// the pre-revert snapshot, finds nothing to do and aborts without --force.
	if err := eventsRevertCommand().Run(nil, map[string]string{"id": "7"}, client); err == nil ||
		!strings.Contains(err.Error(), "already matches backup") {
		t.Fatalf("second revert error = %v", err)
	}

	*requests = nil
	out, err = captureStdout(t, func() error {
		return eventsRevertCommand().Run(nil, map[string]string{"backup": summary.Results[1].BackupPath}, client)
	})
	if err != nil || !strings.Contains(out, `"action": "create"`) || !strings.Contains(out, `"restoredEventId": 99`) {
		t.Fatalf("recreate = %s, %v", out, err)
	}
	post := (*requests)[0]
	if post.method != http.MethodPost || strings.Contains(post.body, `"id"`) || !strings.Contains(post.body, "Dropped") {
		t.Fatalf("recreate POST = %+v", post)
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestEventsRevertErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(eventBackupDirEnv, dir)
	client, requests := newBackupTestServer(t)

	for name, flagSet := range map[string]map[string]string{
		"missing":   {},
		"no backup": {"id": "5"},
		"bad file":  {"backup": dir + "/nope.json"},
	} {
		if err := eventsRevertCommand().Run(nil, flagSet, client); err == nil {
			t.Fatalf("%s: revert succeeded", name)
		}
	}
	empty := dir + "/empty.json"
	if err := os.WriteFile(empty, []byte(`{"eventId":0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := eventsRevertCommand().Run(nil, map[string]string{"backup": empty}, client); err == nil {
		t.Fatal("empty backup accepted")
	}
	if len(*requests) != 0 {
		t.Fatalf("failed reverts reached the API: %+v", *requests)
	}

	t.Setenv(eventBackupDirEnv, dir+"/missing")
	if paths, err := listEventBackups(0); err != nil || paths != nil {
		t.Fatalf("missing dir = %v, %v", paths, err)
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR.
func TestEventsUpdateFailsWhenBackupFails(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("unexpected %s after failed backup", request.Method)
		}
		http.Error(response, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	if err := eventsUpdateCommand().Run([]string{"7"}, map[string]string{"name": "x"}, client); err == nil {
		t.Fatal("update ran without a backup")
	}
	if err := eventsUpdateCommand().Run([]string{"abc"}, map[string]string{"name": "x"}, client); err == nil {
		t.Fatal("non-numeric id accepted")
	}
	result := applyCalendarOperation(client, &icu.CalendarOperation{
		Action: icu.CalendarActionUpdate, EventID: 7, Status: icu.CalendarStatusPending,
	})
	if result.Status != icu.CalendarStatusFailed || !strings.Contains(result.Error, "backup event 7") {
		t.Fatalf("result = %+v", result)
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR and captures stdout.
func TestEventsDeleteBacksUpAndRevertRecreates(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	client, requests := newBackupTestServer(t)

	out, err := captureStdout(t, func() error { return eventsDeleteCommand().Run([]string{"8"}, nil, client) })
	if err != nil || !strings.Contains(out, `"backupPath"`) {
		t.Fatalf("delete = %s, %v", out, err)
	}
	if (*requests)[0].method != http.MethodGet || (*requests)[1].method != http.MethodDelete {
		t.Fatalf("backup must precede the delete: %+v", *requests)
	}
	out, err = captureStdout(t, func() error {
		return eventsRevertCommand().Run(nil, map[string]string{"id": "8"}, client)
	})
	if err != nil || !strings.Contains(out, `"action": "create"`) {
		t.Fatalf("revert deleted = %s, %v", out, err)
	}
	for _, args := range [][]string{nil, {"x"}} {
		if err := eventsDeleteCommand().Run(args, nil, client); err == nil {
			t.Fatalf("delete %v: expected error", args)
		}
	}
}

//nolint:paralleltest // sets ICU_BACKUP_DIR and captures stdout.
func TestEventsCreateUpsertBacksUpMatchingEvent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(eventBackupDirEnv, dir)
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		methods = append(methods, request.Method+" "+request.URL.Path)
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[{"id":5,"external_id":"ext-1"},{"id":6,"external_id":"other"}]`))
		case request.Method == http.MethodGet:
			_, _ = response.Write([]byte(`{"id":5,"external_id":"ext-1","name":"Old"}`))
		default:
			_, _ = response.Write([]byte(`{"id":5}`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	flags := map[string]string{"upsert": "true", "external-id": "ext-1", "name": "New", "start-date": "2026-10-27T07:00:00"}
	if _, err := captureStdout(t, func() error { return eventsCreateCommand().Run(nil, flags, client) }); err != nil {
		t.Fatal(err)
	}
	paths, err := listEventBackups(5)
	if err != nil || len(paths) != 1 || !strings.HasPrefix(methods[len(methods)-1], http.MethodPost) {
		t.Fatalf("paths=%v methods=%v err=%v", paths, methods, err)
	}
	if others, _ := listEventBackups(6); len(others) != 0 {
		t.Fatalf("non-matching event backed up: %v", others)
	}
}

func TestEventDescriptionRejectsEmptyFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(path, []byte("\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := eventDescription(map[string]string{"desc-file": path}); err == nil ||
		!strings.Contains(err.Error(), "is empty") {
		t.Fatalf("err = %v", err)
	}
}
