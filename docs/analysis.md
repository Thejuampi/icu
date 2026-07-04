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
icu analysis cycling [--oldest DATE --newest DATE | --days N] [--fields CSV] [--limit N]
```

Defaults:

- `--days 28`
- built-in activity field contract from `cmd/icu/cmd_analysis.go`

### Upstream Data

The CLI fetches `activities` for the requested date range and passes them to `AnalyzeCyclingActivities`.
The default activity field contract includes load, power-anchor, durability, weather, altitude, gradient, and lactate-related fields required by the analyzer.

### Output Sections

- `scope`: analyzed activity counts, date range, and `timezone`/`timezoneSource` metadata
- `state`: current load-pressure classification and directive
- `volume`: time, distance, and elevation totals
- `powerAnchors`: FTP, CP, W', Pmax, and rolling FTP when available
- `environment`: temperature, wind, altitude, gradient, and strain context
- `load`: total load, monotony, strain, acute/chronic view, CTL/ATL/TSB, and daily load series
- `intensity`: weighted intensity and zone totals
- `durability`: decoupling, efficiency factor, and variability summaries
- `anaerobic`: work above FTP, W' depletion, and repeatability support metrics
- `performance`: repeatability, durability, neural-density, and efficiency classifications
- `sessions`: per-session rollup for qualifying cycling activities
- `warnings`: gaps such as missing power-anchor comparisons or sparse data

### Interpretation

- Only cycling activities are included in the analysis totals.
- The load section is intended as a compact readiness snapshot, not a full planning model.
- Environmental summaries help explain why similar loads may have different physiological cost.
- Session-level W' and decoupling metrics are useful for follow-up inspection, not for single-number decision making.

### Current Limitations

- Output quality depends on Intervals.icu providing the power, weather, and zone fields requested by the default contract.
- If you override `--fields`, you are responsible for preserving any inputs needed by downstream sections.
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
- `history`: completed weekly load/hours, recent tolerance, current state, and completed-week series
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

If `--timezone` is omitted, the CLI uses the system timezone and emits a
data-quality warning because the current config and athlete types do not
expose a reliable athlete timezone field.

### Upstream Data

- Completed activities from `activities`, fetched with a 90-day lookback so
  the selected microcycle can be compared against previous 7/28/90-day load.
- Planned calendar events from `events` unless `--no-plan` is supplied.
  `WORKOUT` events count as planned sessions; notes are context only.
- Wellness records from `wellness` unless `--no-wellness` is supplied.
- Sport settings from `sport-settings/<sport-type>` for FTP and zone
  availability.

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
- Timezone comes from `--timezone` or system fallback because no reliable
  athlete timezone source exists in the current local types/config.
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
- `match`: selected calendar event, confidence, score, reasons, and alternates.
- `plan`: matched event summary plus expanded planned workout steps when
  `workoutDoc` is available.
- `execution`: stream/interval-derived micro-analysis, including warmup,
  cooldown, repeatability, and zone alignment when source data is present.
- `comparison`: session duration/load/intensity deltas, planned vs executed
  work-rep count, step comparisons, repeatability, and zone alignment.
- `warnings`: missing or low-confidence sources such as missing plan, streams,
  intervals, FTP, LTHR, or structured workout steps.

### Interpretation

- `match.confidence` should be checked before treating the selected event as
  authoritative.
- `plan.steps` is expanded from nested repeats so consumers can reason about
  work and recovery steps directly.
- `comparison.repCount` compares planned work steps against executed `WORK`
  intervals when interval data exists.
- The command degrades to session-level comparison when structured plan or
  interval/stream data is unavailable.

### Current Limitations

- Step matching is best-effort. It uses planned order and detected `WORK`
  intervals; unsupported freeride or until-lap behavior is reported through
  warnings rather than invented precision.
- `workout_file_base64` decoding currently supports JSON workout docs only.
- The command is read-only and does not update calendar notes or planned
  workouts.

## Choosing The Right Analysis Command

- Use `cycling` for completed-work review, intensity distribution, and durability signals.
- Use `wellness` for readiness, coverage, and physiology trend inspection.
- Use `plan` for future-block structure, load alignment, and event-aware decision guidance.
- Use `adaptation` for curve-based improvement/decline analysis tied to recent load context.
- Use `microcycle` for LLM-ready microcycle diagnostics; `micro` is its alias.
- Use `workout` for one completed workout against planned-event execution review.
