# Rebalance

`icu rebalance` creates an editable weekly load redistribution proposal and applies it only after explicit acceptance.

## Dry Run

```bash
icu rebalance --dry-run --file rebalance.json --oldest 2026-06-22 --newest 2026-06-28 --type Ride --target POWER --target-load 354 --target-tolerance 10 --start-time 07:00 --min-session-minutes 20 --duration-step-minutes 5 --allocation-basis explicit_equal
```

The dry run fetches activities, events, optional sport settings for the explicit `--type`, and wellness records. It writes pretty JSON to `--file` and does not mutate Intervals.icu.

Use `--max-intensity IF` or `--max-watts WATTS` to cap generated POWER workout intensity. `--max-hr` is rejected because rebalance does not generate HR-target sessions.

Rebalance does not invent hidden physiological or planning defaults. Intensity, tolerance, duration, start time, sport type, target type, and load allocation must come from sport settings, athlete history, existing event context, or explicit flags such as `--type`, `--target`, `--z1-if`, `--z2-if`, `--target-tolerance`, `--start-time`, `--min-session-minutes`, `--duration-step-minutes`, and `--allocation-basis`.

The proposal includes:

- `baseline`: completed load, locked planned load, mutable planned load, and current weekly total
- `context.dynamicTargets`: Z1/Z2 intensity targets when derivable, long-ride minutes, daily load limit, and source method
- `options`: generated sessions and evaluation against target load
- `operations`: explicit `create`, `update`, and `cancel` operations that `accept` may apply
- `validation`: schema and operation validation state

Each generated session records auditable decision sources for sport type, target type, start time, allocation, intensity, duration, and classification.

Dynamic targets prefer sport power zones. If sport zones are unavailable, they use recent cycling activity intensity history with MAD-style outlier filtering. If neither data source is available and no explicit IF is provided, the proposal is blocking instead of using fallback IF values.

## Edit

The file is intentionally user-editable. You can change `selectedOptionId`, remove operations, mark operations as `skipped`, or adjust operation bodies before acceptance.

Do not edit `sourceHash` unless you intentionally want to bypass drift detection. If a calendar event changed after dry-run, `accept` fails that operation instead of mutating stale state.

## Accept

```bash
icu rebalance accept --file rebalance.json
```

`accept` rereads the file, validates it, verifies source hashes for update/cancel operations, applies pending operations, writes an `apply` summary back to the file, and prints that summary.

## Approve

```bash
icu rebalance approve --file rebalance.json --reason "coach override" --target-load 380 --level 0.7
```

`approve` binds an outside-envelope proposal to an explicit reason and explicit limits so `accept` can detect drift before mutating calendar events.

Supported operation actions:

- `create`: creates a calendar workout event
- `update`: updates an existing calendar workout event
- `cancel`: deletes the existing calendar event

## Safety Model

- Dry-run never mutates Intervals.icu.
- Accept applies only explicit operations present in the file.
- Update/cancel operations include source hashes so stale calendar state is detected.
- Completed dates are always locked.
- Past planned events are locked by default and can be reopened only with `--allow-past`.
- Planned events on `--now-date` are mutable only with `--allow-today`.
- Physiological and planning thresholds must be dynamic or user-provided constraints; hidden hardcoded decision caps are not allowed.
- Sparse or missing history produces blocking validation for generated operations rather than silent defaults.
- Wellness context uses a documented 42-day default lookback, overridable with `--wellness-lookback-days`.
