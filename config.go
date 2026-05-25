package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey    string `json:"api_key,omitempty"`
	AthleteID string `json:"athlete_id,omitempty"`
	Output    string `json:"output,omitempty"`
}

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".icu"
	}
	return filepath.Join(home, ".icu")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func loadConfig() (*Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o600)
}

func ResolveAPIKey(flags map[string]string) string {
	if key, ok := flags["api-key"]; ok && key != "" {
		return key
	}
	if key := os.Getenv("INTERVALS_ICU_API_KEY"); key != "" {
		return key
	}
	cfg, _ := loadConfig()
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
	cfg, _ := loadConfig()
	if cfg != nil && cfg.AthleteID != "" {
		return cfg.AthleteID
	}
	return "0"
}

func ResolveOutputFormat(flags map[string]string) OutputFormat {
	output := StringFlag(flags, "output", "")
	if output == "" {
		cfg, _ := loadConfig()
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
