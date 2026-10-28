package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunBaseURLFlagTargetsServerWithoutWarning(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)

		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/athlete/0" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"i0","name":"Mock Athlete"}`))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer

	code := run([]string{"icu", "athlete", "show", "--api-key", "k", "--base-url", server.URL + "/"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run = %d, stderr %q", code, stderr.String())
	}

	if hits.Load() == 0 {
		t.Fatal("mock server was not called")
	}

	if strings.Contains(stderr.String(), "Warning") {
		t.Fatalf("unexpected warning for mock URL: %q", stderr.String())
	}

	if !strings.Contains(stdout.String(), "Mock Athlete") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunBaseURLAcceptedByStrictSchemaCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	// analysis coaching has a strict schema; --base-url must not be rejected
	// as an unknown flag (validation fails later on the bogus flag instead).
	code := run([]string{"icu", "analysis", "coaching", "--base-url", "http://127.0.0.1:1", "--bogus"}, &stdout, &stderr)
	if code != 1 || strings.Contains(stderr.String(), "--base-url") || !strings.Contains(stderr.String(), "unknown flag --bogus") {
		t.Fatalf("run = %d stderr %q", code, stderr.String())
	}
}

func TestWarnRealAPI(t *testing.T) {
	t.Parallel()

	var realAPI, mock bytes.Buffer

	warnRealAPI(&realAPI, "https://intervals.icu")
	warnRealAPI(&mock, "http://127.0.0.1:8765")

	if !strings.Contains(realAPI.String(), "Warning: using the real intervals.icu API") {
		t.Fatalf("real warning = %q", realAPI.String())
	}

	if mock.Len() != 0 {
		t.Fatalf("mock warning = %q, want empty", mock.String())
	}
}

func TestGlobalHelpMentionsBaseURL(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	run([]string{"icu", "help"}, &stdout, &stderr)

	if !strings.Contains(stdout.String(), "--base-url URL") {
		t.Fatalf("help missing --base-url: %q", stdout.String())
	}
}
