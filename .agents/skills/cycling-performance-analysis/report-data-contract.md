# Cycling Report Data Contract

This contract tracks the data needed to render expanded cycling reports like:

- `full physiology overview`
- `full performance signals`
- `full adaptation review`
- `full decision guidance`
- `4-week training plan dry run`

Support status meanings:

- `supported`: available directly from current `icu` output.
- `partial`: some source fields exist, but the grouped report field needs more work.
- `missing`: not currently produced by `icu`.

## Physiology Overview

| Field family | Required data | Source | Status |
| --- | --- | --- | --- |
| HRV ratio, mean, latest, trend, samples | 42-day wellness HRV series | `icu analysis wellness` | supported |
| Resting HR and delta | 42-day wellness resting HR series | `icu analysis wellness` | supported |
| Sleep score and coverage | 42-day wellness sleep fields | `icu analysis wellness` | supported |
| Subjective wellness coverage | fatigue, stress, soreness, mood, motivation | `icu analysis wellness` | supported |
| CTL, ATL, TSB | activity or wellness load fields | `icu analysis cycling` | supported |
| Physiology state | HRV/resting HR/sleep/load-pressure rule | `icu analysis wellness` local deterministic analysis | supported |
| External heat/load context | weather/temp/terrain/VAM fields | activities/weather endpoints | missing |

## Performance Signals

| Field family | Required data | Source | Status |
| --- | --- | --- | --- |
| Load Pressure / recovery priority | latest CTL, ATL, TSB | `icu analysis cycling` | supported |
| WDRM counters | work above FTP, W balance depletion, W prime capacity | activities + sport settings | partial |
| W prime pattern | per-session W balance and anaerobic contribution | `icu analysis cycling` sessions | partial |
| Durability / ISDM | decoupling per session, long endurance count | `icu analysis cycling` | partial |
| Neural density / NDLI | high-intensity days, work above FTP, IF, EF, VI | `icu analysis cycling` | supported |
| Efficiency semantic state | EF and sport context | local deterministic thresholds | missing |

## Adaptation Review

| Field family | Required data | Source | Status |
| --- | --- | --- | --- |
| Power anchors | power curves with activity sources | `icu curves power` | partial |
| Power curve deltas | comparable curve windows | `icu curves power` plus local analysis | missing |
| CP/W prime/Pmax/FTP | MMP model or sport settings | `icu curves mmp`, `icu sports get Ride` | partial |
| System status | curve deltas and training response | local deterministic analysis | missing |
| Lactate calibration | lactate custom/wellness fields | custom/wellness data | missing |
| Phase summary | historical load segmentation | activities/events analysis | missing |

## Decision Guidance

| Field family | Required data | Source | Status |
| --- | --- | --- | --- |
| Primary directive | load pressure + physiology + adaptation + forecast | local decision engine | missing |
| ADE score and penalties | deterministic scoring rules | local decision engine | missing |
| Event target status | target/race events | `icu events list` | partial |
| Planned summary by ISO week | future events grouped by ISO week | `icu analysis plan` | supported |
| Future forecast | planned load + CTL/ATL projection | local forecast engine | missing |
| Training guidance | directive rendered from all sections | AI after numeric contract | missing |

## Training Plan Dry Run

| Field family | Required data | Source | Status |
| --- | --- | --- | --- |
| 12-week completed load history | weekly TSS and hours tolerance | `icu analysis plan` plus `icu analysis cycling` for richer completed-session detail | partial |
| Current load/recovery state | latest CTL, ATL, TSB, ACWR, monotony, strain | `icu analysis cycling` | supported |
| Current physiology state | HRV, resting HR, sleep, coverage, confidence | `icu analysis wellness` | supported |
| Sport anchors | FTP, LTHR, W prime if available | `icu sports get Ride` | partial |
| Existing 4-week calendar | future workouts and notes with duration/load/intensity | `icu analysis plan` | supported |
| Weekly planned-load grouping | ISO-week summary of future events | `icu analysis plan` | supported |
| Planned session structure | high-intensity, tempo/threshold, long endurance, recovery, aerobic, opener, rest | `icu analysis plan` | supported |
| Block phase and week roles | build/recovery/maintenance plus reentry/build/overload/deload roles | `icu analysis plan` | supported |
| Representative workout titles | device-friendly titles from interval pattern, duration, and session class | `icu analysis plan` | supported |
| Device cue messages | preview, work, encouragement, restraint, finish messages by session class | `icu analysis plan` | supported |
| Indoor Z2 variation profile | 4m Z2 waves, 40s HR-control valleys, low/mid/high Z2 rotation, max-Z2 caps | `icu analysis plan` | supported |
| CTL/ATL/TSB forecast | future planned load impulse model | local forecast engine | missing |
| Day-level adjustment rules | HRV/sleep/RHR/TSB/decoupling/heat gates | AI guidance after numeric contract | partial |

Planning output should compare the existing calendar against the athlete's recent tolerance before creating replacement workouts. When the calendar is structurally sound, prefer conditional execution rules and small load/intensity edits.

## Data Quality Checks

| Risk | Required check | Status |
| --- | --- | --- |
| Env API key shadows config API key | `icu config diagnose --athlete-id ATHLETE_ID` with source/fingerprint only | supported |
| Secret leakage during auth debugging | never print raw API keys; use status, length, trim, fingerprint | supported |
| Intervals snake_case payload not mapped to DTO | add unmarshal tests for raw activity/event fields before trusting reports | supported for activities/events |
| Suspicious zero-valued activity/event fields | spot-check raw API or CLI JSON before writing coaching prose | manual |

## Implementation Priority

1. Expand `icu analysis cycling` with stable load-state and performance-signals JSON.
2. Add wellness analysis from `icu wellness list` for HRV/resting HR/sleep coverage and physiology state.
3. Add planned-event analysis for ISO-week planned load and forecast inputs.
4. Add power-curve comparison support for adaptation.
5. Add a deterministic decision-guidance layer only after the numeric sections are stable.
