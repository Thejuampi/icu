package icu

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResolveOutdoorWeatherUsesMockOpenMeteo(t *testing.T) {
	// Not parallel: mutates package Open-Meteo endpoint URLs for httptest.
	start := time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"hourly": map[string]any{
				"time":               []int64{start.Unix(), start.Unix() + 3600},
				"temperature_2m":     []float64{22.0, 23.0},
				"surface_pressure":   []float64{1012.0, 1011.0},
				"wind_speed_10m":     []float64{4.0, 5.0},
				"wind_direction_10m": []float64{45.0, 50.0},
			},
		}
		_ = json.NewEncoder(writer).Encode(payload)
	}))
	t.Cleanup(server.Close)

	prevA, prevF, prevH := openMeteoArchiveURL, openMeteoForecastURL, openMeteoHistoricalForecastURL
	openMeteoArchiveURL = server.URL
	openMeteoForecastURL = server.URL
	openMeteoHistoricalForecastURL = server.URL
	t.Cleanup(func() {
		openMeteoArchiveURL = prevA
		openMeteoForecastURL = prevF
		openMeteoHistoricalForecastURL = prevH
	})

	got := ResolveOutdoorWeather(server.Client(), OutdoorWeatherQuery{
		Lat:         40.75,
		Lon:         -73.94,
		StartUTC:    start,
		DurationSec: 3600,
	})
	if !got.OK {
		t.Fatalf("expected OK: %+v", got)
	}
	if len(got.Hours) == 0 {
		t.Fatal("expected hourly samples")
	}
	if got.MeanWindSpeedMS < 3 {
		t.Fatalf("wind=%v", got.MeanWindSpeedMS)
	}
	if got.MeanRho <= 0 {
		t.Fatalf("rho=%v", got.MeanRho)
	}
}

func TestAttachHourlyWithActivityWind(t *testing.T) {
	// Not parallel: mutates package Open-Meteo endpoint URLs for httptest.
	start := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"hourly": map[string]any{
				"time":               []int64{start.Unix()},
				"temperature_2m":     []float64{19.0},
				"surface_pressure":   []float64{1008.0},
				"wind_speed_10m":     []float64{2.5},
				"wind_direction_10m": []float64{200.0},
			},
		}
		_ = json.NewEncoder(writer).Encode(payload)
	}))
	t.Cleanup(server.Close)

	prevA, prevF, prevH := openMeteoArchiveURL, openMeteoForecastURL, openMeteoHistoricalForecastURL
	openMeteoArchiveURL = server.URL
	openMeteoForecastURL = server.URL
	openMeteoHistoricalForecastURL = server.URL
	t.Cleanup(func() {
		openMeteoArchiveURL = prevA
		openMeteoForecastURL = prevF
		openMeteoHistoricalForecastURL = prevH
	})

	got := ResolveOutdoorWeather(server.Client(), OutdoorWeatherQuery{
		Lat:                     40.75,
		Lon:                     -73.94,
		StartUTC:                start,
		DurationSec:             1800,
		ActivityWindSpeed:       18,
		ActivityWindSpeedIsKmh:  true,
		ActivityHeadwindPercent: 50,
		ActivityTailwindPercent: 10,
		ActivityDeviceTempC:     20,
	})
	if !got.OK {
		t.Fatalf("expected OK: %+v", got)
	}
	if len(got.Hours) == 0 {
		t.Fatal("expected hourly attach alongside activity wind")
	}
	if got.MeanPressureHPa <= 0 {
		t.Fatalf("pressure=%v", got.MeanPressureHPa)
	}
}

func TestDeriveEffectiveHeadwindFromAero(t *testing.T) {
	t.Parallel()

	hw, src, ok := deriveEffectiveHeadwindMS(PowerAeroInputs{
		WindSpeed:       18,
		WindSpeedIsKmh:  true,
		HeadwindPercent: 70,
		TailwindPercent: 20,
	})
	if !ok {
		t.Fatal("expected ok")
	}
	// 18 km/h = 5 m/s; net = 0.5 → 2.5 m/s
	if hw < 2.4 || hw > 2.6 {
		t.Fatalf("hw=%.3f src=%s", hw, src)
	}
}

func TestFetchOpenMeteoHTTPErrorTruncatesBody(t *testing.T) {
	// Not parallel: mutates package Open-Meteo endpoint URLs.
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(strings.Repeat("x", 500)))
	}))
	t.Cleanup(server.Close)
	prevA, prevF, prevH := openMeteoArchiveURL, openMeteoForecastURL, openMeteoHistoricalForecastURL
	openMeteoArchiveURL = server.URL
	openMeteoForecastURL = server.URL
	openMeteoHistoricalForecastURL = server.URL
	t.Cleanup(func() {
		openMeteoArchiveURL = prevA
		openMeteoForecastURL = prevF
		openMeteoHistoricalForecastURL = prevH
	})
	_, _, err := fetchOpenMeteoChain(server.Client(), OutdoorWeatherQuery{
		Lat: 1, Lon: 2, StartUTC: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), DurationSec: 3600,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP") {
		t.Fatalf("err=%v", err)
	}
}

func TestActivityWindMSNotKmh(t *testing.T) {
	t.Parallel()

	got := ResolveOutdoorWeather(nil, OutdoorWeatherQuery{
		ActivityWindSpeed:      4.5, // m/s (below km/h heuristic)
		ActivityWindSpeedIsKmh: false,
		ActivityDeviceTempC:    15,
	})
	if !got.OK || math.Abs(got.MeanWindSpeedMS-4.5) > 0.01 {
		t.Fatalf("got=%+v", got)
	}
}
