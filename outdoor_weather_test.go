package icu_test

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

func TestRelativeHeadwindFullHeadAndTail(t *testing.T) {
	t.Parallel()

	// Riding north (heading 0), wind from north → full headwind.
	hw := icu.RelativeHeadwindMS(5, 0, 0)
	if math.Abs(hw-5) > 1e-9 {
		t.Fatalf("headwind=%.4f want 5", hw)
	}
	// Riding north, wind from south → full tailwind (negative headwind).
	tw := icu.RelativeHeadwindMS(5, 180, 0)
	if math.Abs(tw-(-5)) > 1e-9 {
		t.Fatalf("tailwind component=%.4f want -5", tw)
	}
}

func TestBearingNorthEast(t *testing.T) {
	t.Parallel()

	// Due east from origin-ish point.
	b := icu.BearingDeg(40.0, -74.0, 40.0, -73.0)
	if b < 80 || b > 100 {
		t.Fatalf("bearing=%.1f want ~90°", b)
	}
}

func TestAirDensityFromPressureTempSeaLevel(t *testing.T) {
	t.Parallel()

	// ~1013.25 hPa at 15°C ≈ 1.225 kg/m³
	rho := icu.AirDensityFromPressureTemp(1013.25, 15)
	if math.Abs(rho-1.225) > 0.02 {
		t.Fatalf("rho=%.4f want ~1.225", rho)
	}
}

func TestHeadingSeriesFromLatLngsResamples(t *testing.T) {
	t.Parallel()

	// Straight east then north polyline.
	latlngs := [][]float64{
		{40.0, -74.0},
		{40.0, -73.9},
		{40.1, -73.9},
	}
	headings := icu.HeadingSeriesFromLatLngs(latlngs, 5)
	if len(headings) != 5 {
		t.Fatalf("len=%d want 5", len(headings))
	}
	// First samples should face roughly east (~90°).
	if headings[0] < 60 || headings[0] > 120 {
		t.Fatalf("first heading=%.1f want ~90°", headings[0])
	}
}

func TestHeadwindSeriesFromHoursUsesHeading(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC)
	hours := []icu.OutdoorWeatherHour{
		{TimeUnix: start.Unix(), TempC: 20, PressureHPa: 1013, WindSpeedMS: 4, WindFromDeg: 0},
		{TimeUnix: start.Unix() + 3600, TempC: 21, PressureHPa: 1012, WindSpeedMS: 4, WindFromDeg: 0},
	}
	// Heading 0° (north) with wind from north → +4 m/s headwind.
	headings := []float64{0, 0, 0}
	times := []float64{0, 30, 60}
	series := icu.HeadwindSeriesFromHours(hours, headings, times, start, 3)
	if len(series) != 3 {
		t.Fatalf("len=%d", len(series))
	}
	for i, hw := range series {
		if math.Abs(hw-4) > 0.05 {
			t.Fatalf("sample %d headwind=%.3f want ~4", i, hw)
		}
	}
}

func TestResolveOutdoorWeatherUsesActivityWind(t *testing.T) {
	t.Parallel()

	// No lat/lon → no external fetch; activity wind only.
	got := icu.ResolveOutdoorWeather(nil, icu.OutdoorWeatherQuery{
		ActivityWindSpeed:       18, // km/h
		ActivityWindSpeedIsKmh:  true,
		ActivityHeadwindPercent: 60,
		ActivityTailwindPercent: 20,
		ActivityDeviceTempC:     22,
		ActivityAvgAltitudeM:    50,
	})
	if !got.OK {
		t.Fatalf("expected OK activity weather: %+v", got)
	}
	if !strings.Contains(got.Source, "activity_wind") {
		t.Fatalf("source=%s", got.Source)
	}
	// 18 km/h = 5 m/s; net head fraction = 0.4 → 2 m/s mean headwind.
	if math.Abs(got.MeanWindSpeedMS-5) > 0.01 {
		t.Fatalf("windMS=%.3f want 5", got.MeanWindSpeedMS)
	}
	if math.Abs(got.MeanHeadwindMS-2) > 0.01 {
		t.Fatalf("meanHW=%.3f want 2", got.MeanHeadwindMS)
	}
	if got.MeanRho <= 0 {
		t.Fatalf("expected positive density, got %v", got.MeanRho)
	}
}

func TestParseOpenMeteoHours(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC).Unix()
	payload := map[string]any{
		"hourly": map[string]any{
			"time":               []int64{start, start + 3600},
			"temperature_2m":     []float64{21.5, 22.0},
			"surface_pressure":   []float64{1015.0, 1014.0},
			"wind_speed_10m":     []float64{3.5, 4.0},
			"wind_direction_10m": []float64{180.0, 190.0},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	hours, err := icu.ParseOpenMeteoHours(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(hours) != 2 {
		t.Fatalf("len=%d", len(hours))
	}
	if hours[0].WindSpeedMS != 3.5 || hours[0].WindFromDeg != 180 {
		t.Fatalf("hour0=%+v", hours[0])
	}
}

func TestResolveOutdoorWeatherMissingLocation(t *testing.T) {
	t.Parallel()

	got := icu.ResolveOutdoorWeather(nil, icu.OutdoorWeatherQuery{
		StartUTC: time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC),
	})
	if got.OK {
		t.Fatalf("expected not OK without location/wind, got %+v", got)
	}
	if len(got.Warnings) == 0 {
		t.Fatal("expected warning about missing weather")
	}
}

func TestResolveOutdoorWeatherLiveOpenMeteo(t *testing.T) {
	t.Parallel()
	// Optional live check against free Open-Meteo (skip on network failure).
	if testing.Short() {
		t.Skip("short mode")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	start := time.Now().UTC().Add(-2 * time.Hour)
	got := icu.ResolveOutdoorWeather(client, icu.OutdoorWeatherQuery{
		Lat:         40.75,
		Lon:         -73.94,
		StartUTC:    start,
		DurationSec: 3600,
	})
	if !got.OK {
		t.Skipf("open-meteo unavailable: %v", got.Warnings)
	}
	if got.MeanWindSpeedMS < 0 {
		t.Fatalf("wind=%v", got.MeanWindSpeedMS)
	}
	if got.MeanRho <= 0 {
		t.Fatalf("rho=%v", got.MeanRho)
	}
	if !strings.Contains(got.Source, "open_meteo") {
		t.Fatalf("source=%s", got.Source)
	}
}

func TestMapCentroidFromBounds(t *testing.T) {
	t.Parallel()

	lat, lon, ok := icu.MapCentroid(
		[][]float64{{40.7, -74.0}, {40.9, -73.8}},
		nil,
	)
	if !ok {
		t.Fatal("expected ok")
	}
	if math.Abs(lat-40.8) > 1e-9 || math.Abs(lon-(-73.9)) > 1e-9 {
		t.Fatalf("centroid=%.3f,%.3f", lat, lon)
	}
}

func TestEstimateUsesHeadwindSeriesAero(t *testing.T) {
	t.Parallel()

	params := baseParams()
	// Flat steady missing sample; strong headwind should raise estimate vs still air.
	streams := icu.NullableStreamData{
		"watts":           seriesFrom([]float64{0}, []bool{true}),
		"cadence":         seriesFrom([]float64{0}, []bool{false}),
		"velocity_smooth": seriesFrom([]float64{10}, nil),
		"time":            seriesFrom([]float64{0}, nil),
		"distance":        seriesFrom([]float64{0}, nil),
		"altitude":        seriesFrom([]float64{0}, nil),
	}
	still := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: streams, Params: params, IncludeStreams: true,
	})
	windyParams := params
	windy := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams:          streams,
		Params:           windyParams,
		HeadwindMSSeries: []float64{5}, // +5 m/s headwind
		IncludeStreams:   true,
	})
	if still.BlockingError != "" || windy.BlockingError != "" {
		t.Fatalf("blocking still=%q windy=%q", still.BlockingError, windy.BlockingError)
	}
	if windy.FilledWatts[0] <= still.FilledWatts[0] {
		t.Fatalf("headwind should increase power: still=%.1f windy=%.1f",
			still.FilledWatts[0], windy.FilledWatts[0])
	}
}

func TestApplyOutdoorWeatherSetsDensity(t *testing.T) {
	t.Parallel()

	params := icu.PowerModelParams{}
	aero := icu.PowerAeroInputs{}
	weather := icu.OutdoorWeatherResult{
		OK: true, Source: "test", MeanRho: 1.18, MeanTempC: 25,
		MeanWindSpeedMS: 3, HasWindDirection: true, MeanWindFromDeg: 90,
	}
	warns := icu.ApplyOutdoorWeatherToAero(weather, &params, &aero)
	if params.AirDensity.Value != 1.18 {
		t.Fatalf("density=%v", params.AirDensity.Value)
	}
	if aero.MeanTempC != 25 {
		t.Fatalf("temp=%v", aero.MeanTempC)
	}
	if len(warns) == 0 {
		t.Fatal("expected warnings")
	}
}

func TestDensitySeriesFromHoursUsesPressure(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	hours := []icu.OutdoorWeatherHour{
		{TimeUnix: start.Unix(), TempC: 20, PressureHPa: 1013.25, WindSpeedMS: 2, WindFromDeg: 90},
		{TimeUnix: start.Unix() + 3600, TempC: 21, PressureHPa: 1012, WindSpeedMS: 2, WindFromDeg: 90},
	}
	alt := seriesFrom([]float64{0, 0, 100}, nil)
	times := []float64{0, 30, 60}
	rho := icu.DensitySeriesFromHours(hours, alt, times, start, 3, 20, 1.2)
	if len(rho) != 3 {
		t.Fatalf("len=%d", len(rho))
	}
	if rho[0] <= 1.0 || rho[0] > 1.3 {
		t.Fatalf("rho0=%.3f out of expected range", rho[0])
	}
}

func TestDensitySeriesFallsBackWithoutHours(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	alt := seriesFrom([]float64{200}, nil)
	rho := icu.DensitySeriesFromHours(nil, alt, []float64{0}, start, 1, 15, 0)
	// nil hours → hourBracket fails → altitude path
	if rho[0] <= 0 {
		t.Fatalf("rho=%v", rho[0])
	}
}

func TestMapCentroidFromLatLngs(t *testing.T) {
	t.Parallel()

	lat, lon, ok := icu.MapCentroid(nil, [][]float64{
		{40.0, -74.0},
		{42.0, -72.0},
		{0, 0}, // ignored zero
		{1.0},  // ignored short
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if math.Abs(lat-41) > 1e-9 || math.Abs(lon-(-73)) > 1e-9 {
		t.Fatalf("centroid=%.3f,%.3f", lat, lon)
	}
}

func TestApplyOutdoorWeatherHeadTailShares(t *testing.T) {
	t.Parallel()

	params := icu.PowerModelParams{}
	aero := icu.PowerAeroInputs{}
	weather := icu.OutdoorWeatherResult{
		OK: true, Source: "activity_wind_kmh_head_tail_pct",
		MeanHeadwindMS: 1.5, HasHeadTailShares: true, MeanRho: 1.2,
	}
	_ = icu.ApplyOutdoorWeatherToAero(weather, &params, &aero)
	if params.HeadwindMS.Value != 1.5 {
		t.Fatalf("headwind=%v", params.HeadwindMS.Value)
	}
}

func TestMeanHeadwindFromSeries(t *testing.T) {
	t.Parallel()

	if got := icu.MeanHeadwindFromSeries(nil); got != 0 {
		t.Fatalf("empty=%v", got)
	}
	got := icu.MeanHeadwindFromSeries([]float64{1, 2, 3, 100})
	if got < 1 || got > 3 {
		t.Fatalf("median-ish mean=%v", got)
	}
}

func TestResolveOutdoorWeatherTempAltitudeOnly(t *testing.T) {
	t.Parallel()

	got := icu.ResolveOutdoorWeather(nil, icu.OutdoorWeatherQuery{
		ActivityDeviceTempC:  18,
		ActivityAvgAltitudeM: 300,
	})
	if !got.OK || got.Source != "activity_temp_altitude_only" {
		t.Fatalf("got=%+v", got)
	}
	if got.MeanRho <= 0 {
		t.Fatalf("rho=%v", got.MeanRho)
	}
}

func TestParseOpenMeteoHoursEmpty(t *testing.T) {
	t.Parallel()

	if _, err := icu.ParseOpenMeteoHours([]byte(`{"hourly":{"time":[]}}`)); err == nil {
		t.Fatal("expected empty hourly error")
	}
}

func TestIsOutdoorCyclingActivity(t *testing.T) {
	t.Parallel()

	if !icu.IsOutdoorCyclingActivity("Ride") {
		t.Fatal("Ride should be outdoor")
	}
	if !icu.IsOutdoorCyclingActivity("GravelRide") {
		t.Fatal("GravelRide should be outdoor")
	}
	if icu.IsOutdoorCyclingActivity("VirtualRide") {
		t.Fatal("VirtualRide should not be outdoor")
	}
	if icu.IsOutdoorCyclingActivity("IndoorCycling") {
		t.Fatal("IndoorCycling should not be outdoor")
	}
	if icu.IsOutdoorCyclingActivity("UnknownThing") {
		t.Fatal("unknown should not be outdoor")
	}
}

func TestRelativeHeadwindZeroWind(t *testing.T) {
	t.Parallel()

	if icu.RelativeHeadwindMS(0, 90, 0) != 0 {
		t.Fatal("zero wind should be zero headwind")
	}
}

func TestAirDensityFromPressureTempInvalid(t *testing.T) {
	t.Parallel()

	if icu.AirDensityFromPressureTemp(0, 15) != 0 {
		t.Fatal("zero pressure")
	}
	// Extreme cold temp uses floor kelvin path.
	rho := icu.AirDensityFromPressureTemp(1013, -100)
	if rho <= 0 {
		t.Fatalf("rho=%v", rho)
	}
}

func TestHeadingSeriesEdgeCases(t *testing.T) {
	t.Parallel()

	if icu.HeadingSeriesFromLatLngs(nil, 5) != nil {
		t.Fatal("nil track")
	}
	if icu.HeadingSeriesFromLatLngs([][]float64{{1, 2}}, 5) != nil {
		t.Fatal("single point")
	}
	// sampleCount 1
	h := icu.HeadingSeriesFromLatLngs([][]float64{{40, -74}, {40.1, -74}}, 1)
	if len(h) != 1 {
		t.Fatalf("len=%d", len(h))
	}
	// Invalid/zero points compacted away → short track
	if icu.HeadingSeriesFromLatLngs([][]float64{{0, 0}, {1}}, 3) != nil {
		t.Fatal("invalid points should yield nil")
	}
}

func TestHeadwindSeriesWithoutHeadings(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hours := []icu.OutdoorWeatherHour{{TimeUnix: start.Unix(), WindSpeedMS: 3, WindFromDeg: 90}}
	if icu.HeadwindSeriesFromHours(hours, nil, []float64{0}, start, 1) != nil {
		t.Fatal("no headings → nil series")
	}
}

func TestDensitySeriesFallbackRho(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// No hours, no altitude → fallback rho
	rho := icu.DensitySeriesFromHours(nil, icu.NullableSeries{}, []float64{0}, start, 2, 0, 1.15)
	if len(rho) != 2 || rho[0] != 1.15 {
		t.Fatalf("rho=%v", rho)
	}
}

func TestResolveOutdoorWeatherNoStartTime(t *testing.T) {
	t.Parallel()

	got := icu.ResolveOutdoorWeather(nil, icu.OutdoorWeatherQuery{
		Lat: 40, Lon: -74,
		// StartUTC zero
	})
	if got.OK {
		t.Fatalf("expected fail without start: %+v", got)
	}
	if len(got.Warnings) == 0 {
		t.Fatal("expected warning")
	}
}

func TestApplyOutdoorWeatherNilGuards(t *testing.T) {
	t.Parallel()

	if warns := icu.ApplyOutdoorWeatherToAero(icu.OutdoorWeatherResult{}, nil, nil); len(warns) != 0 {
		t.Fatalf("warns=%v", warns)
	}
	params := &icu.PowerModelParams{}
	aero := &icu.PowerAeroInputs{}
	// Head/tail shares with zero mean headwind should not set HeadwindMS
	_ = icu.ApplyOutdoorWeatherToAero(icu.OutdoorWeatherResult{
		HasHeadTailShares: true, MeanHeadwindMS: 0, MeanWindSpeedMS: 2, HasWindDirection: true, MeanWindFromDeg: 10,
	}, params, aero)
	if params.HeadwindMS.Value != 0 {
		t.Fatalf("unexpected headwind %v", params.HeadwindMS.Value)
	}
	if aero.WindSpeed != 2 {
		t.Fatalf("wind=%v", aero.WindSpeed)
	}
}

func TestMapCentroidEmpty(t *testing.T) {
	t.Parallel()

	if _, _, ok := icu.MapCentroid(nil, nil); ok {
		t.Fatal("expected not ok")
	}
	if _, _, ok := icu.MapCentroid([][]float64{{0, 0}, {0, 0}}, nil); ok {
		t.Fatal("zero bounds")
	}
}

func TestBearingSouth(t *testing.T) {
	t.Parallel()

	b := icu.BearingDeg(40.1, -74.0, 40.0, -74.0)
	if b < 170 || b > 190 {
		t.Fatalf("bearing=%.1f want ~180", b)
	}
}

func TestParseOpenMeteoHoursNullElements(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	body := []byte(fmt.Sprintf(`{
		"hourly": {
			"time": [%d, %d],
			"temperature_2m": [null, 10],
			"surface_pressure": [null, 1000],
			"wind_speed_10m": [null, 2],
			"wind_direction_10m": [null, 90]
		}
	}`, start, start+3600))
	hours, err := icu.ParseOpenMeteoHours(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(hours) != 1 {
		t.Fatalf("len=%d want 1 usable", len(hours))
	}
}
