package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestZeppEventsServesTypeFixture(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "api/v1/athlete/0.json", `{"id":"i0"}`)
	writeFixture(t, dir, "_zepp/events/readiness/watch_score.json", `{"items":[{"value":{"sleepHRV":38}}]}`)

	srv, err := newServer(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	get := func(q string) string {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+mockZeppEventsPath+q, nil)
		req.Header.Set("Apptoken", "mock-app")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)

		return string(b)
	}
	if got := get("?eventType=readiness&subType=watch_score&from=0&to=1"); !strings.Contains(got, `"sleepHRV":38`) {
		t.Fatalf("fixture not served: %s", got)
	}
	// Traversal-looking params stay inside _zepp/events and fall back.
	if got := get("?eventType=../../api/v1/athlete&subType=0&from=0&to=1"); strings.Contains(got, "i0") {
		t.Fatalf("escaped fixture dir: %s", got)
	}
}
