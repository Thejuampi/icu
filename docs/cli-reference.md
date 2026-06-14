# CLI Reference

This document describes the current public CLI surface implemented in `cmd/icu/`.
It is intentionally based on shipped behavior, including a few help-text quirks and exact flag spellings.

## Global Behavior

- Auth is required for every resource except `config`.
- Global help is available through `icu help`, `icu --help`, and `icu -h`.
- A bare resource defaults to `show` if that resource has a `show` action, for example `icu athlete`.
- A bare resource without `show` prints resource help, for example `icu activities`.
- `activity` and `shared-event` support id-first parsing. Example: `icu activity i123 show`.
- Long flags are parsed literally. If a command expects `--calendar_id`, `--minSecs`, or `--external_id`, that exact spelling matters.
- Download commands return raw bytes, not wrapped JSON.
- The global `--output` preference exists in config resolution, but current commands mostly emit JSON regardless of that preference.

## Global Flags

- `--api-key KEY`: overrides all other API key sources
- `--athlete-id ID`: overrides all other athlete ID sources
- `--output json|csv|table`: persisted and resolved, but not broadly wired into current command renderers

## Help Quirks

- Some generated `activity` subcommand help strings currently print `icu.Activity <id> ...`.
  The real invocation is always `icu activity <id> ...`.
- `athlete plan` and `weather config` help show an optional literal `update` token.
  The implementation does not require it.

## activities

- `list`
  Invocation: `icu activities list --oldest DATE --newest DATE [--fields f1,f2] [--limit N] [--route_id ID]`
  Notes: `--route_id` is supported by the implementation even though it is omitted from help text.
  Example: `icu activities list --oldest 2026-05-01 --newest 2026-05-31 --limit 50`

- `get`
  Invocation: `icu activities get <id1> [id2 ...]`
  Notes: multiple IDs are joined into a single request path.
  Example: `icu activities get i123 i456`

- `upload`
  Invocation: `icu activities upload <file> [--name NAME] [--description DESC] [--external_id ID] [--device_name NAME] [--paired_event_id ID]`
  Notes: current code expects `--description` and underscore spellings such as `--external_id`; help text still mentions `--desc` and `--external-id`.
  Example: `icu activities upload ride.fit --name "Morning Ride" --external_id garmin-123`

- `csv`
  Invocation: `icu activities csv`
  Notes: returns raw CSV bytes from the API.
  Example: `icu activities csv > activities.csv`

- `search`
  Invocation: `icu activities search <query> [--limit N]`
  Notes: searches by name or `#tag`.
  Example: `icu activities search "#vo2" --limit 20`

- `search-full`
  Invocation: `icu activities search-full <query> [--limit N]`
  Notes: returns full activity objects instead of search summaries.
  Example: `icu activities search-full "group ride"`

- `interval-search`
  Invocation: `icu activities interval-search --minSecs N --maxSecs N --minIntensity N --maxIntensity N [--type auto|power|hr|pace] [--minReps N] [--maxReps N] [--limit N]`
  Notes: the implementation expects camelCase flag names, not the kebab-case spellings shown in help.
  Example: `icu activities interval-search --minSecs 180 --maxSecs 480 --minIntensity 90 --maxIntensity 120`

- `around`
  Invocation: `icu activities around <activity-id> [--limit N] [--route-id ID]`
  Notes: `--route-id` is converted to the API query `route_id`.
  Example: `icu activities around i123 --limit 10`

- `manual`
  Invocation: `icu activities manual --type Ride --name NAME --moving-time SECS [--distance M] [--training-load N] [--start-date DATE]`
  Notes: `--start-date` is supported even though it is omitted from help.
  Example: `icu activities manual --type Ride --name "Trainer Z2" --moving-time 5400 --distance 42000 --training-load 65`

## activity

Invoke these commands as `icu activity <id> <action>`.

- `show`
  Invocation: `icu activity <id> show [--intervals]`
  Notes: `--intervals` is accepted by help/parse flow but is not currently used by the handler.
  Example: `icu activity i123 show`

- `update`
  Invocation: `icu activity <id> update --name NAME [--description DESC] [--type Ride]`
  Notes: current code expects `--description`; help text says `--desc`.
  Example: `icu activity i123 update --name "Afternoon Ride" --description "Updated title"`

- `delete`
  Invocation: `icu activity <id> delete`
  Example: `icu activity i123 delete`

- `intervals`
  Invocation: `icu activity <id> intervals`
  Example: `icu activity i123 intervals`

- `streams`
  Invocation: `icu activity <id> streams [--types watts,heartrate,cadence]`
  Example: `icu activity i123 streams --types watts,heartrate`

- `power-vs-hr`
  Invocation: `icu activity <id> power-vs-hr`
  Example: `icu activity i123 power-vs-hr`

- `power-curve`
  Invocation: `icu activity <id> power-curve`
  Example: `icu activity i123 power-curve`

- `hr-curve`
  Invocation: `icu activity <id> hr-curve`
  Example: `icu activity i123 hr-curve`

- `pace-curve`
  Invocation: `icu activity <id> pace-curve`
  Example: `icu activity i123 pace-curve`

- `best-efforts`
  Invocation: `icu activity <id> best-efforts [--stream STREAM] [--duration N] [--distance N] [--count N]`
  Example: `icu activity i123 best-efforts --duration 300 --count 5`

- `map`
  Invocation: `icu activity <id> map [--bounds BOUNDS] [--weather]`
  Example: `icu activity i123 map --weather`

- `weather-summary`
  Invocation: `icu activity <id> weather-summary [--descr_config VALUE]`
  Notes: the supported flag name uses an underscore.
  Example: `icu activity i123 weather-summary`

- `weather`
  Invocation: `icu activity <id> weather`
  Notes: alias for `weather-summary`.
  Example: `icu activity i123 weather`

- `segments`
  Invocation: `icu activity <id> segments`
  Example: `icu activity i123 segments`

- `messages`
  Invocation: `icu activity <id> messages`
  Example: `icu activity i123 messages`

- `file`
  Invocation: `icu activity <id> file`
  Notes: returns raw bytes.
  Example: `icu activity i123 file > activity.bin`

- `fit-file`
  Invocation: `icu activity <id> fit-file`
  Notes: returns raw bytes.
  Example: `icu activity i123 fit-file > activity.fit`

- `gpx-file`
  Invocation: `icu activity <id> gpx-file`
  Notes: returns raw bytes.
  Example: `icu activity i123 gpx-file > activity.gpx`

## analysis

- `cycling`
  Invocation: `icu analysis cycling [--oldest DATE --newest DATE | --days N] [--fields CSV] [--limit N]`
  Defaults: `--days 28`
  Notes: `--fields` overrides the built-in activity field contract used by the analyzer.
  Example: `icu analysis cycling --days 42`

- `wellness`
  Invocation: `icu analysis wellness [--oldest DATE --newest DATE | --days N] [--fields CSV]`
  Defaults: `--days 28`
  Example: `icu analysis wellness --oldest 2026-04-20 --newest 2026-05-31`

- `plan`
  Invocation: `icu analysis plan [--history-oldest DATE --history-newest DATE] [--plan-oldest DATE --plan-newest DATE] [--history-days N] [--plan-days N] [--sport-type TYPE] [--calendar_id ID] [--resolve] [--activity-fields CSV]`
  Defaults: `--history-days 84`, `--plan-days 28`, `--sport-type Ride`
  Notes: `--calendar_id` uses an underscore; explicit history and plan ranges must be provided in pairs.
  Example: `icu analysis plan --history-days 84 --plan-days 28 --resolve`

- `adaptation`
  Invocation: `icu analysis adaptation [--oldest DATE --newest DATE | --days N] [--type Ride] [--curves 42d,365d] [--filters FILTERS] [--newest DATE] [--limit N] [--activity-fields CSV]`
  Defaults: `--days 28`, `--type Ride`, `--curves 42d,365d`
  Notes: `--newest` applies to the power-curve fetch; `--limit` applies to activity history fetch.
  Example: `icu analysis adaptation --days 42 --type Ride --curves 42d,365d`

- `microcycle`
  Invocation: `icu analysis microcycle [--date DATE | --week DATE | --from DATE --to DATE] [--json] [--full] [--no-plan] [--no-wellness] [--sport-type TYPE] [--timezone TZ]`
  Defaults: current Monday-Sunday microcycle, `--sport-type Ride`, system timezone fallback when `--timezone` is omitted.
  Notes: Experimental read-only diagnostic contract for LLM/skill consumers. Reads activities with a 90-day lookback, planned events, wellness, and sport settings. JSON is the primary contract; human output is a brief inspection view.
  Example: `icu analysis microcycle --from 2026-06-08 --to 2026-06-14 --json`

- `micro`
  Invocation: `icu analysis micro [--date DATE | --week DATE | --from DATE --to DATE] [--json] [--full] [--no-plan] [--no-wellness] [--sport-type TYPE] [--timezone TZ]`
  Notes: Alias for `analysis microcycle`. This replaces the former experimental per-activity micro-analysis command.
  Example: `icu analysis micro --json`

## athlete

- `show`
  Invocation: `icu athlete show`
  Example: `icu athlete show`

- `update`
  Invocation: `icu athlete update [--weight KG] [--icu-weight KG] [--height METERS] [--name NAME] [--bio TEXT]`
  Notes: fields are optional partial updates.
  Example: `icu athlete update --weight 81.2 --height 1.81`

- `profile`
  Invocation: `icu athlete profile`
  Example: `icu athlete profile`

- `summary`
  Invocation: `icu athlete summary [--start DATE] [--end DATE]`
  Example: `icu athlete summary --start 2026-05-01 --end 2026-05-31`

- `plan`
  Invocation: `icu athlete plan` or `icu athlete plan --plan-id ID [--start-date DATE]`
  Notes: help shows an optional `update` token, but the implementation switches modes based on `--plan-id`.
  Example: `icu athlete plan --plan-id 7 --start-date 2026-06-01`

- `settings`
  Invocation: `icu athlete settings [--device desktop|phone|tablet]`
  Defaults: `desktop`
  Example: `icu athlete settings --device phone`

## chats

- `list`
  Invocation: `icu chats list`
  Example: `icu chats list`

- `get`
  Invocation: `icu chats get <id>`
  Example: `icu chats get 42`

- `messages`
  Invocation: `icu chats messages <id> [--limit N]`
  Example: `icu chats messages 42 --limit 100`

- `send`
  Invocation: `icu chats send --content MSG [--to ATHLETE_ID]`
  Example: `icu chats send --content "See you at 6am" --to i445643`

## config

- `show`
  Invocation: `icu config show`
  Notes: prints masked config values and defaults.
  Example: `icu config show`

- `set`
  Invocation: `icu config set [--api-key KEY] [--athlete-id ID] [--output json|csv|table] [--zepp-login-token TOKEN]`
  Notes: current code supports partial updates even though help text foregrounds `--api-key`.
  Example: `icu config set --api-key secret --athlete-id 0`

- `path`
  Invocation: `icu config path`
  Example: `icu config path`

- `diagnose`
  Invocation: `icu config diagnose [--verbose]`
  Notes: does not require auth.
  Example: `icu config diagnose --verbose`

## curves

- `power`
  Invocation: `icu curves power [--type Ride] [--curves 1y|42d|s0] [--newest DATE] [--filters FILTERS]`
  Defaults: `Ride`
  Example: `icu curves power --type Ride --curves 42d,365d`

- `hr`
  Invocation: `icu curves hr [--type Ride] [--curves CURVES]`
  Defaults: `Ride`
  Example: `icu curves hr --type Ride`

- `pace`
  Invocation: `icu curves pace [--type Run] [--curves CURVES]`
  Defaults: `Run`
  Example: `icu curves pace --type Run`

- `power-hr`
  Invocation: `icu curves power-hr --start DATE --end DATE`
  Example: `icu curves power-hr --start 2026-05-01 --end 2026-05-31`

- `mmp`
  Invocation: `icu curves mmp [--type Ride]`
  Defaults: `Ride`
  Example: `icu curves mmp --type Ride`

## custom

- `list`
  Invocation: `icu custom list`
  Example: `icu custom list`

- `get`
  Invocation: `icu custom get <id>`
  Example: `icu custom get 1`

- `create`
  Invocation: `icu custom create --name NAME --type TYPE`
  Example: `icu custom create --name "Readiness" --type FITNESS_CHART`

- `update`
  Invocation: `icu custom update <id> [--name NAME] [--type TYPE]`
  Example: `icu custom update 1 --name "Readiness v2"`

- `delete`
  Invocation: `icu custom delete <id>`
  Example: `icu custom delete 1`

## events

- `list`
  Invocation: `icu events list --oldest DATE --newest DATE [--category WORKOUT] [--ext zwo|mrc|erg|fit] [--limit N] [--calendar_id ID] [--resolve]`
  Notes: `--calendar_id` uses an underscore and is omitted from help text.
  Example: `icu events list --oldest 2026-06-01 --newest 2026-06-28 --category WORKOUT --resolve`

- `get`
  Invocation: `icu events get <id>`
  Example: `icu events get 123`

- `create`
  Invocation: `icu events create [--category WORKOUT] [--type Ride] --name NAME --start-date DATE [--moving-time SECS] [--training-load N] [--desc DESC] [--color VALUE] [--indoor] [--external-id ID] [--upsert]`
  Defaults: `--category WORKOUT`, `--type Ride`
  Example: `icu events create --name "Threshold" --start-date 2026-06-03 --training-load 90 --indoor`

- `update`
  Invocation: `icu events update <id> [--name NAME] [--desc DESC] [--training-load N]`
  Example: `icu events update 123 --name "Threshold 4x8" --training-load 92`

- `delete`
  Invocation: `icu events delete <id>`
  Example: `icu events delete 123`

- `download`
  Invocation: `icu events download <id> [--ext zwo|mrc|erg|fit]`
  Defaults: `zwo`
  Notes: help text presents `--ext` as required, but the implementation defaults it to `zwo`.
  Example: `icu events download 123 --ext zwo > workout.zwo`

- `tags`
  Invocation: `icu events tags`
  Example: `icu events tags`

## fitness-events

- `list`
  Invocation: `icu fitness-events list`
  Example: `icu fitness-events list`

## folders

- `list`
  Invocation: `icu folders list`
  Example: `icu folders list`

- `create`
  Invocation: `icu folders create --name NAME [--type folder|PLAN] [--desc DESC]`
  Defaults: `folder`
  Example: `icu folders create --name "Base 2026" --type PLAN`

- `update`
  Invocation: `icu folders update <id> [--name NAME] [--desc DESC]`
  Example: `icu folders update 12 --name "Base 2026 A"`

- `delete`
  Invocation: `icu folders delete <id>`
  Example: `icu folders delete 12`

## ftp

- `show`
  Invocation: `icu ftp show [--sport Ride]`
  Defaults: `Ride`
  Example: `icu ftp show --sport Ride`

- `update`
  Invocation: `icu ftp update --value WATTS [--sport Ride] [--indoor]`
  Defaults: `Ride`
  Example: `icu ftp update --value 285 --sport Ride`

## gear

- `list`
  Invocation: `icu gear list`
  Example: `icu gear list`

- `create`
  Invocation: `icu gear create --name NAME [--type Bike] [--distance M]`
  Defaults: `Bike`
  Example: `icu gear create --name "A bike" --distance 1200`

- `update`
  Invocation: `icu gear update <id> [--name NAME] [--distance M]`
  Example: `icu gear update 9 --distance 1800`

- `delete`
  Invocation: `icu gear delete <id>`
  Example: `icu gear delete 9`

## routes

- `list`
  Invocation: `icu routes list`
  Example: `icu routes list`

- `get`
  Invocation: `icu routes get <id> [--include-path]`
  Example: `icu routes get 321 --include-path`

## shared-event

- `get`
  Invocation: `icu shared-event get <id>`
  Notes: id-first parsing is also available, for example `icu shared-event 123 get`.
  Example: `icu shared-event get 123`

## sports

- `list`
  Invocation: `icu sports list`
  Example: `icu sports list`

- `get`
  Invocation: `icu sports get <type|id>`
  Example: `icu sports get Ride`

- `update`
  Invocation: `icu sports update <type|id> [--ftp WATTS] [--indoor-ftp WATTS] [--lthr BPM] [--max-hr BPM]`
  Notes: `--indoor-ftp` is supported by the implementation even though it is omitted from help text.
  Example: `icu sports update Ride --ftp 285 --lthr 172 --max-hr 191`

- `delete`
  Invocation: `icu sports delete <id>`
  Example: `icu sports delete 7`

## tags

- `activities`
  Invocation: `icu tags activities`
  Example: `icu tags activities`

- `events`
  Invocation: `icu tags events`
  Example: `icu tags events`

- `workouts`
  Invocation: `icu tags workouts`
  Example: `icu tags workouts`

## weather

- `config`
  Invocation: `icu weather config` or `icu weather config update --lat LAT --lon LON [--label NAME] [--location NAME] [--provider NAME] [--enabled true|false]`
  Notes: update mode is triggered by any of `--lat`, `--lon`, `--label`, `--location`, `--provider`, or `--enabled`. The current forecast is fetched first and merged with the supplied flags, so `--enabled false` works without re-specifying coordinates.
  Example: `icu weather config --lat -34.60 --lon -58.38 --label "Buenos Aires" --enabled true`
  Example: `icu weather config --enabled false`

- `forecast`
  Invocation: `icu weather forecast`
  Example: `icu weather forecast`

## wellness

- `list`
  Invocation: `icu wellness list --oldest DATE --newest DATE [--fields f1,f2]`
  Example: `icu wellness list --oldest 2026-05-01 --newest 2026-05-31`

- `get`
  Invocation: `icu wellness get <date>`
  Example: `icu wellness get 2026-05-31`

- `update`
  Invocation: `icu wellness update <date> [--weight KG] [--resting-hr BPM] [--hrv VALUE] [--sleep-secs N] [--sleep-score VALUE] [--locked]`
  Notes: `--sleep-score` is supported even though it is omitted from help text.
  Example: `icu wellness update 2026-05-31 --resting-hr 49 --hrv 74.2 --sleep-score 83`

- `bulk`
  Invocation: `icu wellness bulk --file FILE.json`
  Example: `icu wellness bulk --file wellness.json`

- `upload`
  Invocation: `icu wellness upload <file.csv>`
  Example: `icu wellness upload wellness.csv`

## zepp

Zepp (Amazfit) read-only access to wellness data not exposed by Intervals.icu.
Auth uses email/password against the same `api-mifit.huami.com` host the official
Zepp mobile app uses, and then exchanges the result for a per-request
`appToken`. The `country_code` returned by the auth redirect picks the regional
data host automatically (US → `api-mifit.huami.com`, CN → `api-mifit-cn.huami.com`,
DE/FR/IT/ES/GB/NL/PL/RU/TR/SE/NO/FI/DK → `api-mifit-de.huami.com`); you can also
pin it explicitly with `--zepp-country-code` or `ZEPP_COUNTRY_CODE`.

The `summary` and `data_hr` fields from `/v1/data/band_data.json` are returned
as raw base64 **and** decoded into typed structs so the CLI can render them.
Workout `hrSeries`/`paceSeries`/`altitudeSeries`/`powerSeries`/`stepSeries` are
decoded from Zepp's delta-encoded 2-byte shorts back into absolute series.

The Zepp mobile app calculates **BioCharge** (renamed to **HybridCharge** in
Zepp 10.4.0+) on-device from sleep, stress, PAI, and workout history. The public
HTTP API does not return the score itself, so the CLI exposes the raw inputs
the score is derived from. Compute BioCharge in your analysis agent.

- `token`
  Invocation: `icu zepp token --email EMAIL [--password PASSWORD]`
  Notes: Obtain Zepp API tokens without saving to config. WARNING: outputs tokens in plaintext. Use `login` for normal authentication. This command is intended for debugging and manual token management.
  Example: `icu zepp token --email jane@example.com` (the CLI will prompt for the password if it is not provided).

- `login`
  Invocation: `icu zepp login --email EMAIL --password PASSWORD`
  Notes: Persists `zeppLoginToken`, `zeppAppToken`, `zeppUserID`, and
  `zeppCountryCode` in the config file. Set `ZEPP_TOKENS_URL` and
  `ZEPP_LOGIN_URL` to point at a mock server when running the tests.
  Example: `icu zepp login --email jane@example.com --password '...'` (the CLI
  will prompt for the password if it is not provided).

- `logout`
  Invocation: `icu zepp logout`
  Notes: Clears the persisted Zepp tokens from the config file.
  Example: `icu zepp logout`

- `status`
  Invocation: `icu zepp status`
  Notes: Reports whether the local config has a `login_token`. Does not call
  the Zepp API.
  Example: `icu zepp status`

- `profile`
  Invocation: `icu zepp profile`
  Notes: Calls `/v2/user/info` on the regional data host. Falls back to the
  global host on error.
  Example: `icu zepp profile`

- `summary`
  Invocation: `icu zepp summary --oldest DATE --newest DATE`
  Notes: Calls `/v1/data/band_data.json` and decodes the base64-packed
  `summary` field into step and sleep totals.
  Example: `icu zepp summary --oldest 2026-05-01 --newest 2026-05-07`

- `sleep`
  Invocation: `icu zepp sleep --oldest DATE --newest DATE`
  Notes: Decodes the `slp` block (light/deep minutes, stages) from
  `band_data.json`.
  Example: `icu zepp sleep --oldest 2026-05-01 --newest 2026-05-07`

- `heart-rate`
  Invocation: `icu zepp heart-rate --oldest DATE --newest DATE`
  Notes: Decodes the binary `data_hr` field (1440 two-byte shorts per day).
  Sentinel values 254 (no read) and 255 (not required) are mapped to 0.
  Example: `icu zepp heart-rate --oldest 2026-05-01 --newest 2026-05-01`

- `spo2`
  Invocation: `icu zepp spo2 --oldest DATE --newest DATE`
  Notes: Calls `/users/{id}/events?eventType=blood_oxygen` on the events host
  `api-mifit.zepp.com`. The `extra` field is a JSON-encoded string parsed into
  a typed `SpO2Reading.Extra` map.
  Example: `icu zepp spo2 --oldest 2026-05-01 --newest 2026-05-07`

- `stress`
  Invocation: `icu zepp stress --oldest DATE --newest DATE`
  Notes: Calls `/users/{id}/events?eventType=all_day_stress` on the events
  host. Returns per-day min/max/avg/relax%/normal%/medium%/high%.
  Example: `icu zepp stress --oldest 2026-05-01 --newest 2026-05-07`

- `pai`
  Invocation: `icu zepp pai --oldest DATE --newest DATE`
  Notes: Calls `/users/{id}/events?eventType=PaiHealthInfo` on the events
  host. Returns per-day PAI score, resting HR, max HR, and per-zone
  minutes/PAI.
  Example: `icu zepp pai --oldest 2026-05-01 --newest 2026-05-07`

- `workouts`
  Invocation: `icu zepp workouts --oldest DATE --newest DATE`
  Notes: Calls `/v1/sport/run/history.json`. Supports pagination via the
  internal `next` cursor (the CLI follows it transparently).
  Example: `icu zepp workouts --oldest 2026-05-01 --newest 2026-05-31`

- `workout`
  Invocation: `icu zepp workout TRACKID`
  Notes: Calls `/v1/sport/run/detail.json` and decodes the delta-encoded
  HR/pace/altitude/power/step series.
  Example: `icu zepp workout 1717200000`

## workouts

- `list`
  Invocation: `icu workouts list`
  Example: `icu workouts list`

- `get`
  Invocation: `icu workouts get <id>`
  Example: `icu workouts get 88`

- `create`
  Invocation: `icu workouts create --name NAME [--type Ride] [--folder-id ID] [--desc DESC] [--training-load N] [--moving-time SECS]`
  Defaults: `Ride`
  Notes: `--moving-time` is supported even though it is omitted from help text.
  Example: `icu workouts create --name "Tempo 3x12" --type Ride --training-load 78 --moving-time 4200`

- `update`
  Invocation: `icu workouts update <id> [--name NAME] [--desc DESC]`
  Example: `icu workouts update 88 --name "Tempo 3x12 v2"`

- `delete`
  Invocation: `icu workouts delete <id>`
  Example: `icu workouts delete 88`

- `tags`
  Invocation: `icu workouts tags`
  Example: `icu workouts tags`
