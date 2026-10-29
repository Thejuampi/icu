package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCommittedSummaryMatchesActivities is the consistency check: every
// activity-derived total in the committed athlete summary equals the sum of
// the (jittered) activity fixtures in that week.
func TestCommittedSummaryMatchesActivities(t *testing.T) {
	t.Parallel()

	root, err := os.OpenRoot(filepath.Join("..", "..", "testdata", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	activities, err := loadSummaryActivities(root)
	if err != nil || len(activities) == 0 {
		t.Fatalf("activities: %v (%d)", err, len(activities))
	}

	body, err := root.ReadFile("api/v1/athlete/0/athlete-summary.json")
	if err != nil {
		t.Fatal(err)
	}

	var committed, again []any
	if err := decodeJSON(body, &committed); err != nil {
		t.Fatal(err)
	}

	if err := decodeJSON(body, &again); err != nil {
		t.Fatal(err)
	}

	recomputeSummary(again, activities)

	if !reflect.DeepEqual(committed, again) {
		t.Fatal("athlete-summary totals do not match the activity fixtures; re-run icu-mock -anonymize")
	}

	sawActivity := false

	for _, raw := range committed {
		week, _ := raw.(map[string]any)
		movingNum, _ := week["moving_time"].(json.Number)
		zonesNum, _ := week["timeInZonesTot"].(json.Number)
		moving, _ := movingNum.Int64()
		zones, _ := zonesNum.Int64()

		if moving > 0 {
			sawActivity = true

			// Zone times are jittered with moving time; rounding may differ by a few seconds.
			if diff := zones - moving; diff < -10 || diff > 10 {
				t.Errorf("week %v: timeInZonesTot %d vs moving_time %d", week["date"], zones, moving)
			}
		}
	}

	if !sawActivity {
		t.Error("no summary week contains an activity")
	}
}

func TestRecomputeSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()

		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("api/v1/activity/i900000001.json", `{"id":"i900000001","type":"Ride","start_date_local":"2026-10-02T08:00:00",`+
		`"distance":1000.5,"moving_time":100,"elapsed_time":110,"calories":5,"icu_training_load":7,"total_elevation_gain":3,`+
		`"icu_zone_times":[{"id":"Z1","secs":60},{"id":"Z2","secs":40}]}`)
	write("api/v1/activity/i900000002.json", `{"id":"i900000002","type":"Run","start_date_local":"2026-10-12T08:00:00","moving_time":50}`)
	write("api/v1/activity/notes.txt", "x")
	write("api/v1/athlete/0/athlete-summary.json", `[{"date":"2026-09-28","count":9,"distance":1,"timeInZones":[1,1,1],`+
		`"fitness":30,"byCategory":[{"category":"Ride","count":9},{"category":"Run","count":4},"x"]},{"date":"bad"},"x"]`)

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err := recomputeSummaries(root); err != nil {
		t.Fatal(err)
	}

	body, _ := root.ReadFile("api/v1/athlete/0/athlete-summary.json")

	var weeks []any
	if err := json.Unmarshal(body, &weeks); err != nil {
		t.Fatal(err)
	}

	week, _ := weeks[0].(map[string]any)
	categories, _ := week["byCategory"].([]any)
	ride, _ := categories[0].(map[string]any)
	run, _ := categories[1].(map[string]any)

	if week["count"] != 1.0 || week["distance"] != 1000.5 || week["moving_time"] != 100.0 || week["time"] != 100.0 ||
		week["timeInZonesTot"] != 100.0 || week["fitness"] != 30.0 || ride["count"] != 1.0 || run["count"] != 0.0 {
		t.Errorf("recomputed = %s", body)
	}

	empty := t.TempDir()

	emptyRoot, err := os.OpenRoot(empty)
	if err != nil {
		t.Fatal(err)
	}
	defer emptyRoot.Close()

	if err := recomputeSummaries(emptyRoot); err != nil {
		t.Errorf("empty dir: %v", err)
	}
}
