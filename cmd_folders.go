package main

func init() {
	RegisterCommand("folders", "list", &Command{
		Name:        "",
		Usage:       "folders list",
		Description: "List folders, plans, and workouts.",
		Run: func(_ []string, _ map[string]string, client *Client) error {
			var f []Folder
			if err := client.Get("folders", nil, nil, &f); err != nil {
				return err
			}

			return WriteJSON(osStdout(), f)
		},
	})

	RegisterCommand("folders", "create", &Command{
		Name:        "",
		Usage:       "folders create --name NAME [--type FOLDER|PLAN] [--desc DESC]",
		Description: "Create a folder or plan.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			var f FolderCreate
			f.Name = StringFlag(flags, "name", "")
			f.Type = StringFlag(flags, "type", "FOLDER")
			f.Description = StringFlag(flags, "desc", "")

			var result Folder
			if err := client.Post("folders", nil, nil, f, &result); err != nil {
				return err
			}

			return WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("folders", "update", &Command{
		Name:        "",
		Usage:       "folders update <id> --name NAME",
		Description: "Update a folder or plan.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("folder id")
			}

			var f FolderCreate
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
		Name:        "",
		Usage:       "folders delete <id>",
		Description: "Delete a folder or plan.",
		Run: func(args []string, _ map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("folder id")
			}

			return client.Delete("folders", []string{args[0]}, nil, nil)
		},
	})
}
