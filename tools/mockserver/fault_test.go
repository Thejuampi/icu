package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestFaultInjection(t *testing.T) {
	t.Parallel()
	srv, err := newServer("../../testdata/fixtures", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	settings := ts.URL + "/api/v1/athlete/0/sport-settings/GravelRide"
	set := func(query string) int {
		code, _ := do(t, http.MethodPost, ts.URL+"/_mock/fault?"+query, "")

		return code
	}
	for _, mode := range []string{"401", "403", "500", "503"} {
		if code := set("match=/sport-settings/&mode=" + mode); code != http.StatusOK {
			t.Fatalf("set %s: %d", mode, code)
		}
		if code, body := do(t, http.MethodGet, settings, ""); mode != strconv.Itoa(code) || !strings.Contains(body, "injected fault") {
			t.Fatalf("mode %s: got %d %s", mode, code, body)
		}
		// Non-matching paths and the control endpoints are untouched.
		if code, _ := do(t, http.MethodGet, ts.URL+"/_mock/health", ""); code != http.StatusOK {
			t.Fatalf("health under fault: %d", code)
		}
	}
	if code, _ := do(t, http.MethodGet, ts.URL+"/api/v1/athlete/0/sport-settings/Ride", ""); code != http.StatusServiceUnavailable {
		t.Fatalf("Ride must match too: %d", code)
	}
	set("match=/sport-settings/&mode=close")
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, settings, nil)
	if resp, err := ts.Client().Do(req); err == nil {
		resp.Body.Close()
		t.Fatalf("close mode: expected a transport error, got %d", resp.StatusCode)
	}
	if code, _ := do(t, http.MethodDelete, ts.URL+"/_mock/fault", ""); code != http.StatusOK {
		t.Fatalf("clear: %d", code)
	}
	if code, _ := do(t, http.MethodGet, settings, ""); code != http.StatusOK {
		t.Fatalf("after clear: %d", code)
	}
	set("match=/sport-settings/&mode=401")
	do(t, http.MethodPost, ts.URL+"/_mock/reset", "")
	if code, _ := do(t, http.MethodGet, settings, ""); code != http.StatusOK {
		t.Fatalf("reset must clear the fault: %d", code)
	}
	for _, bad := range []string{"match=/x&mode=200", "match=/x&mode=abc", "mode=401", "match=/_mock/writes&mode=500"} {
		if code := set(bad); code != http.StatusBadRequest {
			t.Fatalf("%s: want 400, got %d", bad, code)
		}
	}
	if code, body := do(t, http.MethodGet, ts.URL+"/_mock/writes", ""); code != http.StatusOK || !strings.Contains(body, `"count":0`) {
		t.Fatalf("faults are not writes: %d %s", code, body)
	}
}
