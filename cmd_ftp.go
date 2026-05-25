package main

func init() {
	RegisterCommand("ftp", "show", &Command{
		Usage:       "ftp show [--sport Ride]",
		Description: "Show FTP for a sport type.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			sport := StringFlag(flags, "sport", "Ride")
			var ss SportSettings
			if err := client.Get("sport-settings", []string{sport}, nil, &ss); err != nil {
				return err
			}
			type FTPInfo struct {
				Sport     string `json:"sport"`
				FTP       int    `json:"ftp"`
				IndoorFTP int    `json:"indoor_ftp,omitempty"`
				LTHR      int    `json:"lthr,omitempty"`
			}
			return WriteJSON(osStdout(), FTPInfo{
				Sport:     sport,
				FTP:       ss.FTP,
				IndoorFTP: ss.IndoorFTP,
				LTHR:      ss.LTHR,
			})
		},
	})

	RegisterCommand("ftp", "update", &Command{
		Usage:       "ftp update --value WATTS [--sport Ride] [--indoor]",
		Description: "Update FTP for a sport.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			sport := StringFlag(flags, "sport", "Ride")
			val := IntFlag(flags, "value", 0)
			if val <= 0 {
				return errMissing("--value (watts)")
			}
			ss := SportSettings{}
			if BoolFlag(flags, "indoor") {
				ss.IndoorFTP = val
			} else {
				ss.FTP = val
			}
			var result SportSettings
			if err := client.Put("sport-settings", []string{sport}, map[string]string{"recalcHrZones": "false"}, ss, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})
}
