package icu_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestProjectLoadMatchesExponentialModel(t *testing.T) {
	t.Parallel()

	loads := map[string]float64{}
	ctl, atl := 50.0, 60.0
	// 2026-10-28 (Wed) .. 2026-11-08 (Sun): one partial week, one full week.
	for _, date := range []string{"2026-10-28", "2026-10-29", "2026-10-31", "2026-11-03", "2026-11-05", "2026-11-07"} {
		loads[date] = 100
	}
	got, err := icu.ProjectLoad(&icu.LoadProjectionInput{
		StartDate: "2026-10-27", StartSource: "wellness", StartCTL: ctl, StartATL: atl,
		EndDate: "2026-11-08", DailyLoads: loads, MaxRampPerWeek: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	for day := 28; day <= 31+8; day++ {
		date := "2026-10-" + twoDigits(day)
		if day > 31 {
			date = "2026-11-" + twoDigits(day-31)
		}
		ctl += (loads[date] - ctl) / 42
		atl += (loads[date] - atl) / 7
	}
	if math.Abs(got.EndCTL-ctl) > 0.01 || math.Abs(got.EndATL-atl) > 0.01 || math.Abs(got.EndTSB-(ctl-atl)) > 0.02 {
		t.Fatalf("end = %v/%v/%v, want %v/%v", got.EndCTL, got.EndATL, got.EndTSB, ctl, atl)
	}
	if got.StartTSB != -10 || got.CTLDays != 42 || got.ATLDays != 7 || got.EndDate != "2026-11-08" {
		t.Fatalf("projection = %+v", got)
	}
	if len(got.Weeks) != 2 || got.Weeks[0].WeekStart != "2026-10-26" || got.Weeks[0].Days != 5 ||
		got.Weeks[1].Days != 7 || got.Weeks[1].Load != 300 {
		t.Fatalf("weeks = %+v", got.Weeks)
	}
	first := got.Weeks[0]
	if math.Abs(first.RampPerWeek-first.Ramp*7/5) > 0.02 || !first.OverCap {
		t.Fatalf("first week = %+v, want scaled ramp over the 1/week cap", first)
	}
	if len(got.Warnings) == 0 || !strings.Contains(got.Warnings[0], "week 2026-10-26 CTL ramp") {
		t.Fatalf("warnings = %v", got.Warnings)
	}

	noCap, _ := icu.ProjectLoad(&icu.LoadProjectionInput{
		StartDate: "2026-10-27", StartCTL: 50, StartATL: 60, EndDate: "2026-11-08", DailyLoads: loads,
		CTLDays: 30, ATLDays: 5,
	})
	if len(noCap.Warnings) != 0 || noCap.CTLDays != 30 || noCap.Weeks[0].OverCap {
		t.Fatalf("no-cap projection = %+v", noCap)
	}
	if _, err := icu.ProjectLoad(&icu.LoadProjectionInput{StartDate: "x", EndDate: "2026-11-08"}); err == nil {
		t.Fatal("bad start accepted")
	}
	if _, err := icu.ProjectLoad(&icu.LoadProjectionInput{StartDate: "2026-11-08", EndDate: "y"}); err == nil {
		t.Fatal("bad end accepted")
	}
}

func twoDigits(value int) string {
	return fmt.Sprintf("%02d", value)
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
