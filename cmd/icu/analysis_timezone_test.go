package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
