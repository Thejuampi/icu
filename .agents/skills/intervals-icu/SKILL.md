---
name: intervals-icu
description: Use when working with Intervals.icu API data — activities, wellness, workouts, events, FTP, athlete profiles, sport settings, routes, gear, chats. Covers all endpoints via the `icu` CLI (github.com/Thejuampi/icu). Trigger keywords: intervals.icu, intervals icu, API key, workout, wellness, FTP, training load, cycling, icu CLI.
compatibility: opencode
metadata:
  cli: icu
  domain: intervals-icu-api
  source: project-agent-skill
---

# Intervals.icu API Skill

Complete reference for the Intervals.icu REST API. Use the `icu` CLI tool to interact with all endpoints.

## Quick Start

```bash
# One-time setup
icu config set --api-key YOUR_API_KEY

# Use without --api-key from then on
icu athlete show
icu ftp show
icu activities list --oldest 2026-05-20 --newest 2026-05-24
```

API key resolution order: `--api-key` flag → `INTERVALS_ICU_API_KEY` env → `~/.icu/config.json`

## CLI Tool

The `icu` CLI wraps every intervals.icu endpoint (~70 commands). Install:

```bash
go install github.com/Thejuampi/icu@latest
```

### Getting Help

```bash
icu help              # Global help listing all resources
icu --help            # Same as above
icu ftp --help        # Resource-specific help showing all actions with usage and descriptions
icu activities        # Resources without a "show" action default to help
icu config diagnose   # Safe diagnostic output (no secrets)
icu config diagnose --verbose  # Full diagnostic including key fingerprints
```

Repo: https://github.com/Thejuampi/icu

Official docs:
- Swagger UI: https://intervals.icu/api-docs.html
- API access: https://forum.intervals.icu/t/api-access-to-intervals-icu/609
- OAuth: https://forum.intervals.icu/t/intervals-icu-oauth-support/2759
- Integration cookbook: https://forum.intervals.icu/t/intervals-icu-api-integration-cookbook/80090

## Authentication

### API Key (personal use)
Use HTTP Basic Auth. Username = `API_KEY`, Password = your API key from https://intervals.icu/settings.

```
GET /api/v1/athlete/0
Authorization: Basic base64("API_KEY:your-api-key")
```

### OAuth 2.0 (multi-user apps)
1. Send user to: `https://intervals.icu/oauth/authorize?client_id=...&redirect_uri=...&scope=ACTIVITY:READ,WELLNESS:WRITE&state=...`
2. Exchange code for token: `POST https://intervals.icu/api/oauth/token` with `client_id`, `client_secret`, `code`
3. Use token: `Authorization: Bearer <access_token>`

**Scopes:** ACTIVITY, WELLNESS, CALENDAR, CHATS, LIBRARY, SETTINGS — each with :READ or :WRITE suffix.

### Athlete ID
Use `0` in path params as shorthand for the athlete associated with the API key/token. Use a specific athlete ID (e.g., `i445643`) to access another athlete's data (must follow or coach them).

## Base URL
```
https://intervals.icu
```

## API Reference by Category

### Athletes

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}` | Get athlete with sportSettings and custom_items |
| PUT | `/api/v1/athlete/{id}` | Update athlete profile |
| GET | `/api/v1/athlete/{id}/profile` | Get athlete profile info |
| GET | `/api/v1/athlete/{id}/athlete-summary{ext}` | Summary info for followed athletes |
| GET | `/api/v1/athlete/{id}/settings/{deviceClass}` | Get settings for phone/tablet/desktop |
| GET | `/api/v1/athlete/{id}/training-plan` | Get athlete's training plan |
| PUT | `/api/v1/athlete/{id}/training-plan` | Change athlete's training plan |
| PUT | `/api/v1/athlete-plans` | Change training plans for multiple athletes |

**Update athlete profile:**
```bash
icu athlete update --weight 81 --height 1.81 --icu-weight 81
```

### Activities (Completed)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/activities` | List activities for date range (desc order) |
| POST | `/api/v1/athlete/{id}/activities` | Upload activity file (fit/tcx/gpx/zip/gz) |
| GET | `/api/v1/athlete/{id}/activities.csv` | Download activities as CSV |
| GET | `/api/v1/activity/{id}` | Get single activity (add `?intervals=true`) |
| PUT | `/api/v1/activity/{id}` | Update activity |
| DELETE | `/api/v1/activity/{id}` | Delete activity |
| POST | `/api/v1/athlete/{id}/activities/manual` | Create manual activity |
| POST | `/api/v1/athlete/{id}/activities/manual/bulk` | Create multiple manual activities (upsert on external_id) |
| GET | `/api/v1/athlete/{athleteId}/activities/{ids}` | Fetch multiple activities by ID |
| GET | `/api/v1/athlete/{id}/activities-around` | Activities before/after another activity |
| GET | `/api/v1/athlete/{id}/activities/search` | Search activities by name or tag |
| GET | `/api/v1/athlete/{id}/activities/search-full` | Search activities returning full objects |
| GET | `/api/v1/athlete/{id}/activities/interval-search` | Find activities with matching intervals |

**Key query params for list activities:**
- `oldest` / `newest` — ISO-8601 dates (e.g., `2026-05-20`)
- `fields` — comma-separated field names to return
- `limit` — max results
- `route_id` — filter by route

**List recent activities:**
```bash
icu activities list --oldest 2026-05-20 --newest 2026-05-24 --fields id,name,type,moving_time,distance,icu_training_load,icu_intensity
```

**Upload activity file:**
```bash
icu activities upload activity.fit --name "Morning Ride"
```

### Activity Details

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/activity/{id}/intervals` | Get detected intervals with groups |
| PUT | `/api/v1/activity/{id}/intervals` | Update intervals |
| PUT | `/api/v1/activity/{id}/intervals/{intervalId}` | Update/create single interval |
| PUT | `/api/v1/activity/{id}/delete-intervals` | Delete intervals |
| PUT | `/api/v1/activity/{id}/split-interval` | Split an interval |
| GET | `/api/v1/activity/{id}/streams{ext}` | Get streams (power, HR, cadence, etc.) |
| PUT | `/api/v1/activity/{id}/streams` | Update streams from JSON |
| PUT | `/api/v1/activity/{id}/streams.csv` | Update streams from CSV |
| GET | `/api/v1/activity/{id}/power-curve{ext}` | Activity power curve |
| GET | `/api/v1/activity/{id}/power-curves{ext}` | Activity power curves for streams |
| GET | `/api/v1/activity/{id}/hr-curve{ext}` | Activity HR curve |
| GET | `/api/v1/activity/{id}/pace-curve{ext}` | Activity pace curve |
| GET | `/api/v1/activity/{id}/power-vs-hr{ext}` | Power vs HR data |
| GET | `/api/v1/activity/{id}/power-histogram` | Power histogram |
| GET | `/api/v1/activity/{id}/hr-histogram` | HR histogram |
| GET | `/api/v1/activity/{id}/pace-histogram` | Pace histogram |
| GET | `/api/v1/activity/{id}/gap-histogram` | Gradient adjusted pace histogram |
| GET | `/api/v1/activity/{id}/best-efforts` | Find best efforts |
| GET | `/api/v1/activity/{id}/interval-stats` | Stats for part of activity |
| GET | `/api/v1/activity/{id}/map` | Map data |
| GET | `/api/v1/activity/{id}/segments` | Activity segments |
| GET | `/api/v1/activity/{id}/hr-load-model` | HR training load model |
| GET | `/api/v1/activity/{id}/power-spike-model` | Power spike detection model |
| GET | `/api/v1/activity/{id}/time-at-hr` | Time at heart rate data |
| GET | `/api/v1/activity/{id}/weather-summary` | Weather summary |
| GET | `/api/v1/activity/{id}/file` | Download original activity file |
| GET | `/api/v1/activity/{id}/fit-file` | Download generated FIT file |
| GET | `/api/v1/activity/{id}/gpx-file` | Download generated GPX file |
| POST | `/api/v1/athlete/{id}/download-fit-files` | Download zip of generated fit files |

### Athlete Curves (Aggregated)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/power-curves{ext}` | Best power curves for athlete |
| GET | `/api/v1/athlete/{id}/hr-curves{ext}` | Best HR curves for athlete |
| GET | `/api/v1/athlete/{id}/pace-curves{ext}` | Best pace curves for athlete |
| GET | `/api/v1/athlete/{id}/activity-power-curves{ext}` | Best power for matching activities |
| GET | `/api/v1/athlete/{id}/activity-hr-curves{ext}` | Best HR for matching activities |
| GET | `/api/v1/athlete/{id}/activity-pace-curves{ext}` | Best pace for matching activities |
| GET | `/api/v1/athlete/{id}/power-hr-curve` | Power vs HR curve |
| GET | `/api/v1/athlete/{id}/power-hr-curve` | Power vs HR curve for date range |
| GET | `/api/v1/athlete/{id}/mmp-model` | Power model for %MMP resolution |

**Curve types:** `1y`, `2y`, `42d`, `s0` (current season), `s1` (prev season), `all`, `r.2023-10-01.2023-10-31` (date range). Add `-kj0` or `-kj1` suffix for fatigued curves.

### Wellness

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/wellness{ext}` | List wellness records (use `.csv` for CSV) |
| GET | `/api/v1/athlete/{id}/wellness/{date}` | Get wellness record for date |
| PUT | `/api/v1/athlete/{id}/wellness/{date}` | Update wellness record for date |
| PUT | `/api/v1/athlete/{id}/wellness` | Update wellness record by id |
| POST | `/api/v1/athlete/{id}/wellness` | Upload wellness CSV |
| PUT | `/api/v1/athlete/{id}/wellness-bulk` | Bulk update wellness records |

**Wellness fields:** `id` (ISO-8601 date), `weight`, `restingHR`, `hrv`, `hrvSDNN`, `sleepSecs`, `sleepScore`, `sleepQuality`, `avgSleepingHR`, `readiness`, `baevskySI`, `spO2`, `systolic`, `diastolic`, `kcalConsumed`, `soreness`, `fatigue`, `stress`, `mood`, `motivation`, `injury`, `hydration`, `hydrationVolume`, `bloodGlucose`, `lactate`, `bodyFat`, `abdomen`, `vo2max`, `steps`, `respiration`, `comments`, `ctl`, `atl`, `rampRate`, `sportInfo[]`, `locked`, `carbohydrates`, `protein`, `fatTotal`, `menstrualPhase`.

**Bulk update wellness:**
```bash
# Create a JSON file with records, then:
icu wellness bulk --file records.json
```

Or update a single date:
```bash
icu wellness update 2026-05-24 --weight 81 --resting-hr 50
```

### Events / Calendar

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/events{format}` | List calendar events (.csv for CSV) |
| POST | `/api/v1/athlete/{id}/events` | Create event |
| POST | `/api/v1/athlete/{id}/events/bulk` | Create multiple events |
| GET | `/api/v1/athlete/{id}/events/{eventId}` | Get event by ID |
| PUT | `/api/v1/athlete/{id}/events/{eventId}` | Update event |
| DELETE | `/api/v1/athlete/{id}/events/{eventId}` | Delete event |
| PUT | `/api/v1/athlete/{id}/events` | Update events in date range |
| DELETE | `/api/v1/athlete/{id}/events` | Delete events in range |
| PUT | `/api/v1/athlete/{id}/events/bulk-delete` | Bulk delete by id/external_id |
| POST | `/api/v1/athlete/{id}/events/apply-plan` | Apply plan to calendar |
| POST | `/api/v1/athlete/{id}/events/{eventId}/mark-done` | Create manual activity matching workout |
| POST | `/api/v1/athlete/{id}/duplicate-events` | Duplicate events |
| GET | `/api/v1/athlete/{id}/events/{eventId}/download{ext}` | Download workout (zwo/mrc/erg/fit) |
| GET | `/api/v1/athlete/{id}/workouts.zip` | Download multiple workouts as zip |
| GET | `/api/v1/athlete/{id}/fitness-model-events` | Fitness model events (FITNESS_DAYS, SET_FITNESS, SET_EFTP) |
| GET | `/api/v1/athlete/{id}/event-tags` | List event tags |
| GET | `/api/v1/shared-event/{id}` | Get shared event |

**Event categories:** WORKOUT, RACE_A, RACE_B, RACE_C, NOTE, PLAN, HOLIDAY, SICK, INJURED, SET_EFTP, FITNESS_DAYS, SEASON_START, TARGET, SET_FITNESS.

**List events query params:**
- `oldest` / `newest` — ISO-8601 dates
- `category` — comma-separated (e.g., `WORKOUT`)
- `ext` — convert workouts to `zwo`, `mrc`, `erg`, or `fit`
- `resolve=true` — resolve %FTP to watts
- `limit` — max events

**Create workout event:**
```bash
icu events create \
  --category WORKOUT --type Ride --name "Morning Intervals" \
  --start-date "2026-05-25T07:00:00" --moving-time 3600 --training-load 90 \
  --desc "- 20m 60% Z2\n- 4x8m 105%\n- 10m 55% cooldown"
```

### Workout Library

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/workouts` | List all workouts in library |
| POST | `/api/v1/athlete/{id}/workouts` | Create workout |
| POST | `/api/v1/athlete/{id}/workouts/bulk` | Create multiple workouts |
| GET | `/api/v1/athlete/{id}/workouts/{workoutId}` | Get workout |
| PUT | `/api/v1/athlete/{id}/workouts/{workoutId}` | Update workout |
| DELETE | `/api/v1/athlete/{id}/workouts/{workoutId}` | Delete workout |
| GET | `/api/v1/athlete/{id}/workout-tags` | List workout tags |
| POST | `/api/v1/athlete/{id}/duplicate-workouts` | Duplicate workouts on plan |
| POST | `/api/v1/download-workout{ext}` | Convert workout to zwo/mrc/erg/fit |
| POST | `/api/v1/athlete/{id}/download-workout{ext}` | Convert workout with athlete settings |

### Folders & Plans

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/folders` | List folders, plans, and workouts |
| POST | `/api/v1/athlete/{id}/folders` | Create folder or plan |
| PUT | `/api/v1/athlete/{id}/folders/{folderId}` | Update folder or plan |
| DELETE | `/api/v1/athlete/{id}/folders/{folderId}` | Delete folder/plan including workouts |
| GET | `/api/v1/athlete/{id}/folders/{folderId}/shared-with` | List shared athletes |
| PUT | `/api/v1/athlete/{id}/folders/{folderId}/shared-with` | Update sharing |
| PUT | `/api/v1/athlete/{id}/folders/{folderId}/workouts` | Update range of plan workouts |
| PUT | `/api/v1/athlete/{id}/apply-plan-changes` | Apply plan changes to calendar |
| POST | `/api/v1/athlete/{id}/folders/{folderId}/import-workout` | Import workout file into folder |

### Sport Settings

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{athleteId}/sport-settings` | List all sport settings |
| POST | `/api/v1/athlete/{athleteId}/sport-settings` | Create sport settings |
| PUT | `/api/v1/athlete/{athleteId}/sport-settings` | Update multiple settings |
| GET | `/api/v1/athlete/{athleteId}/sport-settings/{id}` | Get settings by ID or type name |
| PUT | `/api/v1/athlete/{athleteId}/sport-settings/{id}` | Update settings |
| DELETE | `/api/v1/athlete/{athleteId}/sport-settings/{id}` | Delete settings |
| PUT | `/api/v1/athlete/{athleteId}/sport-settings/{id}/apply` | Apply zones to matching activities |
| GET | `/api/v1/athlete/{athleteId}/sport-settings/{id}/matching-activities` | List matching activities |
| GET | `/api/v1/athlete/{athleteId}/sport-settings/{id}/pace_distances` | Pace curve distances |
| GET | `/api/v1/pace_distances` | List pace curve distances |

**Key sport settings fields:** `types[]`, `ftp`, `indoor_ftp`, `w_prime`, `p_max`, `lthr`, `max_hr`, `power_zones[]`, `hr_zones[]`, `pace_zones[]`, `threshold_pace`, `pace_units`, `gap_model`, `hr_load_type`, `pace_load_type`.

**Update FTP for cycling:**
```bash
icu ftp update --value 290
```

### Routes

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/routes` | List routes with activity counts |
| GET | `/api/v1/athlete/{id}/routes/{route_id}` | Get route |
| PUT | `/api/v1/athlete/{id}/routes/{route_id}` | Update route |
| GET | `/api/v1/athlete/{id}/routes/{route_id}/similarity/{other_id}` | Check route similarity |

### Gear

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/gear{ext}` | List gear (.csv for CSV) |
| POST | `/api/v1/athlete/{id}/gear` | Create gear/component |
| PUT | `/api/v1/athlete/{id}/gear/{gearId}` | Update gear |
| DELETE | `/api/v1/athlete/{id}/gear/{gearId}` | Delete gear |
| GET | `/api/v1/athlete/{id}/gear/{gearId}/calc` | Recalculate gear stats |
| POST | `/api/v1/athlete/{id}/gear/{gearId}/replace` | Retire and replace component |
| POST | `/api/v1/athlete/{id}/gear/{gearId}/reminder` | Create reminder |
| PUT | `/api/v1/athlete/{id}/gear/{gearId}/reminder/{reminderId}` | Update reminder |
| DELETE | `/api/v1/athlete/{id}/gear/{gearId}/reminder/{reminderId}` | Delete reminder |

### Chats & Messages

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/chats` | List chats for athlete |
| GET | `/api/v1/chats/{id}` | Get chat by ID |
| GET | `/api/v1/chats/{id}/messages` | List messages |
| POST | `/api/v1/chats/send-message` | Send message |
| PUT | `/api/v1/chats/{id}/messages/{msgId}` | Update message |
| PUT | `/api/v1/chats/{id}/messages/{msgId}/seen` | Update last seen |
| DELETE | `/api/v1/chats/{id}/messages/{msgId}` | Delete message |
| GET | `/api/v1/activity/{id}/messages` | List activity comments |
| POST | `/api/v1/activity/{id}/messages` | Add activity comment |

### Custom Items

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/custom-item` | List custom items |
| POST | `/api/v1/athlete/{id}/custom-item` | Create custom item |
| GET | `/api/v1/athlete/{id}/custom-item/{itemId}` | Get custom item |
| PUT | `/api/v1/athlete/{id}/custom-item/{itemId}` | Update custom item |
| DELETE | `/api/v1/athlete/{id}/custom-item/{itemId}` | Delete custom item |
| POST | `/api/v1/athlete/{id}/custom-item/{itemId}/image` | Upload image |
| PUT | `/api/v1/athlete/{id}/custom-item-indexes` | Re-order custom items |

### Weather

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/weather-config` | Get weather forecast config |
| PUT | `/api/v1/athlete/{id}/weather-config` | Update weather forecast config |
| GET | `/api/v1/athlete/{id}/weather-forecast` | Get weather forecast |

### Tags

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/athlete/{id}/activity-tags` | List activity tags |
| GET | `/api/v1/athlete/{id}/event-tags` | List event tags |
| GET | `/api/v1/athlete/{id}/workout-tags` | List workout tags |

### Other

| Method | Endpoint | Description |
|--------|----------|-------------|
| DELETE | `/api/v1/disconnect-app` | Disconnect OAuth app from athlete |

## Activity Types (enum)

Ride, Run, Swim, WeightTraining, Hike, Walk, AlpineSki, BackcountrySki, Badminton, Canoeing, Crossfit, EBikeRide, EMountainBikeRide, Elliptical, Golf, GravelRide, TrackRide, Handcycle, HighIntensityIntervalTraining, Hockey, IceSkate, InlineSkate, Kayaking, Kitesurf, MountainBikeRide, Cyclocross, NordicSki, OpenWaterSwim, Padel, Pilates, Pickleball, Racquetball, Rugby, RockClimbing, RollerSki, Rowing, Sail, Skateboard, Snowboard, Snowshoe, Soccer, Squash, StairStepper, StandUpPaddling, Surfing, TableTennis, Tennis, TrailRun, Transition, Velomobile, VirtualRide, VirtualRow, VirtualRun, VirtualSki, WaterSport, Wheelchair, Windsurf, Workout, Yoga, Other.

## Activity Fields Reference

Common fields to request via `?fields=` parameter: `id`, `name`, `start_date_local`, `type`, `moving_time`, `elapsed_time`, `distance`, `total_elevation_gain`, `average_heartrate`, `max_heartrate`, `icu_average_watts`, `icu_weighted_avg_watts`, `calories`, `icu_training_load`, `icu_intensity`, `icu_ftp`, `icu_pm_cp`, `icu_pm_w_prime`, `icu_pm_p_max`, `icu_pm_ftp`, `icu_rolling_cp`, `icu_rolling_w_prime`, `icu_rolling_ftp`, `icu_joules_above_ftp`, `icu_max_wbal_depletion`, `decoupling`, `icu_efficiency_factor`, `icu_variability_index`, `icu_power_hr`, `icu_power_hr_z2`, `icu_power_hr_z2_mins`, `icu_cadence_z2`, `average_cadence`, `icu_rpe`, `feel`, `perceived_exertion`, `session_rpe`, `compliance`, `average_speed`, `max_speed`, `average_temp`, `average_weather_temp`, `average_feels_like`, `strain_score`, `icu_zone_times`, `icu_hr_zone_times`, `pace_zone_times`, `gap_zone_times`, `source`, `strava_id`, `external_id`, `tags`, `description`, `device_name`, `gear`, `athlete_max_hr`, `pace`, `threshold_pace`.

## Workout doc format (native Intervals.icu)

```json
{
  "description": "Optional description text",
  "duration": 3600,
  "ftp": 285,
  "lthr": 178,
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
        {"text": "On", "duration": 480, "intensity": "interval",
         "power": {"value": 110, "units": "%ftp"}},
        {"text": "Off", "duration": 480, "intensity": "rest",
         "power": {"value": 50, "units": "%ftp"}}
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

**Step fields:** `text`, `duration` (seconds), `distance` (meters), `reps`, `warmup`, `cooldown`, `ramp`, `freeride`, `maxeffort`, `intensity` (active/rest/warmup/cooldown/recovery/interval/other), `power`, `hr`, `pace`, `cadence`, `hidepower`, `until_lap_press`.

**Value fields:** `value`, `start`, `end`, `units` (%ftp, %lthr, %pace, %mmp, rpm, bpm, m/s, w), `target` (for dual-target steps).

## CLI Command Reference

See `icu help` for the full list. Use `icu <resource> --help` for resource-specific usage. Key commands:

| Category | Example |
|----------|---------|
| Athlete | `icu athlete show`, `icu athlete update --weight 81` |
| Activities | `icu activities list --oldest DATE --newest DATE` |
| Activity detail | `icu activity i123 show`, `icu activity i123 intervals` |
| Wellness | `icu wellness get 2026-05-24`, `icu wellness list --oldest DATE` |
| Events | `icu events list --oldest DATE`, `icu events create --category WORKOUT ...` |
| Workouts | `icu workouts list`, `icu workouts create --name NAME --type Ride` |
| FTP | `icu ftp show`, `icu ftp update --value 290` |
| Sports | `icu sports list`, `icu sports get Ride` |
| Curves | `icu curves power --type Ride --curves 42d`, `icu curves mmp` |
| Config | `icu config show`, `icu config set --api-key KEY`, `icu config diagnose [--verbose]` |

## Error Handling

- **401/403:** Invalid or expired API key/token. Check credentials.
- **404:** Resource not found or you don't have permission.
- **429:** Rate limited. Wait and retry.
- **500:** Server error. Try again later.
- Strava activities return empty stub objects; cannot be updated.
- Use `locked: true` in wellness updates to prevent provider sync from overwriting API changes.
