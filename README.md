# icu

`icu` is a cross-platform Go CLI and reusable Go package for the [Intervals.icu](https://intervals.icu) REST API.
It uses only the Go standard library.

## Install

Install the CLI:

```bash
go install github.com/Thejuampi/icu/cmd/icu@latest
```

Use the Go package in another module:

```bash
go get github.com/Thejuampi/icu@latest
```

Build from source:

```bash
git clone https://github.com/Thejuampi/icu.git
cd icu
make build
```

## Quick Start

```bash
# Save credentials once
icu config set --api-key YOUR_INTERVALS_ICU_API_KEY --athlete-id 0

# Inspect athlete data
icu athlete show
icu activities list --oldest 2026-05-01 --newest 2026-05-31
icu wellness get 2026-05-31

# Analysis commands
icu analysis cycling
icu analysis wellness --days 42
icu analysis plan
icu analysis adaptation --days 42 --type Ride

# Diagnose auth/config resolution without exposing the secret
icu config diagnose
icu config diagnose --verbose
```

## Auth And Config Resolution

API key resolution order:

1. `--api-key`
2. `INTERVALS_ICU_API_KEY`
3. `~/.icu/config.json`

Athlete ID resolution order:

1. `--athlete-id`
2. `INTERVALS_ICU_ATHLETE_ID`
3. `~/.icu/config.json`
4. default `"0"` for the authenticated athlete

`icu config show` prints the config file path and masks the stored API key.
`icu config diagnose` prints source resolution details and a non-reversible fingerprint, not the raw secret.

## Help And Discovery

The CLI has resource-aware help:

```bash
icu help
icu --help
icu analysis --help
icu activity i123 --help
```

Current dispatch behavior:

- Resources with a `show` action default to `show` when invoked without an action, for example `icu athlete`.
- Resources without a `show` action print resource help when invoked without an action, for example `icu activities`.
- `activity` and `shared-event` support id-first parsing for help/dispatch.
- Only `config` commands skip API key enforcement.

## Output Behavior

The CLI exposes `--output json|csv|table` and persists that preference in config, but the current command implementations primarily emit pretty JSON or raw downloaded bytes.
Treat JSON as the effective default output format unless a command explicitly documents another format.

## Documentation Map

- [docs/cli-reference.md](docs/cli-reference.md): exhaustive command reference for the current CLI surface
- [docs/analysis.md](docs/analysis.md): detailed behavior of `analysis cycling`, `analysis wellness`, `analysis plan`, and `analysis adaptation`
- [docs/library.md](docs/library.md): usage-oriented guide for `github.com/Thejuampi/icu`
- [docs/api/README.md](docs/api/README.md): OpenAPI snapshot provenance and usage notes
- [AGENTS.md](AGENTS.md): contributor rules, quality gates, and the documentation gate

## CLI Surface

Current top-level resources:

| Resource | Actions |
| --- | --- |
| `activities` | `around`, `csv`, `get`, `interval-search`, `list`, `manual`, `search`, `search-full`, `upload` |
| `activity` | `best-efforts`, `delete`, `file`, `fit-file`, `gpx-file`, `hr-curve`, `intervals`, `map`, `messages`, `pace-curve`, `power-curve`, `power-vs-hr`, `segments`, `show`, `streams`, `update`, `weather`, `weather-summary` |
| `analysis` | `adaptation`, `cycling`, `plan`, `wellness` |
| `athlete` | `plan`, `profile`, `settings`, `show`, `summary`, `update` |
| `chats` | `get`, `list`, `messages`, `send` |
| `config` | `diagnose`, `path`, `set`, `show` |
| `curves` | `hr`, `mmp`, `pace`, `power`, `power-hr` |
| `custom` | `create`, `delete`, `get`, `list`, `update` |
| `events` | `create`, `delete`, `download`, `get`, `list`, `tags`, `update` |
| `fitness-events` | `list` |
| `folders` | `create`, `delete`, `list`, `update` |
| `ftp` | `show`, `update` |
| `gear` | `create`, `delete`, `list`, `update` |
| `routes` | `get`, `list` |
| `shared-event` | `get` |
| `sports` | `delete`, `get`, `list`, `update` |
| `tags` | `activities`, `events`, `workouts` |
| `weather` | `config`, `forecast` |
| `wellness` | `bulk`, `get`, `list`, `update`, `upload` |
| `workouts` | `create`, `delete`, `get`, `list`, `tags`, `update` |

## Analysis Overview

`icu` currently ships four read-only analysis commands:

- `analysis cycling` summarizes recent cycling load, intensity, environment, durability, anaerobic work, and session-level signals.
- `analysis wellness` summarizes wellness coverage and physiology signals from wellness records.
- `analysis plan` compares completed history with planned calendar events and emits a structured four-week planning view.
- `analysis adaptation` compares power curves and anchors against recent activity and wellness context.

See [docs/analysis.md](docs/analysis.md) for defaults, inputs, output sections, and current limitations.

## Development

Repository quality gates:

```bash
go build ./...
go test ./... -count=1
go vet ./...
golangci-lint run ./...
go test -coverprofile coverage.out ./...
go tool cover -func coverage.out
```

`AGENTS.md` requires documentation updates for every public feature change.

## License

MIT
