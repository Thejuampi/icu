package main

import icu "github.com/Thejuampi/icu"

func registerFTPCommands(registry *CommandRegistry) {
	registry.Register("ftp", "show", ftpShowCommand())
	registry.Register("ftp", "update", ftpUpdateCommand())
}

func ftpShowCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "ftp show [--sport Ride]",
		Description: "Show FTP for a sport type.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			sport := icu.StringFlag(flags, "sport", "Ride")

			var ss icu.SportSettings
			if err := client.Get("sport-settings", []string{sport}, nil, &ss); err != nil {
				return wrapCommandError(err)
			}

			type FTPInfo struct {
				Sport     string `json:"sport"`
				FTP       int    `json:"ftp"`
				IndoorFTP int    `json:"indoorFtp,omitempty"`
				LTHR      int    `json:"lthr,omitempty"`
			}

			return writeJSON(FTPInfo{
				Sport:     sport,
				FTP:       ss.FTP,
				IndoorFTP: ss.IndoorFTP,
				LTHR:      ss.LTHR,
			})
		},
	}
}

func ftpUpdateCommand() *Command {
	return &Command{
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
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	}
}
