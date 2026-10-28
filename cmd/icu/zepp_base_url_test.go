package main

import (
	"bytes"
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
		case zeppTokensPath:
			w.Header().Set("Location", "https://example.com/callback?access=atok&country_code=US")
			w.WriteHeader(http.StatusSeeOther)
		case zeppLoginPathPart:
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
