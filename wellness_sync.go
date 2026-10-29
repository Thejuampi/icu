package icu

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Wellness fields `icu wellness sync` can copy from Zepp, by their
// intervals.icu wire names.
const (
	SyncFieldRestingHR  = "restingHR"
	SyncFieldHRV        = "hrv"
	SyncFieldSleepSecs  = "sleepSecs"
	SyncFieldSleepScore = "sleepScore"
)

// Actions in a wellness sync plan.
const (
	SyncActionFill      = "fill"      // intervals.icu has no value: written.
	SyncActionOverwrite = "overwrite" // differs and --overwrite: written.
	SyncActionKeep      = "keep"      // differs, fill-only mode: not written.
	SyncActionSame      = "same"      // already equal: not written.
)

// HRVEveningCutoffHour is the local hour from which an HRV reading counts as
// the start of the night and belongs to the next calendar day.
const HRVEveningCutoffHour = 18

// syncEpsilon treats float values this close as equal.
const syncEpsilon = 1e-9

// SyncFields lists every syncable field in output order.
func SyncFields() []string {
	return []string{SyncFieldRestingHR, SyncFieldHRV, SyncFieldSleepSecs, SyncFieldSleepScore}
}

// ParseSyncFields parses a --fields CSV (empty = all fields), rejecting
// unknown names and keeping SyncFields order.
func ParseSyncFields(csv string) ([]string, error) {
	if strings.TrimSpace(csv) == "" {
		return SyncFields(), nil
	}

	want := map[string]bool{}
	for _, raw := range strings.Split(csv, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		found := false
		for _, field := range SyncFields() {
			if strings.EqualFold(name, field) {
				want[field], found = true, true
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown field %q (valid: %s)", name, strings.Join(SyncFields(), ","))
		}
	}

	out := []string{}
	for _, field := range SyncFields() {
		if want[field] {
			out = append(out, field)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no fields selected (valid: %s)", strings.Join(SyncFields(), ","))
	}

	return out, nil
}

// HRVLocalDay maps a Zepp HRV reading (epoch millis) to the wellness day it
// belongs to, in the athlete's timezone loc: the local calendar date of the
// reading, except readings from HRVEveningCutoffHour (18:00) to midnight,
// which belong to the next day (the night that ends that morning). Days are
// local calendar days, never UTC (issue #29): 23:30 in New York is already
// the next UTC day but stays a New York evening, so it maps to tomorrow by
// the evening rule, not by the UTC rollover.
func HRVLocalDay(timestampMillis int64, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	local := time.UnixMilli(timestampMillis).In(loc)
	if local.Hour() >= HRVEveningCutoffHour {
		local = local.AddDate(0, 0, 1)
	}

	return local.Format(time.DateOnly)
}

// ZeppWellnessDay is one local day of Zepp wellness values; zero = no value.
type ZeppWellnessDay struct {
	Date       string  `json:"date"`
	RestingHR  int     `json:"restingHR,omitempty"`
	HRV        float64 `json:"hrv,omitempty"`
	SleepSecs  int     `json:"sleepSecs,omitempty"`
	SleepScore float64 `json:"sleepScore,omitempty"`
}

const (
	sleepModeAwake = 7
	secondsPerMin  = 60
)

// SleepSecondsFromBand returns the asleep time of a band sleep record: the
// sum of non-awake stages when stages exist, else deep+light minutes.
func SleepSecondsFromBand(sleep *BandDataSleep) int {
	if sleep == nil {
		return 0
	}
	if len(sleep.Stages) == 0 {
		return (sleep.DeepMinutes + sleep.LightMinutes) * secondsPerMin
	}
	total := 0
	for _, stage := range sleep.Stages {
		if stage.Mode != sleepModeAwake && stage.Stop > stage.Start {
			total += stage.Stop - stage.Start
		}
	}

	return total * secondsPerMin
}

// BuildZeppWellnessDays merges band days (resting HR, sleep: Zepp already
// buckets them by the watch's local day in date_time, the night that ends
// that morning) and HRV readings (mapped with HRVLocalDay) into one record
// per day within [oldest, newest]. When several HRV readings map to one
// day the latest wins. Output is sorted by date.
func BuildZeppWellnessDays(band []BandDataDay, hrv []ZeppHRVEvent, loc *time.Location, oldest, newest string) []ZeppWellnessDay {
	days := map[string]*ZeppWellnessDay{}
	get := func(date string) *ZeppWellnessDay {
		if days[date] == nil {
			days[date] = &ZeppWellnessDay{Date: date}
		}

		return days[date]
	}

	for i := range band {
		date := band[i].Date
		if !inDateRange(date, oldest, newest) || band[i].Summary == nil || band[i].Summary.Sleep == nil {
			continue
		}
		sleep := band[i].Summary.Sleep
		day := get(date)
		day.RestingHR = sleep.RestingHR
		day.SleepSecs = SleepSecondsFromBand(sleep)
		day.SleepScore = float64(sleep.SleepScore)
	}

	latest := map[string]int64{}
	for _, reading := range hrv {
		if reading.Value <= 0 {
			continue
		}
		date := HRVLocalDay(reading.Timestamp, loc)
		if !inDateRange(date, oldest, newest) {
			continue
		}
		if ts, seen := latest[date]; seen && ts > reading.Timestamp {
			continue
		}
		latest[date] = reading.Timestamp
		get(date).HRV = reading.Value
	}

	out := make([]ZeppWellnessDay, 0, len(days))
	for _, day := range days {
		if day.RestingHR != 0 || day.HRV != 0 || day.SleepSecs != 0 || day.SleepScore != 0 {
			out = append(out, *day)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })

	return out
}

func inDateRange(date, oldest, newest string) bool {
	return date >= oldest && date <= newest
}

// WellnessSyncChange is one (day, field) row of a sync plan.
type WellnessSyncChange struct {
	Date      string  `json:"date"`
	Field     string  `json:"field"`
	Intervals float64 `json:"intervals"`
	Zepp      float64 `json:"zepp"`
	Action    string  `json:"action"`
}

// WellnessSyncPlan is the diff between Zepp and intervals.icu and the
// wellness-bulk body that applies it.
type WellnessSyncPlan struct {
	Changes []WellnessSyncChange `json:"changes"`
	Writes  []WellnessWrite      `json:"writes"`
}

// PlanWellnessSync compares Zepp days with existing intervals.icu records.
// Fill-only (overwrite=false) writes a field only when intervals.icu has no
// value; overwrite also replaces differing values. lock sets locked on every
// written record so intervals.icu's own Zepp sync does not overwrite it.
func PlanWellnessSync(zepp []ZeppWellnessDay, existing []Wellness, fields []string, overwrite, lock bool) WellnessSyncPlan {
	byDate := map[string]Wellness{}
	for i := range existing {
		byDate[existing[i].ID] = existing[i]
	}

	plan := WellnessSyncPlan{Changes: []WellnessSyncChange{}, Writes: []WellnessWrite{}}
	for _, day := range zepp {
		current := byDate[day.Date]
		write := WellnessWrite{ID: day.Date, Locked: lock}
		dirty := false
		for _, field := range fields {
			newValue, oldValue := syncFieldValue(day, current, field)
			if newValue == 0 {
				continue
			}
			action := syncAction(oldValue, newValue, overwrite)
			plan.Changes = append(plan.Changes, WellnessSyncChange{Date: day.Date, Field: field, Intervals: oldValue, Zepp: newValue, Action: action})
			if action == SyncActionFill || action == SyncActionOverwrite {
				setSyncField(&write, field, newValue)
				dirty = true
			}
		}
		if dirty {
			plan.Writes = append(plan.Writes, write)
		}
	}

	return plan
}

func syncAction(oldValue, newValue float64, overwrite bool) string {
	switch {
	case oldValue == 0:
		return SyncActionFill
	case math.Abs(oldValue-newValue) < syncEpsilon:
		return SyncActionSame
	case overwrite:
		return SyncActionOverwrite
	default:
		return SyncActionKeep
	}
}

func syncFieldValue(day ZeppWellnessDay, current Wellness, field string) (float64, float64) { //nolint:gocritic // (zepp, intervals)
	switch field {
	case SyncFieldRestingHR:
		return float64(day.RestingHR), float64(current.RestingHR)
	case SyncFieldHRV:
		return day.HRV, current.HRV
	case SyncFieldSleepSecs:
		return float64(day.SleepSecs), float64(current.SleepSecs)
	default:
		return day.SleepScore, current.SleepScore
	}
}

func setSyncField(write *WellnessWrite, field string, value float64) {
	switch field {
	case SyncFieldRestingHR:
		write.RestingHR = int(value)
	case SyncFieldHRV:
		write.HRV = value
	case SyncFieldSleepSecs:
		write.SleepSecs = int(value)
	default:
		write.SleepScore = value
	}
}

// FormatSyncValue renders a plan value for the table ("-" for none).
func FormatSyncValue(value float64) string {
	if value == 0 {
		return "-"
	}

	return strconv.FormatFloat(value, 'f', -1, 64)
}
