# Analysis Commands

The `analysis` resource provides read-only commands built on top of Intervals.icu data already exposed by the CLI.
All analysis output is JSON.

## Shared Behavior

- Date windows use UTC normalization in the CLI layer when no explicit dates are given.
- Use `--oldest`/`--newest` with athlete-local `YYYY-MM-DD` dates for precise daily boundaries.
- `--oldest` and `--newest` must be provided together.
- If explicit dates are omitted, the CLI falls back to day-based defaults.
- Each analysis scope includes `timezone` and `timezoneSource` so consumers can verify which timezone was used for the date range.
- Missing source metrics are represented as zeros, omitted fields, or warnings rather than fabricated estimates.
- These commands do not write back to Intervals.icu.

## analysis coaching

### Purpose

Build one coaching/report context payload from the data that otherwise requires several separate CLI calls. This command is intended for LLM or coaching-layer consumers that need completed history, wellness, athlete anchors, sport settings, upcoming calendar context, NOTE events, and plan analysis in one JSON contract.

### CLI Contract

```bash
icu analysis coaching \
  [--history-oldest DATE --history-newest DATE] \
  [--plan-oldest DATE --plan-newest DATE] \
  [--history-days N] [--plan-days N] \
  [--sport-type TYPE] [--calendar-id ID] [--resolve BOOL] \
  [--activity-fields CSV] [--limit N] \
  [--include-adaptation BOOL] [--adaptation-curves CSV]
```

Defaults:

- `--history-days 84`
- `--plan-days 28`
- `--sport-type Ride`
- `--adaptation-curves 42d,365d` when `--include-adaptation` is set
- history window ends on the current UTC date
- plan window starts on the next ISO block boundary used by the CLI
- explicit `--plan-oldest` and `--plan-newest` dates are aligned to the enclosing ISO week boundaries (Monday/Sunday)

This command uses strict, command-specific parsing. It rejects unknown flags, positional arguments, missing values, malformed integers, and invalid boolean values before authentication or HTTP calls. Boolean flags accept the bare flag or `true`, `false`, `1`, and `0`. Date values must use `YYYY-MM-DD`; explicit history and plan dates must be complete pairs in chronological order and cannot be combined with the corresponding `--history-days` or `--plan-days` flag. Day counts and `--limit` must be positive integers. Plan order is checked both before and after ISO-week alignment. `--limit` has no default and, when supplied, caps only the history activity fetch.

### Upstream Data

The CLI fetches:

- `athlete`
- `sport-settings` for `--sport-type`
- completed `activities` for the history window
- `wellness` for the history window
- `events` for the plan window, optionally with `--resolve`
- `power-curves` and `mmp-model` only when `--include-adaptation` is set

It then runs the existing deterministic analyzers: `AnalyzeCyclingActivities`, `AnalyzeWellness`, `AnalyzeTrainingPlanWithContext`, and optionally `AnalyzeCyclingAdaptation`.

### Output Sections

- `scope`: command, sport, history range, plan range, and `timezone`/`timezoneSource`
- `athlete`: athlete profile returned by the API
- `sportSettings`: sport anchors and zones for the selected sport when available
- `analyses.cycling`: same contract as `analysis cycling`
- `analyses.wellness`: same contract as `analysis wellness`
- `analyses.plan`: same contract as `analysis plan`, with wellness and optional adaptation context
- `analyses.adaptation`: present only with `--include-adaptation`
- `calendar.events`: all events fetched for the plan range; always an array, including `[]` when empty
- `calendar.notes`: extracted NOTE events with date, name, and description; always an array, including `[]` when empty
- `dataQuality`: source, warning, and missing-section metadata. Warnings retain façade warnings and aggregate analyzer warnings in stable cycling, wellness, plan, and optional adaptation order with `cycling:`, `wellness:`, `plan:`, and `adaptation:` prefixes. Exact duplicate messages are removed after prefixing.
- `sideEffects`: explicit read-only declaration; all mutation fields are `false`

### Interpretation

- Prefer this command for weekly reports and planning reviews when the goal is to minimize CLI calls.
- The command bundles existing analysis outputs; it does not add a hidden coaching recommender or new readiness thresholds.
- NOTE events are first-class in the output because they often contain block intent, decision rules, and athlete communication.
- Use `analysis workout` as a second call when reviewing one specific completed activity against its planned session.

### Current Limitations

- Output can be large because it preserves the full underlying analysis sections and calendar events.
- Adaptation is opt-in to avoid extra power-curve and MMP calls in the common report path.
- If sport settings are missing, the command still emits the context with `sportSettings` marked missing, but planning anchors are reduced.

## analysis cycling

### Purpose

Summarize recent cycling history into a numeric payload that can be consumed directly or passed into a higher-level coaching layer.

### CLI Contract

```bash
icu analysis cycling [--oldest DATE --newest DATE | --days N] [--limit N] [--min-included-activities N]
```

Defaults:

- `--days 28`
- built-in activity field contract from `cmd/icu/cmd_analysis.go`
- `--min-included-activities` has no default (D-4); the check is skipped entirely when omitted or `0`

### Upstream Data

The CLI fetches `activities` for the requested date range and passes them to `AnalyzeCyclingActivities`.
The activity field contract is fixed (D-8, wave-2) — see **`--fields`/`--activity-fields` is deprecated and ignored** below.

### Output Sections

- `scope`: analyzed activity counts (real activities only — see `validity` below), date range, and `timezone`/`timezoneSource` metadata. `scope.startDate`/`scope.endDate` are the **observed span of the matched activities' own dates**, not the requested `--oldest`/`--newest` window — e.g. `--oldest 2026-05-01 --newest 2026-05-31` narrows to `2026-05-02`..`2026-05-30` when that is where the actual activities fall. When zero activities match, both keys are **entirely absent** (`omitempty`), not present with a null or empty-string value — do not assume the shape has these two keys unconditionally.
- `validity`: per-activity inclusion accounting (D-2/D-3/D-24) — see **Validity, Ignore Flags, and STRAVA Stubs** below
- `state`: current load-pressure classification and directive
- `volume`: time, distance, and elevation totals
- `powerAnchors`: FTP, CP, W', Pmax, and rolling FTP when available
- `environment`: temperature, wind, altitude, gradient, and strain context
- `load`: total load, monotony, strain, acute/chronic view, CTL/ATL/TSB, per-channel breakdown, and daily load series — see **Tri-Load Channels** below. `latestCtl`/`latestAtl`/`latestForm` come from the newest daily wellness row on or before the range end (clamped to today) when one exists, labeled `latestLoadSource: "wellness"` and `latestLoadDate`; otherwise they are the values recorded on the newest activity (no decay over later rest days, no load from other sports) and both labels are absent.
- `intensity`: weighted intensity and zone totals
- `durability`: decoupling, efficiency factor, and variability summaries
- `anaerobic`: work above FTP, W' depletion, and repeatability support metrics
- `performance`: repeatability, durability, neural-density, and efficiency classifications
- `sessions`: per-session rollup for qualifying cycling activities
- `warnings`: gaps such as missing power-anchor comparisons, sparse data, or a **D-4 section refusal** (see below)

### Validity, Ignore Flags, and STRAVA Stubs

`validity` (D-3) is the single source of which activities contributed to which aggregate, and why one did not:

- `validity.evaluated` / `validity.included` / `validity.excluded`: totals over real (non-stub) activities in scope.
- `validity.exclusionRate`: `excluded / evaluated`, always emitted (even when `0`) so a caller can see the condition before it changes anything.
- `validity.unavailable.stravaStubs`: how many raw records in the fetch window were D-24 STRAVA stubs (5 keys, `_note` present, `source == "STRAVA"`) — not activities at all, so they never occupy `scope.activities`, a load aggregate, or a pairing anywhere in the codebase.
- `validity.excludedByFlag` / `validity.excludedByScope` / `validity.excludedActivityIds`: which `icu_ignore_*` flag excluded a given activity, and which scope (`power`/`hr`/`all`) it was dropped from.
- `validity.zeroSemantics`: **absence is not a measured zero** (D-5) — a channel total of `0` with `nIncluded: 0` means no included activity reported that channel, not that the reported load was zero.
- `validity.source` / `validity.fieldsGuaranteed`: audit trail — `fieldsGuaranteed: true` records that the activity fetch used the fixed D-8 field contract, so validity/pairing/load fields were never silently dropped by a caller-supplied field list.

Per-flag semantics (D-2), applied per activity, before any aggregate sees it:

| Flag | Scope excluded | Still included in |
| --- | --- | --- |
| `icu_ignore_power` | `power` — `load.total`, `load.byChannel.power`/`.trainingLoad`, power-zone `intensity`, `anaerobic`, `powerAnchors` accumulation | `volume`, HR-derived `load`/`intensity` |
| `icu_ignore_hr` | `hr` — `load.byChannel.hr`, HR-zone `intensity`, HR-derived `durability` | `volume`, power-derived `load`/`intensity` |
| `icu_ignore_time` | `all` — every aggregate, **including `volume`**, because duration is the denominator every other aggregate divides by | nothing; the activity is fully excluded |
| 2 or more of the above together | `all` (same as `icu_ignore_time` alone — D-2's union rule) | nothing |

`durability` additionally requires **both** channels trustworthy for an activity to be included (decoupling/efficiency/variability compare power against HR), so an `icu_ignore_power`- or `icu_ignore_hr`-scoped activity is excluded from `durability` even though only one channel was flagged.

**Inertness statement (must be read before relying on ignore-flag filtering):** as of wave-2, every `icu_ignore_power`/`icu_ignore_hr`/`icu_ignore_time` flag has been measured **`false` across all real activities in the live Intervals.icu corpus this feature was built and verified against** (`testdata/wave2_live_corpus.json`, 230 real activities). The per-flag filtering logic above is correct and unit-tested (`W2-P02`/`W2-P03` in `analysis_test.go`), but on that corpus it is **inert** — it changes nothing, because no live activity in it sets any ignore flag. Its only current acceptance evidence is synthetic fixtures constructed in tests. Do not report "ignore-flag filtering fixed activity count N" as an observed live effect; report it as a correct, tested, but so-far-unexercised-in-production code path. The stub-exclusion behavior (D-24) is the opposite: it is measured live and material — 73 of 303 raw records in the same corpus are STRAVA stubs (F-19).

### D-4 Structural Refusal

When a section's `nIncluded` is `0` (every candidate activity was excluded from that section's scope, or there were none), the section's fields are **not restructured or omitted** (D-6 declined restructuring existing output shapes) — they keep their already-zero values, and a named warning is added instead so a caller never mistakes "no included activity" for "measured zero load/intensity/durability":

- `"load section refused: 0 included activities (all excluded by icu_ignore_power/icu_ignore_time)"`
- `"intensity section refused: 0 included activities (all excluded by icu_ignore_power/icu_ignore_time)"`
- `"durability section refused: 0 included activities (all excluded by icu_ignore_power/icu_ignore_hr/icu_ignore_time)"`

`--min-included-activities N` is a separate, optional, no-default threshold: it never blocks output by itself, it only adds a named warning (`"included activity count %d is below the configured minimum %d (--min-included-activities)"`) when `validity.included` falls below it — useful for a consumer that wants to flag a thin week without changing the shape of the response.

### Tri-Load Channels

`load.byChannel` (D-23/T2.5) breaks the legacy `load.total` figure into its component channels, each with its own `nIncluded` so a caller can tell how many activities actually contributed to that channel's total, independent of how many contributed to `load.total` overall:

- `load.byChannel.trainingLoad`: the legacy server-computed training load (same numerator as `load.total`).
- `load.byChannel.hr`: HR-based load (`hrLoad`), only from activities with a trustworthy HR channel.
- `load.byChannel.power`: power-based load contribution, only from activities with a trustworthy power channel.
- `load.byChannel.pace`: pace-based load (`paceLoad`), for non-power sports; typically `0`/`nIncluded: 0` for a pure-cycling window, which is **not** a measured zero (see `validity.zeroSemantics`).
- `load.divergence`: how far `trainingLoad` and the channel-specific estimates disagree on average — a large divergence suggests the server's blended load and one channel's own estimate are telling different stories for this window.

### `--fields`/`--activity-fields` is deprecated and ignored

Wave-1's assumption that a caller-supplied `--fields`/`--activity-fields` override was a meaningful customization point was never true in practice, and became actively dangerous once validity/pairing/load fields were added: a narrower caller-supplied list could silently drop `icu_ignore_*`, `paired_event_id`, `hr_load`, `power_load`, `pace_load`, or `compliance` from the fetch, producing validity/load/pairing output that looked complete but was quietly computed from a data-starved fetch. D-8 removes this risk at a single choke point (`analysisActivityFields()` in `cmd/icu/cmd_analysis.go`): **the analysis activity fetch always uses its own fixed field list; any `--fields`/`--activity-fields` value you pass is accepted for backward CLI compatibility but has no effect.** `validity.fieldsGuaranteed: true` in the output records that this guarantee held for the analysis you are looking at.

### Interpretation

- Only cycling activities are included in the analysis totals.
- The load section is intended as a compact readiness snapshot, not a full planning model.
- Environmental summaries help explain why similar loads may have different physiological cost.
- Session-level W' and decoupling metrics are useful for follow-up inspection, not for single-number decision making.

### Current Limitations

- Output quality depends on Intervals.icu providing the power, weather, and zone fields requested by the default contract.
- `--fields`/`--activity-fields` no longer has any effect on this fetch (D-8); it is accepted only for backward compatibility.
- The CLI does not currently render alternative output formats for this command.

## analysis wellness

### Purpose

Summarize wellness records into a physiology and data-coverage view.

### CLI Contract

```bash
icu analysis wellness [--oldest DATE --newest DATE | --days N] [--fields CSV]
```

Defaults:

- `--days 28`

### Upstream Data

The CLI fetches `wellness` records for the requested range. When Zepp auth is available, it also fetches Zepp HybridCharge/BioCharge for the same dates and passes that named score into `AnalyzeWellness` as the preferred recovery signal. If Zepp HybridCharge is missing or unavailable, the analysis falls back to the legacy wellness `sleepScore`.

### Output Sections

- `scope`: record count, total day span, date range, and `timezone`/`timezoneSource` metadata
- `coverage`: percentage coverage for HRV, resting HR, sleep, and subjective wellness
- `hrv`: mean, latest, ratio, delta, 7-day trend, recent moving mean, baseline mean, baseline MAD, and robust z-score when enough samples exist
- `restingHr`: mean, latest, delta, and trend
- `sleep`: mean, latest, delta, trend, `scoreName`, and optional `fallbackScoreName`
- `lactate`: mean, latest, trend, coverage, and local state
- `subjective`: fatigue, stress, soreness, and motivation means when present
- `load`: latest CTL, ATL, and derived TSB from wellness records
- `state`: overall physiology status with confidence and reasons
- `warnings`: missing-data or low-confidence cues

### Interpretation

- The command is primarily a coverage-aware readiness summary.
- `state` is intentionally conservative and depends on both signal direction and data completeness.
- HRV state uses a dynamic personal baseline: the recent 7-day mean is compared with the prior samples in the requested range using MAD-based robust z-score when possible. The latest/mean ratio remains in the output as context, but it does not by itself trigger HRV `WATCH`.
- Sleep/readiness preference is source-aware: Zepp `HybridCharge`/`BioCharge` is the primary score when available, and `sleepScore` is only a fallback. Check `sleep.scoreName`, `sleep.fallbackScoreName`, and `warnings` before treating the sleep/recovery signal as Zepp-backed.
- Subjective metrics are only summarized when the underlying records include them.

### Current Limitations

- No diagnosis is performed. This is a signal summary, not medical advice.
- If coverage is sparse, the state will be less reliable even when the latest values look normal.
- CTL/ATL/TSB are only present when those values exist in the wellness source records.

## analysis plan

### Purpose

Compare recent completed cycling history with upcoming calendar events and produce a structured planning view for the next block.

### CLI Contract

```bash
icu analysis plan \
  [--history-oldest DATE --history-newest DATE] \
  [--plan-oldest DATE --plan-newest DATE] \
  [--history-days N] [--plan-days N] \
  [--sport-type TYPE] [--calendar-id ID] [--resolve] \
  [--activity-fields CSV]
```

Defaults:

- `--history-days 84`
- `--plan-days 28`
- `--sport-type Ride`
- history window ends on the current UTC date
- plan window starts on the next ISO block boundary used by the CLI
- explicit `--plan-oldest` and `--plan-newest` dates are aligned to the enclosing ISO week boundaries (Monday/Sunday)

### Upstream Data

The CLI fetches:

- completed `activities` for the history window
- `sport-settings` for `--sport-type`
- `wellness` for the history window
- `events` for the plan window, optionally with `--resolve`

It then runs `AnalyzeTrainingPlanWithContext` with sport settings and wellness context.
The CLI currently passes `Adaptation: nil`, even though the library supports adaptation-aware planning context.

### Output Sections

- `scope`: history window, plan window, event counts, and `timezone`/`timezoneSource` metadata
- `history`: completed weekly load/hours, recent tolerance, current state, and completed-week series. `currentCtl`/`currentAtl`/`currentTsb` prefer the latest wellness row at or before the history end (`currentLoadSource: "wellness"`, `currentLoadDate`), falling back to the newest activity (`currentLoadSource: "last_activity"`). Projected future wellness rows are ignored. Every full ISO week inside the history range is counted, with zero-load weeks included in `weeks`, `completedWeeks`, and the weekly averages; partial weeks at the range edges count only when they have rides. History load and hours are cycling-only (`Ride`-family activities); CTL/ATL include all sports. Weekly `meanDecoupling` and `currentDecoupling` exclude per-ride decoupling outliers detected with a MAD robust z-score against the history's own rides (needs at least 5 rides; otherwise all are kept); `weeks[].decouplingExcluded` counts dropped rides.
- `anchors`: FTP, indoor FTP, LTHR, max HR, W', and Pmax from sport settings
- `phase`: inferred block label, pattern, intent, confidence, and source
- `load`: total planned load, weekly progression, peak week, lowest week, and deload percentage
- `targetStatus`: race/target event summary and readiness signal
- `forecast`: CTL/ATL/TSB load forecast across planned days
- `decision`: directive, ADE score, penalties, and training guidance
- `sessions`: high-level counts by session classification
- `dayAdjustments`: per-day rules such as recovery overrides or target-event protection
- `weeks`: week roles such as reentry, build, overload, or deload
- `plannedSessions`: classified event-by-event planning output, including indoor Z2 workout profiles when applicable
- `warnings`: risky combinations or insufficient context warnings

### Interpretation

- The planner is designed around completed-load tolerance plus event heuristics, not a full workout-builder engine.
- Long and aerobic endurance sessions may include an indoor-friendly `workoutProfile` with Z2 waves and HR-control valleys.
- Planned session titles use simple name heuristics for common structures such as `40/20`, `30/15`, and `over/under` workouts when those patterns are present in the event name.
- `forecast` is the fast planning view for expected CTL/ATL/TSB trajectory, not a guarantee of adaptation outcome.
- `decision` is the main top-level recommendation surface for downstream agents.
- When planned events are resolved and include `workoutDoc`, `analysis plan` can derive cycling load locally from `%ftp` power steps if the event is missing `icuTrainingLoad`. Server-provided event load remains authoritative when present.

### Current Limitations

- The CLI does not currently add adaptation context, even though the library supports it.
- Event classification depends on available event category, timing, and load hints; sparse events reduce forecast quality. Local planned-load fallback requires resolved `workoutDoc` data and FTP from sport settings.
- Explicit date overrides must be supplied in complete pairs for history and plan windows.

## analysis adaptation

### Purpose

Measure recent adaptation by combining power-curve comparison, power-model anchors, sport settings, recent activities, and wellness lactate context.

### CLI Contract

```bash
icu analysis adaptation \
  [--oldest DATE --newest DATE | --days N] \
  [--type Ride] [--curves 42d,365d] [--filters FILTERS] [--newest DATE] \
  [--limit N] [--activity-fields CSV]
```

Defaults:

- `--days 28`
- `--type Ride`
- `--curves 42d,365d`

### Upstream Data

The CLI fetches:

- completed `activities` for the analysis date range
- `power-curves` for the selected sport and curve windows
- `mmp-model` for the selected sport
- `sport-settings` for the selected sport
- `wellness` for the same date range

It then runs `AnalyzeWellness` and passes the result into `AnalyzeCyclingAdaptation`.

### Output Sections

- `scope`: date range, curve count, activity count, and `timezone`/`timezoneSource` metadata
- `powerAnchors`: FTP, CP, W', and Pmax with source attribution
- `powerCurveDeltas`: duration-by-duration comparison between current and baseline curves
- `systemStatus`: improving/stable/declining rollup across curve deltas
- `lactate`: the wellness-derived lactate calibration summary
- `phaseSummary`: recent-vs-previous load segment comparison and phase trend
- `warnings`: for example, missing comparable curves

### Interpretation

- With the default `--curves 42d,365d`, the analyzer treats the shorter window as the current curve and the longer window as the baseline.
- Power anchors prefer the MMP model when available and fall back to sport settings otherwise.
- `phaseSummary` provides the recent load-segment context needed to interpret whether curve changes look productive or stale.

### Current Limitations

- At least two comparable curves are required for `powerCurveDeltas`.
- If the requested curve windows do not overlap in meaningful durations, the command may return warnings instead of comparisons.
- Lactate context comes from wellness records only; if wellness coverage is sparse, adaptation context is narrower.
- Known, not yet fixed: `scope.activities` here is a raw `len(activities)` count and is not yet filtered against D-24 STRAVA stubs the way `analysis cycling`/`analysis microcycle` were in wave-2 (this command never received a `schemaVersion`, D-7, and was out of wave-2's scope). Treat this command's activity count as potentially inflated by stub-shaped records until a future wave addresses it.

## analysis microcycle

### Purpose

Experimental read-only diagnostic command for the current or selected training
microcycle. The command is designed as a deterministic data and analysis
contract for a lightweight LLM/skill layer above the CLI. It maximizes
structured evidence, warnings, confidence, and data-quality context without
prescribing workouts or mutating athlete data.

`analysis micro` is a short alias for this command. It replaces the former
experimental per-activity micro-analysis command.

### CLI Contract

```bash
icu analysis microcycle \
  [--date DATE | --week DATE | --from DATE --to DATE] \
  [--json] [--full] [--no-plan] [--no-wellness] \
  [--sport-type Ride] [--timezone TZ]
```

With no date flags, the command analyzes the current Monday-Sunday
microcycle. `--date` and `--week` select the ISO week containing the supplied
date. `--from` and `--to` select an explicit custom range and must be supplied
together. Conflicting date selectors and inverted ranges are rejected.

Timezone precedence is `--timezone`, then the athlete profile `timezone`
(`timezoneSource: "athlete"`), then the system timezone. Only the system
fallback emits the "missing athlete timezone" data-quality warning.

Wellness is read only up to today (in the resolved timezone), never up to the
end of the selected week: Intervals.icu returns projected CTL/ATL rows for
future dates, and the latest row drives the readiness state.

### Upstream Data

- Completed activities from `activities`, fetched with a 90-day lookback so
  the selected microcycle can be compared against previous 7/28/90-day load.
- Planned calendar events from `events` unless `--no-plan` is supplied.
  `WORKOUT` events count as planned sessions; notes are context only.
- Wellness records from `wellness` unless `--no-wellness` is supplied.
- Sport settings from `sport-settings/<sport-type>` for FTP and zone
  availability.

D-24 STRAVA stubs (5 keys, `_note` present, `source == "STRAVA"`) are stripped
from the fetched activities before any counting or matching happens (F-19):
`dataQuality`'s `completedSessions`, `powerAvailability`, and
`heartRateAvailability` all divide by the real-activity count, never by the
raw fetch length.

### Output Sections

- `microcycle` — selected start/end, timezone, partial status, elapsed and
  remaining days.
- `sources` — upstream API surfaces used or intentionally excluded.
- `dataQuality` — availability of activities, plan, power, heart rate,
  wellness, FTP, zones, timezone, and warnings.
- `plannedVsActual` — planned/completed/missed/remaining/extra sessions,
  planned vs actual load, and duration compliance when plan data exists.
- `load` — duration, distance, training load, activity count, CTL/ATL/TSB
  when available, monotony, strain, ACWR, daily load, and sessions.
- `intensity` — zone distribution, HR zone seconds, Z4+ session count,
  intensity density, and warnings.
- `wellness` — readiness availability, confidence impact, multichannel
  wellness state, positive/negative/missing signals, and coverage.
- `fatigueDurability` — fatigue state, long rides, decoupling, latest form,
  and evidence.
- `classification` / `adaptationSignal` — experimental diagnostic
  classification, confidence, rationale, supporting evidence, main positive
  signal, and main risk.
- `risks`, `evidence`, `openQuestions`, `confidence`, and `sideEffects`.

### Interpretation

JSON is the primary contract and is intended for skill/LLM consumption. Human
output is a brief inspection view only. The command may classify the
microcycle as `on_track`, `productive_overload`, `underloaded`, `overloaded`,
`recovery_needed`, `disrupted`, `data_limited`, or
`experimental_uncertain`, but the MVP uses conservative local heuristics and
low confidence when important inputs are absent.

The CLI does not prescribe future workouts. A report or adaptive-planning
layer may consume this output, combine it with conversational context, and
decide how to present coaching guidance.

### Current Limitations

- Marked experimental: field names and classification heuristics may evolve
  before this is treated as a stable contract.
- Planned-vs-actual matching is date-level in the MVP; it does not deeply
  compare workout structure.
- Timezone comes from `--timezone`, the athlete profile, or system fallback.
- The command reads API data but does not sync external services or read
  local caches.
- Missing plan, wellness, FTP, zones, HR, or power data lowers confidence
  rather than being hidden behind confident conclusions.

## analysis workout

### Purpose

Analyze one completed workout against its planned calendar workout context.
This command is designed for `plan vs execution` review: it does not treat the
plan as absolute truth, but it does make the planned intent explicit before
interpreting the executed activity.

### CLI Contract

```bash
icu analysis workout <activity-id> \
  [--event-id ID] [--calendar-id ID] [--sport-type TYPE] \
  [--match-window-hours N] [--stream-types CSV]
```

Defaults:

- `--match-window-hours 24`
- `--stream-types watts,heartrate,cadence`
- `--sport-type` inferred from the activity type, falling back to `Ride`

### Upstream Data

The CLI fetches:

- the completed activity by ID
- detected activity intervals from `activity/<id>/intervals`
- activity streams from `activity/<id>/streams`
- sport settings for FTP/LTHR target resolution
- either the explicit `--event-id` or nearby resolved calendar events around
  the activity local date

### Output Sections

- `scope`: activity ID, matched event ID, and match window.
- `activity`: completed session rollup using the same `CyclingSession` contract
  as `analysis cycling`.
- `match`: selected calendar event, confidence, score, reasons, alternates,
  pairing `source`, and `compliance` — see **Pairing and Compliance (D-21)**
  below.
- `plan`: matched event summary plus expanded planned workout steps when
  `workoutDoc` is available.
- `execution`: stream/interval-derived micro-analysis, including warmup,
  cooldown, repeatability, and zone alignment when source data is present.
- `comparison`: session duration/load/intensity deltas, planned vs executed
  work-rep count, step comparisons, repeatability, and zone alignment.
- `warnings`: missing or low-confidence sources such as missing plan, streams,
  intervals, FTP, LTHR, or structured workout steps.

### Pairing and Compliance (D-21)

`match.source` names how `match.eventId` was chosen, and is always emitted:

| `source` | Meaning |
| --- | --- |
| `server_paired_event_id` | The activity's own `paired_event_id` field (server-authoritative) named this event. Trusted even outside `--match-window-hours` — the server already knows the pairing, so the heuristic's time window does not gate it. |
| `heuristic` | No server-authoritative pairing was present; the date/name heuristic selected this event. |
| `not_a_record` | The activity is a D-24 STRAVA stub (5 keys, `_note` present, `source == "STRAVA"`). Pairing is not attempted on either path — a stub is not a session, so no plan-versus-actual comparison is fabricated for it. |
| `unmatched` | No pairing, server-authoritative or heuristic, was found. |

`match.disagreement` is `true` only when a server-authoritative pairing exists and the independently-run date/name heuristic would have selected a *different* event — a signal that the heuristic's assumptions do not hold for this activity, not an error by itself.

`match.compliance` is a pointer, emitted **jointly with a pairing or not at all** (D-21): it is `nil` whenever `match.eventId` is `0` (`source` is `not_a_record` or `unmatched`), regardless of what the activity's own raw compliance value was. This closes a live-measured defect (F-18): the raw API compliance field reads `0.0` on every unpaired activity, which is indistinguishable from a genuinely poor 0% compliance to a plan — there is no plan to be compliant with when there is no pairing, so the field is omitted rather than reported as a fabricated `0`. When a pairing *does* exist, a genuine `compliance: 0.0` is reported as present (the pointer is non-nil, the JSON key appears with value `0`) — omission means "not applicable," not "zero."

### Interpretation

- `match.confidence` should be checked before treating the selected event as
  authoritative.
- `plan.steps` is expanded from nested repeats so consumers can reason about
  work and recovery steps directly.
- `comparison.repCount` compares planned work steps against executed `WORK`
  intervals when interval data exists.
- The command degrades to session-level comparison when structured plan or
  interval/stream data is unavailable.

### FTP Attribution

The FTP used to compute `execution`'s power zones is resolved per completed
activity (D-10's **activity** partition — distinct from the **event**
partition `rebalance`/`plan` use for calendar events; see
`docs/rebalance.md`). Concretely, `Ride` is a first-class default-outdoor
class (class 2) here, with `Trainer` deciding otherwise, whereas a `Ride`
*event* falls into the broader "ambiguous" class 3 alongside every other
non-inherent cycling type. Do not assume a `Ride` activity and a `Ride` event
resolve by the same rule; they intentionally do not.

The activity partition is the one partition where confidence can land at
`low`: an ambiguous cycling type (not `Ride`, which defaults outdoor) with no
discriminating flag resolves `Source: unknown`, `Confidence: low`. Per D-10c,
read/analysis paths never refuse on `low` — they proceed and surface it as a
warning instead, since the direction-of-safety cost of a wrong read-only FTP
is a wrong number in a report, not a wrong prescribed watt target. `medium`
confidence (type-default or flag-asserted) also proceeds; it is weaker
evidence than `high` (type-inherent, "this type cannot be otherwise"), not a
reason to distrust the number. `ftp_confidence_low` names this `low` case but
is not a refusal gate at this read-only site — see `docs/rebalance.md` for
where a `low`-equivalent situation is refused at write time instead (the
mixed-environment-week gate; the event partition itself never produces
`low`, so there is nothing to gate here).

`not_a_record` (a STRAVA stub-shaped activity — five keys, `_note` present,
`source == "STRAVA"`, D-24) is a distinct `ResolveFTP` outcome from
`Applicable: false` (a type FTP genuinely does not apply to, such as
`Walk`/`Run`/`WeightTraining`): setting `FTPRequest.StubShape` short-circuits
classification entirely — before `Type` is even inspected — whereas a
not-applicable type is classified and then explicitly reported as having no
FTP-dependent values. Neither case fabricates a zero or a default FTP.
**Caller note:** detecting the STRAVA-stub shape itself is the caller's job;
`ResolveFTP` only classifies once told. `analysis workout` sets `StubShape:
inputs.Activity.StubShape` on its `ResolveFTP` call, so a stub-shaped
`Activity` value passed into this code path **is** short-circuited to
`not_a_record` rather than priced through the ordinary activity partition —
that much is code-accurate. It is not, however, something you can observe
by running `analysis workout <stub-id>` against a real STRAVA stub (Stage 6
R13): that command's single-activity fetch (`GET /activity/{id}`) 404s/422s
upstream before an `Activity` value even exists to classify, so this
short-circuit never actually executes for that command against real data.
The D-24 stub classification **is** reachable, and is where the 73-of-303
split (see **Validity, Ignore Flags, and STRAVA Stubs** above) was actually
measured: the bulk/list-shaped fetches behind `analysis cycling`
(`AnalyzeCyclingActivities`'s own `activity.StubShape` filter) and
`analysis microcycle` (`ExcludeStubActivities`) pull many activities per
call, and upstream includes stub-shaped records in that bulk response
without erroring, unlike the single-activity endpoint.

`analysis workout` also emits its resolution's provenance directly:
`ftpSource` and `ftpConfidence` sit alongside `activity.ftp` at the top level
of the payload (`WorkoutExecutionAnalysis`, not per-session — there is only
one activity per invocation), and a `ftp_confidence_low` entry is added to
`warnings` whenever `ftpConfidence` resolves to `low` (D-10c/T1b.5). Both
fields are `omitempty`: they are absent, not empty-stringed, when
`ResolveFTP` reports `Applicable: false` or short-circuits to `not_a_record`,
since there is no FTP provenance to report in either case.

The `"ftp is missing"` warning (distinct from `ftp_confidence_low` above) is
gated on that same `Applicable` flag (Stage 6 R12), not on whether the
command's own effective FTP number happens to be zero: the effective number
falls back to the activity's raw `icuFtp` field when `ResolveFTP` resolves no
watts, and upstream populates that raw field inconsistently across
not-applicable types (e.g. present on some `Walk` records, absent on some
`Run` records) — an artifact of what a given record happened to carry, not
of whether FTP applies to the type. `Walk`/`Run`/`WeightTraining`/etc. never
emit `"ftp is missing"` for this reason, matching the "explicitly reported as
having no FTP-dependent values" promise above; the warning still fires for
an applicable type (e.g. `Ride`) that genuinely has no resolvable FTP.

`analysis cycling` emits a single top-level `ftpDiscriminatorConflicts` key
(`omitempty`, absent when there is nothing to count), mirroring
`rebalance`/`plan`'s event-side `ftpDiscriminatorConflicts` but counting
**activity**-side ambiguous-type-with-flag-asserted-indoor conflicts instead
of event-side ones — it never adds a per-session key, since `CyclingSession`
is shared by four different output shapes and a field there would leak into
all of them. As of this writing the count is **structurally**, not just
empirically, always zero: `Activity.Trainer` is not an exposed Go field
anywhere in this codebase, so an activity-side `ResolveFTP` call can never
observe a `flag_asserted` source (unlike the event partition, where
`Event.Indoor` is exposed and genuinely drives `flag_asserted` sometimes).
The mechanism is implemented and tested against that day; exposing
`Activity.Trainer` is a separate, larger change than this fix.

Each `analysis` payload carries its own `schemaVersion` as the first JSON key
(D-7), the same marker convention as `rebalance`'s `icu.rebalance.v3` and
`plan`'s `icu.plan.v2`. The four payloads version independently because they
change independently — a meaning change in one does not imply a bump in the
others:

| payload | `schemaVersion` |
| --- | --- |
| `analysis cycling` (`CyclingAnalysis`) | `icu.analysis.cycling.v1` |
| `analysis coaching` (`CoachingContext`) | `icu.analysis.coaching.v1` |
| `analysis microcycle` (`MicrocycleAnalysis`) | `icu.analysis.microcycle.v1` |
| `analysis workout` (`WorkoutExecutionAnalysis`) | `icu.analysis.workout.v1` |

The field is always populated — never `omitempty` — because an absent
version is indistinguishable from an old consumer's blind spot, the exact
ambiguity this marker exists to remove. A consumer should read
`schemaVersion` before parsing the rest of the payload and refuse (or fall
back to a compatibility path) on an unrecognized value rather than assume
today's field shape holds across a version it has not seen.

Cross-period comparison note: like `rebalance`/`plan`'s per-session zone
watts, this command's FTP is resolved per activity, not fixed for a whole
history range. A time-in-zone or intensity-distribution comparison across
activities that resolved different FTPs is comparing against different watt
boundaries and is not a like-for-like comparison.

### Current Limitations

- Step matching is best-effort. It uses planned order and detected `WORK`
  intervals; unsupported freeride or until-lap behavior is reported through
  warnings rather than invented precision.
- `workout_file_base64` decoding currently supports JSON workout docs only.
- The command is read-only and does not update calendar notes or planned
  workouts.

## Emit-Only Activity Fields (Wave-4/A2)

Wave-4 typed nine fields on `Activity` that intervals.icu's API already returned but this CLI previously dropped silently on the unmarshal/re-marshal round trip. **These fields are emit-only (D-1): they are present on the wire for a caller with programmatic needs to read verbatim, but no `analysis`/`rebalance`/`plan` command consumes any of them, and no command's output number changes as a result of this wave.** They surface only through commands that emit the raw `Activity` JSON directly — `activities list`, `activities get`, `activities search-full` (see `docs/cli-reference.md`) — not through `analysis cycling`'s per-session `sessions` rows or any other analysis output, which deliberately were not extended so the wave-2 golden-corpus regression artifacts (`testdata/wave2_live_corpus_cycling_analysis.golden.json`) stay byte-for-byte unchanged. `AGENTS.md`'s "Emit-Only Field Classification" section governs the exercise required before any future change may promote one of these into a computed output number.

| Field (JSON) | Go field | Type | Non-null on live corpus (230 real activities) |
| --- | --- | --- | --- |
| `polarizationIndex` | `PolarizationIndex` | `*float64` | 178/230 |
| `ssCp` | `SSCP` | `*float64` | 151/230 |
| `ssWPrime` | `SSWPrime` | `*float64` | 151/230 |
| `ssPMax` | `SSPMax` | `*float64` | 151/230 |
| `carbsUsed` | `CarbsUsed` | `*int` | 145/230 |
| `carbsIngested` | `CarbsIngested` | `*int` | 0/230 |
| `icuRollingFtpDelta` | `RollingFTPDelta` | `*int` | 155/230 |
| `powerField` | `PowerField` | `*string` | 151/230 |
| `icuHrr` | `HRR` (`*HRRMeasurement`, 8-key closed shape) | `*HRRMeasurement` | 49/230 |

**Two disclosures, not silently absorbed:**

1. **`icuHrr.averageWatts` is `null` on all 49 non-null `icuHrr` records in the live corpus — every single one.** The member is correctly typed as a nullable pointer (`*int`) per the schema, but there is currently **zero live evidence it is ever populated**; only a synthetic test fixture can exercise a non-null value (see `TestActivityUnmarshalHRRAverageWattsOnlyExercisedSynthetically`). This is the same shape of claim as wave-2's ignore-flag inertness statement above — say so, do not report a populated `averageWatts` as an observed live effect.
2. **`carbsIngested` is non-null in 0 of 230 real activities.** Present in the schema and correctly emitted when populated, but this account has never once recorded a value for it. Emitting it is still correct for an emit-only surface; there is simply no live evidence it ever carries a value.

**Dropped, with reasons (not typed):**

| Field | Reason |
| --- | --- |
| `pm_battery` | Absent from both the schema and the live payload. |
| `fitness_load` | Absent from both the schema and the live payload. |
| `fatigue_load` | Absent from both the schema and the live payload. |
| `carbs_per_hour` | This is an `Event` **planned target**, not an activity **actual** — a different entity, out of scope for `Activity`'s emit-only surface. |

**Negative `decoupling` constraint.** `Activity.Decoupling` (already existing, not new) takes negative values on the live corpus (49 negative values observed, minimum −43.877388). Any consumer — existing or future — that assumes `decoupling >= 0` is wrong; a negative value is a real, meaningful decoupling reading (HR drifting down relative to power/pace), not an error sentinel.

## Choosing The Right Analysis Command

- Use `cycling` for completed-work review, intensity distribution, and durability signals.
- Use `wellness` for readiness, coverage, and physiology trend inspection.
- Use `plan` for future-block structure, load alignment, and event-aware decision guidance.
- Use `adaptation` for curve-based improvement/decline analysis tied to recent load context.
- Use `microcycle` for LLM-ready microcycle diagnostics; `micro` is its alias.
- Use `workout` for one completed workout against planned-event execution review.
