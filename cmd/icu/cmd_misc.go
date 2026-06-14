package main

import icu "github.com/Thejuampi/icu"

func registerMiscCommands(registry *CommandRegistry) {
	registry.Register("fitness-events", "list", &Command{
		Name:        "",
		Usage:       "fitness-events list",
		Description: "List fitness model events (FITNESS_DAYS, SET_FITNESS, SET_EFTP).",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var events []icu.Event
			if err := client.Get("fitness-model-events", nil, nil, &events); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(events)
		},
	})

	registry.Register("tags", "activities", &Command{
		Name:        "",
		Usage:       "tags activities",
		Description: "List activity tags.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var tags []string
			if err := client.Get("activity-tags", nil, nil, &tags); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(tags)
		},
	})

	registry.Register("tags", "events", &Command{
		Name:        "",
		Usage:       "tags events",
		Description: "List event tags.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var tags []string
			if err := client.Get("event-tags", nil, nil, &tags); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(tags)
		},
	})

	registry.Register("tags", "workouts", &Command{
		Name:        "",
		Usage:       "tags workouts",
		Description: "List workout tags.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var tags []string
			if err := client.Get("workout-tags", nil, nil, &tags); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(tags)
		},
	})

	registry.Register("shared-event", "get", &Command{
		Name:        "",
		Usage:       "shared-event get <id>",
		Description: "Get a shared event.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("shared icu.Event id")
			}

			var v any
			if err := client.Get("shared-event", []string{args[0]}, nil, &v); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(v)
		},
	})
}
