package main

import (
	"encoding/json"
	"fmt"
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
			// Like Intervals.icu: the PUT returns the stored event.
			var stored map[string]any
			_ = json.Unmarshal(body, &stored)
			stored["id"] = 7
			echoed, _ := json.Marshal(stored)
			mu.Lock()
			event7 = string(echoed)
			mu.Unlock()
			_, _ = response.Write(echoed)
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
			_, _ = response.Write([]byte(`[{"id":5,"uid":"u-1"},{"id":6,"uid":"other","external_id":"u-1"}]`))
		case request.Method == http.MethodGet:
			_, _ = response.Write([]byte(`{"id":5,"uid":"u-1","name":"Old"}`))
		default:
			_, _ = response.Write([]byte(`{"id":5}`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	if err := eventsCreateCommand().Run(nil, map[string]string{"upsert": "true", "name": "x"}, client); err == nil {
		t.Fatal("--upsert without --uid accepted")
	}
	flags := map[string]string{"upsert": "true", "uid": "u-1", "name": "New", "start-date": "2026-10-27T07:00:00"}
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

//nolint:paralleltest // sets ICU_BACKUP_DIR and captures stdout.
func TestEventsRevertRecreateIsIdempotent(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	var calendar string
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/events/8"):
			_, _ = response.Write([]byte(`{"id":8,"name":"Z2","category":"WORKOUT","start_date_local":"2026-10-27T07:00:00"}`))
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/events"):
			if calendar == "" {
				_, _ = response.Write([]byte(`[]`))
			} else {
				_, _ = response.Write([]byte(calendar))
			}
		case request.Method == http.MethodPost:
			posts++
			_, _ = response.Write([]byte(`{"id":99}`))
		default:
			_, _ = response.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	if _, err := captureStdout(t, func() error { return eventsDeleteCommand().Run([]string{"8"}, nil, client) }); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error {
		return eventsRevertCommand().Run(nil, map[string]string{"id": "8"}, client)
	}); err != nil || posts != 1 {
		t.Fatalf("first revert: posts=%d err=%v", posts, err)
	}
	// The backup now records restoredEventId: a second revert aborts.
	err := eventsRevertCommand().Run(nil, map[string]string{"id": "8"}, client)
	if err == nil || !strings.Contains(err.Error(), "already restored as event 99") || posts != 1 {
		t.Fatalf("second revert: posts=%d err=%v", posts, err)
	}
	// A fresh backup (e.g. restored by hand) is caught by the calendar lookup.
	paths, _ := listEventBackups(8)
	backup, _ := readEventBackup(paths[0])
	backup.RestoredEventID = 0
	if err := rewriteEventBackup(paths[0], &backup); err != nil {
		t.Fatal(err)
	}
	// A second, similar session that day (same name, other start time or
	// type) is not a copy: the recreate goes ahead.
	calendar = `[{"id":50,"name":"Z2","category":"WORKOUT","start_date_local":"2026-10-27T17:00:00"},` +
		`{"id":51,"name":"Z2","category":"WORKOUT","type":"Run","start_date_local":"2026-10-27T07:00:00"}]`
	if _, err := captureStdout(t, func() error {
		return eventsRevertCommand().Run(nil, map[string]string{"id": "8"}, client)
	}); err != nil || posts != 2 {
		t.Fatalf("sibling revert: posts=%d err=%v", posts, err)
	}
	backup, _ = readEventBackup(paths[0])
	backup.RestoredEventID = 0
	if err := rewriteEventBackup(paths[0], &backup); err != nil {
		t.Fatal(err)
	}
	calendar = `[{"id":99,"name":"Z2","category":"WORKOUT","start_date_local":"2026-10-27T07:00:00"}]`
	err = eventsRevertCommand().Run(nil, map[string]string{"id": "8"}, client)
	if err == nil || !strings.Contains(err.Error(), "already matches backup") || posts != 2 {
		t.Fatalf("lookup revert: posts=%d err=%v", posts, err)
	}
	if _, err := captureStdout(t, func() error {
		return eventsRevertCommand().Run(nil, map[string]string{"id": "8", "force": "true"}, client)
	}); err != nil || posts != 3 {
		t.Fatalf("forced revert: posts=%d err=%v", posts, err)
	}
}

func TestWriteFileAtomicReplacesWithoutLeftovers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	data, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if len(entries) != 1 || string(data) != "new" || info.Mode().Perm() != eventBackupFileMode {
		t.Fatalf("entries %v data %q mode %v", entries, data, info.Mode())
	}
	if err := writeFileAtomic(filepath.Join(dir, "missing", "x.json"), []byte("x")); err == nil {
		t.Fatal("write into a missing dir succeeded")
	}
	// Renaming over a non-empty directory fails and leaves no temp file.
	target := filepath.Join(dir, "busy")
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(target, []byte("x")); err == nil {
		t.Fatal("rename over a directory succeeded")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Fatalf("leftovers: %v", entries)
	}
	if err := rewriteEventBackup(filepath.Join(dir, "missing", "b.json"), &eventBackup{}); err == nil {
		t.Fatal("rewrite into a missing dir succeeded")
	}
}

// upsertCalendarServer serves a calendar whose only uid match is dated
// matchDate: the events list returns it only when the requested range
// covers that date, like intervals.icu. POST returns postID.
func upsertCalendarServer(t *testing.T, matchDate string, postID int) (*icu.Client, *int) {
	t.Helper()
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/events"):
			query := request.URL.Query()
			body := `[{"id":40,"uid":"other","start_date_local":"2026-10-20T07:00:00"}`
			if query.Get("oldest") <= matchDate && matchDate <= query.Get("newest")+"T23:59:59" {
				body += `,{"id":5,"uid":"u-1","start_date_local":"` + matchDate + `"}`
			}
			_, _ = response.Write([]byte(body + `]`))
		case request.Method == http.MethodGet:
			_, _ = response.Write([]byte(`{"id":5,"uid":"u-1","name":"Old"}`))
		default:
			posts++
			_, _ = fmt.Fprintf(response, `{"id":%d,"uid":"u-1","name":"New","start_date_local":"2026-10-27T07:00:00"}`, postID)
		}
	}))
	t.Cleanup(server.Close)

	return icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL)), &posts
}

func TestEventsCreateUpsertBacksUpMatchYearsAway(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	// A uid match nine years before the new date must still be found: this
	// fails if the search window is shrunk.
	client, _ := upsertCalendarServer(t, "2017-11-01T07:00:00", 5)
	flags := map[string]string{"upsert": "true", "uid": "u-1", "name": "New", "start-date": "2026-10-27T07:00:00"}
	if _, err := captureStdout(t, func() error { return eventsCreateCommand().Run(nil, flags, client) }); err != nil {
		t.Fatal(err)
	}
	if paths, _ := listEventBackups(5); len(paths) != 1 {
		t.Fatalf("old uid match not backed up: %v", paths)
	}
}

func TestEventsCreateUpsertFailsWhenOverwrittenEventWasNotBackedUp(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	flags := map[string]string{"upsert": "true", "uid": "u-1", "name": "New", "start-date": "2026-10-27T07:00:00"}
	// The match lies outside any window; the POST updates existing event 5.
	client, posts := upsertCalendarServer(t, "1990-01-01T07:00:00", 5)
	_, err := captureStdout(t, func() error { return eventsCreateCommand().Run(nil, flags, client) })
	if err == nil || !strings.Contains(err.Error(), `ALREADY OVERWROTE existing event 5 (now dated 2026-10-27, named "New"`) ||
		!strings.Contains(err.Error(), "NO BACKUP") || *posts != 1 {
		t.Fatalf("err = %v posts=%d", err, *posts)
	}
	// A new id (above every id seen) is a plain create.
	client, _ = upsertCalendarServer(t, "1990-01-01T07:00:00", 41)
	if _, err := captureStdout(t, func() error { return eventsCreateCommand().Run(nil, flags, client) }); err != nil {
		t.Fatal(err)
	}
}

// editableEventServer stores event 3; PUT replaces it and echoes it back.
func editableEventServer(t *testing.T) (*icu.Client, *string, *int) {
	t.Helper()
	var mu sync.Mutex
	stored := `{"id":3,"name":"Z2","type":"Ride","start_date_local":"2026-10-28T07:00:00","description":"a"}`
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		response.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case http.MethodGet:
			_, _ = response.Write([]byte(stored))
		case http.MethodPut:
			puts++
			body, _ := io.ReadAll(request.Body)
			var current, patch map[string]any
			_ = json.Unmarshal([]byte(stored), &current)
			_ = json.Unmarshal(body, &patch)
			for key, value := range patch {
				current[key] = value
			}
			current["id"] = 3
			data, _ := json.Marshal(current)
			stored = string(data)
			_, _ = response.Write(data)
		default:
			_, _ = response.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)

	return icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL)), &stored, &puts
}

func TestEventsUpdateDetectsThirdPartyEdits(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	client, stored, puts := editableEventServer(t)
	update := func(flags map[string]string) error {
		_, err := captureStdout(t, func() error { return eventsUpdateCommand().Run([]string{"3"}, flags, client) })

		return err
	}
	// First write: no earlier backup, nothing to compare.
	if err := update(map[string]string{"name": "Z2 long"}); err != nil || *puts != 1 {
		t.Fatalf("first update: %v puts=%d", err, *puts)
	}
	paths, _ := listEventBackups(3)
	if backup, _ := readEventBackup(paths[0]); backup.PostWriteHash == "" || backup.PostWriteAt == "" {
		t.Fatalf("post-write hash not stored: %+v", backup)
	}
	// Unchanged since our write: passes.
	if err := update(map[string]string{"name": "Z2 longer"}); err != nil || *puts != 2 {
		t.Fatalf("unchanged update: %v puts=%d", err, *puts)
	}
	// Edited in Intervals.icu: aborts before any PUT.
	*stored = strings.Replace(*stored, `"description":"a"`, `"description":"edited on the web"`, 1)
	err := update(map[string]string{"name": "Z2 again"})
	if err == nil || !strings.Contains(err.Error(), "changed outside this CLI") || *puts != 2 {
		t.Fatalf("edited update: %v puts=%d", err, *puts)
	}
	// --force overwrites.
	if err := update(map[string]string{"name": "Z2 forced", "force": "true"}); err != nil || *puts != 3 {
		t.Fatalf("forced update: %v puts=%d", err, *puts)
	}
}

func TestEventsRevertDetectsThirdPartyEditsAndLegacyBackups(t *testing.T) {
	t.Setenv(eventBackupDirEnv, t.TempDir())
	client, stored, puts := editableEventServer(t)
	if _, err := captureStdout(t, func() error {
		return eventsUpdateCommand().Run([]string{"3"}, map[string]string{"name": "Changed"}, client)
	}); err != nil {
		t.Fatal(err)
	}
	revert := func(flags map[string]string) error {
		_, err := captureStdout(t, func() error { return eventsRevertCommand().Run(nil, flags, client) })

		return err
	}
	*stored = strings.Replace(*stored, `"description":"a"`, `"description":"web edit"`, 1)
	if err := revert(map[string]string{"id": "3"}); err == nil ||
		!strings.Contains(err.Error(), "changed outside this CLI") || *puts != 1 {
		t.Fatalf("edited revert: %v puts=%d", err, *puts)
	}
	// The refused revert wrote no snapshot; undo the web edit: unchanged
	// passes.
	if paths, _ := listEventBackups(3); len(paths) != 1 {
		t.Fatalf("refused revert left a backup: %v", paths)
	}
	*stored = strings.Replace(*stored, `"description":"web edit"`, `"description":"a"`, 1)
	if err := revert(map[string]string{"id": "3"}); err != nil || *puts != 2 {
		t.Fatalf("unchanged revert: %v puts=%d", err, *puts)
	}
	// A backup without postWriteHash (older version) needs --force.
	paths, _ := listEventBackups(3)
	legacy, _ := readEventBackup(paths[0])
	legacy.PostWriteHash = ""
	if err := rewriteEventBackup(paths[0], &legacy); err != nil {
		t.Fatal(err)
	}
	original := paths[len(paths)-1]
	err := revert(map[string]string{"backup": original})
	if err == nil || !strings.Contains(err.Error(), "no post-write hash") || *puts != 2 {
		t.Fatalf("legacy revert: %v puts=%d", err, *puts)
	}
	if err := revert(map[string]string{"backup": original, "force": "true"}); err != nil || *puts != 3 {
		t.Fatalf("forced revert: %v puts=%d", err, *puts)
	}
}
