package main

import icu "github.com/Thejuampi/icu"

func registerWeatherCommands(registry *CommandRegistry) {
	registry.Register("weather", "config", &Command{
		Name:        "",
		Usage:       "weather config [update --lat LAT --lon LON --label NAME --location NAME --provider NAME --enabled true]",
		Description: "Get or update weather config.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			if weatherConfigHasUpdate(flags) {
				return updateWeatherConfig(flags, client)
			}

			return showWeatherConfig(client)
		},
	})

	registry.Register("weather", "forecast", listAllCommand[icu.WeatherDTO]("weather-forecast", "weather forecast", "Get weather forecast."))
}

func weatherConfigHasUpdate(flags map[string]string) bool {
	updateFlags := []string{"lat", "lon", "label", "location", "provider", "enabled"}

	for _, key := range updateFlags {
		if _, ok := flags[key]; ok {
			return true
		}
	}

	return false
}

func updateWeatherConfig(flags map[string]string, client *icu.Client) error {
	var cfg icu.WeatherConfig
	if err := client.Get("weather-config", nil, nil, &cfg); err != nil {
		return wrapCommandError(err)
	}

	f := weatherForecastFromFlags(flags, cfg)
	cfg = icu.WeatherConfig{Forecasts: []icu.Forecast{f}}

	var result icu.WeatherConfig
	if err := client.Put("weather-config", nil, nil, cfg, &result); err != nil {
		return wrapCommandError(err)
	}

	return writeJSON(result)
}

func weatherForecastFromFlags(flags map[string]string, cfg icu.WeatherConfig) icu.Forecast {
	var f icu.Forecast
	if len(cfg.Forecasts) > 0 {
		f = cfg.Forecasts[0]
	}

	if v, ok := flags["label"]; ok {
		f.Label = v
	}

	if v, ok := flags["location"]; ok {
		f.Location = v
	}

	if v, ok := flags["provider"]; ok {
		f.Provider = v
	}

	if v, ok := flags["lat"]; ok && v != "" {
		f.Lat = floatFlagVal(flags, "lat", 0)
	}

	if v, ok := flags["lon"]; ok && v != "" {
		f.Lon = floatFlagVal(flags, "lon", 0)
	}

	if enabled := BoolPtrFlag(flags, "enabled"); enabled != nil {
		f.Enabled = enabled
	}

	return f
}

func showWeatherConfig(client *icu.Client) error {
	var cfg icu.WeatherConfig
	if err := client.Get("weather-config", nil, nil, &cfg); err != nil {
		return wrapCommandError(err)
	}

	return writeJSON(cfg)
}
