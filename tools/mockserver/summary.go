package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	summarySuffix = "/athlete-summary.json"
	activityDir   = "api/v1/activity"
	// zoneCount is the number of power zones (Z1-Z7); a trailing "SS"
	// (sweet spot) entry overlaps them and is not part of the total.
	zoneCount = 7
)

// summaryActivity is the part of an activity detail fixture that feeds the
// weekly athlete summary.
type summaryActivity struct {
	Type      string      `json:"type"`
	Start     string      `json:"start_date_local"`
	Distance  json.Number `json:"distance"`
	Moving    json.Number `json:"moving_time"`
	Elapsed   json.Number `json:"elapsed_time"`
	Calories  json.Number `json:"calories"`
	Load      json.Number `json:"icu_training_load"`
	Elevation json.Number `json:"total_elevation_gain"`
	Zones     []struct {
		Secs json.Number `json:"secs"`
	} `json:"icu_zone_times"`
}

// summaryTotals are the activity-derived fields of a summary week or
// category; everything else (fitness, eftp, ...) is kept as captured.
type summaryTotals struct {
	count, calories, load, moving, elapsed int64
	distance, elevation                    float64
	zones                                  []int64
}

func (t *summaryTotals) add(activity *summaryActivity) {
	t.count++
	t.distance += numberFloat(activity.Distance)
	t.elevation += numberFloat(activity.Elevation)
	t.moving += int64(numberFloat(activity.Moving))
	t.elapsed += int64(numberFloat(activity.Elapsed))
	t.calories += int64(numberFloat(activity.Calories))
	t.load += int64(numberFloat(activity.Load))

	for i, zone := range activity.Zones {
		for len(t.zones) <= i {
			t.zones = append(t.zones, 0)
		}

		t.zones[i] += int64(numberFloat(zone.Secs))
	}
}

func numberFloat(number json.Number) float64 {
	value, err := number.Float64()
	if err != nil {
		return 0
	}

	return value
}

func formatReal(value float64) json.Number {
	return json.Number(strconv.FormatFloat(roundTo(value, realPrecision), 'f', -1, 64))
}

func roundTo(value, precision float64) float64 {
	return float64(int64(value*precision+0.5)) / precision
}

func (t *summaryTotals) write(obj map[string]any) {
	obj["count"] = json.Number(strconv.FormatInt(t.count, 10))
	obj["distance"] = formatReal(t.distance)
	obj["moving_time"] = json.Number(strconv.FormatInt(t.moving, 10))
	obj["time"] = obj["moving_time"]
	obj["elapsed_time"] = json.Number(strconv.FormatInt(t.elapsed, 10))
	obj["calories"] = json.Number(strconv.FormatInt(t.calories, 10))
	obj["training_load"] = json.Number(strconv.FormatInt(t.load, 10))
	obj["total_elevation_gain"] = formatReal(t.elevation)

	zones, ok := obj["timeInZones"].([]any)
	if !ok {
		return
	}

	var total int64

	for i := range zones {
		var secs int64
		if i < len(t.zones) {
			secs = t.zones[i]
		}

		zones[i] = json.Number(strconv.FormatInt(secs, 10))

		if i < zoneCount {
			total += secs
		}
	}

	obj["timeInZonesTot"] = json.Number(strconv.FormatInt(total, 10))
}

// loadSummaryActivities reads every activity detail fixture
// (api/v1/activity/<id>.json).
func loadSummaryActivities(root *os.Root) ([]summaryActivity, error) {
	entries, err := fs.ReadDir(root.FS(), activityDir)
	if err != nil {
		return nil, nil //nolint:nilerr // no activity fixtures: nothing to sum
	}

	var activities []summaryActivity

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		body, readErr := root.ReadFile(path.Join(activityDir, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("read activity: %w", readErr)
		}

		var activity summaryActivity
		if decodeJSON(body, &activity) == nil && activity.Start != "" {
			activities = append(activities, activity)
		}
	}

	return activities, nil
}

// recomputeSummary rewrites the activity-derived totals of every weekly
// summary from the activity fixtures that fall in that week, so the
// summary matches the (jittered) activities instead of the captured,
// real totals. Idempotent.
func recomputeSummary(weeks []any, activities []summaryActivity) []any {
	for _, raw := range weeks {
		week, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		start, err := time.Parse(time.DateOnly, fmt.Sprint(week["date"]))
		if err != nil {
			continue
		}

		end := start.AddDate(0, 0, daysPerWeek)
		total := summaryTotals{}
		byType := map[string]*summaryTotals{}

		for i := range activities {
			activity := &activities[i]
			day, dayErr := time.Parse(time.DateOnly, activity.Start[:min(len(activity.Start), len(time.DateOnly))])
			if dayErr != nil || day.Before(start) || !day.Before(end) {
				continue
			}

			total.add(activity)

			if byType[activity.Type] == nil {
				byType[activity.Type] = &summaryTotals{}
			}

			byType[activity.Type].add(activity)
		}

		total.write(week)

		categories, _ := week["byCategory"].([]any)
		for _, rawCategory := range categories {
			category, ok := rawCategory.(map[string]any)
			if !ok {
				continue
			}

			totals := summaryTotals{}
			if found := byType[fmt.Sprint(category["category"])]; found != nil {
				totals = *found
			}

			totals.write(category)
		}
	}

	return weeks
}

// recomputeSummaries applies recomputeSummary to every athlete-summary
// fixture under root.
func recomputeSummaries(root *os.Root) error {
	activities, err := loadSummaryActivities(root)
	if err != nil {
		return err
	}

	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix("/"+name, summarySuffix) {
			return walkErr
		}

		body, readErr := root.ReadFile(name)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", name, readErr)
		}

		var weeks []any
		if decodeErr := decodeJSON(body, &weeks); decodeErr != nil {
			return nil //nolint:nilerr // non-array summaries are left as captured
		}

		out, marshalErr := json.Marshal(recomputeSummary(weeks, activities))
		if marshalErr != nil {
			return fmt.Errorf("encode %s: %w", name, marshalErr)
		}

		if writeErr := root.WriteFile(name, prettyJSON(out), 0o600); writeErr != nil {
			return fmt.Errorf("write %s: %w", name, writeErr)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk summaries: %w", err)
	}

	return nil
}
