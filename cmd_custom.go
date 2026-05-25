package main

func init() {
	RegisterCommand("custom", "list", &Command{
		Usage:       "custom list",
		Description: "List custom items.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var items []CustomItem
			if err := client.Get("custom-item", nil, nil, &items); err != nil {
				return err
			}
			return WriteJSON(osStdout(), items)
		},
	})

	RegisterCommand("custom", "get", &Command{
		Usage:       "custom get <id>",
		Description: "Get a custom item.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("item id")
			}
			var item CustomItem
			if err := client.Get("custom-item", []string{args[0]}, nil, &item); err != nil {
				return err
			}
			return WriteJSON(osStdout(), item)
		},
	})

	RegisterCommand("custom", "create", &Command{
		Usage:       "custom create --name NAME --type FITNESS_CHART|ACTIVITY_FIELD|...",
		Description: "Create a custom item.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			item := CustomItem{
				Name: StringFlag(flags, "name", ""),
				Type: StringFlag(flags, "type", ""),
			}
			var result CustomItem
			if err := client.Post("custom-item", nil, nil, item, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("custom", "update", &Command{
		Usage:       "custom update <id> --name NAME",
		Description: "Update a custom item.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("item id")
			}
			item := CustomItem{}
			if v := flags["name"]; v != "" { item.Name = v }
			if v := flags["type"]; v != "" { item.Type = v }
			var result CustomItem
			if err := client.Put("custom-item", []string{args[0]}, nil, item, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("custom", "delete", &Command{
		Usage:       "custom delete <id>",
		Description: "Delete a custom item.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("item id")
			}
			return client.Delete("custom-item", []string{args[0]}, nil, nil)
		},
	})
}
