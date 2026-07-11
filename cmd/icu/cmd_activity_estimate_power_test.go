package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

//nolint:paralleltest // setStdoutForTest uses process-global override
func TestEstimatePowerShowDryRun(t *testing.T) {
	registry := NewCommandRegistry()
	registerAllCommands(registry)

	streamsJSON := estimatePowerTestStreamsJSON(t, 80)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/activity/i1") && request.Method == http.MethodGet &&
			!strings.Contains(request.URL.Path, "/streams") && !strings.Contains(request.URL.Path, "/map"):
			_, _ = writer.Write([]byte(`{
				"id":"i1","icuFtp":285,"type":"Ride",
				"startDate":"2026-07-11T11:09:39Z","startDateLocal":"2026-07-11T07:09:39",
				"elapsedTime":3600,"movingTime":3500,"averageTemp":21.5,
				"averageWindSpeed":18,"headwindPercent":55,"tailwindPercent":20,
				"prevailingWindDeg":40,"averageAltitude":50
			}`))
		case strings.Contains(request.URL.Path, "/activity/i1/map"):
			_, _ = writer.Write([]byte(`{
				"bounds":[[40.7,-74.0],[40.9,-73.8]],
				"latlngs":[[40.75,-73.94],[40.76,-73.93],[40.77,-73.92],[40.78,-73.91]]
			}`))
		case strings.Contains(request.URL.Path, "/activity/i1/streams"):
			_, _ = writer.Write(streamsJSON)
		case strings.Contains(request.URL.Path, "/athlete/"):
			_, _ = writer.Write([]byte(`{"id":"0","weight":75,"icuWeight":75}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(request.URL.Path))
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	cmd, ok := registry.Lookup("activity", "estimate-power")
	if !ok {
		t.Fatal("estimate-power not registered")
	}

	outPath := filepath.Join(t.TempDir(), "est.json")
	var buf bytes.Buffer
	setStdoutForTest(&buf)
	t.Cleanup(func() { setStdoutForTest(nil) })

	err := cmd.Run([]string{"i1"}, map[string]string{
		"bike-mass-kg":            "9",
		"cda":                     "0.35",
		"crr":                     "0.0045",
		"drivetrain-eff":          "0.975",
		"calibrate-from-measured": "true",
		"file":                    outPath,
		"no-weather":              "true", // activity wind only; no external Open-Meteo
	}, client)
	if err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}
	assertEstimatePowerShowOutput(t, buf.Bytes(), outPath, 80)
	var result icu.PowerFillResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	// Activity head/tail wind should produce mean headwind under --no-weather.
	if result.Model.Parameters.HeadwindMS.Value == 0 {
		// Still OK if only density was applied; check warnings mention weather/activity.
		joined := strings.Join(result.Warnings, " ")
		if !strings.Contains(joined, "weather") && !strings.Contains(joined, "headwind") &&
			!strings.Contains(joined, "density") && !strings.Contains(joined, "no-weather") {
			t.Fatalf("expected weather-related warnings, got %v", result.Warnings)
		}
	}
}

func estimatePowerTestStreamsJSON(t *testing.T, sampleCount int) []byte {
	t.Helper()
	watts := make([]any, sampleCount)
	cadence := make([]any, sampleCount)
	vel := make([]any, sampleCount)
	dist := make([]any, sampleCount)
	alt := make([]any, sampleCount)
	times := make([]any, sampleCount)
	hr := make([]any, sampleCount)
	half := sampleCount / 2
	for index := range sampleCount {
		times[index] = index
		vel[index] = 8.0
		dist[index] = float64(index) * 8
		alt[index] = 100.0
		hr[index] = 150
		if index < half {
			watts[index] = 200.0
			cadence[index] = 85.0
			continue
		}
		watts[index] = 0.0
		cadence[index] = nil
	}
	payload, err := json.Marshal([]map[string]any{
		{"type": "watts", "data": watts},
		{"type": "cadence", "data": cadence},
		{"type": "velocity_smooth", "data": vel},
		{"type": "distance", "data": dist},
		{"type": "altitude", "data": alt},
		{"type": "time", "data": times},
		{"type": "heartrate", "data": hr},
	})
	if err != nil {
		t.Fatal(err)
	}

	return payload
}

func assertEstimatePowerShowOutput(t *testing.T, stdout []byte, outPath string, sampleCount int) {
	t.Helper()
	var result icu.PowerFillResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		t.Fatalf("stdout json: %v\n%s", err, stdout)
	}
	if result.SideEffects.MutatesActivity {
		t.Fatal("dry-run must not mutate")
	}
	if result.Fill.EstimatedSeconds < 20 {
		t.Fatalf("expected gap fill, got estimated=%d blocking=%s warnings=%v", result.Fill.EstimatedSeconds, result.BlockingError, result.Warnings)
	}
	if result.Classification.MeterDeathIndex == nil {
		t.Fatal("expected meterDeathIndex")
	}
	if len(result.FilledWatts) != 0 {
		t.Fatalf("stdout should omit filledWatts, got %d", len(result.FilledWatts))
	}
	fileData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var fileResult icu.PowerFillResult
	if err := json.Unmarshal(fileData, &fileResult); err != nil {
		t.Fatal(err)
	}
	if len(fileResult.FilledWatts) != sampleCount {
		t.Fatalf("file filledWatts len=%d want %d", len(fileResult.FilledWatts), sampleCount)
	}
}

//nolint:paralleltest // setStdoutForTest uses process-global override
func TestEstimatePowerAcceptPutsStreams(t *testing.T) {
	registry := NewCommandRegistry()
	registerAllCommands(registry)

	var putBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "/streams") && request.Method == http.MethodPut:
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(request.Body)
			putBody = buf.String()
			_, _ = writer.Write([]byte(`{"updated":["watts"]}`))
		case strings.HasSuffix(request.URL.Path, "/activity/i1"):
			_, _ = writer.Write([]byte(`{"id":"i1","icuAverageWatts":180}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	filePath := filepath.Join(t.TempDir(), "accept.json")
	payload := icu.PowerFillResult{
		ActivityID:  "i1",
		FilledWatts: []float64{200, 0, 150},
		Fill:        icu.PowerFillSummary{EstimatedSeconds: 1, MeanEstimatedWatts: 150},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd, ok := registry.Lookup("activity", "estimate-power-accept")
	if !ok {
		t.Fatal("missing accept command")
	}
	var buf bytes.Buffer
	setStdoutForTest(&buf)
	t.Cleanup(func() { setStdoutForTest(nil) })

	if err := cmd.Run([]string{"i1"}, map[string]string{"file": filePath}, client); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if !strings.Contains(putBody, `"watts"`) {
		t.Fatalf("PUT body missing watts: %s", putBody)
	}
	if !strings.Contains(putBody, "150") {
		t.Fatalf("PUT body missing filled value: %s", putBody)
	}
}

func TestEstimatePowerAcceptRejectsActivityMismatch(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAllCommands(registry)
	filePath := filepath.Join(t.TempDir(), "bad.json")
	payload := icu.PowerFillResult{ActivityID: "other", FilledWatts: []float64{1}}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd, _ := registry.Lookup("activity", "estimate-power-accept")
	if err := cmd.Run([]string{"i1"}, map[string]string{"file": filePath}, nil); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestEstimatePowerShowRequiresBikeMass(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "/streams"):
			_, _ = writer.Write([]byte(`[{"type":"watts","data":[0]}]`))
		case strings.Contains(request.URL.Path, "/activity/"):
			_, _ = writer.Write([]byte(`{"id":"i1"}`))
		case strings.Contains(request.URL.Path, "/athlete/"):
			_, _ = writer.Write([]byte(`{"id":"0","weight":75}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	cmd, _ := registry.Lookup("activity", "estimate-power")
	err := cmd.Run([]string{"i1"}, map[string]string{"cda": "0.3"}, client)
	if err == nil || !strings.Contains(err.Error(), "bike mass") {
		t.Fatalf("expected bike mass error, got %v", err)
	}
}

//nolint:paralleltest // setStdoutForTest uses process-global override
func TestEstimatePowerShowCalibrateFromMeasuredPath(t *testing.T) {
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	// Longer fixture so dynamic calibration has enough measured samples.
	streamsJSON := estimatePowerTestStreamsJSON(t, 400)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "/streams"):
			_, _ = writer.Write(streamsJSON)
		case strings.Contains(request.URL.Path, "/activity/"):
			_, _ = writer.Write([]byte(`{"id":"i1","type":"Ride","distance":30000,"icuFtp":280,"icuPmFtp":270}`))
		case strings.Contains(request.URL.Path, "/athlete/"):
			_, _ = writer.Write([]byte(`{"id":"0","icuWeight":78}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	cmd, _ := registry.Lookup("activity", "estimate-power")
	var buf bytes.Buffer
	setStdoutForTest(&buf)
	t.Cleanup(func() { setStdoutForTest(nil) })
	err := cmd.Run([]string{"i1"}, map[string]string{
		"bike-mass-kg":            "8.5",
		"crr":                     "0.0045",
		"drivetrain-eff":          "0.975",
		"calibrate-from-measured": "true",
		"include-streams":         "true",
	}, client)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var result icu.PowerFillResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.BlockingError != "" {
		t.Fatalf("blocking: %s", result.BlockingError)
	}
	if result.Model.Parameters.CdA.Source != "calibrated" && result.Model.Parameters.CdA.Value <= 0 {
		t.Fatalf("expected calibrated CdA, got %+v", result.Model.Parameters.CdA)
	}
	if result.Metrics.After.FTP != 280 {
		t.Fatalf("ftp=%d want 280", result.Metrics.After.FTP)
	}
}

//nolint:paralleltest // setStdoutForTest uses process-global override
func TestEstimatePowerBacktestRejectsVirtualRide(t *testing.T) {
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "/streams"):
			_, _ = writer.Write([]byte(`[{"type":"watts","data":[100,100,100]}]`))
		case strings.Contains(request.URL.Path, "/activity/"):
			_, _ = writer.Write([]byte(`{"id":"i1","type":"VirtualRide","distance":10000}`))
		case strings.Contains(request.URL.Path, "/athlete/"):
			_, _ = writer.Write([]byte(`{"id":"0","weight":80}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	cmd, _ := registry.Lookup("activity", "estimate-power-backtest")
	var buf bytes.Buffer
	setStdoutForTest(&buf)
	t.Cleanup(func() { setStdoutForTest(nil) })
	err := cmd.Run([]string{"i1"}, map[string]string{
		"bike-mass-kg":   "8",
		"crr":            "0.004",
		"drivetrain-eff": "0.97",
		"cda":            "0.3",
	}, client)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var result icu.PowerBacktestResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.BlockingError == "" {
		t.Fatal("expected VirtualRide skip")
	}
}

func TestEstimatePowerAcceptMissingFile(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	cmd, _ := registry.Lookup("activity", "estimate-power-accept")
	err := cmd.Run([]string{"i1"}, map[string]string{}, nil)
	if err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestEstimatePowerAcceptBlockedEstimate(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	path := filepath.Join(t.TempDir(), "blocked.json")
	payload := icu.PowerFillResult{ActivityID: "i1", BlockingError: "nope", FilledWatts: []float64{1}}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd, _ := registry.Lookup("activity", "estimate-power-accept")
	if err := cmd.Run([]string{"i1"}, map[string]string{"file": path}, nil); err == nil {
		t.Fatal("expected blocked estimate error")
	}
}

func TestEstimatePowerShowMissingID(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	cmd, _ := registry.Lookup("activity", "estimate-power")
	if err := cmd.Run(nil, map[string]string{}, nil); err == nil {
		t.Fatal("expected missing id")
	}
}

func TestEstimatePowerAcceptMissingID(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	cmd, _ := registry.Lookup("activity", "estimate-power-accept")
	if err := cmd.Run(nil, map[string]string{"file": "x"}, nil); err == nil {
		t.Fatal("expected missing id")
	}
}

func TestEstimatePowerAcceptEmptyFilledWatts(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	path := filepath.Join(t.TempDir(), "empty.json")
	payload := icu.PowerFillResult{ActivityID: "i1"}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd, _ := registry.Lookup("activity", "estimate-power-accept")
	if err := cmd.Run([]string{"i1"}, map[string]string{"file": path}, nil); err == nil {
		t.Fatal("expected empty filledWatts error")
	}
}

func TestEstimatePowerAcceptBadFile(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd, _ := registry.Lookup("activity", "estimate-power-accept")
	if err := cmd.Run([]string{"i1"}, map[string]string{"file": path}, nil); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestEstimatePowerShowRequiresCdA(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "/streams"):
			_, _ = writer.Write([]byte(`[{"type":"watts","data":[100]}]`))
		case strings.Contains(request.URL.Path, "/activity/"):
			_, _ = writer.Write([]byte(`{"id":"i1"}`))
		case strings.Contains(request.URL.Path, "/athlete/"):
			_, _ = writer.Write([]byte(`{"id":"0","weight":70}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	cmd, _ := registry.Lookup("activity", "estimate-power")
	err := cmd.Run([]string{"i1"}, map[string]string{"bike-mass-kg": "9"}, client)
	if err == nil || !strings.Contains(err.Error(), "CdA") {
		t.Fatalf("expected CdA error, got %v", err)
	}
}

//nolint:paralleltest // setStdoutForTest uses process-global override
func TestEstimatePowerShowRiderMassFromAthleteWeight(t *testing.T) {
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	streamsJSON := estimatePowerTestStreamsJSON(t, 50)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "/streams"):
			_, _ = writer.Write(streamsJSON)
		case strings.Contains(request.URL.Path, "/activity/"):
			_, _ = writer.Write([]byte(`{"id":"i1","icuPmFtp":260}`))
		case strings.Contains(request.URL.Path, "/athlete/"):
			_, _ = writer.Write([]byte(`{"id":"0","weight":72}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	cmd, _ := registry.Lookup("activity", "estimate-power")
	var buf bytes.Buffer
	setStdoutForTest(&buf)
	t.Cleanup(func() { setStdoutForTest(nil) })
	err := cmd.Run([]string{"i1"}, map[string]string{
		"bike-mass-kg": "9",
		"cda":          "0.32",
		"ftp":          "300",
	}, client)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var result icu.PowerFillResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Model.Parameters.RiderMassKg.Value != 72 {
		t.Fatalf("rider mass=%v", result.Model.Parameters.RiderMassKg.Value)
	}
	if result.Metrics.After.FTP != 300 {
		t.Fatalf("ftp=%d", result.Metrics.After.FTP)
	}
}

func TestResolveEstimatePowerFTPFallbacks(t *testing.T) {
	t.Parallel()
	activity := &icu.Activity{FTPWatts: 255}
	ftp, source := resolveEstimatePowerFTP(activity, map[string]string{})
	if ftp != 255 || source != "activity_pm_ftp" {
		t.Fatalf("ftp=%d source=%s", ftp, source)
	}
	ftp, source = resolveEstimatePowerFTP(&icu.Activity{}, map[string]string{})
	if ftp != 0 || source != "" {
		t.Fatalf("empty ftp=%d source=%s", ftp, source)
	}
}

func TestWriteEstimatePowerFileErrors(t *testing.T) {
	t.Parallel()
	if err := writeEstimatePowerFile(filepath.Join(t.TempDir(), "x.json"), nil); err == nil {
		t.Fatal("expected nil result error")
	}
	if err := writeEstimatePowerFile(filepath.Join(t.TempDir(), "x.json"), &icu.PowerFillResult{}); err == nil {
		t.Fatal("expected empty filled watts error")
	}
	path := filepath.Join(t.TempDir(), "ok.json")
	result := &icu.PowerFillResult{ActivityID: "i1", FilledWatts: []float64{1, 2, 3}}
	if err := writeEstimatePowerFile(path, result); err != nil {
		t.Fatal(err)
	}
	got, err := readEstimatePowerFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.FilledWatts) != 3 {
		t.Fatalf("roundtrip len=%d", len(got.FilledWatts))
	}
}

func TestReadEstimatePowerFileMissing(t *testing.T) {
	t.Parallel()
	if _, err := readEstimatePowerFile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestResolveRiderMassExplicitAndAthleteFailure(t *testing.T) {
	t.Parallel()
	mass, source, err := resolveRiderMass(nil, map[string]string{"rider-mass-kg": "77.5"})
	if err != nil || mass != 77.5 || source != "user" {
		t.Fatalf("mass=%v source=%s err=%v", mass, source, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	if _, _, err := resolveRiderMass(client, map[string]string{}); err == nil {
		t.Fatal("expected athlete mass failure")
	}
}

func TestStreamSampleCountDistanceOnly(t *testing.T) {
	t.Parallel()

	streams := icu.NullableStreamData{
		"distance": seriesFromCLI([]float64{0, 1, 2, 3}, nil),
	}
	if streamSampleCount(streams) != 4 {
		t.Fatalf("count=%d", streamSampleCount(streams))
	}
	if streamSampleCount(nil) != 0 {
		t.Fatal("nil streams")
	}
}

func TestFetchEstimateWeatherNoWeatherFlag(t *testing.T) {
	t.Parallel()

	weather, warns := fetchEstimateWeather(map[string]string{"no-weather": "true"}, icu.OutdoorWeatherQuery{
		Lat:                  40,
		Lon:                  -74,
		ActivityDeviceTempC:  20,
		ActivityAvgAltitudeM: 100,
	})
	if len(warns) == 0 {
		t.Fatal("expected no-weather warning")
	}
	if weather.MeanRho <= 0 && !weather.OK {
		// density-only path should still produce rho when temp/alt present
		t.Logf("weather=%+v", weather)
	}
}

func TestBuildWeatherSeriesAndParseStart(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC)
	streams := icu.NullableStreamData{
		"time":     seriesFromCLI([]float64{0, 1, 2}, nil),
		"altitude": seriesFromCLI([]float64{10, 20, 30}, nil),
		"watts":    seriesFromCLI([]float64{100, 100, 100}, nil),
	}
	hours := []icu.OutdoorWeatherHour{
		{TimeUnix: start.Unix(), TempC: 22, PressureHPa: 1013, WindSpeedMS: 4, WindFromDeg: 0},
		{TimeUnix: start.Unix() + 3600, TempC: 23, PressureHPa: 1012, WindSpeedMS: 5, WindFromDeg: 10},
	}
	weather := icu.OutdoorWeatherResult{
		OK: true, Source: "test", Hours: hours, MeanTempC: 22, MeanRho: 1.2,
	}
	params := &icu.PowerModelParams{}
	latlngs := [][]float64{{40.0, -74.0}, {40.01, -74.0}, {40.02, -74.0}}
	hw, rho := buildWeatherSeries(streams, latlngs, weather, start, 3, params)
	if len(hw) != 3 {
		t.Fatalf("headwind len=%d", len(hw))
	}
	if len(rho) != 3 || rho[0] <= 0 {
		t.Fatalf("rho=%v", rho)
	}
	if params.AirDensity.Value <= 0 {
		t.Fatalf("density not set on params")
	}

	act := &icu.Activity{StartDate: "2026-07-11T11:09:39Z"}
	if parseActivityStartUTC(act).IsZero() {
		t.Fatal("expected parsed start")
	}
	act2 := &icu.Activity{StartDateLocal: "2026-07-11T07:09:39"}
	if parseActivityStartUTC(act2).IsZero() {
		t.Fatal("expected local start parse")
	}
	if streamSampleCount(streams) != 3 {
		t.Fatalf("sampleCount=%d", streamSampleCount(streams))
	}
	if activityDurationSec(&icu.Activity{ElapsedTime: 100}, 3) != 100 {
		t.Fatal("elapsed")
	}
	if activityDurationSec(&icu.Activity{MovingTime: 90}, 3) != 90 {
		t.Fatal("moving")
	}
	if activityDurationSec(&icu.Activity{}, 3) != 3 {
		t.Fatal("sample fallback")
	}
}

func seriesFromCLI(values []float64, present []bool) icu.NullableSeries {
	if present == nil {
		present = make([]bool, len(values))
		for i := range present {
			present[i] = true
		}
	}

	return icu.NullableSeries{Values: values, Present: present}
}

func refillMaskStreams(n, death int) icu.NullableStreamData {
	watts := make([]float64, n)
	wp := make([]bool, n)
	cad := make([]float64, n)
	cp := make([]bool, n)
	bal := make([]float64, n)
	bp := make([]bool, n)
	for i := range n {
		if i < death {
			watts[i] = 200
			wp[i] = true
			cad[i] = 90
			cp[i] = true
			bal[i] = 50
			bp[i] = true
			continue
		}
		// Prior accepted fill sits as positive watts without L/R or cadence.
		watts[i] = 180
		wp[i] = true
		cp[i] = false
		bp[i] = false
	}

	return icu.NullableStreamData{
		"watts":              seriesFromCLI(watts, wp),
		"cadence":            seriesFromCLI(cad, cp),
		"left_right_balance": seriesFromCLI(bal, bp),
	}
}

func TestApplyEstimatePowerRefillMaskFromIndex(t *testing.T) {
	t.Parallel()

	streams := refillMaskStreams(100, 40)
	got, warnings, err := applyEstimatePowerRefillMask(streams, map[string]string{
		"refill-from-index": "40",
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected refill warning")
	}
	// Second half cadence must be absent so classification treats it as missing.
	cad := icu.NullableStream(got, "cadence")
	if _, ok := cad.At(50); ok {
		t.Fatal("expected cadence masked after refill index")
	}
	if _, ok := cad.At(10); !ok {
		t.Fatal("first-half cadence must stay present")
	}
}

func TestApplyEstimatePowerRefillMaskInvalidIndex(t *testing.T) {
	t.Parallel()

	_, _, err := applyEstimatePowerRefillMask(refillMaskStreams(20, 10), map[string]string{
		"refill-from-index": "nope",
	})
	if err == nil {
		t.Fatal("expected invalid index error")
	}
	_, _, err = applyEstimatePowerRefillMask(refillMaskStreams(20, 10), map[string]string{
		"refill-from-index": "99",
	})
	if err == nil {
		t.Fatal("expected out-of-range index error")
	}
}

func TestApplyEstimatePowerRefillMaskAfterPMDeath(t *testing.T) {
	t.Parallel()

	streams := refillMaskStreams(120, 50)
	got, warnings, err := applyEstimatePowerRefillMask(streams, map[string]string{
		"refill-after-pm-death": "true",
	})
	if err != nil {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if len(warnings) == 0 {
		t.Fatal("expected mask warning")
	}
	cad := icu.NullableStream(got, "cadence")
	if _, ok := cad.At(80); ok {
		t.Fatal("expected second-half cadence masked")
	}
}

func TestApplyEstimatePowerRefillMaskAfterCadenceDeath(t *testing.T) {
	t.Parallel()

	// Cadence-only death: no balance stream.
	n := 80
	watts := make([]float64, n)
	wp := make([]bool, n)
	cad := make([]float64, n)
	cp := make([]bool, n)
	for i := range n {
		watts[i] = 190
		wp[i] = true
		if i < 40 {
			cad[i] = 85
			cp[i] = true
		}
	}
	streams := icu.NullableStreamData{
		"watts":   seriesFromCLI(watts, wp),
		"cadence": seriesFromCLI(cad, cp),
	}
	got, _, err := applyEstimatePowerRefillMask(streams, map[string]string{
		"refill-after-cadence-death": "true",
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, ok := icu.NullableStream(got, "cadence").At(60); ok {
		t.Fatal("expected cadence masked after death")
	}
}

func TestApplyEstimatePowerRefillMaskPMDeathMissing(t *testing.T) {
	t.Parallel()

	// All balance present — no death tail.
	n := 40
	bal := make([]float64, n)
	bp := make([]bool, n)
	cad := make([]float64, n)
	cp := make([]bool, n)
	watts := make([]float64, n)
	wp := make([]bool, n)
	for i := range n {
		bal[i] = 50
		bp[i] = true
		cad[i] = 90
		cp[i] = true
		watts[i] = 200
		wp[i] = true
	}
	streams := icu.NullableStreamData{
		"watts":              seriesFromCLI(watts, wp),
		"cadence":            seriesFromCLI(cad, cp),
		"left_right_balance": seriesFromCLI(bal, bp),
	}
	_, _, err := applyEstimatePowerRefillMask(streams, map[string]string{
		"refill-after-pm-death": "true",
	})
	if err == nil {
		t.Fatal("expected missing death error")
	}
}

func TestApplyEstimatePowerRefillMaskSoftHintPriorFill(t *testing.T) {
	t.Parallel()

	// No refill flag: long synthetic tail after balance death should warn.
	streams := refillMaskStreams(100, 40)
	_, warnings, err := applyEstimatePowerRefillMask(streams, map[string]string{})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	found := false
	for _, warning := range warnings {
		if strings.Contains(warning, "prior fill") || strings.Contains(warning, "refill-after-pm-death") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected prior-fill soft hint, got %v", warnings)
	}
}

func TestResolveRiderMassFromWellness(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "wellness") {
			_, _ = writer.Write([]byte(`[{"id":"1","weight":0},{"id":"2","weight":81.5}]`))
			return
		}
		writer.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	mass, source, ok := resolveRiderMassFromWellness(client)
	if !ok || mass != 81.5 || source != "wellness_weight" {
		t.Fatalf("mass=%v source=%s ok=%v", mass, source, ok)
	}
}

func TestResolveRiderMassFromWellnessEmpty(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
	if _, _, ok := resolveRiderMassFromWellness(client); ok {
		t.Fatal("expected no mass")
	}
}
