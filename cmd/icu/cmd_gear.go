package main

import icu "github.com/Thejuampi/icu"

func registerGearCommands(registry *CommandRegistry) {
	registry.Register("gear", "list", listAllCommand[[]icu.Gear]("gear", "gear list", "List all gear."))

	registry.Register("gear", "create", createCommand[icu.Gear, icu.Gear](
		"gear",
		"gear create --name NAME --type Bike [--distance M]",
		"Create gear or component.",
		nil,
		func(flags map[string]string) icu.Gear {
			return icu.Gear{
				Name:     icu.StringFlag(flags, "name", ""),
				Type:     icu.StringFlag(flags, "type", "Bike"),
				Distance: floatFlagVal(flags, "distance", 0),
			}
		},
	))

	registry.Register("gear", "update", updateByIDCommand[icu.Gear, icu.Gear](
		"gear",
		"gear update <id> --name NAME",
		"Update gear.",
		"gear id",
		nil,
		func(flags map[string]string) icu.Gear {
			var g icu.Gear
			if v := flags["name"]; v != "" {
				g.Name = v
			}

			if v := flags["distance"]; v != "" {
				g.Distance = floatFlagVal(flags, "distance", 0)
			}

			return g
		},
	))

	registry.Register("gear", "delete", deleteByIDCommand("gear", "gear delete <id>", "Delete gear.", "gear id"))
}
