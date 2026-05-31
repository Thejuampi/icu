---
name: cycling-performance-analysis
description: 'Use when generating cycling training reports or training plans from Intervals.icu data with the icu CLI: weekly report, 4-week plan, dry run, load pressure, recovery priority, CTL/ATL/TSB, ACWR, monotony, strain, HRV, wellness, power curves, W prime, durability, adaptation, planned events, cycling fitness improvement.'
argument-hint: 'report type, date range, athlete id, and focus such as load/recovery/adaptation'
---

# Cycling Performance Analysis

Use this skill to produce cycling coaching reports from the `icu` CLI and to evolve the tool whenever a required metric is missing.

The goal is not to imitate Montis internals. The goal is to make this repository's tool provide the numeric context needed for an AI coaching layer to reason about cycling fitness, fatigue, recovery, adaptation, and planning.

## Operating Rule

Prefer measured Intervals.icu data and deterministic calculations from `icu` over model inference. Use AI for interpretation, prioritization, and coaching language after the data contract is satisfied.

If a requested section requires a metric that `icu` cannot currently fetch or compute:

1. Name the missing metric clearly.
2. Identify the Intervals.icu source field or endpoint needed.
3. Add support to the tool with tests before presenting the metric as available.
4. If support cannot be added in the current turn, mark the report section as unavailable rather than inventing it.

If the investigation exposes a CLI bug, parser mismatch, auth ambiguity, or data-quality problem, fix that side quest with tests first, then return to the coaching request. The coaching layer is only as good as the data surface beneath it.

## Lessons Learned

- Validate auth source before blaming the API. Run `icu config diagnose --athlete-id ATHLETE_ID` when a command that used to work starts returning unauthorized or empty responses. Environment credentials can shadow config credentials.
- Never print API keys. Use diagnostic status, length, trim checks, and fingerprints only.
- Intervals.icu endpoints can return snake_case fields when `fields` is used, even when local DTOs use camelCase JSON tags. If a report shows suspicious zeros for dates, duration, TSS, IF, CTL, ATL, or planned-event load, verify parsing before interpreting the result.
- Spot-check one raw activity or event after adding new fields. A plausible report with zero-valued inputs is worse than an unavailable section.
- Treat existing planned workouts as the baseline plan. For planning requests, first evaluate whether the existing calendar is coherent before proposing replacement workouts.

## Primary Workflow

1. Resolve athlete and range.
   - Default weekly report: 7 days ending today in the athlete timezone.
   - If the user gives dates, use absolute `YYYY-MM-DD` dates.
   - If multiple coached athletes are possible, ask for athlete selection.

2. Collect the current numeric cycling summary.

```bash
icu analysis cycling --oldest START --newest END --athlete-id ATHLETE_ID
icu analysis wellness --oldest WELLNESS_START --newest END --athlete-id ATHLETE_ID
icu analysis plan --history-oldest HISTORY_START --history-newest END --plan-oldest NEXT_START --plan-newest NEXT_END --athlete-id ATHLETE_ID
```

3. Collect supporting Intervals.icu context as needed.

```bash
icu athlete show --athlete-id ATHLETE_ID
icu sports get Ride --athlete-id ATHLETE_ID
icu events list --oldest NEXT_START --newest NEXT_END --athlete-id ATHLETE_ID
icu curves power --type Ride --curves 42d --athlete-id ATHLETE_ID
icu curves mmp --type Ride --athlete-id ATHLETE_ID
```

For planning questions, use a 12-week lookback and the next 4 weeks of events unless the user requests a different horizon.

```bash
icu analysis cycling --oldest HISTORY_START --newest END --athlete-id ATHLETE_ID
icu analysis wellness --oldest WELLNESS_START --newest END --athlete-id ATHLETE_ID
icu analysis plan --history-oldest HISTORY_START --history-newest END --plan-oldest NEXT_START --plan-newest NEXT_END --athlete-id ATHLETE_ID
```

Planning dry-run checklist:

- Summarize the last 12 weeks by weekly TSS, hours, session count, high-intensity days, long endurance sessions, mean IF, and decoupling.
- Classify the current state from CTL/ATL/TSB plus wellness: e.g. `load_accepting`, `load_pressure`, or `recovery_priority`.
- Group the next 4 weeks of planned events by ISO week and compare planned TSS/hours against recent tolerance with `analysis plan`.
- Review week roles and session classifications, not just TSS: `reentry`, `build`, `overload`, `deload`, `high_intensity`, `tempo_threshold`, `long_endurance`, `recovery`, `aerobic`, `opener`, `rest`.
- Review `execution.recommendedTitle` and `execution.cues` for each workout. Use short representative titles such as `4x5 VO2Max`, `2x20 SS`, `3h Z2`, or `3x15 Tempo` when the session structure supports them.
- Use encouragement cues for high-intensity, tempo/threshold, and selected endurance work. Use restraint/recovery cues for easy or recovery rides; do not add hype to Z1 recovery sessions.
- For indoor Z2 sessions, avoid flat prescriptions like `180m @ steady watts`. Prefer varied aerobic structures: 4-minute Z2 waves, 40-second HR-control valleys, low/mid/high Z2 rotation, occasional max-Z2 caps, and mid-Z2 shadow blocks that preserve aerobic intent.
- Preserve the current plan when it is structurally sound; adjust execution rules before rewriting workouts.
- Make intensity days conditional when physiology is `WATCH` or TSB is meaningfully negative.

Use wider ranges for trend questions:

```bash
icu activities list --oldest HISTORY_START --newest END --athlete-id ATHLETE_ID --fields id,name,start_date_local,type,moving_time,distance,icu_training_load,icu_intensity,decoupling,icu_efficiency_factor,icu_variability_index,icu_joules_above_ftp,icu_max_wbal_depletion,icu_ctl,icu_atl
```

4. Check the data contract before writing prose.
   - Required for load/recovery report: athlete, sport settings, 7-day cycling analysis, 42-day wellness if available, next planned events if planning is discussed.
   - Required for adaptation: current power curves or MMP model plus historical activity context.
   - Required for W prime/repeatability: work above FTP and W balance fields in activities or activity intervals.
   - Required for heat/environment: weather or activity environmental fields. Do not infer heat stress from prose alone.
   - Required for 4-week planning: 12-week cycling analysis, 42-day wellness analysis, sport settings, and next 4 weeks of events.

5. Render the report from facts to interpretation.
   - Start with the headline state: e.g. `Load Pressure / recovery_priority`.
   - Separate raw metrics from interpretation.
   - Make confidence clear when data is sparse.
   - End with a concrete coaching directive and one useful reflection question.

## Report Sections

Use these sections when the data exists:

- `REPORT CONTEXT`: athlete, report type, period, timezone, sport, FTP/eFTP, LTHR/max HR, CTL/ATL/TSB.
- `TRAINING LOAD`: hours, TSS/load, distance, CTL/ATL/TSB, ACWR, monotony, strain, daily load, session list.
- `PHYSIOLOGY RESPONSE`: HRV, resting HR, sleep, subjective fatigue/stress/motivation, readiness, coverage.
- `PERFORMANCE INTELLIGENCE`: durability, decoupling, efficiency factor, W prime/use above FTP, intensity density, high-intensity days.
- `ADAPTATION`: power curve deltas, MMP model, strongest/weakest systems, likely training focus.
- `PLANNED BLOCK`: upcoming events and whether the plan matches the current state.
- `ADAPTIVE DECISION`: what to do next and why.

For the expanded field-by-field contract, use [report-data-contract.md](./report-data-contract.md). Treat it as the source of truth for whether a field is currently supported, partial, or missing.

## Metric Policy

- Use `TrainingLoad` as TSS/load when Intervals.icu exposes `icu_training_load`.
- Use `Intensity` as IF when Intervals.icu exposes `icu_intensity`.
- Use `WeightedAvgPower` as NP-like power when Intervals.icu exposes `icu_weighted_avg_watts`.
- Use `LatestForm = CTL - ATL` as TSB-style form when latest CTL/ATL are available.
- Use `acuteChronicWorkRatio` from `icu analysis cycling` as ACWR unless a better Intervals-native value is added.
- Do not call a field authoritative if it is derived locally; label it as local or heuristic where relevant.

## Planning Policy

- Use the athlete's existing calendar as the first candidate plan.
- Use recent completed weeks to define tolerance, not a generic progression template.
- When current state is `Load Pressure` or wellness is `WATCH`, keep build structure conservative: dose VO2/threshold minimally, protect endurance durability, and preserve recovery spacing.
- For a 4-week block, prefer a simple rhythm such as re-entry, build, overload, deload when it matches the athlete's recent load tolerance.
- Add day-level decision rules for HRV, sleep, resting HR, TSB, decoupling, and heat rather than pretending the plan is fixed regardless of recovery state.
- Make workout names human-scannable and device-friendly: representative interval structure first, training system second, extra context last.
- Keep device cue messages short enough to be read during effort. Preview what is coming, cue the purpose of the next block, and only encourage when the workout intensity merits it.
- Keep indoor Z2 profiles engaging without turning them into tempo. HR-control valleys are for drift management, not full recovery; max-Z2 peaks must stay below tempo/threshold intent.

## Development Loop

When adding support to `icu` for a missing metric:

1. Add a failing test in the functional core when possible.
2. Add or extend the data type in `types.go` or `analysis.go`.
3. Update `analysis cycling` only if the metric is cycling-specific and read-only.
4. Keep CLI output JSON stable and additive.
5. Run `go test ./... -count=1`, `go vet ./...`, and `golangci-lint run ./...`.

Current known gap: the repository-wide coverage gate is below 90% because the existing CLI command surface is broadly untested. Do not treat that as caused by a single analysis change.