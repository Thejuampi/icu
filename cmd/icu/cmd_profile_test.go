package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

//nolint:paralleltest // swaps the process-wide config store and stdout.
func TestProfileSetAndShowRoundTrip(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)

	if _, err := captureStdout(t, func(stdout io.Writer) error {
		return profileSetCommand().Run(nil, map[string]string{
			"target-ftp": "280", "indoor-ftp": "250", "max-ramp": "5.5", "usual-days": "Tue, sat",
			"usual-times": "07:00", "min-z2-minutes": "120", "z2-low": "55", "z2-high": "75",
		}, nil, stdout)
	}); err != nil {
		t.Fatalf("profile set error = %v", err)
	}
	if _, err := captureStdout(t, func(stdout io.Writer) error {
		return profileSetCommand().Run(nil, map[string]string{"target-ftp": "-", "usual-times": "-"}, nil, stdout)
	}); err != nil {
		t.Fatalf("profile clear error = %v", err)
	}

	out, err := captureStdout(t, func(stdout io.Writer) error { return profileShowCommand().Run(nil, nil, nil, stdout) })
	if err != nil {
		t.Fatalf("profile show error = %v", err)
	}
	var got icu.TrainingProfile
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json error = %v (%s)", err, out)
	}
	if got.TargetFTP != 0 || got.IndoorFTP != 250 || got.MaxRampPerWeek != 5.5 ||
		strings.Join(got.UsualDays, ",") != "tue,sat" || got.UsualStartTimes != nil || got.MinZ2MinutesPerWeek != 120 {
		t.Fatalf("profile = %+v", got)
	}

	for _, bad := range []map[string]string{{"target-ftp": "x"}, {"max-ramp": "y"}, {"usual-days": "funday"}} {
		if _, err := captureStdout(t, func(stdout io.Writer) error { return profileSetCommand().Run(nil, bad, nil, stdout) }); err == nil {
			t.Fatalf("profile set %v accepted", bad)
		}
	}

	cfg, _ := icu.LoadConfig()
	cfg.Profile = &icu.TrainingProfile{UsualDays: []string{"sat"}}
	if err := icu.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	warnings := planProfileWarnings(&icu.PlanProposalFile{Sessions: []icu.PlanResolvedSession{
		{Date: "2026-11-02", Name: "A", Action: icu.CalendarActionCreate},
	}}, nil)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "not a usual training day") {
		t.Fatalf("plan profile warnings = %v", warnings)
	}
}

//nolint:paralleltest // swaps the process-wide config store and stdout.
func TestProfileShowEmpty(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)

	out, err := captureStdout(t, func(stdout io.Writer) error { return profileShowCommand().Run(nil, nil, nil, stdout) })
	if err != nil || strings.TrimSpace(out) != "{}" {
		t.Fatalf("profile show = %q, %v", out, err)
	}
}

//nolint:paralleltest // swaps the process-wide config store.
func TestPlanProfileWarningsIncludeMonotonyWithoutProfile(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)

	warnings := planProfileWarnings(&icu.PlanProposalFile{Sessions: []icu.PlanResolvedSession{
		{Date: "2026-11-02", Name: "A", Action: icu.CalendarActionCreate, Desc: "- 20m 90%"},
		{Date: "2026-11-04", Name: "B", Action: icu.CalendarActionCreate, Desc: "- 20m 90%"},
		{Date: "2026-11-06", Name: "C", Action: icu.CalendarActionCreate, Desc: "- 20m 90%"},
	}}, nil)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], "monotony:") {
		t.Fatalf("warnings = %v", warnings)
	}
}

//nolint:paralleltest // swaps the process-wide config store.
func TestReadPlanSportSettingsFillsIndoorFTPFromProfile(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)
	if err := icu.SaveConfig(&icu.Config{Profile: &icu.TrainingProfile{IndoorFTP: 250}}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"ftp":266,"types":["Ride","VirtualRide"]}`))
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	settings, warnings, err := readPlanSportSettings(client, "Ride")
	if err != nil || settings.FTP != 266 || settings.IndoorFTP != 250 || len(warnings) != 1 {
		t.Fatalf("settings = %+v, warnings = %v, err = %v", settings, warnings, err)
	}
	rebalance, err := readRebalanceSportSettings(client, map[string]string{"type": "Ride"})
	if err != nil || rebalance.IndoorFTP != 250 {
		t.Fatalf("rebalance settings = %+v, %v", rebalance, err)
	}
}

// Regression: the live sport-settings shape (snake_case indoor_ftp) must reach
// indoor sessions, and a plan without --type must still fetch sport settings.
//
//nolint:paralleltest // swaps the process-wide config store.
func TestPlanUsesIntervalsIndoorFTPWithoutTypeFlag(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)
	var settingsPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", request.Method, request.URL.Path)
		}
		if strings.Contains(request.URL.Path, "sport-settings") {
			settingsPaths = append(settingsPaths, request.URL.Path)
			_, _ = response.Write([]byte(`{"id":1858902,"types":["Ride","VirtualRide","MountainBikeRide","GravelRide","TrackRide","Cyclocross"],"ftp":266,"indoor_ftp":250}`))

			return
		}
		_, _ = response.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	intent := `{"sessions":[` +
		`{"date":"2026-11-03","name":"VR","type":"VirtualRide","startTime":"07:00","desc":"- 60m 70%"},` +
		`{"date":"2026-11-04","name":"Road","type":"Ride","startTime":"07:00","desc":"- 60m 75%"},` +
		`{"date":"2026-11-05","name":"Trainer","type":"Ride","indoor":true,"startTime":"07:00","desc":"- 45m 80%"}]}`
	file := filepath.Join(t.TempDir(), "intent.json")
	if err := os.WriteFile(file, []byte(intent), 0o600); err != nil {
		t.Fatal(err)
	}

	input, err := readPlanInput(map[string]string{"intent-file": file, "now-date": "2026-10-26"}, client)
	if err != nil {
		t.Fatal(err)
	}
	if len(settingsPaths) != 1 || !strings.HasSuffix(settingsPaths[0], "/sport-settings/VirtualRide") {
		t.Fatalf("sport-settings requests = %v", settingsPaths)
	}
	if input.SportSettings.FTP != 266 || input.SportSettings.IndoorFTP != 250 || len(input.Warnings) != 0 {
		t.Fatalf("settings = %+v, warnings = %v", input.SportSettings, input.Warnings)
	}
	proposal := icu.BuildPlanProposal(&input)
	want := map[string]int{"VR": 250, "Road": 266, "Trainer": 250}
	for _, session := range proposal.Sessions {
		if session.FTPApplied != want[session.Name] {
			t.Errorf("%s ftpApplied = %d, want %d", session.Name, session.FTPApplied, want[session.Name])
		}
	}
	if len(proposal.Sessions) != len(want) {
		t.Fatalf("sessions = %+v, validation = %+v", proposal.Sessions, proposal.Validation)
	}
}

//nolint:paralleltest // swaps the process-wide config store and stdout.
func TestProfileSetThresholdsJSON(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)

	if _, err := captureStdout(t, func(stdout io.Writer) error {
		return profileSetCommand().Run(nil, map[string]string{"thresholds": `{"watchZ":2,"redZ":3,"sleepWatch":72}`}, nil, stdout)
	}); err != nil {
		t.Fatal(err)
	}
	got := profileThresholds()
	if got == nil || got.WatchZ != 2 || got.SleepWatch == nil || *got.SleepWatch != 72 {
		t.Fatalf("thresholds = %+v", got)
	}
	for _, bad := range []string{`{"watchZ":3,"redZ":2}`, `{"unknown":1}`, `not json`} {
		if _, err := captureStdout(t, func(stdout io.Writer) error {
			return profileSetCommand().Run(nil, map[string]string{"thresholds": bad}, nil, stdout)
		}); err == nil {
			t.Fatalf("thresholds %s accepted", bad)
		}
	}
	if _, err := captureStdout(t, func(stdout io.Writer) error {
		return profileSetCommand().Run(nil, map[string]string{"thresholds": "-"}, nil, stdout)
	}); err != nil || profileThresholds() != nil {
		t.Fatalf("clear err = %v, thresholds = %+v", err, profileThresholds())
	}
}
