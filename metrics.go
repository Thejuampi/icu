package icu

import "math"

const (
	npWindow   = 30
	npExponent = 4
	npRoot     = 0.25
	decoupMinN = 4
	bpmPerMin  = 60
)

func AveragePower(watts []float64, start, end int) float64 {
	if start >= end || start < 0 || end > len(watts) || len(watts) == 0 {
		return 0
	}

	var sum float64
	for i := start; i < end; i++ {
		sum += watts[i]
	}
	return sum / float64(end-start)
}

func AverageHeartRate(hr []float64, start, end int) float64 {
	return AveragePower(hr, start, end)
}

func NormalizedPower(watts []float64, start, end int) float64 {
	length := end - start
	if length <= 0 || start < 0 || end > len(watts) {
		return 0
	}

	if length < npWindow {
		return AveragePower(watts, start, end)
	}

	// Rolling 30s average is O(n); the prior per-window rescan was O(n*window).
	var windowSum float64
	for i := start; i < start+npWindow; i++ {
		windowSum += watts[i]
	}

	var sum float64
	count := 0
	for i := start; i+npWindow <= end; i++ {
		if i > start {
			windowSum += watts[i+npWindow-1] - watts[i-1]
		}
		avg := windowSum / float64(npWindow)
		sum += math.Pow(avg, npExponent)
		count++
	}

	if count <= 0 {
		return 0
	}

	return math.Pow(sum/float64(count), npRoot)
}

func MaxValue(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	maxVal := -math.MaxFloat64
	found := false
	for i := range data {
		v := data[i]
		if math.IsNaN(v) {
			continue
		}
		if v > maxVal {
			maxVal = v
			found = true
		}
	}

	if !found {
		return 0
	}
	return maxVal
}

func HeartRateDrift(hr []float64, totalSeconds int) float64 {
	if len(hr) < 2 || totalSeconds <= 0 {
		return 0
	}

	secsPerSample := float64(totalSeconds) / float64(len(hr)-1)

	return Slope(hr) / secsPerSample * bpmPerMin
}

func PowerHRRatio(watts, hr []float64, start, end int) float64 {
	avgWatts := AveragePower(watts, start, end)
	if avgWatts == 0 {
		return 0
	}

	avgHR := AverageHeartRate(hr, start, end)
	if avgHR == 0 {
		return 0
	}

	return avgWatts / avgHR
}

func Decoupling(watts, hr []float64) float64 {
	count := len(watts)
	if count < decoupMinN || len(hr) < decoupMinN {
		return 0
	}

	if len(hr) < count {
		count = len(hr)
		watts = watts[:count]
	}

	mid := count / 2

	firstRatio := PowerHRRatio(watts, hr, 0, mid)
	if firstRatio == 0 {
		return 0
	}

	secondRatio := PowerHRRatio(watts, hr, mid, count)
	if secondRatio == 0 {
		return 0
	}

	// Friel aerobic decoupling: percent change relative to the first half EF.
	// ((EF1 - EF2) / EF1) * 100  ==  (1 - EF2/EF1) * 100
	return (1 - secondRatio/firstRatio) * percentScale
}

func TimeInZone(watts []float64, ftp int, minPct, maxPct float64) int {
	if ftp <= 0 {
		return 0
	}

	minWatts := float64(ftp) * minPct / percentScale
	maxWatts := float64(ftp) * maxPct / percentScale

	var count int
	for i := range watts {
		if watts[i] >= minWatts && watts[i] < maxWatts {
			count++
		}
	}

	return count
}

func HeartRateRecovery(hr []float64, peakIndex, windowSize int) float64 {
	if peakIndex < 0 || peakIndex >= len(hr) || windowSize <= 0 {
		return 0
	}

	if peakIndex+windowSize >= len(hr) {
		return 0
	}

	peakHR := hr[peakIndex]
	endHR := hr[peakIndex+windowSize]

	// Conventional HRR is total bpm recovered over the window (e.g. HRR60),
	// not bpm per sample. Decay rate lives on CooldownAnalysis.HRDecayRate.
	return peakHR - endHR
}

func Slope(data []float64) float64 {
	count := len(data)
	if count < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i := range count {
		x := float64(i)
		y := data[i]
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := float64(count)*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}

	return (float64(count)*sumXY - sumX*sumY) / denom
}

func Variance(data []float64) float64 {
	count := len(data)
	if count <= 1 {
		return 0
	}

	var sum float64
	for i := range data {
		sum += data[i]
	}

	meanVal := sum / float64(count)

	var sqDiff float64
	for i := range data {
		diff := data[i] - meanVal
		sqDiff += diff * diff
	}

	return sqDiff / float64(count-1)
}

func HeartRateToLTHR(hr float64, lthr int) float64 {
	if lthr <= 0 {
		return 0
	}

	return hr / float64(lthr) * percentScale
}
