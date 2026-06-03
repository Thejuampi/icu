# Analysis Commands

The `analysis` resource provides four read-only commands built on top of Intervals.icu data already exposed by the CLI.
All analysis output is JSON.

## Shared Behavior

- Date windows use UTC normalization in the CLI layer.
- `--oldest` and `--newest` must be provided together.
- If explicit dates are omitted, the CLI falls back to day-based defaults.
- Missing source metrics are represented as zeros, omitted fields, or warnings rather than fabricated estimates.
- These commands do not write back to Intervals.icu.

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

- `scope`: analyzed activity counts and date range
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

The CLI fetches `wellness` records for the requested range and passes them to `AnalyzeWellness`.

### Output Sections

- `scope`: record count, total day span, and date range
- `coverage`: percentage coverage for HRV, resting HR, sleep, and subjective wellness
- `hrv`: mean, latest, ratio, delta, and 7-day trend
- `restingHr`: mean, latest, delta, and trend
- `sleep`: mean, latest, delta, and trend
- `lactate`: mean, latest, trend, coverage, and local state
- `subjective`: fatigue, stress, soreness, and motivation means when present
- `load`: latest CTL, ATL, and derived TSB from wellness records
- `state`: overall physiology status with confidence and reasons
- `warnings`: missing-data or low-confidence cues

### Interpretation

- The command is primarily a coverage-aware readiness summary.
- `state` is intentionally conservative and depends on both signal direction and data completeness.
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
  [--sport-type TYPE] [--calendar_id ID] [--resolve] \
  [--activity-fields CSV]
```

Defaults:

- `--history-days 84`
- `--plan-days 28`
- `--sport-type Ride`
- history window ends on the current UTC date
- plan window starts on the next ISO block boundary used by the CLI

### Upstream Data

The CLI fetches:

- completed `activities` for the history window
- `sport-settings` for `--sport-type`
- `wellness` for the history window
- `events` for the plan window, optionally with `--resolve`

It then runs `AnalyzeTrainingPlanWithContext` with sport settings and wellness context.
The CLI currently passes `Adaptation: nil`, even though the library supports adaptation-aware planning context.

### Output Sections

- `scope`: history window, plan window, and event counts
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
- `forecast` is the fast planning view for expected CTL/ATL/TSB trajectory, not a guarantee of adaptation outcome.
- `decision` is the main top-level recommendation surface for downstream agents.

### Current Limitations

- The CLI does not currently add adaptation context, even though the library supports it.
- Event classification depends on available event category, timing, and load hints; sparse events reduce forecast quality.
- Explicit date overrides must be supplied in complete pairs for history and plan windows.
- `--calendar_id` uses an underscore because the parser matches exact flag names.

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

- `scope`: date range, curve count, and activity count
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

## Choosing The Right Analysis Command

- Use `cycling` for completed-work review, intensity distribution, and durability signals.
- Use `wellness` for readiness, coverage, and physiology trend inspection.
- Use `plan` for future-block structure, load alignment, and event-aware decision guidance.
- Use `adaptation` for curve-based improvement/decline analysis tied to recent load context.
