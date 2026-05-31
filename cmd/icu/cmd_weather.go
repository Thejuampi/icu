package main

import icu "github.com/Thejuampi/icu"

func registerWeatherCommands(registry *CommandRegistry) {
	registry.Register("weather", "config", &Command{
		Name:        "",
		Usage:       "weather config [update --lat LAT --lon LON --label NAME --enabled true]",
		Description: "Get or update weather config.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			if _, ok := flags["lat"]; ok {
				var f icu.Forecast
				f.Label = icu.StringFlag(flags, "label", "")
				f.Lat = floatFlagVal(flags, "lat", 0)
				f.Lon = floatFlagVal(flags, "lon", 0)
				f.Enabled = BoolFlag(flags, "enabled")

				cfg := icu.WeatherConfig{Forecasts: []icu.Forecast{f}}

				var result icu.WeatherConfig
				if err := client.Put("weather-config", nil, nil, cfg, &result); err != nil {
					return wrapCommandError(err)
				}

				return writeJSON(result)
			}

			var cfg icu.WeatherConfig
			if err := client.Get("weather-config", nil, nil, &cfg); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(cfg)
		},
	})

	registry.Register("weather", "forecast", &Command{
		Name:        "",
		Usage:       "weather forecast",
		Description: "Get weather forecast.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var w icu.WeatherDTO
			if err := client.Get("weather-forecast", nil, nil, &w); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(w)
		},
	})
}
