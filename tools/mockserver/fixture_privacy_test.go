package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

// epochTimeLine is a pretty-printed `"timestamp"`/`"startTime"` line holding
// a 13-digit epoch in milliseconds (the Zepp real-shape fixtures, whose
// dates are synthetic: 2030 or later).
const (
	synthEpochMinMs = 1893456000000 // 2030-01-01T00:00:00Z
	synthEpochMaxMs = 4102444800000 // 2100-01-01T00:00:00Z
)

var epochTimeLine = regexp.MustCompile(`^\s*"(timestamp|startTime)": ([0-9]{13}),?$`)

// isEpochTimeLine reports whether line is only a time key with an epoch-ms
// value in the synthetic window [2030-01-01, 2100-01-01) UTC: a made-up time,
// not an external id or a real date.
func isEpochTimeLine(line string) bool {
	m := epochTimeLine.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	ms, err := strconv.ParseInt(m[2], 10, 64)

	return err == nil && ms >= synthEpochMinMs && ms < synthEpochMaxMs
}

// TestCommittedFixturesHaveNoExternalIDsOrRealWeight scans every committed
// JSON under testdata: the mock fixtures, the analysis corpus
// (wave2_live_corpus.json) and its goldens.
func TestCommittedFixturesHaveNoExternalIDsOrRealWeight(t *testing.T) {
	root, err := os.OpenRoot(filepath.Join("..", "..", "testdata"))
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

		checkFixtureName(t, path)

		// The real athlete number must never reappear.
		if strings.Contains(string(body), "445643") {
			t.Errorf("%s: contains the real athlete number", path)
		}

		checkDenylist(t, path, body, deniedTokens())

		for _, line := range strings.Split(string(body), "\n") {
			if match := longID.FindStringSubmatch(line); match != nil && !isEpochTimeLine(line) {
				t.Errorf("%s: numeric id of 10+ digits %s", path, match[2])
			}
		}

		var value any
		if err := decodeJSON(body, &value); err != nil {
			t.Fatalf("%s: %v", path, err)
		}

		walkWeights(t, path, value)
		walkPrivacy(t, path, "", value)
		walkClockTimes(t, path, value)

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
	for _, line := range []string{`"19999999999"`, `"id": 19999999999,`, `19999999999`, `"id": -19999999999,`} {
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
	in := `{"list":[{"activity_id":["19999999999","i900000001"],"weight":84.2,` +
		`"watts":[364],"watts_per_kg":[4.32]}],"activities":{"19999999999":{"id":"19999999999",` +
		`"name":"Zwift - Real Ride","icu_weight":84.2,"start_date_local":"2025-11-10T18:00:00"},` +
		`"i900000001":{"id":"i900000001","name":"Private","start_date_local":"2025-11-01T08:00:00"}}}`

	out := string(anonSalt(0).anonymizeJSON([]byte(in)))
	for _, leaked := range []string{"19999999999", "Real Ride", "Private", "84.2", "4.32"} {
		if strings.Contains(out, leaked) {
			t.Errorf("leaks %s: %s", leaked, out)
		}
	}

	want := anonSalt(0).remapExternalID("19999999999")
	for _, fragment := range []string{
		`"` + want + `":{`, `"id":"` + want + `"`, `"2025-11-01T08:00:00"`,
		`"2025-11-10T18:00:00"`, `"weight":72.8`, `"watts_per_kg":[5]`, `"name":"Activity ` + want + `"`,
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("missing %s: %s", fragment, out)
		}
	}

	if again := string(anonSalt(0).anonymizeJSON([]byte(out))); again != out {
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

			if group, ok := inner.(string); ok && key == "group" && realGroupHash.MatchString(group) {
				t.Errorf("%s: group = %q is a captured group hash", path, group)
			}

			if text, ok := inner.(string); ok && athleteIDShape.MatchString(text) {
				t.Errorf("%s: %s = %q looks like a real athlete id", path, key, text)
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

		checkActivityText(t, path, typed)
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

		if isRealPrefixedID(typed) {
			t.Errorf("%s: %s = %q is a real activity/gear id", path, key, typed)
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

func TestCheckFixtureName(t *testing.T) {
	for path, want := range map[string]int{
		"streams_i123456789.json":                 1,
		"streams_i686255833.json":                 0,
		"api/v1/activity/i900000001/streams.json": 0,
		"wellness_replay.json":                    0,
	} {
		rec := &recordingTB{}
		checkFixtureName(rec, path)

		if rec.errors != want {
			t.Errorf("%s: %d errors, want %d", path, rec.errors, want)
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
		"group hash":  `{"group":"abcdef01"}`,
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
		`"activities":{"i1":{"name":"Activity i1"}},"group":"mkf40142"}`), &clean); err != nil {
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

// fileID matches an activity id inside a file name (gear ids never name
// files, and "b892133" in a golden name is a git commit).
var fileID = regexp.MustCompile(`i\d{6,}`)

// checkFixtureName flags a testdata file whose name carries a real
// (not synthetic) activity or gear id.
func checkFixtureName(t reporter, path string) {
	t.Helper()

	for _, id := range fileID.FindAllString(path, -1) {
		if isRealPrefixedID(id) {
			t.Errorf("%s: file name carries real id %s", path, id)
		}
	}
}

// athleteIDShape matches real intervals.icu athlete ids ("i" + 5-7 digits);
// the anonymized athlete is "i0" and synthetic activity ids have 9 digits.
var athleteIDShape = regexp.MustCompile(`^i\d{5,7}$`)

// checkActivityText flags free text on activity records: names must be
// "Activity i<digits>", device and power meter names the placeholder, and
// descriptions and OAuth clients empty.
func checkActivityText(t reporter, path string, obj map[string]any) {
	t.Helper()

	if !isActivityObject(obj) {
		return
	}

	if name, ok := obj["name"].(string); ok && !curveName.MatchString(name) {
		t.Errorf("%s: activity %v name %q", path, obj["id"], name)
	}

	for key, want := range activityText() {
		if value, ok := obj[key]; ok && value != nil && value != want {
			t.Errorf("%s: activity %v %s = %v, want %v", path, obj["id"], key, value, want)
		}
	}
}

func TestRemapGroupHashKeepsGroupingAndIsIdempotent(t *testing.T) {
	a, b := anonSalt(0).remapGroupHash("abcdef01"), anonSalt(0).remapGroupHash("abcdef02")
	if a != anonSalt(0).remapGroupHash("abcdef01") || a == b || realGroupHash.MatchString(a) {
		t.Fatalf("remap %q %q", a, b)
	}

	var value any
	if err := decodeJSON([]byte(`[{"group":"abcdef01"},{"group":"abcdef01"},{"group":"`+a+`"}]`), &value); err != nil {
		t.Fatal(err)
	}

	items, _ := anonSalt(0).anonymize(value).([]any)
	for _, item := range items {
		obj, _ := item.(map[string]any)
		if obj["group"] != a {
			t.Fatalf("group = %v, want %s", obj["group"], a)
		}
	}
}

// deniedTokens holds the sha256 of captured values (activity and Strava ids,
// group hashes) that must never reappear in testdata. Only hashes are kept
// so the values themselves are not written anywhere in the repository.
func deniedTokens() map[string]bool {
	return map[string]bool{
		"fef403a235e54861d438f752ba9d37bfefd4958a2a09c31d9adc206e880b8819": true,
		"bd4cc61ea916a335e4e564910d16cd459118e37e805ce69f4bed79d6b60c33f9": true,
		"13e46c7a14b4f64e269132a56efcd199b20f52d6467e836553c2eff86e2f5243": true,
		"8de07dd5015d0be7122ad1d4f39e47636dcda3ddd3267700ce5227060628a989": true,
		"516cc28ded93fc242454d3c51f086906a2aea66f7ea32d17943efbb686069d4d": true,
		"182040b7d0906359a5ec65293b88e5f114040e53c6c246fe87e13a4bce61b6b0": true,
	}
}

// hexToken matches digit runs and hex tokens. Text is lowercased first, so
// a leading "i" (activity ids) or "-" (negative ids) is never part of a
// token: "i123" and "-123" both normalize to "123".
var hexToken = regexp.MustCompile(`[0-9a-f]{6,}`)

// checkDenylist hashes every normalized token of body and flags denied ones.
func checkDenylist(t reporter, path string, body []byte, denied map[string]bool) {
	t.Helper()

	for _, token := range hexToken.FindAll(bytes.ToLower(body), -1) {
		sum := sha256.Sum256(token)
		if denied[hex.EncodeToString(sum[:])] {
			t.Errorf("%s: contains a denylisted captured value (sha256 %x)", path, sum)
		}
	}
}

// walkClockTimes flags any timestamp that is not on the hour: the
// anonymizer truncates every time of day, so a minute or second means a
// captured start time leaked.
func walkClockTimes(t reporter, path string, value any) {
	t.Helper()

	switch typed := value.(type) {
	case map[string]any:
		for _, inner := range typed {
			walkClockTimes(t, path, inner)
		}
	case []any:
		for _, inner := range typed {
			walkClockTimes(t, path, inner)
		}
	case string:
		if roundTimeOfDay(typed) != typed {
			t.Errorf("%s: timestamp %q is not on the hour", path, typed)
		}
	}
}

func TestDenylistAndClockTimes(t *testing.T) {
	body := []byte(`{"id":"i19999999999","x":"-ABCDEF01","y":"abcdef01","z":123456}`)

	rec := &recordingTB{}
	checkDenylist(rec, "case", body, deniedTokens())

	if rec.errors != 0 {
		t.Fatalf("fake values flagged %d times", rec.errors)
	}

	fake := map[string]bool{}
	for _, value := range []string{"19999999999", "abcdef01"} {
		sum := sha256.Sum256([]byte(value))
		fake[hex.EncodeToString(sum[:])] = true
	}

	checkDenylist(rec, "case", body, fake)

	if rec.errors != 3 { // i-prefixed id, upper-case negative hex, plain hex
		t.Fatalf("denylisted values flagged %d times, want 3", rec.errors)
	}

	rec = &recordingTB{}

	var value any
	if err := decodeJSON([]byte(`["2026-09-01T08:00:00","2026-09-01T08:13:00","2026-09-01T08:00:07Z",`+
		`"2026-09-01",{"a":["2026-09-01T09:30:00.000+00:00"]}]`), &value); err != nil {
		t.Fatal(err)
	}

	walkClockTimes(rec, "case", value)

	if rec.errors != 3 {
		t.Fatalf("clock times flagged %d times, want 3", rec.errors)
	}
}

func TestIsEpochTimeLine(t *testing.T) {
	t.Parallel()
	for line, want := range map[string]bool{
		`      "timestamp": 1894622400000,`:                     true,
		`        "startTime": 1894597200000`:                    true,
		`      "timestamp": 9994622400000,`:                     false,
		`      "timestamp": 1792886400000,`:                     false, // 2026: real-looking date
		`      "timestamp": 1893455999999,`:                     false,
		`      "timestamp": 1893456000000,`:                     true,
		`      "deviceId": 1894622400000,`:                      false,
		`      "timestamp": 18946224000,`:                       false,
		`      "timestamp": 1894622400000, "id": 1234567890123`: false,
	} {
		if got := isEpochTimeLine(line); got != want {
			t.Errorf("%q: got %v, want %v", line, got, want)
		}
	}
}

// Minified JSON puts many keys on one line, so the time-line exemption never
// applies and every 10+ digit number is still flagged.
func TestLongIDMinifiedJSONFailsSafe(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		`{"timestamp":1894622400000,"deviceId":1234567890123}`,
		`{"timestamp":1894622400000}`,
		`{"items":[{"startTime":1894597200000,"value":{"sleepHRV":41}}]}`,
	} {
		if longID.FindStringSubmatch(line) == nil || isEpochTimeLine(line) {
			t.Errorf("minified line not flagged: %s", line)
		}
	}
}
