package main

import "fmt"

//nolint:gocognit,gocyclo,cyclop,funlen
func init() {
	RegisterCommand("activities", "list", &Command{
		Name:        "",
		Usage:       "activities list --oldest DATE --newest DATE [--fields f1,f2] [--limit N]",
		Description: "List activities for a date range.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "oldest", "newest", "fields", "limit", "route_id")

			var acts []Activity
			if err := client.Get("activities", nil, q, &acts); err != nil {
				return err
			}

			return WriteJSON(osStdout(), acts)
		},
	})

	RegisterCommand("activities", "get", &Command{
		Name:        "",
		Usage:       "activities get <id1> [id2 ...]",
		Description: "Fetch activities by ID.",
		Run: func(args []string, _ map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id(s)")
			}

			var acts []Activity
			ids := args[0]
			for i := 1; i < len(args); i++ {
				ids += "," + args[i]
			}

			if err := client.Get("activities", []string{ids}, nil, &acts); err != nil {
				return err
			}

			return WriteJSON(osStdout(), acts)
		},
	})

	RegisterCommand("activities", "upload", &Command{
		Name:        "",
		Usage:       "activities upload <file> [--name NAME] [--desc DESC] [--external-id ID]",
		Description: "Upload activity file (fit/tcx/gpx/zip/gz).",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("file path")
			}

			q := queryFromFlags(flags, "name", "description", "external_id", "device_name", "paired_event_id")

			var resp UploadResponse

			return client.UploadFile("activities", "", args[0], q, &resp)
		},
	})

	RegisterCommand("activities", "csv", &Command{
		Name:        "",
		Usage:       "activities csv",
		Description: "Download all activities as CSV.",
		Run: func(_ []string, _ map[string]string, client *Client) error {
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
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("search query")
			}

			q := map[string]string{"q": args[0]}
			if v := flags["limit"]; v != "" {
				q["limit"] = v
			}

			var results []ActivitySearchResult
			if err := client.Get("activities", []string{"search"}, q, &results); err != nil {
				return err
			}

			return WriteJSON(osStdout(), results)
		},
	})

	RegisterCommand("activities", "search-full", &Command{
		Name:        "",
		Usage:       "activities search-full <query> [--limit N]",
		Description: "Search activities returning full objects.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("search query")
			}

			q := map[string]string{"q": args[0]}
			if v := flags["limit"]; v != "" {
				q["limit"] = v
			}

			var results []Activity
			if err := client.Get("activities", []string{"search-full"}, q, &results); err != nil {
				return err
			}

			return WriteJSON(osStdout(), results)
		},
	})

	RegisterCommand("activities", "interval-search", &Command{
		Name:        "",
		Usage:       "activities interval-search --min-secs N --max-secs N --min-intensity N --max-intensity N [--type auto|power|hr|pace]",
		Description: "Find activities with intervals matching duration and intensity.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "minSecs", "maxSecs", "minIntensity", "maxIntensity", "type", "minReps", "maxReps", "limit")

			var acts []Activity
			if err := client.Get("activities", []string{"interval-search"}, q, &acts); err != nil {
				return err
			}

			return WriteJSON(osStdout(), acts)
		},
	})

	RegisterCommand("activities", "around", &Command{
		Name:        "",
		Usage:       "activities around <activity-id> [--limit N] [--route-id ID]",
		Description: "List activities before/after an activity.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			q := map[string]string{"activity_id": args[0]}
			if v := flags["limit"]; v != "" {
				q["limit"] = v
			}

			if v := flags["route-id"]; v != "" {
				q["route_id"] = v
			}

			var acts []Activity
			if err := client.Get("activities", []string{"around"}, q, &acts); err != nil {
				return err
			}

			return WriteJSON(osStdout(), acts)
		},
	})

	RegisterCommand("activities", "manual", &Command{
		Name:        "",
		Usage:       "activities manual --type Ride --name NAME --moving-time SECS [--distance M] [--training-load N]",
		Description: "Create a manual activity.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			var a Activity
			a.Type = StringFlag(flags, "type", "Ride")
			a.Name = StringFlag(flags, "name", "")
			a.MovingTime = IntFlag(flags, "moving-time", 0)
			a.Distance = floatFlagVal(flags, "distance", 0)
			a.TrainingLoad = IntFlag(flags, "training-load", 0)
			a.StartDateLocal = StringFlag(flags, "start-date", "")

			var result Activity
			if err := client.Post("activities", []string{"manual"}, nil, a, &result); err != nil {
				return err
			}

			return WriteJSON(osStdout(), result)
		},
	})
}
