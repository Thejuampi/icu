package main

import icu "github.com/Thejuampi/icu"

func init() {
	RegisterCommand("ftp", "show", &Command{
		Name:        "",
		Usage:       "ftp show [--sport Ride]",
		Description: "Show FTP for a sport type.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			sport := icu.StringFlag(flags, "sport", "Ride")

			var ss icu.SportSettings
			if err := client.Get("sport-settings", []string{sport}, nil, &ss); err != nil {
				return err
			}

			type FTPInfo struct {
				Sport     string `json:"sport"`
				FTP       int    `json:"ftp"`
				IndoorFTP int    `json:"indoorFtp,omitempty"`
				LTHR      int    `json:"lthr,omitempty"`
			}

			return icu.WriteJSON(osStdout(), FTPInfo{
				Sport:     sport,
				FTP:       ss.FTP,
				IndoorFTP: ss.IndoorFTP,
				LTHR:      ss.LTHR,
			})
		},
	})

	RegisterCommand("ftp", "update", &Command{
		Name:        "",
		Usage:       "ftp update --value WATTS [--sport Ride] [--indoor]",
		Description: "Update FTP for a sport.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			sport := icu.StringFlag(flags, "sport", "Ride")
			val := IntFlag(flags, "value", 0)
			if val <= 0 {
				return errMissing("--value (watts)")
			}

			var ss icu.SportSettings
			if BoolFlag(flags, "indoor") {
				ss.IndoorFTP = val
			} else {
				ss.FTP = val
			}

			var result icu.SportSettings
			if err := client.Put("sport-settings", []string{sport}, map[string]string{"recalcHrZones": "false"}, ss, &result); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), result)
		},
	})
}
