package main

func init() {
	RegisterCommand("curves", "power", &Command{
		Name:        "",
		Usage:       "curves power --type Ride [--curves 1y|42d|s0] [--filters FILTERS]",
		Description: "Get best power curves for athlete.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "type", "curves", "newest", "filters")

			var v any
			if err := client.Get("power-curves", nil, q, &v); err != nil {
				return err
			}

			return WriteJSON(osStdout(), v)
		},
	})

	RegisterCommand("curves", "hr", &Command{
		Name:        "",
		Usage:       "curves hr --type Ride",
		Description: "Get best HR curves for athlete.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "type", "curves")

			var v any
			if err := client.Get("hr-curves", nil, q, &v); err != nil {
				return err
			}

			return WriteJSON(osStdout(), v)
		},
	})

	RegisterCommand("curves", "pace", &Command{
		Name:        "",
		Usage:       "curves pace --type Run",
		Description: "Get best pace curves for athlete.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "type", "curves")

			var v any
			if err := client.Get("pace-curves", nil, q, &v); err != nil {
				return err
			}

			return WriteJSON(osStdout(), v)
		},
	})

	RegisterCommand("curves", "power-hr", &Command{
		Name:        "",
		Usage:       "curves power-hr --start DATE --end DATE",
		Description: "Get power vs HR curve.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "start", "end")

			var v any
			if err := client.Get("power-hr-curve", nil, q, &v); err != nil {
				return err
			}

			return WriteJSON(osStdout(), v)
		},
	})

	RegisterCommand("curves", "mmp", &Command{
		Name:        "",
		Usage:       "curves mmp --type Ride",
		Description: "Get power model (MMP) for athlete.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "type")

			var m PowerModel
			if err := client.Get("mmp-model", nil, q, &m); err != nil {
				return err
			}

			return WriteJSON(osStdout(), m)
		},
	})
}
