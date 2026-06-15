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
	registry.Register(
		"events", "list",
		listQueryCommand[[]icu.Event](
			"events",
			"events list --oldest DATE --newest DATE [--category WORKOUT] [--ext zwo|mrc|erg|fit] [--resolve]",
			"List calendar events.",
			eventsListQuery,
		),
	)
	registry.Register("events", "get", getByIDCommand[icu.Event]("events", "events get <id>", "Get event by ID.", "event id", nil))

	registry.Register(
		"events", "create",
		createCommand[icu.EventEx, icu.Event](
			"events",
			"events create --category WORKOUT --type Ride --name NAME --start-date DATE"+
				" [--moving-time SECS] [--training-load N] [--description DESC] [--upsert]",
			"Create calendar event.",
			eventsCreateQuery,
			func(flags map[string]string) icu.EventEx {
				return icu.EventEx{
					Category:       icu.StringFlag(flags, "category", "WORKOUT"),
					Type:           icu.StringFlag(flags, "type", "Ride"),
					Name:           icu.StringFlag(flags, "name", ""),
					StartDateLocal: icu.StringFlag(flags, "start-date", ""),
					MovingTime:     IntFlag(flags, "moving-time", 0),
					TrainingLoad:   IntFlag(flags, "training-load", 0),
					Description:    icu.StringFlag(flags, "description", ""),
					Color:          icu.StringFlag(flags, "color", ""),
					Indoor:         BoolFlag(flags, "indoor"),
					ExternalID:     icu.StringFlag(flags, "external-id", ""),
				}
			},
		),
	)

	registry.Register(
		"events", "update",
		updateByIDCommand[icu.EventEx, icu.Event](
			"events",
			"events update <id> --name NAME [--desc DESC]",
			"Update calendar event.",
			"event id",
			nil,
			func(flags map[string]string) icu.EventEx {
				var ev icu.EventEx
				if v := flags["name"]; v != "" {
					ev.Name = v
				}

				if v := flags["description"]; v != "" {
					ev.Description = v
				}

				if v := flags["training-load"]; v != "" {
					ev.TrainingLoad = IntFlag(flags, "training-load", 0)
				}

				return ev
			},
		),
	)

	registry.Register("events", "delete", deleteByIDCommand("events", "events delete <id>", "Delete calendar event.", "event id"))
	registry.Register("events", "download", eventsDownloadCommand())
	registry.Register("events", "tags", listAllCommand[[]string]("event-tags", "events tags", "List event tags."))
}

func eventsListQuery(flags map[string]string) map[string]string {
	q := queryFromFlags(flags, "oldest", "newest", "category", "ext", "limit", "calendar_id")
	if BoolFlag(flags, "resolve") {
		q["resolve"] = strTrue
	}

	return q
}

func eventsCreateQuery(flags map[string]string) map[string]string {
	q := map[string]string{
		"upsertOnUid": "false",
	}
	if BoolFlag(flags, "upsert") {
		q["upsertOnUid"] = "true"
	}

	return q
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
