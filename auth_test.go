package icu_test

import (
	"encoding/base64"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestBuildAuthHeader(t *testing.T) {
	t.Parallel()

	got := icu.BuildAuthHeader("test-api-key-123")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("API_KEY:test-api-key-123"))

	if got != want {
		t.Errorf("BuildAuthHeader() = %q, want %q", got, want)
	}
}

func TestBuildAuthHeaderEmpty(t *testing.T) {
	t.Parallel()

	got := icu.BuildAuthHeader("")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("API_KEY:"))

	if got != want {
		t.Errorf("BuildAuthHeader(\"\") = %q, want %q", got, want)
	}
}
