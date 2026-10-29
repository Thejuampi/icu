package main

import (
	"encoding/json"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// longID matches an integer of 10+ digits that is not part of a decimal
// fraction, quoted or not (Strava and other external ids).
var longID = regexp.MustCompile(`(^|[^.0-9A-Za-z_-]|[^.0-9A-Za-z_]-)(-?[0-9]{10,})([^0-9A-Za-z_-]|$)`)

func TestCommittedFixturesHaveNoExternalIDsOrRealWeight(t *testing.T) {
	root, err := os.OpenRoot(filepath.Join("..", "..", "testdata", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	checked := 0

	err = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return walkErr
		}

		body, err := root.ReadFile(path)
		if err != nil {
			return err
		}

		checked++

		for _, line := range strings.Split(string(body), "\n") {
			if match := longID.FindStringSubmatch(line); match != nil {
				t.Errorf("%s: numeric id of 10+ digits %s", path, match[2])
			}
		}

		var value any
		if err := decodeJSON(body, &value); err != nil {
			t.Fatalf("%s: %v", path, err)
		}

		walkWeights(t, path, value)
		walkPrivacy(t, path, "", value)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if checked == 0 {
		t.Fatal("no fixtures checked")
	}
}

func walkWeights(t reporter, path string, value any) {
	t.Helper()

	switch typed := value.(type) {
	case map[string]any:
		for key, inner := range typed {
			if isWeightKey(key) {
				checkWeight(t, path, key, inner)
			}

			walkWeights(t, path, inner)
		}
	case []any:
		for _, inner := range typed {
			walkWeights(t, path, inner)
		}
	}
}

func TestLongIDPattern(t *testing.T) {
	for _, line := range []string{`"16605333088"`, `"id": 16605333088,`, `16605333088`, `"id": -16605333088,`} {
		if !longID.MatchString(line) {
			t.Errorf("missed %q", line)
		}
	}

	for _, line := range []string{`"vi": 1.0482456140350878,`, `"id": "i812345678"`, `"x": 123456789`} {
		if longID.MatchString(line) {
			t.Errorf("false positive %q", line)
		}
	}
}

func TestAnonymizeCurveActivities(t *testing.T) {
	in := `{"list":[{"activity_id":["16605333088","i900000001"],"weight":84.2,` +
		`"watts":[364],"watts_per_kg":[4.32]}],"activities":{"16605333088":{"id":"16605333088",` +
		`"name":"Zwift - Real Ride","icu_weight":84.2,"start_date_local":"2025-11-10T18:00:00"},` +
		`"i900000001":{"id":"i900000001","name":"Private","start_date_local":"2025-11-01T08:00:00"}}}`

	out := string(anonymizeJSON([]byte(in)))
	for _, leaked := range []string{"16605333088", "Real Ride", "Private", "84.2", "4.32"} {
		if strings.Contains(out, leaked) {
			t.Errorf("leaks %s: %s", leaked, out)
		}
	}

	want := remapExternalID("16605333088")
	for _, fragment := range []string{
		`"` + want + `":{`, `"id":"` + want + `"`, `"2025-11-01T08:00:00"`,
		`"2025-11-10T18:00:00"`, `"weight":72.8`, `"watts_per_kg":[5]`, `"name":"Activity ` + want + `"`,
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("missing %s: %s", fragment, out)
		}
	}

	if again := string(anonymizeJSON([]byte(out))); again != out {
		t.Errorf("not idempotent:\n%s\n%s", out, again)
	}
}

// checkWeight flags any weight that is not the fuzzed one, numbers and
// numeric strings alike; non-numeric strings ("NONE") and bools are fine.
func checkWeight(t reporter, path, key string, value any) {
	t.Helper()

	switch typed := value.(type) {
	case json.Number:
		if typed != anonWeight {
			t.Errorf("%s: %s = %s, want fuzzed %s", path, key, typed, anonWeight)
		}
	case string:
		if _, err := strconv.ParseFloat(typed, 64); err == nil && typed != string(anonWeight) {
			t.Errorf("%s: %s = %q, want fuzzed %s", path, key, typed, anonWeight)
		}
	case []any:
		for _, inner := range typed {
			checkWeight(t, path, key, inner)
		}
	}
}

// curveName is the only name a power/HR curve activity may carry.
var curveName = regexp.MustCompile(`^Activity i\d+$`)

// walkPrivacy flags 9+ digit ids (optionally negative) in id-like keys and
// in map keys, curve activity names that are not synthetic, and per-kg
// values that do not match the fuzzed weight.
func walkPrivacy(t reporter, path, parent string, value any) {
	t.Helper()

	switch typed := value.(type) {
	case map[string]any:
		for key, inner := range typed {
			if isLongNumericID(key) {
				t.Errorf("%s: 9+ digit id as map key %s", path, key)
			}

			if isIDKey(key) {
				checkIDValue(t, path, key, inner)
			}

			if parent == "activities" && key != "" {
				if activity, ok := inner.(map[string]any); ok {
					if name, ok := activity["name"].(string); ok && !curveName.MatchString(name) {
						t.Errorf("%s: curve activity name %q", path, name)
					}
				}
			}

			if base, ok := perKgBase(key); ok {
				checkPerKg(t, path, key, typed[base], inner)
			}

			walkPrivacy(t, path, key, inner)
		}
	case []any:
		for _, inner := range typed {
			walkPrivacy(t, path, parent, inner)
		}
	}
}

func checkIDValue(t reporter, path, key string, value any) {
	t.Helper()

	switch typed := value.(type) {
	case json.Number:
		if isLongNumericID(string(typed)) {
			t.Errorf("%s: %s = %s is a 9+ digit id", path, key, typed)
		}
	case string:
		if isLongNumericID(typed) {
			t.Errorf("%s: %s = %q is a 9+ digit id", path, key, typed)
		}
	case []any:
		for _, inner := range typed {
			checkIDValue(t, path, key, inner)
		}
	}
}

// wkgTolerance allows for the 6-decimal rounding of per-kg values.
const wkgTolerance = 0.01

// checkPerKg asserts watts / per-kg ≈ the fuzzed weight wherever both are
// present and non-zero, so the real weight cannot be recovered.
func checkPerKg(t reporter, path, key string, power, perKg any) {
	t.Helper()

	switch typed := perKg.(type) {
	case json.Number:
		watts, ok := power.(json.Number)
		if !ok {
			return
		}

		w, errW := watts.Float64()
		k, errK := typed.Float64()

		if errW != nil || errK != nil || w == 0 || k == 0 {
			return
		}

		want, _ := strconv.ParseFloat(string(anonWeight), 64)
		if got := w / k; math.Abs(got-want) > wkgTolerance {
			t.Errorf("%s: %s implies weight %.3f, want %s", path, key, got, anonWeight)
		}
	case []any:
		powers, ok := power.([]any)
		if !ok || len(powers) != len(typed) {
			return
		}

		for i := range typed {
			checkPerKg(t, path, key, powers[i], typed[i])
		}
	}
}

func TestPrivacyWalkersFlagLeaks(t *testing.T) {
	cases := map[string]string{
		"map key":     `{"123456789":{}}`,
		"negative id": `{"id":-123456789}`,
		"string id":   `{"activity_id":"1234567890"}`,
		"array ids":   `{"wkg_activity_id":["i1","987654321"]}`,
		"weight str":  `{"icu_weight_sync":"81.4"}`,
		"curve name":  `{"activities":{"i1":{"name":"Morning Ride"}}}`,
		"per kg":      `{"average_watts":136,"average_watts_kg":1.67}`,
		"eftp per kg": `{"eftp":265.9,"eftpPerKg":3.79}`,
	}
	for name, body := range cases {
		var value any
		if err := decodeJSON([]byte(body), &value); err != nil {
			t.Fatal(err)
		}

		rec := &recordingTB{}
		scanPrivacy(rec, value)

		if rec.errors == 0 {
			t.Errorf("%s: %s not flagged", name, body)
		}
	}

	var clean any
	if err := decodeJSON([]byte(`{"id":800000001,"paired_event_id":700012345,"icu_weight_sync":"NONE",`+
		`"icu_weighted_avg_watts":200,"weight":72.8,"watts":[364],"watts_per_kg":[5],`+
		`"activities":{"i1":{"name":"Activity i1"}}}`), &clean); err != nil {
		t.Fatal(err)
	}

	rec := &recordingTB{}
	scanPrivacy(rec, clean)

	if rec.errors != 0 {
		t.Errorf("clean fixture flagged %d times", rec.errors)
	}
}

// reporter is the subset of testing.T the privacy walkers need, so the
// walkers themselves can be tested against a recording fake.
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

type recordingTB struct{ errors int }

func (*recordingTB) Helper() {}

func (r *recordingTB) Errorf(string, ...any) { r.errors++ }

func scanPrivacy(t reporter, value any) {
	t.Helper()
	walkWeights(t, "case", value)
	walkPrivacy(t, "case", "", value)
}
