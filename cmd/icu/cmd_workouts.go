package main

import (
	"fmt"
	"io"

	icu "github.com/Thejuampi/icu"
)

func registerWorkoutsCommands(registry *CommandRegistry) {
	registry.Register("workouts", "list", listAllCommand[[]icu.Workout]("workouts", "workouts list", "List all workouts in library."))
	registry.Register("workouts", "get", getByIDCommand[icu.Workout]("workouts", "workouts get <id>", "Get a workout.", "icu.Workout id", nil))
	registry.Register("workouts", "calculate", workoutsCalculateCommand())

	registry.Register("workouts", "create", createCommand[icu.WorkoutEx, icu.Workout](
		"workouts",
		"workouts create --name NAME --type Ride [--folder-id ID] [--desc DESC] [--training-load N] [--moving-time SECS]",
		"Create a workout.",
		nil,
		func(flags map[string]string) icu.WorkoutEx {
			return icu.WorkoutEx{
				Name:         icu.StringFlag(flags, "name", ""),
				Type:         icu.StringFlag(flags, "type", "Ride"),
				FolderID:     IntFlag(flags, "folder-id", 0),
				Description:  icu.StringFlag(flags, "desc", ""),
				TrainingLoad: IntFlag(flags, "training-load", 0),
				MovingTime:   IntFlag(flags, "moving-time", 0),
			}
		},
	))

	registry.Register("workouts", "update", updateByIDCommand[icu.WorkoutEx, icu.Workout](
		"workouts",
		"workouts update <id> [--name NAME] [--desc DESC]",
		"Update a workout.",
		"icu.Workout id",
		nil,
		func(flags map[string]string) icu.WorkoutEx {
			var w icu.WorkoutEx
			if v := flags["name"]; v != "" {
				w.Name = v
			}

			if v := flags["desc"]; v != "" {
				w.Description = v
			}

			return w
		},
	))

	registry.Register("workouts", "delete", deleteByIDCommand("workouts", "workouts delete <id>", "Delete a workout.", "icu.Workout id"))
	registry.Register("workouts", "tags", listAllCommand[[]string]("workouts", "workouts tags", "List workout tags.", "tags"))
}

func workoutsCalculateCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "workouts calculate --ftp N --desc DESC",
		Description: "Calculate planned workout duration, IF, NP, and TSS locally.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			estimate, err := workoutEstimateFromFlags(flags)
			if err != nil {
				return wrapCommandError(err)
			}
			if icu.WorkoutDescriptionUsesPowerZones(stringFlagAlias(flags, "desc", "description")) {
				// Offline by design: no sport-settings, so zones use the
				// default bands (the client's fallback), announced here.
				fmt.Fprintf(stderr, "note: workouts calculate runs offline; Z1-Z%d priced with the intervals.icu default bands %v\n",
					len(icu.DefaultPowerZoneBounds()), icu.DefaultPowerZoneBounds())
			}

			for _, warning := range estimate.TargetWarnings {
				fmt.Fprintln(stderr, "warning: "+warning)
			}

			return writeJSON(stdout, estimate)
		},
	}
}

// workoutEstimate is the calculate output: the estimate plus warnings for
// targets above icu.WarnPercentFTPTarget % FTP (also printed to stderr).
type workoutEstimate struct {
	icu.PlannedLoadEstimate
	TargetWarnings []string `json:"targetWarnings,omitempty"`
}

func workoutEstimateFromFlags(flags map[string]string) (workoutEstimate, error) {
	desc := icu.StringFlag(flags, "desc", "")
	if desc == "" {
		desc = icu.StringFlag(flags, "description", "")
	}
	if desc == "" {
		return workoutEstimate{}, errMissing("desc")
	}

	doc, err := icu.ParseWorkoutDescription(desc)
	if err != nil {
		return workoutEstimate{}, fmt.Errorf("parse workout description: %w", err)
	}
	warnings := icu.ValidateWorkoutTargets(doc.Steps, IntFlag(flags, "ftp", 0))

	return workoutEstimate{
		PlannedLoadEstimate: icu.EstimatePlannedLoad(doc, icu.FTPSelectionFromWatts(IntFlag(flags, "ftp", 0))),
		TargetWarnings:      warnings,
	}, nil
}
