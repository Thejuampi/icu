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

type replayActivity struct {
	Type      string  `json:"type"`
	Load      float64 `json:"icuTrainingLoad"`
	HRLoad    float64 `json:"hrLoad"`
	PaceLoad  float64 `json:"paceLoad"`
	PowerLoad float64 `json:"powerLoad"`
}

type replayRow struct {
	ID         string           `json:"id"`
	CTL        float64          `json:"ctl"`
	ATL        float64          `json:"atl"`
	CTLLoad    float64          `json:"ctlLoad"`
	ATLLoad    float64          `json:"atlLoad"`
	Activities []replayActivity `json:"activities"`
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

// replayWorst replays every row from the first and returns the worst CTL and
// ATL error against intervals.icu and the date of the worst CTL miss.
func replayWorst(t *testing.T, rows []replayRow, fitness, fatigue map[string]float64) (float64, float64, string) {
	t.Helper()
	first := rows[0]
	worstCTL, worstATL, worstDate := 0.0, 0.0, ""
	for _, row := range rows[1:] {
		got, err := icu.ProjectLoad(&icu.LoadProjectionInput{
			StartDate: first.ID, StartCTL: first.CTL, StartATL: first.ATL, EndDate: row.ID,
			PlannedLoads: fatigue, PlannedFitnessLoads: fitness,
		})
		if err != nil {
			t.Fatal(err)
		}
		if diff := math.Abs(got.EndCTL - row.CTL); diff > worstCTL {
			worstCTL, worstDate = diff, row.ID
		}
		worstATL = math.Max(worstATL, math.Abs(got.EndATL-row.ATL))
	}

	return worstCTL, worstATL, worstDate
}

// TestProjectLoadReplaysRealWellness replays 2026-07-31..10-26 of real data,
// including the days where the athlete overrode icu_training_load by hand
// (e.g. 09-07 run: 15 = hr_load, pace_load 8) and the strength days that
// intervals.icu counts toward ATL only (08-18, 08-30, 09-06, 09-07, 09-11).
func TestProjectLoadReplaysRealWellness(t *testing.T) {
	t.Parallel()
	rows := loadReplayRows(t)
	wellness := make([]icu.Wellness, 0, len(rows))
	var activities []icu.Activity
	activityLoads := map[string]float64{}
	ctlLoads, atlLoads := map[string]float64{}, map[string]float64{}
	overrides := 0
	for _, row := range rows {
		wellness = append(wellness, icu.Wellness{ID: row.ID, CTLLoad: row.CTLLoad, ATLLoad: row.ATLLoad})
		ctlLoads[row.ID], atlLoads[row.ID] = row.CTLLoad, row.ATLLoad
		for _, activity := range row.Activities {
			activityLoads[row.ID] += activity.Load
			activities = append(activities, icu.Activity{
				Type: activity.Type, StartDateLocal: row.ID + "T12:00:00", TrainingLoad: int(activity.Load),
			})
			if activity.PaceLoad > 0 && activity.Load != activity.PaceLoad {
				overrides++
			}
		}
		// The effective (overridden) icu_training_load is what feeds ATL.
		if math.Abs(activityLoads[row.ID]-row.ATLLoad) > 0.5 {
			t.Fatalf("%s: activity load %.1f != wellness atlLoad %.1f", row.ID, activityLoads[row.ID], row.ATLLoad)
		}
	}
	if overrides == 0 {
		t.Fatal("fixture has no manually overridden load")
	}

	// 1. The wellness ctlLoad/atlLoad pair reproduces intervals.icu exactly.
	if ctlErr, atlErr, date := replayWorst(t, rows, ctlLoads, atlLoads); ctlErr > 0.006 || atlErr > 0.006 {
		t.Fatalf("wellness loads: worst CTL %.3f (%s) ATL %.3f", ctlErr, date, atlErr)
	}

	// 2. Activity loads, with the fitness-excluded types inferred from
	// history, reproduce it too.
	classes, conflicts := icu.ClassifyFitnessLoadTypes(icu.LearnFitnessLoadTypes(wellness, activities))
	excluded, _ := icu.ResolveFitnessLoadTypes(nil, classes, nil, conflicts, nil)
	if !excluded["WeightTraining"] || !excluded["Workout"] || excluded["Ride"] || excluded["VirtualRun"] ||
		len(conflicts) != 0 {
		t.Fatalf("excluded = %v, conflicts %v", excluded, conflicts)
	}
	// Trimming the window start never turns a CTL type into ATL-only (the
	// old whole-day heuristic flagged VirtualRun from 08-14/08-19 and
	// learned nothing from 09-12).
	for _, from := range []string{"2026-08-14", "2026-08-19", "2026-09-01", "2026-09-12"} {
		var trimmedW []icu.Wellness
		var trimmedA []icu.Activity
		for _, row := range wellness {
			if row.ID >= from {
				trimmedW = append(trimmedW, row)
			}
		}
		for _, activity := range activities {
			if activity.StartDateLocal >= from {
				trimmedA = append(trimmedA, activity)
			}
		}
		got, gotConflicts := icu.ClassifyFitnessLoadTypes(icu.LearnFitnessLoadTypes(trimmedW, trimmedA))
		for sport, class := range got {
			if (class == icu.FitnessLoadATLOnly) != excluded[sport] {
				t.Fatalf("from %s: %s classified %s, full window says excluded=%v", from, sport, class, excluded[sport])
			}
		}
		if len(gotConflicts) != 0 {
			t.Fatalf("from %s: conflicts %v", from, gotConflicts)
		}
	}
	fitness := map[string]float64{}
	for _, row := range rows {
		for _, activity := range row.Activities {
			if !excluded[activity.Type] {
				fitness[row.ID] += activity.Load
			}
		}
	}
	if ctlErr, atlErr, date := replayWorst(t, rows, fitness, activityLoads); ctlErr > 0.006 || atlErr > 0.006 {
		t.Fatalf("activity loads: worst CTL %.3f (%s) ATL %.3f", ctlErr, date, atlErr)
	}

	// 3. Feeding every load into CTL (the old model) misses from 08-18 on.
	if ctlErr, _, date := replayWorst(t, rows, nil, activityLoads); ctlErr < 0.1 || date < "2026-08-18" {
		t.Fatalf("single-load CTL error %.3f (%s): fixture no longer discriminates", ctlErr, date)
	}

	// 4. The linear 1/tau factor would be off by more than 0.5 ATL.
	linear, worst := rows[0].ATL, 0.0
	for _, row := range rows[1:] {
		linear += (row.ATLLoad - linear) / 7
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

func TestProjectLoadUsesActualPastAndTodayRemaining(t *testing.T) {
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
	// Past Sunday: actual 30 (not planned 999). Today (Monday): completed 10
	// plus the 80 still planned. Tomorrow: planned 50.
	if got.Weeks[0].Load != 30 || got.Weeks[1].Load != 140 {
		t.Fatalf("weeks = %+v, want 30 then 10+80+50", got.Weeks)
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

func TestPlanProjectedDailyLoadsByModelSkipsCompletedAndFatigueOnly(t *testing.T) {
	t.Parallel()
	events := []icu.Event{
		{ID: 1, Category: "WORKOUT", Type: "Ride", StartDateLocal: "2026-10-26T07:00:00", TrainingLoad: 60},
		{ID: 2, Category: "WORKOUT", Type: "Ride", StartDateLocal: "2026-10-26T18:00:00", TrainingLoad: 40},
		{ID: 3, Category: "WORKOUT", Type: "WeightTraining", StartDateLocal: "2026-10-27T18:00:00", TrainingLoad: 30},
	}
	operations := []icu.CalendarOperation{{
		Action: icu.CalendarActionUpdate, EventID: 1, ExpectedLoad: 999,
		Body: icu.EventEx{StartDateLocal: "2026-10-26T07:00:00"},
	}}
	loads, fitness := icu.PlanProjectedDailyLoadsByModel(operations, events,
		map[string]bool{"WeightTraining": true}, map[int]bool{1: true})
	// Event 1 is done (its update too); event 2 is still planned; strength
	// counts toward ATL only.
	if loads["2026-10-26"] != 40 || fitness["2026-10-26"] != 40 ||
		loads["2026-10-27"] != 30 || fitness["2026-10-27"] != 0 {
		t.Fatalf("loads=%v fitness=%v", loads, fitness)
	}
}

func TestProjectLoadRampKnownGatesOverCap(t *testing.T) {
	t.Parallel()
	planned := map[string]float64{"2026-10-27": 300, "2026-10-28": 300}
	input := icu.LoadProjectionInput{
		StartDate: "2026-10-26", StartCTL: 40, StartATL: 40, EndDate: "2026-10-28",
		PlannedLoads: planned, MaxRampPerWeek: 1,
	}
	// No CTL 7 days back: the ramp is unknown and never flagged.
	got, err := icu.ProjectLoad(&input)
	if err != nil || got.Weeks[0].RampKnown || got.Weeks[0].OverCap || len(got.Warnings) != 0 {
		t.Fatalf("unknown ramp flagged: %+v, %v", got, err)
	}
	input.KnownCTL = map[string]float64{"2026-10-20": 40, "2026-10-21": 40}
	got, err = icu.ProjectLoad(&input)
	if err != nil || !got.Weeks[0].RampKnown || !got.Weeks[0].OverCap || len(got.Warnings) != 1 {
		t.Fatalf("known ramp not flagged: %+v, %v", got, err)
	}
}

func TestLearnFitnessLoadTypesSolvesMixedDays(t *testing.T) {
	t.Parallel()
	wellness := []icu.Wellness{
		// Walk has no sport settings but intervals.icu counted it in CTL.
		{ID: "2026-08-10", CTLLoad: 12, ATLLoad: 12},
		// VirtualRun + WeightTraining the same day: only the run's 2 feeds CTL.
		{ID: "2026-08-30", CTLLoad: 2, ATLLoad: 14},
		// Two types with equal loads, one counted: ambiguous, proves nothing.
		{ID: "2026-09-01", CTLLoad: 10, ATLLoad: 20},
		// Activity loads do not add up to atlLoad: skipped.
		{ID: "2026-09-02", CTLLoad: 0, ATLLoad: 99},
	}
	activities := []icu.Activity{
		{Type: "Walk", StartDateLocal: "2026-08-10T10:00:00", TrainingLoad: 12},
		{Type: "WeightTraining", StartDateLocal: "2026-08-30T17:00:00", TrainingLoad: 12},
		{Type: "VirtualRun", StartDateLocal: "2026-08-30T07:00:00", TrainingLoad: 2},
		{Type: "Yoga", StartDateLocal: "2026-09-01T07:00:00", TrainingLoad: 10},
		{Type: "Pilates", StartDateLocal: "2026-09-01T08:00:00", TrainingLoad: 10},
		{Type: "Ride", StartDateLocal: "2026-09-02T08:00:00", TrainingLoad: 50},
	}
	classes, conflicts := icu.ClassifyFitnessLoadTypes(icu.LearnFitnessLoadTypes(wellness, activities))
	want := map[string]string{"Walk": "ctl", "VirtualRun": "ctl", "WeightTraining": "atl-only"}
	if len(classes) != len(want) || len(conflicts) != 0 {
		t.Fatalf("classes = %v, conflicts %v", classes, conflicts)
	}
	for sport, class := range want {
		if classes[sport] != class {
			t.Fatalf("classes = %v", classes)
		}
	}
}

func TestClassifyFitnessLoadTypesReportsConflicts(t *testing.T) {
	t.Parallel()
	classes, conflicts := icu.ClassifyFitnessLoadTypes(map[string]icu.FitnessLoadEvidence{
		"Row": {CTLDays: 2, ATLOnlyDays: 1}, "Ride": {CTLDays: 3},
	})
	if classes["Ride"] != "ctl" || len(classes) != 1 || len(conflicts) != 1 || conflicts[0] != "Row" {
		t.Fatalf("classes %v conflicts %v", classes, conflicts)
	}
}

func TestResolveFitnessLoadTypesPrecedenceAndUnknown(t *testing.T) {
	t.Parallel()
	excluded, warnings := icu.ResolveFitnessLoadTypes(
		map[string]string{"Workout": "ctl", "Row": "atl-only"},
		map[string]string{"WeightTraining": "atl-only"},
		map[string]string{"Workout": "atl-only", "Pilates": "atl-only"},
		[]string{"Row", "Swim"},
		[]string{"Yoga", "Yoga", "Ride", "", "WeightTraining"},
	)
	if !excluded["WeightTraining"] || !excluded["Pilates"] || !excluded["Row"] || excluded["Workout"] ||
		excluded["Yoga"] || len(excluded) != 3 {
		t.Fatalf("excluded = %v", excluded)
	}
	// Row is manually set, so only Swim's conflict is reported; Ride and Yoga
	// have no class at all.
	if len(warnings) != 3 || !strings.Contains(strings.Join(warnings, "|"), "Swim") ||
		!strings.Contains(warnings[2], "Yoga") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestLearnFitnessLoadTypesSkipsDaysOverTypeCap(t *testing.T) {
	t.Parallel()
	build := func(types int) ([]icu.Wellness, []icu.Activity) {
		var activities []icu.Activity
		for index := range types {
			activities = append(activities, icu.Activity{
				Type: fmt.Sprintf("Sport%02d", index), StartDateLocal: "2026-09-01T07:00:00", TrainingLoad: 1 << index,
			})
		}
		total := float64(int(1)<<types - 1)

		return []icu.Wellness{{ID: "2026-09-01", CTLLoad: total, ATLLoad: total}}, activities
	}
	wellness, activities := build(16)
	if got := icu.LearnFitnessLoadTypes(wellness, activities); len(got) != 16 {
		t.Fatalf("16 types: %d classified", len(got))
	}
	wellness, activities = build(17)
	if got := icu.LearnFitnessLoadTypes(wellness, activities); len(got) != 0 {
		t.Fatalf("17 types (over the cap) classified %d", len(got))
	}
}
