package icu

import (
	"math"
	"strings"
	"testing"
)

func TestComputeWBalDepletionReturnsZeroForInsufficientInputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		watts  []float64
		cp     int
		wprime int
	}{
		{name: "empty watts", watts: nil, cp: 287, wprime: 21000},
		{name: "zero cp", watts: []float64{300, 310}, cp: 0, wprime: 21000},
		{name: "zero wprime", watts: []float64{300, 310}, cp: 287, wprime: 0},
		{name: "negative cp", watts: []float64{300}, cp: -1, wprime: 21000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			depl, pct := ComputeWBalDepletion(tc.watts, tc.cp, tc.wprime)
			if depl != 0 || pct != 0 {
				t.Fatalf("ComputeWBalDepletion(%v, cp=%d, w'=%d) = (%d, %f), want (0, 0)",
					tc.watts, tc.cp, tc.wprime, depl, pct)
			}
		})
	}
}

func TestComputeWBalDepletionZ2SessionStaysNearZero(t *testing.T) {
	t.Parallel()

	watts := make([]float64, 7200)
	for i := range watts {
		watts[i] = 195
	}

	depl, pct := ComputeWBalDepletion(watts, 287, 21000)
	if pct > 1 {
		t.Fatalf("Z2-only session depletion pct = %f, want <= 1 (CP=%d, avg=%f)", pct, 287, 195.0)
	}
	if depl > 300 {
		t.Fatalf("Z2-only session depletion joules = %d, want <= 300", depl)
	}
}

func TestComputeWBalDepletionVO2SessionDrainsWPrime(t *testing.T) {
	t.Parallel()

	watts := make([]float64, 720)
	for i := range watts {
		watts[i] = 330
	}

	depl, pct := ComputeWBalDepletion(watts, 287, 21000)
	if pct < 50 {
		t.Fatalf("VO2 session depletion pct = %f, want >= 50", pct)
	}
	if depl < 10000 {
		t.Fatalf("VO2 session depletion joules = %d, want >= 10000", depl)
	}
}

func TestComputeWBalDepletionRecoveryBetweenEfforts(t *testing.T) {
	t.Parallel()

	watts := make([]float64, 0, 600)
	watts = append(watts, repeatFloat(330, 60)...)
	watts = append(watts, repeatFloat(120, 300)...)
	watts = append(watts, repeatFloat(330, 60)...)
	watts = append(watts, repeatFloat(120, 180)...)

	_, pct := ComputeWBalDepletion(watts, 287, 21000)
	if pct <= 0 {
		t.Fatal("mixed session depletion pct = 0, want > 0")
	}
	if pct > 80 {
		t.Fatalf("mixed session depletion pct = %f, want <= 80 (recovery should limit drain)", pct)
	}
}

func TestComputeWBalDepletionPeakOccursDuringEffort(t *testing.T) {
	t.Parallel()

	cp := 287
	wprime := 21000

	watts := make([]float64, 0, 360)
	watts = append(watts, repeatFloat(400, 60)...)
	watts = append(watts, repeatFloat(100, 300)...)

	depl, _ := ComputeWBalDepletion(watts, cp, wprime)

	consumed := (400.0 - float64(cp)) * 60
	expectedDepl := int(math.Round(consumed))

	if math.Abs(float64(depl-expectedDepl)) > float64(expectedDepl)*0.05 {
		t.Fatalf("peak depletion = %d, expected ~%d (5%% tolerance)", depl, expectedDepl)
	}
}

func TestComputeWBalDepletionRecoveryReducesSecondEffortDepletion(t *testing.T) {
	t.Parallel()

	cp := 287
	wprime := 21000

	wattsWithRecovery := make([]float64, 0, 720)
	wattsWithRecovery = append(wattsWithRecovery, repeatFloat(400, 60)...)
	wattsWithRecovery = append(wattsWithRecovery, repeatFloat(100, 300)...)
	wattsWithRecovery = append(wattsWithRecovery, repeatFloat(400, 60)...)

	wattsNoRecovery := make([]float64, 0, 120)
	wattsNoRecovery = append(wattsNoRecovery, repeatFloat(400, 60)...)
	wattsNoRecovery = append(wattsNoRecovery, repeatFloat(400, 60)...)

	deplWithRecovery, _ := ComputeWBalDepletion(wattsWithRecovery, cp, wprime)
	deplNoRecovery, _ := ComputeWBalDepletion(wattsNoRecovery, cp, wprime)

	if deplWithRecovery >= deplNoRecovery {
		t.Fatalf("with recovery depletion (%d) should be < without recovery (%d)",
			deplWithRecovery, deplNoRecovery)
	}
}

func TestActivityModelReliableDetectsLowCP(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		activity *Activity
		want     bool
	}{
		{name: "nil activity", activity: nil, want: false},
		{name: "CP below 85% of FTP", activity: &Activity{CriticalPower: 172, FTP: 285}, want: false},
		{name: "CP at 100% of FTP", activity: &Activity{CriticalPower: 285, FTP: 285}, want: true},
		{name: "CP above FTP", activity: &Activity{CriticalPower: 295, FTP: 285}, want: true},
		{name: "no CP data", activity: &Activity{FTP: 285}, want: true},
		{name: "no FTP data with CP", activity: &Activity{CriticalPower: 287}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ActivityModelReliable(tc.activity)
			if got != tc.want {
				t.Fatalf("ActivityModelReliable() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDetectFlywheelArtifactsFindsCoastDown(t *testing.T) {
	t.Parallel()

	cp := 287
	watts := buildCoastDownWatts()
	cadence := buildCoastDownCadence()
	hr := repeatFloat(135, 200)

	result := DetectFlywheelArtifacts(watts, cadence, hr, cp)

	if len(result.Artifacts) == 0 {
		t.Fatal("expected at least 1 flywheel artifact, got 0")
	}

	art := result.Artifacts[0]
	if art.DurationSec < 3 {
		t.Fatalf("artifact duration = %d, want >= 3", art.DurationSec)
	}
	if art.StartCadence < 70 || art.EndCadence > 30 {
		t.Fatalf("cadence: start=%.0f end=%.0f, expected start>70 end<30", art.StartCadence, art.EndCadence)
	}
}

func TestDetectFlywheelArtifactsIgnoresRealSprint(t *testing.T) {
	t.Parallel()

	cp := 287
	watts := make([]float64, 100)
	for i := range watts {
		watts[i] = 195
	}
	for i := 60; i < 80; i++ {
		watts[i] = 600
	}
	cadence := repeatFloat(85, 100)
	hr := make([]float64, 100)
	for i := range hr {
		hr[i] = 135
	}
	for i := 60; i < 80; i++ {
		hr[i] = 135 + float64(i-60)*2
	}

	result := DetectFlywheelArtifacts(watts, cadence, hr, cp)

	if len(result.Artifacts) > 0 {
		t.Fatalf("expected 0 artifacts for real sprint (cadence stable, HR rises), got %d", len(result.Artifacts))
	}
}

func TestDetectFlywheelArtifactsIgnoresShortSpikes(t *testing.T) {
	t.Parallel()

	cp := 287
	watts := make([]float64, 100)
	for i := range watts {
		watts[i] = 195
	}
	watts[50] = 500
	watts[51] = 480
	cadence := repeatFloat(80, 100)
	cadence[51] = 20

	result := DetectFlywheelArtifacts(watts, cadence, nil, cp)

	if len(result.Artifacts) > 0 {
		t.Fatalf("expected 0 artifacts for 2s spike, got %d", len(result.Artifacts))
	}
}

func TestDetectFlywheelArtifactsWithoutCadenceReturnsEmpty(t *testing.T) {
	t.Parallel()

	watts := repeatFloat(600, 20)
	result := DetectFlywheelArtifacts(watts, nil, nil, 287)

	if len(result.Artifacts) != 0 {
		t.Fatalf("expected 0 artifacts without cadence, got %d", len(result.Artifacts))
	}
}

func TestCleanPowerStreamZerosArtifacts(t *testing.T) {
	t.Parallel()

	watts := []float64{100, 200, 600, 590, 580, 200, 100}
	artifacts := []FlywheelArtifact{
		{StartIndex: 2, EndIndex: 4, DurationSec: 3},
	}

	cleaned := CleanPowerStream(watts, artifacts)

	for i, v := range cleaned {
		if i >= 2 && i <= 4 {
			if v != 0 {
				t.Fatalf("cleaned[%d] = %f, want 0", i, v)
			}
		} else if v != watts[i] {
			t.Fatalf("cleaned[%d] = %f, want %f (untouched)", i, v, watts[i])
		}
	}
}

func TestCleanPowerStreamReturnsCopy(t *testing.T) {
	t.Parallel()

	watts := []float64{100, 200, 300}
	cleaned := CleanPowerStream(watts, nil)

	cleaned[0] = 999
	if watts[0] == 999 {
		t.Fatal("CleanPowerStream did not return a copy")
	}
}

func TestRecomputeWBalDepletionUsesGlobalModel(t *testing.T) {
	t.Parallel()

	watts := repeatFloat(195, 7200)
	cadence := repeatFloat(85, 7200)
	hr := repeatFloat(135, 7200)

	globalModel := PowerModel{CriticalPower: 287, WPrime: 21000}
	activity := &Activity{CriticalPower: 172, FTP: 285, WPrime: 14577}

	depl, pct, _, warnings := RecomputeWBalDepletion(watts, cadence, hr, globalModel, activity)

	if pct > 1 {
		t.Fatalf("Z2 session with global model: depletion pct = %f, want <= 1", pct)
	}
	if depl > 300 {
		t.Fatalf("Z2 session with global model: depletion joules = %d, want <= 300", depl)
	}

	if !warningsContain(warnings, "unreliable") {
		t.Fatalf("expected unreliable model warning, got %v", warnings)
	}
}

func TestRecomputeWBalDepletionWithFlywheelArtifact(t *testing.T) {
	t.Parallel()

	watts := buildCoastDownWatts()
	cadence := buildCoastDownCadence()
	hr := repeatFloat(135, 200)

	globalModel := PowerModel{CriticalPower: 287, WPrime: 21000}

	depl, _, detection, warnings := RecomputeWBalDepletion(watts, cadence, hr, globalModel, nil)

	if len(detection.Artifacts) == 0 {
		t.Fatal("expected flywheel artifact detection, got 0")
	}
	if !warningsContain(warnings, "flywheel") {
		t.Fatalf("expected flywheel warning, got %v", warnings)
	}

	rawDepl, _ := ComputeWBalDepletion(watts, globalModel.CriticalPower, globalModel.WPrime)
	if depl >= rawDepl {
		t.Fatalf("cleaned depletion (%d) should be < raw depletion (%d)", depl, rawDepl)
	}
}

func TestRecomputeWBalDepletionNoStreamsReturnsZeros(t *testing.T) {
	t.Parallel()

	depl, pct, _, warnings := RecomputeWBalDepletion(nil, nil, nil, PowerModel{}, nil)

	if depl != 0 || pct != 0 {
		t.Fatalf("expected (0, 0), got (%d, %f)", depl, pct)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %v", warnings)
	}
}

func repeatFloat(value float64, count int) []float64 {
	result := make([]float64, count)
	for i := range result {
		result[i] = value
	}
	return result
}

func warningsContain(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func buildCoastDownWatts() []float64 {
	watts := make([]float64, 200)
	for i := range watts {
		watts[i] = 195
	}
	for i := 100; i < 120; i++ {
		watts[i] = 600 - float64(i-100)*8
	}
	return watts
}

func buildCoastDownCadence() []float64 {
	cadence := make([]float64, 200)
	for i := range cadence {
		cadence[i] = 85
	}
	for i := 100; i < 120; i++ {
		cadence[i] = float64(85 - (i-100)*4)
	}
	return cadence
}
