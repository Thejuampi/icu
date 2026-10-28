package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRecorderForwardsGETsAndSavesAnonymized(t *testing.T) {
	t.Parallel()

	var nonGET atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			nonGET.Add(1)
		}

		if r.Header.Get("Authorization") != "Basic abc" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		switch {
		case r.URL.Path == "/api/v1/athlete/0":
			_, _ = w.Write([]byte(`{"id":"i445566","name":"Real Person","email":"real@x.com","icu_date_of_birth":"1985-05-05",` +
				`"strava_id":123,"city":"Somewhere","icu_api_key":"secret","bikes":[{"id":"b1","name":"Nickname"}],"timezone":"America/New_York"}`))
		case r.URL.Path == "/api/v1/athlete/0/activities" && r.URL.Query().Get("oldest") == "2026-01-01":
			_, _ = w.Write([]byte(`[{"id":"i1","name":"Ride","icu_athlete_id":"i445566"}]`))
		case r.URL.Path == "/api/v1/athlete/0/activities":
			_, _ = w.Write([]byte(`[{"id":"i2","name":"Run"},{"id":"i1","name":"Ride"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	dir := t.TempDir()
	ts := httptest.NewServer(newRecorder(upstream.URL+"/", dir, io.Discard))
	defer ts.Close()

	get := func(path string) int {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+path, http.NoBody)
		req.Header.Set("Authorization", "Basic abc")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		return resp.StatusCode
	}

	if code := get("/api/v1/athlete/0"); code != 200 {
		t.Fatalf("athlete = %d", code)
	}

	get("/api/v1/athlete/0/activities?oldest=2026-01-01")
	get("/api/v1/athlete/0/activities?oldest=2026-02-01")

	if code := get("/api/v1/missing"); code != 404 {
		t.Fatalf("missing = %d", code)
	}

	for _, method := range []string{http.MethodPut, http.MethodPost, http.MethodDelete} {
		if code, _ := do(t, method, ts.URL+"/api/v1/athlete/0/events/1", `{"name":"x"}`); code != http.StatusMethodNotAllowed {
			t.Fatalf("%s = %d, want 405", method, code)
		}
	}

	if nonGET.Load() != 0 {
		t.Fatalf("record mode forwarded %d non-GET requests", nonGET.Load())
	}

	athlete, err := os.ReadFile(filepath.Join(dir, "api", "v1", "athlete", "0.json"))
	if err != nil {
		t.Fatal(err)
	}

	for _, leaked := range []string{"Real Person", "real@x.com", "1985-05-05", "123", "Somewhere", "secret", "Nickname", "i445566"} {
		if strings.Contains(string(athlete), leaked) {
			t.Errorf("fixture leaks %q: %s", leaked, athlete)
		}
	}

	if !strings.Contains(string(athlete), "America/New_York") || !strings.Contains(string(athlete), anonName) {
		t.Errorf("fixture lost non-private data: %s", athlete)
	}

	acts, err := os.ReadFile(filepath.Join(dir, "api", "v1", "athlete", "0", "activities.json"))
	if err != nil {
		t.Fatal(err)
	}

	var items []map[string]any
	if err := json.Unmarshal(acts, &items); err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 || strings.Contains(string(acts), "i445566") {
		t.Fatalf("merged activities = %s", acts)
	}
}

func TestRecorderUpstreamErrors(t *testing.T) {
	t.Parallel()

	rec := newRecorder("http://127.0.0.1:1", t.TempDir(), io.Discard)
	ts := httptest.NewServer(rec)
	defer ts.Close()

	if code, _ := do(t, http.MethodGet, ts.URL+"/api/v1/athlete/0", ""); code != http.StatusBadGateway {
		t.Fatalf("unreachable upstream = %d", code)
	}

	bad := newRecorder("http://bad host", t.TempDir(), io.Discard)
	rr := httptest.NewRecorder()
	bad.ServeHTTP(rr, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", http.NoBody))

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("bad upstream url = %d", rr.Code)
	}
}

func TestRecorderSaveError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	blocker := filepath.Join(dir, "api")

	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := newRecorder("", dir, io.Discard)
	if err := rec.save("/api/v1/x", []byte(`{}`)); err == nil {
		t.Fatal("want error when fixture dir is a file")
	}
}

func TestRecorderSaveStaysInsideFixtureDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "fixtures")
	rec := newRecorder("", dir, io.Discard)

	if err := rec.save("/../../escape", []byte(`{}`)); err == nil {
		t.Fatal("want error for a path escaping the fixture dir")
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "..", "escape.json")); err == nil {
		t.Fatal("file written outside the fixture dir")
	}
}

func TestMergeArraysDedupesStreamsByType(t *testing.T) {
	t.Parallel()

	got := string(mergeArrays(
		[]byte(`[{"type":"time","data":[0]},{"type":"watts","data":[1]}]`),
		[]byte(`[{"type":"time","data":[9]},{"type":"heartrate","data":[2]},{"x":1},{"x":1}]`)))

	if strings.Count(got, `"time"`) != 1 || !strings.Contains(got, `[9]`) || !strings.Contains(got, `"watts"`) ||
		!strings.Contains(got, `"heartrate"`) || strings.Count(got, `"x"`) != 2 {
		t.Fatalf("merged = %s", got)
	}
}

func TestCommittedStreamFixturesHaveUniqueTypes(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"i195273358", "i202545713", "i198806466"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "api", "v1", "activity", id, "streams.json"))
		if err != nil {
			t.Fatal(err)
		}

		var streams []struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(body, &streams); err != nil {
			t.Fatal(err)
		}

		seen := map[string]bool{}
		for _, s := range streams {
			if seen[s.Type] {
				t.Errorf("%s: duplicate stream type %q", id, s.Type)
			}

			seen[s.Type] = true
		}
	}
}

func TestAnonymizeHelpers(t *testing.T) {
	t.Parallel()

	if got := string(anonymizeJSON([]byte("not json"))); got != "not json" {
		t.Fatalf("invalid = %s", got)
	}

	got := string(anonymizeJSON([]byte(`[{"type":"latlng","data":[1],"data2":[2]},{"type":"watts","data":[3]},` +
		`{"start_latlng":[1,2],"lat":40.7,"refresh_token":"t","garmin_user_id":"g","created_by_id":"i9","countries":["AR"],"external_id":"999"}]`)))
	for _, leaked := range []string{"[1]", "[2]", "40.7", `"t"`, `"g"`, "i9", "AR", "999"} {
		if strings.Contains(got, leaked) {
			t.Errorf("anonymized output leaks %s: %s", leaked, got)
		}
	}

	if !strings.Contains(got, "[3]") {
		t.Errorf("non-GPS stream dropped: %s", got)
	}

	activity := string(anonymizeJSON([]byte(`{"id":"i202545713","name":"Morning Ride","strava_id":"55"}`)))
	if !strings.Contains(activity, "i202545713") || !strings.Contains(activity, "Morning Ride") || strings.Contains(activity, "55") {
		t.Errorf("activity with strava_id must keep id/name and drop strava_id: %s", activity)
	}

	if isGearObject(map[string]any{"id": "bx1"}) || isGearObject(map[string]any{"id": 5}) || !isGearObject(map[string]any{"id": "g12"}) {
		t.Error("isGearObject")
	}

	if got := string(mergeArrays([]byte(`{}`), []byte(`[1]`))); got != `[1]` {
		t.Fatalf("mergeArrays non-array = %s", got)
	}

	if got := string(prettyJSON([]byte("x"))); got != "x" {
		t.Fatalf("prettyJSON invalid = %s", got)
	}

	if got := string(mergeObjects([]byte(`[1]`), []byte(`{"a":1}`))); got != `{"a":1}` {
		t.Fatalf("mergeObjects non-object base = %s", got)
	}
}

func TestAnonymizeDirMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "api/v1/athlete/0/profile.json", `{"athlete":{"id":"i77","email":"a@b.c","name":"Real"}}`)
	writeFixture(t, dir, "notes.txt", "skip")

	if code := run(context.Background(), []string{"-anonymize", dir}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("anonymize = %d", code)
	}

	body, err := os.ReadFile(filepath.Join(dir, "api", "v1", "athlete", "0", "profile.json"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(body), "a@b.c") || strings.Contains(string(body), "Real") || strings.Contains(string(body), "i77") {
		t.Fatalf("profile = %s", body)
	}

	if code := run(context.Background(), []string{"-anonymize", filepath.Join(dir, "missing")}, io.Discard, io.Discard); code != 1 {
		t.Fatalf("missing dir = %d", code)
	}
}

func TestRunRecordModeStarts(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan int, 1)

	go func() {
		done <- run(ctx, []string{"-record", "-upstream", "http://127.0.0.1:1", "-fixtures", t.TempDir()}, out, io.Discard)
	}()

	for !strings.Contains(out.String(), "listening on") {
		select {
		case code := <-done:
			t.Fatalf("exited early: %d", code)
		default:
		}
	}

	cancel()

	if code := <-done; code != 0 {
		t.Fatalf("run = %d", code)
	}
}
