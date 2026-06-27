package icu

import "math"

const rebalanceRegimeMinSamples = 6

const rebalanceRegimeMinRecent = 3

type RebalanceRegime struct {
	Samples           []float64
	StartIndex        int
	EndIndex          int
	ChangePoints      []int
	RecentDisturbance bool
	Median            float64
	MAD               float64
	Coverage          float64
	Confidence        float64
	Source            string
}

// DetectRebalanceRegime segments a time-ordered weekly load series using an
// L1/MDL binary change-point detector and returns the most recent (vigent)
// regime. Each change point costs log2(N) bits (the information-theoretic cost
// of encoding its position); a split is kept only when the L1 cost reduction
// exceeds that penalty. A trailing regime shorter than the protocol minimum
// (a recent illness/event outlier) is discarded in favour of the previous
// regime, lowering confidence proportionally to the discarded share. ok is
// false when the series is too short for a statistically meaningful regime (a
// documented protocol minimum, not a physiological decision threshold).
func DetectRebalanceRegime(samples []float64) (RebalanceRegime, bool) {
	if len(samples) < rebalanceRegimeMinSamples {
		return RebalanceRegime{}, false
	}
	samplesLen := len(samples)
	changePoints := segmentRegimeL1MDL(samples, 0, samplesLen-1)

	start := 0
	var end int
	disturbance := false
	discardedShare := 0.0
	currentEnd := samplesLen - 1
	for {
		if len(changePoints) == 0 {
			break
		}
		lastSplit := changePoints[len(changePoints)-1]
		segLen := currentEnd - lastSplit + 1
		if segLen >= rebalanceRegimeMinRecent {
			start = lastSplit
			break
		}
		disturbance = true
		discardedShare += float64(segLen) / float64(samplesLen)
		changePoints = changePoints[:len(changePoints)-1]
		currentEnd = lastSplit - 1
	}
	end = currentEnd
	regimeSamples := samples[start : end+1]
	median := medianFloat64(regimeSamples)
	mad := madFloat64(regimeSamples, median)
	coverage := float64(len(regimeSamples)) / float64(samplesLen)
	stability := regimeStability(median, mad)
	confidence := clampFloat01(coverage * stability)
	if disturbance {
		confidence *= (1 - discardedShare)
	}

	return RebalanceRegime{
		Samples:           regimeSamples,
		StartIndex:        start,
		EndIndex:          end,
		ChangePoints:      changePoints,
		RecentDisturbance: disturbance,
		Median:            round3(median),
		MAD:               round3(mad),
		Coverage:          round3(coverage),
		Confidence:        round3(clampFloat01(confidence)),
		Source:            "l1_mdl_binary_segmentation_current_regime",
	}, true
}

func segmentRegimeL1MDL(samples []float64, start, end int) []int {
	length := end - start + 1
	if length < rebalanceRegimeMinSamples {
		return nil
	}
	wholeCost := l1SegmentCost(samples, start, end)
	penalty := math.Log2(float64(length))
	bestGain := 0.0
	bestSplit := -1
	for k := start + 1; k <= end; k++ {
		leftCost := l1SegmentCost(samples, start, k-1)
		rightCost := l1SegmentCost(samples, k, end)
		gain := wholeCost - (leftCost + rightCost)
		if gain > bestGain {
			bestGain = gain
			bestSplit = k
		}
	}
	if bestSplit < 0 || bestGain <= penalty {
		return nil
	}
	left := segmentRegimeL1MDL(samples, start, bestSplit-1)
	right := segmentRegimeL1MDL(samples, bestSplit, end)
	var combined []int
	combined = append(combined, left...)
	combined = append(combined, bestSplit)
	combined = append(combined, right...)
	return combined
}

func l1SegmentCost(samples []float64, start, end int) float64 {
	segment := samples[start : end+1]
	median := medianFloat64(segment)
	var cost float64
	for index := range segment {
		cost += math.Abs(segment[index] - median)
	}
	return cost
}

func regimeStability(median, mad float64) float64 {
	if median <= 0 {
		if mad <= 0 {
			return 1
		}
		return clampFloat01(1 - mad)
	}
	normalized := mad / median
	return clampFloat01(1 - normalized)
}

func clampFloat01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
