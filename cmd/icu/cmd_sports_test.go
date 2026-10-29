package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// fakeSportServer mimics intervals.icu sport-settings on the wire: snake_case
// keys on GET, and PUT bodies with any unknown (e.g. camelCase) key rejected.
type fakeSportServer struct {
	mu           sync.Mutex
	settings     map[string]any
	puts         []map[string]json.RawMessage
	query        string
	ignoreWrites bool
}

func snakeSportSettings() map[string]any {
	return map[string]any{
		"id": 900001, "athlete_id": "i1", "types": []string{"Ride"},
		"ftp": 266, "indoor_ftp": 250, "lthr": 178, "max_hr": 198,
		"hr_zones":         []int{121, 148, 166, 177, 182, 187, 198},
		"hr_zone_names":    []string{"Z1", "Z2", "Z3", "Z4", "Z5a", "Z5b", "Z5c"},
		"power_zones":      []int{55, 75, 90, 105, 120, 150, 999},
		"power_zone_names": []string{"Z1", "Z2", "Z3", "Z4", "Z5", "Z6", "Z7"},
	}
}

func wireWritableSportKey(k string) bool {
	switch k {
	case "ftp", "indoor_ftp", "lthr", "max_hr", "hr_zones", "power_zones":
		return true
	}

	return false
}

func newFakeSportServer(t *testing.T, mutate func(map[string]any)) (*fakeSportServer, *icu.Client) {
	t.Helper()
	f := &fakeSportServer{settings: snakeSportSettings()}
	if mutate != nil {
		mutate(f.settings)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if !strings.HasSuffix(r.URL.Path, "/sport-settings/900001") && !strings.HasSuffix(r.URL.Path, "/sport-settings/Ride") {
			http.NotFound(w, r)

			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			raw, _ := io.ReadAll(r.Body)
			var body map[string]json.RawMessage
			if err := json.Unmarshal(raw, &body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}
			f.puts = append(f.puts, body)
			f.query = r.URL.RawQuery
			for k := range body {
				if !wireWritableSportKey(k) {
					http.Error(w, "unknown field "+k, http.StatusBadRequest)

					return
				}
			}
			if !f.ignoreWrites {
				for k, v := range body {
					var decoded any
					_ = json.Unmarshal(v, &decoded)
					f.settings[k] = decoded
				}
			}
		}
		_ = json.NewEncoder(w).Encode(f.settings)
	}))
	t.Cleanup(server.Close)

	return f, icu.NewClient("k", "i1", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
}

func putKeys(body map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func runSportsUpdateJSON(t *testing.T, client *icu.Client, flags map[string]string) (sportsUpdateResult, error) {
	t.Helper()
	out, err := captureStdout(t, func(stdout io.Writer) error {
		return runSportsUpdate([]string{"900001"}, flags, client, stdout, io.Discard)
	})
	var res sportsUpdateResult
	if strings.TrimSpace(out) != "" {
		if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
			t.Fatalf("unmarshal %q: %v", out, uerr)
		}
	}

	return res, err
}

func TestSportsUpdateHRZonesOnlyWritesByDefault(t *testing.T) {
	t.Parallel()
	f, client := newFakeSportServer(t, nil)
	res, err := runSportsUpdateJSON(t, client, map[string]string{"hr-zones": "120,148,166,177,182,187,198"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.puts) != 1 {
		t.Fatalf("puts = %d, want 1", len(f.puts))
	}
	if got := strings.Join(putKeys(f.puts[0]), ","); got != "hr_zones" {
		t.Fatalf("PUT keys = %s, want hr_zones", got)
	}
	if string(f.puts[0]["hr_zones"]) != "[120,148,166,177,182,187,198]" {
		t.Fatalf("hr_zones = %s", f.puts[0]["hr_zones"])
	}
	if f.query != "recalcHrZones=false" {
		t.Fatalf("query = %q", f.query)
	}
	if !res.Applied || res.ReadBack == nil || res.ReadBack.HRZones[0] != 120 || res.ReadBack.FTP != 266 || res.Message != "" {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Changes) != 1 || res.Changes[0].Field != "hr_zones" {
		t.Fatalf("changes = %+v", res.Changes)
	}
}

func TestSportsUpdateSendsSnakeCaseWireKeys(t *testing.T) {
	t.Parallel()
	f, client := newFakeSportServer(t, nil)
	_, err := runSportsUpdateJSON(t, client, map[string]string{
		"ftp": "270", "indoor-ftp": "255", "lthr": "179", "max-hr": "199",
		"hr-zones": "121,148,166,177,182,187,199", "power-zones": "56,76,91,106,121,151,999",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "ftp,hr_zones,indoor_ftp,lthr,max_hr,power_zones"
	if got := strings.Join(putKeys(f.puts[0]), ","); got != want {
		t.Fatalf("PUT keys = %s, want %s", got, want)
	}
}

func TestSportsUpdateDryRunDoesNotWrite(t *testing.T) {
	t.Parallel()
	f, client := newFakeSportServer(t, nil)
	res, err := runSportsUpdateJSON(t, client, map[string]string{
		"power-zones": "55,75,90,105,120,150,999", "lthr": "170", "dry-run": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.puts) != 0 || !res.DryRun || res.Applied {
		t.Fatalf("puts=%d res=%+v", len(f.puts), res)
	}
	if len(res.Body) != 1 || res.Body["lthr"] == nil || len(res.Changes) != 1 || res.Changes[0].Field != "lthr" {
		t.Fatalf("unchanged fields must be omitted, got %+v", res)
	}
}

func TestSportsUpdateNoChangesSkipsWrite(t *testing.T) {
	t.Parallel()
	f, client := newFakeSportServer(t, nil)
	res, err := runSportsUpdateJSON(t, client, map[string]string{"ftp": "266"})
	if err != nil || len(f.puts) != 0 || res.Applied {
		t.Fatalf("err=%v puts=%d res=%+v", err, len(f.puts), res)
	}
}

func TestSportsUpdateReadBackMismatchExitsNonZero(t *testing.T) {
	t.Parallel()
	f, client := newFakeSportServer(t, nil)
	f.ignoreWrites = true
	res, err := runSportsUpdateJSON(t, client, map[string]string{"max-hr": "199", "lthr": "179"})
	if err == nil || !strings.Contains(err.Error(), "lthr,max_hr") {
		t.Fatalf("err = %v, want read-back mismatch on lthr,max_hr", err)
	}
	if !res.Applied || !strings.Contains(res.Message, "read-back mismatch") {
		t.Fatalf("result = %+v", res)
	}
}

func TestReadBackMismatchDetectsEachField(t *testing.T) {
	t.Parallel()
	after := icu.SportSettings{
		FTP: 266, IndoorFTP: 250, LTHR: 178, MaxHR: 198,
		HRZones: []int{1, 2}, PowerZones: []int{3, 4},
	}
	match := map[string]any{
		"ftp": 266, "indoor_ftp": 250, "lthr": 178, "max_hr": 198,
		"hr_zones": []int{1, 2}, "power_zones": []int{3, 4},
	}
	if got := readBackMismatch(match, &after); got != "" {
		t.Fatalf("matching read-back reported %q", got)
	}
	for field, bad := range map[string]any{
		"ftp": 1, "indoor_ftp": 1, "lthr": 1, "max_hr": 1, "hr_zones": []int{1, 3}, "power_zones": []int{3},
	} {
		if got := readBackMismatch(map[string]any{field: bad}, &after); got != field {
			t.Fatalf("field %s: got %q", field, got)
		}
	}
}

func TestSportsUpdateHRZonesBoundedByEffectiveMaxHR(t *testing.T) {
	t.Parallel()
	_, client := newFakeSportServer(t, nil)
	if _, err := runSportsUpdateJSON(t, client, map[string]string{"hr-zones": "121,148,166,177,182,187,205"}); err == nil {
		t.Fatal("want error: zone above current max_hr 198")
	}
	f, client := newFakeSportServer(t, nil)
	if _, err := runSportsUpdateJSON(t, client, map[string]string{"hr-zones": "121,148,166,177,182,187,205", "max-hr": "205"}); err != nil {
		t.Fatalf("new max_hr in same command should allow 205: %v", err)
	}
	if got := strings.Join(putKeys(f.puts[0]), ","); got != "hr_zones,max_hr" {
		t.Fatalf("PUT keys = %s", got)
	}
}

func TestSportsUpdateZoneCountWhenCurrentZonesEmpty(t *testing.T) {
	t.Parallel()
	empty := func(s map[string]any) { delete(s, "hr_zones") }
	_, client := newFakeSportServer(t, empty)
	if _, err := runSportsUpdateJSON(t, client, map[string]string{"hr-zones": "121,148,166"}); err == nil {
		t.Fatal("want error: count must match 7 zone names")
	}
	f, client := newFakeSportServer(t, empty)
	if _, err := runSportsUpdateJSON(t, client, map[string]string{"hr-zones": "121,148,166,177,182,187,198"}); err != nil || len(f.puts) != 1 {
		t.Fatalf("err=%v puts=%d", err, len(f.puts))
	}
	_, client = newFakeSportServer(t, func(s map[string]any) { delete(s, "hr_zones"); delete(s, "hr_zone_names") })
	if _, err := runSportsUpdateJSON(t, client, map[string]string{"hr-zones": "121,148,166,177,182,187,198"}); err == nil {
		t.Fatal("want error: no zones and no zone names")
	}
}

func TestSportsUpdateValidationRejectsBeforeWrite(t *testing.T) {
	t.Parallel()
	cases := []map[string]string{
		{},
		{"hr-zones": "121,148,140,177,182,187,198"},
		{"hr-zones": "121,148,166"},
		{"hr-zones": "0,148,166,177,182,187,198"},
		{"power-zones": "55,75,x,105,120,150,999"},
		{"power-zones": "55,75,75,105,120,150,999"},
		{"power-zones": "55,75,90,105,120,150,1000"},
		{"ftp": "-5"},
		{"max-hr": "abc"},
	}
	for _, flags := range cases {
		f, client := newFakeSportServer(t, nil)
		if _, err := runSportsUpdateJSON(t, client, flags); err == nil {
			t.Fatalf("flags %v: want error", flags)
		}
		if len(f.puts) != 0 {
			t.Fatalf("flags %v: wrote despite invalid input", flags)
		}
	}
}

func TestFTPUpdateSendsSnakeCaseWireKeys(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		flags map[string]string
		want  string
	}{
		{map[string]string{"value": "270"}, "ftp"},
		{map[string]string{"value": "255", "indoor": "true"}, "indoor_ftp"},
	} {
		f, client := newFakeSportServer(t, nil)
		if _, err := captureStdout(t, func(stdout io.Writer) error { return ftpUpdateCommand().Run(nil, tc.flags, client, stdout, io.Discard) }); err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(putKeys(f.puts[0]), ","); got != tc.want {
			t.Fatalf("flags %v: PUT keys = %s, want %s", tc.flags, got, tc.want)
		}
	}
}

func TestParseZones(t *testing.T) {
	t.Parallel()
	z, err := parseZones("hr-zones", " 121, 148,166 ", "bpm", 0)
	if err != nil || !intsEqual(z, []int{121, 148, 166}) {
		t.Fatalf("got %v %v", z, err)
	}
}
