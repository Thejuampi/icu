package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDateShiftMovesEveryDateByWholeWeeks(t *testing.T) {
	t.Parallel()

	for range 50 {
		shift := randomDateShift()
		if shift.days%daysPerWeek != 0 || shift.days > -minShiftWeeks*daysPerWeek || shift.days < -maxShiftWeeks*daysPerWeek {
			t.Fatalf("days = %d", shift.days)
		}
	}

	shift := dateShift{days: -28, salt: 7}
	for in, want := range map[string]string{
		"2026-10-26":                    "2026-09-28",
		"2026-10-09T02:00:00.000+00:00": "2026-09-11T02:00:00.000+00:00",
		"W1 Sat 2026-06-06 ALT 3h20":    "W1 Sat 2026-05-09 ALT 3h20",
		"12026-10-26":                   "12026-10-26",
		"2026-13-40":                    "2026-13-40",
		"no date":                       "no date",
	} {
		if got := shift.text(in); got != want {
			t.Errorf("text(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDateShiftJittersActivitiesConsistently(t *testing.T) {
	t.Parallel()

	shift := dateShift{days: -21, salt: 42}
	body := `{"a":{"id":"i900000001","distance":40000.5,"icu_distance":40000.5,"moving_time":3600,` +
		`"elapsed_time":3700,"average_speed":11.111,"icu_hr_zone_times":[1800,1800],` +
		`"icu_zone_times":[{"id":"Z1","secs":3600}],"start_date_local":"2026-10-01T08:00:00"},` +
		`"b":{"id":"i900000001","moving_time":3600},` +
		`"icu_date_of_birth":"1990-01-01","2026-10-01":{"id":5,"distance":10}}`

	var raw struct {
		DOB      string             `json:"icu_date_of_birth"`
		A        map[string]any     `json:"a"`
		B        map[string]any     `json:"b"`
		Shifted  map[string]float64 `json:"2026-09-10"`
		Original map[string]float64 `json:"2026-10-01"`
	}
	if err := json.Unmarshal(shift.shiftJSON([]byte(body)), &raw); err != nil {
		t.Fatal(err)
	}

	if raw.DOB != "1990-01-01" {
		t.Errorf("redacted placeholder shifted: %v", raw.DOB)
	}

	if raw.Shifted == nil || raw.Original != nil {
		t.Errorf("date map key not shifted: %+v", raw)
	}

	if raw.A["start_date_local"] != "2026-09-10T08:00:00" {
		t.Errorf("start = %v", raw.A["start_date_local"])
	}

	num := func(key string) float64 {
		value, _ := raw.A[key].(float64)

		return value
	}

	fd, fm := shift.factor("i900000001", channelDistance), shift.factor("i900000001", channelMoving)
	if fd == fm {
		t.Fatalf("distance and moving factors must differ: %v", fd)
	}

	for key, want := range map[string]float64{
		"distance": 40000.5 * fd, "icu_distance": 40000.5 * fd, "moving_time": 3600 * fm,
		"elapsed_time": 3700 * fm, "average_speed": 11.111 * fd / fm,
	} {
		if got := num(key); got < want-1 || got > want+1 {
			t.Errorf("%s = %v, want ~%v", key, got, want)
		}
	}

	if num("moving_time") > num("elapsed_time") {
		t.Errorf("elapsed < moving: %v", raw.A)
	}

	if raw.B["moving_time"] != raw.A["moving_time"] {
		t.Errorf("same activity jittered differently across documents")
	}

	if raw.Shifted["distance"] != 10 {
		t.Errorf("non-activity object jittered")
	}

	if got := string(shift.shiftJSON([]byte("not json"))); got != "not json" {
		t.Errorf("invalid json = %q", got)
	}
}

func TestJitterFactorsAreAtLeastOnePercent(t *testing.T) {
	t.Parallel()

	for salt := range uint32(200) {
		shift := dateShift{salt: salt * 7919}
		for _, id := range []string{"i900000001", "i900000002", "i900000003", "i617302033"} {
			for _, channel := range []string{channelDistance, channelMoving} {
				f := shift.factor(id, channel)
				if d := math.Abs(f - 1); d < 0.01-1e-12 || d > 0.04+1e-12 {
					t.Fatalf("factor(%s,%s) = %v, |f-1| outside [0.01,0.04]", id, channel, f)
				}
			}
		}
	}
}

func TestShiftStreamsScalesWithActivityAndDropsAltitude(t *testing.T) {
	t.Parallel()

	shift := dateShift{salt: 3}
	body := `[{"type":"time","data":[0,1,2]},{"type":"distance","data":[null,10,20.5]},` +
		`{"type":"velocity_smooth","data":[null,5]},{"type":"altitude","data":[12]},{"type":"temp","data":[20]},` +
		`{"type":"watts","data":[100]},"odd"]`

	var streams []struct {
		Type string    `json:"type"`
		Data []float64 `json:"data"`
	}

	out := shift.shiftFile("api/v1/activity/i900000001/streams.json", []byte(body))
	if err := json.Unmarshal([]byte(strings.Replace(string(out), `,"odd"`, "", 1)), &streams); err != nil {
		t.Fatalf("%v: %s", err, out)
	}

	fd, fm := shift.factor("i900000001", channelDistance), shift.factor("i900000001", channelMoving)
	got := map[string][]float64{}

	for _, stream := range streams {
		got[stream.Type] = stream.Data
	}

	if _, ok := got["altitude"]; ok || got["temp"] != nil {
		t.Errorf("altitude/temp kept: %s", out)
	}

	near := func(a, b float64) bool { return math.Abs(a-b) < 0.002 }
	if !near(got["time"][2], 2*fm) || !near(got["distance"][2], 20.5*fd) ||
		!near(got["velocity_smooth"][1], 5*fd/fm) || got["watts"][0] != 100 {
		t.Errorf("streams not scaled with the activity: %s", out)
	}

	if !strings.Contains(string(out), `"odd"`) {
		t.Errorf("non-object stream entry dropped: %s", out)
	}

	if _, ok := streamsActivity("api/v1/activity/i900000001.json"); ok {
		t.Error("detail file matched as streams")
	}
}

func TestAnonymizeDirShiftsOnlyOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "wellness.json")

	if err := os.WriteFile(file, []byte(`[{"id":"2026-10-26","updated":"2026-10-26T20:33:31.877+00:00","weight":84.2}]`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := anonymizeDirWith(dir, dateShift{days: -14}); err != nil {
		t.Fatal(err)
	}

	first, _ := os.ReadFile(file)
	for _, want := range []string{`"2026-10-12"`, `"2026-10-12T20:00:00.000+00:00"`, `72.8`} {
		if !strings.Contains(string(first), want) {
			t.Errorf("missing %s: %s", want, first)
		}
	}

	if err := anonymizeDirWith(dir, dateShift{days: -70}); err != nil {
		t.Fatal(err)
	}

	second, _ := os.ReadFile(file)
	if string(first) != string(second) {
		t.Errorf("second run drifted:\n%s\n%s", first, second)
	}

	if _, err := os.Stat(filepath.Join(dir, shiftMarker)); err != nil {
		t.Errorf("marker: %v", err)
	}
}

func TestRoundTimeOfDay(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"2026-10-09T02:15:49.052+00:00": "2026-10-09T02:00:00.000+00:00",
		"2025-10-20T18:15:25":           "2025-10-20T18:00:00",
		"2026-10-09T02:15Z":             "2026-10-09T02:00Z",
		"2026-10-09":                    "2026-10-09",
		"hello":                         "hello",
	} {
		if got := roundTimeOfDay(in); got != want {
			t.Errorf("roundTimeOfDay(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAnonymizeRemapsLongIDsAndPerKg(t *testing.T) {
	t.Parallel()

	out := string(anonymizeJSON([]byte(`{"id":-123456789,"paired_event_id":800000001,"wkg_activity_id":["987654321","i1"],` +
		`"folder_id":{"x":1},"icu_weight_sync":"81.4","weight_pref_lb":false,"average_watts":136,"average_watts_kg":1.67,` +
		`"eftp":265.9014,"eftpPerKg":3.79,"submax_watts":[[100]],"submax_watts_per_kg":[[1.2]],"max_watts_kg":null,` +
		`"weight_kg":80,"curve":{"activities":{"123456789":{"name":"x"}}}}`)))

	for _, leaked := range []string{"123456789", "987654321", "81.4", "1.67", "3.79", "1.2]"} {
		if strings.Contains(out, leaked) {
			t.Errorf("leaks %s: %s", leaked, out)
		}
	}

	for _, want := range []string{
		`"paired_event_id":800000001`, `"icu_weight_sync":"72.8"`, `"weight_pref_lb":false`, `"average_watts_kg":1.868132`,
		`"eftpPerKg":3.652492`, `"submax_watts_per_kg":[[1.373626]]`, `"max_watts_kg":null`, `"weight_kg":72.8`,
		`"Activity ` + remapExternalID("123456789") + `"`, `"folder_id":{"x":1}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s: %s", want, out)
		}
	}

	if again := string(anonymizeJSON([]byte(out))); again != out {
		t.Errorf("not idempotent:\n%s\n%s", out, again)
	}
}
