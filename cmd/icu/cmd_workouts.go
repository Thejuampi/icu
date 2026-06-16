package main

import icu "github.com/Thejuampi/icu"

func registerWorkoutsCommands(registry *CommandRegistry) {
	registry.Register("workouts", "list", listAllCommand[[]icu.Workout]("workouts", "workouts list", "List all workouts in library."))
	registry.Register("workouts", "get", getByIDCommand[icu.Workout]("workouts", "workouts get <id>", "Get a workout.", "icu.Workout id", nil))

	registry.Register("workouts", "create", createCommand[icu.WorkoutEx, icu.Workout](
		"workouts",
		"workouts create --name NAME --type Ride [--folder-id ID] [--desc DESC] [--training-load N]",
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
		"workouts update <id> --name NAME",
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
