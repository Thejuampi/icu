# Changelog

## Unreleased

### Improvements from `feat/icu-improvements` (#18)

Breaking changes:
- `analysis wellness` → `physiologyState.source` is now `athlete_baseline_robust_z` (was `local_wellness_heuristic`), and `reasons` codes are now `hrv_below_baseline`, `resting_hr_above_baseline`, `sleep_below_baseline`, `form_below_baseline` (the old fixed-threshold codes such as `hrv_zscore_watch` are gone). Levels come from the athlete's own median/MAD baselines, not fixed numbers; consumers that matched the old strings must be updated.
- The `--curves` / `--adaptation-curves` default change from #17 applies here too.
- The local config file is written atomically (temp file + rename).
- `events delete` now prints `{"deleted": ID, "backupPath": PATH}` instead of nothing, and rejects a non-numeric id before any request.
- `events create --upsert` now requires `--uid`: intervals.icu's `upsertOnUid` matches on `uid` (not `external_id`), so without a uid it never updated anything. The pre-upsert backup searches ten years either side of the date; if the POST still updates an existing event that was not backed up, the command exits with an error saying that event was ALREADY OVERWRITTEN with NO BACKUP, giving its id, date, name and uid.
- `--desc-file` pointing at an empty (or whitespace-only) file is an error.
- Profile validation checks the *effective* `watchZ < redZ` (a single override is paired with the other default), so e.g. `watchZ: 3` alone is now rejected.

New JSON fields:
- `plan preview` → `projection` (CTL/ATL/TSB per ISO week). `projection.weeks[].rampPerWeek` is the largest rolling 7-day CTL change (`CTL[d] − CTL[d−7]`) that ends in the week, seeded with real wellness CTL; `rampKnown` is false when fewer than 7 earlier days are known, and `overCap` is only set when it is known.
- `*.threshold` objects (`watch`, `red`, `baseline`, `scale`, `samples`, `source`, `watchOnly`) on wellness signals and `analysis plan` → `history.decouplingThreshold`, plus `history.recentDecoupling`.

Behaviour:
- Load projection uses Intervals.icu's EWMA, `k = 1 − exp(−1/τ)` (τ = 42/7 days), and feeds CTL and ATL separate loads as Intervals.icu does (wellness `ctlLoad`/`atlLoad`): sports such as WeightTraining and Workout count toward ATL only. Which types count toward CTL is learned per day: the subset of that day's activity loads that sums exactly to `ctlLoad` (ambiguous days prove nothing). Learned classes are used in memory; `plan preview --save-learned` also saves them in the local profile (`learnedFitnessLoadTypes`, written atomically) so a shorter window does not forget them, and a type later seen both ways drops its saved class. Without the flag nothing is written; `profile set --fitness-load-types WeightTraining=atl-only,Yoga=ctl` overrides them. A type with no evidence (or conflicting evidence) is assumed to count toward CTL, with a warning. Replayed against real data 2026-07-31..10-26 (`testdata/wellness_replay.json`, including manual load overrides): CTL and ATL match every day. It starts from the last wellness row before today; days before today use wellness `ctlLoad`/`atlLoad`, today uses completed load plus planned sessions not yet paired with a completed activity, later days use planned load. `plan preview --now-date DATE` / `--timezone TZ` set "today" (default: today in the athlete's timezone).
- `history.recentDecoupling` is the mean of the last 7 days of history and is judged against the rides *before* that window; no recent rides means no judgement.
- Wellness baselines (HRV, resting HR, sleep, TSB) use a fixed 42-day window ending 7 days before the newest sample, so thresholds no longer move with `--oldest` and the recent week is not in its own baseline. Wellness reads always reach back 49 days for the baselines (rows before `--oldest` feed only the baselines), and a warning says when less history exists. When no signal can be judged the aggregate state is `INSUFFICIENT` with `low` confidence and a warning, instead of `OK`.
- A profile that pins only a WATCH bound (no history) can reach WATCH but never RED.
- `events delete` and `events create --upsert` (the event with the same `uid`) back up the event first. `events revert` recreates deleted events, records `restoredEventId` in the backup and refuses to recreate again (or when an event with the same uid, or the same name, category, type and start time, already exists on that date) unless `--force`; it aborts when the event already matches the backup (`--force` overrides), and `--id` skips pre-revert snapshots so a second revert does not undo the first.
- Third-party-edit check: after every write (`events update`, `events revert`, `plan accept` updates, upserts that matched a backed-up event) the backup records `postWriteHash`/`postWriteAt` (hash of id, name, start, description, load, moving time as returned by the API). `events update` and `events revert` GET the event and compare it with the newest backup's `postWriteHash`; if it was edited in Intervals.icu since, they abort before writing (and write no new backup) unless `--force`. Backups from older versions have no `postWriteHash`, so the next update/revert of that event is refused until `--force` is given once. Backups taken for a delete/cancel are not compared.

### Fixes from `fix/icu-bugs` (#17)

New JSON fields:
- `analysis cycling` → `load.latestLoadSource` (`wellness` | `last_activity`) and `load.latestLoadDate`. CTL/ATL/TSB now come from the daily wellness row at the range end, clamped to today in the athlete's timezone, instead of the CTL/ATL frozen on the last activity (B3). If wellness can't be read, `warnings` contains `wellness unavailable, CTL/ATL/TSB are from the last activity: …`.
- `analysis plan` → `history.currentLoadSource`, `history.currentLoadDate` (same meaning).
- `analysis plan` → `history.weeks[].decouplingExcluded`: rides dropped from `meanDecoupling`. A ride is dropped if it is shorter than the athlete-relative floor (min of the P25 moving time and half the median moving time) or is a MAD robust-z outlier (B5).
- `analysis plan` → `history.weeks[].partial`: an edge week only partly inside the history range. It is listed but excluded from `averageWeeklyLoad`, `peakWeeklyLoad`, `recentWeeklyLoad` and `completedWeeks`.
- `weather forecast` → `forecasts[]` with `id`, `lat`, `lon`, `error`, `daily`, `hourly` (B12).

Behaviour changes:
- Weeks with no rides inside the history range count as zero-load weeks in averages (B4).
- `analysis microcycle` uses the athlete profile timezone when `--timezone` is not given (`timezoneSource: "athlete"`), and ignores projected future wellness (B1, B2).
- **Breaking:** the default `--curves` / `--adaptation-curves` changed from `42d,365d` to two non-overlapping windows derived from the analysis range (B8). Pass `--curves 42d,365d` for the old behaviour.
- `weather config` updates keep every existing config entry (B11).
- Workout descriptions accept compound durations (`2m30s`, `1h30m`, B13) and Intervals.icu's un-indented repeat blocks (B6).
- `plan accept` no longer writes the session UID into `external_id` (B14).

Tooling:
- `.golangci.yml` migrated to the golangci-lint v2 format, so `make lint` works with v2.
