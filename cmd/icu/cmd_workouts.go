package main

import icu "github.com/Thejuampi/icu"

func registerWorkoutsCommands(registry *CommandRegistry) {
	registry.Register("workouts", "list", workoutsListCommand())
	registry.Register("workouts", "get", workoutsGetCommand())
	registry.Register("workouts", "create", workoutsCreateCommand())
	registry.Register("workouts", "update", workoutsUpdateCommand())
	registry.Register("workouts", "delete", workoutsDeleteCommand())
	registry.Register("workouts", "tags", workoutsTagsCommand())
}

func workoutsListCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "workouts list",
		Description: "List all workouts in library.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var w []icu.Workout
			if err := client.Get("workouts", nil, nil, &w); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(w)
		},
	}
}

func workoutsGetCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "workouts get <id>",
		Description: "Get a icu.Workout.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Workout id")
			}

			var w icu.Workout
			if err := client.Get("workouts", []string{args[0]}, nil, &w); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(w)
		},
	}
}

func workoutsCreateCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "workouts create --name NAME --type Ride [--folder-id ID] [--desc DESC] [--training-load N]",
		Description: "Create a icu.Workout.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			var w icu.WorkoutEx
			w.Name = icu.StringFlag(flags, "name", "")
			w.Type = icu.StringFlag(flags, "type", "Ride")
			w.FolderID = IntFlag(flags, "folder-id", 0)
			w.Description = icu.StringFlag(flags, "desc", "")
			w.TrainingLoad = IntFlag(flags, "training-load", 0)
			w.MovingTime = IntFlag(flags, "moving-time", 0)

			var result icu.Workout
			if err := client.Post("workouts", nil, nil, w, &result); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	}
}

func workoutsUpdateCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "workouts update <id> --name NAME",
		Description: "Update a icu.Workout.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Workout id")
			}

			var w icu.WorkoutEx
			if v := flags["name"]; v != "" {
				w.Name = v
			}

			if v := flags["desc"]; v != "" {
				w.Description = v
			}

			var result icu.Workout
			if err := client.Put("workouts", []string{args[0]}, nil, w, &result); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	}
}

func workoutsDeleteCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "workouts delete <id>",
		Description: "Delete a icu.Workout.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Workout id")
			}

			return wrapCommandError(client.Delete("workouts", []string{args[0]}, nil, nil))
		},
	}
}

func workoutsTagsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "workouts tags",
		Description: "List icu.Workout tags.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var tags []string
			if err := client.Get("workouts", []string{"tags"}, nil, &tags); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(tags)
		},
	}
}
