package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

// tzFake is a GET-only intervals.icu mock: athlete in New York, one ride
// whose only timestamp is start_date (UTC), counting every request and the
// /athlete ones separately.
type tzFake struct {
	requests, athleteRequests atomic.Int32
	client                    *icu.Client
}

func newTZFake(t *testing.T, startDate string) *tzFake {
	t.Helper()
	fake := &tzFake{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fake.requests.Add(1)
		if request.Method != http.MethodGet {
			t.Errorf("non-GET %s %s", request.Method, request.URL.Path)
		}
		switch path := request.URL.Path; {
		case strings.HasSuffix(path, "/athlete/i1"):
			fake.athleteRequests.Add(1)
			_ = json.NewEncoder(writer).Encode(map[string]any{"id": "i1", "timezone": "America/New_York"})
		case strings.HasSuffix(path, "/activities"):
			_ = json.NewEncoder(writer).Encode([]map[string]any{{
				"id": "a1", "type": "Ride", "start_date": startDate, "icu_training_load": 50, "moving_time": 3600,
			}})
		case strings.Contains(path, "power-curves"):
			_, _ = writer.Write([]byte(`{"list":[]}`))
		case strings.Contains(path, "mmp-model"), strings.Contains(path, "sport-settings"):
			_, _ = writer.Write([]byte(`{"ftp":250}`))
		default:
			_, _ = writer.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)
	fake.client = icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	return fake
}

func runTZ(t *testing.T, command *Command, fake *tzFake, flags map[string]string) map[string]any {
	t.Helper()
	var out strings.Builder
	if err := command.Run(nil, flags, fake.client, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}

	return decoded
}

// An unknown --timezone is rejected before any request, in every analysis
// command that resolves the timezone.
func TestAnalysisInvalidTimezoneMakesNoRequests(t *testing.T) {
	t.Parallel()
	for name, command := range map[string]*Command{
		"cycling": analysisCyclingCommand(), "coaching": analysisCoachingCommand(), "wellness": analysisWellnessCommand(),
		"plan": analysisPlanCommand(), "adaptation": analysisAdaptationCommand(),
	} {
		fake := newTZFake(t, "2026-10-28T03:30:00Z")
		err := command.Run(nil, map[string]string{"timezone": "Not/AZone"}, fake.client, io.Discard, io.Discard)
		if !errors.Is(err, errInvalidValue) || fake.requests.Load() != 0 {
			t.Fatalf("%s: err %v, %d requests, want errInvalidValue and 0", name, err, fake.requests.Load())
		}
	}
}

// The timezone is resolved once per run: one /athlete GET in cycling and
// plan (wellness clamping reuses it).
func TestAnalysisTimezoneResolvedOnce(t *testing.T) {
	t.Parallel()
	for name, command := range map[string]*Command{"cycling": analysisCyclingCommand(), "plan": analysisPlanCommand()} {
		fake := newTZFake(t, "2026-10-28T03:30:00Z")
		runTZ(t, command, fake, map[string]string{})
		if fake.athleteRequests.Load() != 1 {
			t.Fatalf("%s: %d /athlete requests, want 1", name, fake.athleteRequests.Load())
		}
	}
}

// coaching: a 23:30 New York ride (03:30Z next day) lands on the local day.
func TestAnalysisCoachingLocalizesActivityDates(t *testing.T) {
	t.Parallel()
	fake := newTZFake(t, "2026-10-28T03:30:00Z")
	got := runTZ(t, analysisCoachingCommand(), fake, map[string]string{
		"history-oldest": "2026-10-20", "history-newest": "2026-10-28",
	})
	analyses, _ := got["analyses"].(map[string]any)
	cycling, _ := analyses["cycling"].(map[string]any)
	sessions, _ := cycling["sessions"].([]any)
	if len(sessions) != 1 {
		t.Fatalf("coaching sessions = %v", cycling["sessions"])
	}
	if session, _ := sessions[0].(map[string]any); session["date"] != "2026-10-27" {
		t.Fatalf("coaching session = %v, want date 2026-10-27", session)
	}
}

// plan: Sunday 23:30 in New York is Monday 03:30Z; the load belongs to the
// local ISO week (W43), not the UTC one (W44).
func TestAnalysisPlanLocalizesActivityDates(t *testing.T) {
	t.Parallel()
	fake := newTZFake(t, "2026-10-26T03:30:00Z")
	got := runTZ(t, analysisPlanCommand(), fake, map[string]string{
		"history-oldest": "2026-10-19", "history-newest": "2026-10-28",
	})
	history, _ := got["history"].(map[string]any)
	weeks, _ := history["weeks"].([]any)
	loads := map[string]float64{}
	for _, item := range weeks {
		week, _ := item.(map[string]any)
		name, _ := week["isoWeek"].(string)
		loads[name], _ = week["load"].(float64)
	}
	if loads["2026-W43"] != 50 || loads["2026-W44"] != 0 {
		t.Fatalf("weekly loads = %v, want the ride in 2026-W43", loads)
	}
}

// adaptation: the ride on the window's last local day (newest 2026-10-27)
// counts; in UTC it would fall on 10-28, outside the window.
func TestAnalysisAdaptationLocalizesActivityDates(t *testing.T) {
	t.Parallel()
	fake := newTZFake(t, "2026-10-28T03:30:00Z")
	got := runTZ(t, analysisAdaptationCommand(), fake, map[string]string{"oldest": "2026-10-01", "newest": "2026-10-27"})
	phase, _ := got["phaseSummary"].(map[string]any)
	if phase["recentLoad"] != 50.0 {
		t.Fatalf("phaseSummary = %v, want recentLoad 50 (ride on local 10-27)", phase)
	}
}

// wellness "today" follows the injected clock in the athlete's timezone:
// 03:30Z on Oct 28 is 23:30 on Oct 27 in New York, so the default window
// ends 10-27.
func TestAnalysisWellnessTodayUsesInjectedClock(t *testing.T) {
	t.Parallel()
	var newest atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch path := request.URL.Path; {
		case strings.HasSuffix(path, "/athlete/i1"):
			_ = json.NewEncoder(writer).Encode(map[string]any{"id": "i1", "timezone": "America/New_York"})
		case strings.HasSuffix(path, "/wellness"):
			newest.Store(request.URL.Query().Get("newest"))
			_, _ = writer.Write([]byte(`[]`))
		default:
			_, _ = writer.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	clock := func() time.Time { return time.Date(2026, 10, 28, 3, 30, 0, 0, time.UTC) }
	var out strings.Builder
	if err := analysisWellnessCommandWithClock(clock).Run(nil, map[string]string{}, client, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if newest.Load() != "2026-10-27" {
		t.Fatalf("wellness newest = %v, want 2026-10-27", newest.Load())
	}
	if !strings.Contains(out.String(), `"endDate": "2026-10-27"`) {
		t.Fatalf("output = %s", out.String())
	}
}

// coaching reuses the profile read by the timezone resolver: one /athlete.
func TestAnalysisCoachingFetchesAthleteOnce(t *testing.T) {
	t.Parallel()
	fake := newTZFake(t, "2026-10-28T03:30:00Z")
	runTZ(t, analysisCoachingCommand(), fake, map[string]string{})
	if fake.athleteRequests.Load() != 1 {
		t.Fatalf("/athlete requests = %d, want 1", fake.athleteRequests.Load())
	}
	// With --timezone the resolver skips /athlete, so coaching reads it once.
	flagged := newTZFake(t, "2026-10-28T03:30:00Z")
	runTZ(t, analysisCoachingCommand(), flagged, map[string]string{"timezone": "Europe/Madrid"})
	if flagged.athleteRequests.Load() != 1 {
		t.Fatalf("/athlete requests with --timezone = %d, want 1", flagged.athleteRequests.Load())
	}
}
