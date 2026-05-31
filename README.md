# icu — Intervals.icu CLI

A fast, cross-platform CLI for the full [Intervals.icu](https://intervals.icu) API.
Zero dependencies beyond the Go standard library.

## Install

```bash
go install github.com/Thejuampi/icu@latest
```

Or build from source:

```bash
git clone https://github.com/Thejuampi/icu.git
cd icu
make build
```

## Quick Start

```bash
# One-time setup: save your API key
icu config set --api-key YOUR_INTERVALS_ICU_API_KEY

# Now everything works without --api-key
icu athlete show
icu activities list --oldest 2026-05-20 --newest 2026-05-24
icu ftp show
icu wellness get 2026-05-24
icu analysis cycling --days 28
icu analysis wellness --days 42
icu analysis plan
```

API key priority: `--api-key` flag → `INTERVALS_ICU_API_KEY` env → `~/.icu/config.json`

## Commands (~70)

| Resource | Actions |
| -------- | ------- |
| `athlete` | show, update, profile, summary, plan, settings |
| `activities` | list, get, upload, csv, search, search-full, interval-search, around, manual |
| `activity <id>` | show, update, delete, intervals, streams, power-curve, hr-curve, pace-curve, power-vs-hr, best-efforts, map, weather, weather-summary, file, fit-file, gpx-file, segments, messages |
| `analysis` | cycling, wellness, plan |
| `wellness` | list, get, update, bulk, upload |
| `events` | list, get, create, update, delete, download, tags |
| `workouts` | list, get, create, update, delete, tags |
| `folders` | list, create, update, delete |
| `sports` | list, get, update, delete |
| `ftp` | show, update |
| `curves` | power, hr, pace, power-hr, mmp |
| `routes` | list, get |
| `gear` | list, create, update, delete |
| `chats` | list, get, messages, send |
| `custom` | list, get, create, update, delete |
| `weather` | config, forecast |
| `fitness-events` | list |
| `tags` | activities, events, workouts |
| `shared-event` | get |
| `config` | show, set, path, diagnose |

## Numeric Cycling Analysis

`icu analysis cycling` fetches completed activities from Intervals.icu and returns a JSON payload designed for downstream coaching analysis. It is intentionally numeric and read-only so it can be combined with an AI layer without hiding the underlying signals.

```bash
# Last 28 days by default
icu analysis cycling

# Explicit range
icu analysis cycling --oldest 2026-05-01 --newest 2026-05-29

# Rolling window
icu analysis cycling --days 42
```

The output includes cycling activity count, volume, training load, daily load, monotony, strain, ACWR, zone totals, weighted intensity, decoupling, efficiency factor, variability index, and W' related metrics when Intervals.icu provides them.

`icu analysis wellness` summarizes wellness records for physiology and planning decisions:

```bash
icu analysis wellness --days 42
icu analysis wellness --oldest 2026-04-18 --newest 2026-05-30
```

The output includes HRV, resting heart rate, sleep score, subjective wellness coverage, CTL/ATL/TSB from wellness records, and a local physiology state (`OK`, `WATCH`, or `RED`).

`icu analysis plan` compares recent completed cycling history with future calendar events to produce a numeric 4-week planning view:

```bash
# Default: 12-week completed history and the next 4-week ISO block
icu analysis plan

# Explicit planning window
icu analysis plan --history-oldest 2026-03-08 --history-newest 2026-05-30 \
	--plan-oldest 2026-06-01 --plan-newest 2026-06-28
```

The output includes completed-history tolerance, planned load alignment, block phase (`build`, `recovery`, `maintenance`, etc.), week roles such as `reentry`, `build`, `overload`, and `deload`, session classification (`high_intensity`, `tempo_threshold`, `long_endurance`, `recovery`, `aerobic`, `opener`, `rest`), recommended workout titles, execution intent, device-cue messages, indoor Z2 variation profiles, key sessions, weekly focus, and warnings for risky load/session combinations.

For long or aerobic Z2 rides, `analysis plan` also emits an indoor-friendly `workoutProfile` with 4-minute Z2 waves, 40-second HR-control valleys, low/mid/high Z2 rotation, and max-Z2 caps so indoor endurance sessions do not become a flat steady-power block.

## Auth Diagnostics

Use `icu config diagnose` when authentication behaves unexpectedly. It reports where the CLI resolves values from without printing secrets.

```bash
icu config diagnose
icu config diagnose --athlete-id i445643
```

The API key section includes only set/missing status, length, trim length, whitespace status, and a short non-reversible fingerprint for flag, environment, config file, and resolved key.

## Development

```bash
make build        # Build binary
make test         # Run tests
make test-cover   # Tests + coverage
make test-race    # Race detector
make lint         # golangci-lint
make fmt          # Format code
make vet          # go vet
make ci           # All quality gates
```

## License

MIT
