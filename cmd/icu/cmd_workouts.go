package main

import icu "github.com/Thejuampi/icu"

//nolint:funlen
func init() {
	RegisterCommand("workouts", "list", &Command{
		Name:        "",
		Usage:       "workouts list",
		Description: "List all workouts in library.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var w []icu.Workout
			if err := client.Get("workouts", nil, nil, &w); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), w)
		},
	})

	RegisterCommand("workouts", "get", &Command{
		Name:        "",
		Usage:       "workouts get <id>",
		Description: "Get a icu.Workout.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Workout id")
			}

			var w icu.Workout
			if err := client.Get("workouts", []string{args[0]}, nil, &w); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), w)
		},
	})

	RegisterCommand("workouts", "create", &Command{
		Name:        "",
		Usage:       "workouts create --name NAME --type Ride [--icu.Folder-id ID] [--desc DESC] [--training-load N]",
		Description: "Create a icu.Workout.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			var w icu.WorkoutEx
			w.Name = icu.StringFlag(flags, "name", "")
			w.Type = icu.StringFlag(flags, "type", "Ride")
			w.FolderID = IntFlag(flags, "icu.Folder-id", 0)
			w.Description = icu.StringFlag(flags, "desc", "")
			w.TrainingLoad = IntFlag(flags, "training-load", 0)
			w.MovingTime = IntFlag(flags, "moving-time", 0)

			var result icu.Workout
			if err := client.Post("workouts", nil, nil, w, &result); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("workouts", "update", &Command{
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
				return err
			}

			return icu.WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("workouts", "delete", &Command{
		Name:        "",
		Usage:       "workouts delete <id>",
		Description: "Delete a icu.Workout.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Workout id")
			}

			return client.Delete("workouts", []string{args[0]}, nil, nil)
		},
	})

	RegisterCommand("workouts", "tags", &Command{
		Name:        "",
		Usage:       "workouts tags",
		Description: "List icu.Workout tags.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var tags []string
			if err := client.Get("workouts", []string{"tags"}, nil, &tags); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), tags)
		},
	})
}
