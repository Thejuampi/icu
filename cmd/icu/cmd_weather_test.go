package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

type weatherTestServer struct {
	getBody string
	putBody string
	putSeen string
}

func newWeatherTestClient(t *testing.T, state *weatherTestServer) *icu.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPut {
			body, _ := io.ReadAll(request.Body) //nolint:errcheck // test server: an unreadable body fails the assertion below.
			state.putSeen = string(body)
			_, _ = response.Write([]byte(state.putBody))

			return
		}
		_, _ = response.Write([]byte(state.getBody))
	}))
	t.Cleanup(server.Close)

	return icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
}

func decodeWeatherPut(t *testing.T, body string) []map[string]any {
	t.Helper()
	var cfg struct {
		Forecasts []map[string]any `json:"forecasts"`
	}
	if err := json.Unmarshal([]byte(body), &cfg); err != nil {
		t.Fatalf("PUT body %q: %v", body, err)
	}

	return cfg.Forecasts
}

const twoForecastConfig = `{"forecasts":[` +
	`{"id":0,"provider":"OPEN_WEATHER","label":"Home","location":"LIC","lat":40.744,"lon":-73.949,"enabled":true},` +
	`{"id":1,"provider":"OPEN_WEATHER","label":"Work","lat":0,"lon":0,"enabled":true}]}`

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestWeatherConfigUpdatePreservesOtherEntriesAndFields(t *testing.T) {
	state := &weatherTestServer{getBody: twoForecastConfig, putBody: twoForecastConfig}
	client := newWeatherTestClient(t, state)

	_, err := captureStdout(t, func(stdout io.Writer) error {
		return updateWeatherConfig(stdout, map[string]string{"id": "1", "enabled": "false"}, client)
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}

	got := decodeWeatherPut(t, state.putSeen)
	if len(got) != 2 || got[0]["label"] != "Home" || got[0]["id"] != 0.0 || got[1]["enabled"] != false ||
		got[1]["provider"] != "OPEN_WEATHER" || got[1]["lat"] != 0.0 {
		t.Fatalf("PUT forecasts = %v, want both entries with id 0 kept and only entry 1 disabled", got)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestWeatherConfigUpdateSingleEntryKeepsIDZero(t *testing.T) {
	single := `{"forecasts":[{"id":0,"provider":"OPEN_WEATHER","label":"Home","lat":40.7,"lon":-73.9,"enabled":true}]}`
	state := &weatherTestServer{getBody: single, putBody: single}
	client := newWeatherTestClient(t, state)

	if _, err := captureStdout(t, func(stdout io.Writer) error {
		return updateWeatherConfig(stdout, map[string]string{"label": "Casa"}, client)
	}); err != nil {
		t.Fatalf("update error: %v", err)
	}

	got := decodeWeatherPut(t, state.putSeen)
	if len(got) != 1 || got[0]["id"] != 0.0 || got[0]["label"] != "Casa" || got[0]["lat"] != 40.7 || got[0]["enabled"] != true {
		t.Fatalf("PUT forecasts = %v, want id 0 entry relabeled with coordinates and enabled kept", got)
	}
}

func TestMergeWeatherConfigCreateDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	created, err := mergeWeatherConfig(icu.WeatherConfig{}, map[string]string{"lat": "-34.6", "lon": "-58.38", "label": "BA"})
	if err != nil || len(created.Forecasts) != 1 || created.Forecasts[0].Provider != weatherDefaultProvider ||
		created.Forecasts[0].Enabled == nil || !*created.Forecasts[0].Enabled {
		t.Fatalf("create = %+v err=%v, want OPEN_WEATHER enabled entry", created, err)
	}

	errorCases := []struct {
		name  string
		cfg   icu.WeatherConfig
		flags map[string]string
	}{
		{"create without coordinates", icu.WeatherConfig{}, map[string]string{"label": "BA"}},
		{"ambiguous without id", icu.WeatherConfig{Forecasts: []icu.Forecast{{ID: 0}, {ID: 1}}}, map[string]string{"label": "x"}},
		{"bad id", icu.WeatherConfig{}, map[string]string{"id": "x", "lat": "1", "lon": "1"}},
		{"latitude out of range", icu.WeatherConfig{}, map[string]string{"lat": "91", "lon": "1"}},
		{"unparsable longitude", icu.WeatherConfig{}, map[string]string{"lat": "1", "lon": "abc"}},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := mergeWeatherConfig(tc.cfg, tc.flags); err == nil {
				t.Fatal("error = nil, want error")
			}
		})
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestWeatherConfigUpdateRejectsEmptyResponse(t *testing.T) {
	state := &weatherTestServer{getBody: twoForecastConfig, putBody: `{"forecasts":[]}`}
	client := newWeatherTestClient(t, state)

	_, err := captureStdout(t, func(stdout io.Writer) error {
		return updateWeatherConfig(stdout, map[string]string{"id": "0", "label": "Home"}, client)
	})
	if err == nil || !strings.Contains(err.Error(), "no forecasts") {
		t.Fatalf("error = %v, want empty-response error", err)
	}
}

//nolint:paralleltest // captureStdout uses a package-level stdout override.
func TestWeatherForecastOutputsDailyHourlyAndIDZero(t *testing.T) {
	fixture, err := os.ReadFile("testdata/weather_forecast.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	client := newWeatherTestClient(t, &weatherTestServer{getBody: string(fixture)})

	registry := NewCommandRegistry()
	registerWeatherCommands(registry)
	cmd, _ := registry.Lookup("weather", "forecast")
	out, err := captureStdout(t, func(stdout io.Writer) error { return cmd.Run(nil, map[string]string{}, client, stdout, io.Discard) })
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	var got struct {
		Forecasts []struct {
			ID     *int              `json:"id"`
			Daily  []json.RawMessage `json:"daily"`
			Hourly []json.RawMessage `json:"hourly"`
		} `json:"forecasts"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got.Forecasts) != 1 || got.Forecasts[0].ID == nil ||
		*got.Forecasts[0].ID != 0 || len(got.Forecasts[0].Daily) != 2 || len(got.Forecasts[0].Hourly) != 2 ||
		!strings.Contains(out, `"sky is clear"`) {
		t.Fatalf("output = %s err=%v, want id 0 with 2 daily and 2 hourly rows verbatim", out, err)
	}
}
