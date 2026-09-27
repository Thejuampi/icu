package icu

// D-2's scope names: which aggregates an excluded activity drops out of.
const (
	// ValidityScopePower marks icu_ignore_power: excluded from power-derived
	// aggregates, retained in volume and HR zone times.
	ValidityScopePower = "power"
	// ValidityScopeHR marks icu_ignore_hr: the mirror image of
	// ValidityScopePower.
	ValidityScopeHR = "hr"
	// ValidityScopeAll marks icu_ignore_time, or two-or-more flags together
	// (D-2's union rule): excluded from everything including volume, because
	// duration is the denominator every other aggregate divides by.
	ValidityScopeAll = "all"
)

// ActivityValidityClass is ClassifyActivityValidity's total result set
// (D-2, D-24).
type ActivityValidityClass string

const (
	// ActivityValidityIncluded carries no icu_ignore_* flag and is not a
	// STRAVA stub: every aggregate may use it.
	ActivityValidityIncluded ActivityValidityClass = "included"
	// ActivityValidityExcluded carries at least one icu_ignore_* flag;
	// ActivityValidity.Scope names which aggregates it drops from.
	ActivityValidityExcluded ActivityValidityClass = "excluded"
	// ActivityValidityNotARecord is D-24's STRAVA stub: not a session, so it
	// is not merely "excluded" — it never occupies a training slot, a
	// pairing, or a count in the first place.
	ActivityValidityNotARecord ActivityValidityClass = "not_a_record"
)

// ActivityValidity is ClassifyActivityValidity's result.
type ActivityValidity struct {
	Class ActivityValidityClass
	// Scope is one of ValidityScopePower/HR/All, set only when Class is
	// ActivityValidityExcluded.
	Scope string
}

// ClassifyActivityValidity is D-2's single per-flag ignore rule, plus D-24's
// stub class (T2.2): the single source of both judgements, so no consumer
// re-derives them independently. A STRAVA stub short-circuits before any
// icu_ignore_* flag is consulted — a stub is not a session, so nothing
// about it can be "ignored" in the ordinary sense.
func ClassifyActivityValidity(activity *Activity) ActivityValidity {
	if activity == nil || activity.StubShape {
		return ActivityValidity{Class: ActivityValidityNotARecord}
	}

	scope := activityIgnoreScope(activity)
	if scope == "" {
		return ActivityValidity{Class: ActivityValidityIncluded}
	}

	return ActivityValidity{Class: ActivityValidityExcluded, Scope: scope}
}

// activityIgnoreScope implements D-2's per-flag semantics: icu_ignore_time
// excludes from everything including volume; icu_ignore_power/icu_ignore_hr
// each exclude only their own derived aggregates; two or more flags
// together union to "all", the same outcome as icu_ignore_time alone.
func activityIgnoreScope(activity *Activity) string {
	flagCount := boolCount(activity.IgnoreTime, activity.IgnorePower, activity.IgnoreHR)

	switch {
	case activity.IgnoreTime || flagCount >= 2:
		return ValidityScopeAll
	case activity.IgnorePower:
		return ValidityScopePower
	case activity.IgnoreHR:
		return ValidityScopeHR
	default:
		return ""
	}
}

// ExcludeStubActivities is the intended single shared choke point (T2.3/
// T2.4) for every consumer of a caller-supplied activity slice: a D-24
// STRAVA stub is not a session, so it should never occupy a count, a load
// aggregate, a "day already has a completed session" check, or a pairing
// anywhere in the codebase. Centralizing the predicate here means fixing a
// stub-contamination defect once instead of re-deriving it at every call
// site.
//
// Stage 6 R14 correction: the previous version of this comment claimed that
// property as already true "anywhere in the codebase." It was not — nothing
// enforces that a consumer actually calls this function, and
// adaptation_analysis.go/planning_analysis.go silently summed raw activity
// slices for months without going through it (the same bug, one level out,
// as Stage 5's F2). This function only implements the predicate; it cannot
// compel a caller to use it. Known callers today: rebalance.go, plan.go,
// microcycle_analysis.go, adaptation_analysis.go, planning_analysis.go. The
// part of this guarantee that is actually enforced is narrower and lives in
// TestScopeActivitiesInvariantAcrossCommands
// (scope_activities_invariant_test.go): every command in that test's
// literal, human-maintained table must report the same scope.activities
// count for the same window, so a future aggregator that bypasses this
// function and exposes scope.activities fails a test instead of shipping a
// silently wrong number. That test does not, and cannot, catch a bypass in
// a command outside its enumerated table (e.g. a future load-only
// aggregator with no scope.activities field) — this comment does not
// overclaim past what is actually checked.
func ExcludeStubActivities(activities []Activity) []Activity {
	filtered := make([]Activity, 0, len(activities))

	for index := range activities {
		if activities[index].StubShape {
			continue
		}

		filtered = append(filtered, activities[index])
	}

	return filtered
}

// StubActivitiesOnly is ExcludeStubActivities' complement (T2.8/D-23): a
// caller that must keep a stub's time visible for accounting purposes (never
// priced, never silently dropped) after the exclusion filter runs needs the
// discarded set, not just the survivors.
func StubActivitiesOnly(activities []Activity) []Activity {
	var stubs []Activity

	for index := range activities {
		if activities[index].StubShape {
			stubs = append(stubs, activities[index])
		}
	}

	return stubs
}

func boolCount(flags ...bool) int {
	count := 0

	for _, flag := range flags {
		if flag {
			count++
		}
	}

	return count
}
