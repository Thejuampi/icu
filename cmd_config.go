package main

import "fmt"

func init() {
	RegisterCommand("config", "show", &Command{
		Usage:       "config show",
		Description: "Show current configuration.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			fmt.Printf("Config file: %s\n\n", configPath())
			if cfg.APIKey != "" {
				fmt.Printf("  api_key:    %s...%s\n", cfg.APIKey[:4], cfg.APIKey[len(cfg.APIKey)-4:])
			} else {
				fmt.Println("  api_key:    (not set)")
			}
			if cfg.AthleteID != "" {
				fmt.Printf("  athlete_id: %s\n", cfg.AthleteID)
			} else {
				fmt.Println("  athlete_id: 0 (default)")
			}
			if cfg.Output != "" {
				fmt.Printf("  output:     %s\n", cfg.Output)
			} else {
				fmt.Println("  output:     json (default)")
			}
			return nil
		},
	})

	RegisterCommand("config", "set", &Command{
		Usage:       "config set --api-key KEY [--athlete-id ID] [--output json|csv|table]",
		Description: "Set configuration values.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			cfg, _ := loadConfig()
			if cfg == nil {
				cfg = &Config{}
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
			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Printf("Config saved to %s\n", configPath())
			return nil
		},
	})

	RegisterCommand("config", "path", &Command{
		Usage:       "config path",
		Description: "Show config file path.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			fmt.Println(configPath())
			return nil
		},
	})
}
