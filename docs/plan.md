# Plan

`icu plan` turns an explicit multi-session intent file into an editable calendar proposal, then applies it only after preview/accept.

This is the preferred path for writing multiple planned workouts. Prefer `plan` over repeated `events create/update` when the week has several sessions. Prefer `rebalance` when the goal is load redistribution of existing structure rather than writing an explicit session list.

## Intent File

Write a JSON intent (`schemaVersion: icu.plan.intent.v1`):

```json
{
  "schemaVersion": "icu.plan.intent.v1",
  "oldest": "2026-07-28",
  "newest": "2026-08-02",
  "ftp": 285,
  "weeklyTargets": [
    {"weekStart": "2026-07-28", "targetLoad": 150, "tolerance": 30}
  ],
  "strictTargets": false,
  "constraints": {
    "allowCreate": true,
    "allowUpdate": true,
    "allowCancel": false,
    "allowToday": false,
    "allowPast": false,
    "defaultStartTime": "07:00",
    "defaultType": "Ride",
    "defaultCategory": "WORKOUT",
    "defaultTarget": "POWER"
  },
  "sessions": [
    {
      "date": "2026-07-28",
      "name": "Z2 HR-Control Waves",
      "category": "WORKOUT",
      "type": "Ride",
      "startTime": "07:00",
      "desc": "- 60m 70% FTP",
      "descLocal": "- 60m 70%",
      "role": "aerobic",
      "uid": ""
    }
  ],
  "notes": []
}
```

Field notes:

- `sessions` are explicit. Plan does not invent a session library or coaching heuristics.
- `desc` is written to the Intervals event body (Intervals-friendly workout text).
- `descLocal` is used only for local load calculation when present; otherwise `desc` is parsed.
- Matching existing events uses `uid` when set, otherwise `date` + `name`.
- `allowCancel` defaults to false. When true, unmatched mutable events in range are cancelled, and only `WORKOUT` unless `cancelCategories` is set.
- `weeklyTargets` produce warnings when planned week load is outside target±tolerance. Set `strictTargets: true` to block accept/preview.
- Sessions must include `date` and `name`. Dates outside `oldest`/`newest` are blocking errors (no pending ops).
- Inverted ranges (`oldest` > `newest`) are blocking.
- `WORKOUT` sessions require a parseable power description (`descLocal` else `desc`) with calculated `expectedLoad > 0`. Unparseable text or zero load blocks; `NOTE` (and other non-workout categories) may use free text without load.
- FTP comes from intent `ftp` or `--type` sport settings.

## Show (Dry Run)

```bash
icu plan show --file plan.json --intent-file intent.json --now-date 2026-07-27 --type Ride
```

`show` fetches activities and events for the intent range (and sport settings when `--type` is set), builds `icu.plan.v1` proposal JSON, and writes it to `--file` with mode `0600`. It does not mutate Intervals.icu.

CLI overrides:

- `--allow-today` / `--allow-past` reopen planned dates locked by default
- `--now-date DATE` sets the lock boundary used for past/today checks
- `--type SPORT` loads sport settings FTP when intent omits `ftp`

The proposal includes:

- `baseline`: completed load, locked planned load, mutable planned load
- `sessions`: resolved create/update/cancel decisions with load sources
- `operations`: explicit calendar ops (`create` / `update` / `cancel`) with source hashes for update/cancel
- `preview`: pending action counts and expected load
- `weeklyLoads`: optional target evaluation
- `validation`: schema/operation errors and weekly target warnings

## Preview

```bash
icu plan preview --file plan.json
icu plan preview --file plan.json --no-live-check
```

`preview` validates the proposal and prints a non-mutating operation summary that includes `validation.blocking` / `errors` / `warnings`. It exits non-zero when validation is blocking. By default it live-checks source hashes and baseline drift against Intervals.icu only when validation is non-blocking. `--no-live-check` skips network drift checks.

## Accept

```bash
icu plan accept --file plan.json
```

`accept` re-validates the proposal (including stored `validation.errors`, `strictTargets` weekly misses, schema/ops checks) and hard-fails before any API mutation when `validation.blocking` is true. It then rejects baseline drift, applies pending operations, writes an `apply` summary back into the proposal file, and prints the apply summary.

## Safety Model

- `show` and `preview` never mutate Intervals.icu
- `accept` applies only explicit pending operations in the file
- Update/cancel operations carry `sourceHash`; stale calendar events fail that op
- Completed dates are always locked
- Past planned dates are locked unless `--allow-past` / intent `allowPast`
- Today is locked unless `--allow-today` / intent `allowToday`
- No hidden physiological coach defaults: sessions, times, types, and targets come from intent or explicit flags
