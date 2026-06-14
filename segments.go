package icu

type ActivitySection struct {
	Name       string
	StartIndex int
	EndIndex   int
	Metadata   map[string]any
}

const (
	minSectionLen       = 30
	hrStableWindow      = 10
	varianceThreshold   = 3.0
	cooldownDropPercent = 0.85
	cooldownRecoverPct  = 0.95
)

func DetectSectionsByHRStabilization(hr, _ []float64) (ActivitySection, ActivitySection, ActivitySection) { //nolint:gocritic
	count := len(hr)
	if count < 3 {
		return ActivitySection{Name: "main", StartIndex: 0, EndIndex: count},
			ActivitySection{},
			ActivitySection{}
	}

	var hrStableStart int
	for i := minSectionLen; i < count-hrStableWindow; i++ {
		if Variance(hr[i:i+hrStableWindow]) < varianceThreshold {
			hrStableStart = i
			break
		}
	}

	if hrStableStart < minSectionLen {
		hrStableStart = minSectionLen
	}

	maxHR := MaxValue(hr)

	hrDecayStart := count
	for pos := count - 1; pos >= count-minSectionLen; pos-- {
		if hr[pos] >= maxHR*cooldownDropPercent {
			continue
		}

		back := pos
		for back > 0 {
			if hr[back] >= maxHR*cooldownRecoverPct {
				break
			}
			back--
		}
		hrDecayStart = back + 1
		if hrDecayStart < hrStableStart+minSectionLen {
			hrDecayStart = count
		}
		break
	}

	warmup := ActivitySection{
		Name:       "warmup",
		StartIndex: 0,
		EndIndex:   hrStableStart,
		Metadata:   map[string]any{"method": "hr_stabilization"},
	}

	cooldown := ActivitySection{
		Name:       "cooldown",
		StartIndex: hrDecayStart,
		EndIndex:   count,
		Metadata:   map[string]any{"method": "hr_decay"},
	}

	if hrStableStart < hrDecayStart {
		main := ActivitySection{
			Name:       "main",
			StartIndex: hrStableStart,
			EndIndex:   hrDecayStart,
			Metadata:   map[string]any{"method": "hr_stable"},
		}
		return warmup, cooldown, main
	}

	return warmup, cooldown, ActivitySection{Name: "main", StartIndex: 0, EndIndex: 0}
}

func DetectIntervalsFromDTO(dto *IntervalsDTO) []ActivitySection {
	if dto == nil || len(dto.Intervals) == 0 {
		return nil
	}

	sections := make([]ActivitySection, 0, len(dto.Intervals))
	for i := range dto.Intervals {
		iv := &dto.Intervals[i]
		if iv.StartIndex < 0 || iv.EndIndex <= iv.StartIndex {
			continue
		}
		label := iv.Type
		if label == "" {
			label = "unknown"
		}
		sections = append(sections, ActivitySection{
			Name:       label,
			StartIndex: iv.StartIndex,
			EndIndex:   iv.EndIndex,
			Metadata: map[string]any{
				"type":         label,
				"avgPower":     iv.AvgPower,
				"avgHR":        iv.AvgHR,
				"duration":     iv.MovingTime,
				"trainingLoad": iv.TrainingLoad,
				"intensity":    iv.Intensity,
				"groupID":      iv.GroupID,
			},
		})
	}
	return sections
}
