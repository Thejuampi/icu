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
```

API key priority: `--api-key` flag → `INTERVALS_ICU_API_KEY` env → `~/.icu/config.json`

## Commands (~70)

| Resource | Actions |
|----------|---------|
| `athlete` | show, update, profile, summary, plan, settings |
| `activities` | list, get, upload, csv, search, search-full, interval-search, around, manual |
| `activity <id>` | show, update, delete, intervals, streams, power-curve, hr-curve, pace-curve, power-vs-hr, best-efforts, map, weather, weather-summary, file, fit-file, gpx-file, segments, messages |
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
| `config` | show, set, path |

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
