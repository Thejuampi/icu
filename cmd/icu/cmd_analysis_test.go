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
	analysisTestAthletePath = "/api/v1/athlete/i123"
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

func TestAnalysisDateRangeRejectsMalformedInvertedAndConflictingInputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 29, 12, 0, 0, 0, time.UTC)
	tests := []map[string]string{
		{"oldest": "2026-02-30", "newest": "2026-03-01"},
		{"oldest": "2026-02-01", "newest": "2026-02-30"},
		{"oldest": "2026-05-30", "newest": "2026-05-29"},
		{"oldest": "2026-05-01", "newest": "2026-05-29", "days": "7"},
		{"days": "abc"},
		{"days": "-1"},
	}

	for _, flags := range tests {
		if _, _, err := analysisDateRange(flags, now); err == nil {
			t.Fatalf("analysisDateRange(%v) error = nil, want error", flags)
		}
	}
}

func TestAnalysisCommandsRejectInvalidDayFlagsBeforeHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command *Command
		flags   map[string]string
	}{
		{name: "coaching", command: analysisCoachingCommand(), flags: map[string]string{"history-days": "abc"}},
		{name: "cycling", command: analysisCyclingCommand(), flags: map[string]string{"days": "abc"}},
		{name: "wellness", command: analysisWellnessCommand(), flags: map[string]string{"days": "abc"}},
		{name: "plan", command: analysisPlanCommand(), flags: map[string]string{"plan-days": "abc"}},
		{name: "adaptation", command: analysisAdaptationCommand(), flags: map[string]string{"days": "abc"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := test.command.Run(nil, test.flags, nil); err == nil {
				t.Fatal("Run error = nil, want invalid day error before client use")
			}
		})
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

func TestTrainingPlanDateRangesAlignExplicitPlanDatesToISOWeek(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 30, 12, 0, 0, 0, time.UTC)
	got, explicit, err := trainingPlanDateRanges(map[string]string{
		"history-oldest": "2026-03-01",
		"history-newest": "2026-05-30",
		"plan-oldest":    "2026-06-20",
		"plan-newest":    "2026-07-17",
	}, now)
	want := trainingPlanRanges{
		History: analysisRange{Oldest: "2026-03-01", Newest: "2026-05-30"},
		Plan:    analysisRange{Oldest: "2026-06-15", Newest: "2026-07-19"},
	}

	if err != nil || got != want || !explicit {
		t.Fatalf("trainingPlanDateRanges aligned = %+v explicit=%v %v, want %+v true nil", got, explicit, err, want)
	}
}

func TestTrainingPlanDateRangesHandleSundayBoundaries(t *testing.T) {
	t.Parallel()
	const planMonday = "2026-06-01"

	now := time.Date(2026, time.May, 31, 12, 0, 0, 0, time.UTC)
	got, explicit, err := trainingPlanDateRanges(map[string]string{
		"plan-oldest": "2026-06-07",
		"plan-newest": "2026-06-14",
	}, now)
	if err != nil || !explicit || got.Plan.Oldest != planMonday || got.Plan.Newest != "2026-06-14" {
		t.Fatalf("trainingPlanDateRanges = %+v explicit %v error %v, want 2026-06-01..2026-06-14 true nil", got, explicit, err)
	}

	defaults, defaultExplicit, err := trainingPlanDateRanges(map[string]string{}, now)
	if err != nil || defaultExplicit || defaults.Plan.Oldest != planMonday {
		t.Fatalf("default trainingPlanDateRanges = %+v explicit %v error %v, want next Monday", defaults, defaultExplicit, err)
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

func TestTrainingPlanDateRangesRejectMalformedInvertedAndConflictingInputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 29, 12, 0, 0, 0, time.UTC)
	tests := []map[string]string{
		{"history-oldest": "2026-02-30", "history-newest": "2026-03-01"},
		{"history-oldest": "2026-05-30", "history-newest": "2026-05-29"},
		{"history-oldest": "2026-05-01", "history-newest": "2026-05-29", "history-days": "7"},
		{"history-days": "abc"},
		{"history-days": "-1"},
		{"plan-oldest": "2026-07-05", "plan-newest": "2026-06-29"},
		{"plan-oldest": "2026-07-03", "plan-newest": "2026-06-30"},
		{"plan-oldest": "2026-06-29", "plan-newest": "2026-07-26", "plan-days": "28"},
		{"plan-days": "abc"},
		{"plan-days": "-1"},
	}

	for _, flags := range tests {
		if _, _, err := trainingPlanDateRanges(flags, now); err == nil {
			t.Fatalf("trainingPlanDateRanges(%v) error = nil, want error", flags)
		}
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisPlanCommandUsesValidatedAlignedRanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		default:
			_, _ = response.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient("test-key", "i123", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	output, err := captureStdout(t, func() error {
		return analysisPlanCommand().Run(nil, map[string]string{
			"history-oldest": "2026-04-06",
			"history-newest": "2026-06-28",
			"plan-oldest":    "2026-07-01",
			"plan-newest":    "2026-07-24",
			"resolve":        "false",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}

	var analysis icu.TrainingPlanAnalysis
	if err := json.Unmarshal([]byte(output), &analysis); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if analysis.Scope.PlanStartDate != "2026-06-29" || analysis.Scope.PlanEndDate != "2026-07-26" {
		t.Fatalf("plan scope = %s..%s, want aligned 2026-06-29..2026-07-26", analysis.Scope.PlanStartDate, analysis.Scope.PlanEndDate)
	}
}

func TestAnalysisPlanCommandReportsUpstreamFailures(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v1/athlete/i123/activities",
		"/api/v1/athlete/i123/sport-settings/Ride",
		"/api/v1/athlete/i123/wellness",
		"/api/v1/athlete/i123/events",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			client := newAnalysisFailureClient(t, path)
			err := analysisPlanCommand().Run(nil, map[string]string{
				"history-oldest": "2026-04-06",
				"history-newest": "2026-06-28",
				"plan-oldest":    "2026-06-29",
				"plan-newest":    "2026-07-26",
			}, client)
			if err == nil {
				t.Fatal("Run error = nil, want upstream failure")
			}
		})
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisWellnessCommandPrefersZeppHybridCharge(t *testing.T) {
	t.Setenv("ZEPP_LOGIN_TOKEN", "tok")
	t.Setenv("ZEPP_APP_TOKEN", "app")
	t.Setenv("ZEPP_USER_ID", "u1")

	zeppServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path != "/v2/users/me/events" {
			http.NotFound(response, request)

			return
		}
		if !strings.Contains(request.URL.RawQuery, "eventType=Charge") || !strings.Contains(request.URL.RawQuery, "subType=insight_data") {
			http.Error(response, "missing hybridcharge preset", http.StatusBadRequest)

			return
		}

		_, _ = response.Write([]byte(`{"items":[{"timestamp":1780272000000,"value":90},{"timestamp":1780358400000,"value":88}]}`))
	}))
	t.Cleanup(zeppServer.Close)
	t.Setenv("ZEPP_EVENTS_URL", zeppServer.URL)

	intervalsServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[{"id":"2026-06-01","sleepScore":60,"restingHr":50},{"id":"2026-06-02","sleepScore":55,"restingHr":49}]`))
		default:
			_, _ = response.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(intervalsServer.Close)

	client := icu.NewClient("test-key", "i123", icu.WithHTTPClient(intervalsServer.Client()), icu.WithBaseURL(intervalsServer.URL))
	output, err := captureStdout(t, func() error {
		return analysisWellnessCommand().Run(nil, map[string]string{
			"oldest": "2026-06-01",
			"newest": "2026-06-02",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}

	var got icu.WellnessAnalysis
	err = json.Unmarshal([]byte(output), &got)
	if err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if got.Sleep.ScoreName != "zepp_hybridcharge" || got.Sleep.Latest != 88 || got.State.State != "OK" {
		t.Fatalf("Sleep = %+v State = %+v, want zepp_hybridcharge latest 88 state OK", got.Sleep, got.State)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisCoachingCommandPropagatesPreferredWellnessScore(t *testing.T) {
	withHybridChargeZeppTestServer(t, `{"items":[{"timestamp":1780272000000,"value":90},{"timestamp":1780358400000,"value":88}]}`)

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == analysisTestAthletePath:
			_, _ = response.Write([]byte(`{"id":"i123","name":"Rider"}`))
		case strings.Contains(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[]`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		case strings.Contains(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[{"id":"2026-06-01","sleepScore":60,"restingHr":50},{"id":"2026-06-02","sleepScore":55,"restingHr":49}]`))
		case strings.Contains(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[]`))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient("test-key", "i123", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	out, err := captureStdout(t, func() error {
		return analysisCoachingCommand().Run(nil, map[string]string{
			"history-oldest": "2026-06-01",
			"history-newest": "2026-06-02",
			"plan-oldest":    "2026-06-01",
			"plan-newest":    "2026-06-07",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	var got icu.CoachingContext
	err = json.Unmarshal([]byte(out), &got)
	if err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if got.Analyses.Wellness == nil || got.Analyses.Wellness.Sleep.ScoreName != "zepp_hybridcharge" || got.Analyses.Wellness.State.State != "OK" {
		t.Fatalf("wellness = %+v, want zepp_hybridcharge state OK", got.Analyses.Wellness)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisPlanCommandPrefersHybridChargeForWellnessRules(t *testing.T) {
	withHybridChargeZeppTestServer(t, `{"items":[{"timestamp":1780185600000,"value":92},{"timestamp":1780272000000,"value":90}]}`)

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[]`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		case strings.Contains(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[{"id":"2026-05-30","sleepScore":60,"restingHr":45},{"id":"2026-05-31","sleepScore":55,"restingHr":45}]`))
		case strings.Contains(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[{"id":1,"category":"WORKOUT","type":"Ride","name":"VO2 Key","startDateLocal":"2026-06-01T08:00:00","icuTrainingLoad":110,"movingTime":3600,"icuIntensity":0.9}]`))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient("test-key", "i123", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	out, err := captureStdout(t, func() error {
		return analysisPlanCommand().Run(nil, map[string]string{
			"history-oldest": "2026-05-30",
			"history-newest": "2026-05-31",
			"plan-oldest":    "2026-06-01",
			"plan-newest":    "2026-06-01",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	var got icu.TrainingPlanAnalysis
	err = json.Unmarshal([]byte(out), &got)
	if err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if len(got.PlannedSessions) != 1 || !got.PlannedSessions[0].KeySession || hasAdjustmentCondition(got.DayAdjustments, "sleep_score_watch") || hasAdjustmentCondition(got.DayAdjustments, "sleep_score_red") || hasAdjustmentCondition(got.DayAdjustments, "wellness_state_watch") || hasAdjustmentCondition(got.DayAdjustments, "wellness_state_red") {
		t.Fatalf("plannedSessions/dayAdjustments = %+v / %+v, want key session without sleep or wellness fallback gates", got.PlannedSessions, got.DayAdjustments)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisMicrocycleCommandPrefersHybridChargeInWellnessSummary(t *testing.T) {
	withHybridChargeZeppTestServer(t, `{"items":[{"timestamp":1780272000000,"value":90},{"timestamp":1780358400000,"value":88}]}`)

	out := runMicrocycleCommandJSONTest(t, map[string]string{
		"from":     "2026-06-01",
		"to":       "2026-06-07",
		"json":     "true",
		"timezone": "UTC",
	}, func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
		case strings.Contains(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[` + eventJSON() + `]`))
		case strings.Contains(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[{"id":"2026-06-01","sleepScore":60,"restingHr":50},{"id":"2026-06-02","sleepScore":55,"restingHr":49}]`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	})

	var got icu.MicrocycleAnalysis
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if got.Wellness.State.State != "OK" || !containsString(got.Wellness.PositiveSignals, "sleep_score_ok") {
		t.Fatalf("wellness = %+v, want OK state with sleep_score_ok signal", got.Wellness)
	}
}

func TestAnalysisAdaptationCommandReportsUpstreamFailures(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v1/athlete/i123/activities",
		"/api/v1/athlete/i123/power-curves",
		"/api/v1/athlete/i123/mmp-model",
		"/api/v1/athlete/i123/sport-settings/Ride",
		"/api/v1/athlete/i123/wellness",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			client := newAnalysisFailureClient(t, path)
			err := analysisAdaptationCommand().Run(nil, map[string]string{
				"oldest": "2026-04-06",
				"newest": "2026-06-28",
			}, client)
			if err == nil {
				t.Fatal("Run error = nil, want upstream failure")
			}
		})
	}
}

func newAnalysisFailureClient(t *testing.T, failedPath string) *icu.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == failedPath {
			http.Error(response, "upstream failure", http.StatusInternalServerError)

			return
		}

		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		case strings.Contains(request.URL.Path, "/power-curves"):
			_, _ = response.Write([]byte(`{"list":[]}`))
		case strings.Contains(request.URL.Path, "/mmp-model"):
			_, _ = response.Write([]byte(`{}`))
		default:
			_, _ = response.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)

	return icu.NewClient("test-key", "i123", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
}

func TestAnalysisCoachingCommandRegistered(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAnalysisCommands(registry)

	cmd, ok := registry.Lookup("analysis", "coaching")
	if !ok || cmd == nil {
		t.Fatal("analysis coaching not found")
	}
}

func TestAnalysisCoachingCommandReportsUpstreamFailure(t *testing.T) {
	t.Parallel()

	client := newAnalysisFailureClient(t, analysisTestAthletePath)
	err := analysisCoachingCommand().Run(nil, map[string]string{
		"history-oldest": "2026-04-06",
		"history-newest": "2026-06-28",
		"plan-oldest":    "2026-06-29",
		"plan-newest":    "2026-07-26",
	}, client)
	if err == nil {
		t.Fatal("Run error = nil, want upstream failure")
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisCoachingCommandCombinesContext(t *testing.T) {
	requested := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requested[request.URL.Path]++
		response.Header().Set("Content-Type", "application/json")

		switch {
		case request.URL.Path == analysisTestAthletePath:
			_, _ = response.Write([]byte(`{"id":"i123","name":"Rider","timezone":"Europe/Madrid"}`))
		case strings.Contains(request.URL.Path, "/activities"):
			if request.URL.Query().Get("oldest") != "2026-04-06" {
				t.Fatalf("activities oldest = %s, want 2026-04-06", request.URL.Query().Get("oldest"))
			}
			_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		case strings.Contains(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[` + wellnessJSON() + `]`))
		case strings.Contains(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[` + eventJSON() + `,{"id":2,"category":"NOTE","name":"Travel","description":"late flight","startDateLocal":"2026-07-01T00:00:00"}]`))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient(
		"test-key",
		"i123",
		icu.WithHTTPClient(server.Client()),
		icu.WithBaseURL(server.URL),
	)
	cmd := analysisCoachingCommand()
	out, err := captureStdout(t, func() error {
		return cmd.Run(nil, map[string]string{
			"history-oldest": "2026-04-06",
			"history-newest": "2026-06-28",
			"plan-oldest":    "2026-06-29",
			"plan-newest":    "2026-07-26",
			"sport-type":     "Ride",
			"resolve":        "true",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	var got icu.CoachingContext
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json = %q err=%v", out, err)
	}
	if got.Scope.Command != "analysis coaching" || len(got.Calendar.Notes) != 1 || got.Analyses.Plan == nil {
		t.Fatalf("context = %+v, want coaching command with note and plan analysis", got)
	}
	for _, want := range []string{analysisTestAthletePath, "/api/v1/athlete/i123/activities", "/api/v1/athlete/i123/sport-settings/Ride", "/api/v1/athlete/i123/wellness", "/api/v1/athlete/i123/events"} {
		if requested[want] == 0 {
			t.Fatalf("request %s count = 0, want fetched", want)
		}
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisCoachingCommandIncludesAdaptationWhenRequested(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == analysisTestAthletePath:
			_, _ = response.Write([]byte(`{"id":"i123","name":"Rider"}`))
		case strings.Contains(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		case strings.Contains(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[` + wellnessJSON() + `]`))
		case strings.Contains(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[` + eventJSON() + `]`))
		case strings.Contains(request.URL.Path, "/power-curves"):
			_, _ = response.Write([]byte(`{"list":[{"id":"current","label":"42d","days":42,"secs":[60],"values":[500]},{"id":"baseline","label":"365d","days":365,"secs":[60],"values":[480]}]}`))
		case strings.Contains(request.URL.Path, "/mmp-model"):
			_, _ = response.Write([]byte(`{"criticalPower":285,"wPrime":21000,"ftp":285}`))
		default:
			_, _ = response.Write([]byte(okJSON))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient(
		"test-key",
		"i123",
		icu.WithHTTPClient(server.Client()),
		icu.WithBaseURL(server.URL),
	)
	cmd := analysisCoachingCommand()
	out, err := captureStdout(t, func() error {
		return cmd.Run(nil, map[string]string{
			"history-oldest":     "2026-04-06",
			"history-newest":     "2026-06-28",
			"plan-oldest":        "2026-06-29",
			"plan-newest":        "2026-07-26",
			"include-adaptation": "true",
			"adaptation-curves":  "42d,365d",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	var got icu.CoachingContext
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json = %q err=%v", out, err)
	}
	if got.Analyses.Adaptation == nil {
		t.Fatal("adaptation = nil, want included")
	}
}

func TestReadCoachingContextInputsReportsEachUpstreamFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		failedPath        string
		includeAdaptation bool
	}{
		{name: "athlete", failedPath: analysisTestAthletePath},
		{name: "sport settings", failedPath: "/api/v1/athlete/i123/sport-settings/Ride"},
		{name: "activities", failedPath: "/api/v1/athlete/i123/activities"},
		{name: "wellness", failedPath: "/api/v1/athlete/i123/wellness"},
		{name: "events", failedPath: "/api/v1/athlete/i123/events"},
		{name: "power curves", failedPath: "/api/v1/athlete/i123/power-curves", includeAdaptation: true},
		{name: "mmp model", failedPath: "/api/v1/athlete/i123/mmp-model", includeAdaptation: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path == test.failedPath {
					http.Error(response, "upstream failure", http.StatusInternalServerError)

					return
				}

				response.Header().Set("Content-Type", "application/json")
				switch {
				case request.URL.Path == analysisTestAthletePath:
					_, _ = response.Write([]byte(`{"id":"i123"}`))
				case strings.Contains(request.URL.Path, "/sport-settings"):
					_, _ = response.Write([]byte(sportJSON()))
				case strings.Contains(request.URL.Path, "/power-curves"):
					_, _ = response.Write([]byte(`{"list":[]}`))
				case strings.Contains(request.URL.Path, "/mmp-model"):
					_, _ = response.Write([]byte(`{}`))
				default:
					_, _ = response.Write([]byte(`[]`))
				}
			}))
			t.Cleanup(server.Close)

			client := icu.NewClient("test-key", "i123", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
			flags := map[string]string{}
			if test.includeAdaptation {
				flags["include-adaptation"] = "true"
			}
			_, err := readCoachingContextInputs(client, flags, trainingPlanRanges{
				History: analysisRange{Oldest: "2026-04-06", Newest: "2026-06-28"},
				Plan:    analysisRange{Oldest: "2026-06-29", Newest: "2026-07-26"},
			}, true)
			if err == nil {
				t.Fatal("readCoachingContextInputs error = nil, want upstream error")
			}
		})
	}
}

func TestReadCoachingContextInputsAllowsMissingSportSettings(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == analysisTestAthletePath:
			_, _ = response.Write([]byte(`{"id":"i123"}`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			http.Error(response, "missing", http.StatusNotFound)
		default:
			_, _ = response.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient("test-key", "i123", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	inputs, err := readCoachingContextInputs(client, map[string]string{}, trainingPlanRanges{
		History: analysisRange{Oldest: "2026-04-06", Newest: "2026-06-28"},
		Plan:    analysisRange{Oldest: "2026-06-29", Newest: "2026-07-26"},
	}, true)
	if err != nil || inputs.SportSettings != nil {
		t.Fatalf("readCoachingContextInputs = sportSettings %+v error %v, want absent settings and no error", inputs.SportSettings, err)
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

func TestAnalysisAdaptationCommandRegistered(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAnalysisCommands(registry)

	cmd, ok := registry.Lookup("analysis", "adaptation")
	if !ok {
		t.Fatal("analysis adaptation not found")
	}
	if cmd == nil {
		t.Fatal("analysis adaptation command is nil")
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

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestAnalysisWorkoutCommandRecalculatesWBalWithGlobalModel(t *testing.T) {
	streamWatts := "[" + strings.TrimRight(strings.Repeat("195,", 3600), ",") + "]"
	streamHR := "[" + strings.TrimRight(strings.Repeat("135,", 3600), ",") + "]"
	streamCad := "[" + strings.TrimRight(strings.Repeat("85,", 3600), ",") + "]"
	activityBody := `{"id":"a2","name":"2h Z2","type":"Ride",` +
		`"startDateLocal":"2026-06-21T08:00:00","movingTime":7200,` +
		`"icuTrainingLoad":92,"icuIntensity":0.625,` +
		`"icuWeightedAvgWatts":178,"averageHeartrate":133,` +
		`"icuFtp":285,"icuPmCp":172,"icuPmWPrime":14577,` +
		`"icuMaxWbalDepletion":5728}`
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/activity/a2/intervals"):
			_, _ = response.Write([]byte(`{}`))
		case strings.Contains(request.URL.Path, "/activity/a2/streams"):
			_, _ = response.Write([]byte(`[{"type":"watts","data":` + streamWatts + `},{"type":"heartrate","data":` + streamHR + `},{"type":"cadence","data":` + streamCad + `}]`))
		case strings.Contains(request.URL.Path, "/activity/a2"):
			_, _ = response.Write([]byte(activityBody))
		case strings.Contains(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[]`))
		case strings.Contains(request.URL.Path, "/sport-settings"):
			_, _ = response.Write([]byte(sportJSON()))
		case strings.Contains(request.URL.Path, "/mmp-model"):
			_, _ = response.Write([]byte(`{"type":"Ride","criticalPower":287,"wPrime":21000,"pMax":1315,"ftp":293}`))
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
		return cmd.Run([]string{"a2"}, map[string]string{"json": "true"}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	var got icu.WorkoutExecutionAnalysis
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json unmarshal error: %v\noutput: %s", err, out)
	}

	assertWBalRecalculatedWithGlobalModel(t, &got)
}

func assertWBalRecalculatedWithGlobalModel(t *testing.T, got *icu.WorkoutExecutionAnalysis) {
	t.Helper()

	if got.WBal == nil {
		t.Fatal("expected WBal section in output, got nil")
	}
	if got.WBal.ModelSource != "global" {
		t.Fatalf("modelSource = %s, want global", got.WBal.ModelSource)
	}
	if got.WBal.CriticalPower != 287 {
		t.Fatalf("criticalPower = %d, want 287", got.WBal.CriticalPower)
	}
	if got.WBal.OriginalDepletionPct < 30 {
		t.Fatalf("original depletion pct = %f, should be high from unreliable activity model", got.WBal.OriginalDepletionPct)
	}
	if got.WBal.RecomputedDepletionPct > 5 {
		t.Fatalf("recomputed depletion pct = %f, want <= 5 for Z2 with global model (CP=287)", got.WBal.RecomputedDepletionPct)
	}
	if got.WBal.RecomputedDepletionPct >= got.WBal.OriginalDepletionPct {
		t.Fatalf("recomputed (%f) should be < original (%f) when activity model is unreliable",
			got.WBal.RecomputedDepletionPct, got.WBal.OriginalDepletionPct)
	}

	foundUnreliableWarning := false
	for _, w := range got.WBal.DataQualityWarnings {
		if strings.Contains(w, "unreliable") {
			foundUnreliableWarning = true
		}
	}
	if !foundUnreliableWarning {
		t.Fatalf("expected unreliable model warning, got %v", got.WBal.DataQualityWarnings)
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
func TestAnalysisMicrocycleCommandForwardsCalendarID(t *testing.T) {
	var eventsQuery string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/events"):
			eventsQuery = request.URL.RawQuery
			_, _ = response.Write([]byte(`[` + eventJSON() + `]`))
		case strings.Contains(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[` + activityJSON() + `]`))
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
	cmd := analysisMicrocycleCommand()
	_, err := captureStdout(t, func() error {
		return cmd.Run(nil, map[string]string{
			"from":        "2026-06-01",
			"to":          "2026-06-07",
			"json":        "true",
			"timezone":    "UTC",
			"calendar-id": "42",
			"no-wellness": "true",
		}, client)
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	// queryFromFlags maps calendar-id → calendar_id for the Intervals API.
	if !strings.Contains(eventsQuery, "calendar_id=42") {
		t.Fatalf("events query = %q, want calendar_id=42 (pre-fix used wrong flag key)", eventsQuery)
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

func withHybridChargeZeppTestServer(t *testing.T, payload string) {
	t.Helper()
	t.Setenv("ZEPP_LOGIN_TOKEN", "tok")
	t.Setenv("ZEPP_APP_TOKEN", "app")
	t.Setenv("ZEPP_USER_ID", "u1")

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path != "/v2/users/me/events" {
			http.NotFound(response, request)

			return
		}
		if !strings.Contains(request.URL.RawQuery, "eventType=Charge") || !strings.Contains(request.URL.RawQuery, "subType=insight_data") {
			http.Error(response, "missing hybridcharge preset", http.StatusBadRequest)

			return
		}

		_, _ = response.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)
	t.Setenv("ZEPP_EVENTS_URL", server.URL)
}

func hasAdjustmentCondition(adjustments []icu.TrainingPlanDayAdjustment, condition string) bool {
	for index := range adjustments {
		if adjustments[index].Condition == condition {
			return true
		}
	}

	return false
}

func containsString(values []string, want string) bool {
	for index := range values {
		if values[index] == want {
			return true
		}
	}

	return false
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
