package icu

import "strings"

const plannedLoadSourceLocalPower = "local_power_workout_description"

type PlannedLoadEstimate struct {
	DurationSeconds int      `json:"durationSeconds"`
	AveragePower    float64  `json:"averagePower,omitempty"`
	NormalizedPower float64  `json:"normalizedPower,omitempty"`
	IntensityFactor float64  `json:"intensityFactor,omitempty"`
	TrainingLoad    int      `json:"trainingLoad,omitempty"`
	Source          string   `json:"source,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

func EstimatePlannedLoad(doc *WorkoutDoc, ftp int) PlannedLoadEstimate {
	var estimate PlannedLoadEstimate
	estimate.Source = plannedLoadSourceLocalPower
	if doc == nil {
		estimate.Warnings = append(estimate.Warnings, "workout doc is missing")
		return estimate
	}
	if ftp <= 0 {
		estimate.DurationSeconds = doc.Duration
		estimate.Warnings = append(estimate.Warnings, "ftp is missing")
		return estimate
	}

	steps := ExpandWorkoutSteps(doc)
	watts := plannedWorkoutWatts(steps, ftp)
	estimate.DurationSeconds = len(watts)
	if estimate.DurationSeconds == 0 {
		estimate.Warnings = append(estimate.Warnings, "workout has no power steps")
		return estimate
	}

	estimate.AveragePower = round2(AveragePower(watts, 0, len(watts)))
	estimate.NormalizedPower = round2(NormalizedPower(watts, 0, len(watts)))
	estimate.IntensityFactor = round3(estimate.NormalizedPower / float64(ftp))
	estimate.TrainingLoad = int(round2(float64(estimate.DurationSeconds) / secondsPerHour * estimate.IntensityFactor * estimate.IntensityFactor * percentScale))

	return estimate
}

func plannedWorkoutWatts(steps []PlannedWorkoutStep, ftp int) []float64 {
	var watts []float64
	for index := range steps {
		step := steps[index]
		if step.DurationSeconds <= 0 || step.Power == nil {
			continue
		}
		watts = append(watts, plannedStepWatts(&step, ftp)...)
	}

	return watts
}

func plannedStepWatts(step *PlannedWorkoutStep, ftp int) []float64 {
	watts := make([]float64, step.DurationSeconds)
	start := plannedPowerWattsForTarget(step.Power, ftp)
	end := start
	if step.Power.Value == 0 && step.Power.End > 0 {
		end = plannedPowerValueWatts(step.Power.End, step.Power.Units, ftp)
	}

	if len(watts) == 1 {
		watts[0] = start
		return watts
	}

	for second := range watts {
		fraction := float64(second) / float64(len(watts)-1)
		watts[second] = start + (end-start)*fraction
	}

	return watts
}

func plannedPowerWattsForTarget(target *WorkoutTarget, ftp int) float64 {
	if target == nil || ftp <= 0 {
		return 0
	}
	if target.Value > 0 {
		return plannedPowerValueWatts(target.Value, target.Units, ftp)
	}
	if target.Start > 0 {
		return plannedPowerValueWatts(target.Start, target.Units, ftp)
	}
	return plannedPowerValueWatts(target.End, target.Units, ftp)
}

func plannedPowerValueWatts(value float64, units string, ftp int) float64 {
	if strings.EqualFold(units, "%ftp") {
		return float64(ftp) * value / percentScale
	}
	return value
}
