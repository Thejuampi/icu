package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// zoneEventServer is a synthetic intervals.icu: event 1 (type Ride) and, when
// settings is non-empty, GET sport-settings/Ride returns it; otherwise 404.
// settings "status:N" answers N, and "network" drops the connection.
type zoneEventServer struct {
	mu       sync.Mutex
	settings string
	methods  []string
	paths    []string
	puts     []string
}

func (s *zoneEventServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.methods = append(s.methods, r.Method)
	s.paths = append(s.paths, r.URL.Path)
	if r.Method == http.MethodPut {
		s.puts = append(s.puts, string(body))
	}
	s.mu.Unlock()
	switch {
	case strings.HasSuffix(r.URL.Path, "/sport-settings/Ride"):
		if s.settings == "" {
			w.WriteHeader(http.StatusNotFound)

			return
		}
		if code, ok := strings.CutPrefix(s.settings, "status:"); ok {
			status, _ := strconv.Atoi(code)
			w.WriteHeader(status)

			return
		}
		if s.settings == "network" {
			if hijacker, ok := w.(http.Hijacker); ok {
				if conn, _, err := hijacker.Hijack(); err == nil {
					_ = conn.Close()
				}
			}

			return
		}
		_, _ = io.WriteString(w, s.settings)
	case strings.HasSuffix(r.URL.Path, "/events/1"):
		_, _ = io.WriteString(w, `{"id":1,"type":"Ride","name":"Synthetic","description":"- 10m 50%"}`)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// runZoneUpdate runs events update 1 --calculate-load --ftp 266 and returns
// stdout, stderr, the server and the error. Backups go to a temp config dir.
func runZoneUpdate(t *testing.T, settings, desc string, extra ...string) (string, string, *zoneEventServer, error) {
	t.Helper()
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	srv := &zoneEventServer{settings: settings}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(ts.Client()), icu.WithBaseURL(ts.URL))
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	cmd, _ := registry.Lookup("events", "update")
	var stdout, stderr bytes.Buffer
	err := cmd.Run([]string{"1"}, flags(append([]string{"desc", desc, "calculate-load", "true", "ftp", "266"}, extra...)...), client, &stdout, &stderr)

	return stdout.String(), stderr.String(), srv, err
}

func putLoad(t *testing.T, srv *zoneEventServer) float64 {
	t.Helper()
	if len(srv.puts) != 1 {
		t.Fatalf("puts %v (%v %v)", srv.puts, srv.methods, srv.paths)
	}
	var body struct {
		Load float64 `json:"icu_training_load"`
		Time int     `json:"moving_time"`
	}
	if err := json.Unmarshal([]byte(srv.puts[0]), &body); err != nil || body.Load <= 0 {
		t.Fatalf("put %s err=%v", srv.puts[0], err)
	}

	return body.Load
}

func zoneLoad(t *testing.T, settings, desc string) float64 {
	t.Helper()
	result := runZoneUpdateResult(t, settings, desc)
	if result.err != nil {
		t.Fatalf("%q: %v", desc, result.err)
	}

	return putLoad(t, result.srv)
}

type zoneRun struct {
	srv *zoneEventServer
	err error
}

func runZoneUpdateResult(t *testing.T, settings, desc string) zoneRun {
	t.Helper()
	_, _, srv, err := runZoneUpdate(t, settings, desc)

	return zoneRun{srv: srv, err: err}
}

// Synthetic non-default bands: Z2 = (51+70)/2 = 60.5% here, 65.5% by default.
const syntheticRideZones = `{"types":["Ride"],"ftp":266,"power_zones":[50,70,85,100,999]}`

//nolint:paralleltest // t.Setenv
func TestEventsUpdateZonesFromSportSettings(t *testing.T) {
	out, stderr, srv, err := runZoneUpdate(t, syntheticRideZones, "- 60m Z2")
	if err != nil || strings.Contains(stderr, "note:") {
		t.Fatalf("err=%v stderr=%s", err, stderr)
	}
	custom := putLoad(t, srv)
	var got calculatedEventResponse
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Zones == nil || got.Zones.Source != "sport-settings" ||
		got.Zones.Sport != "Ride" || got.Zones.Midpoints[1] != 60.5 || got.Zones.Note != "" {
		t.Fatalf("json %s err=%v", out, err)
	}
	if zoneLoad(t, syntheticRideZones, "- 60m 60.5%") != custom {
		t.Fatalf("Z2 from settings must price as 60.5%%")
	}
	if _, _, _, err := runZoneUpdate(t, syntheticRideZones, "- 10m Z1\n- 10m Z6"); err == nil ||
		!strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "Z1-Z5") {
		t.Fatalf("Z6 with five zones: %v", err)
	}
}

//nolint:paralleltest // t.Setenv
func TestEventsUpdateZonesFallbackIsAnnounced(t *testing.T) {
	for name, settings := range map[string]string{
		"404": "", "missing power_zones": `{"types":["Ride"],"ftp":266}`, "empty power_zones": `{"types":["Ride"],"power_zones":[]}`,
	} {
		out, stderr, srv, err := runZoneUpdate(t, settings, "- 60m Z2")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(stderr, "note: could not use Ride sport-settings power zones") || !strings.Contains(stderr, "default bands") {
			t.Fatalf("%s: stderr %q", name, stderr)
		}
		var got calculatedEventResponse
		if json.Unmarshal([]byte(out), &got) != nil || got.Zones == nil || got.Zones.Source != "default-fallback" ||
			got.Zones.Note == "" || got.Zones.Midpoints[1] != 65.5 {
			t.Fatalf("%s: json %s", name, out)
		}
		if putLoad(t, srv) != zoneLoad(t, settings, "- 60m 65.5%") {
			t.Fatalf("%s: fallback Z2 must price as 65.5%%", name)
		}
	}
	// Dry run: same note and field, no PUT.
	out, stderr, srv, err := runZoneUpdate(t, "", "- 60m Z2", "dry-run", "true")
	if err != nil || !strings.Contains(stderr, "note:") || !strings.Contains(out, `"default-fallback"`) || len(srv.puts) != 0 {
		t.Fatalf("dry run err=%v stderr=%s out=%s puts=%v", err, stderr, out, srv.puts)
	}
}

//nolint:paralleltest // t.Setenv
func TestEventsUpdateWithoutZonesSkipsSportSettings(t *testing.T) {
	out, stderr, srv, err := runZoneUpdate(t, syntheticRideZones, "- 10m 110-130w easy spin")
	if err != nil || stderr != "" || strings.Contains(out, "powerZones") {
		t.Fatalf("err=%v stderr=%s out=%s", err, stderr, out)
	}
	for _, p := range srv.paths {
		if strings.Contains(p, "sport-settings") {
			t.Fatalf("no zone step: sport-settings must not be read (%v)", srv.paths)
		}
	}
	if putLoad(t, srv) != zoneLoad(t, "", "- 10m 120w") {
		t.Fatal("110-130w must price as 120w")
	}
}

//nolint:paralleltest // t.Setenv
func TestEventsUpdateRejectsBeforeWriting(t *testing.T) {
	for desc, want := range map[string]string{
		"- 10m Z1\n- 10m easy":      "line 2",
		"- 10m 50%\n- 30m Z2 HR":    "HR, LTHR or Pace",
		"- 1m 99999w":               "above 3000 W",
		"- 10m 50%\n- 1m 1001%":     "line 2",
		"- 10m Z3-Z4":               "zone ranges",
		"- 10m 50%\n- 30m 70% Pace": "line 2",
	} {
		_, _, srv, err := runZoneUpdate(t, syntheticRideZones, desc)
		if err == nil || !strings.Contains(err.Error(), want) || len(srv.puts) != 0 {
			t.Fatalf("%q: err=%v puts=%v", desc, err, srv.puts)
		}
	}
}

//nolint:paralleltest // captureStdout
func TestWorkoutsCalculateZonesNote(t *testing.T) {
	cmd := workoutsCalculateCommand()
	var stderr bytes.Buffer
	if err := cmd.Run(nil, flags("ftp", "266", "desc", "- 10m Z2"), nil, io.Discard, &stderr); err != nil ||
		!strings.Contains(stderr.String(), "runs offline") {
		t.Fatalf("err=%v stderr=%s", err, stderr.String())
	}
	if _, err := workoutEstimateFromFlags(flags("ftp", "266", "desc", "- 1m 99999w")); err == nil {
		t.Fatal("watt cap")
	}
}

// Hugo's m2 decision: only "no zones configured" falls back; 401, 403, 5xx
// and network failures abort before any write.
//
//nolint:paralleltest // t.Setenv
func TestEventsUpdateZonesReadFailureWritesNothing(t *testing.T) {
	for _, settings := range []string{"status:401", "status:403", "status:500", "status:503", "network"} {
		_, stderr, srv, err := runZoneUpdate(t, settings, "- 60m Z2")
		if err == nil || !strings.Contains(err.Error(), "read Ride sport-settings power zones (nothing written") ||
			len(srv.puts) != 0 || strings.Contains(stderr, "default bands") {
			t.Fatalf("%s: err=%v stderr=%q puts=%v", settings, err, stderr, srv.puts)
		}
		srv.mu.Lock()
		methods := append([]string(nil), srv.methods...)
		srv.mu.Unlock()
		for _, method := range methods {
			if method != http.MethodGet {
				t.Fatalf("%s: only GETs allowed, got %v", settings, methods)
			}
		}
		_, _, srv, err = runZoneUpdate(t, settings, "- 60m Z2", "dry-run", "true")
		if err == nil || len(srv.puts) != 0 {
			t.Fatalf("%s dry run: err=%v puts=%v", settings, err, srv.puts)
		}
	}
}

// Above 300% FTP is a warning (stderr and JSON), not a rejection.
//
//nolint:paralleltest // t.Setenv
func TestEventsUpdateHighTargetWarns(t *testing.T) {
	out, stderr, srv, err := runZoneUpdate(t, syntheticRideZones, "- 10m 50%\n- 10s 1000w")
	if err != nil || len(srv.puts) != 1 {
		t.Fatalf("1000w at FTP 266 must pass: err=%v puts=%v", err, srv.puts)
	}
	const want = "line 2: 1000w is above 300% of FTP 266 (798w); check for a typo"
	var got calculatedEventResponse
	if !strings.Contains(stderr, "warning: "+want) || json.Unmarshal([]byte(out), &got) != nil || !slicesContains(got.Warnings, want) {
		t.Fatalf("stderr=%q out=%s", stderr, out)
	}
	out, stderr, srv, err = runZoneUpdate(t, syntheticRideZones, "- 10s 1000w", "dry-run", "true")
	if err != nil || len(srv.puts) != 0 || !strings.Contains(stderr, "warning: line 1") || !strings.Contains(out, `"warnings"`) {
		t.Fatalf("dry run: err=%v stderr=%q out=%s", err, stderr, out)
	}
}

func slicesContains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}

	return false
}

//nolint:paralleltest // captureStdout
func TestWorkoutsCalculateHighTargetWarns(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := workoutsCalculateCommand().Run(nil, flags("ftp", "266", "desc", "- 10s 1000w"), nil, &stdout, &stderr)
	if err != nil || !strings.Contains(stderr.String(), "warning: line 1: 1000w") || !strings.Contains(stdout.String(), `"warnings"`) {
		t.Fatalf("err=%v stderr=%q out=%s", err, stderr.String(), stdout.String())
	}
}

// m2: bad sport-settings bounds abort before any write; m4: the 404 note
// names the requested type.
//
//nolint:paralleltest // t.Setenv
func TestEventsUpdateInvalidZonesAndNoteType(t *testing.T) {
	for _, zones := range []string{"[0,75,999]", "[90,75,55,999]", "[55,75,999,999]"} {
		_, _, srv, err := runZoneUpdate(t, `{"types":["Ride"],"power_zones":`+zones+`}`, "- 60m Z2")
		if err == nil || !strings.Contains(err.Error(), "invalid power zones") || !strings.Contains(err.Error(), "nothing written") || len(srv.puts) != 0 {
			t.Fatalf("%s: err=%v puts=%v", zones, err, srv.puts)
		}
	}
	_, stderr, _, err := runZoneUpdate(t, "", "- 60m Z2", "dry-run", "true")
	if err != nil || !strings.Contains(stderr, "sport-settings for requested type Ride not found (404)") {
		t.Fatalf("err=%v stderr=%q", err, stderr)
	}
}
