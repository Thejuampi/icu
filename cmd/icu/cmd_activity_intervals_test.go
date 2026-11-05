package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// intervalsServer serves the synthetic fixture (invented values, no real
// ids) for GET /activity/{id}?intervals=true and fails any other request.
func intervalsServer(t *testing.T) *icu.Client {
	t.Helper()
	body, err := os.ReadFile("../../testdata/activity_intervals/synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/activity/i970000001" || r.URL.Query().Get("intervals") != "true" {
			t.Errorf("unexpected %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusNotFound)

			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	return icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
}

func TestActivityIntervalsTable(t *testing.T) {
	t.Parallel()
	client := intervalsServer(t)
	out, err := captureStdout(t, func(stdout io.Writer) error {
		return activityIntervalsCommand().Run([]string{"i970000001"}, map[string]string{}, client, stdout, io.Discard)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"activity i970000001: 3 interval(s), 1 group(s)",
		"RECOVERY", "WORK", "rep 1", "312", "315", "161", "4.2%", "5.1%", "8:00", "10:00", "g-syn-1",
		"group g-syn-1: 2 x, avg 310 W, avg HR 162",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestActivityIntervalsJSON(t *testing.T) {
	t.Parallel()
	client := intervalsServer(t)
	out, err := captureStdout(t, func(stdout io.Writer) error {
		return activityIntervalsCommand().Run([]string{"i970000001"}, map[string]string{"json": "true"}, client, stdout, io.Discard)
	})
	if err != nil {
		t.Fatal(err)
	}
	var dto icu.IntervalsDTO
	if err := json.Unmarshal([]byte(out), &dto); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if dto.ID != "i970000001" || len(dto.Intervals) != 3 || len(dto.Groups) != 1 {
		t.Fatalf("dto %+v", dto)
	}
	w := dto.Intervals[1]
	if w.Type != "WORK" || w.AvgPower != 312 || w.WeightedPower != 315 || w.AvgHR != 161 || w.Decoupling != 4.2 ||
		w.MovingTime != 480 || w.StartTime != 600 || w.GroupID != "g-syn-1" || w.Label != "rep 1" || w.TrainingLoad != 14.2 {
		t.Fatalf("work interval %+v", w)
	}
	if g := dto.Groups[0]; g.ID != "g-syn-1" || g.Count != 2 || g.AvgPower != 310 || g.Decoupling != 4.6 {
		t.Fatalf("group %+v", g)
	}
	if !strings.Contains(out, `"icuIntervals"`) || !strings.Contains(out, `"icuGroups"`) {
		t.Fatalf("json keys missing:\n%s", out)
	}
}

func TestActivityIntervalsEmpty(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"id":"i970000002","icu_intervals":null}`)
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	out, err := captureStdout(t, func(stdout io.Writer) error {
		return activityIntervalsCommand().Run([]string{"i970000002"}, map[string]string{"json": "true"}, client, stdout, io.Discard)
	})
	if err != nil || !strings.Contains(out, `"icuIntervals": []`) {
		t.Fatalf("err=%v out=%s", err, out)
	}
	out, _ = captureStdout(t, func(stdout io.Writer) error {
		return activityIntervalsCommand().Run([]string{"i970000002"}, map[string]string{}, client, stdout, io.Discard)
	})
	if !strings.Contains(out, "no intervals") {
		t.Fatalf("table: %s", out)
	}
	if err := activityIntervalsCommand().Run(nil, map[string]string{}, client, io.Discard, io.Discard); err == nil {
		t.Fatal("missing id must fail")
	}
}
