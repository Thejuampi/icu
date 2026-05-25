package main

//nolint:funlen
func init() {
	RegisterCommand("workouts", "list", &Command{
		Name:        "",
		Usage:       "workouts list",
		Description: "List all workouts in library.",
		Run: func(_ []string, _ map[string]string, client *Client) error {
			var w []Workout
			if err := client.Get("workouts", nil, nil, &w); err != nil {
				return err
			}

			return WriteJSON(osStdout(), w)
		},
	})

	RegisterCommand("workouts", "get", &Command{
		Name:        "",
		Usage:       "workouts get <id>",
		Description: "Get a workout.",
		Run: func(args []string, _ map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("workout id")
			}

			var w Workout
			if err := client.Get("workouts", []string{args[0]}, nil, &w); err != nil {
				return err
			}

			return WriteJSON(osStdout(), w)
		},
	})

	RegisterCommand("workouts", "create", &Command{
		Name:        "",
		Usage:       "workouts create --name NAME --type Ride [--folder-id ID] [--desc DESC] [--training-load N]",
		Description: "Create a workout.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			var w WorkoutEx
			w.Name = StringFlag(flags, "name", "")
			w.Type = StringFlag(flags, "type", "Ride")
			w.FolderID = IntFlag(flags, "folder-id", 0)
			w.Description = StringFlag(flags, "desc", "")
			w.TrainingLoad = IntFlag(flags, "training-load", 0)
			w.MovingTime = IntFlag(flags, "moving-time", 0)

			var result Workout
			if err := client.Post("workouts", nil, nil, w, &result); err != nil {
				return err
			}

			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("workouts", "update", &Command{
		Name:        "",
		Usage:       "workouts update <id> --name NAME",
		Description: "Update a workout.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("workout id")
			}

			var w WorkoutEx
			if v := flags["name"]; v != "" {
				w.Name = v
			}

			if v := flags["desc"]; v != "" {
				w.Description = v
			}

			var result Workout
			if err := client.Put("workouts", []string{args[0]}, nil, w, &result); err != nil {
				return err
			}

			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("workouts", "delete", &Command{
		Name:        "",
		Usage:       "workouts delete <id>",
		Description: "Delete a workout.",
		Run: func(args []string, _ map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("workout id")
			}

			return client.Delete("workouts", []string{args[0]}, nil, nil)
		},
	})

	RegisterCommand("workouts", "tags", &Command{
		Name:        "",
		Usage:       "workouts tags",
		Description: "List workout tags.",
		Run: func(_ []string, _ map[string]string, client *Client) error {
			var tags []string
			if err := client.Get("workouts", []string{"tags"}, nil, &tags); err != nil {
				return err
			}

			return WriteJSON(osStdout(), tags)
		},
	})
}
