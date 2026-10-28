package icu_test

import (
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
