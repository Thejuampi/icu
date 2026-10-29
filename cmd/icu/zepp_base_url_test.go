package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// newZeppAuthMock serves both Zepp auth endpoints on one host, as
// --zepp-base-url expects.
func newZeppAuthMock(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)

		switch r.URL.Path {
		case "/v2/registrations/tokens": // literal on purpose: pins the real Zepp path
			w.Header().Set("Location", "https://example.com/callback?access=atok&country_code=US")
			w.WriteHeader(http.StatusSeeOther)
		case "/v2/client/login": // literal on purpose: pins the real Zepp path
			_, _ = w.Write([]byte(`{"result":"ok","token_info":{"login_token":"lt","app_token":"at","user_id":"u1"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestZeppAuthUsesZeppBaseURLNotGlobalBaseURL(t *testing.T) {
	var zeppHits, icuHits atomic.Int32

	zepp := newZeppAuthMock(t, &zeppHits)
	icuSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		icuHits.Add(1)
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(icuSrv.Close)

	for _, action := range []string{"token", "login"} {
		for _, leading := range []bool{false, true} {
			withZeppTestEnv(t)
			t.Setenv("ZEPP_TOKENS_URL", "")
			t.Setenv("ZEPP_LOGIN_URL", "")

			before := zeppHits.Load()
			args := []string{"icu", "zepp", action, "--email", "u@example.com", "--password", "secret", "--zepp-base-url", zepp.URL + "/", "--base-url", icuSrv.URL}

			if leading {
				args = append([]string{"icu", "--base-url", icuSrv.URL, "zepp", action}, args[3:len(args)-2]...)
			}

			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != 0 {
				t.Fatalf("%s leading=%v: code=%d stderr=%q", action, leading, code, stderr.String())
			}

			if got := zeppHits.Load() - before; got != 2 {
				t.Fatalf("%s leading=%v: zepp mock hits = %d, want 2 (tokens + login)", action, leading, got)
			}

			if action == "token" && !strings.Contains(stdout.String(), `"loginToken": "lt"`) {
				t.Fatalf("token output = %q", stdout.String())
			}

			if action == "login" {
				cfg, err := icu.LoadConfig()
				if err != nil || cfg.ZeppLoginToken != "lt" {
					t.Fatalf("login config = %+v, err=%v", cfg, err)
				}
			}
		}
	}

	if n := icuHits.Load(); n != 0 {
		t.Fatalf("global --base-url leaked into Zepp: intervals mock hit %d times", n)
	}
}

func TestZeppLoginBaseURLFlagOverridesEnv(t *testing.T) {
	var hits atomic.Int32

	zepp := newZeppAuthMock(t, &hits)

	withZeppTestEnv(t)
	t.Setenv("ZEPP_TOKENS_URL", "http://127.0.0.1:1/v2/registrations/tokens")
	t.Setenv("ZEPP_LOGIN_URL", "http://127.0.0.1:1/v2/client/login")

	auth, err := zeppLogin("u@example.com", "secret", zepp.URL)
	if err != nil || auth.LoginToken != "lt" || hits.Load() != 2 {
		t.Fatalf("auth=%+v err=%v hits=%d", auth, err, hits.Load())
	}
}

func TestCheckZeppBaseURL(t *testing.T) {
	cases := []struct {
		raw     string
		wantErr bool
		warn    bool
	}{
		{"", false, false},
		{"http://127.0.0.1:8080", false, false},
		{"http://localhost:9", false, false},
		{"http://[::1]:9/", false, false},
		{"https://api-user.zepp.com", false, false},
		{"https://account.huami.com/", false, false},
		{"https://zepp.com.", false, false},
		{"https://evil.example.com", false, true},
		{"https://notzepp.com", false, true},
		{"https://zepp.com.evil.io", false, true},
		{"http://api-user.zepp.com", true, false},
		{"http://evil.example.com", true, false},
		{"ftp://127.0.0.1", true, false},
		{"127.0.0.1:8080", true, false},
		{"://bad", true, false},
	}

	for _, tc := range cases {
		var stderr bytes.Buffer

		err := checkZeppBaseURL(tc.raw, &stderr)
		if (err != nil) != tc.wantErr {
			t.Errorf("%q: err=%v, wantErr=%v", tc.raw, err, tc.wantErr)
		}

		if err != nil && !errors.Is(err, errInvalidValue) {
			t.Errorf("%q: err=%v, want errInvalidValue", tc.raw, err)
		}

		if got := strings.Contains(stderr.String(), "warning:"); got != tc.warn {
			t.Errorf("%q: warned=%v, want %v (stderr=%q)", tc.raw, got, tc.warn, stderr.String())
		}
	}
}

func TestZeppAuthRefusesPlainHTTPRemoteBeforeAnyRequest(t *testing.T) {
	withZeppTestEnv(t)

	var stdout, stderr bytes.Buffer

	code := run([]string{"icu", "zepp", "token", "--email", "u@example.com", "--password", "secret", "--zepp-base-url", "http://zepp.example.com"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "refuses plain http") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestIsLoopbackHost(t *testing.T) {
	t.Parallel()

	for host, want := range map[string]bool{
		"localhost":   true,
		"127.0.0.1":   true,
		"127.0.0.2":   true,
		"::1":         true,
		"10.0.0.5":    false,
		"192.168.1.2": false,
		"172.16.0.1":  false,
		"0.0.0.0":     false,
		"zepp.com":    false,
		"":            false,
	} {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
