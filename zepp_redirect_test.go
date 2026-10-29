package icu_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func newRedirectPair(t *testing.T, sameOrigin bool) (*httptest.Server, *atomic.Value) {
	t.Helper()

	var leaked atomic.Value

	leaked.Store("")

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Store(r.Header.Get("Apptoken"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"message":"success","data":[]}`))
	}))
	t.Cleanup(other.Close)

	var origin *httptest.Server

	origin = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("moved") == "1" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"message":"success","data":[]}`))

			return
		}

		target := other.URL + "/moved"
		if sameOrigin {
			target = origin.URL + "/moved?moved=1"
		}

		http.Redirect(w, r, target, http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	return origin, &leaked
}

func newRedirectZeppClient(base string, options ...icu.ZeppClientOption) *icu.ZeppClient {
	auth := &icu.ZeppAuthResult{LoginToken: "tok-abc", AppToken: "secret-app-token", UserID: "u1", CountryCode: "US"}
	all := append([]icu.ZeppClientOption{icu.WithZeppBaseURL(base), icu.WithZeppEventsURL(base)}, options...)

	return icu.NewZeppClientFromAuth(auth, all...)
}

func TestZeppClientCrossHostRedirectDoesNotLeakApptoken(t *testing.T) {
	t.Parallel()

	for name, options := range map[string][]icu.ZeppClientOption{
		"default client":  nil,
		"injected client": {icu.WithZeppHTTPClient(&http.Client{})},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			origin, leaked := newRedirectPair(t, false)
			client := newRedirectZeppClient(origin.URL, options...)

			_, err := client.BandData(t.Context(), "2026-08-07", "2026-08-08")

			if got, _ := leaked.Load().(string); got != "" {
				t.Fatalf("other host received Apptoken %q", got)
			}

			if !errors.Is(err, icu.ErrCrossOriginRedirect) {
				t.Fatalf("BandData error = %v, want ErrCrossOriginRedirect", err)
			}
		})
	}
}

func TestZeppClientSameOriginRedirectStillFollowed(t *testing.T) {
	t.Parallel()

	origin, _ := newRedirectPair(t, true)
	client := newRedirectZeppClient(origin.URL)

	if _, err := client.BandData(t.Context(), "2026-08-07", "2026-08-08"); err != nil {
		t.Fatalf("BandData after same-origin redirect: %v", err)
	}
}
