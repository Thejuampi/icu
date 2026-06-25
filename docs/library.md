# Library Guide

`github.com/Thejuampi/icu` exposes the reusable Go package behind the CLI.
It is intended for applications that want direct access to the Intervals.icu API, config helpers, output writers, and analysis functions without shelling out to the binary.

## Install

```bash
go get github.com/Thejuampi/icu@latest
```

## What The Package Provides

- `Client`: authenticated HTTP client for the supported Intervals.icu resources
- DTOs from `types.go`: activities, wellness, events, sport settings, workouts, routes, chats, and more
- `ZeppClient`: read-only client for Zepp/Amazfit wellness data (sleep, HR,
  SpO2, stress, PAI, steps, workouts). Use it for the same data the Zepp
  mobile app shows that Intervals.icu does not mirror.
- Config and auth helpers: API key, athlete ID, config file storage, and diagnostics
- Output helpers: pretty JSON, compact JSON, CSV, and table writers
- Analysis functions: cycling, wellness, adaptation, training-plan, microcycle, and workout-execution analysis
- Planned workout helpers: parse line-oriented workout descriptions and estimate cycling load from FTP-based power targets

For field-level DTO reference, use `go doc github.com/Thejuampi/icu` or inspect [types.go](../types.go).

## Constructing A Client

```go
package main

import (
	"fmt"

	icu "github.com/Thejuampi/icu"
)

func main() {
	client := icu.NewClient("your-api-key", "0")

	var athlete icu.Athlete
	if err := client.Get("athlete", nil, nil, &athlete); err != nil {
		panic(err)
	}

	fmt.Println(athlete.Name)
}
```

### Options

- `icu.WithHTTPClient(httpClient)`: inject a custom `*http.Client`
- `icu.WithBaseURL(baseURL)`: override the base URL, mainly useful for tests

Example:

```go
client := icu.NewClient(
	apiKey,
	"0",
	icu.WithHTTPClient(customHTTPClient),
	icu.WithBaseURL(testServer.URL),
)
```

## Core Client Methods

- `Get(resource string, parts []string, query map[string]string, result any) error`
- `Post(resource string, parts []string, query map[string]string, body, result any) error`
- `Put(resource string, parts []string, query map[string]string, body, result any) error`
- `Delete(resource string, parts []string, query map[string]string, result any) error`
- `Download(resource string, parts []string, query map[string]string) ([]byte, error)`
- `UploadFile(resource, localPath, filePath string, query map[string]string, result any) error`

The `resource` and `parts` values are the same routing inputs used by the CLI.
For example:

- `client.Get("activities", nil, query, &activities)`
- `client.Get("activity", []string{"i123"}, nil, &activity)`
- `client.Download("events", []string{"123", "download.zwo"}, nil)`

## Config And Auth Helpers

### Resolve values the same way as the CLI

```go
flags := map[string]string{
	"api-key":    "override-key",
	"athlete-id": "0",
}

apiKey := icu.ResolveAPIKey(flags)
athleteID := icu.ResolveAthleteID(flags)
```

Resolution order:

- API key: flag, environment, config file
- athlete ID: flag, environment, config file, default `"0"`

### Persist local config

```go
cfg := &icu.Config{
	APIKey:    "secret",
	AthleteID: "0",
	Output:    "json",
}

if err := icu.SaveConfig(cfg); err != nil {
	panic(err)
}
```

Relevant helpers:

- `ConfigPath() string`
- `LoadConfig() (*Config, error)`
- `SaveConfig(cfg *Config) error`
- `ResolveAPIKey(flags map[string]string) string`
- `ResolveAthleteID(flags map[string]string) string`
- `ResolveOutputFormat(flags map[string]string) OutputFormat`

### Diagnose config sources without exposing secrets

```go
diag := icu.DiagnoseConfig(map[string]string{
	"athlete-id": "0",
})
```

Useful exported types:

- `ConfigDiagnostic`
- `APIKeyDiagnostic`
- `SecretDiagnostic`
- `ConfigValueDiagnostic`

The diagnostic output includes a short fingerprint instead of the raw API key.

## URL And Header Helpers

The package also exports the helpers used by the client transport layer:

- `BuildAuthHeader(apiKey string) string`
- `BuildPath(athleteID, resource string, parts ...string) string`
- `BuildURL(path string, query map[string]string) string`

Example:

```go
path := icu.BuildPath("0", "activities")
url := icu.BuildURL(path, map[string]string{
	"oldest": "2026-05-01",
	"newest": "2026-05-31",
})
```

## Output Helpers

The package can write common render formats to any `io.Writer`:

- `WriteJSON`
- `WriteCompactJSON`
- `WriteCSV`
- `WriteTable`

Example:

```go
import "os"

if err := icu.WriteJSON(os.Stdout, athlete); err != nil {
	panic(err)
}
```

## Analysis Entry Points

The analysis functions operate on already-fetched DTOs and do not perform network I/O themselves.

## Planned Workout Load

Use `ParseWorkoutDescription` and `EstimatePlannedLoad` to calculate planned cycling duration, average power, normalized power, IF, and TSS without contacting Intervals.icu.

```go
doc, err := icu.ParseWorkoutDescription("- 60m 70%")
if err != nil {
	panic(err)
}

estimate := icu.EstimatePlannedLoad(doc, 300)
fmt.Println(estimate.TrainingLoad)
```

The parser intentionally supports the explicit line-oriented grammar used by the CLI planning workflow: single steps like `- 10m 55-75%`, steady targets like `- 5m 90%`, and repeat blocks like `3x` with indented child steps. Unsupported text returns `ErrWorkoutDescriptionUnsupported` instead of guessing.

### Cycling analysis

```go
analysis := icu.AnalyzeCyclingActivities(activities, icu.AnalysisOptions{
	StartDate: "2026-05-01",
	EndDate:   "2026-05-31",
})
```

### Wellness analysis

```go
wellnessAnalysis := icu.AnalyzeWellness(records, icu.AnalysisOptions{
	StartDate: "2026-05-01",
	EndDate:   "2026-05-31",
})
```

`WellnessAnalysis.HRV` includes the raw latest/mean ratio plus dynamic fields (`RecentMean`, `BaselineMean`, `BaselineMAD`, `ZScore`, and `ZScoreSource`) when enough records exist. The physiology state uses the dynamic recent-vs-baseline HRV signal instead of hard-coded absolute HRV values.

### Adaptation analysis

```go
adaptation := icu.AnalyzeCyclingAdaptation(
	curves,
	model,
	&sportSettings,
	activities,
	&wellnessAnalysis,
	icu.AnalysisOptions{StartDate: "2026-05-01", EndDate: "2026-05-31"},
)
```

### Training-plan analysis

```go
plan := icu.AnalyzeTrainingPlanWithContext(
	activities,
	events,
	icu.TrainingPlanOptions{
		HistoryStartDate: "2026-03-08",
		HistoryEndDate:   "2026-05-30",
		PlanStartDate:    "2026-06-01",
		PlanEndDate:      "2026-06-28",
	},
	icu.TrainingPlanContext{
		SportSettings: &sportSettings,
		Wellness:      &wellnessAnalysis,
		Adaptation:    &adaptation,
	},
)
```

### Workout execution analysis

```go
analysis := icu.AnalyzeWorkoutExecution(
	icu.WorkoutExecutionInputs{
		Activity:      &activity,
		Streams:       streams,
		Intervals:     &intervals,
		Events:        events,
		SportSettings: &sportSettings,
	},
	icu.WorkoutExecutionOptions{MatchWindowHours: 24},
)
```

Use `DecodeWorkoutDoc` and `ExpandWorkoutSteps` when you need to inspect or
pre-process a planned event's structured workout document directly.

Relevant exported analysis types:

- `AnalysisOptions`
- `CyclingAnalysis`
- `WellnessAnalysis`
- `CyclingAdaptationAnalysis`
- `TrainingPlanOptions`
- `TrainingPlanAnalysis`
- `TrainingPlanContext`
- `WorkoutExecutionInputs`
- `WorkoutExecutionOptions`
- `WorkoutExecutionAnalysis`
- `WorkoutDoc`
- `PlannedWorkoutStep`

See [docs/analysis.md](analysis.md) for the meaning of the major output sections.

## Using The Zepp Client

The `ZeppClient` is a separate client from `Client` because the Zepp API is
hosted on `api-mifit.huami.com` and uses its own auth flow. The flow is:

1. `ZeppLogin(email, password)` (or `ZeppLoginWithURLs` for tests) returns a
   `ZeppAuthResult` with `LoginToken`, `AppToken`, `UserID`, and
   `CountryCode`. The auth flow is two-step: an AES-CBC encrypted POST to
   `/v2/registrations/tokens` that returns a 303 redirect, then a form POST
   to `/v2/client/login` that returns the token bundle.
2. `NewZeppClientFromAuth(auth, ...)` constructs a `ZeppClient`.
3. Call `BandData`, `SleepDays`, `HeartRateSeries`, `HeartRateEndpoint`,
   `StressDays`, `SpO2Readings`, `PAIDays`, `HRVSDNNDays`, `HRVRMSSDDays`,
   `ReadinessDays`, `BodyBatteryDays`, `HealthSummaryDays`, `MoodDays`,
   `SkinTempDays`, `StressMinuteDays`, `RespiratoryRateDays`,
   `BloodPressureDays`, `BloodPressureUser`, `WeightRecords`, `ManualData`,
   `SecondHeartRateFiles`, `SpO2Windows`, `SportLoad`, `VO2Max`, `Workouts`,
   `Workout`, `UserInfo`, or `FetchV2Events` to fetch data. `FetchV2Events`
   is the low-level escape hatch for the watch-centric `/v2/users/me/events`
   stream (HRV, body battery, readiness, etc.). `Workouts` and `Workout`
   accept a sport name (`run`, `walking`, `ride`/`cycling`, `swimming`)
   resolved via `SportNameToSegment`. Exported URL builders (`V2EventsURL`,
   `WatchSportStatisticsURL`, `UserHeartRateURL`, `WeightRecordsURL`,
   `ManualDataURL`, `BloodPressureUserURL`, `SecondHeartRateFilesURL`,
   `SpO2WindowsURL`, `SportHistoryURL`, `SportDetailURL`) are available for
   consumers that need to construct signed requests manually.

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	icu "github.com/Thejuampi/icu"
)

func main() {
	auth, err := icu.ZeppLogin(os.Getenv("ZEPP_EMAIL"), os.Getenv("ZEPP_PASSWORD"))
	if err != nil {
		panic(err)
	}

	client := icu.NewZeppClientFromAuth(auth, icu.WithZeppCountryCode(auth.CountryCode))

	ctx := context.Background()
	days, err := client.BandData(ctx, "2026-05-01", "2026-05-07")
	if err != nil {
		panic(err)
	}

	for _, d := range days {
		// SummaryRaw is the base64-packed "summary" blob from Zepp;
		// Summary is the decoded version with stp/slp typed structs.
		fmt.Println(d.Date, d.Summary.Steps.Total, d.Summary.Sleep.DeepMinutes)
	}

	// Per-day SpO2 events come from a separate host (api-mifit.zepp.com).
	spo2, err := client.SpO2Readings(ctx, "2026-05-01", "2026-05-07")
	if err != nil {
		panic(err)
	}

	for _, r := range spo2 {
		b, _ := json.Marshal(r)
		fmt.Println(string(b))
	}
}
```

### Regional hosts

`api-mifit.huami.com` is the global data host. Zepp routes CN users to
`api-mifit-cn.huami.com` and a list of EU country codes to
`api-mifit-de.huami.com`. `WithZeppCountryCode` picks the right host for you;
otherwise the client derives it from `auth.CountryCode`. `WithZeppBaseURL` and
`WithZeppEventsURL` exist for tests and self-hosted proxies.

### Decoded vs raw payloads

For binary data, the client returns both the raw form Zepp sends and the
decoded typed form so downstream code can do whichever is convenient:

- `BandDataDay.SummaryRaw` (`json.RawMessage`): the base64-decoded JSON that
  was packed inside `summary`. `BandDataDay.Summary` is the typed
  `BandDataSummary` parsed from it.
- `BandDataDay.DataHRRaw` (`[]byte`): the raw 2-byte little-endian shorts.
  `BandDataDay.HeartRate` is the slice of `BandDataHeartPoint` decoded from it.
- `WorkoutDetail.HRSeries` / `PaceSeries` / `AltSeries` / `PowerSeries` /
  `StepSeries` are decoded from Zepp's delta-encoded 2-byte shorts back into
  absolute values. The first short is absolute, each subsequent short is the
  signed delta from the previous one. The cumulative sum reconstructs the
  absolute series.

### V2 wellness events

`FetchV2Events` returns raw bytes from `/v2/users/me/events`. Use
`DecodeV2Events` to normalize them, or call `V2EventPresets` /
`V2EventPresetByName` to look up supported presets:

```go
preset, ok := icu.V2EventPresetByName("body-battery")
if !ok {
    panic("unknown preset")
}

raw, err := client.FetchV2Events(ctx, preset, "2026-06-01", "2026-06-07")
if err != nil {
    panic(err)
}

events, err := icu.DecodeV2Events(raw)
if err != nil {
    panic(err)
}
```

The generic CLI command `icu zepp events --preset <name>` uses the same
primitives. Dedicated commands (`hrv`, `readiness`, etc.) add type-specific
decoding on top.

### BioCharge / HybridCharge

Zepp 10.4.0+ calculates **BioCharge** (renamed to **HybridCharge**) on-device
from sleep, stress, PAI, and workout history. The public HTTP API does not
return the score itself, so the CLI exposes the raw inputs the score is
derived from. To compute BioCharge in your analysis agent, combine `sleep`
(deep/light/REM minutes), `stress` (relax%/normal%/medium%/high%), `pai`
(daily PAI), and `workouts` (HR zones and duration). The exact weighting is
documented only inside the Zepp mobile app and changes between releases; this
library does not implement the proprietary formula.

### Error sentinels

`ErrZeppNotAuthenticated` is returned when `BandData`/`SleepDays`/etc. are
called without `AppToken` and `UserID` set. Use `errors.Is(err,
icu.ErrZeppNotAuthenticated)` to detect it.

`ErrZeppUnknownSport` is returned by `Workouts` and `Workout` when the
requested sport is not supported by `SportNameToSegment`.

## Testing Patterns

The repo itself uses:

- `httptest.NewServer` for HTTP-client tests
- `WithBaseURL`, `WithHTTPClient`, `WithZeppBaseURL`, and `WithZeppEventsURL`
  to redirect the client into test servers
- `ZeppLoginWithURLs(tokensURL, loginURL, email, password)` to drive the
  auth flow against mock servers
- `bytes.Buffer` for output-writer tests

That is the recommended pattern for consumers extending or embedding the package in their own tests.
