package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

// 02:00Z on Oct 28 is 22:00 on Oct 27 in New York.
func clockNY1027() time.Time { return time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC) }

type clampFake struct {
	mu             sync.Mutex
	wellnessNewest []string
	client         *icu.Client
}

func newClampFake(t *testing.T) *clampFake {
	t.Helper()
	fake := &clampFake{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("non-GET %s %s", request.Method, request.URL.Path)
		}
		switch path := request.URL.Path; {
		case strings.HasSuffix(path, "/athlete/i1"):
			_ = json.NewEncoder(writer).Encode(map[string]any{"id": "i1", "timezone": "America/New_York"})
		case strings.HasSuffix(path, "/wellness"):
			fake.mu.Lock()
			fake.wellnessNewest = append(fake.wellnessNewest, request.URL.Query().Get("newest"))
			fake.mu.Unlock()
			_, _ = writer.Write([]byte(`[]`))
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

func (f *clampFake) newest() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string{}, f.wellnessNewest...)
}

func runClamp(t *testing.T, command *Command, fake *clampFake, flags map[string]string) {
	t.Helper()
	if err := command.Run(nil, flags, fake.client, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	newest := fake.newest()
	if len(newest) == 0 {
		t.Fatal("no wellness read")
	}
	for _, got := range newest {
		if got != "2026-10-27" {
			t.Fatalf("wellness newest = %v, want 2026-10-27 (today in New York)", newest)
		}
	}
}

// cycling clamps its wellness read to the athlete's today (10-27 in New York
// at 02:00Z on 10-28), not the UTC date.
func TestAnalysisCyclingClampsWellnessToAthleteToday(t *testing.T) {
	t.Parallel()
	runClamp(t, analysisCyclingCommandWithClock(clockNY1027), newClampFake(t),
		map[string]string{"oldest": "2026-10-01", "newest": "2026-10-31"})
}

func TestAnalysisPlanClampsWellnessToAthleteToday(t *testing.T) {
	t.Parallel()
	runClamp(t, analysisPlanCommandWithClock(clockNY1027), newClampFake(t),
		map[string]string{"history-oldest": "2026-10-05", "history-newest": "2026-10-31"})
}

func TestClampDateToTodayNewYorkEvening(t *testing.T) {
	t.Parallel()
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := clockNY1027().In(location)
	for in, want := range map[string]string{"2026-10-28": "2026-10-27", "2026-10-31": "2026-10-27", "2026-10-27": "2026-10-27", "2026-10-20": "2026-10-20"} {
		if got := clampDateToToday(in, now); got != want {
			t.Fatalf("clampDateToToday(%s) = %s, want %s", in, got, want)
		}
	}
}

// coaching reports the timezone it used in scope.timezone/timezoneSource,
// the same fields analysis wellness reports in its scope.
func TestAnalysisCoachingReportsTimezoneLikeWellness(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		flags      map[string]string
		zone, from string
	}{
		"flag":    {map[string]string{"timezone": "Europe/Madrid"}, "Europe/Madrid", "flag"},
		"athlete": {map[string]string{}, "America/New_York", "athlete"},
	}
	for name, test := range cases {
		coaching := runTZ(t, analysisCoachingCommand(), newTZFake(t, "2026-10-20T12:00:00Z"), test.flags)
		wellness := runTZ(t, analysisWellnessCommand(), newTZFake(t, "2026-10-20T12:00:00Z"), test.flags)
		coachingScope, _ := coaching["scope"].(map[string]any)
		wellnessScope, _ := wellness["scope"].(map[string]any)
		if coachingScope["timezone"] != test.zone || coachingScope["timezoneSource"] != test.from {
			t.Fatalf("%s: coaching scope = %v, want %s/%s", name, coachingScope, test.zone, test.from)
		}
		if coachingScope["timezone"] != wellnessScope["timezone"] || coachingScope["timezoneSource"] != wellnessScope["timezoneSource"] {
			t.Fatalf("%s: coaching %v vs wellness %v", name, coachingScope, wellnessScope)
		}
	}
}
