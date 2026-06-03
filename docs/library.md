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
- Config and auth helpers: API key, athlete ID, config file storage, and diagnostics
- Output helpers: pretty JSON, compact JSON, CSV, and table writers
- Analysis functions: cycling, wellness, adaptation, and training-plan analysis

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

Relevant exported analysis types:

- `AnalysisOptions`
- `CyclingAnalysis`
- `WellnessAnalysis`
- `CyclingAdaptationAnalysis`
- `TrainingPlanOptions`
- `TrainingPlanAnalysis`
- `TrainingPlanContext`

See [docs/analysis.md](analysis.md) for the meaning of the major output sections.

## Testing Patterns

The repo itself uses:

- `httptest.NewServer` for HTTP-client tests
- `WithBaseURL` and `WithHTTPClient` to redirect the client into test servers
- `bytes.Buffer` for output-writer tests

That is the recommended pattern for consumers extending or embedding the package in their own tests.
