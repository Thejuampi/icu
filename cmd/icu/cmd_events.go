package main

import (
	"encoding/json"
	"fmt"
	"os"

	icu "github.com/Thejuampi/icu"
)

func osReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}

	return data, nil
}

func jsonUnmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	return nil
}

func registerEventsCommands(registry *CommandRegistry) {
	registry.Register("events", "list", eventsListCommand())
	registry.Register("events", "get", eventsGetCommand())
	registry.Register("events", "create", eventsCreateCommand())
	registry.Register("events", "update", eventsUpdateCommand())
	registry.Register("events", "delete", eventsDeleteCommand())
	registry.Register("events", "download", eventsDownloadCommand())
	registry.Register("events", "tags", eventsTagsCommand())
}

func eventsListCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "events list --oldest DATE --newest DATE [--category WORKOUT] [--ext zwo|mrc|erg|fit] [--resolve]",
		Description: "List calendar events.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := queryFromFlags(flags, "oldest", "newest", "category", "ext", "limit", "calendar_id")
			if BoolFlag(flags, "resolve") {
				q["resolve"] = strTrue
			}

			var events []icu.Event
			if err := client.Get("events", nil, q, &events); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(events)
		},
	}
}

func eventsGetCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "events get <id>",
		Description: "Get event by ID.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			var e icu.Event
			if err := client.Get("events", []string{args[0]}, nil, &e); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(e)
		},
	}
}

func eventsCreateCommand() *Command {
	return &Command{
		Name: "",
		Usage: "events create --category WORKOUT --type Ride --name NAME --start-date DATE" +
			" [--moving-time SECS] [--training-load N] [--desc DESC] [--upsert]",
		Description: "Create calendar event.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			var ev icu.EventEx
			ev.Category = icu.StringFlag(flags, "category", "WORKOUT")
			ev.Type = icu.StringFlag(flags, "type", "Ride")
			ev.Name = icu.StringFlag(flags, "name", "")
			ev.StartDateLocal = icu.StringFlag(flags, "start-date", "")
			ev.MovingTime = IntFlag(flags, "moving-time", 0)
			ev.TrainingLoad = IntFlag(flags, "training-load", 0)
			ev.Description = icu.StringFlag(flags, "desc", "")
			ev.Color = icu.StringFlag(flags, "color", "")
			ev.Indoor = BoolFlag(flags, "indoor")
			ev.ExternalID = icu.StringFlag(flags, "external-id", "")

			query := map[string]string{
				"upsertOnUid": "false",
			}
			if BoolFlag(flags, "upsert") {
				query["upsertOnUid"] = "true"
			}

			var result icu.Event
			if err := client.Post("events", nil, query, ev, &result); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	}
}

func eventsUpdateCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "events update <id> --name NAME [--desc DESC]",
		Description: "Update calendar event.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			var ev icu.EventEx
			if v := flags["name"]; v != "" {
				ev.Name = v
			}

			if v := flags["desc"]; v != "" {
				ev.Description = v
			}

			if v := flags["training-load"]; v != "" {
				ev.TrainingLoad = IntFlag(flags, "training-load", 0)
			}

			var result icu.Event
			if err := client.Put("events", []string{args[0]}, nil, ev, &result); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	}
}

func eventsDeleteCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "events delete <id>",
		Description: "Delete calendar event.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			return wrapCommandError(client.Delete("events", []string{args[0]}, nil, nil))
		},
	}
}

func eventsDownloadCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "events download <id> --ext zwo|mrc|erg|fit",
		Description: "Download planned workout file.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			ext := icu.StringFlag(flags, "ext", "zwo")

			data, err := client.Download("events", []string{args[0], "download." + ext}, nil)
			if err != nil {
				return wrapCommandError(err)
			}

			return writeOutput(data)
		},
	}
}

func eventsTagsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "events tags",
		Description: "List event tags.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var tags []string
			if err := client.Get("event-tags", nil, nil, &tags); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(tags)
		},
	}
}
