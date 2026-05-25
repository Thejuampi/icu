package main

func init() {
	RegisterCommand("gear", "list", &Command{
		Usage:       "gear list",
		Description: "List all gear.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var g []Gear
			if err := client.Get("gear", nil, nil, &g); err != nil {
				return err
			}
			return WriteJSON(osStdout(), g)
		},
	})

	RegisterCommand("gear", "create", &Command{
		Usage:       "gear create --name NAME --type Bike [--distance M]",
		Description: "Create gear or component.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			g := Gear{
				Name:     StringFlag(flags, "name", ""),
				Type:     StringFlag(flags, "type", "Bike"),
				Distance: floatFlagVal(flags, "distance", 0),
			}
			var result Gear
			if err := client.Post("gear", nil, nil, g, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("gear", "update", &Command{
		Usage:       "gear update <id> --name NAME",
		Description: "Update gear.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("gear id")
			}
			g := Gear{}
			if v := flags["name"]; v != "" {
				g.Name = v
			}
			if v := flags["distance"]; v != "" {
				g.Distance = floatFlagVal(flags, "distance", 0)
			}
			var result Gear
			if err := client.Put("gear", []string{args[0]}, nil, g, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("gear", "delete", &Command{
		Usage:       "gear delete <id>",
		Description: "Delete gear.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("gear id")
			}
			return client.Delete("gear", []string{args[0]}, nil, nil)
		},
	})
}
