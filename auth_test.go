package main

import (
	"encoding/base64"
	"testing"
)

func TestBuildAuthHeader(t *testing.T) {
	got := BuildAuthHeader("test-api-key-123")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("API_KEY:test-api-key-123"))
	if got != want {
		t.Errorf("BuildAuthHeader() = %q, want %q", got, want)
	}
}

func TestBuildAuthHeaderEmpty(t *testing.T) {
	got := BuildAuthHeader("")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("API_KEY:"))
	if got != want {
		t.Errorf("BuildAuthHeader(\"\") = %q, want %q", got, want)
	}
}
