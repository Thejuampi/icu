---
name: intervals-icu
description: Intervals.icu API reference and icu CLI usage. Activities, wellness, workouts, events, FTP, athlete profiles, sport settings, routes, gear, chats. Use for any intervals.icu API task.
when_to_use: intervals.icu, intervals icu, API key, workout, wellness, FTP, training load, cycling, icu CLI, activities, events, athlete
---

# Intervals.icu CLI Skill

Use this skill when the task is about reading, changing, importing, exporting, or analyzing Intervals.icu data. The default interface is the `icu` CLI. Do not hand-roll REST requests unless the user explicitly asks for API-level work or you are implementing a missing CLI command.

## Operating Principles

- Treat `icu` as the source of truth for authentication, base URL, athlete ID shorthand, request formatting, response parsing, and REST errors.
- Discover exact flags from the installed build with `icu help` and `icu <resource> --help` before assuming a command shape.
- Use athlete ID `0` unless the user specifies another athlete.
- Prefer narrow reads with date ranges, `--fields`, and resource-specific filters.
- Never print, store, or invent API keys. Use `icu config diagnose` for safe credential checks.
- For destructive or broad writes, confirm intent unless the user already gave explicit instructions.
- After every write (`events create/update/delete/revert`, `plan accept`, `rebalance accept`, `sports update`, `wellness update`, `activity update`, ...), re-read what you wrote (`icu events get <id>`, `icu events list --oldest D --newest D`, `icu sports get TYPE`, ...) and verify type, date, name, description line breaks, `moving_time` and `icu_training_load` match what you intended. Intervals.icu may recalculate or normalize fields. Report any mismatch; do not assume a write succeeded as sent.
- Put multi-line descriptions in a file and pass `--desc-file PATH`. The CLI rejects literal `\n`, `\r`, `\t` sequences in descriptions. For non-cycling workouts (`WeightTraining`, `Run`, ...) set `--type` and give `--moving-time`/`--training-load`; `--calculate-load` is FTP-based and cycling-only.

## Validate Numbers And Challenge Requests

- Treat CLI analysis output as derived data. Before you state a number that drives a decision (FTP, CTL/ATL/TSB, ramp, weekly load, decoupling, power-curve deltas), check it against the raw API values: `icu sports get Ride` for `ftp`/`indoor_ftp`, `icu wellness list --oldest D --newest D` for daily `ctl`/`atl`, and `icu activities list` / `icu events get <id>` for per-session load. Say which source each number comes from and its date. If the derived and raw numbers disagree, report both and do not guess.
- Use today's wellness for current fitness, not the last activity. Ignore projected future wellness rows.
- Price each session at the right FTP: indoor (`VirtualRide` or `"indoor": true` in the intent) at indoor FTP, outdoor at outdoor FTP. If Intervals.icu has no `indoor_ftp`, the CLI uses the local profile (`icu profile show`) and says so in an `ftp:` warning.
- Challenge athlete requests that do not make sense before you write anything. Examples: a weekly CTL ramp above the athlete's `maxRampPerWeek`, intensity stacked on consecutive days while TSB is already very negative, an intent FTP above the current or target FTP, several identical sessions in one week (`monotony:` warnings), sessions outside the athlete's usual days/times, or a week below their Z2 floor. Explain the concern with the numbers, propose an alternative, and only proceed when the athlete confirms.
- Do not invent thresholds. Use the athlete's profile (`icu profile set ...`) and Intervals.icu settings; when a cap is not configured, say so instead of assuming one.
- Before writing, run `icu plan preview --file plan.json` and read `projection` (end-of-period CTL/ATL/TSB, weekly ramp) and every warning. `plan accept` and `events update` save a local backup of each changed event; if a change was wrong, `icu events revert --id ID --dry-run` and then without `--dry-run` restore it.

## Setup And Diagnostics

```bash
# Install or update the CLI
go install github.com/Thejuampi/icu/cmd/icu@latest

# One-time local setup
icu config set --api-key YOUR_API_KEY

# Verify configuration without exposing secrets
icu config diagnose
icu config diagnose --verbose

# Show saved config values that are safe to display
icu config show
```

API key resolution order: `--api-key` flag, then `INTERVALS_ICU_API_KEY`, then `~/.icu/config.json`.

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

### Activities

```bash
icu activities list --oldest 2026-05-20 --newest 2026-05-24
icu activities list --oldest 2026-05-20 --newest 2026-05-24 --fields id,name,type,moving_time,distance,icu_training_load,icu_intensity
icu activities upload activity.fit --name "Morning Ride"
icu activity i123 show
icu activity i123 intervals
icu activity i123 streams
```

Useful activity fields: `id`, `name`, `start_date_local`, `type`, `moving_time`, `elapsed_time`, `distance`, `total_elevation_gain`, `average_heartrate`, `max_heartrate`, `icu_average_watts`, `icu_weighted_avg_watts`, `calories`, `icu_training_load`, `icu_intensity`, `icu_ftp`, `icu_pm_cp`, `icu_pm_w_prime`, `icu_pm_p_max`, `icu_rolling_cp`, `icu_rolling_w_prime`, `icu_rolling_ftp`, `decoupling`, `icu_efficiency_factor`, `icu_variability_index`, `icu_power_hr`, `icu_rpe`, `feel`, `perceived_exertion`, `session_rpe`, `compliance`, `average_speed`, `average_temp`, `source`, `external_id`, `tags`, `description`, `device_name`, `gear`.

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
icu events create --category WORKOUT --type Ride --name "Morning Intervals" --start-date "2026-05-25T07:00:00" --moving-time 3600 --training-load 90 --desc $'- 20m 60% Z2\n- 4x8m 105%\n- 10m 55% cooldown'
```

Common event categories: `WORKOUT`, `RACE_A`, `RACE_B`, `RACE_C`, `NOTE`, `PLAN`, `HOLIDAY`, `SICK`, `INJURED`, `SET_EFTP`, `FITNESS_DAYS`, `SEASON_START`, `TARGET`, `SET_FITNESS`.

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
