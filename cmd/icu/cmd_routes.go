package main

import icu "github.com/Thejuampi/icu"

func init() {
	RegisterCommand("routes", "list", &Command{
		Name:        "",
		Usage:       "routes list",
		Description: "List routes with icu.Activity counts.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var r any
			if err := client.Get("routes", nil, nil, &r); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), r)
		},
	})

	RegisterCommand("routes", "get", &Command{
		Name:        "",
		Usage:       "routes get <id> [--include-path]",
		Description: "Get icu.Route by ID.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Route id")
			}

			q := map[string]string{}
			if BoolFlag(flags, "include-path") {
				q["includePath"] = strTrue
			}

			var r icu.Route
			if err := client.Get("routes", []string{args[0]}, q, &r); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), r)
		},
	})
}
