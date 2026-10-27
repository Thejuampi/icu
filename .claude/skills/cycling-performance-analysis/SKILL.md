---
name: cycling-performance-analysis
description: Use when coaching from the icu CLI — weekly reports, 4-week plans, daily "should I train" decisions, calendar NOTE review, post-illness/surgery/injury or off-bike weeks, load pressure, recovery, CTL/ATL/TSB, HRV, wellness, adaptation, planned events, or a missing analysis metric.
compatibility: opencode, claude
metadata:
  cli: icu
  domain: cycling-training
  source: project-agent-skill
---

# Cycling Performance Analysis

Prefer measured `icu` data over model inference. Read calendar NOTE events before coaching. Preserve the existing plan until it is actually wrong.

Power targets are **%FTP**. Read outdoor `ftp` and indoor `indoorFtp` from Ride sport settings. HR caps come from those zones, or a tighter ceiling on the calendar notes / recent durable rides.

Do not invent a missing metric. Name it, mark the section unavailable, and add CLI support only when the user asks to build it.

## Companion files

Load a file only when the job needs it. Do not load the whole pack for a daily decision.

| File | Load when |
|------|-----------|
| [athlete.md](./athlete.md) | How to resolve FTP, zones, timezone, and language from the CLI |
| [session-library.md](./session-library.md) | Writing or rewriting workouts |
| [report-data-contract.md](./report-data-contract.md) | Full weekly or 4-week report |
| **intervals-icu** skill | Auth diagnose, event writes, desc parser, `plan` / `rebalance`, power-fill, load repair |

CLI mechanics live in **intervals-icu**. Do not copy them here.

## Request router

Classify the ask, then fetch only that row.

| Ask | Fetch | Do not fetch |
|-----|--------|--------------|
| Today / "should I…?" / read this week's notes | This week's NOTE events, today's event, wellness last 3–7 days, last completed activity | 12-week history, 4-week plan dump, adaptation, session library |
| Weekly review | `icu analysis coaching` for that ISO week with explicit dates | Adaptation unless they ask about fitness change |
| Rewrite / 4-week plan | 12-week history + next 4 weeks + [athlete.md](./athlete.md) + [session-library.md](./session-library.md) | Full report prose before the calendar is understood |
| One completed ride | `icu analysis workout ACTIVITY_ID` plus surrounding notes | A new plan |
| Missing metric + "build it" | Development loop below | Coaching prose that pretends the number exists |

`analysis coaching` is the primary contract for weekly and planning work. `analysis workout` is the contract for one completed session.

## Resolve athlete and dates

1. Run `icu config show`. Use the configured `athlete_id` unless the user names another athlete. Do not ask for an ID when a non-zero default exists.
2. Pass explicit `YYYY-MM-DD` dates in the athlete's local timezone. Default analysis ranges are UTC and can shift "today" by a day.
3. `scope.timezone: UTC` with `timezoneSource: explicit` after you passed calendar dates is expected. Those dates are athlete-local calendar days. Do not re-fetch.
4. Unauthorized or empty: `icu config diagnose --athlete-id ID`. Never print API keys. Use status, length, trim, and fingerprint only.

## Plan-preserving

- Existing WORKOUT + NOTE events are the baseline plan.
- Do not analyze a session in isolation. Read the day/week/block notes first. If they are missing, say the analysis is incomplete and fetch them.
- Frame advice as plan-preserving: did the session match its role? Adjust execution rules or the existing ALT before proposing a different workout.
- If the calendar is sound on load but flat on structure, rewrite structure in place and hold weekly TSS. Load [session-library.md](./session-library.md) for that write.

## Disruption week

Use this class when notes, events, or the user describe illness, surgery, extraction, infection, injury, or tissue healing (including saddle skin).

- Role is healing, not TSS. Clinical instructions beat training notes.
- Flat, unstructured movement is allowed and often required.
- Do not map the day onto Ride high-Z1 or low-Z2.
- Do not invent a clinical HR cap. Use the calendar note or the clinician.
- Missed easy days are not debt. Do not catch them up.
- Bike return needs the gate already written on the calendar.
- Session Structure Norm does not apply to these days.

## Calendar writes

Read/decide turns do not write the calendar.

Write a NOTE only when one of these is true:

- the user asked to write it
- the decision changes remaining sessions this week
- a later gate needs a new fact that is not already on the calendar

Prefer updating the week NOTE over stacking a same-day NOTE. A duplicate of an existing week note is not a write.

When a new block is approved, create compact notes only: (a) one block-overview, (b) weekly focus, (c) short ALT notes if the decision tree is long. Intervals NOTE events are all-day; long descriptions can fail with HTTP 500.

When writing workouts, load [session-library.md](./session-library.md) and the **intervals-icu** desc / `plan` / `rebalance` rules. Calculate local load with `icu workouts calculate` before any write that changes weekly TSS.

## Output shape

Match the question. Do not dump the full report on a daily call.

**Daily / same-day:** plan vs the athlete's idea vs the limiter vs what to do in the next hour. One reflection question.

**Weekly / 4-week:** load [report-data-contract.md](./report-data-contract.md). Use REPORT CONTEXT, TRAINING LOAD, PHYSIOLOGY, PERFORMANCE SIGNALS, ADAPTATION (if requested), PLANNED BLOCK, DECISION GUIDANCE. Separate raw metrics from interpretation. Check `dataQuality.missing` and `dataQuality.warnings` first.

Chat language follows the user. Calendar NOTE language follows existing notes, then the athlete locale.

## Physiology and load

- HRV: use `recentMean`, `baselineMean`, `baselineMad`, `zScore`, `zScoreSource`. Treat raw `ratio` as context, not a stop/go rule.
- Recovery score: `analyses.wellness.sleep.scoreName` is the source of truth. Prefer Zepp BioCharge / `zepp_hybridcharge`. Intervals `sleepScore` is fallback. Read `fallbackScoreName` and `warnings`.
- Today's empty wellness record means "not logged yet," not a red flag. Use the latest complete day and say so.
- `ftpApplied` `confidence: medium` is usable. Mention it only when the session environment looks wrong. `confidence: high` needs no mention.
- When `context.timeAccounting.loadAccounting` is `partial`, say the weekly load excludes the enumerated non-cycling sessions. Do not present that total as complete.

## Metric policy

- `TrainingLoad` ← `icu_training_load`. `Intensity` ← `icu_intensity`. `WeightedAvgPower` ← `icu_weighted_avg_watts`. `LatestForm` ← CTL − ATL. ACWR ← `acuteChronicWorkRatio`.
- Label locally derived fields as local or heuristic.
- `analysis cycling` / `coaching` / `microcycle` already drop STRAVA stubs. A low `scope.activities` is the real-activity count.
- Credit `icu_ignore_*` filtering only when `validity.excluded > 0` on that run.
- `analysis workout` `match.compliance` is absent when `match.eventId` is 0. That is "no pairing," not 0% compliant.

Field-by-field support: [report-data-contract.md](./report-data-contract.md).

## Development loop

Only when the user asks to add a missing metric:

1. Failing test in the functional core when possible.
2. Extend the type in `types.go` or `analysis.go`.
3. Touch `analysis cycling` only for cycling-specific read-only metrics.
4. Keep CLI JSON stable and additive.
5. `go test ./... -count=1`, `go vet ./...`, `golangci-lint run ./...`.

A coaching question is not permission to start a CLI change.

## Common mistakes

| Excuse | Reality |
|--------|---------|
| "This touches the plan, so pull 12 weeks" | Daily decisions use the today row of the router. |
| "High Z1 / low Z2 is the easy option" | On a disruption week the calendar HR cap is the session. |
| "I should write a NOTE so the next agent sees this" | Write only if the decision changes the week or a gate needs a new fact. |
| "UTC in scope means I used the wrong timezone" | Explicit calendar dates + `timezoneSource: explicit` is correct. |
| "The metric would help, I'll add it while coaching" | Mark unavailable. Build it only when asked. |
