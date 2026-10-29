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

# Multi-session plan from explicit intent (preferred for writing several workouts)
icu plan show --file plan.json --intent-file intent.json --now-date 2026-07-27 --type Ride
icu plan preview --file plan.json
icu plan accept --file plan.json

# Dry-run an editable calendar rebalance proposal, then apply it
icu rebalance show --file rebalance.json --oldest 2026-06-22 --newest 2026-06-28 --type Ride --target POWER --target-load 354 --target-tolerance 10 --start-time 07:00 --min-session-minutes 20 --duration-step-minutes 5 --allocation-basis explicit_equal
icu rebalance preview --file rebalance.json
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

API base URL resolution order:

1. `--base-url URL`
2. `ICU_BASE_URL`
3. default `https://intervals.icu` (the real API; a warning is printed on stderr whenever this is used)

`--base-url` may appear before the resource (`icu --base-url URL athlete show`) or among the command flags. It only redirects intervals.icu calls and HTTP redirects to a different scheme or host are refused. Two outside services ignore it:

- **Open-Meteo** (weather for `activity estimate-power` and `activity estimate-power-backtest`): pass `--no-weather` when testing against the mock. Only those two commands fetch weather and accept `--no-weather`; other commands (for example `analysis coaching`) make no weather calls and reject it as an unknown flag.
- **Zepp** (`zepp ...`): not served by the mock. `zepp login`/`zepp token` take `--zepp-base-url URL` (wins over `ZEPP_TOKENS_URL`/`ZEPP_LOGIN_URL`); other `zepp` commands use the `ZEPP_*` settings. The global `--base-url` is never passed to Zepp.

`icu config show` prints the config file path and shows only a 12-character SHA256 fingerprint for the stored API key (never partial or raw characters).
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
- [docs/rebalance.md](docs/rebalance.md): dry-run, preview, and accept workflow for editable calendar load redistribution proposals
- [docs/plan.md](docs/plan.md): intent → show/preview/accept workflow for explicit multi-session calendar writes
- [docs/library.md](docs/library.md): usage-oriented guide for `github.com/Thejuampi/icu`, including the `ZeppClient`
- [docs/api/README.md](docs/api/README.md): OpenAPI snapshot provenance and usage notes
- [AGENTS.md](AGENTS.md): contributor rules, evidence scope and temporal validity, quality gates, and the documentation gate

## CLI Surface

Current top-level resources:

| Resource | Actions |
| --- | --- |
| `activities` | `around`, `csv`, `get`, `interval-search`, `list`, `manual`, `search`, `search-full`, `upload` |
| `activity` | `best-efforts`, `delete`, `estimate-power`, `estimate-power-accept`, `estimate-power-backtest`, `file`, `fit-file`, `gpx-file`, `hr-curve`, `intervals`, `map`, `messages`, `pace-curve`, `power-curve`, `power-vs-hr`, `segments`, `show`, `streams`, `update`, `weather`, `weather-summary` |
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
| `plan` | `accept`, `preview`, `show` |
| `rebalance` | `accept`, `approve`, `preview`, `show` |
| `routes` | `get`, `list` |
| `shared-event` | `get` |
| `sports` | `delete`, `get`, `list`, `update` |
| `tags` | `activities`, `events`, `workouts` |
| `weather` | `config`, `forecast` |
| `wellness` | `bulk`, `get`, `list`, `update`, `upload` |
| `workouts` | `calculate`, `create`, `delete`, `get`, `list`, `tags`, `update` |
| `zepp` | `biocharge`, `blood-pressure`, `body-battery`, `events`, `health-summary`, `heart-rate`, `hrv`, `hybridcharge`, `login`, `logout`, `manual-data`, `mood`, `pai`, `profile`, `readiness`, `respiratory-rate`, `second-heart-rate`, `skin-temp`, `sleep`, `spo2`, `spo2-windows`, `sport-load`, `status`, `stress`, `stress-minute`, `summary`, `token`, `vo2`, `weight`, `workout`, `workouts` |

## Power Gap Fill (PM Death)

When a power meter dies mid-ride, use outdoor physics fill rather than inventing TSS from RPE:

> **Rider mass:** without `--rider-mass-kg`, `estimate-power` now uses the athlete's `icu_weight` (falling back to `weight`, then wellness weight). Earlier versions never decoded `icu_weight` from the API, so they fell through to `weight`. If those two differ, estimated watts change; pass `--rider-mass-kg` to pin the old value.

```bash
# Dry-run (classifies measured / true_zero / missing; real weather aero outdoors)
icu activity i123 estimate-power --rider-mass-kg 81 --bike-mass-kg 7.8 --calibrate-from-measured --file fill.json

# If a prior fill already wrote watts into the dead half, re-open via L/R balance (then cadence)
icu activity i123 estimate-power --rider-mass-kg 81 --bike-mass-kg 7.8 --calibrate-from-measured --refill-after-pm-death --file fill.json

# Optional: score the model on a continuous measured outdoor ride first
icu activity i123 estimate-power-backtest --rider-mass-kg 81 --bike-mass-kg 7.8 --calibrate-from-measured

# Accept after review (mutates watts stream on Intervals; supporter feature)
icu activity i123 estimate-power-accept --file fill.json
```

Dual-sided meters: death is detected from the long null tail of `left_right_balance` after a live first half (real watts + RPM + L/R). Outdoor aero uses activity wind when present, otherwise free Open-Meteo (archive → forecast past → historical-forecast) with track heading for relative wind. Back up streams and the dry-run fill file before accept so you can re-accept a rollback payload if needed.

**Strava does not receive the filled watts.** Accept only updates Intervals.icu streams. To fix Strava, export the Intervals-processed FIT (`icu activity i123 fit-file` — includes stream edits; not `file`, which is the original device upload), delete the broken Strava activity, and re-upload the FIT. Title/description sync is metadata-only.

Details: [docs/cli-reference.md](docs/cli-reference.md), [docs/library.md](docs/library.md).

## Analysis Overview

`icu` currently ships seven primary read-only analysis commands plus the `analysis micro` alias:

- `analysis coaching` bundles athlete, sport settings, activities, wellness, planned events, NOTE context, plan analysis, and optional adaptation into one JSON coaching payload.
- `analysis cycling` summarizes recent cycling load, intensity, environment, durability, anaerobic work, and session-level signals.
- `analysis wellness` summarizes wellness coverage and physiology signals from wellness records.
- `analysis plan` compares completed history with planned calendar events and emits a structured four-week planning view.
- `analysis adaptation` compares power curves and anchors against recent activity and wellness context.
- `analysis microcycle` emits an experimental, LLM-ready diagnostic contract for the current or selected training microcycle. `analysis micro` is its short alias.
- `analysis workout` compares one completed activity against its planned workout event, structured workout steps, intervals, and streams.

`analysis cycling`, `analysis coaching`, and `analysis microcycle` exclude STRAVA-stub records (5 raw keys, `_note` present, `source == "STRAVA"`) from every activity count and load aggregate, and `analysis cycling` reports a `validity` block naming which activities were excluded and why (`icu_ignore_power`/`icu_ignore_hr`/`icu_ignore_time`, D-2). That ignore-flag filtering is correct and tested, but has been measured `false` on every real activity checked against a live account so far — see [docs/analysis.md](docs/analysis.md) for the inertness statement before crediting it with a reported number change.

### Aerobic decoupling (B15, experimental)

The stream method is **experimental and opt-in**: `activity decoupling` help is prefixed `[EXPERIMENTAL]` and its JSON carries `"experimental": true`; sessions with a stream result carry `decouplingExperimental: true`.

intervals.icu's `decoupling` field remains the default value consumed by `analysis cycling|coaching|plan|microcycle` (`--decoupling-source api`, no extra requests). A stream-based method is available next to it and is not yet the default because it has not been validated on enough rides. With `--decoupling-source streams` the analysis commands recompute decoupling from each ride's streams (`GET /activity/{id}/streams`, read-only), report both values, and consume the stream value when it is measured.

- **Method** (Friel Pa:HR; GAP:HR from `ga_velocity`, or pace:HR, without power; speeds in m/s), on moving-time minutes. Nothing is fitted to a reference ride:
  1. Moving samples: speed above a stop speed derived so the moving sample time matches the activity's `moving_time` (a rank in the ride's own speed distribution, so it does not depend on units). When that rank is 0 the rule is plain `speed>0`; `stopRule` reports which (`moving_time_rank` or `speed>0`).
  2. Trim: `--decoupling-warmup-min` / `--decoupling-cooldown-min` (integers) if given. Otherwise the paired workout's leading/trailing steps that are ramps, labeled warm-up/cool-down, or below the workout's duration-weighted median target. Then steady-state edges are dropped: leading/trailing minutes whose output is below the ride's lower quartile.
  3. HR lag: `--decoupling-hr-lag` (integer seconds), or the 0–120 s lag that maximizes the power/HR cross-correlation (0 when uncorrelated).
  4. 60-moving-second buckets from the trim start. `missing_hr` when lagged HR covers under half the minute. `coasting` (power basis) when watts = 0 for more than half of the minute's moving samples (the coasting share is a strict majority rule; a minute with ≤ 50 % coasting is kept).
  5. Tukey fences `Q1 − k·IQR … Q3 + k·IQR` on the per-minute ratio of the remaining minutes (`--decoupling-iqr-k`, default 1.5, 0 disables): `ratio_outlier`.
  6. Kept minutes are split at the range midpoint: `decoupling = (1 − (P2/HR2)/(P1/HR1)) × 100`. Each half needs at least 4 kept minutes.
  7. Workout gate: when the paired workout's main set (inside the trim) spans more than one power zone (the doc's `zoneTimes`; without them the athlete's Ride power zones from sport settings; `power_zone` units directly; watt targets converted to % FTP with the doc's or the athlete's FTP) it is `intervals`; when main-set steps (in a repeat block or written as flat steps) use more than one cadence target it is `ladder`. A main set with pace targets (runs), or watt targets with no FTP, is `unverified`. All three are `not_measurable`.
  8. Steadiness gate: the VI cutoff is `Q3 + 1.5·IQR` of the VIs of rides paired with *steady* workouts in the last 84 days (also in analysis), needing at least 4 such rides. A ride above it is `not_measurable`. With no cutoff, a ride is measurable only if its paired workout is steady; otherwise `not_measurable` (steadiness unverified). The in-window CV of per-minute output (`powerCv`) is reported but not gated, since no data-derived cutoff exists for it yet.
  **Unpaired rides are not measured** unless at least 4 rides paired with steady workouts in the last 84 days give a VI cutoff. For an athlete who rarely pairs steady workouts this means the stream method almost never measures anything today (for the reference athlete: 1 steady-paired ride in 84 days, so only that ride is measured).
  A result that is not `measured` never exposes a stream value (`decouplingStreams`/`streamsDecoupling` are null) and is never consumed.
- **Labels**: each session has `decoupling` (the consumed value; a real 0 is kept), `decouplingSource` (`api` or `streams`), `decouplingApi`, `decouplingStreams` and `decouplingStreamsStatus` (`measured`, `not_measurable` or `unavailable`, with the reason in `decouplingNote`). `durability.averageDecouplingApi` and `durability.averageDecouplingStreams` average each source separately when streams mode is on (API value of every ride that has one; stream value of every measured ride) next to the consumed `averageDecoupling`. `durability.decouplingSources`, `history.decouplingSources` and microcycle `fatigueDurability.decouplingSources` count the sources.
- **API volume**: streams mode adds one streams GET per cycling/running activity with HR (4 in parallel) and one events GET for the paired workouts. Results are cached under `~/.icu/cache/decoupling/` (or `$ICU_CONFIG_DIR/cache/decoupling/`), keyed by algorithm version, options, plan trim, moving time and the activity's `analyzed` timestamp, so an edit or reprocess invalidates the entry. Results that are not measured expire after 24 h. Fetch errors are never cached.
- `icu activity <id> decoupling` shows both values and the full breakdown for one activity: trim and its source, HR lag, stop speed, kept/excluded minutes by reason, fences, half means and the VI cutoff, plus a `summary` (status, reason, API vs streams value, trim, lag and r, minutes kept/in range, exclusions by reason, workout kind, VI vs cutoff, powerCv, stopRule).

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

### Testing against the mock

Release candidates and manual tests run only against a local mock of the intervals.icu API, never against a real account. The mock serves anonymized fixtures from `testdata/fixtures` (athlete, profile, sport settings, wellness, events, activities, curves, and streams for `i900000001`, `i900000002`, `i900000003`; ids, gear, event ids, height, weight and timezone are remapped/fuzzed and do not match any real account). It accepts writes (PUT/POST/DELETE/PATCH) in memory only and logs them.

```bash
scripts/rc-mock.sh            # builds and starts the mock on 127.0.0.1, prints PORT, BASE_URL, PID, LOG
scripts/rc-mock.sh 8765       # fixed port
# or in the foreground:
go run ./tools/mockserver -port 8765        # make mock  -> ./icu-mock binary

export INTERVALS_ICU_API_KEY=mock INTERVALS_ICU_ATHLETE_ID=0
icu athlete show --base-url http://127.0.0.1:8765
icu activity i900000002 decoupling --base-url http://127.0.0.1:8765
icu analysis coaching --base-url http://127.0.0.1:8765
icu events get 800000001 --base-url http://127.0.0.1:8765
curl -s http://127.0.0.1:8765/_mock/writes          # {"count":N,"writes":[...]}: what the CLI tried to write
curl -s -X POST http://127.0.0.1:8765/_mock/reset   # clear the write log and in-memory changes
```

Any athlete id maps onto the `0` fixtures; list endpoints honour `oldest`/`newest`. A missing fixture returns 404 and is logged. `-record -upstream URL` (the upstream is required; there is no default) is the one-time, GET-only fixture capture mode (non-GET requests are refused locally); `-anonymize DIR` re-applies the anonymizer (DOB, Strava/device ids, email, city/state/country, names, gear nicknames, tokens/API keys, GPS) to captured fixtures.

## License

MIT
