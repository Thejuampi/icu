package main

import (
	"fmt"

	icu "github.com/Thejuampi/icu"
)

//nolint:gocognit,gocyclo,cyclop,funlen
func init() {
	RegisterCommand("activities", "list", &Command{
		Name:        "",
		Usage:       "activities list --oldest DATE --newest DATE [--fields f1,f2] [--limit N]",
		Description: "List activities for a date range.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := queryFromFlags(flags, "oldest", "newest", "fields", "limit", "route_id")

			var acts []icu.Activity
			if err := client.Get("activities", nil, q, &acts); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), acts)
		},
	})

	RegisterCommand("activities", "get", &Command{
		Name:        "",
		Usage:       "activities get <id1> [id2 ...]",
		Description: "Fetch activities by ID.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Activity id(s)")
			}

			var acts []icu.Activity
			ids := args[0]
			for i := 1; i < len(args); i++ {
				ids += "," + args[i]
			}

			if err := client.Get("activities", []string{ids}, nil, &acts); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), acts)
		},
	})

	RegisterCommand("activities", "upload", &Command{
		Name:        "",
		Usage:       "activities upload <file> [--name NAME] [--desc DESC] [--external-id ID]",
		Description: "Upload icu.Activity file (fit/tcx/gpx/zip/gz).",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("file path")
			}

			q := queryFromFlags(flags, "name", "description", "external_id", "device_name", "paired_event_id")

			var resp icu.UploadResponse

			return client.UploadFile("activities", "", args[0], q, &resp)
		},
	})

	RegisterCommand("activities", "csv", &Command{
		Name:        "",
		Usage:       "activities csv",
		Description: "Download all activities as CSV.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			data, err := client.Download("activities", []string{}, nil)
			if err != nil {
				return err
			}

			if _, err := osStdout().Write(data); err != nil {
				return fmt.Errorf("writing output: %w", err)
			}

			return nil
		},
	})

	RegisterCommand("activities", "search", &Command{
		Name:        "",
		Usage:       "activities search <query> [--limit N]",
		Description: "Search activities by name or #tag.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("search query")
			}

			q := map[string]string{"q": args[0]}
			if v := flags["limit"]; v != "" {
				q["limit"] = v
			}

			var results []icu.ActivitySearchResult
			if err := client.Get("activities", []string{"search"}, q, &results); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), results)
		},
	})

	RegisterCommand("activities", "search-full", &Command{
		Name:        "",
		Usage:       "activities search-full <query> [--limit N]",
		Description: "Search activities returning full objects.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("search query")
			}

			q := map[string]string{"q": args[0]}
			if v := flags["limit"]; v != "" {
				q["limit"] = v
			}

			var results []icu.Activity
			if err := client.Get("activities", []string{"search-full"}, q, &results); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), results)
		},
	})

	RegisterCommand("activities", "icu.Interval-search", &Command{
		Name:        "",
		Usage:       "activities icu.Interval-search --min-secs N --max-secs N --min-intensity N --max-intensity N [--type auto|power|hr|pace]",
		Description: "Find activities with intervals matching duration and intensity.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := queryFromFlags(flags, "minSecs", "maxSecs", "minIntensity", "maxIntensity", "type", "minReps", "maxReps", "limit")

			var acts []icu.Activity
			if err := client.Get("activities", []string{"icu.Interval-search"}, q, &acts); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), acts)
		},
	})

	RegisterCommand("activities", "around", &Command{
		Name:        "",
		Usage:       "activities around <icu.Activity-id> [--limit N] [--icu.Route-id ID]",
		Description: "List activities before/after an icu.Activity.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			q := map[string]string{"activity_id": args[0]}
			if v := flags["limit"]; v != "" {
				q["limit"] = v
			}

			if v := flags["icu.Route-id"]; v != "" {
				q["route_id"] = v
			}

			var acts []icu.Activity
			if err := client.Get("activities", []string{"around"}, q, &acts); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), acts)
		},
	})

	RegisterCommand("activities", "manual", &Command{
		Name:        "",
		Usage:       "activities manual --type Ride --name NAME --moving-time SECS [--distance M] [--training-load N]",
		Description: "Create a manual icu.Activity.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			var a icu.Activity
			a.Type = icu.StringFlag(flags, "type", "Ride")
			a.Name = icu.StringFlag(flags, "name", "")
			a.MovingTime = IntFlag(flags, "moving-time", 0)
			a.Distance = floatFlagVal(flags, "distance", 0)
			a.TrainingLoad = IntFlag(flags, "training-load", 0)
			a.StartDateLocal = icu.StringFlag(flags, "start-date", "")

			var result icu.Activity
			if err := client.Post("activities", []string{"manual"}, nil, a, &result); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), result)
		},
	})
}
