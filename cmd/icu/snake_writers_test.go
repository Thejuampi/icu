package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	icu "github.com/Thejuampi/icu"
)

type recordedWrite struct {
	method, path string
	keys         string
	body         map[string]json.RawMessage
}

type writeRecorder struct {
	mu       sync.Mutex
	writes   []recordedWrite
	getBody  string
	applyPut bool
}

func newWriteRecorder(t *testing.T, getBody string) (*writeRecorder, *icu.Client) {
	t.Helper()
	rec := &writeRecorder{getBody: getBody}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(rec.getBody))

			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]json.RawMessage
		_ = json.Unmarshal(raw, &body)
		keys := make([]string, 0, len(body))
		for k := range body {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		rec.writes = append(rec.writes, recordedWrite{r.Method, r.URL.Path, strings.Join(keys, ","), body})
		if rec.applyPut {
			var cur map[string]json.RawMessage
			_ = json.Unmarshal([]byte(rec.getBody), &cur)
			for k, v := range body {
				cur[k] = v
			}
			updated, _ := json.Marshal(cur)
			rec.getBody = string(updated)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	return rec, icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestWriterCommandsSendSnakeCaseKeys(t *testing.T) {
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	cases := []struct {
		resource, action string
		args             []string
		flags            map[string]string
		wantKeys         string
	}{
		{"athlete", "update", nil, map[string]string{"weight": "81", "icu-weight": "80.5", "height": "1.81"}, "height,icu_weight,weight"},
		{
			"workouts", "create", nil,
			map[string]string{"name": "Z2", "folder-id": "7", "training-load": "50", "moving-time": "3600", "desc": "d"},
			"description,folder_id,icu_training_load,moving_time,name,type",
		},
		{"workouts", "update", []string{"1"}, map[string]string{"name": "Z2"}, "name"},
		{"folders", "create", nil, map[string]string{"name": "F"}, "name,type"},
		{"gear", "create", nil, map[string]string{"name": "Bike", "distance": "10"}, "distance,name,type"},
		{"gear", "update", []string{"g1"}, map[string]string{"name": "Bike"}, "name"},
		{"custom", "create", nil, map[string]string{"name": "C", "type": "FITNESS_CHART"}, "name,type"},
		{"custom", "update", []string{"1"}, map[string]string{"name": "C"}, "name"},
		{"chats", "send", nil, map[string]string{"content": "hi", "to": "i2"}, "content,to_athlete_id"},
	}
	for _, tc := range cases {
		rec, client := newWriteRecorder(t, `{}`)
		cmd, ok := registry.Lookup(tc.resource, tc.action)
		if !ok {
			t.Fatalf("missing %s %s", tc.resource, tc.action)
		}
		if _, err := captureStdout(t, func() error { return cmd.Run(tc.args, tc.flags, client) }); err != nil {
			t.Fatalf("%s %s: %v", tc.resource, tc.action, err)
		}
		if len(rec.writes) != 1 || rec.writes[0].keys != tc.wantKeys {
			t.Fatalf("%s %s: writes = %+v, want keys %s", tc.resource, tc.action, rec.writes, tc.wantKeys)
		}
	}
}

const ftpSettingsJSON = `{"id":1,"types":["Ride"],"ftp":266,"indoor_ftp":250}`

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestFTPUpdateWiresSnakeKeysAndReadsBack(t *testing.T) {
	for _, tc := range []struct {
		flags map[string]string
		want  string
	}{
		{map[string]string{"value": "270"}, "ftp"},
		{map[string]string{"value": "255", "indoor": "true"}, "indoor_ftp"},
	} {
		rec, client := newWriteRecorder(t, ftpSettingsJSON)
		rec.applyPut = true
		out, err := captureStdout(t, func() error { return runFTPUpdate(nil, tc.flags, client) })
		if err != nil {
			t.Fatal(err)
		}
		if len(rec.writes) != 1 || rec.writes[0].keys != tc.want {
			t.Fatalf("writes = %+v, want %s", rec.writes, tc.want)
		}
		var res ftpUpdateResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatal(err)
		}
		if !res.Applied || res.ReadBack == nil || res.Message != "" || res.Field != tc.want {
			t.Fatalf("result = %+v", res)
		}
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestFTPUpdateDryRunDoesNotWrite(t *testing.T) {
	rec, client := newWriteRecorder(t, ftpSettingsJSON)
	out, err := captureStdout(t, func() error {
		return runFTPUpdate(nil, map[string]string{"value": "255", "indoor": "true", "dry-run": "true"}, client)
	})
	if err != nil || len(rec.writes) != 0 {
		t.Fatalf("err=%v writes=%d", err, len(rec.writes))
	}
	var res ftpUpdateResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.Before != 250 || res.After != 255 || res.Body["indoor_ftp"] != 255 {
		t.Fatalf("result = %+v", res)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestFTPUpdateReadBackMismatchExitsNonZero(t *testing.T) {
	rec, client := newWriteRecorder(t, ftpSettingsJSON) // applyPut=false: server ignores the write
	_, err := captureStdout(t, func() error { return runFTPUpdate(nil, map[string]string{"value": "270"}, client) })
	if err == nil || !strings.Contains(err.Error(), "read-back mismatch on ftp") || len(rec.writes) != 1 {
		t.Fatalf("err=%v writes=%d", err, len(rec.writes))
	}
}

func TestFTPUpdateRejectsMissingValue(t *testing.T) {
	t.Parallel()
	if err := runFTPUpdate(nil, map[string]string{}, nil); err == nil {
		t.Fatal("want error")
	}
}
