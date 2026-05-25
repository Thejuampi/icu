package main

import (
	"encoding/json"
	"fmt"
	"os"
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

//nolint:gocognit,gocyclo,cyclop,funlen
func init() {
	RegisterCommand("events", "list", &Command{
		Name:        "",
		Usage:       "events list --oldest DATE --newest DATE [--category WORKOUT] [--ext zwo|mrc|erg|fit] [--resolve]",
		Description: "List calendar events.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "oldest", "newest", "category", "ext", "limit", "calendar_id")
			if BoolFlag(flags, "resolve") {
				q["resolve"] = strTrue
			}

			var events []Event
			if err := client.Get("events", nil, q, &events); err != nil {
				return err
			}

			return WriteJSON(osStdout(), events)
		},
	})

	RegisterCommand("events", "get", &Command{
		Name:        "",
		Usage:       "events get <id>",
		Description: "Get event by ID.",
		Run: func(args []string, _ map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			var e Event
			if err := client.Get("events", []string{args[0]}, nil, &e); err != nil {
				return err
			}

			return WriteJSON(osStdout(), e)
		},
	})

	RegisterCommand("events", "create", &Command{
		Name: "",
		Usage: "events create --category WORKOUT --type Ride --name NAME --start-date DATE" +
			" [--moving-time SECS] [--training-load N] [--desc DESC]",
		Description: "Create calendar event.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			var ev EventEx
			ev.Category = StringFlag(flags, "category", "WORKOUT")
			ev.Type = StringFlag(flags, "type", "Ride")
			ev.Name = StringFlag(flags, "name", "")
			ev.StartDateLocal = StringFlag(flags, "start-date", "")
			ev.MovingTime = IntFlag(flags, "moving-time", 0)
			ev.TrainingLoad = IntFlag(flags, "training-load", 0)
			ev.Description = StringFlag(flags, "desc", "")
			ev.Color = StringFlag(flags, "color", "")
			ev.Indoor = BoolFlag(flags, "indoor")
			ev.ExternalID = StringFlag(flags, "external-id", "")

			var result Event
			if err := client.Post("events", nil, nil, ev, &result); err != nil {
				return err
			}

			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("events", "update", &Command{
		Name:        "",
		Usage:       "events update <id> --name NAME [--desc DESC]",
		Description: "Update calendar event.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			var ev EventEx
			if v := flags["name"]; v != "" {
				ev.Name = v
			}

			if v := flags["desc"]; v != "" {
				ev.Description = v
			}

			if v := flags["training-load"]; v != "" {
				ev.TrainingLoad = IntFlag(flags, "training-load", 0)
			}

			var result Event
			if err := client.Put("events", []string{args[0]}, nil, ev, &result); err != nil {
				return err
			}

			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("events", "delete", &Command{
		Name:        "",
		Usage:       "events delete <id>",
		Description: "Delete calendar event.",
		Run: func(args []string, _ map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			return client.Delete("events", []string{args[0]}, nil, nil)
		},
	})

	RegisterCommand("events", "download", &Command{
		Name:        "",
		Usage:       "events download <id> --ext zwo|mrc|erg|fit",
		Description: "Download planned workout file.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			ext := StringFlag(flags, "ext", "zwo")

			data, err := client.Download("events", []string{args[0], "download." + ext}, nil)
			if err != nil {
				return err
			}

			if _, err := osStdout().Write(data); err != nil {
				return fmt.Errorf("writing output: %w", err)
			}

			return nil
		},
	})

	RegisterCommand("events", "tags", &Command{
		Name:        "",
		Usage:       "events tags",
		Description: "List event tags.",
		Run: func(_ []string, _ map[string]string, client *Client) error {
			var tags []string
			if err := client.Get("event-tags", nil, nil, &tags); err != nil {
				return err
			}

			return WriteJSON(osStdout(), tags)
		},
	})
}
