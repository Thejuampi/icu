package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	icu "github.com/Thejuampi/icu"
)

type fakeSportServer struct {
	mu       sync.Mutex
	settings map[string]any
	puts     []map[string]any
	query    string
}

func newFakeSportServer(t *testing.T) (*fakeSportServer, *icu.Client) {
	t.Helper()
	f := &fakeSportServer{settings: map[string]any{
		"id": 1858902, "ftp": 250, "lthr": 165, "maxHr": 190,
		"hrZones":    []int{120, 145, 160, 170, 178, 185, 190},
		"powerZones": []int{55, 75, 90, 105, 120, 150, 999},
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if !strings.HasSuffix(r.URL.Path, "/sport-settings/1858902") {
			http.NotFound(w, r)

			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			f.puts = append(f.puts, body)
			f.query = r.URL.RawQuery
			for k, v := range body {
				f.settings[k] = v
			}
		}
		_ = json.NewEncoder(w).Encode(f.settings)
	}))
	t.Cleanup(server.Close)

	return f, icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
}

func runSportsUpdateJSON(t *testing.T, client *icu.Client, flags map[string]string) (sportsUpdateResult, error) {
	t.Helper()
	out, err := captureStdout(t, func() error { return runSportsUpdate([]string{"1858902"}, flags, client) })
	var res sportsUpdateResult
	if err == nil {
		if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
			t.Fatalf("unmarshal %q: %v", out, uerr)
		}
	}

	return res, err
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestSportsUpdateHRZonesOnlyWithYes(t *testing.T) {
	f, client := newFakeSportServer(t)
	res, err := runSportsUpdateJSON(t, client, map[string]string{"hr-zones": "121,148,166,177,182,187,198", "yes": "true"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.puts) != 1 {
		t.Fatalf("puts = %d, want 1", len(f.puts))
	}
	if len(f.puts[0]) != 1 || f.puts[0]["hrZones"] == nil {
		t.Fatalf("body = %v, want only hrZones", f.puts[0])
	}
	if f.query != "recalcHrZones=false" {
		t.Fatalf("query = %q", f.query)
	}
	if !res.Applied || res.ReadBack == nil || res.ReadBack.HRZones[6] != 198 || res.ReadBack.FTP != 250 || res.Message != "" {
		t.Fatalf("result = %+v", res)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestSportsUpdatePreviewWithoutYesDoesNotWrite(t *testing.T) {
	for _, flags := range []map[string]string{
		{"power-zones": "55,75,90,105,120,150,999", "lthr": "170"},
		{"lthr": "170", "yes": "true", "dry-run": "true"},
	} {
		f, client := newFakeSportServer(t)
		res, err := runSportsUpdateJSON(t, client, flags)
		if err != nil {
			t.Fatal(err)
		}
		if len(f.puts) != 0 || !res.DryRun || res.Applied {
			t.Fatalf("flags %v: puts=%d res=%+v", flags, len(f.puts), res)
		}
		if len(res.Body) != 1 || len(res.Changes) != 1 || res.Changes[0].Field != "lthr" {
			t.Fatalf("flags %v: unchanged fields must be omitted, got %+v", flags, res)
		}
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestSportsUpdateNoChangesSkipsWrite(t *testing.T) {
	f, client := newFakeSportServer(t)
	res, err := runSportsUpdateJSON(t, client, map[string]string{"ftp": "250", "yes": "true"})
	if err != nil || len(f.puts) != 0 || res.Applied {
		t.Fatalf("err=%v puts=%d res=%+v", err, len(f.puts), res)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestSportsUpdateValidationRejectsBeforeWrite(t *testing.T) {
	cases := []map[string]string{
		{},
		{"hr-zones": "121,148,140,177,182,187,198"},
		{"hr-zones": "121,148,166"},
		{"hr-zones": "0,148,166,177,182,187,198"},
		{"hr-zones": "121,148,166,177,182,187,300"},
		{"power-zones": "55,75,x,105,120,150,999"},
		{"power-zones": "55,75,75,105,120,150,999"},
		{"ftp": "-5"},
		{"max-hr": "abc"},
	}
	for _, flags := range cases {
		flags["yes"] = "true"
		f, client := newFakeSportServer(t)
		if _, err := runSportsUpdateJSON(t, client, flags); err == nil {
			t.Fatalf("flags %v: want error", flags)
		}
		if len(f.puts) != 0 {
			t.Fatalf("flags %v: wrote despite invalid input", flags)
		}
	}
}

func TestParseZones(t *testing.T) {
	t.Parallel()
	z, err := parseZones("hr-zones", " 121, 148,166 ", "bpm", maxHRZoneBPM)
	if err != nil || !intsEqual(z, []int{121, 148, 166}) {
		t.Fatalf("got %v %v", z, err)
	}
}
