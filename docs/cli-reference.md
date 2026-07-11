# CLI Reference

This document describes the current public CLI surface implemented in `cmd/icu/`.
It is intentionally based on shipped behavior, including a few help-text quirks and exact flag spellings.

## Global Behavior

- Auth is required for every resource except `config`.
- Global help is available through `icu help`, `icu --help`, and `icu -h`.
- A bare resource defaults to `show` if that resource has a `show` action, for example `icu athlete`.
- A bare resource without `show` prints resource help, for example `icu activities`.
- `activity` and `shared-event` support id-first parsing. Example: `icu activity i123 show`.
- Long flags prefer hyphen-case. Snake_case spellings are normalized for compatibility where applicable, and API query fields are converted back to snake_case internally.
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
  Notes: `--route_id` is supported by the implementation even though it is omitted from help text. Activity JSON includes Intervals load fields such as `icuTrainingLoad`, and when provided by the API also exposes HR-derived `hrLoad`, `hrLoadType`, and `trimp`.
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
  Notes: `--intervals` adds interval data to the response.
  Example: `icu activity i123 show`
  Example: `icu activity i123 show --intervals`

- `update`
  Invocation: `icu activity <id> update --name NAME [--description DESC] [--type Ride] [--training-load N]`
  Notes: `--training-load` writes the activity `icuTrainingLoad` value. Use a measured or API-provided HR-derived load when correcting activities with incomplete power data; the CLI does not invent HRSS/tTSS values.
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

- `estimate-power`
  Invocation: `icu activity <id> estimate-power --bike-mass-kg N [--rider-mass-kg N] [--cda N | --calibrate-from-measured] [--crr N] [--drivetrain-eff N] [--air-density N] [--min-gap-seconds N] [--include-streams] [--file PATH] [--types CSV] [--ftp N] [--no-weather] [--refill-after-pm-death | --refill-after-cadence-death | --refill-from-index N]`
  Defaults: stream `--types watts,cadence,left_right_balance,altitude,distance,velocity_smooth,heartrate,time`; when omitted, `--crr` defaults to `0.0045` and `--drivetrain-eff` to `0.975` (both labeled in output warnings as explicit omissions, not silent coaching thresholds). Rider mass falls back to athlete `icuWeight`/`weight` when `--rider-mass-kg` is omitted.
  Notes: Read-only dry-run. Classifies each sample as `measured`, `true_zero`, or `missing`. Dual-sided PM edge cases use `left_right_balance`: L/R is present only while the meter is alive (first half with real watts+RPM+balance stays measured); a long null L/R tail after the last present sample is meter death. Cadence freewheels mid-ride are not death (end-anchored cadence null only). Prior accepted fills leave positive watts after death; pass `--refill-after-pm-death` (balance → cadence) to re-open only that second-half gap. Outdoor aero weather priority: activity fields → Open-Meteo archive → forecast past → historical-forecast. Per-sample headwind uses weather × track heading; density from pressure/temp when available. Calibration from the measured segment uses those per-sample series so wind is not absorbed into CdA as still air. Prefer `--calibrate-from-measured` on the alive segment. Does not mutate Intervals.icu. Does not update Strava or other platforms.
  Example: `icu activity i123 estimate-power --bike-mass-kg 9 --calibrate-from-measured --refill-after-pm-death --file fill.json`

- `estimate-power-accept`
  Invocation: `icu activity <id> estimate-power-accept --file PATH`
  Notes: Mutates the activity by `PUT`ing the filled `watts` stream from a prior `estimate-power --file` result. Requires matching `activityId` and non-empty `filledWatts`. Does not invent `icu_training_load`; server reanalysis owns load after the stream write. Supporter-only upstream. Back up streams and the dry-run fill file before accept for rollback (re-accept a prior watts payload). **Strava is not updated:** export `fit-file` after accept, delete the broken Strava activity, and re-upload; see notes under `fit-file` below.
  Example: `icu activity i123 estimate-power-accept --file fill.json`

- `estimate-power-backtest`
  Invocation: `icu activity <id> estimate-power-backtest --bike-mass-kg N [--rider-mass-kg N] [--crr N] [--drivetrain-eff N] [--calibrate-from-measured] [--mode mask_second_half|mask_after_fraction|mask_scatter] [--mask-fraction 0.5] [--no-weather]`
  Defaults: `--mode mask_second_half`, `--calibrate-from-measured` when `--cda` is omitted, `--mask-fraction 0.5` for `mask_after_fraction`, `0.35` for `mask_scatter`.
  Notes: Read-only replay for **outdoor** rides only (`VirtualRide`/Zwift/indoor rejected — not real free-air physics). Uses the same real-weather aero path as `estimate-power`. Prefer continuous measured PM (no prior fill accept) so held-out watts are real. Modes: `mask_second_half` / `mask_after_fraction` simulate mid-ride PM death; `mask_scatter` drops samples across the ride to score model fidelity when neighbors still exist. Scores include pearsonR, spearmanRho, bias, MAD residual z-scores, robustRmse (outlier-aware), zScorePearsonR. In-repo automated gates: physics-consistent synthetic mask → pearsonR/spearmanRho ≥ 0.95 and residualZMedianAbs ≤ 1.0; outdoor-shaped noisy fixture → pearsonR/spearmanRho ≥ 0.80, |bias| within absolute/relative bounds, residualZMedianAbs ≤ 1.5. Known planted headwind series must beat still-air. Live outdoor scores are corroboration only (draft/GPS noise); CFD is out of scope.
  Example: `icu activity i123 estimate-power-backtest --rider-mass-kg 81 --bike-mass-kg 7.8 --calibrate-from-measured`

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
  Notes: Returns raw bytes for the **original** uploaded device/source file. Does **not** include Intervals stream edits (for example filled watts after `estimate-power-accept`).
  Example: `icu activity i123 file > activity.bin`

- `fit-file`
  Invocation: `icu activity <id> fit-file`
  Notes: Returns raw bytes for an Intervals-generated FIT that **includes processed streams** (power/HR fixes and other stream edits on the activity). Use this after `estimate-power-accept` when re-uploading to Strava or another platform: Strava does not receive Intervals stream `PUT`s, and the Strava API cannot rewrite watts on an existing activity. Typical workflow: accept fill on Intervals → download `fit-file` → delete the broken Strava activity → upload the FIT at https://www.strava.com/upload/select. Watch for duplicate re-imports into Intervals from Strava/Wahoo after re-upload.
  Example: `icu activity i123 fit-file > activity-filled.fit`

- `gpx-file`
  Invocation: `icu activity <id> gpx-file`
  Notes: returns raw bytes.
  Example: `icu activity i123 gpx-file > activity.gpx`

## analysis

- `coaching`
  Invocation: `icu analysis coaching [--history-oldest DATE --history-newest DATE] [--plan-oldest DATE --plan-newest DATE] [--history-days N] [--plan-days N] [--sport-type TYPE] [--calendar-id ID] [--resolve BOOL] [--activity-fields CSV] [--limit N] [--include-adaptation BOOL] [--adaptation-curves CSV]`
  Defaults: `--history-days 84`, `--plan-days 28`, `--sport-type Ride`, `--adaptation-curves 42d,365d` when adaptation is requested
  Notes: Read-only JSON facade for coaching/report workflows. Fetches athlete, sport settings, history activities, wellness, plan-range events, NOTE events, and optional adaptation inputs, then bundles existing cycling, wellness, plan, and adaptation analyses into one payload. Wellness-backed readiness prefers Zepp `HybridCharge`/`BioCharge` when Zepp auth is available and falls back to Intervals wellness `sleepScore` otherwise. Parsing is strict: unknown flags, positional arguments, missing values, malformed dates/integers, and invalid booleans fail before auth or HTTP. Booleans accept bare/`true`/`false`/`1`/`0`; explicit date pairs cannot be mixed with their day flags; day counts and `--limit` must be positive. `--limit` caps only history activities. Plan ranges are validated before and after ISO-week alignment. Empty calendar collections are `[]`; top-level warnings include stable, prefixed, exact-deduplicated analyzer warnings.
  Example: `icu analysis coaching --history-oldest 2026-04-06 --history-newest 2026-06-28 --plan-oldest 2026-06-29 --plan-newest 2026-07-26 --resolve`

- `cycling`
  Invocation: `icu analysis cycling [--oldest DATE --newest DATE | --days N] [--fields CSV] [--limit N]`
  Defaults: `--days 28`
  Notes: `--fields` overrides the built-in activity field contract used by the analyzer. `--days` must be a positive integer; invalid values are errors. Explicit dates must use `YYYY-MM-DD`, be chronological, and cannot be combined with `--days`. Default date ranges are calculated in UTC; use explicit dates in the athlete's local timezone for daily-accurate boundaries. Output scope includes `timezone` and `timezoneSource`.
  Example: `icu analysis cycling --days 42`

- `wellness`
  Invocation: `icu analysis wellness [--oldest DATE --newest DATE | --days N] [--fields CSV]`
  Defaults: `--days 28`
  Notes: `--days` must be a positive integer; invalid values are errors. Explicit dates must use `YYYY-MM-DD`, be chronological, and cannot be combined with `--days`. Default date ranges are calculated in UTC; use explicit dates in the athlete's local timezone for daily-accurate boundaries. Output scope includes `timezone` and `timezoneSource`. HRV readiness uses a dynamic recent-vs-baseline comparison with robust z-score when enough samples exist; the raw latest/mean ratio is contextual only. The sleep/recovery signal prefers Zepp `HybridCharge`/`BioCharge` when Zepp auth is available and exposes the chosen source through `sleep.scoreName`, with `sleep.fallbackScoreName` and `warnings` when it had to fall back to `sleepScore`.
  Example: `icu analysis wellness --oldest 2026-04-20 --newest 2026-05-31`

- `plan`
  Invocation: `icu analysis plan [--history-oldest DATE --history-newest DATE] [--plan-oldest DATE --plan-newest DATE] [--history-days N] [--plan-days N] [--sport-type TYPE] [--calendar-id ID] [--resolve] [--activity-fields CSV]`
  Defaults: `--history-days 84`, `--plan-days 28`, `--sport-type Ride`
  Notes: Explicit history and plan ranges must be complete chronological `YYYY-MM-DD` pairs and cannot be combined with their corresponding day flags. `--history-days` and `--plan-days` must be positive integers; invalid values are errors. Plan dates are aligned to ISO week boundaries (Monday/Sunday) and the aligned range is revalidated so weekly planned loads are complete. Default date ranges are calculated in UTC; use explicit dates in the athlete's local timezone for daily-accurate boundaries. Output scope includes `timezone` and `timezoneSource`. The embedded wellness context uses the same Zepp `HybridCharge`/`BioCharge` preference and `sleepScore` fallback as `analysis wellness`.
  Example: `icu analysis plan --history-days 84 --plan-days 28 --resolve`

- `adaptation`
  Invocation: `icu analysis adaptation [--oldest DATE --newest DATE | --days N] [--type Ride] [--curves 42d,365d] [--filters FILTERS] [--newest DATE] [--limit N] [--activity-fields CSV]`
  Defaults: `--days 28`, `--type Ride`, `--curves 42d,365d`
  Notes: `--newest` applies to the power-curve fetch; `--limit` applies to activity history fetch. `--days` must be a positive integer; invalid values are errors. Explicit dates must use `YYYY-MM-DD`, be chronological, and cannot be combined with `--days`. Default date ranges are calculated in UTC; use explicit dates in the athlete's local timezone for daily-accurate boundaries. Output scope includes `timezone` and `timezoneSource`.
  Example: `icu analysis adaptation --days 42 --type Ride --curves 42d,365d`

- `microcycle`
  Invocation: `icu analysis microcycle [--date DATE | --week DATE | --from DATE --to DATE] [--json] [--full] [--no-plan] [--no-wellness] [--sport-type TYPE] [--timezone TZ]`
  Defaults: current Monday-Sunday microcycle, `--sport-type Ride`, system timezone fallback when `--timezone` is omitted.
  Notes: Experimental read-only diagnostic contract for LLM/skill consumers. Reads activities with a 90-day lookback, planned events, wellness, and sport settings. When wellness is enabled, the readiness sleep/recovery signal prefers Zepp `HybridCharge`/`BioCharge` and falls back to `sleepScore` when needed. JSON is the primary contract; human output is a brief inspection view.
  Example: `icu analysis microcycle --from 2026-06-08 --to 2026-06-14 --json`

- `micro`
  Invocation: `icu analysis micro [--date DATE | --week DATE | --from DATE --to DATE] [--json] [--full] [--no-plan] [--no-wellness] [--sport-type TYPE] [--timezone TZ]`
  Notes: Alias for `analysis microcycle`. This replaces the former experimental per-activity micro-analysis command.
  Example: `icu analysis micro --json`

- `workout`
  Invocation: `icu analysis workout <activity-id> [--event-id ID] [--calendar-id ID] [--sport-type TYPE] [--match-window-hours N] [--stream-types CSV]`
  Defaults: `--match-window-hours 24`, `--stream-types watts,heartrate,cadence`, `--sport-type` inferred from the activity type and falling back to `Ride`.
  Notes: Read-only JSON contract for planned-workout execution review. The command fetches the activity, detected intervals, raw streams, sport settings, and either the explicit `--event-id` or nearby resolved calendar events. It emits match confidence, planned workout steps, execution micro metrics, session/rep/step comparison, and warnings when plan, stream, interval, FTP, or LTHR data is missing.
  Example: `icu analysis workout i123 --calendar-id 1`

## rebalance

- `show`
  Invocation: `icu rebalance show --file PATH --oldest DATE --newest DATE [--target-load N] [--target-tolerance N] [--now-date DATE] [--type SPORT] [--target POWER] [--start-time HH:MM] [--min-session-minutes N] [--duration-step-minutes N] [--allocation-basis explicit_equal] [--allow-today] [--allow-past] [--wellness-lookback-days N] [--max-intensity IF] [--max-watts WATTS]`
  Notes: Dry-run command. It fetches completed activities, calendar events, sport settings only when `--type` is explicitly provided, and wellness context, then writes a pretty JSON proposal to `--file`. It does not mutate Intervals.icu. Wellness-backed readiness prefers Zepp `HybridCharge`/`BioCharge` when Zepp auth is available and falls back to `sleepScore` otherwise. The proposal includes baseline load, dynamic targets, selected operations, validation, source hashes for update/cancel operations, and per-session decision sources for sport type, target type, time, allocation, intensity, duration, and classification. `--max-intensity` and `--max-watts` cap generated POWER workout intensity; `--max-hr` is rejected because rebalance does not generate HR-target sessions, and `--target` must be `POWER` when provided. Rebalance does not use hidden fallback IF, sport type, target type, tolerance, duration, time, or allocation defaults; missing sport settings/history must be supplied through explicit flags or the proposal is blocking. `--wellness-lookback-days` defaults to `42` as the analysis window used to fetch wellness context.
  Example: `icu rebalance show --file rebalance.json --oldest 2026-06-22 --newest 2026-06-28 --type Ride --target POWER --target-load 354 --target-tolerance 10 --start-time 07:00 --min-session-minutes 20 --duration-step-minutes 5 --allocation-basis explicit_equal`

- `accept`
  Invocation: `icu rebalance accept --file PATH`
  Notes: Reads the edited proposal, validates schema and operations, verifies source hashes for update/cancel operations, applies pending `create`, `update`, and `cancel` operations, writes apply results back to the same file, and prints the apply summary.
  Example: `icu rebalance accept --file rebalance.json`

- `approve`
  Invocation: `icu rebalance approve --file PATH --reason TEXT [--target-load N] [--level X] [--mode MODE]`
  Notes: Binds an outside-envelope proposal to an explicit rationale and explicit limits. Explicit approval limits are stored under `approve` so the original proposal constraints and evaluations remain unchanged; `accept` recomputes the approval fingerprint and rejects drifted proposals.
  Example: `icu rebalance approve --file rebalance.json --reason "coach override" --target-load 380 --level 0.7`

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
  Notes: prints config path and non-secret values. API key and Zepp login token show only a 12-character SHA256 fingerprint and length, never partial or raw secret characters.
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
  Invocation: `icu events create [--category WORKOUT] [--type Ride] --name NAME --start-date DATE [--moving-time SECS] [--training-load N] [--desc DESC] [--calculate-load --ftp N] [--color VALUE] [--indoor] [--external-id ID] [--upsert]`
  Defaults: `--category WORKOUT`, `--type Ride`
  Notes: `NOTE` events behave like all-day calendar notes in Intervals.icu; the date is preserved, but the time-of-day portion of `--start-date` may be normalized away. Long `NOTE` descriptions can also fail upstream with HTTP 500, so prefer shorter notes or split large guidance across multiple notes. With `--calculate-load`, the CLI parses `--desc`, calculates `moving_time` and `icu_training_load` locally from power targets, sends those fields, and returns `{event, estimate, warnings}` so server values can be compared against the local estimate.
  Example: `icu events create --name "Threshold" --start-date 2026-06-03 --training-load 90 --indoor`
  Example: `icu events create --name "Endurance" --start-date 2026-06-03 --desc "- 60m 70%" --calculate-load --ftp 300`

- `update`
  Invocation: `icu events update <id> [--name NAME] [--desc DESC] [--training-load N] [--moving-time SECS] [--calculate-load --ftp N]`
  Notes: `--calculate-load` is explicit only; descriptions are not parsed unless the flag is present. When enabled, calculated `moving_time` and `icu_training_load` override manual values for the request body.
  Example: `icu events update 123 --name "Threshold 4x8" --training-load 92`
  Example: `icu events update 123 --desc "- 60m 70%" --calculate-load --ftp 300`

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

Recent Zepp app versions expose the energy score behind **BioCharge** /
**HybridCharge** through `/v2/users/me/events` as `Charge/insight_data` for some
accounts and app builds. The CLI exposes that stream directly as
`zepp hybridcharge` and `zepp biocharge`, while keeping the lower-level
readiness/body-battery/health-summary inputs available when you need to inspect
the raw components.

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
  Notes: Clears all persisted Zepp credentials from the config file (`zeppLoginToken`, `zeppAppToken`, `zeppUserID`, `zeppCountryCode`).
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
  Invocation: `icu zepp heart-rate --source band|app --oldest DATE --newest DATE`
  Notes: Default source `band` decodes the binary `data_hr` field (1440
  two-byte shorts per day) from `band_data.json`; sentinel values 254 (no
  read) and 255 (not required) are mapped to 0. Source `app` calls
  `/users/{id}/heartRate` on the regional data host.
  Example: `icu zepp heart-rate --source app --oldest 2026-05-01 --newest 2026-05-01`

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

- `events`
  Invocation: `icu zepp events --preset NAME --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Calls `/v2/users/me/events` on the Zepp events host
  `api-mifit.zepp.com`. Presets map
  to Zepp `eventType`/`subType` pairs (e.g. `body-battery` →
  `Charge`/`real_data`). Use `icu zepp events --help` to list presets.
  Example: `icu zepp events --preset body-battery --oldest 2026-06-01 --newest 2026-06-07`

- `hrv`
  Invocation: `icu zepp hrv --metric sdnn|rmssd --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Fetches nightly HRV from `/v2/users/me/events`. Default metric is
  `sdnn`; use `rmssd` for the RMSSD variant.
  Example: `icu zepp hrv --metric rmssd --oldest 2026-06-01 --newest 2026-06-07`

- `readiness`
  Invocation: `icu zepp readiness --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Daily readiness scores from `/v2/users/me/events`.
  Example: `icu zepp readiness --oldest 2026-06-01 --newest 2026-06-07`

- `body-battery`
  Invocation: `icu zepp body-battery --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Body-battery / Charge levels from `/v2/users/me/events`.
  Example: `icu zepp body-battery --oldest 2026-06-01 --newest 2026-06-07`

- `hybridcharge`
  Invocation: `icu zepp hybridcharge --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: HybridCharge energy scores from `/v2/users/me/events` using `Charge/insight_data`. Zepp app 10.4+ renamed BioCharge to HybridCharge.
  Example: `icu zepp hybridcharge --oldest 2026-06-01 --newest 2026-06-07`

- `biocharge`
  Invocation: `icu zepp biocharge --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Alias for `zepp hybridcharge` for older Zepp terminology.
  Example: `icu zepp biocharge --oldest 2026-06-01 --newest 2026-06-07`

- `health-summary`
  Invocation: `icu zepp health-summary --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Daily health summaries from `/v2/users/me/events`
  (`DailyHealth/summary`).
  Example: `icu zepp health-summary --oldest 2026-06-01 --newest 2026-06-07`

- `mood`
  Invocation: `icu zepp mood --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Mood / emotion readings from `/v2/users/me/events`.
  Example: `icu zepp mood --oldest 2026-06-01 --newest 2026-06-07`

- `skin-temp`
  Invocation: `icu zepp skin-temp --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Skin temperature delta readings from `/v2/users/me/events`.
  Example: `icu zepp skin-temp --oldest 2026-06-01 --newest 2026-06-07`

- `weight`
  Invocation: `icu zepp weight --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Weight measurements from `/users/{id}/members/-1/weightRecords` on
  the regional data host.
  Example: `icu zepp weight --oldest 2026-06-01 --newest 2026-06-07`

- `manual-data`
  Invocation: `icu zepp manual-data --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Manually entered wellness records from `/v1/user/manualData.json`
  on the regional data host.
  Example: `icu zepp manual-data --oldest 2026-06-01 --newest 2026-06-07`

- `second-heart-rate`
  Invocation: `icu zepp second-heart-rate --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Per-second heart-rate COS file index from
  `/users/me/fileInfo/events`. Blobs are not downloaded.
  Example: `icu zepp second-heart-rate --oldest 2026-06-01 --newest 2026-06-07`

- `spo2-windows`
  Invocation: `icu zepp spo2-windows --date YYYY-MM-DD [--timezone TZ]`
  Notes: SpO2 ODI windows for a single day from
  `/users/{id}/events/dateString`. Defaults to `UTC`.
  Example: `icu zepp spo2-windows --date 2026-06-07`

- `stress-minute`
  Invocation: `icu zepp stress-minute --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Per-minute stress readings from `/v2/users/me/events`
  (`Charge`/`stress_data`).
  Example: `icu zepp stress-minute --oldest 2026-06-01 --newest 2026-06-07`

- `respiratory-rate`
  Invocation: `icu zepp respiratory-rate --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Overnight respiratory rate readings from `/v2/users/me/events`.
  Example: `icu zepp respiratory-rate --oldest 2026-06-01 --newest 2026-06-07`

- `blood-pressure`
  Invocation: `icu zepp blood-pressure --source watch|user --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Blood-pressure readings. Default source `watch` uses
  `/v2/users/me/events`; `user` uses `/users/me/bloodPressure`.
  Example: `icu zepp blood-pressure --source user --oldest 2026-06-01 --newest 2026-06-07`

- `sport-load`
  Invocation: `icu zepp sport-load --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: Daily training load from `/v2/watch/users/{id}/WatchSportStatistics/SPORT_LOAD`.
  Example: `icu zepp sport-load --oldest 2026-06-01 --newest 2026-06-07`

- `vo2`
  Invocation: `icu zepp vo2 --oldest YYYY-MM-DD --newest YYYY-MM-DD`
  Notes: VO2 max estimates from `/v2/watch/users/{id}/WatchSportStatistics/VO2_MAX`.
  Example: `icu zepp vo2 --oldest 2026-06-01 --newest 2026-06-07`

- `workouts`
  Invocation: `icu zepp workouts [--sport NAME] --oldest DATE --newest DATE`
  Notes: Calls `/v1/sport/{sport}/history.json` (default `run`). Supports
  pagination via the internal `next` cursor (the CLI follows it
  transparently).
  Example: `icu zepp workouts --sport walking --oldest 2026-05-01 --newest 2026-05-31`

- `workout`
  Invocation: `icu zepp workout [--sport NAME] TRACKID`
  Notes: Calls `/v1/sport/{sport}/detail.json` (default `run`) and decodes
  the delta-encoded HR/pace/altitude/power/step series.
  Example: `icu zepp workout 1717200000`

## workouts

- `list`
  Invocation: `icu workouts list`
  Example: `icu workouts list`

- `get`
  Invocation: `icu workouts get <id>`
  Example: `icu workouts get 88`

- `calculate`
  Invocation: `icu workouts calculate --ftp N --desc DESC`
  Notes: Calculates planned cycling duration, average power, normalized power, IF, and TSS locally without writing to Intervals.icu. Supported description grammar is line-oriented: `- 10m 55-75%`, `- 5m 90%`, and repeat blocks such as `3x` with indented child steps. Power targets are interpreted as `%ftp`.
  Example: `icu workouts calculate --ftp 300 --desc "- 60m 70%"`

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
