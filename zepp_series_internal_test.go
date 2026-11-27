package icu

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDecodeSleepAdvancesDayWhenStartGoesBack(t *testing.T) {
	t.Parallel()

	// The first stage ends exactly at midnight (no wrap); the next one
	// starts earlier on the clock, so it is on the next day.
	raw := `{"stage":[{"start":1380,"stop":1439,"mode":4},{"start":10,"stop":100,"mode":5},{"start":100,"stop":200,"mode":4}]}`

	sleep, err := decodeSleep([]byte(raw), "2026-06-01")
	if err != nil {
		t.Fatalf("decodeSleep: %v", err)
	}

	next := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC).Unix()
	if got := sleep.Stages[1].StartEpoch; got != next+10*60 {
		t.Fatalf("stage 1 start = %d, want %d", got, next+10*60)
	}

	if got := sleep.Stages[2].StartEpoch; got != next+100*60 {
		t.Fatalf("stage 2 start = %d, want %d (no double advance)", got, next+100*60)
	}
}

func TestZeppItemLocation(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"":                 "2026-06-01",
		"America/New_York": "2026-05-31",
		"-5":               "2026-05-31",
		"+05:30":           "2026-06-01",
		"-03:00":           "2026-05-31",
		"not/a-zone":       "2026-06-01",
		"99":               "2026-06-01",
	}

	for name, want := range cases {
		if got := at.In(zeppItemLocation(name)).Format(time.DateOnly); got != want {
			t.Fatalf("%q: date = %s, want %s", name, got, want)
		}
	}
}

func TestZeppTrackIDStringDecodesStringNumberAndNull(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{`"17"`: "17", `17`: "17", `null`: "", ``: ""} {
		var got zeppTrackIDString
		if err := got.UnmarshalJSON([]byte(raw)); err != nil || string(got) != want {
			t.Fatalf("%q: got %q, err %v", raw, got, err)
		}
	}

	var bad zeppTrackIDString
	if err := bad.UnmarshalJSON([]byte(`{}`)); err == nil {
		t.Fatalf("expected an error for an object")
	}

	if err := bad.UnmarshalJSON([]byte(`"x`)); err == nil {
		t.Fatalf("expected an error for a broken string")
	}
}

func TestRedactZeppEndpoint(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"https://api-mifit.zepp.com/users/mock-user-42/events?userId=mock-user-42&from=1":              "https://api-mifit.zepp.com/users/{id}/events",
		"https://host/v2/watch/users/mock-user-42/WatchSportStatistics/SPORT_LOAD?startDay=2026-06-01": "https://host/v2/watch/users/{id}/WatchSportStatistics/SPORT_LOAD",
		"://bogus": "{redacted}",
	}
	for raw, want := range cases {
		if got := redactZeppEndpoint(raw); got != want {
			t.Fatalf("%q: got %q, want %q", raw, got, want)
		}
	}

	reg := "https://api-user.huami.com/registrations/" + "fixture@example.invalid" + "/tokens"
	if got := redactZeppEndpoint(reg); got != "https://api-user.huami.com/registrations/{id}/tokens" {
		t.Fatalf("registrations: got %q", got)
	}

	// Build the userinfo form at runtime so static scanners do not treat
	// it as a committed credential.
	raw := "https://" + "fixture-user" + ":" + "fixture-token" + "@host/users/me/bloodPressure?days=7"
	if got := redactZeppEndpoint(raw); got != "https://host/users/me/bloodPressure" {
		t.Fatalf("userinfo: got %q", got)
	}
}

func TestWrapZeppTransportWithoutURLError(t *testing.T) {
	t.Parallel()

	err := wrapZeppTransport("do request", "https://h/users/mock-user-42/events?userId=mock-user-42", "mock-user-42", errors.New("dial mock-user-42 failed"))
	if msg := err.Error(); strings.Contains(msg, "mock-user-42") || !strings.Contains(msg, "/users/{id}/events") {
		t.Fatalf("err = %q", msg)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestWrapZeppTransportPreservesUnwrapAndTimeout(t *testing.T) {
	t.Parallel()

	canceled := wrapZeppTransport("do request", "https://h/users/mock-user-42/events?userId=mock-user-42", "mock-user-42", context.Canceled)
	if !errors.Is(canceled, context.Canceled) {
		t.Fatalf("canceled: %v", canceled)
	}

	deadline := wrapZeppTransport("do request", "https://h/users/u1/events", "u1", context.DeadlineExceeded)
	if !errors.Is(deadline, context.DeadlineExceeded) {
		t.Fatalf("deadline: %v", deadline)
	}

	// The inner text still names the user id, so a redacted wrapper is used,
	// but Unwrap must still reach the sentinel (and the context error).
	inner := fmt.Errorf("dial mock-user-42 at https://h/users/mock-user-42/events: %w", context.Canceled)
	got := wrapZeppTransport("do request", "https://h/users/mock-user-42/events?userId=mock-user-42", "mock-user-42", inner)
	if !errors.Is(got, context.Canceled) {
		t.Fatalf("redacted wrapper lost Unwrap: %v", got)
	}
	if msg := got.Error(); strings.Contains(msg, "mock-user-42") {
		t.Fatalf("redacted wrapper leaked the user id: %q", msg)
	}

	timeout := wrapZeppTransport("do request", "https://h/v2/client/login", "", timeoutError{})
	var netErr net.Error
	if !errors.As(timeout, &netErr) || !netErr.Timeout() {
		t.Fatalf("timeout: err = %v, as net.Error = %v", timeout, netErr)
	}
}

func TestZeppLoginTransportErrorPreservesTimeoutAndOmitsEmail(t *testing.T) {
	// Listen but never Accept so the HTTP client blocks until its timeout,
	// without a handler that could pin httptest.Server.Close.
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	old := zeppAuthTimeout
	zeppAuthTimeout = 80 * time.Millisecond
	t.Cleanup(func() { zeppAuthTimeout = old })

	email := "fixture@example.invalid"
	base := "http://" + ln.Addr().String()
	_, err = ZeppLoginWithURLs(base+"/v2/registrations/tokens", base+"/v2/client/login", email, "synthetic-only")
	if err == nil {
		t.Fatalf("expected a timeout")
	}

	if msg := err.Error(); strings.Contains(msg, email) {
		t.Fatalf("login error leaked the email: %q", msg)
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("login err = %v, want a timeout or deadline", err)
		}
	}
}
