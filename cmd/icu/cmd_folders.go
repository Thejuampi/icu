package main

import icu "github.com/Thejuampi/icu"

func registerFoldersCommands(registry *CommandRegistry) {
	registry.Register("folders", "list", listAllCommand[[]icu.Folder]("folders", "folders list", "List folders, plans, and workouts."))

	registry.Register("folders", "create", createCommand[icu.FolderCreate, icu.Folder](
		"folders",
		"folders create --name NAME [--type icu.Folder|PLAN] [--desc DESC]",
		"Create a folder or plan.",
		nil,
		func(flags map[string]string) icu.FolderCreate {
			return icu.FolderCreate{
				Name:        icu.StringFlag(flags, "name", ""),
				Type:        icu.StringFlag(flags, "type", "folder"),
				Description: icu.StringFlag(flags, "desc", ""),
			}
		},
	))

	registry.Register("folders", "update", updateByIDCommand[icu.FolderCreate, icu.Folder](
		"folders",
		"folders update <id> --name NAME",
		"Update a folder or plan.",
		"icu.Folder id",
		nil,
		func(flags map[string]string) icu.FolderCreate {
			var f icu.FolderCreate
			if v := flags["name"]; v != "" {
				f.Name = v
			}

			if v := flags["desc"]; v != "" {
				f.Description = v
			}

			return f
		},
	))

	registry.Register("folders", "delete", deleteByIDCommand("folders", "folders delete <id>", "Delete a folder or plan.", "icu.Folder id"))
}
