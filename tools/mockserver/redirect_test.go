package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newRedirectTestServer is the mock with redirect mode on: the sink is a
// second httptest listener, like the second port -redirect-sink opens.
func newRedirectTestServer(t *testing.T) (*httptest.Server, *httptest.Server) {
	t.Helper()

	dir := t.TempDir()
	writeFixture(t, dir, "api/v1/athlete/0.json", `{"id":"i0"}`)

	srv, err := newServer(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	sink := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(sink.Close)

	srv.sink = newRedirectSink(sink.URL)
	sink.Config.Handler = srv.sink

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	return ts, sink
}

func noFollowClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// sent is the status and headers of one response (its body already closed).
type sent struct {
	StatusCode int
	Header     http.Header
}

func send(t *testing.T, client *http.Client, method, target, body string, header map[string]string) sent {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	for name, value := range header {
		req.Header.Set(name, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	return sent{StatusCode: resp.StatusCode, Header: resp.Header}
}

type leakReport struct {
	Count            int    `json:"count"`
	CodeReceived     bool   `json:"codeReceived"`
	APIKeyReceived   bool   `json:"apiKeyReceived"`
	AppTokenReceived bool   `json:"appTokenReceived"`
	Requests         []Leak `json:"requests"`
}

func leaks(t *testing.T, base string) leakReport {
	t.Helper()

	code, body := do(t, http.MethodGet, base+"/_mock/leaks", "")
	if code != http.StatusOK {
		t.Fatalf("leaks = %d %s", code, body)
	}

	var report leakReport
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		t.Fatal(err)
	}

	return report
}

func TestRedirectModeOffKeepsNormalBehaviour(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	if code, _ := do(t, http.MethodGet, ts.URL+"/_mock/leaks", ""); code != http.StatusNotFound {
		t.Fatalf("leaks without redirect mode = %d, want 404", code)
	}

	if resp := send(t, noFollowClient(), http.MethodGet, ts.URL+"/_redirect/cross307/api/v1/athlete/0", "", nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("redirect path without redirect mode = %d, want 404", resp.StatusCode)
	}
}

func TestRedirectModeLocations(t *testing.T) {
	t.Parallel()

	ts, sink := newRedirectTestServer(t)

	cases := []struct {
		path, location string
		status         int
	}{
		{"/_redirect/cross307/v2/client/login?x=1", sink.URL + "/v2/client/login?x=1", http.StatusTemporaryRedirect},
		{"/_redirect/cross308/api/v1/athlete/0", sink.URL + "/api/v1/athlete/0", http.StatusPermanentRedirect},
		{"/_redirect/same307/v2/client/login", "/v2/client/login", http.StatusTemporaryRedirect},
		{"/_redirect/same308/v1/data/band_data.json", "/v1/data/band_data.json", http.StatusPermanentRedirect},
		{"/_redirect/cross307-tokens/v2/registrations/tokens", sink.URL + "/v2/registrations/tokens", http.StatusTemporaryRedirect},
	}

	for _, tc := range cases {
		resp := send(t, noFollowClient(), http.MethodPost, ts.URL+tc.path, "a=b", nil)
		if resp.StatusCode != tc.status || resp.Header.Get("Location") != tc.location {
			t.Errorf("%s = %d %q, want %d %q", tc.path, resp.StatusCode, resp.Header.Get("Location"), tc.status, tc.location)
		}
	}

	if resp := send(t, noFollowClient(), http.MethodGet, ts.URL+"/_redirect/sideways307/x", "", nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown mode = %d, want 404", resp.StatusCode)
	}

	if resp := send(t, noFollowClient(), http.MethodGet, ts.URL+"/_redirect/cross301/x", "", nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown status = %d, want 404", resp.StatusCode)
	}

	// The tokens endpoint under a plain mode is served normally (303 with the
	// synthetic access code), so a login reaches the code-bearing call.
	resp := send(t, noFollowClient(), http.MethodPost, ts.URL+"/_redirect/cross307/v2/registrations/tokens", "", nil)
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "access="+mockZeppAccessCode) {
		t.Fatalf("tokens under cross307 = %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	if got := leaks(t, ts.URL); got.Count != 0 {
		t.Fatalf("nothing followed, sink count = %d", got.Count)
	}
}

func TestRedirectModeSyntheticZeppLogin(t *testing.T) {
	t.Parallel()

	ts, _ := newRedirectTestServer(t)

	form := url.Values{"code": {mockZeppAccessCode}}.Encode()
	if resp := send(t, noFollowClient(), http.MethodPost, ts.URL+mockZeppLoginPath, form, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("login with code = %d", resp.StatusCode)
	}

	if resp := send(t, noFollowClient(), http.MethodPost, ts.URL+mockZeppLoginPath, "code=wrong", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login with wrong code = %d", resp.StatusCode)
	}

	// Same-origin redirect completes: a following client ends on the login.
	follow := &http.Client{}
	if resp := send(t, follow, http.MethodPost, ts.URL+"/_redirect/same307"+mockZeppLoginPath, form, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("same-origin login = %d", resp.StatusCode)
	}

	if got := leaks(t, ts.URL); got.Count != 0 {
		t.Fatalf("same-origin redirect reached the sink: %+v", got)
	}
}

func TestRedirectSinkRecordsAndRedacts(t *testing.T) {
	t.Parallel()

	ts, _ := newRedirectTestServer(t)

	// A client that follows cross-host redirects (what the fix prevents)
	// leaks everything into the sink: the report must say so without echoing
	// the secret values.
	follow := &http.Client{}
	send(t, follow, http.MethodPost, ts.URL+"/_redirect/cross307"+mockZeppLoginPath+"?code=q-secret", "code=body-secret",
		map[string]string{"Apptoken": "token-secret", "Authorization": "Basic key-secret"})

	report := leaks(t, ts.URL)
	if report.Count != 1 || !report.CodeReceived || !report.AppTokenReceived {
		t.Fatalf("report = %+v", report)
	}

	got := report.Requests[0]
	if got.Method != http.MethodPost || got.Path != mockZeppLoginPath || got.Headers["Apptoken"] != redactedValue ||
		len(got.FormKeys) != 1 || got.FormKeys[0] != "code" || got.QueryKeys[0] != "code" {
		t.Fatalf("leak = %+v", got)
	}

	_, raw := do(t, http.MethodGet, ts.URL+"/_mock/leaks", "")
	for _, secret := range []string{"q-secret", "body-secret", "token-secret", "key-secret"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("leak report echoes %q: %s", secret, raw)
		}
	}

	if code, _ := do(t, http.MethodPost, ts.URL+"/_mock/reset", ""); code != http.StatusOK {
		t.Fatalf("reset = %d", code)
	}

	if got := leaks(t, ts.URL); got.Count != 0 || got.CodeReceived {
		t.Fatalf("after reset = %+v", got)
	}
}

func TestRedirectSinkReportsAPIKey(t *testing.T) {
	t.Parallel()

	ts, sink := newRedirectTestServer(t)

	send(t, noFollowClient(), http.MethodGet, sink.URL+"/api/v1/athlete/0", "", map[string]string{"Authorization": "Basic key-secret"})

	report := leaks(t, ts.URL)
	if !report.APIKeyReceived || report.Requests[0].Headers["Authorization"] != redactedValue || report.Requests[0].FormKeys != nil {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunRedirectSink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "api/v1/athlete/0.json", `{"id":"i0"}`)

	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan int, 1)

	go func() { done <- run(ctx, []string{"-fixtures", dir, "-redirect-sink"}, out, io.Discard) }()

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "listening on") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	var mainURL, sinkURL string

	for line := range strings.SplitSeq(out.String(), "\n") {
		if after, ok := strings.CutPrefix(line, "icu-mock listening on "); ok {
			mainURL = after
		}

		if after, ok := strings.CutPrefix(line, "icu-mock redirect sink on "); ok {
			sinkURL = after
		}
	}

	if mainURL == "" || sinkURL == "" || mainURL == sinkURL {
		t.Fatalf("output = %q", out.String())
	}

	resp := send(t, noFollowClient(), http.MethodPost, mainURL+"/_redirect/cross308"+mockZeppLoginPath, "code=x", nil)
	if resp.StatusCode != http.StatusPermanentRedirect || resp.Header.Get("Location") != sinkURL+mockZeppLoginPath {
		t.Fatalf("redirect = %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	cancel()

	if code := <-done; code != 0 {
		t.Fatalf("run = %d", code)
	}

	if code := run(context.Background(), []string{"-fixtures", dir, "-redirect-sink", "-host", "256.0.0.1"}, io.Discard, io.Discard); code != 1 {
		t.Fatalf("bad host with sink = %d", code)
	}
}
