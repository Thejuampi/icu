---
name: intervals-icu
description: "Use when working with Intervals.icu data through the `icu` CLI: activities, wellness, workouts, events, FTP, athlete profiles, sport settings, routes, gear, chats, training load, cycling, API key setup. Prefer CLI commands over direct REST endpoint work; the CLI owns REST details."
compatibility: opencode, claude
metadata:
  cli: icu
  domain: intervals-icu-cli
  source: project-agent-skill
---

# Intervals.icu CLI Skill

Use this skill when the task is about reading, changing, importing, exporting, or analyzing Intervals.icu data. The default interface is the `icu` CLI. Do not hand-roll REST requests unless the user explicitly asks for API-level work or you are implementing a missing CLI command.

## Operating Principles

- Treat `icu` as the source of truth for authentication, base URL, athlete ID shorthand, request formatting, response parsing, and REST errors.
- Discover exact flags from the installed build with `icu help` and `icu <resource> --help` before assuming a command shape.
- Prefer hyphen-case CLI flags in examples and command entry, such as `--calendar-id`, `--external-id`, and `--route-id`. Treat snake_case as the API naming layer; raw responses and `--fields` values may still use snake_case.
- Use the configured default athlete ID. If none is configured, the CLI falls back to athlete ID `0`; override with `--athlete-id` when needed.
- Prefer narrow reads with date ranges, `--fields`, and resource-specific filters.
- Never print, store, or invent API keys. Use `icu config diagnose` for safe credential checks.
- For destructive or broad writes, confirm intent unless the user already gave explicit instructions.

## Setup And Diagnostics

```bash
# Install or update the CLI
go install github.com/Thejuampi/icu@latest

# One-time local setup
icu config set --api-key YOUR_API_KEY
icu config set --athlete-id YOUR_ATHLETE_ID

# Verify configuration without exposing secrets
icu config diagnose
icu config diagnose --verbose

# Show saved config values that are safe to display
icu config show
```

API key resolution order: `--api-key` flag, then `INTERVALS_ICU_API_KEY`, then `~/.icu/config.json`.

## Athlete ID Resolution

- Run `icu config show` to see the configured default `athlete_id`.
- If `athlete_id` is a non-zero value, use it for every command. Only pass `--athlete-id` when you need a different athlete.
- If `athlete_id` is `0` or absent, ask the user for their athlete ID before running commands that require one.
- Do not ask for an athlete ID when a valid default is already configured.

## Help And Discovery

```bash
icu help
icu --help
icu activities --help
icu wellness --help
icu events --help
icu workouts --help
icu sports --help
```

If CLI behavior is unclear while working in this repo, inspect the matching command file under `cmd/icu/`, such as `cmd_activities.go`, `cmd_wellness.go`, or `cmd_events.go`.

## Common Workflows

### Athlete, FTP, And Sport Settings

```bash
icu athlete show
icu athlete update --weight 81 --height 1.81 --icu-weight 81
icu ftp show
icu ftp update --value 290
icu sports list
icu sports get Ride
```

Use sport settings for FTP, indoor FTP, W prime, p-max, LTHR, max HR, power zones, HR zones, pace zones, threshold pace, and load model settings.

### Weather Config

```bash
icu weather config
icu weather config --lat 40.7282 --lon -73.7949 --label "Queens" --enabled true
icu weather config --enabled false
icu weather forecast
```

The forecast location controls the weather strip shown on the training calendar. `update` is triggered by any flag; the CLI fetches the existing config first and merges in your changes, so `--enabled false` works without re-entering coordinates.

If `icu weather forecast` returns an HTTP 500, the issue is on the Intervals.icu backend (OpenWeatherMap integration). In that case leave `weather-config` empty or report it on the forum; the CLI cannot fix a server-side weather provider failure.

### Activities

```bash
icu activities list --oldest 2026-05-20 --newest 2026-05-24
icu activities list --oldest 2026-05-20 --newest 2026-05-24 --fields id,name,type,moving_time,distance,icu_training_load,icu_intensity
icu activities upload activity.fit --name "Morning Ride"
icu activity i123 show
icu activity i123 show --intervals
icu activity i123 intervals
icu activity i123 streams
```

Use `icu activity <id> show --intervals` when the task needs the full activity payload plus interval data in a single response. Use `icu activity <id> intervals` when only the interval block is needed.

Useful activity fields: `id`, `name`, `start_date_local`, `type`, `moving_time`, `elapsed_time`, `distance`, `total_elevation_gain`, `average_heartrate`, `max_heartrate`, `icu_average_watts`, `icu_weighted_avg_watts`, `calories`, `icu_training_load`, `hr_load`, `hr_load_type`, `trimp`, `icu_intensity`, `icu_ftp`, `icu_pm_cp`, `icu_pm_w_prime`, `icu_pm_p_max`, `icu_rolling_cp`, `icu_rolling_w_prime`, `icu_rolling_ftp`, `decoupling`, `icu_efficiency_factor`, `icu_variability_index`, `icu_power_hr`, `icu_rpe`, `feel`, `perceived_exertion`, `session_rpe`, `compliance`, `average_speed`, `average_temp`, `average_wind_speed`, `headwind_percent`, `tailwind_percent`, `prevailing_wind_deg`, `average_altitude`, `source`, `external_id`, `tags`, `description`, `device_name`, `gear`.

### Power Meter Death / Estimate Power Gaps

When a dual-sided (or other) power meter dies mid-ride, do **not** treat every zero as coasting and do **not** invent load from RPE.

**Null vs zero**

- Coasting (meter alive): `watts=0` and `cadence` present `0` → `true_zero` (do not fill).
- PM death: `watts=0` (or null) and **cadence null** while moving → `missing` (fill candidate).
- Dual-sided PM: `left_right_balance` present while alive (real first half with power + RPM + L/R); long **null L/R tail after the last present balance sample** marks death. Prefer balance over cadence for the death index (mid-ride freewheels null cadence temporarily).

**Standard dry-run → accept**

```bash
# Dry-run (does not mutate). Default types include left_right_balance.
icu activity i123 estimate-power --rider-mass-kg 81 --bike-mass-kg 7.8 --calibrate-from-measured --file fill.json

# Inspect: classification.meterDeathIndex, deathSource, fill.*, metrics, weather warnings
# Accept only after review (supporter feature; mutates watts stream)
icu activity i123 estimate-power-accept --file fill.json
```

**Re-fill after a prior accept** (positive watts already written into the dead half look measured):

```bash
# Prefer: L/R balance death, then cadence end-tail
icu activity i123 estimate-power --rider-mass-kg 81 --bike-mass-kg 7.8 \
  --calibrate-from-measured --refill-after-pm-death --file fill.json
icu activity i123 estimate-power-accept --file fill.json
```

Other refill controls: `--refill-from-index N`, `--refill-after-cadence-death` (cadence-only). First half with real power stays measured for calibration.

**Weather / aero (outdoor only)**

- Automatic for outdoor rides: activity wind/temp/head-tail if present, else free Open-Meteo (archive → forecast past → historical-forecast). No API key.
- Relative wind uses track heading from activity map. Density from pressure/temp when available.
- `--no-weather` skips external fetch (activity fields only). Do not invent wind constants.
- Indoor / `VirtualRide`: not free-air physics; do not use outdoor estimate-power as truth (prefer `hrLoad` load repair when needed).

**Backtest (outdoor replay validation)**

```bash
icu activity i123 estimate-power-backtest --rider-mass-kg 81 --bike-mass-kg 7.8 --calibrate-from-measured
# Outdoor only. Prefer continuous measured PM (no prior fill). Scores: pearsonR, spearmanRho, bias, residual MAD-z, robustRmse.
# In-repo gates: physics ≥0.95 r; outdoor-shaped fixture ≥0.80 r + bias/MAD-z bounds. CFD is not the fidelity path.
# Modes: mask_second_half | mask_after_fraction | mask_scatter
```

**Backup before accept (rollback)**

Accept mutates the Intervals watts stream. Before publishing:

1. Save `icu activity ID streams --types watts,...` (or full streams) to a local backup dir.
2. Keep the dry-run `--file fill.json` that will be accepted.
3. Build a rollback payload: a prior fill file or streams snapshot that can be re-accepted to restore previous watts.
4. Only then run `estimate-power-accept`.

Example restore after a bad publish:

```bash
icu activity i123 estimate-power-accept --file rollback-accept-payload.json
```

**Load after fill**

- Accept writes `watts` only. Intervals reanalysis owns NP/load. If load remains wrong with good HR coverage, use API `hrLoad`/`HRSS` via `activity update --training-load N` only with explicit user approval.

**Strava (and other platforms) after fill**

Intervals stream writes do **not** update Strava power. Strava keeps the original upload (often Wahoo/Garmin with dead-PM zeros). The Strava API cannot rewrite watts on an existing activity.

To fix Strava:

1. Export the **Intervals-processed** FIT (includes modified streams after accept), not the original device file:
   ```bash
   icu activity i123 fit-file > activity-filled.fit
   # Optional reference (original device bytes, pre-edit):
   icu activity i123 file > activity-original.bin
   ```
   On Windows PowerShell prefer `icu activity i123 fit-file` written to a path the CLI supports, or download **Fit File** from the Intervals activity UI (not “Original”).
2. On Strava: **delete** the broken ride, then upload `activity-filled.fit` at https://www.strava.com/upload/select.
3. Avoid Intervals duplicates: if Strava/Wahoo re-import creates a second activity, keep the already-fixed Intervals activity as training truth and delete/merge the duplicate.
4. Optional metadata-only: Intervals settings can sync title/description to linked Strava activities; that does **not** fix power.

### Wellness

```bash
icu wellness list --oldest 2026-05-20 --newest 2026-05-24
icu wellness get 2026-05-24
icu wellness update 2026-05-24 --weight 81 --resting-hr 50
icu wellness bulk --file records.json
```

Useful wellness fields: `id`, `weight`, `restingHR`, `hrv`, `hrvSDNN`, `sleepSecs`, `sleepScore`, `sleepQuality`, `avgSleepingHR`, `readiness`, `soreness`, `fatigue`, `stress`, `mood`, `motivation`, `injury`, `hydration`, `ctl`, `atl`, `rampRate`, `vo2max`, `steps`, `comments`, `locked`.

### Events And Calendar

```bash
icu events list --oldest 2026-05-20 --newest 2026-05-31
icu events list --oldest 2026-05-20 --newest 2026-05-31 --category WORKOUT
icu events create --category WORKOUT --type Ride --name "Morning Intervals" --start-date "2026-05-25T07:00:00" --moving-time 3600 --training-load 90 --desc "- 15m Ramp 55-72% FTP\n- 4x8m 105% FTP\n- 10m Ramp 55-50% FTP"
```

**Prefer `icu plan` for multi-session calendar writes.** Build an intent JSON with explicit sessions, then:

```bash
icu plan show --file plan.json --intent-file intent.json --now-date 2026-07-27 --type Ride
icu plan preview --file plan.json
# inspect/edit plan.json operations if needed
icu plan accept --file plan.json
```

Use single `events create/update` only for one-off edits. Use `icu rebalance` for weekly load redistribution of existing structure (show → optional preview → accept), not for writing an explicit session list. See `docs/plan.md` and `docs/rebalance.md`.

Common event categories: `WORKOUT`, `RACE_A`, `RACE_B`, `RACE_C`, `NOTE`, `PLAN`, `HOLIDAY`, `SICK`, `INJURED`, `SET_EFTP`, `FITNESS_DAYS`, `SEASON_START`, `TARGET`, `SET_FITNESS`.

Practical behavior to remember:

- `WORKOUT` events keep their planned start time and contribute to projected load.
- `NOTE` events behave like all-day calendar notes in Intervals.icu. The date is preserved, but the time-of-day portion of `--start-date` may be normalized away in the stored event.
- Keep `NOTE` descriptions compact. Long planning notes can fail upstream with HTTP 500; split large decision logic across multiple notes when needed.

#### Workout Description Format

`icu events create` sends `description` to Intervals.icu, which parses it into a native `workoutDoc`. The CLI does **not** accept a raw `workoutDoc`; the description is the only way to shape the workout.

Rules learned from real usage:

- **No nested reps.** Intervals.icu does not parse `3x` blocks that contain another `13x` block. Flatten the structure into separate blocks.
- **Dash + space.** Use `- 30s 112-115%`. `-30s ...` (no space) is not recognized as a step.
- **Recovery between blocks must be explicit.** Write each series as its own block followed by the rest interval.
- **Ramps work in both directions.** Use `- 20m Ramp 55-40% FTP` for a descending cooldown ramp and `- 15m Ramp 55-85% FTP` for an ascending warm-up ramp. The word `Ramp` and trailing `FTP` make the parser more reliable.
- **Intervals.icu may recalculate `moving-time` and `training-load`.** Always verify the created event with `icu events get <id>`.

Example: a 3×13×(30s/15s) VO2Max session must be written as three flat `13x` blocks, not as a nested `3x` → `13x`:

```bash
icu events create --category WORKOUT --type Ride --name "3x13 30/15 VO2Max" `
  --start-date "2026-06-16T07:00:00" `
  --moving-time 4455 `
  --training-load 92 `
  --desc "VO2Max 30/15 session.`n`n- 15m Ramp 55-85% FTP`n`n13x`n  - 30s 112-115% FTP`n  - 15s 50% FTP`n- 5m 50-55% FTP`n`n13x`n  - 30s 112-115% FTP`n  - 15s 50% FTP`n- 5m 50-55% FTP`n`n13x`n  - 30s 112-115% FTP`n  - 15s 50% FTP`n`n- 20m Ramp 55-40% FTP"
```

#### Replacing A Planned Workout

When replacing an existing calendar workout, delete the old `WORKOUT` event and any associated `NOTE` alternatives so the calendar does not keep contradictory options. Recreate the primary session as `WORKOUT` so projected CTL/ATL stays accurate.

```bash
icu events list --oldest 2026-06-16 --newest 2026-06-16
icu events delete <old-workout-id>
icu events delete <old-alternative-note-id>
icu events create --category WORKOUT --type Ride --name "New session" ...
```

#### Trainer Platform Sync

Zwift, TrainerRoad, and similar platforms sync planned workouts automatically from Intervals.icu. There is no need to download `.zwo`, `.mrc`, or `.erg` files manually unless the user explicitly asks for an offline copy.

### Flexible Planning With Optional Alternatives

When planning day-by-day decisions based on recovery markers (HRV, resting HR, sleep, readiness), load the primary session as a `WORKOUT` event so Intervals.icu projects CTL/ATL correctly, and add fallback sessions as `NOTE` events so they do not inflate projected load.

```bash
# Primary session: contributes to projected load
icu events create --category WORKOUT --type Ride --name "4x5 VO2Max" --start-date "2026-06-16T07:00:00" --moving-time 4380 --training-load 91 --desc "- 15m Ramp 55-72% FTP\n4x\n  - 5m 112-115% FTP\n  - 4m 60% FTP\n- 22m Ramp 68-55% FTP"

# Alternative session: does NOT contribute to projected load
# Note: the CLI treats the two characters '\n' literally. Use your shell's real newline mechanism
# (e.g. PowerShell backtick-n: `n, bash $'...\n...') so the description renders with line breaks.
icu events create --category NOTE --type Ride --name "ALT B — 90m Z2" --start-date "2026-06-16T07:00:00" --desc "Do if HRV <= baseline, resting HR elevated, or legs flat.`n- 15m 55-70%`n- 60m 68-72%`n- 15m 55-60%"
```

Decision rules commonly used:
- HRV below personal baseline or resting HR elevated → choose the NOTE alternative.
- Sleep score < 70 or high subjective fatigue → choose the NOTE alternative or take the OFF day.
- If the alternative is chosen consistently for several days, consider converting it into the new WORKOUT and deleting/re-scheduling the harder primary session rather than accumulating skipped workouts.

This keeps projected fitness/fatigue curves clean while preserving fallback options in the calendar.

When publishing a full block, prefer one compact block-overview note plus short weekly notes and short `ALT B` notes. Do not rely on a single long NOTE to carry every threshold and branch.

### Workouts And Library

```bash
icu workouts list
icu workouts create --name "Endurance Ride" --type Ride
```

Native workout documents use JSON with `description`, `duration`, `ftp`, `lthr`, `target`, and nested `steps`. Step fields include `text`, `duration`, `distance`, `reps`, `warmup`, `cooldown`, `ramp`, `freeride`, `maxeffort`, `intensity`, `power`, `hr`, `pace`, `cadence`, `hidepower`, and `until_lap_press`.

```json
{
  "description": "Optional description text",
  "duration": 3600,
  "ftp": 285,
  "target": "POWER",
  "steps": [
    {
      "text": "Warm up",
      "duration": 600,
      "warmup": true,
      "ramp": true,
      "power": {"start": 35, "end": 55, "units": "%ftp"},
      "cadence": {"value": 85, "units": "rpm"}
    },
    {
      "text": "4x8m VO2 Max",
      "reps": 4,
      "steps": [
        {"text": "On", "duration": 480, "intensity": "interval", "power": {"value": 110, "units": "%ftp"}},
        {"text": "Off", "duration": 480, "intensity": "rest", "power": {"value": 50, "units": "%ftp"}}
      ]
    },
    {
      "text": "Cool down",
      "duration": 600,
      "cooldown": true,
      "power": {"value": 50, "units": "%ftp"}
    }
  ]
}
```

Workout target value units include `%ftp`, `%lthr`, `%pace`, `%mmp`, `rpm`, `bpm`, `m/s`, and `w`.

### Curves, Routes, Gear, Chats, And Other Resources

```bash
icu curves power --type Ride --curves 42d
icu curves mmp
icu routes list
icu gear list
icu chats list
```

For less common resources, start with `icu <resource> --help` and prefer the documented command over direct API calls.

## Dates, Types, And Filters

- Use ISO dates for date ranges, for example `2026-05-20`.
- Use local timestamps for calendar starts when needed, for example `2026-05-25T07:00:00`.
- Common activity types include `Ride`, `VirtualRide`, `Run`, `TrailRun`, `Swim`, `OpenWaterSwim`, `Walk`, `Hike`, `GravelRide`, `MountainBikeRide`, `WeightTraining`, `Workout`, `Yoga`, and `Other`.
- Use `--fields` on list commands when the task only needs a few values.

## Error Handling

- `401` or `403`: credentials are missing, invalid, expired, or lack permission. Run `icu config diagnose`.
- `404`: the resource does not exist or the athlete does not have access.
- `429`: rate limited. Wait before retrying.
- `500`: Intervals.icu server error. Retry later.
- Strava-backed stub activities can be incomplete and may not support updates.

## When REST Details Matter

Use API documentation only when maintaining the CLI, checking whether a command is missing coverage, or debugging a CLI implementation. For normal data tasks, stay at the CLI layer.

Useful references:

- CLI repo: https://github.com/Thejuampi/icu
- Official Swagger UI: https://intervals.icu/api-docs.html
- API access notes: https://forum.intervals.icu/t/api-access-to-intervals-icu/609
- OAuth notes: https://forum.intervals.icu/t/intervals-icu-oauth-support/2759
