# Rebalance

`icu rebalance` creates an editable weekly load redistribution proposal and applies it only after explicit acceptance.

## Dry Run

```bash
icu rebalance show --file rebalance.json --oldest 2026-06-22 --newest 2026-06-28 --type Ride --target POWER --target-load 354 --target-tolerance 10 --start-time 07:00 --min-session-minutes 20 --duration-step-minutes 5 --allocation-basis explicit_equal --max-intensity 0.85 --max-watts 280
```

The dry run fetches activities, events, optional sport settings for the explicit `--type`, and wellness records. When Zepp auth is available it also overlays Zepp `HybridCharge`/`BioCharge` as the preferred recovery score for the wellness analysis, with Intervals wellness `sleepScore` kept only as fallback. It writes pretty JSON to `--file` and does not mutate Intervals.icu.

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

## FTP Selection (Per Session)

`schemaVersion` is `icu.rebalance.v3`. The bump exists because this wave changes what a proposal *means*, not just its shape: pre-v3 files priced every session in the week at one FTP number regardless of indoor/outdoor context, which could disagree with the athlete's real indoor FTP by 15W or more. `rebalance preview` and `rebalance accept` reject an older `schemaVersion` outright with a migration message rather than silently repricing a stale file.

Every generated `session` carries its own resolved FTP, independent of the week-level `context.ftp`:

- `ftpApplied`: the watts this specific session was priced at.
- `ftpSource`: `type_inherent` (an indoor-only type like `VirtualRide`/`IndoorCycling` — cannot be otherwise), `type_default` (an ambiguous cycling type with no `Indoor` flag, defaults outdoor), `flag_asserted` (the event's own `Indoor: true` decided it), `explicit` (`--session-environment` decided it), or `athlete_default` (indoor and outdoor FTP are the same number or `IndoorFTP` is unset — no discrimination needed, no gate, no warning).
- `ftpDisagreement`: true when the resolved environment contradicts another signal on the same record (for example a type-inherent indoor session whose `Indoor` flag is `false`).
- `confidence`: `high` (type-inherent only — this type cannot be otherwise) or `medium` (type-default or flag/explicit-asserted). **Interpretation:** `medium` still prices and proceeds — it is weaker evidence than `high`, not a warning to ignore, and it is not the same thing as `low`. `low` is a distinct tier reserved for activities of an ambiguous cycling type with no discriminating flag at all (the *read/analysis* path, see `docs/analysis.md`); the rebalance/plan write path operates only on calendar events, and the event partition never produces `low` (an ambiguous event with no flag resolves `type_default`/`medium`, never `unknown`/`low` — see `TestResolveFTPEventPartition` in `ftp_selection_test.go`). Consequently `ftp_confidence_low` is not a gate you will see fire out of `rebalance`; the write-time equivalent of "refuse when the environment cannot be determined" is the mixed-environment-week refusal below.
- `context.ftpDiscriminatorConflicts.eventFlagAssertedIndoor`: counts events where an ambiguous type's `Indoor` flag was explicitly asserted `true` — kept observable rather than assumed, since the flag's positive direction has very few historical examples to validate against.

The week-level `context.ftp` / `context.ftpSource` remain for backward compatibility and human summary only — they document "what an unclassified session of the week's default sport type would use," not a value any generated session is actually priced at. Always read a session's own `ftpApplied`.

`context.zoneBasis` is `"per-session"`: zone watt boundaries (Z1/Z2/etc, in watts) now vary per session because each session may resolve a different FTP. A weekly time-in-zone total is therefore not comparable, across this boundary, to a historical aggregate computed against one fixed FTP — the boundaries the minutes were measured against are not identical.

`Ride` resolves differently depending on which side of the tool it is seen from: at analysis time (a completed activity, `docs/analysis.md`) it is a distinct class-2 "default outdoor" case; at rebalance/plan time (a calendar event) it falls into the broader class-3 "ambiguous" bucket alongside other cycling types. Both resolve outdoor by default with `medium` confidence, but do not assume the two tables are the same rule — a `Ride` activity trusts a `false`/absent `Trainer` flag by default, while a `Ride` event only leaves class 3 when its own `Indoor` flag is `false`/absent.

### `--session-environment`

`--session-environment indoor|outdoor` is a per-invocation override for sessions with no per-record signal (D-10c). It is distinct from `--indoor` on `events create` / `ftp update` / `sports update`: those flags set a specific record's own stored `Indoor`/environment flag directly; `--session-environment` instead decides which FTP number *this proposal* prices otherwise-ambiguous sessions at. It appears in `icu rebalance show --help`.

**On an `update` operation, `--session-environment outdoor` does change the outgoing request body** — it is not purely an FTP-pricing input. `EventEx.Indoor` is a bare `bool` with `omitempty` (deliberately not a pointer — see the Safety Model note below for why pointerising it would be wrong), so the override's effect on the *wire* is asymmetric by direction:

- Preserving a session's already-resolved `Indoor: true` (no override, or an override that agrees) marshals the key as `"indoor":true`. This is correct **regardless of whether Intervals.icu's `PUT` clears an omitted field or merges over the existing record** — carrying the value costs nothing under either server behavior.
- Overriding to outdoor removes the `Indoor: true` that would otherwise have been carried, so the body omits the `indoor` key entirely — byte-identical to a body that never resolved an environment at all. **Whether this actually flips the *stored* record to outdoor, or leaves the server's existing `true` untouched, is unverified and will not be verified by this project**, because establishing it would require writing to Juan's live calendar, which this project's tooling does not do (see `rebalance_test.go`'s `TestRebalanceUpdateBodyWirePreservesIndoorTrue` / `TestRebalanceUpdateBodyWireOverrideOmitsIndoorKey`, and `plan_test.go`'s plan-side twins, which pin the outgoing bytes for both directions — the part this repository can assert without a live write).

`--session-environment` is rejected outright — the whole proposal is blocking, naming the mixed week — when the week already contains sessions that naturally resolve to different environments (for example a `VirtualRide` alongside a `Ride`). A single scalar cannot correctly describe a week that already splits indoor/outdoor by type; accepting it there would reintroduce, one level up, the "one FTP for the whole week" assumption this wave removes.

## Time Accounting (`timeAccounting`)

Two categories of session are enumerated, not silently dropped and not silently priced at zero (D-23):

- Sessions whose type is not applicable to FTP pricing at all (D-10 class 4 — `Walk`, `Run`, `VirtualRun`, `WeightTraining`, `Workout`, `NOTE`), each with `loadSource: "none"`.
- Completed activities in STRAVA-stub shape (`Class: not_a_record`, D-24, wave-2/T2.8). A stub is not a session — it is excluded from every load aggregate and cannot mark a planned event's date as already-completed — but its time still occupied a slot, so it is enumerated here with `loadSource: "not_a_record"` rather than disappearing without a trace.

Fields:

- `timeAccounting.entries`: one entry per such session/activity, with `date`, `name`, `type`, `movingTimeSecs`, and `loadSource`.
- `timeAccounting.unaccountedSessions`: how many such entries exist in the week (both categories combined).
- `timeAccounting.loadAccounting`: `"partial"` whenever `unaccountedSessions > 0`, otherwise `"complete"`. **It resolves** — moves from `"partial"` back to `"complete"` — once every class-4 session in the week either leaves the week or gains a load basis of its own (for example a future wave's HR-based load for runs); rebalance does not attempt to price these sessions today (F-27: no per-day capacity bound exists until wave-3), it only makes the gap visible. A `not_a_record` entry can never resolve to priced, because it is not a session at all.

## Edit

The file is intentionally user-editable. You can change `selectedOptionId`, remove operations, mark operations as `skipped`, or adjust operation bodies before acceptance.

Do not edit `sourceHash` unless you intentionally want to bypass drift detection. If a calendar event changed after dry-run, `accept` fails that operation instead of mutating stale state.

## Preview

```bash
icu rebalance preview --file rebalance.json
icu rebalance preview --file rebalance.json --no-live-check
```

`preview` validates the proposal and prints a non-mutating operation summary (pending create/update/cancel counts, expected load, and `validation` blocking/errors/warnings). It exits non-zero when validation is blocking. Live-checks run only when non-blocking unless `--no-live-check` is set. Preview never mutates Intervals.icu.

For explicit multi-session writes (not load redistribution), prefer [`docs/plan.md`](plan.md).

## Accept

```bash
icu rebalance accept --file rebalance.json
```

`accept` rereads the file, re-validates it (including stored blocking errors), and hard-fails before any API mutation when blocking. It then verifies source hashes for update/cancel operations, applies pending operations, writes an `apply` summary back to the file, and prints that summary.

## Approve

```bash
icu rebalance approve --file rebalance.json --reason "coach override" --target-load 380 --level 0.7
```

`approve` binds an outside-envelope proposal to an explicit reason and explicit limits so `accept` can detect drift before mutating calendar events.
The explicit override limits are stored under `approve` and do not rewrite the proposal's existing `constraints`, `baseline`, or `options`.

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
- Wellness context uses a documented 42-day default lookback, overridable with `--wellness-lookback-days`, and prefers Zepp `HybridCharge`/`BioCharge` over `sleepScore` when Zepp auth is configured.
- `update` operation bodies preserve the event's existing indoor/outdoor classification (`Indoor`) unless `--session-environment` explicitly overrides it — an update that does not intend an environment change never resets `Indoor` to Go's zero value. Preserving `Indoor: true` is correct regardless of the server's `PUT` clear-vs-merge semantics (see `--session-environment`, above). `EventEx.Indoor` stays a bare `bool` + `omitempty`, not a pointer: pointerising it to let the override direction assert an explicit `false` would regress the preserve direction, which is the one this guarantee actually depends on. The override-to-outdoor direction's effect on the *stored* record is consequently unverified — its request body omits `indoor` entirely, indistinguishable on the wire from an update that never resolved an environment.
