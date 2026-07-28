---
name: cycling-performance-analysis
description: 'Use when generating cycling training reports or training plans from Intervals.icu data with the icu CLI: weekly report, 4-week plan, dry run, load pressure, recovery priority, CTL/ATL/TSB, ACWR, monotony, strain, HRV, wellness, power curves, W prime, durability, adaptation, planned events, cycling fitness improvement.'
compatibility: opencode, claude
metadata:
  cli: icu
  domain: cycling-training
  source: project-agent-skill
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
- Review existing NOTE events on the calendar before rendering analysis. Notes carry coaching context, block structure, decision rules, and athlete-facing communication. Skimming only WORKOUT events misses the full picture. Collect them explicitly alongside planned sessions.
- **Do not analyze a planned session in isolation.** If the user asks about a workout that sits inside a training block, first retrieve the planned event and the relevant NOTE events for that day/week/block, then compare planned intent vs executed data. If that context is missing, say the analysis is incomplete and fetch it before giving conclusions.
- For weekly reports and planning reviews, prefer `icu analysis coaching` as the primary data contract. It bundles athlete, sport settings, cycling analysis, wellness analysis, plan analysis, events, NOTE context, and optional adaptation, reducing the workflow to one CLI call in most cases.
- For single completed workouts inside an active plan, run `icu analysis workout ACTIVITY_ID --athlete-id ATHLETE_ID` as the second data contract. Use its `match`, `plan`, `execution`, `comparison`, and `warnings` sections before writing coaching prose.
- **Do not give generic recommendations that bypass the plan.** Recommendations must be framed as `plan-preserving` first: confirm whether the executed session matched the intended role, then adjust execution rules or fallback options before suggesting a different workout or extra recovery.
- After producing a plan review, write key findings back as NOTE events on the calendar so future analyses and the athlete can reference them. Keep notes compact: Intervals.icu NOTE events behave as all-day notes and long descriptions can fail upstream with HTTP 500. If the plan is approved, create: (a) one short block-overview note with week roles, load targets, decision thresholds, and FTP; (b) weekly focus notes with session classification and conditional rules; (c) separate short ALT notes when the decision tree is long.
- `icu analysis` commands compute default date ranges in **UTC**. When the athlete is not in UTC, the implied "today" can shift by up to a day relative to the athlete's local date. Always use explicit `--oldest`/`--newest` dates in the athlete's local timezone for daily-accurate reports, and verify the `timezone`/`timezoneSource` fields in the output to confirm which timezone was used.
- Do not hardcode absolute HRV readiness thresholds in coaching logic. HRV is athlete-specific and changes over time. Use `icu analysis wellness` dynamic HRV fields (`recentMean`, `baselineMean`, `baselineMad`, `zScore`, `zScoreSource`) and interpret HRV state from moving baseline context; treat the raw latest/mean `ratio` as context, not a standalone stop/go rule.
- Treat `analyses.wellness.sleep.scoreName` as the source-of-truth for the recovery score. Prefer Zepp `zepp_hybridcharge`/BioCharge whenever the CLI provides it. Treat Intervals wellness `sleepScore` as fallback only, and read `sleep.fallbackScoreName` plus `warnings` before assuming the recovery signal is Zepp-backed.
- The `--desc` text parser in Intervals.icu requires a specific format. **Never use inline `- NxMm XX%` syntax** — the API will not expand it into a repeat block. Instead use the **multi-line repeat block format**:

```
# CORRECT — repeats parsed as nested steps:
3x
  - 15m 88-92%
  - 5m 55-60%

# WRONG — inline repeats are treated as a single step:
- 3x15m 88-92% with 5m rest
```

Each `-` step line is `duration zone%` (e.g., `15m 55-72%`). A bare `Nx` on its own line starts a repeating block; the indented `-` lines beneath it define the steps to repeat. Do not include rest duration text like `with 5m rest` in the step line — rest is its own indented step.
- The `workout_doc` and `workout` fields in the EventEx schema are **read-only** in practice. The API ignores them in POST/PUT requests and only generates `workoutDoc` from the `--desc` text description. Workout structure (including step-level `text` cues) can only be set via the description text parser. The text parser strips arbitrary text annotations — only power/HR zones and keywords (warmup, cooldown, etc.) are preserved as step text. Step-level coaching cues must be added manually in the Intervals.icu UI after creation.
- Before creating or updating planned workouts for load-sensitive blocks, calculate local load with `icu workouts calculate --ftp FTP --desc DESC`. This is mandatory for deload weeks, ramp correction, and any plan option where weekly TSS/CTL ramp is part of the decision.
- **Dual description format when writing calendar workouts.** Local `icu workouts calculate` accepts strict steps like `- 15m 55-85%` and `Nx` blocks; it rejects prose and does not require `Ramp`/`FTP` tokens. Intervals.icu's upstream parser reliably expands repeats only with the multi-line `Nx` form, blank lines between sections, and often `Ramp` + `FTP` suffixes (e.g. `- 15m Ramp 55-85% FTP`). Workflow: (1) design structure, (2) `workouts calculate` on the local-clean desc, (3) `events create/update` with the Intervals-friendly desc plus explicit `--moving-time` / `--training-load` from the local estimate (or `--calculate-load --ftp` when the local desc is used end-to-end), (4) verify server `workoutDoc` has `reps > 1` on interval blocks and that server load/time match local within tolerance. If server load collapses on a structured session, the upstream parser dropped the repeats — fix the desc format, do not accept a flat single-pass parse.
- **Session structure is varied by default.** Never ship a plan whose aerobic/easy/long sessions are only `warmup → one long constant block → cooldown`. That shape is a failure mode, not a baseline. See Planning Policy → Session Structure Norm.
- **Use the session library.** Canonical templates live in [session-library.md](./session-library.md). Pick a template by role, scale it, calculate load, then write the calendar. Do not freehand a flat session when a library model fits. For this athlete, default Z2 is `z2_hr_control_waves` (2–3 min mid/high Z2 + 30–40s valley when HR rises); long durable HR ceiling is athlete-specific (~≤140 bpm), not the full Intervals Z2 chart ceiling.
- Use `events create/update --calculate-load --ftp FTP --desc DESC` when writing workouts that should carry locally calculated `moving_time` and `icu_training_load`. The flag is explicit by design: if `--calculate-load` is absent, the CLI must not assume or overwrite load values.
- For multi-session calendar writes, prefer `icu plan show --file plan.json --intent-file intent.json [--type Ride] [--now-date DATE]`, then `icu plan preview --file plan.json`, inspect/edit operations, then `icu plan accept --file plan.json`. Intent sessions are explicit (name/date/desc/descLocal); plan does not invent a session library. `desc` goes to Intervals; `descLocal` is for local load calc. Matching uses uid or date+name. Cancel defaults off.
- For multi-day load redistribution (not explicit session lists), prefer `icu rebalance show --file FILE --oldest START --newest END --target-load LOAD --target-tolerance TOL --start-time HH:MM --min-session-minutes MIN --duration-step-minutes STEP`, optional `icu rebalance preview --file FILE`, inspect/edit the JSON, then run `icu rebalance accept --file FILE`. The dry run is non-mutating; accept applies only explicit pending operations and checks source hashes for stale calendar events. Do not rely on hidden fallback intensity, timing, duration, or allocation defaults; provide explicit constraints when athlete history or sport settings are sparse.
- Treat local planned-load calculation as the pre-write precision check and Intervals.icu's returned event as the post-write calibration check. If the response contains `{event, estimate, warnings}`, inspect warnings before trusting the event for plan math.
- The local parser supports the same strict line-oriented workout format used above: `- 10m 55-75%`, `- 5m 90%`, and repeat blocks like `3x` with indented child steps. Do not use free prose when exact TSS is required.
- When completed ride power is incomplete or corrupt (e.g. power meter battery dies mid-ride), do not infer replacement TSS from feel. First fetch `icu_training_load`, `hr_load`, `hr_load_type`, `trimp`, HR zones, power zones, average/weighted watts, IF, VI, CTL/ATL, and duration. If HR coverage is good and Intervals exposes `hrLoad` with a known `hrLoadType` such as `HRSS`, use that API-provided HR-derived load as the candidate replacement for `icuTrainingLoad`; document the original load, replacement load, and reason. If power coverage is complete and plausible, keep power-based `icuTrainingLoad` as source-of-truth and treat `hrLoad`/`trimp` as secondary context only.
- **Power zero ≠ power missing.** After a mid-ride PM death, head units often keep writing **watts=0** while **cadence becomes null**. Coasting while the meter is alive is watts=0 with cadence present and 0 — that is a true zero and must not be filled. Dual-sided meters also stop reporting **`left_right_balance`** after death while the first half keeps real watts + RPM + L/R. Prefer L/R for gap range detection over cadence alone (mid-ride freewheels null cadence temporarily; death is an end-anchored L/R null tail after a live balance segment).
- **Power gap fill (outdoor) workflow.** Prefer physics fill when outdoor GPS + a measured power segment exist; prefer `hrLoad` load repair when indoor/no GPS or estimation confidence is low. Never invent load from RPE.
  1. Dry-run: `icu activity ID estimate-power --rider-mass-kg N --bike-mass-kg N --calibrate-from-measured --file fill.json` (bike mass required; rider mass falls back to athlete/wellness weight). Default streams include `left_right_balance`.
  2. Review classification: `measured` / `true_zero` / `missing`, `meterDeathIndex`, `deathSource` (`left_right_balance` preferred, then `cadence`, then missing-run). First half with real power+RPM+balance must stay measured for calibration.
  3. Weather aero is automatic for outdoor rides: activity wind/temp if present, else free Open-Meteo archive → forecast past → historical-forecast (no API key). Per-sample headwind = weather × track heading from activity map. Use `--no-weather` only for offline/activity-fields-only. Never invent wind constants.
  4. **Prior fill already accepted:** positive watts after death look measured. Re-open only the dead half with `--refill-after-pm-death` (balance → cadence). Or `--refill-from-index N` / `--refill-after-cadence-death`. Do not re-mask the live first half.
  5. Validate when possible: `icu activity ID estimate-power-backtest --rider-mass-kg N --bike-mass-kg N --calibrate-from-measured` (outdoor only; VirtualRide/indoor rejected). Prefer rides with continuous measured PM (no prior fill accept) so held-out watts are real. Modes: `mask_second_half` / `mask_after_fraction` = mid-ride PM death; `mask_scatter` = model fidelity with neighbors present. Multi-metric scores (not a single r): pearsonR, spearmanRho, bias, residual MAD-z, robustRmse.
     - **In-repo acceptance (automated):** physics-consistent synthetic mask → pearsonR/spearmanRho ≥ 0.95, residualZMedianAbs ≤ 1.0; outdoor-shaped noisy fixture → pearsonR/spearmanRho ≥ 0.80, |bias| ≤ 20 W or ≤10% of mean actual, residualZMedianAbs ≤ 1.5, robustRmse ≤ rmse, compared half ≥ ~15 min. Known planted headwind series must beat still-air on pearsonR or robustRmse.
     - **Live outdoor:** treat as corroboration only (draft/GPS/unmodeled noise). Do not demand synthetic 0.95 on free-air outdoor rides. CFD / gear CdA databases are out of scope (stdlib-only free-air + weather); improve fidelity via weather series, calibration, and residual ensemble — prove with backtest score gates.
  6. **Backup before accept.** Save streams and the exact dry-run fill file so `estimate-power-accept` can restore prior watts if needed. Accept is a stream mutation, not a soft preview.
  7. Accept only after review: `icu activity ID estimate-power-accept --file fill.json` (mutates watts stream; Intervals supporter feature). Server reanalysis owns load/NP; do not invent `icu_training_load` from the fill alone. If load is still wrong after a good watts fill, use HRSS/`hrLoad` repair separately with user approval.
  8. **Strava is separate.** Accept does not push watts to Strava (or Wahoo). Export `icu activity ID fit-file` (Intervals-processed FIT with edited streams), delete the broken Strava activity, re-upload the FIT. Do not use `activity file` (original device bytes) if you need the filled power. Metadata sync (title/desc) is not a power fix. Watch for duplicate re-imports into Intervals after re-upload.
  9. Cargo/bike mass must include real system mass (rider + bike + cargo). Pass explicit `--crr` when calibrated Crr looks non-physical (e.g. ≫0.01 on road).
- For activity load repair, use `icu activities list --oldest DATE --newest DATE --fields id,name,start_date_local,type,moving_time,icu_training_load,hr_load,hr_load_type,trimp,icu_hr_zone_times,icu_zone_times,average_heartrate,max_heartrate,icu_average_watts,icu_weighted_avg_watts,icu_intensity,icu_variability_index,lthr,athlete_max_hr` to verify the data contract. Only update with `icu activity ACTIVITY_ID update --training-load N` after explicit user approval, because it changes Intervals.icu history and downstream CTL/ATL/TSB.
- When adding an audit note to a completed activity description after a manual load repair, include the current activity name in the same `activity update` request. Intervals.icu can normalize/reset the name when only `description` is sent, so first read the activity and then update with both `--name CURRENT_NAME` and `--description NOTE`.

## Primary Workflow

1. Resolve athlete and range.
   - **Check the configured default athlete first.** Run `icu config show`. If `athlete_id` is set to a non-zero value, use that athlete for every command below unless the user explicitly asks for a different one. Do **not** ask for an athlete ID when a valid default is already configured.
   - If `athlete_id` is `0` or missing, ask the user for their Intervals.icu athlete ID before proceeding.
   - If the user says "my", "I", or similar without naming an athlete, use the configured default when it exists; otherwise ask.
   - If multiple coached athletes are possible and no default is configured, ask for athlete selection.
   - Default weekly report: 7 days ending today. Note: default ranges are calculated in UTC, so use absolute `YYYY-MM-DD` dates in the athlete's local timezone for precise daily boundaries.
   - If the user gives dates, use absolute `YYYY-MM-DD` dates.

2. Collect the primary coaching context.

**Note:** The `--athlete-id` flag in every command below is optional when `icu config show` reports a configured default athlete (anything other than `0`). Omit the flag to use the default; include it only to override.

```bash
icu analysis coaching --history-oldest HISTORY_START --history-newest END --plan-oldest NEXT_START --plan-newest NEXT_END --resolve [--athlete-id ATHLETE_ID]
  # NOTE: explicit plan dates are automatically aligned to ISO week boundaries (Mon-Sun) by the CLI.
icu analysis coaching --history-oldest HISTORY_START --history-newest END --plan-oldest NEXT_START --plan-newest NEXT_END --resolve --include-adaptation [--athlete-id ATHLETE_ID]
```

Use the non-adaptation command for routine weekly reports. Add `--include-adaptation` when the user asks about fitness change, power-curve trends, training focus, or planning decisions that need adaptation context.

Date ranges must be complete `YYYY-MM-DD` pairs. Do not combine explicit history dates with `--history-days`, or explicit plan dates with `--plan-days`. Use `--limit N` only when an explicit cap on fetched history activities is required; omit it for complete analysis.

3. Collect supporting Intervals.icu context only when needed.

```bash
icu analysis workout ACTIVITY_ID [--athlete-id ATHLETE_ID]
```

Run `analysis workout` for a specific completed workout review. Otherwise, `analysis coaching` already includes athlete, sport settings, plan events, NOTE events, cycling, wellness, and optional adaptation context.

4. Review existing calendar notes (NOTE events) for coaching context.

```bash
icu analysis coaching --history-oldest HISTORY_START --history-newest END --plan-oldest PLAN_START --plan-newest PLAN_END --resolve [--athlete-id ATHLETE_ID]
```

Check `dataQuality.missing` and `dataQuality.warnings` before interpreting any analyzer. Prefixed warnings identify their source (`cycling:`, `wellness:`, `plan:`, or `adaptation:`). Then read `calendar.notes` first and inspect `calendar.events` if more detail is needed. Both calendar fields are valid arrays; `[]` means no matching items, not malformed output. NOTE events contain block intentions, decision rules, physiological thresholds, and athlete communication. Missing them means the analysis is working with incomplete context.

If notes exist, incorporate their content into the review. If notes are absent or outdated, flag it and offer to write them after the analysis.

For any single-workout review inside an active plan, first run `icu analysis workout ACTIVITY_ID --athlete-id ATHLETE_ID`. Also retrieve surrounding week/block `NOTE` events when they are not already present in the workout-analysis context. The minimum valid frame is: `analysis workout` output, relevant notes, and current block state. Do not skip straight from activity metrics to coaching advice.

For planning questions, use a 12-week lookback and the next 4 weeks of events unless the user requests a different horizon.

```bash
icu analysis coaching --history-oldest HISTORY_START --history-newest END --plan-oldest NEXT_START --plan-newest NEXT_END --resolve --include-adaptation [--athlete-id ATHLETE_ID]
  # NOTE: explicit plan dates are automatically aligned to ISO week boundaries (Mon-Sun) by the CLI.
```

Planning dry-run checklist:

- Summarize the last 12 weeks by weekly TSS, hours, session count, high-intensity days, long endurance sessions, mean IF, and decoupling.
- Classify the current state from CTL/ATL/TSB plus wellness: e.g. `load_accepting`, `load_pressure`, or `recovery_priority`.
- Group the next 4 weeks of planned events by ISO week and compare planned TSS/hours against recent tolerance with `analysis plan`.
- For any new or edited workout that changes weekly load, run `icu workouts calculate --ftp FTP --desc DESC` first and use the returned `trainingLoad`, `durationSeconds`, and `intensityFactor` in the plan math. Never insert estimated deload values by feel.
- Review week roles and session classifications, not just TSS: `reentry`, `build`, `overload`, `deload`, `high_intensity`, `tempo_threshold`, `long_endurance`, `recovery`, `aerobic`, `opener`, `rest`.
- Review `execution.recommendedTitle` and `execution.cues` for each workout. Use short representative titles such as `4x5 VO2Max`, `2x20 SS`, `Z2 Cruise-Float`, or `3x15 Tempo` when the session structure supports them. Prefer structure-first names over generic `Z2 Aerobic`.
- Use encouragement cues for high-intensity, tempo/threshold, and selected endurance work. Use restraint/recovery cues for easy or recovery rides; do not add hype to Z1 recovery sessions.
- **Apply Session Structure Norm to every planned WORKOUT** (easy, Z2, long, tempo, VO2, over-unders, ALTs). Reject flat `warmup + constant main + cooldown` unless the athlete explicitly asked for a continuous free-ride or a true recovery spin with minimal structure.
- Before publishing a week, spot-check each desc: at least two distinct work phases or one repeat block with changing targets inside the aerobic band; HI sessions need openers and a controlled closer, not only the main set.
- Preserve the current plan when it is structurally sound; adjust execution rules before rewriting workouts. If the calendar is sound on load but monotonous on structure, rewrite structure in place while holding weekly TSS targets.
- Make intensity days conditional when physiology is `WATCH` or TSB is meaningfully negative.

Use wider ranges for trend questions:

```bash
icu activities list --oldest HISTORY_START --newest END [--athlete-id ATHLETE_ID] --fields id,name,start_date_local,type,moving_time,distance,icu_training_load,icu_intensity,decoupling,icu_efficiency_factor,icu_variability_index,icu_joules_above_ftp,icu_max_wbal_depletion,icu_ctl,icu_atl
```

5. Check the data contract before writing prose.
   - Required for load/recovery report: `analysis coaching` with athlete, sport settings, cycling analysis, wellness analysis, and next planned events if planning is discussed.
   - Required for adaptation: current power curves or MMP model plus historical activity context.
   - Required for W prime/repeatability: work above FTP and W balance fields in activities or activity intervals.
   - Required for heat/environment: weather or activity environmental fields. Do not infer heat stress from prose alone.
   - Required for 4-week planning: 12-week cycling analysis, 42-day wellness analysis, sport settings, and next 4 weeks of events.

6. Render the report from facts to interpretation.
   - Start with the headline state: e.g. `Load Pressure / recovery_priority`.
   - Separate raw metrics from interpretation.
   - Make confidence clear when data is sparse.
   - For planned-session reviews, present `plan vs execution vs implication` from `analysis workout` before any directive.
   - End with a concrete coaching directive only when it is supported by the current plan context; otherwise say what context is missing.
   - Add one useful reflection question only after the plan-context analysis is complete.

## Report Sections

Use these sections when the data exists:

- `REPORT CONTEXT`: athlete, report type, period, timezone, sport, FTP/eFTP, LTHR/max HR, CTL/ATL/TSB.
- `TRAINING LOAD`: hours, TSS/load, distance, CTL/ATL/TSB, ACWR, monotony, strain, daily load, session list.
- `PHYSIOLOGY OVERVIEW`: HRV, resting HR, sleep, subjective fatigue/stress/motivation, readiness, coverage.
- `PERFORMANCE SIGNALS`: durability, decoupling, efficiency factor, W prime/use above FTP, intensity density, high-intensity days.
- `ADAPTATION REVIEW`: power curve deltas, MMP model, strongest/weakest systems, likely training focus.
- `PLANNED BLOCK`: upcoming events and whether the plan matches the current state.
- `DECISION GUIDANCE`: what to do next and why.

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

### Session library

Canonical models: [session-library.md](./session-library.md). Workflow: **pick template → scale duration/reps/IF → `workouts calculate` → write event → optional week `rebalance`**. Prefer library IDs in titles (`Z2 HR-Control Waves`, `3x13 30/15 VO2`). Expand the library when a repeated athlete-specific pattern is missing; do not grow it with one-off Zwift map names.

### Session Structure Norm (default for all planned rides)

**Norm, not exception:** planned sessions must feel like a sequence of purposeful phases. Monotone steady blocks are banned as the default template for easy, aerobic, endurance, and long rides (indoor or outdoor erg/power targets). Outdoor free-ride / terrain-led rides may be lighter on prescribed steps, but still name intent and optional surges when power targets are used.

**Banned default shape**

```
- 15m warmup
- 90m 68%          # one constant main block
- 15m cooldown
```

**Required shape properties** (every WORKOUT unless athlete opted out)

1. **≥2 distinct work phases** after openers, or **≥1 repeat block** that alternates targets inside the session's intensity band.
2. **Phase changes every ~8–20 min** on endurance rides longer than ~45 min (waves, rotate, cruise-float, ladder, or story blocks).
3. **HI / tempo / VO2 / over-under:** structured openers (short build or activation reps), clear main sets, and a controlled spin-down — not warmup → main only → cooldown.
4. **Stay in role:** variety must not sneak intensity up a zone. Easy stays easy; Z2 peaks stay ≤ high-Z2 / max-Z2, never tempo/threshold; valleys are HR-control floats, not full Z1 nap intervals unless the session is recovery.
5. **Name the pattern** in the event title (`Z2 Waves`, `Cruise-Float`, `Low-Mid Rotate`, `Z2 Ladder`, `Durable Story`, `Easy Undulate`) so the calendar is scannable.
6. **ALTs follow the same norm.** Fallback Z2 notes must also be multi-phase, not a flat continuous block.
7. **Load first, spice second.** Build the varied desc, then `icu workouts calculate`; adjust phase lengths to hit the week's TSS target. Do not flatten structure to hit load.

**Pattern library (pick per session; rotate across the week so days do not clone each other)**

| Pattern | Intent | Sketch |
|---------|--------|--------|
| `Easy Undulate` | recovery / health | short low-Z1/Z2 oscillations (e.g. 3m up / 1m down) |
| `Z2 Waves` | aerobic engagement | 3–5m mid-Z2 + 30–45s valley |
| `Cruise-Float` | durable aerobic | 6–10m cruise + 1–2m float |
| `Low-Mid Rotate` | anti-monotony Z2 | alternate low-Z2 and mid-Z2 blocks with brief resets |
| `Z2 Ladder` | progressive aerobic | step low → mid → high-Z2, then descend |
| `Durable Story` | long ride | multi-act: settle → mid waves → high-Z2 caps → steady → finish (all ≤ Z2) |
| `HI with openers` | tempo/VO2/O-U | activation reps → main sets → optional spin-down pickups |

**Self-check before create/update**

- Would a rider on a trainer be bored in the first 20 minutes of the main set? If yes, rewrite.
- Does `workoutDoc` after write show repeat blocks (`reps > 1`) where expected?
- Did weekly TSS stay on target after the rewrite?

## Development Loop

When adding support to `icu` for a missing metric:

1. Add a failing test in the functional core when possible.
2. Add or extend the data type in `types.go` or `analysis.go`.
3. Update `analysis cycling` only if the metric is cycling-specific and read-only.
4. Keep CLI output JSON stable and additive.
5. Run `go test ./... -count=1`, `go vet ./...`, and `golangci-lint run ./...`.
