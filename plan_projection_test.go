package icu_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

type replayRow struct {
	ID   string  `json:"id"`
	CTL  float64 `json:"ctl"`
	ATL  float64 `json:"atl"`
	Load float64 `json:"load"`
}

func loadReplayRows(t *testing.T) []replayRow {
	t.Helper()
	data, err := os.ReadFile("testdata/wellness_replay.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Rows []replayRow `json:"rows"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}

	return fixture.Rows
}

// TestProjectLoadReplaysRealWellness replays 45 real days (read-only GETs of
// wellness and activities, see the fixture's _source): starting from the
// first row's CTL/ATL and feeding the real daily loads, every later day must
// match intervals.icu's own CTL/ATL to 3 decimals. The linear 1/tau model
// misses ATL by up to 0.8 on the same data.
func TestProjectLoadReplaysRealWellness(t *testing.T) {
	t.Parallel()
	rows := loadReplayRows(t)
	loads := map[string]float64{}
	for _, row := range rows {
		loads[row.ID] = row.Load
	}
	first := rows[0]
	for _, row := range rows[1:] {
		got, err := icu.ProjectLoad(&icu.LoadProjectionInput{
			StartDate: first.ID, StartCTL: first.CTL, StartATL: first.ATL, EndDate: row.ID, PlannedLoads: loads,
		})
		if err != nil {
			t.Fatal(err)
		}
		// End values are rounded to 2 decimals, so compare at that precision.
		if math.Abs(got.EndCTL-row.CTL) > 0.006 || math.Abs(got.EndATL-row.ATL) > 0.006 {
			t.Fatalf("%s: projected %.3f/%.3f, intervals.icu %.3f/%.3f", row.ID, got.EndCTL, got.EndATL, row.CTL, row.ATL)
		}
	}
	// The same replay with the linear factor would be off by more than 0.5 ATL.
	linear := first.ATL
	worst := 0.0
	for _, row := range rows[1:] {
		linear += (row.Load - linear) / 7
		worst = math.Max(worst, math.Abs(linear-row.ATL))
	}
	if worst < 0.5 {
		t.Fatalf("linear model error %.3f, fixture no longer discriminates the models", worst)
	}
}

func TestProjectLoadRollingRampDoesNotExtrapolatePartialWeeks(t *testing.T) {
	t.Parallel()
	// Start Saturday 2026-10-31: the first ISO week has one projected day
	// (Sunday). One 120-load day raises CTL ~2.1; the old Ramp*7/Days rule
	// reported ~14.8/week. The real rolling 7-day ramp against wellness CTL
	// is ~2.1, under a 5/week cap.
	known := map[string]float64{}
	for day := 24; day <= 31; day++ {
		known[fmt.Sprintf("2026-10-%02d", day)] = 50
	}
	got, err := icu.ProjectLoad(&icu.LoadProjectionInput{
		StartDate: "2026-10-31", StartCTL: 50, StartATL: 50, EndDate: "2026-11-01",
		PlannedLoads: map[string]float64{"2026-11-01": 140}, KnownCTL: known, MaxRampPerWeek: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	week := got.Weeks[0]
	if week.Days != 1 || !week.RampKnown || week.RampPerWeek > 3 || week.OverCap || len(got.Warnings) != 0 {
		t.Fatalf("week = %+v warnings=%v, want a ~2 rolling ramp and no over-cap", week, got.Warnings)
	}

	// Without earlier CTL the ramp is unknown and never flagged.
	unknown, _ := icu.ProjectLoad(&icu.LoadProjectionInput{
		StartDate: "2026-10-31", StartCTL: 50, StartATL: 50, EndDate: "2026-11-01",
		PlannedLoads: map[string]float64{"2026-11-01": 500}, MaxRampPerWeek: 1,
	})
	if unknown.Weeks[0].RampKnown || unknown.Weeks[0].OverCap {
		t.Fatalf("unknown ramp week = %+v", unknown.Weeks[0])
	}

	// A sustained build is flagged once 7 days of CTL exist.
	planned := map[string]float64{}
	for day := 1; day <= 14; day++ {
		planned[fmt.Sprintf("2026-11-%02d", day)] = 150
	}
	build, _ := icu.ProjectLoad(&icu.LoadProjectionInput{
		StartDate: "2026-10-31", StartCTL: 50, StartATL: 50, EndDate: "2026-11-14",
		PlannedLoads: planned, KnownCTL: known, MaxRampPerWeek: 5,
	})
	if !build.Weeks[1].OverCap || !strings.Contains(build.Warnings[0], "rolling 7-day CTL ramp") {
		t.Fatalf("build weeks = %+v warnings=%v", build.Weeks, build.Warnings)
	}
}

func TestProjectLoadUsesActualPastAndTodayMax(t *testing.T) {
	t.Parallel()
	input := icu.LoadProjectionInput{
		StartDate: "2026-10-24", StartCTL: 0, StartATL: 0, EndDate: "2026-10-27", Today: "2026-10-26",
		ActualLoads:  map[string]float64{"2026-10-25": 30, "2026-10-26": 10},
		PlannedLoads: map[string]float64{"2026-10-25": 999, "2026-10-26": 80, "2026-10-27": 50},
	}
	got, err := icu.ProjectLoad(&input)
	if err != nil {
		t.Fatal(err)
	}
	// Past Sunday: actual 30 (not planned 999). Today (Monday): max(10, 80).
	// Tomorrow: planned 50.
	if got.Weeks[0].Load != 30 || got.Weeks[1].Load != 130 {
		t.Fatalf("weeks = %+v, want 30 then 80+50", got.Weeks)
	}
	if _, err := icu.ProjectLoad(&icu.LoadProjectionInput{StartDate: "x", EndDate: "2026-11-08"}); err == nil {
		t.Fatal("bad start accepted")
	}
	if _, err := icu.ProjectLoad(&icu.LoadProjectionInput{StartDate: "2026-11-08", EndDate: "y"}); err == nil {
		t.Fatal("bad end accepted")
	}
}

func TestPlanProjectedDailyLoads(t *testing.T) {
	t.Parallel()

	operations := []icu.CalendarOperation{
		{Action: icu.CalendarActionCreate, ExpectedLoad: 80, Body: icu.EventEx{StartDateLocal: "2026-11-02T07:00:00"}},
		{Action: icu.CalendarActionUpdate, EventID: 2, ExpectedLoad: 60, Body: icu.EventEx{StartDateLocal: "2026-11-03T07:00:00"}},
		{Action: icu.CalendarActionCancel, EventID: 3},
		{
			Action: icu.CalendarActionCreate, Status: icu.CalendarStatusApplied, ExpectedLoad: 999,
			Body: icu.EventEx{StartDateLocal: "2026-11-04T07:00:00"},
		},
	}
	events := []icu.Event{
		{ID: 1, Category: "WORKOUT", StartDateLocal: "2026-11-02T18:00:00", TrainingLoad: 40},
		{ID: 2, Category: "WORKOUT", StartDateLocal: "2026-11-03T07:00:00", TrainingLoad: 100},
		{ID: 3, Category: "WORKOUT", StartDateLocal: "2026-11-05T07:00:00", TrainingLoad: 100},
		{ID: 4, Category: "WORKOUT", StartDateLocal: "2026-11-06T07:00:00", LoadTarget: 55},
		{ID: 5, Category: "NOTE", StartDateLocal: "2026-11-06T07:00:00", TrainingLoad: 500},
	}

	got := icu.PlanProjectedDailyLoads(operations, events)
	if got["2026-11-02"] != 120 || got["2026-11-03"] != 60 || got["2026-11-05"] != 0 ||
		got["2026-11-06"] != 55 || got["2026-11-04"] != 0 {
		t.Fatalf("loads = %v", got)
	}
}

func TestLatestWellnessFitnessSkipsFutureAndEmptyRows(t *testing.T) {
	t.Parallel()

	records := []icu.Wellness{
		{ID: "2026-10-24", CTL: 40, ATL: 50},
		{ID: "2026-10-26", CTL: 41, ATL: 52},
		{ID: "2026-10-27", CTL: 0, ATL: 0},
		{ID: "2026-10-30", CTL: 45, ATL: 30},
	}
	got, ok := icu.LatestWellnessFitness(records, "2026-10-27")
	if !ok || got.ID != "2026-10-26" {
		t.Fatalf("latest = %+v, %v", got, ok)
	}
	if _, ok := icu.LatestWellnessFitness(nil, "2026-10-27"); ok {
		t.Fatal("empty records returned a row")
	}
}
