package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // zone data on any CI image

	icu "github.com/Thejuampi/icu"
)

func TestAnalysisTimezoneFromPrecedence(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		flag, athlete, wantZone, wantSource string
	}{
		{"", "America/New_York", "America/New_York", timezoneSourceAthlete},
		{"Europe/Madrid", "America/New_York", "Europe/Madrid", timezoneSourceFlag},
		{"", "Not/AZone", icu.DefaultAnalysisTimezone, icu.DefaultAnalysisTimezoneSource},
		{"", "", icu.DefaultAnalysisTimezone, icu.DefaultAnalysisTimezoneSource},
	} {
		got := analysisTimezoneFrom(test.flag, test.athlete)
		if got.timezone != test.wantZone || got.source != test.wantSource || got.location.String() != test.wantZone {
			t.Fatalf("%q/%q = %+v, want %s %s", test.flag, test.athlete, got, test.wantZone, test.wantSource)
		}
	}
}

func TestResolveAnalysisTimezoneUsesAthleteProfile(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("non-GET %s", request.Method)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"id": "i1", "timezone": "America/New_York"})
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	if got, err := resolveAnalysisTimezone(client, map[string]string{}); err != nil || got.timezone != "America/New_York" || got.source != timezoneSourceAthlete {
		t.Fatalf("athlete tz = %+v", got)
	}
	broken := icu.NewClient("k", "i1", icu.WithBaseURL("http://127.0.0.1:1"))
	if got, err := resolveAnalysisTimezone(broken, map[string]string{}); err != nil || got.source != icu.DefaultAnalysisTimezoneSource {
		t.Fatalf("fallback = %+v", got)
	}
}

// At 22:00 EDT on Oct 27 it is already Oct 28 in UTC: the default windows
// must end on the athlete's day, not the UTC one.
func TestAnalysisDateWindowsFollowAthleteDay(t *testing.T) {
	t.Parallel()
	newYork := analysisTimezoneFrom("", "America/New_York").location
	now := time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC).In(newYork)
	got, _, err := analysisDateRange(map[string]string{}, now)
	if err != nil || got.Newest != "2026-10-27" {
		t.Fatalf("cycling window = %+v %v, want newest 2026-10-27", got, err)
	}
	plan, _, err := trainingPlanDateRanges(map[string]string{}, now)
	if err != nil || plan.History.Newest != "2026-10-27" {
		t.Fatalf("plan windows = %+v %v", plan, err)
	}
	utc, _, _ := analysisDateRange(map[string]string{}, now.UTC())
	if utc.Newest != "2026-10-28" {
		t.Fatalf("utc window = %+v", utc)
	}
}

// analysis cycling with only start_date: the athlete is in New York and the
// ride started at 23:30 local (03:30 UTC next day); the session keeps the
// local day and scope reports the athlete timezone.
func TestAnalysisCyclingUsesAthleteLocalDay(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("non-GET %s", request.Method)
		}
		switch path := request.URL.Path; {
		case strings.HasSuffix(path, "/athlete/i1"):
			_ = json.NewEncoder(writer).Encode(map[string]any{"id": "i1", "timezone": "America/New_York"})
		case strings.HasSuffix(path, "/activities"):
			_ = json.NewEncoder(writer).Encode([]map[string]any{{"id": "a1", "type": "Ride", "start_date": "2026-10-28T03:30:00Z"}})
		default:
			_, _ = writer.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	var out strings.Builder
	flags := map[string]string{"oldest": "2026-10-20", "newest": "2026-10-28"}
	if err := analysisCyclingCommand().Run(nil, flags, client, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Scope struct {
			Timezone       string `json:"timezone"`
			TimezoneSource string `json:"timezoneSource"`
		} `json:"scope"`
		Sessions []struct {
			Date string `json:"date"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatal(err)
	}
	if got.Scope.Timezone != "America/New_York" || got.Scope.TimezoneSource != timezoneSourceAthlete ||
		len(got.Sessions) != 1 || got.Sessions[0].Date != "2026-10-27" {
		t.Fatalf("output = %s", out.String())
	}
}

// 23:30 EDT on Tue Oct 27 (03:30 UTC Oct 28): history ends today in New
// York and the plan starts the next day, with no gap to the next ISO week.
func TestTrainingPlanWindowsNewYorkLateEvening(t *testing.T) {
	t.Parallel()
	newYork := analysisTimezoneFrom("", "America/New_York").location
	now := time.Date(2026, 10, 27, 23, 30, 0, 0, newYork)
	got, _, err := trainingPlanDateRanges(map[string]string{}, now)
	if err != nil || got.History.Newest != "2026-10-27" || got.Plan.Oldest != "2026-10-28" || got.Plan.Newest != "2026-11-24" {
		t.Fatalf("NY windows = %+v %v", got, err)
	}
	utc, _, _ := trainingPlanDateRanges(map[string]string{}, now.UTC())
	if utc.History.Newest != "2026-10-28" || utc.Plan.Oldest != "2026-10-29" {
		t.Fatalf("UTC windows = %+v", utc)
	}
	cycling, _, _ := analysisDateRange(map[string]string{}, now)
	if cycling.Newest != "2026-10-27" {
		t.Fatalf("NY cycling window = %+v", cycling)
	}
}

// info.now() reads the injected clock in the analysis timezone.
func TestAnalysisTimezoneInfoNowUsesInjectedClock(t *testing.T) {
	t.Parallel()
	info := analysisTimezoneFrom("", "America/New_York")
	info.clock = func() time.Time { return time.Date(2026, 10, 28, 3, 30, 0, 0, time.UTC) }
	now := info.now()
	if now.Format("2006-01-02 15:04") != "2026-10-27 23:30" || now.Location().String() != "America/New_York" {
		t.Fatalf("now = %v", now)
	}
	got, _, err := trainingPlanDateRanges(map[string]string{}, info.now())
	if err != nil || got.History.Newest != "2026-10-27" || got.Plan.Oldest != "2026-10-28" {
		t.Fatalf("windows = %+v %v", got, err)
	}
	if since := time.Since(analysisTimezoneFrom("", "").now()); since < 0 || since > time.Minute {
		t.Fatalf("default clock is not time.Now: %v", since)
	}
}

// 2026-11-01 01:30 in New York happens twice (EDT 05:30Z, then EST
// 06:30Z after the fall-back): both are Nov 1 locally, so history ends
// 11-01 and the plan starts 11-02.
func TestAnalysisWindowsNewYorkDSTFallBack(t *testing.T) {
	t.Parallel()
	info := analysisTimezoneFrom("", "America/New_York")
	for _, instant := range []time.Time{
		time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC), time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC),
	} {
		info.clock = func() time.Time { return instant }
		now := info.now()
		if now.Format("2006-01-02 15:04") != "2026-11-01 01:30" {
			t.Fatalf("%v local = %v", instant, now)
		}
		got, _, err := trainingPlanDateRanges(map[string]string{}, now)
		if err != nil || got.History.Newest != "2026-11-01" || got.Plan.Oldest != "2026-11-02" {
			t.Fatalf("%v windows = %+v %v", instant, got, err)
		}
		if cycling, _, _ := analysisDateRange(map[string]string{}, now); cycling.Newest != "2026-11-01" {
			t.Fatalf("%v cycling = %+v", instant, cycling)
		}
	}
}

// Explicit plan dates still snap to ISO weeks (documented in --help).
func TestExplicitPlanWindowSnapsToISOWeek(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 27, 23, 30, 0, 0, analysisTimezoneFrom("", "America/New_York").location)
	got, explicit, err := trainingPlanDateRanges(map[string]string{"plan-oldest": "2026-10-28", "plan-newest": "2026-11-04"}, now)
	if err != nil || !explicit || got.Plan.Oldest != "2026-10-26" || got.Plan.Newest != "2026-11-08" {
		t.Fatalf("explicit = %+v %v %v", got, explicit, err)
	}
}

// An invalid --timezone is an error, not a silent UTC fallback.
func TestResolveAnalysisTimezoneRejectsInvalidFlag(t *testing.T) {
	t.Parallel()
	if _, err := resolveAnalysisTimezone(nil, map[string]string{"timezone": "Not/AZone"}); !errors.Is(err, errInvalidValue) {
		t.Fatalf("err = %v, want errInvalidValue", err)
	}
	if got, err := resolveAnalysisTimezone(nil, map[string]string{"timezone": "Europe/Madrid"}); err != nil || got.source != timezoneSourceFlag {
		t.Fatalf("flag = %+v %v", got, err)
	}
}
