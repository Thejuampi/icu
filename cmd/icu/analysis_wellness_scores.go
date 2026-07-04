package main

import (
	"context"
	"errors"

	icu "github.com/Thejuampi/icu"
)

const preferredWellnessScoreName = "zepp_hybridcharge"

func readWellnessRecordsForAnalysis(client *icu.Client, oldest, newest string, query map[string]string) ([]icu.Wellness, []string, error) {
	var records []icu.Wellness
	if query == nil {
		query = map[string]string{}
	}
	query["oldest"] = oldest
	query["newest"] = newest
	if err := client.Get("wellness", nil, query, &records); err != nil {
		return nil, nil, wrapCommandError(err)
	}

	scores, warnings := readPreferredWellnessScores(oldest, newest)

	return icu.WithPreferredWellnessScores(records, scores), warnings, nil
}

func readPreferredWellnessScores(oldest, newest string) ([]icu.DatedWellnessScore, []string) {
	var scores []icu.DatedWellnessScore
	var warnings []string
	err := runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
		events, err := client.HybridChargeDays(ctx, oldest, newest)
		if err != nil {
			return err
		}

		for index := range events {
			if events[index].Date == "" || events[index].Score == 0 {
				continue
			}

			scores = append(scores, icu.DatedWellnessScore{
				Date: events[index].Date,
				Score: icu.NamedWellnessScore{
					Name:  preferredWellnessScoreName,
					Value: events[index].Score,
				},
			})
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, icu.ErrZeppNotAuthenticated) {
			warnings = append(warnings, "zepp hybridcharge unavailable; using wellness sleepScore fallback")

			return nil, warnings
		}

		warnings = append(warnings, "zepp hybridcharge fetch failed; using wellness sleepScore fallback")

		return nil, warnings
	}
	if len(scores) == 0 {
		warnings = append(warnings, "no zepp hybridcharge records found; using wellness sleepScore fallback where needed")
	}

	return scores, warnings
}

func analyzeWellness(records []icu.Wellness, warnings []string, options icu.AnalysisOptions) icu.WellnessAnalysis {
	analysis := icu.AnalyzeWellness(records, options)
	analysis.Warnings = append(analysis.Warnings, warnings...)

	return analysis
}
