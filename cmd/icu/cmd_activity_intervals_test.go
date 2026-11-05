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

func bodyClient(t *testing.T, body string) *icu.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	return icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
}

func runIntervals(t *testing.T, client *icu.Client, id string, flags map[string]string) string {
	t.Helper()
	out, err := captureStdout(t, func(stdout io.Writer) error {
		return activityIntervalsCommand().Run([]string{id}, flags, client, stdout, io.Discard)
	})
	if err != nil {
		t.Fatal(err)
	}

	return out
}

// Kills: missing id falls back to the argument; missing icu_groups is [] not null.
func TestActivityIntervalsIDFallbackAndEmptyGroups(t *testing.T) {
	t.Parallel()
	client := bodyClient(t, `{"icu_intervals":[{"type":"WORK","moving_time":60}]}`)
	out := runIntervals(t, client, "i970000020", map[string]string{"json": "true"})
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if string(got["id"]) != `"i970000020"` || string(got["icuGroups"]) != "[]" {
		t.Fatalf("id=%s icuGroups=%s", got["id"], got["icuGroups"])
	}
	if table := runIntervals(t, client, "i970000020", map[string]string{}); !strings.Contains(table, "activity i970000020: 1 interval(s), 0 group(s)") {
		t.Fatalf("table: %s", table)
	}
}

// Kills the hours branch of clock; real 0 decoupling/zone print 0; tabs and
// newlines in labels become spaces.
func TestActivityIntervalsCells(t *testing.T) {
	t.Parallel()
	client := bodyClient(t, `{"id":"i970000021","icu_intervals":[`+
		`{"type":"WORK","label":"a\tb\nc","start_time":3600,"moving_time":3725,"decoupling":0,"zone":0},`+
		`{"type":"RECOVERY","moving_time":59}]}`)
	out := runIntervals(t, client, "i970000021", map[string]string{})
	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		t.Fatalf("output: %s", out)
	}
	work, rec := strings.Fields(lines[2]), strings.Fields(lines[3])
	// # TYPE LABEL(a b c) START DURATION AVG NP MAX AVGHR MAXHR DEC ZONE LOAD GROUP
	want := []string{"1", "WORK", "a", "b", "c", "1:00:00", "1:02:05", "-", "-", "-", "-", "-", "0.0%", "0", "-", "-"}
	if strings.Join(work, "|") != strings.Join(want, "|") {
		t.Fatalf("work row %q\nwant %q", work, want)
	}
	if rec[3] != "0:00" || rec[4] != "0:59" || rec[10] != "-" || rec[11] != "-" {
		t.Fatalf("recovery row %q", rec)
	}
}

// analysis workout reads GET /activity/{id}/intervals; the real API sends
// snake_case there too, which must reach the analysis (synthetic fixture).
//
//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisWorkoutReadsSnakeCaseIntervals(t *testing.T) {
	intervals, err := os.ReadFile("../../testdata/activity_intervals/analysis_snake_intervals.json")
	if err != nil {
		t.Fatal(err)
	}
	watts := "[" + strings.TrimRight(strings.Repeat("250,", 120), ",") + "]"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch p := r.URL.Path; {
		case strings.HasSuffix(p, "/activity/i970000010/intervals"):
			_, _ = w.Write(intervals)
		case strings.HasSuffix(p, "/activity/i970000010/streams"):
			_, _ = io.WriteString(w, `[{"type":"watts","data":`+watts+`}]`)
		case strings.HasSuffix(p, "/activity/i970000010"):
			_, _ = io.WriteString(w, `{"id":"i970000010","name":"Synthetic 2x18","type":"Ride",`+
				`"startDateLocal":"2030-03-05T18:00:00","movingTime":3000}`)
		case strings.Contains(p, "/events"):
			_, _ = io.WriteString(w, `[{"id":5,"category":"WORKOUT","type":"Ride","name":"Synthetic 2x18",`+
				`"startDateLocal":"2030-03-05T00:00:00","movingTime":3000,"workoutDoc":{"steps":[{"reps":2,`+
				`"steps":[{"duration":1080,"power":{"start":90,"end":94,"units":"%ftp"}}]}]}}]`)
		case strings.Contains(p, "/sport-settings"):
			_, _ = io.WriteString(w, sportJSON())
		default:
			_, _ = io.WriteString(w, okJSON)
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	out, err := captureStdout(t, func(stdout io.Writer) error {
		return analysisWorkoutCommand().Run([]string{"i970000010"}, map[string]string{"json": "true"}, client, stdout, io.Discard)
	})
	if err != nil {
		t.Fatal(err)
	}
	var got icu.WorkoutExecutionAnalysis
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Execution.IntervalsFound != 3 || got.Comparison.RepCount.Executed != 2 {
		t.Fatalf("intervalsFound=%d repCount=%+v", got.Execution.IntervalsFound, got.Comparison.RepCount)
	}
	var power []float64
	for _, step := range got.Comparison.StepComparison {
		if step.ExecutedAveragePower > 0 {
			power = append(power, step.ExecutedAveragePower)
		}
	}
	if len(power) != 2 || power[0] != 263 || power[1] != 259 {
		t.Fatalf("executed power from snake average_watts = %v (steps %+v)", power, got.Comparison.StepComparison)
	}
}

// Each control character is replaced on its own, so dropping any pair from the
// replacer fails here (a tab would otherwise become a tabwriter cell break).
func TestDashIfEmpty(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"": "-", "rep": "rep", "a\tb": "a b", "a\nb": "a b", "a\rb": "a b", "\t\n\r": "   "} {
		if got := dashIfEmpty(in); got != want {
			t.Errorf("dashIfEmpty(%q) = %q, want %q", in, got, want)
		}
	}
}

// The exact row: a tab left in the label would shift every later column.
func TestActivityIntervalsLabelRowExact(t *testing.T) {
	t.Parallel()
	client := bodyClient(t, `{"id":"i970000022","icu_intervals":[{"type":"WORK","label":"x\ty","moving_time":60,"average_watts":200}]}`)
	out := runIntervals(t, client, "i970000022", map[string]string{})
	lines := strings.Split(out, "\n")
	header, row := lines[1], lines[2]
	if !strings.HasPrefix(row, "1  WORK  x y    0:00   1:00      200") {
		t.Fatalf("row %q", row)
	}
	if strings.Index(header, "AVG W") != strings.Index(row, "200") {
		t.Fatalf("AVG W column misaligned:\n%s\n%s", header, row)
	}
}
