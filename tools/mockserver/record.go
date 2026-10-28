package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
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
}

func newRecorder(upstream, dir string, log io.Writer) *recorder {
	return &recorder{
		upstream: strings.TrimRight(upstream, "/"),
		dir:      dir,
		client:   &http.Client{Timeout: upstreamTimeout, CheckRedirect: sameOriginRedirect},
		log:      log,
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
		body = anonymizeJSON(body)
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
	if json.Unmarshal(body, &value) != nil {
		return body
	}

	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return body
	}

	return append(out, '\n')
}

// Placeholder values written over private data.
const (
	anonName  = "Mock Athlete"
	anonEmail = "athlete@example.invalid"
	anonText  = "redacted"
	anonDOB   = "1990-01-01"
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
	if json.Unmarshal(body, &value) != nil {
		return body
	}

	out, err := json.Marshal(anonymize(value))
	if err != nil {
		return body
	}

	return out
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
				typed[key] = "Gear " + fmt.Sprint(typed["id"])

				continue
			}

			if person && key == "name" {
				typed[key] = anonName

				continue
			}

			typed[key] = anonymize(inner)
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
	default:
		return value
	}
}

// anonymizeDir rewrites every fixture under dir through the anonymizer.
// Anonymization is idempotent, so it is safe to re-run after rule changes.
func anonymizeDir(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("open fixture dir: %w", err)
	}
	defer root.Close()

	walkErr := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(name, ".json") {
			return walkErr
		}

		body, err := root.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		if err := root.WriteFile(name, prettyJSON(anonymizeJSON(body)), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}

		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("anonymize %s: %w", dir, walkErr)
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
