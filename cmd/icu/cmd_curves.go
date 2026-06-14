package main

import icu "github.com/Thejuampi/icu"

func registerCurvesCommands(registry *CommandRegistry) {
	registry.Register("curves", "power", &Command{
		Name:        "",
		Usage:       "curves power --type Ride [--curves 1y|42d|s0] [--filters FILTERS]",
		Description: "Get best power curves for the athlete.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := queryFromFlags(flags, "type", "curves", "newest", "filters")

			var v any
			if err := client.Get("power-curves", nil, q, &v); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(v)
		},
	})

	registry.Register("curves", "hr", &Command{
		Name:        "",
		Usage:       "curves hr --type Ride",
		Description: "Get best HR curves for the athlete.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := queryFromFlags(flags, "type", "curves")

			var v any
			if err := client.Get("hr-curves", nil, q, &v); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(v)
		},
	})

	registry.Register("curves", "pace", &Command{
		Name:        "",
		Usage:       "curves pace --type Run",
		Description: "Get best pace curves for the athlete.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := queryFromFlags(flags, "type", "curves")

			var v any
			if err := client.Get("pace-curves", nil, q, &v); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(v)
		},
	})

	registry.Register("curves", "power-hr", &Command{
		Name:        "",
		Usage:       "curves power-hr --start DATE --end DATE",
		Description: "Get power vs HR curve.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := queryFromFlags(flags, "start", "end")

			var v any
			if err := client.Get("power-hr-curve", nil, q, &v); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(v)
		},
	})

	registry.Register("curves", "mmp", &Command{
		Name:        "",
		Usage:       "curves mmp --type Ride",
		Description: "Get power model (MMP) for the athlete.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := queryFromFlags(flags, "type")

			var m icu.PowerModel
			if err := client.Get("mmp-model", nil, q, &m); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(m)
		},
	})
}
