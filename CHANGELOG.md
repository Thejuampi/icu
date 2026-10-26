# Changelog

## Unreleased

### Improvements from `feat/icu-improvements` (#18)

Breaking changes:
- `analysis wellness` → `physiologyState.source` is now `athlete_baseline_robust_z` (was `local_wellness_heuristic`), and `reasons` codes are now `hrv_below_baseline`, `resting_hr_above_baseline`, `sleep_below_baseline`, `form_below_baseline` (the old fixed-threshold codes such as `hrv_zscore_watch` are gone). Levels come from the athlete's own median/MAD baselines, not fixed numbers; consumers that matched the old strings must be updated.
- The `--curves` / `--adaptation-curves` default change from #17 applies here too.
- `events delete` now prints `{"deleted": ID, "backupPath": PATH}` instead of nothing.
- `--desc-file` pointing at an empty (or whitespace-only) file is an error.
- Profile validation checks the *effective* `watchZ < redZ` (a single override is paired with the other default), so e.g. `watchZ: 3` alone is now rejected.

New JSON fields:
- `plan preview` → `projection` (CTL/ATL/TSB per ISO week). `projection.weeks[].rampPerWeek` is the largest rolling 7-day CTL change (`CTL[d] − CTL[d−7]`) that ends in the week, seeded with real wellness CTL; `rampKnown` is false when fewer than 7 earlier days are known, and `overCap` is only set when it is known.
- `*.threshold` objects (`watch`, `red`, `baseline`, `scale`, `samples`, `source`, `watchOnly`) on wellness signals and `analysis plan` → `history.decouplingThreshold`, plus `history.recentDecoupling`.

Behaviour:
- Load projection uses Intervals.icu's EWMA, `k = 1 − exp(−1/τ)` (τ = 42/7 days); replayed against real wellness data in `testdata/wellness_replay.json`. It starts from the last wellness row before today; days before today use completed activity load, today uses `max(completed, planned)`, later days use planned load. `plan preview --now-date DATE` / `--timezone TZ` set "today" (default: today in the athlete's timezone).
- `history.recentDecoupling` is the mean of the last 7 days of history and is judged against the rides *before* that window; no recent rides means no judgement.
- A profile that pins only a WATCH bound (no history) can reach WATCH but never RED.
- `events delete` and `events create --upsert` (for events on that date with the same `external_id`) back up the event first. `events revert` recreates deleted events, aborts when the event already matches the backup (`--force` overrides), and `--id` skips pre-revert snapshots so a second revert does not undo the first.

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
