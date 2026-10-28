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
		{"wellness", "update", []string{"2026-10-27"}, map[string]string{"resting-hr": "48", "hrv": "60"}, "hrv,restingHR"},
		{
			"activities", "manual", nil,
			map[string]string{"type": "Ride", "name": "R", "moving-time": "3600", "distance": "30000", "training-load": "60", "start-date": "2026-10-27T08:00:00"},
			"distance,icu_training_load,moving_time,name,start_date_local,type",
		},
		{"chats", "send", nil, map[string]string{"content": "hi", "to": "i2"}, "content,to_athlete_id"},
	}
	for _, tc := range cases {
		rec, client := newWriteRecorder(t, `{}`)
		rec.applyPut = true
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

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestFTPUpdateSkipsWriteWhenUnchanged(t *testing.T) {
	rec, client := newWriteRecorder(t, ftpSettingsJSON)
	out, err := captureStdout(t, func() error { return runFTPUpdate(nil, map[string]string{"value": "266"}, client) })
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.writes) != 0 || !strings.Contains(out, "no changes") {
		t.Fatalf("writes=%+v out=%s", rec.writes, out)
	}
}

const athleteSnakeJSON = `{"id":"i1","name":"A","weight":80,"icu_weight":79.5,"height":1.8,"icu_resting_hr":48,"height_units":"CM"}`

func TestAthleteDecodesSnakeFields(t *testing.T) {
	t.Parallel()
	var a icu.Athlete
	if err := json.Unmarshal([]byte(athleteSnakeJSON), &a); err != nil {
		t.Fatal(err)
	}
	if a.ICUWeight != 79.5 || a.RestingHR != 48 || a.HeightUnits != "CM" {
		t.Fatalf("athlete = %+v", a)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAthleteUpdateDryRunDoesNotWrite(t *testing.T) {
	rec, client := newWriteRecorder(t, athleteSnakeJSON)
	out, err := captureStdout(t, func() error {
		return runAthleteUpdate(nil, map[string]string{"icu-weight": "78", "dry-run": "true"}, client)
	})
	if err != nil {
		t.Fatal(err)
	}
	var res athleteUpdateResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if len(rec.writes) != 0 || !res.DryRun || res.Applied || res.Before["icu_weight"] != 79.5 {
		t.Fatalf("writes=%+v res=%+v", rec.writes, res)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAthleteUpdateReadsBack(t *testing.T) {
	rec, client := newWriteRecorder(t, athleteSnakeJSON)
	rec.applyPut = true
	flags := map[string]string{"weight": "81", "icu-weight": "78", "height": "1.82", "name": "B", "bio": "x"}
	out, err := captureStdout(t, func() error { return runAthleteUpdate(nil, flags, client) })
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.writes) != 1 || rec.writes[0].keys != "bio,height,icu_weight,name,weight" {
		t.Fatalf("writes=%+v", rec.writes)
	}
	if !strings.Contains(out, `"applied": true`) && !strings.Contains(out, `"applied":true`) {
		t.Fatalf("out=%s", out)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAthleteUpdateReadBackMismatchExitsNonZero(t *testing.T) {
	_, client := newWriteRecorder(t, athleteSnakeJSON) // applyPut=false: server ignores the write
	_, err := captureStdout(t, func() error { return runAthleteUpdate(nil, map[string]string{"icu-weight": "70"}, client) })
	if err == nil || !strings.Contains(err.Error(), "icu_weight") {
		t.Fatalf("err = %v", err)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAthleteUpdateGetError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	if err := runAthleteUpdate(nil, map[string]string{"weight": "1"}, client); err == nil {
		t.Fatal("want error")
	}
}
