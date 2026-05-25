package main

func init() {
	RegisterCommand("folders", "list", &Command{
		Usage:       "folders list",
		Description: "List folders, plans, and workouts.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var f []Folder
			if err := client.Get("folders", nil, nil, &f); err != nil {
				return err
			}
			return WriteJSON(osStdout(), f)
		},
	})

	RegisterCommand("folders", "create", &Command{
		Usage:       "folders create --name NAME [--type FOLDER|PLAN] [--desc DESC]",
		Description: "Create a folder or plan.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			f := FolderCreate{
				Name:        StringFlag(flags, "name", ""),
				Type:        StringFlag(flags, "type", "FOLDER"),
				Description: StringFlag(flags, "desc", ""),
			}
			var result Folder
			if err := client.Post("folders", nil, nil, f, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("folders", "update", &Command{
		Usage:       "folders update <id> --name NAME",
		Description: "Update a folder or plan.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("folder id")
			}
			f := FolderCreate{}
			if v := flags["name"]; v != "" {
				f.Name = v
			}
			if v := flags["desc"]; v != "" {
				f.Description = v
			}
			var result Folder
			if err := client.Put("folders", []string{args[0]}, nil, f, &result); err != nil {
				return err
			}
			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("folders", "delete", &Command{
		Usage:       "folders delete <id>",
		Description: "Delete a folder or plan.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("folder id")
			}
			return client.Delete("folders", []string{args[0]}, nil, nil)
		},
	})
}
