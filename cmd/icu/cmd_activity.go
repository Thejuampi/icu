package main

import (
	"errors"
	"fmt"

	icu "github.com/Thejuampi/icu"
)

var errMissingRequired = errors.New("missing required argument")

func errMissing(what string) error {
	return fmt.Errorf("%w: %s", errMissingRequired, what)
}

func registerActivityCommands(registry *CommandRegistry) {
	registry.Register("activity", "show", activityShowCommand())
	registry.Register("activity", "update", activityUpdateCommand())
	registry.Register("activity", "delete", activityDeleteCommand())
	registry.Register("activity", "intervals", activityIntervalsCommand())
	registry.Register("activity", "streams", activityStreamsCommand())
	registry.Register("activity", "power-vs-hr", activityPowerVsHRCommand())
	registry.Register("activity", "weather", activityWeatherCommand())
	registry.Register("activity", "file", activityFileCommand())
	registerActivityCurveCommands(registry)
	registerActivityDetailCommands(registry)
	registerActivityDownloadCommands(registry)
}

func activityShowCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activity <id> show [--intervals]",
		Description: "Get a single activity.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			return activityCmd(args, client)
		},
	}
}

func activityUpdateCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activity <id> update --name NAME [--desc DESC] [--type Ride]",
		Description: "Update an activity.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			var a icu.Activity
			if v := flags["name"]; v != "" {
				a.Name = v
			}

			if v := flags["description"]; v != "" {
				a.Description = v
			}

			if v := flags["type"]; v != "" {
				a.Type = v
			}

			var result icu.Activity
			if err := client.Put("activity", []string{args[0]}, nil, a, &result); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	}
}

func activityDeleteCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activity <id> delete",
		Description: "Delete an activity.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			var resp icu.DeleteResponse
			if err := client.Delete("activity", []string{args[0]}, nil, &resp); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(resp)
		},
	}
}

func activityIntervalsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activity <id> intervals",
		Description: "Get detected intervals.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			var dto icu.IntervalsDTO
			if err := client.Get("activity", []string{args[0], "intervals"}, nil, &dto); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(dto)
		},
	}
}

func activityStreamsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activity <id> streams [--types watts,heartrate,cadence]",
		Description: "Get activity streams.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			q := queryFromFlags(flags, "types")

			var streams []icu.ActivityStream
			if err := client.Get("activity", []string{args[0], "streams"}, q, &streams); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(streams)
		},
	}
}

func registerActivityCurveCommands(registry *CommandRegistry) {
	registerActivityCurve(registry, "power-curve", "Get activity power curve.")
	registerActivityCurve(registry, "hr-curve", "Get activity HR curve.")
	registerActivityCurve(registry, "pace-curve", "Get activity pace curve.")
}

func activityPowerVsHRCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activity <id> power-vs-hr",
		Description: "Get power vs HR data.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			var v any
			if err := client.Get("activity", []string{args[0], "power-vs-hr"}, nil, &v); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(v)
		},
	}
}

func registerActivityDetailCommands(registry *CommandRegistry) {
	registerActivityDetail(registry, "best-efforts", "Find best efforts.", "stream", "duration", "distance", "count")
	registerActivityDetail(registry, "map", "Get map data.", "bounds", "weather")
	registerActivityDetail(registry, "weather-summary", "Get weather summary.", "descr_config")
	registerActivityDetail(registry, "segments", "Get activity segments.")
	registerActivityDetail(registry, "messages", "Get activity comments.")
}

func activityWeatherCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activity <id> weather",
		Description: "Get weather summary (alias for weather-summary).",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			var v any
			if err := client.Get("activity", []string{args[0], "weather-summary"}, nil, &v); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(v)
		},
	}
}

func activityFileCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "activity <id> file",
		Description: "Download original activity file.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			data, err := client.Download("activity", []string{args[0], "file"}, nil)
			if err != nil {
				return wrapCommandError(err)
			}

			return writeOutput(data)
		},
	}
}

func registerActivityDownloadCommands(registry *CommandRegistry) {
	registerActivityDownload(registry, "fit-file")
	registerActivityDownload(registry, "gpx-file")
}

func activityCmd(args []string, client *icu.Client) error {
	if len(args) == 0 {
		return errMissing("activity id")
	}

	var a icu.Activity
	if err := client.Get("activity", []string{args[0]}, nil, &a); err != nil {
		return wrapCommandError(err)
	}

	return writeJSON(a)
}

func registerActivityCurve(registry *CommandRegistry, name, desc string) {
	registry.Register("activity", name, &Command{
		Name:        "",
		Usage:       "icu.Activity <id> " + name,
		Description: desc,
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			var v any
			if err := client.Get("activity", []string{args[0], name}, nil, &v); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(v)
		},
	})
}

func registerActivityDetail(registry *CommandRegistry, name, desc string, queryKeys ...string) {
	registry.Register("activity", name, &Command{
		Name:        "",
		Usage:       "icu.Activity <id> " + name,
		Description: desc,
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			q := queryFromFlags(flags, queryKeys...)

			var v any
			if err := client.Get("activity", []string{args[0], name}, q, &v); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(v)
		},
	})
}

func registerActivityDownload(registry *CommandRegistry, name string) {
	registry.Register("activity", name, &Command{
		Name:        "",
		Usage:       "icu.Activity <id> " + name,
		Description: "Download " + name + ".",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}

			data, err := client.Download("activity", []string{args[0], name}, nil)
			if err != nil {
				return wrapCommandError(err)
			}

			return writeOutput(data)
		},
	})
}
