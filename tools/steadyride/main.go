// Command steadyride regenerates the synthetic steady Z2 ride fixture
// (testdata/fixtures/api/v1/activity/i900000004*) from a fixed seed, so the
// fixture is reproducible and contains no captured data.
//
//	go run ./tools/steadyride -fixtures testdata/fixtures
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
)

const (
	activityID  = "i900000004"
	seed1       = 20261028
	seed2       = 4
	durationSec = 100 * 60
	rampSec     = 10 * 60
	basePower   = 172.0
	ftp         = 266
	weightKg    = 72.8
	hrLagSec    = 45
	apiDecouple = 2.5

	wavePeriodSec  = 420  // slow power wave, about 7 min
	waveWatts      = 16.0 // +- amplitude of the wave
	warmupStartW   = 110.0
	cooldownEndW   = 105.0
	noiseAR        = 0.8  // AR(1) power noise memory
	noiseWatts     = 8.0  // AR(1) innovation sd
	minWatts       = 60.0 // never coasting
	hrTau          = 25.0 // HR smoothing, seconds
	hrBase         = 58.0
	hrPerWatt      = 0.4
	hrDriftBPM     = 1.6 // over the ride: a small positive decoupling
	hrNoiseBPM     = 0.6
	cadenceBase    = 86.0
	cadencePerWatt = 1.0 / 60
	cadenceNoise   = 1.5
	speedK         = 0.26 // P = k v^3 on flat road, no wind
	npWindow       = 30
	millis         = 1000
	tenths         = 10
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("steadyride", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("fixtures", "testdata/fixtures", "fixture directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	activity, streams, err := Generate()
	if err == nil {
		err = write(*dir, activity, streams)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "steadyride:", err)

		return 1
	}

	return 0
}

func write(dir string, activity, streams []byte) error {
	base := filepath.Join(dir, "api", "v1", "activity")
	if err := os.MkdirAll(filepath.Join(base, activityID), 0o750); err != nil {
		return fmt.Errorf("create fixture dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(base, activityID+".json"), activity, 0o600); err != nil {
		return fmt.Errorf("write activity: %w", err)
	}
	if err := os.WriteFile(filepath.Join(base, activityID, "streams.json"), streams, 0o600); err != nil {
		return fmt.Errorf("write streams: %w", err)
	}

	return nil
}

// Ride is the generated 1 Hz ride.
type Ride struct {
	Watts, HR, Cadence []int
	Velocity, Distance []float64
}

// target is the planned power: 10 min ramp up, steady block with a slow
// +-16 W wave, 10 min ramp down.
func target(sec int) float64 {
	wave := waveWatts * math.Sin(2*math.Pi*float64(sec)/wavePeriodSec)
	switch {
	case sec < rampSec:
		return warmupStartW + (basePower-warmupStartW)*float64(sec)/rampSec
	case sec >= durationSec-rampSec:
		return basePower - (basePower-cooldownEndW)*float64(sec-(durationSec-rampSec))/rampSec
	default:
		return basePower + wave
	}
}

// Simulate builds the streams from the fixed seed.
func Simulate() Ride {
	rng := rand.New(rand.NewPCG(seed1, seed2)) //nolint:gosec // deterministic fixture data, not security
	ride := Ride{}
	noise, smooth, distance := 0.0, target(0), 0.0
	power := make([]float64, durationSec)
	for sec := range durationSec {
		noise = noiseAR*noise + rng.NormFloat64()*noiseWatts
		power[sec] = math.Max(minWatts, target(sec)+noise)
		ride.Watts = append(ride.Watts, int(math.Round(power[sec])))
	}
	for sec := range durationSec {
		lagged := power[max(0, sec-hrLagSec)]
		smooth += (lagged - smooth) / hrTau
		drift := hrDriftBPM * float64(sec) / durationSec
		hr := hrBase + hrPerWatt*smooth + drift + rng.NormFloat64()*hrNoiseBPM
		ride.HR = append(ride.HR, int(math.Round(hr)))
		ride.Cadence = append(ride.Cadence, int(math.Round(cadenceBase+power[sec]*cadencePerWatt+rng.NormFloat64()*cadenceNoise)))
		velocity := math.Round(math.Cbrt(power[sec]/speedK)*millis) / millis
		ride.Velocity = append(ride.Velocity, velocity)
		ride.Distance = append(ride.Distance, math.Round(distance*tenths)/tenths)
		distance += velocity
	}

	return ride
}

type stream struct {
	AllNull          bool   `json:"allNull"`
	Anomalies        any    `json:"anomalies"`
	Custom           bool   `json:"custom"`
	Data             any    `json:"data"`
	Data2            any    `json:"data2"`
	Name             any    `json:"name"`
	Type             string `json:"type"`
	ValueType        string `json:"valueType"`
	ValueTypeIsArray bool   `json:"valueTypeIsArray"`
}

func mean[T int | float64](values []T) float64 {
	sum := 0.0
	for _, value := range values {
		sum += float64(value)
	}

	return sum / float64(len(values))
}

func maxInt(values []int) int {
	best := values[0]
	for _, value := range values {
		best = max(best, value)
	}

	return best
}

func maxFloat(values []float64) float64 {
	best := values[0]
	for _, value := range values {
		best = max(best, value)
	}

	return best
}

// normalizedPower is the 30 s rolling-average NP.
func normalizedPower(watts []int) float64 {
	sum, total := 0.0, 0.0
	for i, value := range watts {
		sum += float64(value)
		if i >= npWindow {
			sum -= float64(watts[i-npWindow])
		}
		if i >= npWindow-1 {
			total += math.Pow(sum/npWindow, 4) //nolint:mnd // NP is the fourth-power mean
		}
	}

	return math.Pow(total/float64(len(watts)-npWindow+1), 0.25) //nolint:mnd // fourth root
}

func round(value float64, digits int) float64 {
	scale := math.Pow(10, float64(digits))

	return math.Round(value*scale) / scale
}

// Generate returns the activity and streams JSON.
func Generate() ([]byte, []byte, error) {
	ride := Simulate()
	time := make([]int, durationSec)
	for i := range time {
		time[i] = i
	}
	streams := []stream{
		{Data: time, Type: "time", ValueType: "java.lang.Integer"},
		{Data: ride.Watts, Type: "watts", ValueType: "java.lang.Integer"},
		{Data: ride.HR, Type: "heartrate", ValueType: "java.lang.Integer"},
		{Data: ride.Velocity, Type: "velocity_smooth", ValueType: "java.lang.Float"},
		{Data: ride.Cadence, Type: "cadence", ValueType: "java.lang.Integer"},
		{Data: ride.Distance, Type: "distance", ValueType: "java.lang.Float"},
	}
	distance := ride.Distance[len(ride.Distance)-1] + ride.Velocity[len(ride.Velocity)-1]
	np := normalizedPower(ride.Watts)
	avg := mean(ride.Watts)
	activity := map[string]any{
		"id": activityID, "name": "Activity " + activityID, "type": "Ride",
		"start_date_local": "2026-09-17T18:00:00", "start_date": "2026-09-17T22:00:00Z",
		"created": "2026-09-17T23:00:00.000+00:00", "analyzed": "2026-09-18T02:00:00.000+00:00",
		"icu_sync_date": "2026-09-18T02:00:00.000+00:00",
		"moving_time":   durationSec, "elapsed_time": durationSec, "icu_recording_time": durationSec,
		"coasting_time": 0, "distance": round(distance, 1), "icu_distance": round(distance, 1),
		"average_speed": round(distance/durationSec, 3), "max_speed": maxFloat(ride.Velocity),
		"average_heartrate": int(math.Round(mean(ride.HR))), "max_heartrate": maxInt(ride.HR),
		"average_cadence": round(mean(ride.Cadence), 2), "icu_average_watts": int(math.Round(avg)),
		"icu_weighted_avg_watts": int(math.Round(np)), "icu_variability_index": round(np/avg, 3),
		"icu_intensity": round(100*np/ftp, 2), "icu_ftp": ftp, "icu_weight": weightKg,
		"icu_power_zones": []int{55, 75, 90, 105, 120, 150, 999}, "device_watts": true,
		"trainer": false, "commute": false, "file_type": "fit", "device_name": "Mock Device",
		"power_meter": "Mock Device", "description": nil, "paired_event_id": nil,
		"decoupling": apiDecouple, "icu_athlete_id": "i0",
		"stream_types": []string{"time", "watts", "cadence", "heartrate", "distance", "velocity_smooth"},
		// Not derivable from these streams: no altitude, temperature, GPS,
		// pedal metrics or running dynamics.
		"average_altitude": nil, "total_elevation_gain": nil, "total_elevation_loss": nil,
		"average_temp": nil, "average_weather_temp": nil, "average_step_length": nil,
		"average_stride": nil, "avg_lr_balance": nil, "start_latlng": nil, "end_latlng": nil,
	}
	activityJSON, err := marshal(activity)
	if err != nil {
		return nil, nil, err
	}
	streamsJSON, err := marshal(streams)

	return activityJSON, streamsJSON, err
}

func marshal(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(value)

	return buffer.Bytes(), err
}
