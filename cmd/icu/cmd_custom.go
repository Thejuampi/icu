package main

import icu "github.com/Thejuampi/icu"

func registerCustomCommands(registry *CommandRegistry) {
	registry.Register("custom", "list", listAllCommand[[]icu.CustomItem]("custom-item", "custom list", "List custom items."))
	registry.Register("custom", "get", getByIDCommand[icu.CustomItem]("custom-item", "custom get <id>", "Get a custom item.", "item id", nil))

	registry.Register("custom", "create", createCommand[icu.CustomItem, icu.CustomItem](
		"custom-item",
		"custom create --name NAME --type FITNESS_CHART|ACTIVITY_FIELD|...",
		"Create a custom item.",
		nil,
		func(flags map[string]string) icu.CustomItem {
			return icu.CustomItem{
				Name: icu.StringFlag(flags, "name", ""),
				Type: icu.StringFlag(flags, "type", ""),
			}
		},
	))

	registry.Register("custom", "update", updateByIDCommand[icu.CustomItem, icu.CustomItem](
		"custom-item",
		"custom update <id> --name NAME",
		"Update a custom item.",
		"item id",
		nil,
		func(flags map[string]string) icu.CustomItem {
			var item icu.CustomItem
			if v := flags["name"]; v != "" {
				item.Name = v
			}

			if v := flags["type"]; v != "" {
				item.Type = v
			}

			return item
		},
	))

	registry.Register("custom", "delete", deleteByIDCommand("custom-item", "custom delete <id>", "Delete a custom item.", "item id"))
}
