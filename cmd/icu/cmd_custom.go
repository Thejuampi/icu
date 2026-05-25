package main

import icu "github.com/Thejuampi/icu"

func init() {
	RegisterCommand("custom", "list", &Command{
		Name:        "",
		Usage:       "custom list",
		Description: "List custom items.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var items []icu.CustomItem
			if err := client.Get("custom-item", nil, nil, &items); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), items)
		},
	})

	RegisterCommand("custom", "get", &Command{
		Name:        "",
		Usage:       "custom get <id>",
		Description: "Get a custom item.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("item id")
			}

			var item icu.CustomItem
			if err := client.Get("custom-item", []string{args[0]}, nil, &item); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), item)
		},
	})

	RegisterCommand("custom", "create", &Command{
		Name:        "",
		Usage:       "custom create --name NAME --type FITNESS_CHART|ACTIVITY_FIELD|...",
		Description: "Create a custom item.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			var item icu.CustomItem
			item.Name = icu.StringFlag(flags, "name", "")
			item.Type = icu.StringFlag(flags, "type", "")

			var result icu.CustomItem
			if err := client.Post("custom-item", nil, nil, item, &result); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("custom", "update", &Command{
		Name:        "",
		Usage:       "custom update <id> --name NAME",
		Description: "Update a custom item.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("item id")
			}

			var item icu.CustomItem
			if v := flags["name"]; v != "" {
				item.Name = v
			}

			if v := flags["type"]; v != "" {
				item.Type = v
			}

			var result icu.CustomItem
			if err := client.Put("custom-item", []string{args[0]}, nil, item, &result); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("custom", "delete", &Command{
		Name:        "",
		Usage:       "custom delete <id>",
		Description: "Delete a custom item.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("item id")
			}

			return client.Delete("custom-item", []string{args[0]}, nil, nil)
		},
	})
}
