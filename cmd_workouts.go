package main

func init() {
	RegisterCommand("workouts", "list", &Command{
		Usage:       "workouts list",
		Description: "List all workouts in library.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var w []Workout
			if err := client.Get("workouts", nil, nil, &w); err != nil {
				return err
			}
			return WriteJSON(osStdout(), w)
		},
	})

	RegisterCommand("workouts", "get", &Command{
		Usage:       "workouts get <id>",
		Description: "Get a workout.",
		Run: func(args []string, flags map[string]string, client *Client) error {
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
		Usage:       "workouts create --name NAME --type Ride [--folder-id ID] [--desc DESC] [--training-load N]",
		Description: "Create a workout.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			w := WorkoutEx{
				Name:          StringFlag(flags, "name", ""),
				Type:          StringFlag(flags, "type", "Ride"),
				FolderID:      IntFlag(flags, "folder-id", 0),
				Description:   StringFlag(flags, "desc", ""),
				TrainingLoad:  IntFlag(flags, "training-load", 0),
				MovingTime:    IntFlag(flags, "moving-time", 0),
			}
			var result Workout
			if err := client.Post("workouts", nil, nil, w, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("workouts", "update", &Command{
		Usage:       "workouts update <id> --name NAME",
		Description: "Update a workout.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("workout id")
			}
			w := WorkoutEx{}
			if v := flags["name"]; v != "" { w.Name = v }
			if v := flags["desc"]; v != "" { w.Description = v }
			var result Workout
			if err := client.Put("workouts", []string{args[0]}, nil, w, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("workouts", "delete", &Command{
		Usage:       "workouts delete <id>",
		Description: "Delete a workout.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("workout id")
			}
			return client.Delete("workouts", []string{args[0]}, nil, nil)
		},
	})

	RegisterCommand("workouts", "tags", &Command{
		Usage:       "workouts tags",
		Description: "List workout tags.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var tags []string
			if err := client.Get("workouts", []string{"tags"}, nil, &tags); err != nil {
				return err
			}
			return WriteJSON(osStdout(), tags)
		},
	})
}
