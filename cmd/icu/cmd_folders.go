package main

import icu "github.com/Thejuampi/icu"

func init() {
	RegisterCommand("folders", "list", &Command{
		Name:        "",
		Usage:       "folders list",
		Description: "List folders, plans, and workouts.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var f []icu.Folder
			if err := client.Get("folders", nil, nil, &f); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), f)
		},
	})

	RegisterCommand("folders", "create", &Command{
		Name:        "",
		Usage:       "folders create --name NAME [--type icu.Folder|PLAN] [--desc DESC]",
		Description: "Create a icu.Folder or plan.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			var f icu.FolderCreate
			f.Name = icu.StringFlag(flags, "name", "")
			f.Type = icu.StringFlag(flags, "type", "folder")
			f.Description = icu.StringFlag(flags, "desc", "")

			var result icu.Folder
			if err := client.Post("folders", nil, nil, f, &result); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("folders", "update", &Command{
		Name:        "",
		Usage:       "folders update <id> --name NAME",
		Description: "Update a icu.Folder or plan.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Folder id")
			}

			var f icu.FolderCreate
			if v := flags["name"]; v != "" {
				f.Name = v
			}

			if v := flags["desc"]; v != "" {
				f.Description = v
			}

			var result icu.Folder
			if err := client.Put("folders", []string{args[0]}, nil, f, &result); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), result)
		},
	})

	RegisterCommand("folders", "delete", &Command{
		Name:        "",
		Usage:       "folders delete <id>",
		Description: "Delete a icu.Folder or plan.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Folder id")
			}

			return client.Delete("folders", []string{args[0]}, nil, nil)
		},
	})
}
