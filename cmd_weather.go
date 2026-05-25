package main

func init() {
	RegisterCommand("weather", "config", &Command{
		Name:        "",
		Usage:       "weather config [update --lat LAT --lon LON --label NAME --enabled true]",
		Description: "Get or update weather config.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			if _, ok := flags["lat"]; ok {
				var f Forecast
				f.Label = StringFlag(flags, "label", "")
				f.Lat = floatFlagVal(flags, "lat", 0)
				f.Lon = floatFlagVal(flags, "lon", 0)
				f.Enabled = BoolFlag(flags, "enabled")

				cfg := WeatherConfig{Forecasts: []Forecast{f}}

				var result WeatherConfig
				if err := client.Put("weather-config", nil, nil, cfg, &result); err != nil {
					return err
				}

				return WriteJSON(osStdout(), result)
			}

			var cfg WeatherConfig
			if err := client.Get("weather-config", nil, nil, &cfg); err != nil {
				return err
			}

			return WriteJSON(osStdout(), cfg)
		},
	})

	RegisterCommand("weather", "forecast", &Command{
		Name:        "",
		Usage:       "weather forecast",
		Description: "Get weather forecast.",
		Run: func(_ []string, _ map[string]string, client *Client) error {
			var w WeatherDTO
			if err := client.Get("weather-forecast", nil, nil, &w); err != nil {
				return err
			}

			return WriteJSON(osStdout(), w)
		},
	})
}
