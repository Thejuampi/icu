# Rebalance

`icu rebalance` creates an editable weekly load redistribution proposal and applies it only after explicit acceptance.

## Dry Run

```bash
icu rebalance --dry-run --file rebalance.json --oldest 2026-06-22 --newest 2026-06-28 --target-load 354
```

The dry run fetches activities, events, sport settings, and wellness records. It writes pretty JSON to `--file` and does not mutate Intervals.icu.

Use `--max-intensity IF` or `--max-watts WATTS` to cap generated POWER workout intensity. `--max-hr` is rejected because rebalance does not generate HR-target sessions.

The proposal includes:

- `baseline`: completed load, locked planned load, mutable planned load, and current weekly total
- `context.dynamicTargets`: Z1/Z2 intensity targets, long-ride minutes, daily load limit, and source method
- `options`: generated sessions and evaluation against target load
- `operations`: explicit `create`, `update`, and `cancel` operations that `accept` may apply
- `validation`: schema and operation validation state

Dynamic targets prefer sport power zones. If sport zones are unavailable, they use recent cycling activity intensity history with MAD-style outlier filtering. Fallback values are labelled as fallback and should be treated as lower confidence.

## Edit

The file is intentionally user-editable. You can change `selectedOptionId`, remove operations, mark operations as `skipped`, or adjust operation bodies before acceptance.

Do not edit `sourceHash` unless you intentionally want to bypass drift detection. If a calendar event changed after dry-run, `accept` fails that operation instead of mutating stale state.

## Accept

```bash
icu rebalance accept --file rebalance.json
```

`accept` rereads the file, validates it, verifies source hashes for update/cancel operations, applies pending operations, writes an `apply` summary back to the file, and prints that summary.

Supported operation actions:

- `create`: creates a calendar workout event
- `update`: updates an existing calendar workout event
- `cancel`: deletes the existing calendar event

## Safety Model

- Dry-run never mutates Intervals.icu.
- Accept applies only explicit operations present in the file.
- Update/cancel operations include source hashes so stale calendar state is detected.
- Completed/past events are locked by default unless `--now-date` and constraints make them mutable.
- Physiological and planning thresholds must be dynamic or user-provided constraints; hidden hardcoded decision caps are not allowed.
