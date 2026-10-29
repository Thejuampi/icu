package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// longID matches an integer of 10+ digits that is not part of a decimal
// fraction, quoted or not (Strava and other external ids).
var longID = regexp.MustCompile(`(^|[^.0-9A-Za-z_-])([0-9]{10,})([^0-9A-Za-z_-]|$)`)

func TestCommittedFixturesHaveNoExternalIDsOrRealWeight(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "fixtures")
	checked := 0

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return walkErr
		}

		body, err := os.ReadFile(path)
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

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if checked == 0 {
		t.Fatal("no fixtures checked")
	}
}

func walkWeights(t *testing.T, path string, value any) {
	t.Helper()

	switch typed := value.(type) {
	case map[string]any:
		for key, inner := range typed {
			if number, ok := inner.(json.Number); ok && isWeightKey(key) && number != anonWeight {
				t.Errorf("%s: %s = %s, want fuzzed %s", path, key, number, anonWeight)
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
	for _, line := range []string{`"16605333088"`, `"id": 16605333088,`, `16605333088`} {
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
	for _, leaked := range []string{"16605333088", "Real Ride", "Private", "84.2", "2025-11-10", "4.32"} {
		if strings.Contains(out, leaked) {
			t.Errorf("leaks %s: %s", leaked, out)
		}
	}

	want := remapExternalID("16605333088")
	for _, fragment := range []string{`"` + want + `":{`, `"id":"` + want + `"`, `"2025-11-01T08:00:00"`,
		`"2025-11-01T18:00:00"`, `"weight":72.8`, `"watts_per_kg":[5]`} {
		if !strings.Contains(out, fragment) {
			t.Errorf("missing %s: %s", fragment, out)
		}
	}

	if again := string(anonymizeJSON([]byte(out))); again != out {
		t.Errorf("not idempotent:\n%s\n%s", out, again)
	}
}
