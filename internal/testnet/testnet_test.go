package testnet_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Thejuampi/icu/internal/testnet"
)

func TestLoopbackOnly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer server.Close()

	client := &http.Client{Transport: testnet.LoopbackOnly{Base: &http.Transport{}}}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("loopback refused: %v", err)
	}
	_ = resp.Body.Close()

	for _, url := range []string{"https://intervals.icu/api/v1/athlete/0", "https://archive-api.open-meteo.com/v1/archive", "http://127.0.0.1:1/x"} {
		req, _ = http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)

		resp, err = client.Do(req)
		if resp != nil {
			_ = resp.Body.Close()
		}

		wantRefused := !strings.Contains(url, "127.0.0.1")
		if wantRefused && (err == nil || !strings.Contains(err.Error(), "refusing non-loopback")) {
			t.Errorf("%s: err = %v, want refusal", url, err)
		}

		if !wantRefused && (err == nil || strings.Contains(err.Error(), "refusing")) {
			t.Errorf("%s: err = %v, want a connection error, not a refusal", url, err)
		}
	}
}

func TestIsLoopback(t *testing.T) {
	t.Parallel()

	for host, want := range map[string]bool{"localhost": true, "LOCALHOST.": true, "127.0.0.1": true, "::1": true, "intervals.icu": false, "10.0.0.1": false} {
		if got := testnet.IsLoopback(host); got != want {
			t.Errorf("IsLoopback(%q) = %v", host, got)
		}
	}
}

//nolint:paralleltest // mutates http.DefaultTransport
func TestInstall(t *testing.T) {
	before := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = before })

	testnet.Install()

	if _, ok := http.DefaultTransport.(testnet.LoopbackOnly); !ok {
		t.Fatalf("DefaultTransport = %T", http.DefaultTransport)
	}
}
