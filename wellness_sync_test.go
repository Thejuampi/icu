package icu

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}

	return loc
}

func TestHRVLocalDayEdges(t *testing.T) {
	t.Parallel()
	ny := mustLoc(t, "America/New_York")
	tokyo := mustLoc(t, "Asia/Tokyo")
	at := func(loc *time.Location, s string) int64 {
		tm, err := time.ParseInLocation("2006-01-02 15:04:05", s, loc)
		if err != nil {
			t.Fatal(err)
		}

		return tm.UnixMilli()
	}
	cases := []struct {
		name string
		ts   int64
		loc  *time.Location
		want string
	}{
		{"local midnight belongs to that day", at(ny, "2026-08-10 00:00:00"), ny, "2026-08-10"},
		{"early morning same day", at(ny, "2026-08-10 04:30:00"), ny, "2026-08-10"},
		{"17:59 stays on the day", at(ny, "2026-08-10 17:59:59"), ny, "2026-08-10"},
		{"18:00 rolls to next day", at(ny, "2026-08-10 18:00:00"), ny, "2026-08-11"},
		{"23:59 rolls to next day", at(ny, "2026-08-10 23:59:59"), ny, "2026-08-11"},
		// 23:30 NY = 03:30 UTC next day: UTC would say the 11th as well, but
		// 01:30 NY = 05:30 UTC on the 10th must stay the 10th.
		{"after midnight NY is still the 10th", at(time.UTC, "2026-08-10 05:30:00"), ny, "2026-08-10"},
		{"03:30Z is previous NY evening -> 10th", at(time.UTC, "2026-08-10 03:30:00"), ny, "2026-08-10"},
		// Tokyo 02:00 on the 10th is 17:00 UTC on the 9th: UTC day would be wrong.
		{"tokyo early morning is not the UTC day", at(tokyo, "2026-08-10 02:00:00"), tokyo, "2026-08-10"},
		{"month rollover", at(ny, "2026-08-31 22:00:00"), ny, "2026-09-01"},
		{"nil location is UTC", at(time.UTC, "2026-08-10 12:00:00"), nil, "2026-08-10"},
		{"DST fall-back night", at(ny, "2026-11-01 01:30:00"), ny, "2026-11-01"},
	}
	for _, tc := range cases {
		if got := HRVLocalDay(tc.ts, tc.loc); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestParseSyncFields(t *testing.T) {
	t.Parallel()
	if got, err := ParseSyncFields(""); err != nil || !reflect.DeepEqual(got, SyncFields()) {
		t.Fatalf("default = %v %v", got, err)
	}
	if got, err := ParseSyncFields(" sleepscore, HRV ,,"); err != nil || !reflect.DeepEqual(got, []string{SyncFieldHRV, SyncFieldSleepScore}) {
		t.Fatalf("got %v %v", got, err)
	}
	if _, err := ParseSyncFields("weight"); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := ParseSyncFields(" , "); err == nil {
		t.Fatal("empty selection accepted")
	}
}

func TestSleepSecondsFromBand(t *testing.T) {
	t.Parallel()
	if SleepSecondsFromBand(nil) != 0 {
		t.Fatal("nil")
	}
	if got := SleepSecondsFromBand(&BandDataSleep{DeepMinutes: 60, LightMinutes: 300}); got != 360*60 {
		t.Fatalf("dp+lt = %d", got)
	}
	stages := []BandDataSleepStage{{Start: 0, Stop: 100, Mode: 4}, {Start: 100, Stop: 130, Mode: sleepModeAwake}, {Start: 130, Stop: 200, Mode: 8}, {Start: 5, Stop: 5, Mode: 5}}
	if got := SleepSecondsFromBand(&BandDataSleep{DeepMinutes: 1, Stages: stages}); got != 170*60 {
		t.Fatalf("stages = %d", got)
	}
}

func TestBuildZeppWellnessDays(t *testing.T) {
	t.Parallel()
	ny := mustLoc(t, "America/New_York")
	sleep := func(rhr, ss int) *BandDataSummary {
		return &BandDataSummary{Sleep: &BandDataSleep{DeepMinutes: 60, LightMinutes: 240, RestingHR: rhr, SleepScore: ss}}
	}
	band := []BandDataDay{
		{Date: "2026-08-09", Summary: sleep(40, 50)}, // out of range
		{Date: "2026-08-10", Summary: sleep(50, 80)},
		{Date: "2026-08-11", Summary: &BandDataSummary{}}, // no sleep
		{Date: "2026-08-12"},                              // no summary
	}
	evening := time.Date(2026, 8, 10, 22, 0, 0, 0, ny).UnixMilli() // -> 11th
	morning := time.Date(2026, 8, 10, 2, 0, 0, 0, ny).UnixMilli()  // -> 10th
	later := time.Date(2026, 8, 10, 5, 0, 0, 0, ny).UnixMilli()    // -> 10th, latest wins
	hrv := []ZeppHRVEvent{
		{Timestamp: later, Value: 44},
		{Timestamp: morning, Value: 40},
		{Timestamp: evening, Value: 55},
		{Timestamp: time.Date(2026, 8, 12, 20, 0, 0, 0, ny).UnixMilli(), Value: 60}, // -> 13th, out of range
		{Timestamp: morning, Value: 0},                                              // ignored
	}
	got := BuildZeppWellnessDays(band, hrv, ny, "2026-08-10", "2026-08-12")
	want := []ZeppWellnessDay{
		{Date: "2026-08-10", RestingHR: 50, HRV: 44, SleepSecs: 300 * 60, SleepScore: 80},
		{Date: "2026-08-11", HRV: 55},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got := BuildZeppWellnessDays(nil, nil, ny, "2026-08-10", "2026-08-12"); len(got) != 0 {
		t.Fatalf("empty = %+v", got)
	}
}

func TestPlanWellnessSyncFillOnlyAndOverwrite(t *testing.T) {
	t.Parallel()
	zepp := []ZeppWellnessDay{
		{Date: "2026-08-10", RestingHR: 50, HRV: 44, SleepSecs: 1000, SleepScore: 80},
		{Date: "2026-08-11", RestingHR: 51, HRV: 45.5},
	}
	existing := []Wellness{{ID: "2026-08-10", RestingHR: 50, HRV: 40}}
	fields := SyncFields()

	fill := PlanWellnessSync(zepp, existing, fields, false, false)
	actions := map[string]string{}
	for _, c := range fill.Changes {
		actions[c.Date+"/"+c.Field] = c.Action
	}
	wantActions := map[string]string{
		"2026-08-10/restingHR": SyncActionSame, "2026-08-10/hrv": SyncActionKeep,
		"2026-08-10/sleepSecs": SyncActionFill, "2026-08-10/sleepScore": SyncActionFill,
		"2026-08-11/restingHR": SyncActionFill, "2026-08-11/hrv": SyncActionFill,
	}
	if !reflect.DeepEqual(actions, wantActions) {
		t.Fatalf("actions %v", actions)
	}
	wantWrites := []WellnessWrite{
		{ID: "2026-08-10", SleepSecs: 1000, SleepScore: 80},
		{ID: "2026-08-11", RestingHR: 51, HRV: 45.5},
	}
	if !reflect.DeepEqual(fill.Writes, wantWrites) {
		t.Fatalf("fill writes %+v", fill.Writes)
	}

	over := PlanWellnessSync(zepp, existing, []string{SyncFieldHRV}, true, true)
	wantOver := []WellnessWrite{{ID: "2026-08-10", HRV: 44, Locked: true}, {ID: "2026-08-11", HRV: 45.5, Locked: true}}
	if !reflect.DeepEqual(over.Writes, wantOver) || over.Changes[0].Action != SyncActionOverwrite {
		t.Fatalf("overwrite %+v", over)
	}

	same := PlanWellnessSync(zepp[:1], []Wellness{{ID: "2026-08-10", RestingHR: 50, HRV: 44, SleepSecs: 1000, SleepScore: 80}}, fields, true, false)
	if len(same.Writes) != 0 {
		t.Fatalf("equal values written: %+v", same.Writes)
	}
}

func TestFormatSyncValue(t *testing.T) {
	t.Parallel()
	if FormatSyncValue(0) != "-" || FormatSyncValue(44.5) != "44.5" || FormatSyncValue(50) != "50" {
		t.Fatal("format")
	}
}

func TestZeppStatusError(t *testing.T) {
	t.Parallel()
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		if err := zeppStatusError(code); !errors.Is(err, ErrZeppUnauthorized) || errors.Is(err, ErrZeppServer) {
			t.Fatalf("%d: %v", code, err)
		}
	}
	if err := zeppStatusError(http.StatusBadGateway); !errors.Is(err, ErrZeppServer) {
		t.Fatalf("502: %v", err)
	}
}

func TestDecodeSleepRestingHRAndScore(t *testing.T) {
	t.Parallel()
	got, err := decodeSleep([]byte(`{"dp":10,"lt":20,"rhr":49,"ss":81}`), "2026-08-10")
	if err != nil || got.RestingHR != 49 || got.SleepScore != 81 {
		t.Fatalf("%+v %v", got, err)
	}
}
