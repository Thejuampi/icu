package icu_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// newLeakSink is a second loopback server that records any body or API key
// that reaches it.
func newLeakSink(t *testing.T) (*httptest.Server, *atomic.Value) {
	t.Helper()

	var got atomic.Value

	got.Store("")

	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		user, pass, _ := r.BasicAuth()
		got.Store(string(body) + user + pass)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token_info":{"login_token":"lt","app_token":"at","user_id":"u1"}}`))
	}))
	t.Cleanup(sink.Close)

	return sink, &got
}

func TestZeppLoginCrossHostRedirectDoesNotResendBody(t *testing.T) {
	t.Parallel()

	for _, code := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			t.Parallel()

			sink, leaked := newLeakSink(t)

			tokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "https://example.com/callback?access=secret-access-code&country_code=US")
				w.WriteHeader(http.StatusSeeOther)
			}))
			t.Cleanup(tokens.Close)

			login := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", sink.URL+"/v2/client/login")
				w.WriteHeader(code)
			}))
			t.Cleanup(login.Close)

			_, err := icu.ZeppLoginWithURLs(tokens.URL+"/v2/registrations/tokens", login.URL+"/v2/client/login", "u@example.com", "secret")

			if got, _ := leaked.Load().(string); got != "" {
				t.Fatalf("other host received %q", got)
			}

			if !errors.Is(err, icu.ErrCrossOriginRedirect) {
				t.Fatalf("ZeppLoginWithURLs error = %v, want ErrCrossOriginRedirect", err)
			}
		})
	}
}

func TestWithHTTPClientAddsSameOriginRedirects(t *testing.T) {
	t.Parallel()

	sink, leaked := newLeakSink(t)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/same") {
			_, _ = w.Write([]byte(`{"ok":true}`))

			return
		}

		if strings.HasSuffix(r.URL.Path, "/hop") {
			http.Redirect(w, r, "/api/v1/athlete/0/same", http.StatusFound)

			return
		}

		http.Redirect(w, r, sink.URL+"/away", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)

	injected := &http.Client{}
	client := icu.NewClient("secret-api-key", "0", icu.WithBaseURL(origin.URL), icu.WithHTTPClient(injected))

	var out map[string]any
	if err := client.Get("athlete", []string{"0", "hop"}, nil, &out); err != nil || out["ok"] != true {
		t.Fatalf("same-origin redirect: err=%v out=%v", err, out)
	}

	err := client.Get("athlete", []string{"0", "away"}, nil, &out)
	if got, _ := leaked.Load().(string); got != "" {
		t.Fatalf("other host received %q", got)
	}

	if !errors.Is(err, icu.ErrCrossOriginRedirect) {
		t.Fatalf("cross-origin redirect error = %v, want ErrCrossOriginRedirect", err)
	}

	if injected.CheckRedirect != nil {
		t.Fatal("WithHTTPClient mutated the caller's client")
	}
}

func TestWithHTTPClientKeepsExistingCheckRedirect(t *testing.T) {
	t.Parallel()

	sink, leaked := newLeakSink(t)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, sink.URL+"/away", http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	var calls int

	own := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		calls++

		return http.ErrUseLastResponse
	}}
	client := icu.NewClient("secret-api-key", "0", icu.WithBaseURL(origin.URL), icu.WithHTTPClient(own))

	var out map[string]any

	_ = client.Get("athlete", []string{"0"}, nil, &out)

	if calls != 1 {
		t.Fatalf("caller CheckRedirect calls = %d, want 1", calls)
	}

	if got, _ := leaked.Load().(string); got != "" {
		t.Fatalf("other host received %q", got)
	}
}
