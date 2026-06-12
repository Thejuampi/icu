package icu

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

const fingerprintLength = 12

type ConfigDiagnostic struct {
	ConfigPath     string                `json:"configPath"`
	ConfigError    string                `json:"configError,omitempty"`
	APIKey         APIKeyDiagnostic      `json:"apiKey"`
	AthleteID      ConfigValueDiagnostic `json:"athleteId"`
	Output         ConfigValueDiagnostic `json:"output"`
	ZeppLoginToken APIKeyDiagnostic      `json:"zeppLoginToken"`
}

type APIKeyDiagnostic struct {
	Flag           SecretDiagnostic `json:"flag"`
	Env            SecretDiagnostic `json:"env"`
	Config         SecretDiagnostic `json:"config"`
	Resolved       SecretDiagnostic `json:"resolved"`
	ResolvedSource string           `json:"resolvedSource"`
}

type SecretDiagnostic struct {
	Set                   bool   `json:"set"`
	Length                int    `json:"length"`
	TrimLength            int    `json:"trimLength"`
	StartsOrEndsWithSpace bool   `json:"startsOrEndsWithSpace"`
	Fingerprint           string `json:"fingerprint,omitempty"`
}

type ConfigValueDiagnostic struct {
	Flag           string `json:"flag,omitempty"`
	Env            string `json:"env,omitempty"`
	Config         string `json:"config,omitempty"`
	Default        string `json:"default,omitempty"`
	Resolved       string `json:"resolved"`
	ResolvedSource string `json:"resolvedSource"`
}

func DiagnoseConfig(flags map[string]string) ConfigDiagnostic {
	cfg, err := LoadConfig()

	var configError string

	if err != nil {
		configError = err.Error()
	}

	return BuildConfigDiagnostic(
		flags,
		os.Getenv("INTERVALS_ICU_API_KEY"),
		os.Getenv("INTERVALS_ICU_ATHLETE_ID"),
		os.Getenv("ZEPP_LOGIN_TOKEN"),
		cfg,
		ConfigPath(),
		configError,
	)
}

func BuildConfigDiagnostic(
	flags map[string]string,
	envAPIKey string,
	envAthleteID string,
	envZeppLoginToken string,
	cfg *Config,
	configPath string,
	configError string,
) ConfigDiagnostic {
	if cfg == nil {
		cfg = &Config{APIKey: "", AthleteID: "", Output: ""}
	}

	return ConfigDiagnostic{
		ConfigPath:  configPath,
		ConfigError: configError,
		APIKey:      buildAPIKeyDiagnostic(flags, envAPIKey, cfg.APIKey),
		AthleteID: buildValueDiagnostic(
			StringFlag(flags, "athlete-id", ""),
			envAthleteID,
			cfg.AthleteID,
			"0",
		),
		Output: buildValueDiagnostic(
			StringFlag(flags, "output", ""),
			"",
			cfg.Output,
			"json",
		),
		ZeppLoginToken: buildAPIKeyDiagnostic(flags, envZeppLoginToken, cfg.ZeppLoginToken),
	}
}

func buildAPIKeyDiagnostic(flags map[string]string, envAPIKey, configAPIKey string) APIKeyDiagnostic {
	flagAPIKey := StringFlag(flags, "api-key", "")
	resolved := firstNonEmpty(flagAPIKey, envAPIKey, configAPIKey)

	return APIKeyDiagnostic{
		Flag:     DiagnoseSecret(flagAPIKey),
		Env:      DiagnoseSecret(envAPIKey),
		Config:   DiagnoseSecret(configAPIKey),
		Resolved: DiagnoseSecret(resolved),
		ResolvedSource: resolvedSource([]sourceValue{
			{source: "flag", value: flagAPIKey},
			{source: "env", value: envAPIKey},
			{source: "config", value: configAPIKey},
		}),
	}
}

func DiagnoseSecret(value string) SecretDiagnostic {
	trimmed := trimSpace(value)

	return SecretDiagnostic{
		Set:                   value != "",
		Length:                len(value),
		TrimLength:            len(trimmed),
		StartsOrEndsWithSpace: value != trimmed,
		Fingerprint:           secretFingerprint(value),
	}
}

func buildValueDiagnostic(flagValue, envValue, configValue, defaultValue string) ConfigValueDiagnostic {
	resolved := firstNonEmpty(flagValue, envValue, configValue, defaultValue)

	return ConfigValueDiagnostic{
		Flag:     flagValue,
		Env:      envValue,
		Config:   configValue,
		Default:  defaultValue,
		Resolved: resolved,
		ResolvedSource: resolvedSource([]sourceValue{
			{source: "flag", value: flagValue},
			{source: "env", value: envValue},
			{source: "config", value: configValue},
			{source: "default", value: defaultValue},
		}),
	}
}

type sourceValue struct {
	source string
	value  string
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func resolvedSource(values []sourceValue) string {
	for _, value := range values {
		if value.value != "" {
			return value.source
		}
	}

	return "none"
}

func secretFingerprint(value string) string {
	if value == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(value))
	encoded := hex.EncodeToString(sum[:])

	return encoded[:fingerprintLength]
}

func SecretFingerprint(value string) string {
	return secretFingerprint(value)
}

func trimSpace(value string) string {
	start := 0
	end := len(value)

	for start < end && isASCIIWhitespace(value[start]) {
		start++
	}

	for end > start && isASCIIWhitespace(value[end-1]) {
		end--
	}

	return value[start:end]
}

func isASCIIWhitespace(value byte) bool {
	switch value {
	case ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}
