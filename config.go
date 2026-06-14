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
	if key, ok := flags["api-key"]; ok && key != "" {
		return key
	}

	if key := os.Getenv("INTERVALS_ICU_API_KEY"); key != "" {
		return key
	}

	cfg, _ := LoadConfig()
	if cfg != nil && cfg.APIKey != "" {
		return cfg.APIKey
	}

	return ""
}

func ResolveAthleteID(flags map[string]string) string {
	if id, ok := flags["athlete-id"]; ok && id != "" {
		return id
	}

	if id := os.Getenv("INTERVALS_ICU_ATHLETE_ID"); id != "" {
		return id
	}

	cfg, _ := LoadConfig()
	if cfg != nil && cfg.AthleteID != "" {
		return cfg.AthleteID
	}

	return "0"
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
	if token, ok := flags["zepp-login-token"]; ok && token != "" {
		return token
	}

	if token := os.Getenv("ZEPP_LOGIN_TOKEN"); token != "" {
		return token
	}

	cfg, _ := LoadConfig()
	if cfg != nil && cfg.ZeppLoginToken != "" {
		return cfg.ZeppLoginToken
	}

	return ""
}

func ResolveZeppAppToken(flags map[string]string) string {
	if token, ok := flags["zepp-app-token"]; ok && token != "" {
		return token
	}

	if token := os.Getenv("ZEPP_APP_TOKEN"); token != "" {
		return token
	}

	cfg, _ := LoadConfig()
	if cfg != nil && cfg.ZeppAppToken != "" {
		return cfg.ZeppAppToken
	}

	return ""
}

func ResolveZeppUserID(flags map[string]string) string {
	if id, ok := flags["zepp-user-id"]; ok && id != "" {
		return id
	}

	if id := os.Getenv("ZEPP_USER_ID"); id != "" {
		return id
	}

	cfg, _ := LoadConfig()
	if cfg != nil && cfg.ZeppUserID != "" {
		return cfg.ZeppUserID
	}

	return ""
}

func ResolveZeppCountryCode(flags map[string]string) string {
	if code, ok := flags["zepp-country-code"]; ok && code != "" {
		return code
	}

	if code := os.Getenv("ZEPP_COUNTRY_CODE"); code != "" {
		return code
	}

	cfg, _ := LoadConfig()
	if cfg != nil && cfg.ZeppCountryCode != "" {
		return cfg.ZeppCountryCode
	}

	return ""
}
