package icu_test

import (
	"encoding/json"
	"os"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// scopeActivitiesInvariantCommands is Stage 6 R14's structural enforcement
// for the "every consumer of a caller-supplied activity slice passes
// through ExcludeStubActivities" convention documented in
// activity_validity.go. It is a literal, human-maintained table — the same
// discipline as T2.LIVE's wave2SplitInvariantNewKeyPrefixes in
// analysis_live_corpus_test.go — not a reflective sweep: a future analysis
// command that starts reporting scope.activities is invisible to this test
// until a person adds it here. What the table does guarantee is that every
// command it lists reports the same activity count for the same input
// slice, so a future aggregator that bypasses ExcludeStubActivities and
// exposes scope.activities fails TestScopeActivitiesInvariantAcrossCommands
// instead of shipping a silently inflated number the way
// analysis adaptation did (Stage 6 R14: 25 vs. analysis cycling's 7 on the
// same 2025-11 window before this fix).
//
// Enumerated commands and why each qualifies: both compute scope.activities
// as "every non-stub activity in the fetched window," independent of any
// sport-type filter (AnalyzeCyclingActivities.Scope.Activities is
// deliberately the whole-fetch count, not the cycling-only one — see its
// own doc comment). A command whose activity count has different semantics
// (e.g. a type-filtered or category-filtered count) does not belong here;
// forcing it into this table would make the invariant assert something
// false, not something safer.
var scopeActivitiesInvariantCommands = []struct { //nolint:gochecknoglobals // fixed enumerated table (T2.LIVE-style), see doc comment above.
	name  string
	scope func(activities []icu.Activity) int
}{
	{
		name: "analysis cycling",
		scope: func(activities []icu.Activity) int {
			return icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{}).Scope.Activities
		},
	},
	{
		name: "analysis adaptation",
		scope: func(activities []icu.Activity) int {
			return icu.AnalyzeCyclingAdaptation(nil, icu.PowerModel{}, nil, activities, nil, icu.AnalysisOptions{}).Scope.Activities
		},
	},
}

// TestScopeActivitiesInvariantAcrossCommands is Stage 6 R14's demonstrable
// guard: run every enumerated command in scopeActivitiesInvariantCommands
// against the same activity slice (the frozen live corpus — 303 raw
// records, 73 of them D-24 STRAVA stubs, same fixture T2.LIVE uses) and
// require they all report the identical scope.activities count. This is
// the mechanism, not a restatement of the fix: it fails if any enumerated
// command's own filtering regresses to counting stubs, without needing to
// know which internal helper that command happens to call today.
func TestScopeActivitiesInvariantAcrossCommands(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/wave2_live_corpus.json")
	if err != nil {
		t.Fatalf("read live corpus fixture: %v", err)
	}

	var activities []icu.Activity
	if err := json.Unmarshal(raw, &activities); err != nil {
		t.Fatalf("unmarshal live corpus fixture: %v", err)
	}

	want := scopeActivitiesInvariantCommands[0].scope(activities)
	for _, command := range scopeActivitiesInvariantCommands[1:] {
		got := command.scope(activities)
		if got != want {
			t.Fatalf(
				"scope.activities disagree on the same %d-activity window: %s=%d, %s=%d",
				len(activities), scopeActivitiesInvariantCommands[0].name, want, command.name, got,
			)
		}
	}
}
