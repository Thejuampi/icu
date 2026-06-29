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
| HRV ratio, mean, latest, trend, samples, recent mean, baseline mean, robust z-score | 42-day wellness HRV series | `icu analysis wellness` | supported |
| Resting HR and delta | 42-day wellness resting HR series | `icu analysis wellness` | supported |
| Sleep score and coverage | 42-day wellness sleep fields | `icu analysis wellness` | supported |
| Subjective wellness coverage | fatigue, stress, soreness, mood, motivation | `icu analysis wellness` | supported |
| CTL, ATL, TSB | activity or wellness load fields | `icu analysis cycling` | supported |
| Physiology state | HRV/resting HR/sleep/load-pressure rule | `icu analysis wellness` local deterministic analysis | supported |
| External heat/load context | weather/temp/terrain/VAM fields | `icu analysis cycling` environment context from activity fields | supported |

## Performance Signals

| Field family | Required data | Source | Status |
| --- | --- | --- | --- |
| Load Pressure / recovery priority | latest CTL, ATL, TSB | `icu analysis cycling` | supported |
| WDRM counters | work above FTP, W balance depletion, W prime capacity | `icu analysis cycling` anaerobic and repeatability sections | supported |
| W prime pattern | per-session W balance and anaerobic contribution | `icu analysis cycling` sessions and repeatability section | supported |
| Durability / ISDM | decoupling per session, long endurance count | `icu analysis cycling` durability signal | supported |
| Neural density / NDLI | high-intensity days, work above FTP, IF, EF, VI | `icu analysis cycling` | supported |
| Efficiency semantic state | EF and VI context | `icu analysis cycling` local deterministic thresholds | supported |

## Adaptation Review

| Field family | Required data | Source | Status |
| --- | --- | --- | --- |
| Power anchors | power curves with activity sources | `icu analysis adaptation` | supported |
| Power curve deltas | comparable curve windows | `icu analysis adaptation` power curve deltas | supported |
| CP/W prime/Pmax/FTP | MMP model or sport settings | `icu analysis adaptation` power anchors | supported |
| System status | curve deltas and training response | `icu analysis adaptation` system status | supported |
| Lactate calibration | lactate wellness fields | `icu analysis wellness` lactate calibration | supported |
| Phase summary | historical load segmentation | `icu analysis adaptation` phase summary | supported |

## Decision Guidance

| Field family | Required data | Source | Status |
| --- | --- | --- | --- |
| Primary directive | load pressure + physiology + forecast (adaptation pending) | `icu analysis plan` decision guidance | partial |
| ADE score and penalties | deterministic scoring rules | `icu analysis plan` decision guidance | supported |
| Event target status | target/race events | `icu analysis plan` target status | supported |
| Planned summary by ISO week | future events grouped by ISO week | `icu analysis plan` | supported |
| Future forecast | planned load + CTL/ATL projection | `icu analysis plan` forecast | supported |
| Training guidance | directive rendered from all sections | `icu analysis plan` decision guidance (adaptation pending) | partial |

## Training Plan Dry Run

| Field family | Required data | Source | Status |
| --- | --- | --- | --- |
| 12-week completed load history | weekly TSS, hours, sessions, intensity, decoupling, and tolerance | `icu analysis plan` completed week series | supported |
| Current load/recovery state | latest CTL, ATL, TSB, ACWR, monotony, strain | `icu analysis cycling` | supported |
| Current physiology state | HRV, resting HR, sleep, coverage, confidence | `icu analysis wellness` | supported |
| Sport anchors | FTP, LTHR, W prime if available | `icu analysis plan` sport anchors from sport settings | supported |
| Existing 4-week calendar | future workouts and notes with duration/load/intensity | `icu analysis plan` | supported |
| Weekly planned-load grouping | ISO-week summary of future events | `icu analysis plan` | supported |
| Planned session structure | high-intensity, tempo/threshold, long endurance, recovery, aerobic, opener, rest | `icu analysis plan` | supported |
| Block phase and week roles | build/recovery/maintenance plus reentry/build/overload/deload roles | `icu analysis plan` | supported |
| Representative workout titles | device-friendly titles from interval pattern, duration, and session class | `icu analysis plan` | supported |
| Device cue messages | preview, work, encouragement, restraint, finish messages by session class | `icu analysis plan` | supported |
| Indoor Z2 variation profile | 4m Z2 waves, 40s HR-control valleys, low/mid/high Z2 rotation, max-Z2 caps | `icu analysis plan` | supported |
| CTL/ATL/TSB forecast | future planned load impulse model | `icu analysis plan` forecast | supported |
| Day-level adjustment rules | HRV/sleep/RHR/TSB/decoupling/heat gates | `icu analysis plan` deterministic day adjustments | supported |

Planning output should compare the existing calendar against the athlete's recent tolerance before creating replacement workouts. When the calendar is structurally sound, prefer conditional execution rules and small load/intensity edits.

## Data Quality Checks

| Risk | Required check | Status |
| --- | --- | --- |
| Env API key shadows config API key | `icu config diagnose --athlete-id ATHLETE_ID` with source/fingerprint only | supported |
| Secret leakage during auth debugging | never print raw API keys; use status, length, trim, fingerprint | supported |
| Intervals snake_case payload not mapped to DTO | add unmarshal tests for raw activity/event/supporting fields before trusting reports | supported for activities/events/weather/curves/sports/custom items |
| Suspicious zero-valued activity/event fields | spot-check raw API or CLI JSON before writing coaching prose | manual |

## Timezone and Date Range Contract

| Concern | Current behavior | Recommendation |
| --- | --- | --- |
| Default analysis date range | Calculated in **UTC** (`timezone: "UTC"`, `timezoneSource: "default_utc"` in scope output) | Use explicit `--oldest`/`--newest` dates in the athlete's local timezone for daily-accurate reports. |
| API timestamps | `startDateLocal` and related fields are returned by Intervals.icu in the athlete's configured timezone; `icu` does not convert them. | Treat API timestamps as athlete-local unless the output scope says otherwise. |
| Wellness "latest" value | Uses the newest record inside the requested date range; if today's wellness has not synced, the latest value will be from yesterday. | Spot-check `icu wellness get YYYY-MM-DD` when a value looks stale. |
| Readiness score | Not stored in Intervals.icu wellness records unless the source syncs it. | Do not assume readiness score from another app is available in `icu analysis wellness`. |

## Implementation Priority

1. Add aggregate `analysis report` output that fuses cycling, wellness, plan, adaptation, and decision sections.
2. Wire `analysis report` so `primary directive` and `training guidance` receive adaptation context directly from CLI output.
3. Expand adaptation source detail with activity IDs behind best power curve points when Intervals.icu exposes them in curve payloads.
