package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	upstreamTimeout = 60 * time.Second
	fixtureDirPerm  = 0o750
	maxRedirects    = 10
)

// recorder is the one-time fixture capture proxy. It forwards GET requests
// only; any other method is refused locally and never reaches upstream.
type recorder struct {
	mu       sync.Mutex
	upstream string
	dir      string
	client   *http.Client
	log      io.Writer
	shift    dateShift
}

func newRecorder(upstream, dir string, log io.Writer) *recorder {
	return &recorder{
		upstream: strings.TrimRight(upstream, "/"),
		dir:      dir,
		client:   &http.Client{Timeout: upstreamTimeout, CheckRedirect: sameOriginRedirect},
		log:      log,
		shift:    randomDateShift(),
	}
}

func (rec *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fmt.Fprintf(rec.log, "icu-mock record: refused %s %s (GET only)\n", r.Method, r.URL.Path)
		writeError(w, http.StatusMethodNotAllowed, "record mode is GET-only")

		return
	}

	// The upstream host is fixed by the operator's -upstream flag; only the
	// path/query come from the (local, GET-only) request.
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, rec.upstream+r.URL.RequestURI(), http.NoBody) //nolint:gosec // see above
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())

		return
	}

	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("Accept", "application/json")

	resp, err := rec.client.Do(req) //nolint:gosec // upstream fixed by -upstream
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())

		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())

		return
	}

	if resp.StatusCode == http.StatusOK && json.Valid(body) {
		// One random shift per capture session; captures from different
		// sessions are not date-consistent with each other.
		body = rec.shift.shiftJSON(anonymizeJSON(body))
		if saveErr := rec.save(fixtureKey(r.URL.Path), body); saveErr != nil {
			fmt.Fprintf(rec.log, "icu-mock record: %v\n", saveErr)
		}
	}

	writeJSON(w, resp.StatusCode, body)
}

// save writes the fixture, merging arrays by "id" with an existing file so
// several date-range GETs of the same list accumulate.
func (rec *recorder) save(key string, body []byte) error {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	if err := os.MkdirAll(rec.dir, fixtureDirPerm); err != nil {
		return fmt.Errorf("create fixture dir: %w", err)
	}

	// os.Root confines every access to the fixture dir, so a crafted
	// request path ("..") cannot write outside it.
	root, err := os.OpenRoot(rec.dir)
	if err != nil {
		return fmt.Errorf("open fixture dir: %w", err)
	}
	defer root.Close()

	rel := strings.TrimPrefix(key, "/") + ".json"
	if existing, readErr := root.ReadFile(rel); readErr == nil {
		body = mergeArrays(existing, body)
	}

	if err := root.MkdirAll(path.Dir(rel), fixtureDirPerm); err != nil {
		return fmt.Errorf("create fixture subdir: %w", err)
	}

	if err := root.WriteFile(rel, prettyJSON(body), 0o600); err != nil {
		return fmt.Errorf("write fixture: %w", err)
	}

	// Recorded bodies are already shifted, so a later -anonymize must not
	// shift them again.
	if err := root.WriteFile(shiftMarker, []byte(shiftMarkerText), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", shiftMarker, err)
	}

	return nil
}

func mergeArrays(existing, incoming []byte) []byte {
	var oldItems, newItems []map[string]any
	if json.Unmarshal(existing, &oldItems) != nil || json.Unmarshal(incoming, &newItems) != nil {
		return incoming
	}

	seen := map[string]bool{}
	merged := make([]map[string]any, 0, len(oldItems)+len(newItems))

	for _, item := range append(newItems, oldItems...) {
		key, ok := mergeKey(item)
		if ok && seen[key] {
			continue
		}

		seen[key] = true
		merged = append(merged, item)
	}

	out, err := json.Marshal(merged)
	if err != nil {
		return incoming
	}

	return out
}

// mergeKey identifies a list element: by "id" (activities, events, wellness)
// or, for stream entries that have no id, by stream "type".
func mergeKey(item map[string]any) (string, bool) {
	if item["id"] != nil {
		return "id:" + fmt.Sprint(item["id"]), true
	}

	if streamType, ok := item["type"].(string); ok {
		return "type:" + streamType, true
	}

	return "", false
}

func prettyJSON(body []byte) []byte {
	var value any
	if decodeJSON(body, &value) != nil {
		return body
	}

	var out bytes.Buffer

	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(value); err != nil {
		return body
	}

	return out.Bytes()
}

// Placeholder values written over private data.
const (
	anonName   = "Mock Athlete"
	anonEmail  = "athlete@example.invalid"
	anonText   = "redacted"
	anonDOB    = "1990-01-01"
	anonDevice = "Mock Device"
)

// redactedKeys are always replaced, wherever they appear.
func redactedKey(key string) (any, bool) {
	lower := strings.ToLower(key)

	switch lower {
	case "email":
		return anonEmail, true
	case "firstname", "first_name", "lastname", "last_name", "athlete_name", "username":
		return anonName, true
	case "icu_date_of_birth", "date_of_birth", "dob", "birthday":
		return anonDOB, true
	case "athlete_id", "icu_athlete_id", "athleteid", "created_by_id", "plan_athlete_id":
		return "i0", true
	case "athleteids", "countries", "languages":
		return []any{}, true
	case "power_meter_serial", "serial", "serial_number", "external_id":
		return nil, true
	case "city", "state", "country", "address", "location", "home", "bio", "website", "profile", "profile_medium",
		"avatar", "avatar_url", "phone", "postcode", "zip":
		return anonText, true
	case "strava_id", "strava_athlete_id", "garmin_id", "polar_id", "wahoo_id", "suunto_id", "coros_id", "zepp_id":
		return nil, true
	case "start_latlng", "end_latlng", "latlng", "polyline", "summary_polyline", "map":
		return nil, true
	case "lat", "lng", "lon", "latitude", "longitude", "start_lat", "start_lng", "end_lat", "end_lng":
		return 0, true
	}

	if strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password") || strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "apikey") || strings.HasSuffix(lower, "_user_id") {
		return nil, true
	}

	return nil, false
}

// personMarkers identify an object describing a person, whose "name" is PII.
func isPersonObject(obj map[string]any) bool {
	for _, key := range []string{"email", "firstname", "lastname", "icu_date_of_birth", "sportSettings"} {
		if _, ok := obj[key]; ok {
			return true
		}
	}

	return false
}

// isGearObject matches bikes ("b123") and shoes ("g123"), whose names are
// personal nicknames.
func isGearObject(obj map[string]any) bool {
	id, ok := obj["id"].(string)
	if !ok || len(id) < 2 || (id[0] != 'b' && id[0] != 'g') {
		return false
	}

	for _, r := range id[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

func anonymizeJSON(body []byte) []byte {
	var value any
	if decodeJSON(body, &value) != nil {
		return body
	}

	var out bytes.Buffer

	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(anonymize(value)); err != nil {
		return body
	}

	return bytes.TrimRight(out.Bytes(), "\n")
}

func anonymize(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		person := isPersonObject(typed)
		gear := isGearObject(typed)
		for key, inner := range typed {
			if replacement, ok := redactedKey(key); ok {
				typed[key] = replacement

				continue
			}

			if person && key == "id" {
				typed[key] = "i0"

				continue
			}

			if gear && key == "name" {
				continue // set below, after the id is remapped
			}

			if person && key == "name" {
				typed[key] = anonName

				continue
			}

			if isWeightKey(key) {
				if replacement, ok := fuzzWeight(inner); ok {
					typed[key] = replacement

					continue
				}
			}

			if isIDKey(key) {
				typed[key] = remapIDValue(inner)

				continue
			}

			typed[key] = anonymize(inner)
		}

		if _, ok := typed["name"]; ok && gear {
			typed["name"] = "Gear " + fmt.Sprint(typed["id"])
		}

		scrubActivityText(typed)
		rescalePerKg(typed)

		if curves, ok := typed["activities"].(map[string]any); ok {
			typed["activities"] = anonymizeCurveActivities(curves)
		}

		// GPS streams are location data: drop them entirely.
		if streamType, ok := typed["type"].(string); ok && streamType == "latlng" {
			typed["data"] = []any{}
			typed["data2"] = []any{}
		}

		return typed
	case []any:
		for i, inner := range typed {
			typed[i] = anonymize(inner)
		}

		return typed
	case string:
		if isExternalID(typed) {
			return remapExternalID(typed)
		}

		return roundTimeOfDay(typed)
	default:
		return value
	}
}

// clockTime matches the time of day of a full timestamp string.
var clockTime = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}):(\d{2})(:\d{2}(\.\d+)?)?`)

// roundTimeOfDay truncates a timestamp to the hour (keeping its layout and
// zone suffix), so exact ride start times are not published. Idempotent.
func roundTimeOfDay(value string) string {
	match := clockTime.FindStringSubmatchIndex(value)
	if match == nil {
		return value
	}

	head := value[:match[3]]
	rest := []byte(value[match[3]:match[1]])

	for i, r := range rest {
		if r >= '0' && r <= '9' {
			rest[i] = '0'
		}
	}

	return head + string(rest) + value[match[1]:]
}

// fuzzWeight replaces a numeric (or numeric-string) weight with anonWeight.
func fuzzWeight(value any) (any, bool) {
	switch typed := value.(type) {
	case json.Number:
		return anonWeight, true
	case string:
		if _, err := strconv.ParseFloat(typed, 64); err == nil {
			return string(anonWeight), true
		}
	}

	return nil, false
}

// syntheticID matches the fake 9-digit ids this repo uses on purpose
// (events 800000xxx, remapped ids 7xxxxxxxx).
var syntheticID = regexp.MustCompile(`^(7\d{8}|8000\d{5})$`)

// prefixedID matches intervals.icu activity ("i123"), bike and shoe ids.
var prefixedID = regexp.MustCompile(`^([bgi])(\d{6,})$`)

// syntheticPrefixedID matches the fake prefixed ids: fixture activities
// i9000xxxxx, remapped curve ids i8xxxxxxxx and corpus ids i6xxxxxxxx,
// fixture gear b9xxxx/g9xxxx and remapped gear b6xxxxxxx/g6xxxxxxx.
var syntheticPrefixedID = regexp.MustCompile(`^(i[689]\d{8}|[bg]9\d{4}|[bg]6\d{7})$`)

// isRealPrefixedID reports a real (not synthetic) activity or gear id.
func isRealPrefixedID(value string) bool {
	return prefixedID.MatchString(value) && !syntheticPrefixedID.MatchString(value)
}

// remapPrefixedID maps a real activity or gear id to a stable synthetic one
// with the same prefix ("i173884602" -> "i6xxxxxxxx").
func remapPrefixedID(id string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(id))

	if id[0] == 'i' {
		return fmt.Sprintf("i6%08d", hash.Sum32()%100_000_000)
	}

	return fmt.Sprintf("%c6%07d", id[0], hash.Sum32()%10_000_000)
}

// activityText lists free-text activity fields and their replacement; the
// name is handled separately ("Activity <id>").
func activityText() map[string]any {
	return map[string]any{
		"description":         nil,
		"device_name":         anonDevice,
		"power_meter":         anonDevice,
		"oauth_client_id":     nil,
		"oauth_client_name":   nil,
		"skyline_chart_bytes": nil,
	}
}

// isActivityObject matches an activity record (list item, detail or curve
// entry): an "i<digits>" id plus a start date.
func isActivityObject(obj map[string]any) bool {
	id, ok := obj["id"].(string)
	if !ok || !activityID.MatchString(id) {
		return false
	}

	_, dated := obj["start_date_local"]

	return dated
}

// scrubActivityText replaces names, descriptions, device and OAuth client
// names of activities. Idempotent.
func scrubActivityText(obj map[string]any) {
	if !isActivityObject(obj) {
		return
	}

	if _, ok := obj["name"]; ok {
		obj["name"] = "Activity " + fmt.Sprint(obj["id"])
	}

	for key, replacement := range activityText() {
		if value, ok := obj[key]; ok && value != nil {
			obj[key] = replacement
		}
	}
}

// longNumericID matches an all-digit id of 9+ digits, optionally negative.
var longNumericID = regexp.MustCompile(`^-?\d{9,}$`)

func isIDKey(key string) bool {
	lower := strings.ToLower(key)

	return lower == "id" || lower == "ids" || strings.HasSuffix(lower, "_id") || strings.HasSuffix(lower, "_ids") ||
		strings.HasSuffix(key, "Id") || strings.HasSuffix(key, "Ids")
}

func isLongNumericID(value string) bool {
	return longNumericID.MatchString(value) && !syntheticID.MatchString(value)
}

// remapIDValue remaps 9+ digit numeric ids held in id-like keys (numbers or
// strings, also inside arrays) to stable synthetic 700000xxx ids.
func remapIDValue(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if isLongNumericID(string(typed)) {
			return json.Number(remapNumericID(string(typed)))
		}
	case string:
		if isLongNumericID(typed) {
			return remapNumericID(typed)
		}

		if isRealPrefixedID(typed) {
			return remapPrefixedID(typed)
		}

		return anonymize(typed)
	case []any:
		for i, inner := range typed {
			typed[i] = remapIDValue(inner)
		}

		return typed
	default:
		return anonymize(value)
	}

	return value
}

func remapNumericID(id string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(id))

	return fmt.Sprintf("7%08d", hash.Sum32()%100_000_000)
}

// anonWeight is the single fuzzed body weight written over every weight
// field; it is deliberately far from the athlete's real weight.
const anonWeight = json.Number("72.8")

// wkgPrecision rounds rescaled watts_per_kg to 6 decimals, as the API does.
const wkgPrecision = 1e6

// isWeightKey matches any key naming a body weight; keys about watts
// ("icu_weighted_avg_watts") and weighted averages ("weightedAvgPower") are
// power, not weight.
func isWeightKey(key string) bool {
	lower := strings.ToLower(key)

	return strings.Contains(lower, "weight") && !strings.Contains(lower, "watts") && !strings.Contains(lower, "weighted")
}

// isExternalID matches all-digit ids of 10+ digits (Strava activity ids).
func isExternalID(value string) bool {
	if len(value) < 10 {
		return false
	}

	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// remapExternalID turns an external id into a stable fake intervals.icu id
// ("i8" + 8 digits), so the same id maps to the same value in every file.
func remapExternalID(id string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(id))

	return fmt.Sprintf("i8%08d", hash.Sum32()%100_000_000)
}

// anonymizeCurveActivities rewrites the activity map embedded in power and
// HR curve responses: keys and ids are remapped and names replaced. Dates
// are moved by the shared date shift (see shift.go).
func anonymizeCurveActivities(curves map[string]any) map[string]any {
	out := make(map[string]any, len(curves))
	for key, inner := range curves {
		external := isExternalID(key) || isLongNumericID(key)
		if external {
			key = remapExternalID(key)
		}

		if activity, ok := inner.(map[string]any); ok {
			activity["id"] = key
			activity["name"] = "Activity " + key
		}

		out[key] = inner
	}

	return out
}

// anonymizeDirWith rewrites every fixture under dir through the anonymizer.
// Anonymization is idempotent, so it is safe to re-run after rule changes.
// The first run also applies the date shift to the whole directory and drops
// shiftMarker, so later runs never shift (or jitter) again.
func anonymizeDirWith(dir string, shift dateShift) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("open fixture dir: %w", err)
	}
	defer root.Close()

	doShift := !dirShifted(root)

	walkErr := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(name, ".json") {
			return walkErr
		}

		body, err := root.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		body = anonymizeJSON(body)
		if doShift {
			body = shift.shiftFile(name, body)
		}

		if err := root.WriteFile(name, prettyJSON(body), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}

		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("anonymize %s: %w", dir, walkErr)
	}

	if err := recomputeSummaries(root); err != nil {
		return fmt.Errorf("athlete summary: %w", err)
	}

	if doShift {
		if err := root.WriteFile(shiftMarker, []byte(shiftMarkerText), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", shiftMarker, err)
		}
	}

	return nil
}

// sameOriginRedirect refuses any redirect that changes scheme or host, so a
// capture can never be bounced to an unexpected server.
func sameOriginRedirect(req *http.Request, via []*http.Request) error {
	origin := via[0].URL
	if !strings.EqualFold(req.URL.Scheme, origin.Scheme) || !strings.EqualFold(req.URL.Host, origin.Host) || len(via) >= maxRedirects {
		return fmt.Errorf("refusing redirect %s://%s -> %s://%s", origin.Scheme, origin.Host, req.URL.Scheme, req.URL.Host)
	}

	return nil
}

// decodeJSON keeps numbers as written, so rewriting a fixture never changes
// their formatting.
func decodeJSON(body []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	return nil
}

// perKgBase returns the absolute-power key a per-kg key is derived from:
// watts_per_kg -> watts, average_watts_kg -> average_watts, eftpPerKg -> eftp.
func perKgBase(key string) (string, bool) {
	for _, suffix := range []string{"_per_kg", "PerKg", "_kg"} {
		if base, ok := strings.CutSuffix(key, suffix); ok && base != "" && !isWeightKey(key) {
			return base, true
		}
	}

	return "", false
}

// rescalePerKg recomputes every per-kg value from its absolute power and the
// fuzzed weight, so the real weight cannot be derived from the ratio.
func rescalePerKg(obj map[string]any) {
	weight, _ := strconv.ParseFloat(string(anonWeight), 64)

	for key, perKg := range obj {
		base, ok := perKgBase(key)
		if !ok || perKg == nil {
			continue
		}

		if power, ok := obj[base]; ok {
			obj[key] = divideBy(power, perKg, weight)
		}
	}
}

// divideBy mirrors the shape of perKg (number or nested arrays), replacing
// each number with power/weight; entries without a matching power are kept.
func divideBy(power, perKg any, weight float64) any {
	switch typed := perKg.(type) {
	case json.Number:
		number, ok := power.(json.Number)
		if !ok {
			return perKg
		}

		value, err := number.Float64()
		if err != nil {
			return perKg
		}

		return json.Number(strconv.FormatFloat(math.Round(value/weight*wkgPrecision)/wkgPrecision, 'f', -1, 64))
	case []any:
		powers, ok := power.([]any)
		if !ok || len(powers) != len(typed) {
			return perKg
		}

		out := make([]any, len(typed))
		for i := range typed {
			out[i] = divideBy(powers[i], typed[i], weight)
		}

		return out
	default:
		return perKg
	}
}
