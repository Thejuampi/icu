package icu

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// hangingServer accepts requests and never answers until the test ends.
func hangingServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })

	return srv
}

func shortZeppAuthTimeout(t *testing.T) {
	t.Helper()
	old := zeppAuthTimeout
	zeppAuthTimeout = 100 * time.Millisecond
	t.Cleanup(func() { zeppAuthTimeout = old })
}

func isTimeout(err error) bool {
	var netErr net.Error

	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Under the package's test transport the deadline surfaces as text.
	return errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline exceeded") ||
		strings.Contains(err.Error(), "Client.Timeout")
}

func TestZeppAuthTimeoutDefaultIs30s(t *testing.T) { //nolint:paralleltest // reads a test-overridable var
	if zeppAuthTimeout != 30*time.Second {
		t.Fatalf("zeppAuthTimeout = %v, want 30s", zeppAuthTimeout)
	}
}

func TestZeppLoginStepTimesOut(t *testing.T) { //nolint:paralleltest // overrides zeppAuthTimeout
	shortZeppAuthTimeout(t)
	srv := hangingServer(t)
	start := time.Now()
	err := withDeadline(t, func() error {
		_, err := performLoginWithURL(srv.URL, "synthetic-access", "US")

		return err
	})
	if err == nil || !isTimeout(err) {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("login step took %v", elapsed)
	}
}

func TestZeppTokensStepTimesOut(t *testing.T) { //nolint:paralleltest // overrides zeppAuthTimeout
	shortZeppAuthTimeout(t)
	srv := hangingServer(t)
	start := time.Now()
	err := withDeadline(t, func() error {
		_, _, err := getAccessTokenWithURL(srv.URL, "user@example.com", "synthetic-password")

		return err
	})
	if err == nil || !isTimeout(err) {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("tokens step took %v", elapsed)
	}
}

// withDeadline runs fn and fails the test (instead of hanging until the go
// test timeout) if fn has not returned within 10s.
func withDeadline(t *testing.T, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("call did not return within 10s: the auth timeout is not applied")

		return nil
	}
}
