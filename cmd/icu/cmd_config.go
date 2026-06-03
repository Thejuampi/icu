package main

import (
	"fmt"

	icu "github.com/Thejuampi/icu"
)

func registerConfigCommands(registry *CommandRegistry) {
	registry.Register("config", "show", configShowCommand())
	registry.Register("config", "set", configSetCommand())
	registry.Register("config", "path", configPathCommand())
	registry.Register("config", "diagnose", configDiagnoseCommand())
}

func configShowCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "config show",
		Description: "Show current configuration.",
		Run: func(_ []string, _ map[string]string, _ *icu.Client) error {
			out := osStdout()

			cfg, err := icu.LoadConfig()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			fmt.Fprintf(out, "config file: %s\n\n", icu.ConfigPath())
			if cfg.APIKey != "" {
				fmt.Fprintf(out, "  api_key:    %s...%s\n", cfg.APIKey[:4], cfg.APIKey[len(cfg.APIKey)-4:])
			} else {
				fmt.Fprintln(out, "  api_key:    (not set)")
			}

			if cfg.AthleteID != "" {
				fmt.Fprintf(out, "  athlete_id: %s\n", cfg.AthleteID)
			} else {
				fmt.Fprintln(out, "  athlete_id: 0 (default)")
			}

			if cfg.Output != "" {
				fmt.Fprintf(out, "  output:     %s\n", cfg.Output)
			} else {
				fmt.Fprintln(out, "  output:     json (default)")
			}

			return nil
		},
	}
}

func configSetCommand() *Command {
	return &Command{
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

			fmt.Fprintf(osStdout(), "config saved to %s\n", icu.ConfigPath())

			return nil
		},
	}
}

func configPathCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "config path",
		Description: "Show config file path.",
		Run: func(_ []string, _ map[string]string, _ *icu.Client) error {
			fmt.Fprintln(osStdout(), icu.ConfigPath())

			return nil
		},
	}
}

func configDiagnoseCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "config diagnose [--verbose]",
		Description: "Show non-secret diagnostics for auth and config resolution.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			diag := icu.DiagnoseConfig(flags)

			if !BoolFlag(flags, "verbose") {
				return writeJSON(configDiagnoseSafe(&diag))
			}

			return writeJSON(diag)
		},
	}
}

func configDiagnoseSafe(diag *icu.ConfigDiagnostic) map[string]any {
	return map[string]any{
		"configPath":   diag.ConfigPath,
		"configError":  diag.ConfigError,
		"apiKeySource": diag.APIKey.ResolvedSource,
		"athleteId": map[string]any{
			"config":         diag.AthleteID.Config,
			"default":        diag.AthleteID.Default,
			"resolved":       diag.AthleteID.Resolved,
			"resolvedSource": diag.AthleteID.ResolvedSource,
		},
		"output": map[string]any{
			"default":        diag.Output.Default,
			"resolved":       diag.Output.Resolved,
			"resolvedSource": diag.Output.ResolvedSource,
		},
	}
}
