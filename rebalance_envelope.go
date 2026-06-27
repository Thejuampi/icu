package icu

import (
	"math"
	"strconv"
)

const rebalanceEnvelopeFenceMAD = 3.0

type RebalanceEnvelopeReport struct {
	Envelope        RebalanceEnvelope `json:"envelope"`
	CurrentLoad     float64           `json:"currentLoad"`
	OutsideEnvelope bool              `json:"outsideEnvelope"`
	Confidence      float64           `json:"confidence"`
	Completeness    float64           `json:"completeness"`
	LowSource       string            `json:"lowSource,omitempty"`
	HighSource      string            `json:"highSource,omitempty"`
	Sources         []string          `json:"sources,omitempty"`
}

// BuildRebalanceEnvelope derives the data-backed low/current/high weekly-load
// envelope from the vigent regime and the current calendar load. low/high are
// robust MAD fences (median +/- k*MAD with k=3), the canonical robust-statistics
// protocol constant; a physiological ceiling caps the high when provided.
// Missing optional metrics reduce confidence but never activate defaults, and
// ok is false only when there is no regime to back an envelope.
func BuildRebalanceEnvelope(regime *RebalanceRegime, currentLoad, physiologicalCeiling float64, metricsAvailable, metricsTotal int) (RebalanceEnvelopeReport, bool) {
	if len(regime.Samples) == 0 {
		return RebalanceEnvelopeReport{}, false
	}
	median := medianFloat64(regime.Samples)
	mad := madFloat64(regime.Samples, median)

	lowScalar := math.Max(0, median-rebalanceEnvelopeFenceMAD*mad)
	highScalar := median + rebalanceEnvelopeFenceMAD*mad
	highSource := "data_robust_fence"
	if physiologicalCeiling > 0 && physiologicalCeiling < highScalar {
		highScalar = physiologicalCeiling
		highSource = "data_robust_fence_capped_by_physiological_ceiling"
	}

	low, lowOK := NewRebalanceRatFromDyadic(lowScalar)
	current, currentOK := NewRebalanceRatFromDyadic(currentLoad)
	high, highOK := NewRebalanceRatFromDyadic(highScalar)
	if !lowOK || !currentOK || !highOK {
		return RebalanceEnvelopeReport{}, false
	}
	if low.Cmp(current) > 0 {
		low = current.clone()
	}
	if high.Cmp(current) < 0 {
		high = current.clone()
	}

	outside := currentLoad < lowScalar || currentLoad > highScalar
	completeness := 1.0
	if metricsTotal > 0 {
		completeness = clampFloat01(float64(metricsAvailable) / float64(metricsTotal))
	}
	confidence := clampFloat01(regime.Confidence * completeness)

	return RebalanceEnvelopeReport{
		Envelope: RebalanceEnvelope{
			Low:     low,
			Current: current,
			High:    high,
		},
		CurrentLoad:     currentLoad,
		OutsideEnvelope: outside,
		Confidence:      round3(confidence),
		Completeness:    round3(completeness),
		LowSource:       "data_robust_fence",
		HighSource:      highSource,
		Sources:         envelopeSources(metricsAvailable, metricsTotal),
	}, true
}

func envelopeSources(available, total int) []string {
	if total <= 0 {
		return nil
	}
	return []string{pluralizeMetricCount(available, total)}
}

func pluralizeMetricCount(available, total int) string {
	return "metrics_available:" + strconv.Itoa(available) + "/" + strconv.Itoa(total)
}
