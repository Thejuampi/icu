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
icu analysis coaching
icu analysis cycling
icu analysis wellness --days 42
icu analysis plan
icu analysis adaptation --days 42 --type Ride
icu analysis microcycle --json
icu analysis workout i123

# Calculate planned workout load locally before writing events
icu workouts calculate --ftp 300 --desc "- 60m 70%"

# Dry-run an editable calendar rebalance proposal, then apply it
icu rebalance show --file rebalance.json --oldest 2026-06-22 --newest 2026-06-28 --type Ride --target POWER --target-load 354 --target-tolerance 10 --start-time 07:00 --min-session-minutes 20 --duration-step-minutes 5 --allocation-basis explicit_equal
icu rebalance approve --file rebalance.json --reason "coach override" --target-load 380 --level 0.7
icu rebalance accept --file rebalance.json

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

### Zepp / Amazfit Authentication

The `zepp` resource reads the same data the official Zepp mobile app shows
(sleep, HR, SpO2, stress, PAI, steps, workouts) directly from the
`api-mifit.huami.com` v1 API the app uses. Login is the standard
email/password exchange the mobile app performs — there is no public OAuth
flow, and Zepp does not issue developer credentials for health data.

```bash
# 1. Log in (the password can be entered interactively if --password is omitted)
icu zepp login --email jane@example.com --password '...'

# 2. Verify the connection
icu zepp status

# 3. Read data (--oldest/--newest as YYYY-MM-DD, --date as YYYY-MM-DD)
icu zepp summary --oldest 2026-06-01 --newest 2026-06-07
icu zepp sleep   --oldest 2026-06-01 --newest 2026-06-07
icu zepp heart-rate --source band --oldest 2026-06-01 --newest 2026-06-01
icu zepp spo2    --oldest 2026-06-01 --newest 2026-06-07
icu zepp stress  --oldest 2026-06-01 --newest 2026-06-07
icu zepp pai     --oldest 2026-06-01 --newest 2026-06-07
icu zepp hrv     --metric rmssd --oldest 2026-06-01 --newest 2026-06-07
icu zepp hybridcharge --oldest 2026-06-01 --newest 2026-06-07
icu zepp body-battery --oldest 2026-06-01 --newest 2026-06-07
icu zepp sport-load --oldest 2026-06-01 --newest 2026-06-07
icu zepp weight  --oldest 2026-06-01 --newest 2026-06-07
icu zepp second-heart-rate --oldest 2026-06-01 --newest 2026-06-07
icu zepp spo2-windows --date 2026-06-07
icu zepp workouts --sport run --oldest 2026-06-01 --newest 2026-06-30
icu zepp workout --sport run 1717200000
```

The auth flow returns a `country_code` that picks the regional data host
automatically: `api-mifit.huami.com` for US/global, `api-mifit-cn.huami.com`
for CN, and `api-mifit-de.huami.com` for DE/FR/IT/ES/GB/NL/PL/RU/TR/SE/NO/FI/DK.
You can pin it explicitly with `--zepp-country-code` or `ZEPP_COUNTRY_CODE`.

Zepp field/token resolution order:

1. CLI flags (`--zepp-login-token`, `--zepp-country-code`, …)
2. Environment variables (`ZEPP_LOGIN_TOKEN`, `ZEPP_APP_TOKEN`,
   `ZEPP_USER_ID`, `ZEPP_COUNTRY_CODE`)
3. `~/.icu/config.json` (set by `icu zepp login` or
   `icu config set --zepp-login-token <token>`)

Tokens are stored next to the Intervals.icu credentials in `~/.icu/config.json`
and are never printed in clear text. `icu zepp status` and
`icu config diagnose` show only a 12-character fingerprint.

`ZEPP_BASE_URL` and `ZEPP_EVENTS_URL` are hidden env vars that override the
data and events host for tests and self-hosted proxies. Production users
should leave them unset.

#### BioCharge / HybridCharge

Recent Zepp app versions expose the energy score behind **BioCharge**
(renamed **HybridCharge** in Zepp 10.4.0+) through the Zepp V2 events stream.
Use `icu zepp hybridcharge` or the legacy alias `icu zepp biocharge` when the
score is available for your account/app build. The lower-level supporting
signals are still available through `icu zepp sleep`, `icu zepp stress`,
`icu zepp pai`, `icu zepp readiness`, `icu zepp body-battery`, and
`icu zepp workouts` when you need to inspect the components directly.

When Zepp auth is configured, `analysis wellness`, `analysis coaching`,
`analysis plan`, `analysis microcycle`, and `rebalance` prefer
`HybridCharge`/`BioCharge` for the sleep/recovery signal and only fall back to
the Intervals wellness `sleepScore` when the Zepp score is unavailable.

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
- `config` and `zepp` commands skip API key enforcement.

## Output Behavior

The CLI exposes `--output json|csv|table` and persists that preference in config, but the current command implementations primarily emit pretty JSON or raw downloaded bytes.
Treat JSON as the effective default output format unless a command explicitly documents another format.

## Documentation Map

- [docs/cli-reference.md](docs/cli-reference.md): exhaustive command reference for the current CLI surface, including the `zepp` resource
- [docs/analysis.md](docs/analysis.md): detailed behavior of `analysis coaching`, `analysis cycling`, `analysis wellness`, `analysis plan`, `analysis adaptation`, and `analysis microcycle`
- [docs/rebalance.md](docs/rebalance.md): dry-run and accept workflow for editable calendar load redistribution proposals
- [docs/library.md](docs/library.md): usage-oriented guide for `github.com/Thejuampi/icu`, including the `ZeppClient`
- [docs/api/README.md](docs/api/README.md): OpenAPI snapshot provenance and usage notes
- [AGENTS.md](AGENTS.md): contributor rules, quality gates, and the documentation gate

## CLI Surface

Current top-level resources:

| Resource | Actions |
| --- | --- |
| `activities` | `around`, `csv`, `get`, `interval-search`, `list`, `manual`, `search`, `search-full`, `upload` |
| `activity` | `best-efforts`, `delete`, `file`, `fit-file`, `gpx-file`, `hr-curve`, `intervals`, `map`, `messages`, `pace-curve`, `power-curve`, `power-vs-hr`, `segments`, `show`, `streams`, `update`, `weather`, `weather-summary` |
| `analysis` | `adaptation`, `coaching`, `cycling`, `micro`, `microcycle`, `plan`, `wellness`, `workout` |
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
| `rebalance` | `accept`, `show` |
| `routes` | `get`, `list` |
| `shared-event` | `get` |
| `sports` | `delete`, `get`, `list`, `update` |
| `tags` | `activities`, `events`, `workouts` |
| `weather` | `config`, `forecast` |
| `wellness` | `bulk`, `get`, `list`, `update`, `upload` |
| `workouts` | `calculate`, `create`, `delete`, `get`, `list`, `tags`, `update` |
| `zepp` | `biocharge`, `blood-pressure`, `body-battery`, `events`, `health-summary`, `heart-rate`, `hrv`, `hybridcharge`, `login`, `logout`, `manual-data`, `mood`, `pai`, `profile`, `readiness`, `respiratory-rate`, `second-heart-rate`, `skin-temp`, `sleep`, `spo2`, `spo2-windows`, `sport-load`, `status`, `stress`, `stress-minute`, `summary`, `token`, `vo2`, `weight`, `workout`, `workouts` |

## Analysis Overview

`icu` currently ships seven primary read-only analysis commands plus the `analysis micro` alias:

- `analysis coaching` bundles athlete, sport settings, activities, wellness, planned events, NOTE context, plan analysis, and optional adaptation into one JSON coaching payload.
- `analysis cycling` summarizes recent cycling load, intensity, environment, durability, anaerobic work, and session-level signals.
- `analysis wellness` summarizes wellness coverage and physiology signals from wellness records.
- `analysis plan` compares completed history with planned calendar events and emits a structured four-week planning view.
- `analysis adaptation` compares power curves and anchors against recent activity and wellness context.
- `analysis microcycle` emits an experimental, LLM-ready diagnostic contract for the current or selected training microcycle. `analysis micro` is its short alias.
- `analysis workout` compares one completed activity against its planned workout event, structured workout steps, intervals, and streams.

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
