package main

import icu "github.com/Thejuampi/icu"

func init() {
	RegisterCommand("fitness-events", "list", &Command{
		Name:        "",
		Usage:       "fitness-events list",
		Description: "List fitness model events (FITNESS_DAYS, SET_FITNESS, SET_EFTP).",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var events []icu.Event
			if err := client.Get("fitness-model-events", nil, nil, &events); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), events)
		},
	})

	RegisterCommand("tags", "activities", &Command{
		Name:        "",
		Usage:       "tags activities",
		Description: "List icu.Activity tags.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var tags []string
			if err := client.Get("icu.Activity-tags", nil, nil, &tags); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), tags)
		},
	})

	RegisterCommand("tags", "events", &Command{
		Name:        "",
		Usage:       "tags events",
		Description: "List event tags.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var tags []string
			if err := client.Get("event-tags", nil, nil, &tags); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), tags)
		},
	})

	RegisterCommand("tags", "workouts", &Command{
		Name:        "",
		Usage:       "tags workouts",
		Description: "List icu.Workout tags.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var tags []string
			if err := client.Get("icu.Workout-tags", nil, nil, &tags); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), tags)
		},
	})

	RegisterCommand("shared-event", "get", &Command{
		Name:        "",
		Usage:       "shared-event get <id>",
		Description: "Get a shared icu.Event.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("shared icu.Event id")
			}

			var v any
			if err := client.Get("shared-event", []string{args[0]}, nil, &v); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), v)
		},
	})
}
