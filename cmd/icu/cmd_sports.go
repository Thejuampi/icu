package main

import icu "github.com/Thejuampi/icu"

func registerSportsCommands(registry *CommandRegistry) {
	registry.Register("sports", "list", listAllCommand[[]icu.SportSettings]("sport-settings", "sports list", "List all sport settings."))
	registry.Register("sports", "get", getByIDCommand[icu.SportSettings]("sport-settings", "sports get <type|id>", "Get sport settings by type or ID.", "sport type or id", nil))

	registry.Register("sports", "update", updateByIDCommand[icu.SportSettings, icu.SportSettings](
		"sport-settings",
		"sports update <type|id> --ftp WATTS [--lthr BPM] [--max-hr BPM]",
		"Update sport settings.",
		"sport type or id",
		staticQuery(map[string]string{"recalcHrZones": "false"}),
		func(flags map[string]string) icu.SportSettings {
			var ss icu.SportSettings
			if v := IntFlag(flags, "ftp", -1); v >= 0 {
				ss.FTP = v
			}

			if v := IntFlag(flags, "lthr", -1); v >= 0 {
				ss.LTHR = v
			}

			if v := IntFlag(flags, "max-hr", -1); v >= 0 {
				ss.MaxHR = v
			}

			if v := IntFlag(flags, "indoor-ftp", -1); v >= 0 {
				ss.IndoorFTP = v
			}

			return ss
		},
	))

	registry.Register("sports", "delete", deleteByIDCommand("sport-settings", "sports delete <id>", "Delete sport settings.", "sport settings id"))
}
