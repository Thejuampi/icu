package main

import icu "github.com/Thejuampi/icu"

func registerGearCommands(registry *CommandRegistry) {
	registry.Register("gear", "list", &Command{
		Name:        "",
		Usage:       "gear list",
		Description: "List all gear.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var g []icu.Gear
			if err := client.Get("gear", nil, nil, &g); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(g)
		},
	})

	registry.Register("gear", "create", &Command{
		Name:        "",
		Usage:       "gear create --name NAME --type Bike [--distance M]",
		Description: "Create gear or component.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			var g icu.Gear
			g.Name = icu.StringFlag(flags, "name", "")
			g.Type = icu.StringFlag(flags, "type", "Bike")
			g.Distance = floatFlagVal(flags, "distance", 0)

			var result icu.Gear
			if err := client.Post("gear", nil, nil, g, &result); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	})

	registry.Register("gear", "update", &Command{
		Name:        "",
		Usage:       "gear update <id> --name NAME",
		Description: "Update gear.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("gear id")
			}

			var g icu.Gear
			if v := flags["name"]; v != "" {
				g.Name = v
			}

			if v := flags["distance"]; v != "" {
				g.Distance = floatFlagVal(flags, "distance", 0)
			}

			var result icu.Gear
			if err := client.Put("gear", []string{args[0]}, nil, g, &result); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	})

	registry.Register("gear", "delete", &Command{
		Name:        "",
		Usage:       "gear delete <id>",
		Description: "Delete gear.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("gear id")
			}

			return wrapCommandError(client.Delete("gear", []string{args[0]}, nil, nil))
		},
	})
}
