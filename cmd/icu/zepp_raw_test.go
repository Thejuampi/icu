package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

const rawEventsBody = `{"items":[{"timestamp":1761710400000,"eventType":"readiness","subType":"watch_score",` +
	`"userId":"SYN-USER","deviceId":"SYN-DEV","value":{"sleepHRV":38,"sn":"SYN-SN","appToken":"SYN-TOK"}}]}`

func TestZeppRawRedactsEventsAndReadiness(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != v2EventsPathCLI {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, rawEventsBody
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())
	t.Setenv("ZEPP_EVENTS_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cases := map[string]map[string]string{
		"events":    {"preset": "readiness", "raw": "true", "oldest": "2026-10-25", "newest": "2026-10-29"},
		"readiness": {"raw": "true", "oldest": "2026-10-25", "newest": "2026-10-29"},
	}
	for name, flags := range cases {
		cmd, ok := registry.Lookup("zepp", name)
		if !ok {
			t.Fatalf("no zepp %s", name)
		}
		var out bytes.Buffer
		if err := cmd.Run(nil, flags, nil, &out, io.Discard); err != nil {
			t.Fatalf("%s --raw: %v", name, err)
		}
		s := out.String()
		for _, secret := range []string{"SYN-USER", "SYN-DEV", "SYN-SN", "SYN-TOK", "test-app-token"} {
			if strings.Contains(s, secret) {
				t.Fatalf("%s --raw leaked %s: %s", name, secret, s)
			}
		}
		if !strings.Contains(s, `"sleepHRV":38`) || !strings.Contains(s, `"watch_score"`) {
			t.Fatalf("%s --raw lost payload: %s", name, s)
		}
	}

	// Without --raw the decoded output keeps the payload under extra.value.
	cmd, _ := registry.Lookup("zepp", "readiness")
	var out bytes.Buffer
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-10-25", "newest": "2026-10-29"}, nil, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"sleepHRV": 38`) && !strings.Contains(out.String(), `"sleepHRV":38`) {
		t.Fatalf("decoded readiness dropped payload: %s", out.String())
	}
}

func TestZeppRawRefusesNonJSON(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(zeppMockRequest) (int, string) { return http.StatusOK, "userId=SYN-USER" })
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())
	t.Setenv("ZEPP_EVENTS_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)
	cmd, _ := registry.Lookup("zepp", "readiness")
	var out bytes.Buffer
	err := cmd.Run(nil, map[string]string{"raw": "true"}, nil, &out, io.Discard)
	if err == nil || strings.Contains(out.String()+err.Error(), "SYN-USER") {
		t.Fatalf("err=%v out=%q", err, out.String())
	}
}
