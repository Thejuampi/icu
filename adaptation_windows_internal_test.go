package icu

import "testing"

func TestAdaptationCurveWindowsAreNonOverlapping(t *testing.T) {
	t.Parallel()

	got, err := AdaptationCurveWindows("2026-08-01", "2026-10-26")
	if err != nil || got != "r.2026-08-01.2026-10-26,r.2026-05-06.2026-07-31" {
		t.Fatalf("windows = %q, %v", got, err)
	}

	for _, bad := range [][2]string{{"x", "2026-01-01"}, {"2026-01-01", "y"}, {"2026-02-01", "2026-01-01"}} {
		if _, err := AdaptationCurveWindows(bad[0], bad[1]); err == nil {
			t.Fatalf("windows(%q,%q) accepted", bad[0], bad[1])
		}
	}
}

func TestAdaptationComparableCurvesPrefersLatestEndDate(t *testing.T) {
	t.Parallel()

	prior := DataCurve{ID: "prior", StartDate: "2026-05-06T00:00:00", EndDate: "2026-07-31T23:59:59", Days: 87}
	current := DataCurve{ID: "current", StartDate: "2026-08-01T00:00:00", EndDate: "2026-10-26T23:59:59", Days: 87}

	pair := adaptationComparableCurves([]DataCurve{prior, current})
	if !pair.ok || pair.current.ID != "current" || pair.baseline.ID != "prior" {
		t.Fatalf("pair = %+v", pair)
	}

	short := DataCurve{ID: "42d", EndDate: "2026-10-26", Days: 42}
	long := DataCurve{ID: "365d", EndDate: "2026-10-26", Days: 365}

	pair = adaptationComparableCurves([]DataCurve{long, short})
	if pair.current.ID != "42d" || pair.baseline.ID != "365d" {
		t.Fatalf("nested pair = %+v", pair)
	}

	if curveEndDay(&DataCurve{EndDate: "short"}) != "short" {
		t.Fatal("curveEndDay short value")
	}
}

func TestAdaptationSystemStatusCountsAnchorsOnly(t *testing.T) {
	t.Parallel()

	deltas := []PowerCurveDelta{{DurationSecs: 60, Status: "improved"}}
	for secs := 61; secs < 70; secs++ {
		deltas = append(deltas, PowerCurveDelta{DurationSecs: secs, Status: "declined"})
	}

	got := adaptationSystemStatus(deltas)
	if got.Improved != 1 || got.Declined != 0 {
		t.Fatalf("status = %+v, want only the 60s anchor counted", got)
	}
}
