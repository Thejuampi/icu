package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	icu "github.com/Thejuampi/icu"
)

func registerProfileCommands(registry *CommandRegistry) {
	registry.Register("profile", "show", profileShowCommand())
	registry.Register("profile", "set", profileSetCommand())
}

func profileShowCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "profile show",
		Description: "Show the local training profile (goals and caps). Local config only; nothing is sent to Intervals.icu.",
		Run: func(_ []string, _ map[string]string, _ *icu.Client) error {
			profile, err := loadTrainingProfile()
			if err != nil {
				return err
			}
			if profile == nil {
				profile = &icu.TrainingProfile{}
			}

			return writeJSON(profile)
		},
	}
}

func profileSetCommand() *Command {
	return &Command{
		Name: "",
		Usage: "profile set [--target-ftp W] [--outdoor-ftp W] [--indoor-ftp W] [--max-ramp CTL/WEEK] " +
			"[--min-z2-minutes N] [--z2-low PCT --z2-high PCT] [--usual-days mon,wed,...] [--usual-times HH:MM,...] " +
			"[--max-similar-sessions N] [--ctl-days N] [--atl-days N] [--thresholds JSON|-]",
		Description: "Update the local training profile. Pass an empty value (e.g. --max-ramp \"\") to keep, " +
			"or 0 / \"-\" to clear a field. Local config only; nothing is sent to Intervals.icu.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			cfg, err := icu.LoadConfig()
			if err != nil || cfg == nil {
				cfg = &icu.Config{}
			}
			profile := icu.TrainingProfile{}
			if cfg.Profile != nil {
				profile = *cfg.Profile
			}
			if err := applyProfileFlags(&profile, flags); err != nil {
				return err
			}
			if err := icu.ValidateTrainingProfile(&profile); err != nil {
				return fmt.Errorf("%w: %w", errInvalidValue, err)
			}
			cfg.Profile = &profile
			if err := icu.SaveConfigWithAction(cfg, "profile_set"); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			return writeJSON(profile)
		},
	}
}

func loadTrainingProfile() (*icu.TrainingProfile, error) {
	cfg, err := icu.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if cfg == nil {
		return nil, nil //nolint:nilnil // no profile configured is a valid state.
	}

	return cfg.Profile, nil
}

func applyProfileFlags(profile *icu.TrainingProfile, flags map[string]string) error {
	ints := map[string]*int{
		"target-ftp": &profile.TargetFTP, "outdoor-ftp": &profile.OutdoorFTP, "indoor-ftp": &profile.IndoorFTP,
		"min-z2-minutes": &profile.MinZ2MinutesPerWeek, "max-similar-sessions": &profile.MaxSimilarSessionsPerWeek,
		"ctl-days": &profile.CTLDays, "atl-days": &profile.ATLDays,
	}
	for name, target := range ints {
		if value, ok := profileFlag(flags, name); ok {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("%w: --%s %q", errInvalidValue, name, value)
			}
			*target = parsed
		}
	}
	floats := map[string]*float64{
		"max-ramp": &profile.MaxRampPerWeek, "z2-low": &profile.Z2LowPct, "z2-high": &profile.Z2HighPct,
	}
	for name, target := range floats {
		if value, ok := profileFlag(flags, name); ok {
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("%w: --%s %q", errInvalidValue, name, value)
			}
			*target = parsed
		}
	}
	if value, ok := profileFlag(flags, "usual-days"); ok {
		profile.UsualDays = splitProfileList(strings.ToLower(value))
	}
	if value, ok := profileFlag(flags, "usual-times"); ok {
		profile.UsualStartTimes = splitProfileList(value)
	}

	return applyThresholdsFlag(profile, flags)
}

// applyThresholdsFlag replaces the threshold overrides with a JSON object
// (e.g. '{"watchZ":2,"sleepWatch":72}'); "-" clears them.
func applyThresholdsFlag(profile *icu.TrainingProfile, flags map[string]string) error {
	value := flags["thresholds"]
	switch value {
	case "":
		return nil
	case "-":
		profile.Thresholds = nil

		return nil
	}
	var thresholds icu.TrainingThresholds
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&thresholds); err != nil {
		return fmt.Errorf("%w: --thresholds: %w", errInvalidValue, err)
	}
	profile.Thresholds = &thresholds

	return nil
}

// profileFlag returns a flag's value, mapping "-" to the zero value so a
// field can be cleared.
func profileFlag(flags map[string]string, name string) (string, bool) {
	value, ok := flags[name]
	if !ok || value == "" {
		return "", false
	}
	if value == "-" {
		if name == "usual-days" || name == "usual-times" {
			return "", true
		}

		return "0", true
	}

	return value, true
}

func splitProfileList(value string) []string {
	var out []string
	for part := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

// profileThresholds returns the profile's threshold overrides for analysis;
// without a readable profile every bound is derived from history.
func profileThresholds() *icu.TrainingThresholds {
	profile, err := loadTrainingProfile()
	if err != nil || profile == nil {
		return nil
	}

	return profile.Thresholds
}
