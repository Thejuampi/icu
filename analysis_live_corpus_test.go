package icu_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// TestAnalyzeCyclingActivitiesMatchesLiveCorpusGolden is W2-R01, the wave-2
// split invariant (T2.LIVE): testdata/wave2_live_corpus.json is a read-only
// GET snapshot of Juan's real intervals.icu activity history — 303 raw
// records, 73 of them D-24 STRAVA stubs (5 keys, "_note" present,
// source=="STRAVA") — fetched via the CLI's own credentials at
// C:\Users\Juan\.icu\config.json and frozen against commit
// b8921333954b1bcad25236113972185262b62caa (wave-1b + D-7 addendum, the
// commit wave-2 built on top of). No write of any kind was made to the
// live account to produce this fixture. Regenerate only via explicit
// reviewed action (re-fetch, re-freeze, re-diff both halves below by hand);
// never regenerate to silence a failing assertion.
//
// The golden file is every field AnalyzeCyclingActivities emits for this
// exact corpus today. Diffing it against the same call run at
// b8921333954b1bcad25236113972185262b62caa (reconstructed via
// `git show b8921333954b1bcad25236113972185262b62caa:analysis.go` compiled
// against this same corpus) established the split invariant:
//
//   - Bit-identity half: every pre-existing key's value is byte-identical
//     before/after wave-2 — volume, powerAnchors, environment, load.total/
//     averagePerActivity/perHour/monotony/strain/acute7/chronic28/
//     acuteChronicWorkRatio/daily, intensity.*, durability.*, anaerobic.*,
//     dateRange, state. This is expected, not coincidental: every
//     pre-wave-2 average already divided by Scope.CyclingActivities (a
//     count built by looping activities and skipping non-cycling Type),
//     and a STRAVA stub's Type is always "" — already failing
//     IsCyclingActivityType before wave-2 ever existed. validity.excluded
//     is 0 and every excludedByFlag/excludedByScope count is empty: F-19's
//     thesis that icu_ignore_* is measured false across the live corpus
//     holds here too.
//   - Expected-delta half, every activity-count denominator that stub
//     contamination touched, found by searching the diff (not by recall):
//     scope.activities 303 -> 230 (the one field older code fed directly
//     from len(activities) with no cycling-type filter and no stub
//     filter); plus wave-2's wholly new fields with no pre-wave-2
//     equivalent at all: validity (evaluated/included/excluded/
//     unavailable.stravaStubs=73/zeroSemantics/source/fieldsGuaranteed),
//     load.nIncluded, load.byChannel (trainingLoad/hr/power/pace),
//     load.divergence, intensity.nIncluded, durability.nIncluded.
func TestAnalyzeCyclingActivitiesMatchesLiveCorpusGolden(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/wave2_live_corpus.json")
	if err != nil {
		t.Fatalf("read live corpus fixture: %v", err)
	}

	var activities []icu.Activity
	if err := json.Unmarshal(raw, &activities); err != nil {
		t.Fatalf("unmarshal live corpus fixture: %v", err)
	}

	got, err := json.MarshalIndent(icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{}), "", "  ")
	if err != nil {
		t.Fatalf("marshal analysis: %v", err)
	}

	want, err := os.ReadFile("testdata/wave2_live_corpus_cycling_analysis.golden.json")
	if err != nil {
		t.Fatalf("read golden fixture: %v", err)
	}

	if string(got) != string(want) {
		gotPath := filepath.Join(t.TempDir(), "got.json")
		if writeErr := os.WriteFile(gotPath, got, 0o600); writeErr != nil {
			t.Fatalf("analysis output does not match testdata/wave2_live_corpus_cycling_analysis.golden.json (also failed to write %s for diffing: %v)", gotPath, writeErr)
		}
		t.Fatalf("analysis output does not match testdata/wave2_live_corpus_cycling_analysis.golden.json; actual output written to %s for diffing", gotPath)
	}
}
