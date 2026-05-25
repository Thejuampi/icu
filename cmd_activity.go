package main

import "fmt"

func errMissing(what string) error {
	return fmt.Errorf("missing required argument: %s", what)
}

func init() {
	RegisterCommand("activity", "show", &Command{
		Usage:       "activity <id> show [--intervals]",
		Description: "Get a single activity.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			return activityCmd(args, client)
		},
	})

	RegisterCommand("activity", "update", &Command{
		Usage:       "activity <id> update --name NAME [--desc DESC] [--type Ride]",
		Description: "Update an activity.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			a := Activity{}
			if v := flags["name"]; v != "" {
				a.Name = v
			}
			if v := flags["description"]; v != "" {
				a.Description = v
			}
			if v := flags["type"]; v != "" {
				a.Type = v
			}
			var result Activity
			if err := client.Put("activity", []string{args[0]}, nil, a, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("activity", "delete", &Command{
		Usage:       "activity <id> delete",
		Description: "Delete an activity.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			var resp DeleteResponse
			if err := client.Delete("activity", []string{args[0]}, nil, &resp); err != nil {
				return err
			}
			return WriteJSON(osStdout(), resp)
		},
	})

	RegisterCommand("activity", "intervals", &Command{
		Usage:       "activity <id> intervals",
		Description: "Get detected intervals.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			var dto IntervalsDTO
			if err := client.Get("activity", []string{args[0], "intervals"}, nil, &dto); err != nil {
				return err
			}
			return WriteJSON(osStdout(), dto)
		},
	})

	RegisterCommand("activity", "streams", &Command{
		Usage:       "activity <id> streams [--types watts,heartrate,cadence]",
		Description: "Get activity streams.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			q := queryFromFlags(flags, "types")
			var streams []ActivityStream
			if err := client.Get("activity", []string{args[0], "streams"}, q, &streams); err != nil {
				return err
			}
			return WriteJSON(osStdout(), streams)
		},
	})

	registerActivityCurve("power-curve", "Get activity power curve.")
	registerActivityCurve("hr-curve", "Get activity HR curve.")
	registerActivityCurve("pace-curve", "Get activity pace curve.")

	RegisterCommand("activity", "power-vs-hr", &Command{
		Usage:       "activity <id> power-vs-hr",
		Description: "Get power vs HR data.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			var v any
			if err := client.Get("activity", []string{args[0], "power-vs-hr"}, nil, &v); err != nil {
				return err
			}
			return WriteJSON(osStdout(), v)
		},
	})

	registerActivityDetail("best-efforts", "Find best efforts.", "stream", "duration", "distance", "count")
	registerActivityDetail("map", "Get map data.", "bounds", "weather")
	registerActivityDetail("weather-summary", "Get weather summary.", "descr_config")
	RegisterCommand("activity", "weather", &Command{
		Usage:       "activity <id> weather",
		Description: "Get weather summary (alias for weather-summary).",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			var v any
			if err := client.Get("activity", []string{args[0], "weather-summary"}, nil, &v); err != nil {
				return err
			}
			return WriteJSON(osStdout(), v)
		},
	})

	RegisterCommand("activity", "file", &Command{
		Usage:       "activity <id> file",
		Description: "Download original activity file.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			data, err := client.Download("activity", []string{args[0], "file"}, nil)
			if err != nil {
				return err
			}
			_, err = osStdout().Write(data)
			return err
		},
	})

	registerActivityDownload("fit-file")
	registerActivityDownload("gpx-file")
	registerActivityDetail("segments", "Get activity segments.")
	registerActivityDetail("messages", "Get activity comments.")
}

func activityCmd(args []string, client *Client) error {
	if len(args) == 0 {
		return errMissing("activity id")
	}
	var a Activity
	if err := client.Get("activity", []string{args[0]}, nil, &a); err != nil {
		return err
	}
	return WriteJSON(osStdout(), a)
}

func registerActivityCurve(name, desc string) {
	RegisterCommand("activity", name, &Command{
		Usage:       "activity <id> " + name,
		Description: desc,
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			var v any
			if err := client.Get("activity", []string{args[0], name}, nil, &v); err != nil {
				return err
			}
			return WriteJSON(osStdout(), v)
		},
	})
}

func registerActivityDetail(name, desc string, queryKeys ...string) {
	RegisterCommand("activity", name, &Command{
		Usage:       "activity <id> " + name,
		Description: desc,
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			q := queryFromFlags(flags, queryKeys...)
			var v any
			if err := client.Get("activity", []string{args[0], name}, q, &v); err != nil {
				return err
			}
			return WriteJSON(osStdout(), v)
		},
	})
}

func registerActivityDownload(name string) {
	RegisterCommand("activity", name, &Command{
		Usage:       "activity <id> " + name,
		Description: "Download " + name + ".",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			data, err := client.Download("activity", []string{args[0], name}, nil)
			if err != nil {
				return err
			}
			_, err = osStdout().Write(data)
			return err
		},
	})
}
