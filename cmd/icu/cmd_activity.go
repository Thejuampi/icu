package main

import icu "github.com/Thejuampi/icu"

func registerActivityCommands(registry *CommandRegistry) {
	registry.Register(
		"activity", "show",
		getByIDCommand[icu.Activity]("activity", "activity <id> show [--intervals]", "Get a single activity.", "activity id", nil),
	)
	registry.Register(
		"activity", "update",
		updateByIDCommand[icu.Activity, icu.Activity](
			"activity",
			"activity <id> update --name NAME [--description DESC] [--type Ride]",
			"Update an activity.",
			"activity id",
			nil,
			func(flags map[string]string) icu.Activity {
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

				return a
			},
		),
	)
	registry.Register(
		"activity", "delete",
		deleteByIDWithResponseCommand[icu.DeleteResponse]("activity", "activity <id> delete", "Delete an activity.", "activity id"),
	)
	registry.Register(
		"activity", "intervals",
		getByIDCommand[icu.IntervalsDTO]("activity", "activity <id> intervals", "Get detected intervals.", "activity id", nil, "intervals"),
	)
	registry.Register(
		"activity", "streams",
		getByIDCommand[[]icu.ActivityStream](
			"activity",
			"activity <id> streams [--types watts,heartrate,cadence]",
			"Get activity streams.",
			"activity id",
			queryBuilder("types"),
			"streams",
		),
	)
	registry.Register(
		"activity", "power-vs-hr",
		getByIDCommand[any]("activity", "activity <id> power-vs-hr", "Get power vs HR data.", "activity id", nil, "power-vs-hr"),
	)
	registry.Register(
		"activity", "weather",
		getByIDCommand[any]("activity", "activity <id> weather", "Get weather summary (alias for weather-summary).", "activity id", nil, "weather-summary"),
	)
	registry.Register(
		"activity", "file",
		downloadByIDCommand("activity", "activity <id> file", "Download original activity file.", "activity id", "file"),
	)
	registerActivityCurveCommands(registry)
	registerActivityDetailCommands(registry)
	registerActivityDownloadCommands(registry)
}

func registerActivityCurveCommands(registry *CommandRegistry) {
	registerActivityCurve(registry, "power-curve", "Get activity power curve.")
	registerActivityCurve(registry, "hr-curve", "Get activity HR curve.")
	registerActivityCurve(registry, "pace-curve", "Get activity pace curve.")
}

func registerActivityDetailCommands(registry *CommandRegistry) {
	registerActivityDetail(registry, "best-efforts", "Find best efforts.", "stream", "duration", "distance", "count")
	registerActivityDetail(registry, "map", "Get map data.", "bounds", "weather")
	registerActivityDetail(registry, "weather-summary", "Get weather summary.", "descr_config")
	registerActivityDetail(registry, "segments", "Get activity segments.")
	registerActivityDetail(registry, "messages", "Get activity comments.")
}

func registerActivityDownloadCommands(registry *CommandRegistry) {
	registerActivityDownload(registry, "fit-file")
	registerActivityDownload(registry, "gpx-file")
}

func registerActivityCurve(registry *CommandRegistry, name, desc string) {
	registry.Register("activity", name, getByIDCommand[any]("activity", "icu.Activity <id> "+name, desc, "activity id", nil, name))
}

func registerActivityDetail(registry *CommandRegistry, name, desc string, queryKeys ...string) {
	registry.Register("activity", name, getByIDCommand[any]("activity", "icu.Activity <id> "+name, desc, "activity id", queryBuilder(queryKeys...), name))
}

func registerActivityDownload(registry *CommandRegistry, name string) {
	registry.Register("activity", name, downloadByIDCommand("activity", "icu.Activity <id> "+name, "Download "+name+".", "activity id", name))
}
