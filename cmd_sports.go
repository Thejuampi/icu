package main

func init() {
	RegisterCommand("sports", "list", &Command{
		Usage:       "sports list",
		Description: "List all sport settings.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var ss []SportSettings
			if err := client.Get("sport-settings", nil, nil, &ss); err != nil {
				return err
			}
			return WriteJSON(osStdout(), ss)
		},
	})

	RegisterCommand("sports", "get", &Command{
		Usage:       "sports get <type|id>",
		Description: "Get sport settings by type or ID.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("sport type or id")
			}
			var ss SportSettings
			if err := client.Get("sport-settings", []string{args[0]}, nil, &ss); err != nil {
				return err
			}
			return WriteJSON(osStdout(), ss)
		},
	})

	RegisterCommand("sports", "update", &Command{
		Usage:       "sports update <type|id> --ftp WATTS [--lthr BPM] [--max-hr BPM]",
		Description: "Update sport settings.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("sport type or id")
			}
			ss := SportSettings{}
			if v := IntFlag(flags, "ftp", -1); v >= 0 { ss.FTP = v }
			if v := IntFlag(flags, "lthr", -1); v >= 0 { ss.LTHR = v }
			if v := IntFlag(flags, "max-hr", -1); v >= 0 { ss.MaxHR = v }
			if v := IntFlag(flags, "indoor-ftp", -1); v >= 0 { ss.IndoorFTP = v }
			var result SportSettings
			if err := client.Put("sport-settings", []string{args[0]}, map[string]string{"recalcHrZones": "false"}, ss, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("sports", "delete", &Command{
		Usage:       "sports delete <id>",
		Description: "Delete sport settings.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("sport settings id")
			}
			return client.Delete("sport-settings", []string{args[0]}, nil, nil)
		},
	})
}
