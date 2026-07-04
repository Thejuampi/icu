package icu

// WithPreferredWellnessScores overlays named scores onto wellness records by date.
// Missing wellness days are added so downstream analysis can still use the score.
func WithPreferredWellnessScores(records []Wellness, scores []DatedWellnessScore) []Wellness {
	merged := append([]Wellness(nil), records...)
	indexByDate := map[string]int{}

	for index := range merged {
		if merged[index].ID == "" {
			continue
		}

		indexByDate[merged[index].ID] = index
	}

	for index := range scores {
		score := scores[index]
		if score.Date == "" || score.Score.Name == "" || score.Score.Value == 0 {
			continue
		}

		if recordIndex, ok := indexByDate[score.Date]; ok {
			merged[recordIndex].PreferredScore = score.Score

			continue
		}

		merged = append(merged, Wellness{ID: score.Date, PreferredScore: score.Score})
		indexByDate[score.Date] = len(merged) - 1
	}

	return merged
}
