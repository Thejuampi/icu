package icu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixtures in testdata/zepp/real_shape keep the structure of a real
// readiness / HRVRMSSD / hrv_sdnn / Charge response (keys, nesting, types,
// 255 sentinels, empty hrv_sdnn) with every value synthetic and every
// identifier redacted. No real value or id is committed.
func readRealShape(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "zepp", "real_shape", name))
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func hrvByDay(days []ZeppWellnessDay) string {
	var b strings.Builder
	for _, d := range days {
		fmt.Fprintf(&b, "%s=%v ", d.Date, d.HRV)
	}

	return b.String()
}

func TestRealShapeHRVComesFromReadinessSleepHRV(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata")
	}
	sleep, err := DecodeSleepHRV(readRealShape(t, "readiness.json"))
	if err != nil {
		t.Fatal(err)
	}
	rmssd, err := DecodeHRV(readRealShape(t, "hrv-rmssd.json"), "rmssd")
	if err != nil {
		t.Fatal(err)
	}
	sdnn, err := DecodeHRV(readRealShape(t, "hrv-sdnn.json"), "sdnn")
	if err != nil {
		t.Fatal(err)
	}
	if len(sdnn) != 0 {
		t.Fatalf("hrv_sdnn is empty in the real shape, got %d events", len(sdnn))
	}
	charge, err := DecodeHRV(readRealShape(t, "hybridcharge.json"), "rmssd")
	if err != nil {
		t.Fatal(err)
	}
	if len(charge) != 0 {
		t.Fatalf("HybridCharge carries no HRV, got %+v", charge)
	}
	if len(sleep) != 5 || len(rmssd) != 5 {
		t.Fatalf("sleep=%d rmssd=%d, want 5/5", len(sleep), len(rmssd))
	}

	const want = "2030-01-14=41 2030-01-15=40 2030-01-16=36 2030-01-17=39 2030-01-18=47 "
	// Readiness wins whatever the order of the sources.
	for name, hrv := range map[string][]ZeppHRVEvent{
		"readiness first":  append(append([]ZeppHRVEvent{}, sleep...), rmssd...),
		"samples first":    append(append([]ZeppHRVEvent{}, rmssd...), sleep...),
		"with sdnn+charge": append(append(append(append([]ZeppHRVEvent{}, rmssd...), charge...), sdnn...), sleep...),
	} {
		if got := hrvByDay(BuildZeppWellnessDays(nil, hrv, loc, "2030-01-14", "2030-01-18")); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}

	// Without readiness, the sample mean (0 and 255 dropped) is the fallback.
	const fallback = "2030-01-14=52.3 2030-01-15=59 2030-01-16=49 2030-01-17=45 2030-01-18=57 "
	if got := hrvByDay(BuildZeppWellnessDays(nil, rmssd, loc, "2030-01-14", "2030-01-18")); got != fallback {
		t.Errorf("samples only: got %q, want %q", got, fallback)
	}
}
