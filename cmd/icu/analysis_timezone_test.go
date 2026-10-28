package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

func TestAnalysisTimezoneFromPrecedence(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		flag, athlete, wantZone, wantSource string
	}{
		{"", "America/New_York", "America/New_York", timezoneSourceAthlete},
		{"Europe/Madrid", "America/New_York", "Europe/Madrid", timezoneSourceFlag},
		{"Not/AZone", "America/New_York", "America/New_York", timezoneSourceAthlete},
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
	if got := resolveAnalysisTimezone(client, map[string]string{}); got.timezone != "America/New_York" || got.source != timezoneSourceAthlete {
		t.Fatalf("athlete tz = %+v", got)
	}
	broken := icu.NewClient("k", "i1", icu.WithBaseURL("http://127.0.0.1:1"))
	if got := resolveAnalysisTimezone(broken, map[string]string{}); got.source != icu.DefaultAnalysisTimezoneSource {
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
