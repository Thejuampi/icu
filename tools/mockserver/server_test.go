package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func writeFixture(t *testing.T, dir, rel, body string) {
	t.Helper()

	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	dir := t.TempDir()
	writeFixture(t, dir, "api/v1/athlete/0.json", `{"id":"i0","name":"Mock Athlete","timezone":"America/New_York"}`)
	writeFixture(t, dir, "api/v1/athlete/0/events/148627465.json", `{"id":148627465,"name":"Endurance"}`)
	writeFixture(t, dir, "api/v1/athlete/0/wellness.json",
		`[{"id":"2026-10-01","ctl":50},{"id":"2026-10-10","ctl":51},{"id":"2026-10-20","ctl":52},{"note":"undated"}]`)
	writeFixture(t, dir, "api/v1/activity/i195273358/streams.json", `[{"type":"watts","data":[100,200]}]`)
	writeFixture(t, dir, "README.txt", "ignored")

	srv, err := newServer(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	srv.now = func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) }

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	return ts
}

func do(t *testing.T, method, url, body string) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	return resp.StatusCode, string(raw)
}

func TestServerServesFixtures(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	cases := []struct {
		path string
		code int
		want string
	}{
		{"/api/v1/athlete/0", 200, "America/New_York"},
		{"/api/v1/athlete/i445566", 200, "Mock Athlete"},
		{"/api/v1/athlete/0/", 200, "Mock Athlete"},
		{"/api/v1/athlete/0/events/148627465", 200, "Endurance"},
		{"/api/v1/activity/i195273358/streams", 200, "watts"},
		{"/api/v1/activity/i195273358/streams.json", 200, "watts"},
		{"/api/v1/activity/i000/streams", 404, "no fixture"},
		{"/_mock/health", 200, "ok"},
	}

	for _, tc := range cases {
		code, body := do(t, http.MethodGet, ts.URL+tc.path, "")
		if code != tc.code || !strings.Contains(body, tc.want) {
			t.Errorf("GET %s = %d %q, want %d containing %q", tc.path, code, body, tc.code, tc.want)
		}
	}
}

func TestServerFiltersListsByDate(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	_, body := do(t, http.MethodGet, ts.URL+"/api/v1/athlete/0/wellness?oldest=2026-10-05&newest=2026-10-15", "")

	var items []map[string]any
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 || items[0]["note"] != "undated" || items[1]["id"] != "2026-10-10" {
		t.Fatalf("filtered = %v", items)
	}

	_, all := do(t, http.MethodGet, ts.URL+"/api/v1/athlete/0/wellness", "")
	if strings.Count(all, `"id"`) != 3 {
		t.Fatalf("unfiltered = %s", all)
	}
}

func TestServerRecordsWritesInMemoryAndResets(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)
	eventURL := ts.URL + "/api/v1/athlete/0/events/148627465"

	code, body := do(t, http.MethodPut, eventURL, `{"name":"x"}`)
	if code != 200 || !strings.Contains(body, `"name":"x"`) || !strings.Contains(body, "148627465") {
		t.Fatalf("PUT = %d %s", code, body)
	}

	if _, got := do(t, http.MethodGet, eventURL, ""); !strings.Contains(got, `"name":"x"`) {
		t.Fatalf("GET after PUT = %s, want in-memory overlay", got)
	}

	if code, body = do(t, http.MethodPost, ts.URL+"/api/v1/athlete/0/events?upsert=true", `{"name":"new"}`); code != 200 || body != `{"name":"new"}` {
		t.Fatalf("POST = %d %s", code, body)
	}

	if code, body = do(t, http.MethodPost, ts.URL+"/api/v1/athlete/0/wellness-bulk", "plain text"); code != 200 || body != `{}` {
		t.Fatalf("POST text = %d %s", code, body)
	}

	if code, body = do(t, http.MethodDelete, eventURL, ""); code != 200 || body != `{}` {
		t.Fatalf("DELETE = %d %s", code, body)
	}

	if code, body = do(t, http.MethodPut, ts.URL+"/api/v1/athlete/0/new", `[1,2]`); code != 200 || body != `[1,2]` {
		t.Fatalf("PUT array = %d %s", code, body)
	}

	if code, body = do(t, http.MethodPatch, ts.URL+"/api/v1/athlete/0/x", `nope`); code != 200 || body != `{}` {
		t.Fatalf("PATCH invalid = %d %s", code, body)
	}

	_, writes := do(t, http.MethodGet, ts.URL+"/_mock/writes", "")

	var got struct {
		Count  int     `json:"count"`
		Writes []Write `json:"writes"`
	}
	if err := json.Unmarshal([]byte(writes), &got); err != nil {
		t.Fatal(err)
	}

	if got.Count != 6 || got.Writes[0].Method != http.MethodPut || got.Writes[0].Path != "/api/v1/athlete/0/events/148627465" ||
		string(got.Writes[0].Body) != `{"name":"x"}` || got.Writes[1].Query != "upsert=true" || string(got.Writes[2].Body) != `"plain text"` ||
		got.Writes[0].Time != "2026-10-27T12:00:00Z" {
		t.Fatalf("writes = %+v", got)
	}

	if code, _ = do(t, http.MethodPost, ts.URL+"/_mock/reset", ""); code != 200 {
		t.Fatalf("reset = %d", code)
	}

	if _, got2 := do(t, http.MethodGet, ts.URL+"/_mock/writes", ""); !strings.Contains(got2, `"count":0`) {
		t.Fatalf("after reset = %s", got2)
	}

	if _, after := do(t, http.MethodGet, eventURL, ""); !strings.Contains(after, "Endurance") {
		t.Fatalf("overlay not cleared: %s", after)
	}
}

func TestServerRejectsUnknownMethod(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	if code, _ := do(t, http.MethodOptions, ts.URL+"/api/v1/athlete/0", ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("OPTIONS = %d", code)
	}
}

func TestLoadFixturesErrors(t *testing.T) {
	t.Parallel()

	if _, err := newServer(filepath.Join(t.TempDir(), "missing"), io.Discard); err == nil {
		t.Fatal("missing dir: want error")
	}

	if _, err := newServer(t.TempDir(), io.Discard); err == nil || !strings.Contains(err.Error(), "no fixtures") {
		t.Fatalf("empty dir: err = %v", err)
	}

	dir := t.TempDir()
	writeFixture(t, dir, "bad.json", "{")

	if _, err := newServer(dir, io.Discard); err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("bad json: err = %v", err)
	}
}

func TestCommittedFixturesCoverCLIEndpoints(t *testing.T) {
	t.Parallel()

	srv, err := newServer("../../testdata/fixtures", io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{
		"/api/v1/athlete/0",
		"/api/v1/athlete/0/profile",
		"/api/v1/athlete/0/sport-settings",
		"/api/v1/athlete/0/wellness",
		"/api/v1/athlete/0/events",
		"/api/v1/athlete/0/activities",
		"/api/v1/activity/i195273358/streams",
		"/api/v1/activity/i202545713/streams",
		"/api/v1/activity/i198806466/streams",
	} {
		if _, ok := srv.lookup(key); !ok {
			t.Errorf("missing fixture %s", key)
		}
	}

	// Private data must stay anonymized in the committed fixtures.
	for key, body := range srv.fixtures {
		if !bytes.Equal(anonymizeJSON(body), anonymizeJSON(anonymizeJSON(body))) {
			t.Errorf("%s: anonymizer not idempotent", key)
		}

		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			t.Fatal(err)
		}

		again, err := json.Marshal(anonymize(value))
		if err != nil {
			t.Fatal(err)
		}

		var orig any
		if err := json.Unmarshal(body, &orig); err != nil {
			t.Fatal(err)
		}

		origCompact, err := json.Marshal(orig)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(again, origCompact) {
			t.Errorf("%s: fixture contains data the anonymizer would still redact", key)
		}
	}
}

func TestRunServesUntilCanceled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "api/v1/athlete/0.json", `{"id":"i0"}`)

	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan int, 1)

	go func() { done <- run(ctx, []string{"-fixtures", dir}, out, io.Discard) }()

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "listening on") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	addr := strings.TrimSpace(strings.TrimPrefix(out.String(), "icu-mock listening on "))
	if code, body := do(t, http.MethodGet, addr+"/api/v1/athlete/0", ""); code != 200 || !strings.Contains(body, "i0") {
		t.Fatalf("GET = %d %s", code, body)
	}

	cancel()

	if code := <-done; code != 0 {
		t.Fatalf("run = %d", code)
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	if code := run(ctx, []string{"-bogus"}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("bad flag = %d", code)
	}

	if code := run(ctx, []string{"-port", "70000"}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("bad port = %d", code)
	}

	if code := run(ctx, []string{"-fixtures", t.TempDir()}, io.Discard, io.Discard); code != 1 {
		t.Fatalf("empty fixtures = %d", code)
	}

	dir := t.TempDir()
	writeFixture(t, dir, "a.json", `{}`)

	if code := run(ctx, []string{"-fixtures", dir, "-host", "256.0.0.1"}, io.Discard, io.Discard); code != 1 {
		t.Fatalf("bad host = %d", code)
	}
}

func TestServeReportsServeError(t *testing.T) {
	t.Parallel()

	var lc net.ListenConfig

	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	_ = listener.Close()

	if code := serve(context.Background(), listener, http.NotFoundHandler(), io.Discard, io.Discard); code != 1 {
		t.Fatalf("serve on closed listener = %d", code)
	}
}

func TestFilterByDateEdgeCases(t *testing.T) {
	t.Parallel()

	if got := string(filterByDate([]byte(`{"a":1}`), "2026-01-01", "")); got != `{"a":1}` {
		t.Fatalf("object = %s", got)
	}

	got := string(filterByDate([]byte(`[{"start_date_local":"2026-10-02T08:00:00"},{"start_date_local":"2026-09-01T08:00:00"}]`), "2026-10-01", ""))
	if !strings.Contains(got, "2026-10-02") || strings.Contains(got, "2026-09-01") {
		t.Fatalf("oldest only = %s", got)
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}
