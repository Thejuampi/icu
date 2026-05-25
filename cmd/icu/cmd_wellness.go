package main

import icu "github.com/Thejuampi/icu"

//nolint:gocognit,funlen
func init() {
	RegisterCommand("wellness", "list", &Command{
		Name:        "",
		Usage:       "wellness list --oldest DATE --newest DATE [--fields f1,f2]",
		Description: "List wellness records for a date range.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := queryFromFlags(flags, "oldest", "newest", "fields")

			var w []icu.Wellness
			if err := client.Get("wellness", nil, q, &w); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), w)
		},
	})

	RegisterCommand("wellness", "get", &Command{
		Name:        "",
		Usage:       "wellness get <date>",
		Description: "Get wellness record for a date.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("date (YYYY-MM-DD)")
			}

			var w icu.Wellness
			if err := client.Get("wellness", []string{args[0]}, nil, &w); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), w)
		},
	})

	RegisterCommand("wellness", "update", &Command{
		Name:        "",
		Usage:       "wellness update <date> --weight 81 --resting-hr 50 --hrv 72.5 [--sleep-secs 28800] [--locked]",
		Description: "Update wellness record for a date.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("date (YYYY-MM-DD)")
			}

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

			var result icu.Wellness
			if err := client.Put("wellness", []string{args[0]}, nil, w, &result); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("wellness", "bulk", &Command{
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

			return client.Put("wellness-bulk", nil, nil, records, nil)
		},
	})

	RegisterCommand("wellness", "upload", &Command{
		Name:        "",
		Usage:       "wellness upload <file.csv>",
		Description: "Upload wellness CSV file.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("csv file path")
			}

			var resp any

			return client.UploadFile("wellness", "", args[0], nil, &resp)
		},
	})
}
