package main

import icu "github.com/Thejuampi/icu"

func registerActivitiesCommands(registry *CommandRegistry) {
	registry.Register(
		"activities", "list",
		listQueryCommand[[]icu.Activity](
			"activities",
			"activities list --oldest DATE --newest DATE [--fields f1,f2] [--limit N]",
			"List activities for a date range.",
			queryBuilder("oldest", "newest", "fields", "limit", "route-id"),
		),
	)
	registry.Register("activities", "get", activitiesGetCommand())
	registry.Register("activities", "upload", activitiesUploadCommand())
	registry.Register("activities", "csv", downloadCommand("activities", "activities csv", "Download all activities as CSV."))
	registry.Register(
		"activities", "search",
		searchCommand[[]icu.ActivitySearchResult](
			"activities",
			"activities search <query> [--limit N]",
			"Search activities by name or #tag.",
			"search",
		),
	)
	registry.Register(
		"activities", "search-full",
		searchCommand[[]icu.Activity](
			"activities",
			"activities search-full <query> [--limit N]",
			"Search activities returning full objects.",
			"search-full",
		),
	)
	registry.Register(
		"activities", "interval-search",
		listQueryCommand[[]icu.Activity](
			"activities",
			"activities interval-search --minSecs N --maxSecs N --minIntensity N --maxIntensity N [--type auto|power|hr|pace]",
			"Find activities with intervals matching duration and intensity.",
			queryBuilder("minSecs", "maxSecs", "minIntensity", "maxIntensity", "type", "minReps", "maxReps", "limit"),
			"interval-search",
		),
	)
	registry.Register("activities", "around", activitiesAroundCommand())
	registry.Register(
		"activities", "manual",
		createCommand[icu.Activity, icu.Activity](
			"activities",
			"activities manual --type Ride --name NAME --moving-time SECS [--distance M] [--training-load N]",
			"Create a manual activity.",
			nil,
			func(flags map[string]string) icu.Activity {
				return icu.Activity{
					Type:           icu.StringFlag(flags, "type", "Ride"),
					Name:           icu.StringFlag(flags, "name", ""),
					MovingTime:     IntFlag(flags, "moving-time", 0),
					Distance:       floatFlagVal(flags, "distance", 0),
					TrainingLoad:   IntFlag(flags, "training-load", 0),
					StartDateLocal: icu.StringFlag(flags, "start-date", ""),
				}
			},
			"manual",
		),
	)
}

func activitiesGetCommand() *Command {
	return &Command{
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
				return wrapCommandError(err)
			}

			return writeJSON(acts)
		},
	}
}

func activitiesUploadCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activities upload <file> [--name NAME] [--description DESC] [--external-id ID]",
		Description: "Upload an activity file (fit/tcx/gpx/zip/gz).",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("file path")
			}

			q := queryFromFlags(flags, "name", "description", "external-id", "device-name", "paired-event-id")

			var resp icu.UploadResponse

			return wrapCommandError(client.UploadFile("activities", "", args[0], q, &resp))
		},
	}
}

func searchCommand[T any](resource, usage, description, endpoint string) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("search query")
			}

			q := map[string]string{"q": args[0]}
			if v := flags["limit"]; v != "" {
				q["limit"] = v
			}

			var results T
			if err := client.Get(resource, []string{endpoint}, q, &results); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(results)
		},
	}
}

func activitiesAroundCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activities around <activity-id> [--limit N] [--route-id ID]",
		Description: "List activities before/after an activity.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
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

			var acts []icu.Activity
			if err := client.Get("activities", []string{"around"}, q, &acts); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(acts)
		},
	}
}
