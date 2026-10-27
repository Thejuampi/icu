# Changelog

## Unreleased

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
