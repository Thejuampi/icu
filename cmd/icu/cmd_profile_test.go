package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

//nolint:paralleltest // swaps the process-wide config store and stdout.
func TestProfileSetAndShowRoundTrip(t *testing.T) {
	restore := icu.SetConfigStoreForTesting(icu.NewMemoryConfigStore())
	t.Cleanup(restore)

	if _, err := captureStdout(t, func() error {
		return profileSetCommand().Run(nil, map[string]string{
			"target-ftp": "280", "indoor-ftp": "250", "max-ramp": "5.5", "usual-days": "Tue, sat",
			"usual-times": "07:00", "min-z2-minutes": "120", "z2-low": "55", "z2-high": "75",
		}, nil)
	}); err != nil {
		t.Fatalf("profile set error = %v", err)
	}
	if _, err := captureStdout(t, func() error {
		return profileSetCommand().Run(nil, map[string]string{"target-ftp": "-", "usual-times": "-"}, nil)
	}); err != nil {
		t.Fatalf("profile clear error = %v", err)
	}

	out, err := captureStdout(t, func() error { return profileShowCommand().Run(nil, nil, nil) })
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
		if _, err := captureStdout(t, func() error { return profileSetCommand().Run(nil, bad, nil) }); err == nil {
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

	out, err := captureStdout(t, func() error { return profileShowCommand().Run(nil, nil, nil) })
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
