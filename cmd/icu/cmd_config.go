package main

import (
	"fmt"
	"os"

	icu "github.com/Thejuampi/icu"
)

//nolint:gocognit
func init() {
	RegisterCommand("config", "show", &Command{
		Name:        "",
		Usage:       "config show",
		Description: "Show current configuration.",
		Run: func(_ []string, _ map[string]string, _ *icu.Client) error {
			cfg, err := icu.LoadConfig()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			fmt.Fprintf(os.Stdout, "config file: %s\n\n", icu.ConfigPath())
			if cfg.APIKey != "" {
				fmt.Fprintf(os.Stdout, "  api_key:    %s...%s\n", cfg.APIKey[:4], cfg.APIKey[len(cfg.APIKey)-4:])
			} else {
				fmt.Fprintln(os.Stdout, "  api_key:    (not set)")
			}

			if cfg.AthleteID != "" {
				fmt.Fprintf(os.Stdout, "  athlete_id: %s\n", cfg.AthleteID)
			} else {
				fmt.Fprintln(os.Stdout, "  athlete_id: 0 (default)")
			}

			if cfg.Output != "" {
				fmt.Fprintf(os.Stdout, "  output:     %s\n", cfg.Output)
			} else {
				fmt.Fprintln(os.Stdout, "  output:     json (default)")
			}

			return nil
		},
	})

	RegisterCommand("config", "set", &Command{
		Name:        "",
		Usage:       "config set --api-key KEY [--athlete-id ID] [--output json|csv|table]",
		Description: "Set configuration values.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			cfg, _ := icu.LoadConfig()
			if cfg == nil {
				var empty icu.Config
				cfg = &empty
			}

			if v, ok := flags["api-key"]; ok {
				cfg.APIKey = v
			}

			if v, ok := flags["athlete-id"]; ok {
				cfg.AthleteID = v
			}

			if v, ok := flags["output"]; ok {
				cfg.Output = v
			}

			if err := icu.SaveConfig(cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Fprintf(os.Stdout, "config saved to %s\n", icu.ConfigPath())

			return nil
		},
	})

	RegisterCommand("config", "path", &Command{
		Name:        "",
		Usage:       "config path",
		Description: "Show config file path.",
		Run: func(_ []string, _ map[string]string, _ *icu.Client) error {
			fmt.Fprintln(os.Stdout, icu.ConfigPath())

			return nil
		},
	})
}
