package icu_test

import (
	"math"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestEstimatePlannedLoadSteadyPower(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 60m 70%")
	if err != nil {
		t.Fatalf("ParseWorkoutDescription error = %v", err)
	}

	got := icu.EstimatePlannedLoad(doc, 300)

	if got.DurationSeconds != 3600 || math.Abs(got.IntensityFactor-0.70) > 0.001 || got.TrainingLoad != 49 {
		t.Fatalf("EstimatePlannedLoad steady = %+v, want 3600s IF 0.70 TSS 49", got)
	}
}

func TestEstimatePlannedLoadVariablePowerRaisesNormalizedPower(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("2x\n  - 5m 120%\n  - 5m 50%")
	if err != nil {
		t.Fatalf("ParseWorkoutDescription error = %v", err)
	}

	got := icu.EstimatePlannedLoad(doc, 300)

	if got.NormalizedPower <= got.AveragePower {
		t.Fatalf("EstimatePlannedLoad NP = %.2f average = %.2f, want NP > average", got.NormalizedPower, got.AveragePower)
	}
}

func TestEstimatePlannedLoadUsesRampInterpolation(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 10m 50-100%")
	if err != nil {
		t.Fatalf("ParseWorkoutDescription error = %v", err)
	}

	got := icu.EstimatePlannedLoad(doc, 300)

	if math.Abs(got.AveragePower-225) > 1 {
		t.Fatalf("EstimatePlannedLoad ramp average = %.2f, want about 225", got.AveragePower)
	}
}

func TestEstimatePlannedLoadMissingFTPWarns(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 60m 70%")
	if err != nil {
		t.Fatalf("ParseWorkoutDescription error = %v", err)
	}

	got := icu.EstimatePlannedLoad(doc, 0)

	if len(got.Warnings) != 1 || got.TrainingLoad != 0 {
		t.Fatalf("EstimatePlannedLoad missing FTP = %+v, want one warning and zero load", got)
	}
}

func TestEstimatePlannedLoadNilDocWarns(t *testing.T) {
	t.Parallel()

	got := icu.EstimatePlannedLoad(nil, 300)

	if len(got.Warnings) != 1 {
		t.Fatalf("EstimatePlannedLoad nil doc = %+v, want one warning", got)
	}
}

func TestEstimatePlannedLoadNoPowerStepsWarns(t *testing.T) {
	t.Parallel()

	got := icu.EstimatePlannedLoad(&icu.WorkoutDoc{Duration: 60, Steps: []icu.WorkoutStep{{Duration: 60}}}, 300)

	if len(got.Warnings) != 1 || got.DurationSeconds != 0 {
		t.Fatalf("EstimatePlannedLoad no power = %+v, want warning and zero duration", got)
	}
}

func TestEstimatePlannedLoadSupportsAbsoluteWatts(t *testing.T) {
	t.Parallel()

	doc := &icu.WorkoutDoc{Duration: 60, Steps: []icu.WorkoutStep{{Duration: 60, Power: &icu.WorkoutTarget{Value: 210, Units: "w"}}}}

	got := icu.EstimatePlannedLoad(doc, 300)

	if math.Abs(got.IntensityFactor-0.7) > 0.001 {
		t.Fatalf("EstimatePlannedLoad absolute watts IF = %.3f, want 0.700", got.IntensityFactor)
	}
}
