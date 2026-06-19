package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

const (
	microcycleTestWeekEnd   = "2026-06-14"
	microcycleTestWeekStart = "2026-06-08"
)

func TestDefaultAnalysisFieldsIncludeReportContractInputs(t *testing.T) {
	t.Parallel()

	required := []string{
		"icu_pm_cp",
		"icu_pm_w_prime",
		"icu_pm_p_max",
		"icu_pm_ftp",
		"icu_rolling_ftp",
		"average_temp",
		"min_temp",
		"max_temp",
		"average_weather_temp",
		"average_feels_like",
		"average_wind_speed",
		"headwind_percent",
		"tailwind_percent",
		"average_altitude",
		"average_gradient",
		"average_lactate",
		"strain_score",
	}

	var missing []string

	for _, field := range required {
		if !strings.Contains(defaultAnalysisFields, field) {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("defaultAnalysisFields missing %v", missing)
	}
}

func TestAnalysisDateRangeUsesExplicitDates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 29, 12, 0, 0, 0, time.UTC)
	got, explicit, err := analysisDateRange(map[string]string{
		"oldest": "2026-05-01",
		"newest": "2026-05-29",
	}, now)

	if err != nil || got.Oldest != "2026-05-01" || got.Newest != "2026-05-29" || !explicit {
		t.Fatalf("analysisDateRange explicit = %+v explicit=%v %v, want 2026-05-01 2026-05-29 true nil", got, explicit, err)
	}
}

func TestAnalysisDateRangeDefaultsToDays(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 29, 12, 0, 0, 0, time.UTC)
	got, explicit, err := analysisDateRange(map[string]string{"days": "7"}, now)

	if err != nil || got.Oldest != "2026-05-23" || got.Newest != "2026-05-29" || explicit {
		t.Fatalf("analysisDateRange days = %+v explicit=%v %v, want 2026-05-23 2026-05-29 false nil", got, explicit, err)
	}
}

func TestAnalysisDateRangeRejectsMissingPairAndInvalidDays(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 29, 12, 0, 0, 0, time.UTC)
	if _, _, err := analysisDateRange(map[string]string{"oldest": "2026-05-01"}, now); err == nil {
		t.Fatal("analysisDateRange missing newest error = nil, want error")
	}
	if _, _, err := analysisDateRange(map[string]string{"days": "0"}, now); err == nil {
		t.Fatal("analysisDateRange days=0 error = nil, want error")
	}
}

func TestTrainingPlanDateRangesDefaultToTwelveWeekHistoryAndNextISOBlock(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 30, 12, 0, 0, 0, time.UTC)
	got, explicit, err := trainingPlanDateRanges(map[string]string{}, now)
	want := trainingPlanRanges{
		History: analysisRange{Oldest: "2026-03-08", Newest: "2026-05-30"},
		Plan:    analysisRange{Oldest: "2026-06-01", Newest: "2026-06-28"},
	}

	if err != nil || got != want || explicit {
		t.Fatalf("trainingPlanDateRanges default = %+v explicit=%v %v, want %+v false nil", got, explicit, err, want)
	}
}

func TestTrainingPlanDateRangesUseExplicitDates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 30, 12, 0, 0, 0, time.UTC)
	got, explicit, err := trainingPlanDateRanges(map[string]string{
		"history-oldest": "2026-03-01",
		"history-newest": "2026-05-30",
		"plan-oldest":    "2026-06-01",
		"plan-newest":    "2026-06-28",
	}, now)
	want := trainingPlanRanges{
		History: analysisRange{Oldest: "2026-03-01", Newest: "2026-05-30"},
		Plan:    analysisRange{Oldest: "2026-06-01", Newest: "2026-06-28"},
	}

	if err != nil || got != want || !explicit {
		t.Fatalf("trainingPlanDateRanges explicit = %+v explicit=%v %v, want %+v true nil", got, explicit, err, want)
	}
}

func TestTrainingPlanDateRangesRejectInvalidInputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 30, 12, 0, 0, 0, time.UTC)
	cases := []map[string]string{
		{"history-oldest": "2026-03-01"},
		{"plan-oldest": "2026-06-01"},
		{"history-days": "0"},
		{"plan-days": "0"},
	}
	for _, flags := range cases {
		if _, _, err := trainingPlanDateRanges(flags, now); err == nil {
			t.Fatalf("trainingPlanDateRanges(%v) error = nil, want error", flags)
		}
	}
}

func TestMicrocycleDateRangeUsesExplicitRange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)
	got, _, err := microcycleDateRange(map[string]string{
		"from": "2026-06-08",
		"to":   "2026-06-14",
	}, now)

	if err != nil || got.Oldest != microcycleTestWeekStart || got.Newest != microcycleTestWeekEnd {
		t.Fatalf("microcycleDateRange explicit = %+v %v, want 2026-06-08 2026-06-14 nil", got, err)
	}
}

func TestMicrocycleDateRangeRejectsConflicts(t *testing.T) {
	t.Parallel()

	_, _, err := microcycleDateRange(map[string]string{
		"week": "2026-06-08",
		"from": "2026-06-08",
		"to":   "2026-06-14",
	}, time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC))

	if err == nil {
		t.Fatal("expected conflicting date selector error, got nil")
	}
}

func TestMicrocycleDateRangeRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)
	cases := []map[string]string{
		{"from": "2026-06-08"},
		{"from": "2026-06-15", "to": "2026-06-14"},
		{"date": "not-a-date"},
		{"week": "2026-06-08", "date": "2026-06-09"},
		{"timezone": "Invalid/Zone"},
	}

	for _, flags := range cases {
		_, _, err := microcycleDateRange(flags, now)
		if err == nil {
			t.Fatalf("microcycleDateRange(%v) error = nil, want error", flags)
		}
	}
}

func TestMicrocycleDateRangeSelectsDateWeek(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)
	got, _, err := microcycleDateRange(map[string]string{"date": "2026-06-12", "timezone": "UTC"}, now)
	if err != nil || got.Oldest != microcycleTestWeekStart || got.Newest != microcycleTestWeekEnd {
		t.Fatalf("microcycleDateRange date = %+v %v, want 2026-06-08 2026-06-14 nil", got, err)
	}
}

func TestMicrocycleSmallHelpers(t *testing.T) {
	t.Parallel()

	if got := microcycleTimezoneSource(map[string]string{"timezone": "UTC"}); got != "flag" {
		t.Fatalf("timezone source = %s, want flag", got)
	}
	if _, err := addDays("bad-date", 1); err == nil {
		t.Fatal("addDays error = nil, want error")
	}
	if err := validateDateRange("bad", "2026-06-14"); err == nil {
		t.Fatal("validateDateRange bad from error = nil, want error")
	}
	if err := validateDateRange("2026-06-08", "bad"); err == nil {
		t.Fatal("validateDateRange bad to error = nil, want error")
	}
}

func TestMicrocycleDateRangeDefaultsToISOWeek(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)
	got, _, err := microcycleDateRange(map[string]string{}, now)

	if err != nil || got.Oldest != microcycleTestWeekStart || got.Newest != microcycleTestWeekEnd {
		t.Fatalf("microcycleDateRange default = %+v %v, want 2026-06-08 2026-06-14 nil", got, err)
	}
}

func TestAnalysisMicroCommandRegistered(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAnalysisCommands(registry)

	cmd, ok := registry.Lookup("analysis", "micro")
	if !ok {
		t.Fatal("analysis micro not found")
	}
	if cmd == nil {
		t.Fatal("analysis micro command is nil")
	}
}

func TestAnalysisMicrocycleCommandRegistered(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAnalysisCommands(registry)

	cmd, ok := registry.Lookup("analysis", "microcycle")
	if !ok {
		t.Fatal("analysis microcycle not found")
	}
	if cmd == nil {
		t.Fatal("analysis microcycle command is nil")
	}
}

func TestAnalysisWorkoutCommandRegistered(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAnalysisCommands(registry)

	cmd, ok := registry.Lookup("analysis", "workout")
	if !ok {
		t.Fatal("analysis workout not found")
	}
	if cmd == nil {
		t.Fatal("analysis workout command is nil")
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisWorkoutCommandWritesJSON(t *testing.T) {
	streamWatts := "[" + strings.TrimRight(strings.Repeat("250,", 120), ",") + "]"
	streamHR := "[" + strings.TrimRight(strings.Repeat("150,", 120), ",") + "]"
	activityBody := `{"id":"a1","name":"3x18 Sweet Spot","type":"Ride",` +
		`"startDateLocal":"2026-06-18T18:09:46","movingTime":6743,` +
		`"icuTrainingLoad":106,"icuIntensity":0.751,"icuWeightedAvgWatts":214,"averageHeartrate":136}`
	eventBody := `[{"id":2,"category":"WORKOUT","type":"Ride",` +
		`"name":"W3 Thu 2026-06-18 ALT 3x18 Sweet Spot",` +
		`"startDateLocal":"2026-06-18T00:00:00","movingTime":6420,` +
		`"icuTrainingLoad":118,"icuIntensity":0.813,` +
		`"workoutDoc":{"steps":[{"reps":1,"steps":[{"duration":1080,` +
		`"power":{"start":90,"end":94,"units":"%ftp"}}]}]}}]`
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/activity/a1/intervals"):
			_, _ = response.Write([]byte(`{"icuIntervals":[{"startIndex":0,"endIndex":1080,"type":"WORK","movingTime":1080,"averageWatts":258,"averageHeartrate":158}]}`))
		case strings.Contains(request.URL.Path, "/activity/a1/streams"):
			_, _ = response.Write([]byte(`[{"type":"watts","data":` + streamWatts + `},{"type":"heartrate","data":` + streamHR + `}]`))
		case strings.Contains(request.URL.Path, "/activity/a1"):
			_, _ = response.Write([]byte(activityBody))
		case strings.Contains(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(eventBody))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient(
		"test-key",
		"0",
		icu.WithHTTPClient(server.Client()),
		icu.WithBaseURL(server.URL),
	)
	cmd := analysisWorkoutCommand()
	out, err := captureStdout(t, func() error {
		return cmd.Run([]string{"a1"}, map[string]string{"json": "true"}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	var got icu.WorkoutExecutionAnalysis
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Match.EventID != 2 {
		t.Fatalf("json = %q err=%v match=%+v", out, err, got.Match)
	}
}

func newMicrocycleCommandTestClient(serverURL string, httpClient *http.Client) *icu.Client {
	return icu.NewClient(
		"test-key",
		"0",
		icu.WithHTTPClient(httpClient),
		icu.WithBaseURL(serverURL),
	)
}

func runMicrocycleCommandJSONTest(
	t *testing.T,
	flags map[string]string,
	handler http.HandlerFunc,
) string {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := newMicrocycleCommandTestClient(server.URL, server.Client())
	cmd := analysisMicrocycleCommand()
	out, err := captureStdout(t, func() error {
		return cmd.Run(nil, flags, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	return out
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisMicrocycleCommandWritesJSON(t *testing.T) {
	cases := []struct {
		name  string
		flags map[string]string
	}{
		{
			name: "json flag",
			flags: map[string]string{
				"from":     "2026-06-01",
				"to":       "2026-06-07",
				"json":     "true",
				"timezone": "UTC",
			},
		},
		{
			name: "output flag",
			flags: map[string]string{
				"from":     "2026-06-01",
				"to":       "2026-06-07",
				"output":   "json",
				"timezone": "UTC",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runMicrocycleCommandJSONTest(t, tc.flags, func(response http.ResponseWriter, request *http.Request) {
				response.Header().Set("Content-Type", "application/json")
				switch {
				case strings.Contains(request.URL.Path, "/activities"):
					_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
				case strings.Contains(request.URL.Path, "/events"):
					_, _ = response.Write([]byte(`[` + eventJSON() + `]`))
				case strings.Contains(request.URL.Path, "/wellness"):
					_, _ = response.Write([]byte(`[` + wellnessJSON() + `]`))
				case strings.Contains(request.URL.Path, "/sport-settings"):
					_, _ = response.Write([]byte(sportJSON()))
				default:
					_, _ = response.Write([]byte(okJSON))
				}
			})

			var got icu.MicrocycleAnalysis
			if err := json.Unmarshal([]byte(out), &got); err != nil || got.Command != "analysis microcycle" {
				t.Fatalf("json = %q err=%v command=%q", out, err, got.Command)
			}
		})
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisMicrocycleCommandSkipsPlanAndWellness(t *testing.T) {
	requested := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requested[request.URL.Path]++
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient(
		"test-key",
		"0",
		icu.WithHTTPClient(server.Client()),
		icu.WithBaseURL(server.URL),
	)
	cmd := analysisMicrocycleCommand()
	out, err := captureStdout(t, func() error {
		return cmd.Run(nil, map[string]string{
			"from":        "2026-06-01",
			"to":          "2026-06-07",
			"json":        "true",
			"timezone":    "UTC",
			"no-plan":     "true",
			"no-wellness": "true",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if strings.Contains(out, `"plan": "api.events"`) {
		t.Fatalf("output = %s, want plan excluded", out)
	}
	for path := range requested {
		if strings.Contains(path, "/events") || strings.Contains(path, "/wellness") {
			t.Fatalf("unexpected request path %s", path)
		}
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisMicrocycleCommandWritesHumanOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient(
		"test-key",
		"0",
		icu.WithHTTPClient(server.Client()),
		icu.WithBaseURL(server.URL),
	)
	cmd := analysisMicrocycleCommand()
	out, err := captureStdout(t, func() error {
		return cmd.Run(nil, map[string]string{
			"from":        "2026-06-01",
			"to":          "2026-06-07",
			"output":      "table",
			"timezone":    "UTC",
			"no-plan":     "true",
			"no-wellness": "true",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out, "Microcycle: 2026-06-01 to 2026-06-07") {
		t.Fatalf("human output = %q, want microcycle header", out)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisMicrocycleCommandAllowsMissingSportSettings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
		case strings.Contains(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[` + eventJSON() + `]`))
		case strings.Contains(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[` + wellnessJSON() + `]`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			response.WriteHeader(http.StatusNotFound)
			_, _ = response.Write([]byte(`{"error":"missing"}`))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient(
		"test-key",
		"0",
		icu.WithHTTPClient(server.Client()),
		icu.WithBaseURL(server.URL),
	)
	cmd := analysisMicrocycleCommand()
	out, err := captureStdout(t, func() error {
		return cmd.Run(nil, map[string]string{
			"from":     "2026-06-01",
			"to":       "2026-06-07",
			"json":     "true",
			"timezone": "UTC",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out, `"zones": "missing"`) {
		t.Fatalf("json = %q, want missing zones warning", out)
	}
}

func TestReadMicrocycleInputsReturnsSourceErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		failPath    string
		includePlan bool
		includeWell bool
	}{
		{name: "events", failPath: "/events", includePlan: true, includeWell: false},
		{name: "wellness", failPath: "/wellness", includePlan: false, includeWell: true},
		{name: "sport", failPath: "/sport-settings", includePlan: false, includeWell: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if strings.Contains(request.URL.Path, tc.failPath) {
					response.WriteHeader(http.StatusInternalServerError)
					_, _ = response.Write([]byte(`{"error":"failed"}`))

					return
				}

				response.Header().Set("Content-Type", "application/json")
				switch {
				case strings.Contains(request.URL.Path, "/activities"):
					_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
				case strings.Contains(request.URL.Path, "/events"):
					_, _ = response.Write([]byte(`[` + eventJSON() + `]`))
				case strings.Contains(request.URL.Path, "/wellness"):
					_, _ = response.Write([]byte(`[` + wellnessJSON() + `]`))
				case strings.Contains(request.URL.Path, "/sport-settings"):
					_, _ = response.Write([]byte(sportJSON()))
				default:
					_, _ = response.Write([]byte(okJSON))
				}
			}))
			t.Cleanup(server.Close)

			client := icu.NewClient(
				"test-key",
				"0",
				icu.WithHTTPClient(server.Client()),
				icu.WithBaseURL(server.URL),
			)
			_, err := readMicrocycleInputs(
				client,
				map[string]string{"timezone": "UTC"},
				analysisRange{Oldest: "2026-06-01", Newest: "2026-06-07"},
				"2026-03-03",
				tc.includePlan,
				tc.includeWell,
				"UTC",
				"flag",
			)
			if err == nil {
				t.Fatal("readMicrocycleInputs error = nil, want source error")
			}
		})
	}
}

func TestReadMicrocycleInputsAllowsMissingSportSettings(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "/sport-settings") {
			response.WriteHeader(http.StatusNotFound)
			_, _ = response.Write([]byte(`{"error":"missing"}`))

			return
		}

		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
		case strings.Contains(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[` + eventJSON() + `]`))
		case strings.Contains(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[` + wellnessJSON() + `]`))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient(
		"test-key",
		"0",
		icu.WithHTTPClient(server.Client()),
		icu.WithBaseURL(server.URL),
	)
	_, err := readMicrocycleInputs(
		client,
		map[string]string{"timezone": "UTC"},
		analysisRange{Oldest: "2026-06-01", Newest: "2026-06-07"},
		"2026-03-03",
		false,
		false,
		"UTC",
		"flag",
	)
	if err != nil {
		t.Fatalf("readMicrocycleInputs error = %v, want nil on 404 sport-settings", err)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestWriteMicrocycleHuman(t *testing.T) {
	analysis := icu.MicrocycleAnalysis{
		Microcycle: icu.MicrocycleScope{
			StartDate:      "2026-06-08",
			EndDate:        "2026-06-14",
			Timezone:       "UTC",
			TimezoneSource: "flag",
			ElapsedDays:    7,
		},
		Classification: icu.MicrocycleAdaptationSignal{
			Value:              "on_track",
			MainPositiveSignal: "load recorded",
			MainRisk:           "no major risk",
		},
		Confidence: "medium",
		Load: icu.MicrocycleLoad{
			ActivityCount: 2,
			TSS:           120,
		},
		Intensity: icu.MicrocycleIntensity{Z4PlusSessions: 1},
	}

	out, err := captureStdout(t, func() error {
		return writeMicrocycleHuman(&analysis)
	})
	if err != nil {
		t.Fatalf("writeMicrocycleHuman error: %v", err)
	}
	if !strings.Contains(out, "Classification: on_track") {
		t.Fatalf("human output = %q, want classification", out)
	}
}
