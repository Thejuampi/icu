package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestWithPreferredWellnessScoresAddsMissingDay(t *testing.T) {
	t.Parallel()

	records := []icu.Wellness{{ID: "2026-06-01", SleepScore: 80}}
	scores := []icu.DatedWellnessScore{{
		Date: "2026-06-02",
		Score: icu.NamedWellnessScore{
			Name:  "zepp_hybridcharge",
			Value: 91,
		},
	}}

	got := icu.WithPreferredWellnessScores(records, scores)

	if len(got) != 2 || got[1].ID != "2026-06-02" || got[1].PreferredScore.Name != "zepp_hybridcharge" || got[1].PreferredScore.Value != 91 {
		t.Fatalf("records = %+v, want added preferred score day", got)
	}
}
