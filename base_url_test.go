package icu_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestResolveBaseURLPrecedence(t *testing.T) {
	t.Setenv(icu.BaseURLEnv, "")

	if got := icu.ResolveBaseURL(map[string]string{}); got != icu.BaseURL {
		t.Fatalf("default = %q, want %q", got, icu.BaseURL)
	}

	t.Setenv(icu.BaseURLEnv, "http://127.0.0.1:9000/")

	if got := icu.ResolveBaseURL(map[string]string{}); got != "http://127.0.0.1:9000" {
		t.Fatalf("env = %q", got)
	}

	if got := icu.ResolveBaseURL(map[string]string{"base-url": "http://127.0.0.1:9001"}); got != "http://127.0.0.1:9001" {
		t.Fatalf("flag should win over env, got %q", got)
	}
}

func TestIsRealAPIBaseURL(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"https://intervals.icu":        true,
		"https://INTERVALS.ICU/":       true,
		"https://www.intervals.icu":    true,
		"https://intervals.icu.":       true,
		"https://INTERVALS.ICU.:443/x": true,
		"http://127.0.0.1:8765":        false,
		"http://localhost":             false,
		"https://notintervals.icu":     false,
		"https://intervals.icu.evil.x": false,
		"not a url":                    false,
		"://bad":                       false,
	}

	for in, want := range cases {
		if got := icu.IsRealAPIBaseURL(in); got != want {
			t.Errorf("IsRealAPIBaseURL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestClientRefusesCrossOriginRedirect(t *testing.T) {
	t.Parallel()

	var otherHits int

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		otherHits++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer other.Close()

	// 127.0.0.1 vs localhost: same machine, different host, so it must be refused.
	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/same") {
			_, _ = w.Write([]byte(`{"ok":true}`))

			return
		}

		if strings.HasSuffix(r.URL.Path, "/hop") {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/hop")+"/same", http.StatusFound) //nolint:gosec // test server, fixed target

			return
		}

		http.Redirect(w, r, otherURL+r.URL.Path, http.StatusFound) //nolint:gosec // test server, fixed target
	}))
	defer origin.Close()

	client := icu.NewClient("k", "0", icu.WithBaseURL(origin.URL))

	var out map[string]any
	if err := client.Get("athlete", []string{"0", "hop"}, nil, &out); err != nil || out["ok"] != true {
		t.Fatalf("same-origin redirect: err=%v out=%v", err, out)
	}

	err := client.Get("athlete", []string{"0", "away"}, nil, &out)
	if err == nil || !strings.Contains(err.Error(), "refusing redirect") || otherHits != 0 {
		t.Fatalf("cross-origin redirect: err=%v otherHits=%d", err, otherHits)
	}
}

func TestSameOriginRedirectsLimit(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1/x", http.NoBody)
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = req
	}

	if err := icu.SameOriginRedirects(req, via); err == nil || errors.Is(err, icu.ErrCrossOriginRedirect) {
		t.Fatalf("err = %v, want redirect limit error", err)
	}

	if err := icu.SameOriginRedirects(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://127.0.0.1/x", http.NoBody), via[:1]); !errors.Is(err, icu.ErrCrossOriginRedirect) {
		t.Fatalf("scheme change err = %v", err)
	}
}
