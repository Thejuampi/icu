package icu_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
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

// wave2SplitInvariantChangedKey is T2.LIVE/A-2's single enumerated
// pre-existing key allowed to change value between the pre-wave-2 and
// post-wave-2 goldens below.
const wave2SplitInvariantChangedKey = "scope.activities"

// wave2SplitInvariantNewKeyPrefixes is T2.LIVE/A-2's literal enumerated table
// of the only wholly new leaf-path prefixes wave-2 is allowed to have
// introduced. This table is a planning artifact, not a test-time judgement
// call (A-3): a leaf that is new and matches none of these prefixes is a
// planning gap to report, never something to add here to make a run pass.
var wave2SplitInvariantNewKeyPrefixes = []string{ //nolint:gochecknoglobals // fixed enumerated table (A-2/A-3), see doc comment above.
	"validity",
	"load.byChannel",
	"load.divergence",
	"load.nIncluded",
	"intensity.nIncluded",
	"durability.nIncluded",
}

// loadJSONDocument reads and unmarshals a JSON fixture into a generic
// document tree for structural comparison; it fails the test immediately on
// any read or parse error rather than letting a caller dereference a nil
// document.
func loadJSONDocument(t *testing.T, path string) interface{} {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var doc interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}

	return doc
}

// flattenJSONLeaves walks a generic JSON document tree (as produced by
// json.Unmarshal into interface{}) and records every scalar leaf under a
// dotted/bracketed path key (e.g. "load.byChannel.hr.total",
// "load.daily[3].date"), so two documents can be compared key-by-key instead
// of as an opaque whole-document diff.
func flattenJSONLeaves(prefix string, value interface{}, leaves map[string]interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, val := range typed {
			childPath := key
			if prefix != "" {
				childPath = prefix + "." + key
			}
			flattenJSONLeaves(childPath, val, leaves)
		}
	case []interface{}:
		for index, val := range typed {
			flattenJSONLeaves(fmt.Sprintf("%s[%d]", prefix, index), val, leaves)
		}
	default:
		leaves[prefix] = value
	}
}

// leafPathHasAllowedNewPrefix reports whether path is, or is nested under,
// one of wave2SplitInvariantNewKeyPrefixes.
func leafPathHasAllowedNewPrefix(path string) bool {
	for _, prefix := range wave2SplitInvariantNewKeyPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+".") || strings.HasPrefix(path, prefix+"[") {
			return true
		}
	}

	return false
}

// TestWaveTwoSplitInvariantBitIdentityHalf is T2.LIVE/A-2's bit-identity half,
// mechanised rather than left as prose in a doc comment.
// testdata/wave2_pre_b892133_cycling_analysis.golden.json is the frozen
// output of AnalyzeCyclingActivities compiled at commit
// b8921333954b1bcad25236113972185262b62caa (wave-1b + D-7 addendum, the
// commit wave-2 built on top of) run over the exact same corpus as
// testdata/wave2_live_corpus_cycling_analysis.golden.json. Like the live
// corpus fixture itself, it must never be regenerated to silence a failing
// assertion; regenerate only via explicit reviewed action (recompile at that
// exact SHA in a throwaway worktree, rerun over the same corpus, re-freeze by
// hand, re-diff both halves).
//
// This test asserts every leaf key present in the pre-wave-2 artifact still
// has an identical value in the post-wave-2 artifact, with the single named
// exception of wave2SplitInvariantChangedKey. A pre-existing key that moved
// and is not that one key fails the test by naming itself in the failure
// message, not by a generic "output differs" — that is the entire reason
// this is split out from the whole-document golden comparison above.
func TestWaveTwoSplitInvariantBitIdentityHalf(t *testing.T) {
	t.Parallel()

	preLeaves := map[string]interface{}{}
	flattenJSONLeaves("", loadJSONDocument(t, "testdata/wave2_pre_b892133_cycling_analysis.golden.json"), preLeaves)
	postLeaves := map[string]interface{}{}
	flattenJSONLeaves("", loadJSONDocument(t, "testdata/wave2_live_corpus_cycling_analysis.golden.json"), postLeaves)

	var violations []string
	for path, preValue := range preLeaves {
		postValue, present := postLeaves[path]
		switch {
		case !present:
			violations = append(violations, fmt.Sprintf("pre-existing key %q was removed", path))
		case path == wave2SplitInvariantChangedKey:
			// The one enumerated exception; TestWaveTwoSplitInvariantExpectedDeltaHalf
			// asserts its actual before/after values.
		case !reflect.DeepEqual(preValue, postValue):
			violations = append(violations, fmt.Sprintf("pre-existing key %q changed: pre=%v post=%v", path, preValue, postValue))
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("bit-identity half broken (%d violation(s)):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}

// TestWaveTwoSplitInvariantExpectedDeltaHalf is T2.LIVE/A-2's expected-delta
// half: the only pre-existing key allowed to change is scope.activities
// (303 -> 230, the one field pre-wave-2 code fed directly from
// len(activities) with no cycling-type filter and no stub filter), and the
// only wholly new keys allowed are the ones wave-2 actually introduced
// (validity.*, load.byChannel.*, load.divergence, load.nIncluded,
// intensity.nIncluded, durability.nIncluded). Per A-3, a new key matching
// none of those prefixes is a gap in the wave-2 plan's own enumeration, not
// license to widen this table — such a finding must be reported, not
// silenced by adding it here.
func TestWaveTwoSplitInvariantExpectedDeltaHalf(t *testing.T) {
	t.Parallel()

	preLeaves := map[string]interface{}{}
	flattenJSONLeaves("", loadJSONDocument(t, "testdata/wave2_pre_b892133_cycling_analysis.golden.json"), preLeaves)
	postLeaves := map[string]interface{}{}
	flattenJSONLeaves("", loadJSONDocument(t, "testdata/wave2_live_corpus_cycling_analysis.golden.json"), postLeaves)

	var violations []string

	preActivities, preOK := preLeaves[wave2SplitInvariantChangedKey]
	postActivities, postOK := postLeaves[wave2SplitInvariantChangedKey]
	if !preOK || !postOK || preActivities != 303.0 || postActivities != 230.0 {
		violations = append(violations, fmt.Sprintf(
			"%s: want pre=303 post=230, got pre=%v(present=%v) post=%v(present=%v)",
			wave2SplitInvariantChangedKey, preActivities, preOK, postActivities, postOK,
		))
	}

	for path := range postLeaves {
		if _, existedBefore := preLeaves[path]; existedBefore {
			continue
		}
		if !leafPathHasAllowedNewPrefix(path) {
			violations = append(violations, fmt.Sprintf(
				"new key %q is not in the wave-2 enumerated delta table (planning gap: report it, do not widen the table to pass)", path,
			))
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("expected-delta half broken (%d violation(s)):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}

// TestWaveTwoSplitInvariantExcludedByFlagIsZero is T2.LIVE/A-2's third named
// invariant: F-19's thesis is that icu_ignore_* filtering is measured false
// across the live corpus, so CyclingValidity.ExcludedByFlag must be empty
// when AnalyzeCyclingActivities runs over it. This is asserted directly on
// the live Go struct rather than inferred from the golden JSON's absence of
// an "excludedByFlag" key, because that field is `json:",omitempty"` and a
// nil map is indistinguishable in JSON from a map that was never touched —
// exactly the absence-vs-measured-zero conflation D-5 exists to prevent.
func TestWaveTwoSplitInvariantExcludedByFlagIsZero(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/wave2_live_corpus.json")
	if err != nil {
		t.Fatalf("read live corpus fixture: %v", err)
	}

	var activities []icu.Activity
	if err := json.Unmarshal(raw, &activities); err != nil {
		t.Fatalf("unmarshal live corpus fixture: %v", err)
	}

	got := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{})
	if len(got.Validity.ExcludedByFlag) != 0 {
		t.Fatalf("expected ExcludedByFlag empty (F-19: icu_ignore_* measured false across the live corpus), got %v", got.Validity.ExcludedByFlag)
	}
}
