package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestClassifyActivityValidityStubShapeIsNotARecord(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{StubShape: true, IgnorePower: true}

	got := icu.ClassifyActivityValidity(&activity)

	if got.Class != icu.ActivityValidityNotARecord {
		t.Fatalf("Class = %q, want not_a_record even with a flag set", got.Class)
	}
}

func TestClassifyActivityValidityTableDriven(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		activity  icu.Activity
		wantClass icu.ActivityValidityClass
		wantScope string
	}{
		"no flags":            {icu.Activity{}, icu.ActivityValidityIncluded, ""},
		"power only":          {icu.Activity{IgnorePower: true}, icu.ActivityValidityExcluded, icu.ValidityScopePower},
		"hr only":             {icu.Activity{IgnoreHR: true}, icu.ActivityValidityExcluded, icu.ValidityScopeHR},
		"time only":           {icu.Activity{IgnoreTime: true}, icu.ActivityValidityExcluded, icu.ValidityScopeAll},
		"power and hr union":  {icu.Activity{IgnorePower: true, IgnoreHR: true}, icu.ActivityValidityExcluded, icu.ValidityScopeAll},
		"all three flags set": {icu.Activity{IgnorePower: true, IgnoreHR: true, IgnoreTime: true}, icu.ActivityValidityExcluded, icu.ValidityScopeAll},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := icu.ClassifyActivityValidity(&testCase.activity)

			if got.Class != testCase.wantClass || got.Scope != testCase.wantScope {
				t.Fatalf("ClassifyActivityValidity(%s) = %+v, want class %q scope %q", name, got, testCase.wantClass, testCase.wantScope)
			}
		})
	}
}

func TestExcludeStubActivitiesDropsOnlyStubs(t *testing.T) {
	t.Parallel()

	activities := []icu.Activity{
		{ID: "real1"},
		{ID: "stub1", StubShape: true},
		{ID: "real2"},
	}

	got := icu.ExcludeStubActivities(activities)

	if len(got) != 2 || got[0].ID != "real1" || got[1].ID != "real2" {
		t.Fatalf("ExcludeStubActivities = %+v, want [real1 real2]", got)
	}
}

func TestClassifyActivityValidityNilActivityIsNotARecord(t *testing.T) {
	t.Parallel()

	got := icu.ClassifyActivityValidity(nil)

	if got.Class != icu.ActivityValidityNotARecord {
		t.Fatalf("Class = %q, want not_a_record for a nil activity", got.Class)
	}
}
