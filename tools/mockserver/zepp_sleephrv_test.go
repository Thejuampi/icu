package main

import "testing"

// The mock's readiness sleepHRV must differ from the HRVRMSSD samples mean,
// or tests could not tell which HRV source a sync used.
func TestMockSleepHRVDiffersFromSamplesMean(t *testing.T) {
	t.Parallel()
	const want = 17.0 // documented offset
	for _, hrv := range []float64{38, 44, 52.5} {
		samples, ok := mockHRVRMSSDItem(0, hrv)["value"].(map[string]any)["samples"].([]map[string]any)
		if !ok {
			t.Fatalf("samples shape %T", mockHRVRMSSDItem(0, hrv)["value"])
		}
		sum, n := 0.0, 0
		for _, s := range samples {
			v, _ := s["hrv"].(float64)
			if v <= 0 || v == mockHRVNone {
				continue
			}
			sum += v
			n++
		}
		mean := sum / float64(n)
		sleep, _ := mockReadinessItem(0, hrv)["value"].(map[string]any)["sleepHRV"].(float64)
		if sleep == mean || sleep-mean != want {
			t.Fatalf("hrv=%v: sleepHRV %v, samples mean %v, want mean+%v", hrv, sleep, mean, want)
		}
	}
}
