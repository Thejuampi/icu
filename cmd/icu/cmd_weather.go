package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	icu "github.com/Thejuampi/icu"
)

const (
	weatherDefaultProvider = "OPEN_WEATHER"
	weatherMaxLatitude     = 90
	weatherMaxLongitude    = 180
)

var errWeatherConfig = errors.New("weather config")

func registerWeatherCommands(registry *CommandRegistry) {
	registry.Register("weather", "config", &Command{
		Name: "",
		Usage: "weather config [update [--id ID] --lat LAT --lon LON --label NAME --location NAME " +
			"--provider NAME --enabled true]",
		Description: "Get or update weather config.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			if weatherConfigHasUpdate(flags) {
				return updateWeatherConfig(flags, client)
			}

			return showWeatherConfig(client)
		},
	})

	registry.Register("weather", "forecast", listAllCommand[icu.WeatherDTO]("weather-forecast", "weather forecast",
		"Get weather forecast: configured locations with daily and hourly forecast data."))
}

func weatherConfigHasUpdate(flags map[string]string) bool {
	updateFlags := []string{"id", "lat", "lon", "label", "location", "provider", "enabled"}

	for _, key := range updateFlags {
		if _, ok := flags[key]; ok {
			return true
		}
	}

	return false
}

// updateWeatherConfig reads the full forecast list, edits one entry, and
// PUTs the whole list back so other configured locations are preserved.
func updateWeatherConfig(flags map[string]string, client *icu.Client) error {
	var cfg icu.WeatherConfig
	if err := client.Get("weather-config", nil, nil, &cfg); err != nil {
		return wrapCommandError(err)
	}

	updated, err := mergeWeatherConfig(cfg, flags)
	if err != nil {
		return err
	}

	var result icu.WeatherConfig
	if err := client.Put("weather-config", nil, nil, updated, &result); err != nil {
		return wrapCommandError(err)
	}
	if len(result.Forecasts) == 0 {
		return fmt.Errorf("%w: update returned no forecasts; check the config in Intervals.icu", errWeatherConfig)
	}

	return writeJSON(result)
}

// mergeWeatherConfig selects the entry to edit (by --id, or the only entry
// when there is exactly one), applies the passed flags, and returns the full
// list. An --id that does not exist, or an empty config, creates a new entry,
// which requires --lat and --lon and defaults to provider OPEN_WEATHER and
// enabled true.
func mergeWeatherConfig(cfg icu.WeatherConfig, flags map[string]string) (icu.WeatherConfig, error) {
	forecasts := append([]icu.Forecast(nil), cfg.Forecasts...)

	index, id, err := selectWeatherForecast(forecasts, flags)
	if err != nil {
		return icu.WeatherConfig{}, err
	}

	var entry icu.Forecast
	if index >= 0 {
		entry = forecasts[index]
	} else {
		if !hasFlagValue(flags, "lat") || !hasFlagValue(flags, "lon") {
			return icu.WeatherConfig{}, fmt.Errorf("%w: creating a forecast location requires --lat and --lon", errMissingRequired)
		}
		enabled := true
		entry = icu.Forecast{ID: id, Provider: weatherDefaultProvider, Enabled: &enabled}
	}

	if err := applyWeatherFlags(&entry, flags); err != nil {
		return icu.WeatherConfig{}, err
	}

	if index >= 0 {
		forecasts[index] = entry
	} else {
		forecasts = append(forecasts, entry)
	}

	return icu.WeatherConfig{Forecasts: forecasts}, nil
}

func selectWeatherForecast(forecasts []icu.Forecast, flags map[string]string) (int, int, error) {
	if raw, ok := flags["id"]; ok {
		id, err := strconv.Atoi(raw)
		if err != nil || id < 0 {
			return 0, 0, fmt.Errorf("%w: invalid --id %s", errInvalidValue, raw)
		}
		for index := range forecasts {
			if forecasts[index].ID == id {
				return index, id, nil
			}
		}

		return -1, id, nil
	}

	switch len(forecasts) {
	case 0:
		return -1, 0, nil
	case 1:
		return 0, forecasts[0].ID, nil
	default:
		return 0, 0, fmt.Errorf("%w: %d forecast locations configured; pass --id to choose one", errMissingRequired, len(forecasts))
	}
}

func applyWeatherFlags(entry *icu.Forecast, flags map[string]string) error {
	if v, ok := flags["label"]; ok {
		entry.Label = v
	}
	if v, ok := flags["location"]; ok {
		entry.Location = v
	}
	if v, ok := flags["provider"]; ok && v != "" {
		entry.Provider = v
	}
	if hasFlagValue(flags, "lat") {
		lat, err := weatherCoordinate(flags["lat"], "lat", weatherMaxLatitude)
		if err != nil {
			return err
		}
		entry.Lat = lat
	}
	if hasFlagValue(flags, "lon") {
		lon, err := weatherCoordinate(flags["lon"], "lon", weatherMaxLongitude)
		if err != nil {
			return err
		}
		entry.Lon = lon
	}
	if enabled := BoolPtrFlag(flags, "enabled"); enabled != nil {
		entry.Enabled = enabled
	}

	return nil
}

func weatherCoordinate(raw, name string, limit float64) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.Abs(value) > limit {
		return 0, fmt.Errorf("%w: invalid --%s %s", errInvalidValue, name, raw)
	}

	return value, nil
}

func hasFlagValue(flags map[string]string, name string) bool {
	v, ok := flags[name]

	return ok && v != ""
}

func showWeatherConfig(client *icu.Client) error {
	var cfg icu.WeatherConfig
	if err := client.Get("weather-config", nil, nil, &cfg); err != nil {
		return wrapCommandError(err)
	}

	return writeJSON(cfg)
}
