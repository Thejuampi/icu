package icu_test

import (
	"testing"
	"time"
	_ "time/tzdata" // zone data on any CI image

	icu "github.com/Thejuampi/icu"
)

// 23:30 EDT on Oct 27 is 03:30 UTC on Oct 28: without start_date_local the
// day must come from start_date in the athlete's timezone.
func TestActivityLocalDateNewYorkLateEvening(t *testing.T) {
	t.Parallel()
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	late := icu.Activity{StartDate: "2026-10-28T03:30:00Z"}
	if got := icu.ActivityLocalDate(&late, newYork); got != "2026-10-27" {
		t.Fatalf("NY local date = %s, want 2026-10-27", got)
	}
	if got := icu.ActivityLocalDate(&late, nil); got != "2026-10-28" {
		t.Fatalf("UTC date = %s", got)
	}
	local := icu.Activity{StartDateLocal: "2026-10-27T23:30:00", StartDate: "2026-10-28T03:30:00Z"}
	if got := icu.ActivityLocalDate(&local, time.UTC); got != "2026-10-27" {
		t.Fatalf("start_date_local wins: %s", got)
	}
	bad := icu.Activity{StartDate: "2026-10-28 junk"}
	if got := icu.ActivityLocalDate(&bad, newYork); got != "2026-10-28" {
		t.Fatalf("unparseable fallback = %s", got)
	}
	if got := icu.ActivityLocalDate(&icu.Activity{}, newYork); got != "" {
		t.Fatalf("empty = %q", got)
	}
	activities := []icu.Activity{late, local, bad}
	icu.LocalizeActivityDates(activities, newYork)
	if activities[0].StartDateLocal != "2026-10-27T23:30:00" || activities[1].StartDateLocal != "2026-10-27T23:30:00" ||
		activities[2].StartDateLocal != "" {
		t.Fatalf("localized = %+v", activities)
	}
	analysis := icu.AnalyzeCyclingActivities([]icu.Activity{{ID: "a", Type: "Ride", StartDate: "2026-10-28T03:30:00Z"}}, icu.AnalysisOptions{})
	if len(analysis.Sessions) != 1 || analysis.Sessions[0].Date != "2026-10-28" {
		t.Fatalf("library default stays UTC: %+v", analysis.Sessions)
	}
}

// LocalizeActivityDates keeps an existing start_date_local even when it
// disagrees with start_date in the given timezone (the API value wins).
func TestLocalizeActivityDatesKeepsExistingLocal(t *testing.T) {
	t.Parallel()
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	activities := []icu.Activity{{StartDateLocal: "2026-10-28T05:30:00", StartDate: "2026-10-28T03:30:00Z"}}
	icu.LocalizeActivityDates(activities, newYork)
	if activities[0].StartDateLocal != "2026-10-28T05:30:00" || icu.ActivityLocalDate(&activities[0], newYork) != "2026-10-28" {
		t.Fatalf("existing local overwritten: %+v", activities[0])
	}
}
