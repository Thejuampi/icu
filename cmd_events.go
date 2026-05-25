package main

import (
	"encoding/json"
	"os"
)

func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

func init() {
	RegisterCommand("events", "list", &Command{
		Usage:       "events list --oldest DATE --newest DATE [--category WORKOUT] [--ext zwo|mrc|erg|fit] [--resolve]",
		Description: "List calendar events.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "oldest", "newest", "category", "ext", "limit", "calendar_id")
			if BoolFlag(flags, "resolve") {
				q["resolve"] = "true"
			}
			var events []Event
			if err := client.Get("events", nil, q, &events); err != nil {
				return err
			}
			return WriteJSON(osStdout(), events)
		},
	})

	RegisterCommand("events", "get", &Command{
		Usage:       "events get <id>",
		Description: "Get event by ID.",
		Run: func(args []string, flags map[string]string, client *Client) error {
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
		Usage:       "events create --category WORKOUT --type Ride --name NAME --start-date DATE [--moving-time SECS] [--training-load N] [--desc DESC]",
		Description: "Create calendar event.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			ev := EventEx{
				Category:       StringFlag(flags, "category", "WORKOUT"),
				Type:           StringFlag(flags, "type", "Ride"),
				Name:           StringFlag(flags, "name", ""),
				StartDateLocal: StringFlag(flags, "start-date", ""),
				MovingTime:     IntFlag(flags, "moving-time", 0),
				TrainingLoad:   IntFlag(flags, "training-load", 0),
				Description:    StringFlag(flags, "desc", ""),
				Color:          StringFlag(flags, "color", ""),
				Indoor:         BoolFlag(flags, "indoor"),
				ExternalID:     StringFlag(flags, "external-id", ""),
			}
			var result Event
			if err := client.Post("events", nil, nil, ev, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("events", "update", &Command{
		Usage:       "events update <id> --name NAME [--desc DESC]",
		Description: "Update calendar event.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}
			ev := EventEx{}
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
		Usage:       "events delete <id>",
		Description: "Delete calendar event.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}
			return client.Delete("events", []string{args[0]}, nil, nil)
		},
	})

	RegisterCommand("events", "download", &Command{
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
			_, err = osStdout().Write(data)
			return err
		},
	})

	RegisterCommand("events", "tags", &Command{
		Usage:       "events tags",
		Description: "List event tags.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var tags []string
			if err := client.Get("event-tags", nil, nil, &tags); err != nil {
				return err
			}
			return WriteJSON(osStdout(), tags)
		},
	})
}
