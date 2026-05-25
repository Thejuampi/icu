package main

import icu "github.com/Thejuampi/icu"

func init() {
	RegisterCommand("weather", "config", &Command{
		Name:        "",
		Usage:       "weather icu.Config [update --lat LAT --lon LON --label NAME --enabled true]",
		Description: "Get or update weather icu.Config.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			if _, ok := flags["lat"]; ok {
				var f icu.Forecast
				f.Label = icu.StringFlag(flags, "label", "")
				f.Lat = floatFlagVal(flags, "lat", 0)
				f.Lon = floatFlagVal(flags, "lon", 0)
				f.Enabled = BoolFlag(flags, "enabled")

				cfg := icu.WeatherConfig{Forecasts: []icu.Forecast{f}}

				var result icu.WeatherConfig
				if err := client.Put("weather-icu.Config", nil, nil, cfg, &result); err != nil {
					return err
				}

				return icu.WriteJSON(osStdout(), result)
			}

			var cfg icu.WeatherConfig
			if err := client.Get("weather-icu.Config", nil, nil, &cfg); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), cfg)
		},
	})

	RegisterCommand("weather", "icu.Forecast", &Command{
		Name:        "",
		Usage:       "weather icu.Forecast",
		Description: "Get weather icu.Forecast.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var w icu.WeatherDTO
			if err := client.Get("weather-icu.Forecast", nil, nil, &w); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), w)
		},
	})
}
