package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxBodyBytes = 10 << 20

// Write is one write request the mock accepted (and did not persist).
type Write struct {
	Time   string          `json:"time"`
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Query  string          `json:"query,omitempty"`
	Body   json.RawMessage `json:"body,omitempty"`
}

type server struct {
	mu       sync.Mutex
	fixtures map[string][]byte
	overlay  map[string][]byte
	writes   []Write
	log      io.Writer
	now      func() time.Time
	// sink is the second listener for -redirect-sink; nil = mode off.
	sink *redirectSink
}

func newServer(dir string, log io.Writer) (*server, error) {
	fixtures, err := loadFixtures(dir)
	if err != nil {
		return nil, err
	}

	return &server{
		fixtures: fixtures,
		overlay:  map[string][]byte{},
		log:      log,
		now:      time.Now,
	}, nil
}

// loadFixtures maps "/api/v1/..." request paths to fixture bodies: the file
// testdata/fixtures/api/v1/athlete/0/wellness.json serves /api/v1/athlete/0/wellness.
func loadFixtures(dir string) (map[string][]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("load fixtures from %s: %w", dir, err)
	}
	defer root.Close()

	fixtures := map[string][]byte{}

	walkErr := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(name, ".json") {
			return walkErr
		}

		body, err := root.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		if !json.Valid(body) {
			return fmt.Errorf("fixture %s is not valid JSON", name)
		}

		fixtures["/"+strings.TrimSuffix(name, ".json")] = body

		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("load fixtures from %s: %w", dir, walkErr)
	}

	if len(fixtures) == 0 {
		return nil, fmt.Errorf("no fixtures found in %s", dir)
	}

	return fixtures, nil
}

// fixtureKey normalizes a request path so any athlete id maps onto the "0"
// (self) fixtures and a trailing ".json" suffix is optional.
func fixtureKey(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "athlete" && i > 0 && parts[i-1] == "v1" {
			parts[i+1] = "0"
		}
	}

	return strings.Join(parts, "/")
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.serveRedirectMode(w, r) {
		return
	}

	switch {
	case r.URL.Path == "/_mock/writes" && r.Method == http.MethodGet:
		s.handleWrites(w)
	case r.URL.Path == "/_mock/reset" && (r.Method == http.MethodPost || r.Method == http.MethodDelete):
		s.handleReset(w)
	case r.URL.Path == "/_mock/health":
		writeJSON(w, http.StatusOK, []byte(`{"ok":true}`))
	case isZeppPath(r.URL.Path) && r.Method == http.MethodGet:
		s.handleZepp(w, r)
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		s.handleGet(w, r)
	case r.Method == http.MethodPut || r.Method == http.MethodPost || r.Method == http.MethodDelete || r.Method == http.MethodPatch:
		s.handleWrite(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleWrites(w http.ResponseWriter) {
	s.mu.Lock()
	writes := append([]Write{}, s.writes...)
	s.mu.Unlock()

	body, err := json.Marshal(map[string]any{"count": len(writes), "writes": writes})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())

		return
	}

	writeJSON(w, http.StatusOK, body)
}

func (s *server) handleReset(w http.ResponseWriter) {
	s.mu.Lock()
	s.writes = nil
	s.overlay = map[string][]byte{}
	s.mu.Unlock()

	if s.sink != nil {
		s.sink.reset()
	}

	writeJSON(w, http.StatusOK, []byte(`{"reset":true}`))
}

func (s *server) lookup(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if body, ok := s.overlay[key]; ok {
		return body, true
	}

	if body, ok := s.fixtures[key]; ok {
		return body, true
	}

	body, ok := s.fixtures[strings.TrimSuffix(key, ".json")]

	return body, ok
}

func (s *server) handleGet(w http.ResponseWriter, r *http.Request) {
	key := fixtureKey(r.URL.Path)

	body, ok := s.lookup(key)
	if !ok {
		fmt.Fprintf(s.log, "icu-mock: no fixture for GET %s\n", r.URL.Path)
		writeError(w, http.StatusNotFound, "no fixture for "+r.URL.Path)

		return
	}

	writeJSON(w, http.StatusOK, filterByDate(body, r.URL.Query().Get("oldest"), r.URL.Query().Get("newest")))
}

func (s *server) handleWrite(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())

		return
	}

	entry := Write{
		Time:   s.now().UTC().Format(time.RFC3339Nano),
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
	}

	if len(raw) > 0 {
		if isSecretForm(r) {
			// Form bodies (Zepp password, access code, any other auth form)
			// are never kept in the write log. Field names stay, values are
			// redacted.
			quoted, _ := json.Marshal(redactForm(raw)) //nolint:errchkjson // marshaling a string cannot fail
			entry.Body = quoted
		} else if json.Valid(raw) {
			entry.Body = raw
		} else {
			quoted, _ := json.Marshal(string(raw)) //nolint:errchkjson // marshaling a string cannot fail
			entry.Body = quoted
		}
	}

	key := fixtureKey(r.URL.Path)
	resp := s.record(entry, key, raw)

	fmt.Fprintf(s.log, "icu-mock: recorded %s %s (not persisted)\n", r.Method, r.URL.Path)
	writeJSON(w, http.StatusOK, resp)
}

// isSecretForm reports whether a write body must be redacted: every
// application/x-www-form-urlencoded body, and anything sent to a Zepp auth
// path. Paths are compared after path.Clean so variants such as
// "/v2/registrations/tokens/" or "//v2/client/login" cannot bypass it.
func isSecretForm(r *http.Request) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil && mt == "application/x-www-form-urlencoded" {
		return true
	}
	clean := path.Clean("/" + r.URL.Path)

	return clean == mockZeppTokensPath || clean == mockZeppLoginPath ||
		strings.HasSuffix(clean, mockZeppTokensPath) || strings.HasSuffix(clean, mockZeppLoginPath)
}

// redactForm returns a form body with every value replaced by redactedValue
// (field names kept, sorted); a body that does not parse as a form becomes
// redactedValue.
func redactForm(raw []byte) string {
	values, err := url.ParseQuery(string(raw))
	if err != nil || len(values) == 0 {
		return redactedValue
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, url.QueryEscape(key)+"="+redactedValue)
	}

	return strings.Join(parts, "&")
}

// record stores the write and returns the response body: DELETE answers {},
// PUT/PATCH merge the JSON object body over the current resource (and keep
// it in memory so a later GET sees it), POST echoes the body. Exception:
// PUT/PATCH on */streams answers {"updated":[types...]} like intervals.icu
// and is not kept in memory, so a later GET still returns the fixture.
func (s *server) record(entry Write, key string, raw []byte) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.writes = append(s.writes, entry)

	switch entry.Method {
	case http.MethodDelete:
		return []byte(`{}`)
	case http.MethodPut, http.MethodPatch:
		if strings.HasSuffix(entry.Path, "/streams") {
			return streamsUpdateResponse(raw)
		}
		if strings.HasSuffix(key, "/wellness-bulk") {
			return s.applyWellnessBulk(key, raw)
		}
		current := s.overlay[key]
		if current == nil {
			current = s.fixtures[key]
		}

		merged := mergeObjects(current, raw)
		s.overlay[key] = merged

		return merged
	default:
		if json.Valid(raw) && len(raw) > 0 {
			return raw
		}

		return []byte(`{}`)
	}
}

func mergeObjects(base, patch []byte) []byte {
	var baseObj, patchObj map[string]any
	if json.Unmarshal(patch, &patchObj) != nil {
		if json.Valid(patch) && len(patch) > 0 {
			return patch
		}

		return []byte(`{}`)
	}

	if json.Unmarshal(base, &baseObj) != nil || baseObj == nil {
		baseObj = map[string]any{}
	}

	for k, v := range patchObj {
		baseObj[k] = v
	}

	out, err := json.Marshal(baseObj)
	if err != nil {
		return patch
	}

	return out
}

// filterByDate keeps array elements whose date (id for wellness,
// start_date_local for activities and events) lies within [oldest, newest].
// Non-array bodies and elements without a date are returned unchanged.
func filterByDate(body []byte, oldest, newest string) []byte {
	if oldest == "" && newest == "" {
		return body
	}

	var items []map[string]any
	if json.Unmarshal(body, &items) != nil {
		return body
	}

	kept := make([]map[string]any, 0, len(items))
	for _, item := range items {
		day := itemDay(item)
		if day == "" || inRange(day, oldest, newest) {
			kept = append(kept, item)
		}
	}

	sort.SliceStable(kept, func(i, j int) bool { return itemDay(kept[i]) < itemDay(kept[j]) })

	out, err := json.Marshal(kept)
	if err != nil {
		return body
	}

	return out
}

const dayLen = len("2006-01-02")

func itemDay(item map[string]any) string {
	for _, key := range []string{"start_date_local", "id"} {
		if value, ok := item[key].(string); ok && len(value) >= dayLen && value[4] == '-' && value[7] == '-' {
			return value[:dayLen]
		}
	}

	return ""
}

func inRange(day, oldest, newest string) bool {
	if oldest != "" && len(oldest) >= dayLen && day < oldest[:dayLen] {
		return false
	}

	if newest != "" && len(newest) >= dayLen && day > newest[:dayLen] {
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body) //nolint:gosec // G705: JSON API mock served as application/json, never rendered as HTML
}

func writeError(w http.ResponseWriter, status int, msg string) {
	body, _ := json.Marshal(map[string]string{"error": msg}) //nolint:errchkjson // map[string]string always marshals

	writeJSON(w, status, body)
}

// streamsUpdateResponse mirrors PUT /api/v1/activity/{id}/streams, which
// answers {"updated":[types...]} for the stream types in the body.
func streamsUpdateResponse(raw []byte) []byte {
	var streams []struct {
		Type string `json:"type"`
	}
	updated := []string{}
	if json.Unmarshal(raw, &streams) == nil {
		for _, stream := range streams {
			if stream.Type != "" {
				updated = append(updated, stream.Type)
			}
		}
	}
	body, _ := json.Marshal(map[string][]string{"updated": updated}) //nolint:errchkjson // a string slice map cannot fail

	return body
}
