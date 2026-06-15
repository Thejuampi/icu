package main

import icu "github.com/Thejuampi/icu"

func registerWellnessCommands(registry *CommandRegistry) {
	registry.Register(
		"wellness", "list",
		listQueryCommand[[]icu.Wellness](
			"wellness",
			"wellness list --oldest DATE --newest DATE [--fields f1,f2]",
			"List wellness records for a date range.",
			queryBuilder("oldest", "newest", "fields"),
		),
	)
	registry.Register("wellness", "get", getByIDCommand[icu.Wellness]("wellness", "wellness get <date>", "Get wellness record for a date.", "date (YYYY-MM-DD)", nil))

	registry.Register(
		"wellness", "update",
		updateByIDCommand[icu.Wellness, icu.Wellness](
			"wellness",
			"wellness update <date> --weight 81 --resting-hr 50 --hrv 72.5 [--sleep-secs 28800] [--locked]",
			"Update wellness record for a date.",
			"date (YYYY-MM-DD)",
			nil,
			func(flags map[string]string) icu.Wellness {
				var w icu.Wellness
				if _, ok := flags["weight"]; ok {
					w.Weight = floatFlagVal(flags, "weight", 0)
				}

				w.RestingHR = IntFlag(flags, "resting-hr", -999)
				if w.RestingHR == -999 {
					w.RestingHR = 0
				}

				if _, ok := flags["hrv"]; ok {
					w.HRV = floatFlagVal(flags, "hrv", 0)
				}

				w.SleepSecs = IntFlag(flags, "sleep-secs", -1)
				if w.SleepSecs == -1 {
					w.SleepSecs = 0
				}

				w.SleepScore = floatFlagVal(flags, "sleep-score", -1)
				if w.SleepScore == -1 {
					w.SleepScore = 0
				}

				w.Locked = BoolFlag(flags, "locked")

				return w
			},
		),
	)

	registry.Register("wellness", "bulk", wellnessBulkCommand())
	registry.Register("wellness", "upload", wellnessUploadCommand())
}

func wellnessBulkCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "wellness bulk --file FILE.json",
		Description: "Bulk update wellness records from a JSON array file.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			fpath := icu.StringFlag(flags, "file", "")
			if fpath == "" {
				return errMissing("--file")
			}

			data, err := osReadFile(fpath)
			if err != nil {
				return err
			}

			var records []icu.Wellness
			if err := jsonUnmarshal(data, &records); err != nil {
				return err
			}

			return wrapCommandError(client.Put("wellness-bulk", nil, nil, records, nil))
		},
	}
}

func wellnessUploadCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "wellness upload <file.csv>",
		Description: "Upload wellness CSV file.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("csv file path")
			}

			var resp any

			return wrapCommandError(client.UploadFile("wellness", "", args[0], nil, &resp))
		},
	}
}
