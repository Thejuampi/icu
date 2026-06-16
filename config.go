package icu

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey          string `json:"apiKey,omitempty"`
	AthleteID       string `json:"athleteId,omitempty"`
	Output          string `json:"output,omitempty"`
	ZeppLoginToken  string `json:"zeppLoginToken,omitempty"`
	ZeppAppToken    string `json:"zeppAppToken,omitempty"`
	ZeppUserID      string `json:"zeppUserId,omitempty"`
	ZeppCountryCode string `json:"zeppCountryCode,omitempty"`
}

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".icu"
	}

	return filepath.Join(home, ".icu")
}

func ConfigPath() string {
	return filepath.Join(configDir(), "config.json")
}

func LoadConfig() (*Config, error) {
	path := ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			var cfg Config

			return &cfg, nil
		}

		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if len(data) == 0 {
		return &cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("cannot save nil config")
	}

	dir := configDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(ConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

func StringFlag(args map[string]string, name, defaultVal string) string {
	if v, ok := args[name]; ok && v != "" {
		return v
	}

	return defaultVal
}

func ResolveAPIKey(flags map[string]string) string {
	return resolveString(flags, "api-key", "INTERVALS_ICU_API_KEY", "", func(c *Config) string { return c.APIKey })
}

func ResolveAthleteID(flags map[string]string) string {
	return resolveString(flags, "athlete-id", "INTERVALS_ICU_ATHLETE_ID", "0", func(c *Config) string { return c.AthleteID })
}

func ResolveOutputFormat(flags map[string]string) OutputFormat {
	output := StringFlag(flags, "output", "")
	if output == "" {
		cfg, _ := LoadConfig()
		if cfg != nil && cfg.Output != "" {
			output = cfg.Output
		}
	}

	switch output {
	case "csv":
		return FormatCSV
	case "table":
		return FormatTable
	default:
		return FormatJSON
	}
}

func ResolveZeppLoginToken(flags map[string]string) string {
	return resolveString(flags, "zepp-login-token", "ZEPP_LOGIN_TOKEN", "", func(c *Config) string { return c.ZeppLoginToken })
}

func ResolveZeppAppToken(flags map[string]string) string {
	return resolveString(flags, "zepp-app-token", "ZEPP_APP_TOKEN", "", func(c *Config) string { return c.ZeppAppToken })
}

func ResolveZeppUserID(flags map[string]string) string {
	return resolveString(flags, "zepp-user-id", "ZEPP_USER_ID", "", func(c *Config) string { return c.ZeppUserID })
}

func ResolveZeppCountryCode(flags map[string]string) string {
	return resolveString(flags, "zepp-country-code", "ZEPP_COUNTRY_CODE", "", func(c *Config) string { return c.ZeppCountryCode })
}

func resolveString(flags map[string]string, flagName, envName, defaultVal string, configVal func(*Config) string) string {
	if v, ok := flags[flagName]; ok && v != "" {
		return v
	}

	if v := os.Getenv(envName); v != "" {
		return v
	}

	cfg, _ := LoadConfig()
	if cfg != nil {
		if v := configVal(cfg); v != "" {
			return v
		}
	}

	return defaultVal
}
