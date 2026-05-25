package main

func init() {
	RegisterCommand("routes", "list", &Command{
		Usage:       "routes list",
		Description: "List routes with activity counts.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var r any
			if err := client.Get("routes", nil, nil, &r); err != nil {
				return err
			}
			return WriteJSON(osStdout(), r)
		},
	})

	RegisterCommand("routes", "get", &Command{
		Usage:       "routes get <id> [--include-path]",
		Description: "Get route by ID.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("route id")
			}
			q := map[string]string{}
			if BoolFlag(flags, "include-path") {
				q["includePath"] = "true"
			}
			var r Route
			if err := client.Get("routes", []string{args[0]}, q, &r); err != nil {
				return err
			}
			return WriteJSON(osStdout(), r)
		},
	})
}
