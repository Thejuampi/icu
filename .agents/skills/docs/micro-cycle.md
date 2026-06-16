# Analysis Microcycle

# PRD — `analysis microcycle` / `analysis micro`

## 1. Summary

`analysis microcycle` is a read-only diagnostic CLI command that analyzes the current or selected athlete training microcycle.

The command summarizes execution, training load, intensity distribution, wellness/readiness, fatigue, adaptation signals, risks, anomalies and data-quality issues.

It does not modify the plan, prescribe workouts, sync data, mutate configuration or generate the full weekly report.

`analysis micro` may exist as a short alias, but the canonical command name should be `analysis microcycle` because it is clearer and less ambiguous.

---

## 2. Status

Proposed status: Experimental / diagnostic.

The command should not be considered part of the stable official CLI until it has:

- complete documentation;
- help text;
- examples;
- stable output contract;
- tests for success, empty states and error states;
- clear behavior for missing plan, missing wellness, missing zones and missing external data.

---

## 3. Problem

The system needs a way to inspect a training microcycle before generating broader reports or making adaptive planning decisions.

Today, weekly reports and adaptive decisions may mix several concerns:

- What happened this week?
- Was the plan followed?
- Was the load appropriate?
- Is the athlete accumulating fatigue?
- Are there data-quality issues?
- Should the next workout or week change?

These concerns need a narrower diagnostic layer that can analyze the microcycle objectively without making plan changes.

---

## 4. Goals

The command should:

1. Analyze the current or selected microcycle.
2. Compare planned vs actual training when plan data exists.
3. Summarize load, duration, TSS/load, intensity and key sessions.
4. Evaluate wellness/readiness using multiple signals, not HRV alone.
5. Detect fatigue, durability and overload risks.
6. Detect missing, stale, malformed or incomplete data.
7. Produce a structured output usable by humans and by later report/adaptive layers.
8. Remain read-only.
9. Be safe to run repeatedly.
10. Clearly communicate confidence and data limitations.

---

## 5. Non-goals

The command must not:

1. Modify the athlete plan.
2. Recommend or prescribe specific future workouts.
3. Sync new data from external APIs.
4. Mutate config, cache, activities or local files.
5. Render the full weekly report.
6. Make final release/readiness decisions.
7. Hide missing data behind confident conclusions.
8. Rely on HRV alone to judge readiness.
9. Print tokens, credentials, auth headers or raw secrets.
10. Treat malformed API responses as valid empty data.

---

## 6. Target users

Primary users:

- Developer/operator validating training analysis behavior.
- Power user inspecting current training state.
- QA agent testing the training analysis pipeline.
- Future weekly-report or adaptive-decision logic consuming microcycle diagnostics.

Secondary users:

- Coach-like workflows that need a compact diagnostic snapshot before making planning decisions.

---

## 7. Command naming

## 7.1 Canonical command

```text
analysis microcycle
```

## 7.2 Short alias

```text
analysis micro
```

## 7.3 Recommended help text

```text
Analyze the current or selected training microcycle. Read-only diagnostic command that summarizes execution, load, intensity, wellness, fatigue, adaptation signals, risks and data-quality issues.
```

## 7.4 Rationale

`micro` alone is ambiguous. It could mean “small analysis”, “micro benchmark” or “minimal mode”. `microcycle` is domain-specific and clearer.

The alias `analysis micro` is acceptable for convenience, but docs should prefer `analysis microcycle`.

---

## 8. Default behavior

When run with no arguments:

```text
analysis microcycle
```

The command analyzes the athlete’s current microcycle.

Default microcycle definition:

* Start: configured week start, default Monday 00:00.
* End: configured week end, default Sunday 23:59:59.
* Timezone: athlete timezone from config.
* If athlete timezone is missing, fall back to system timezone and emit a data-quality warning.
* If the current week is still in progress, analyze from week start through now and mark the microcycle as partial.

The command must return a valid analysis even when data is incomplete.

---

## 9. Supported inputs

### No arguments

```text
analysis microcycle
```

Analyze current microcycle.

### Select microcycle by date

```text
analysis microcycle --date 2026-06-12
```

Analyze the microcycle containing the selected date.

### Select microcycle by week

```text
analysis microcycle --week 2026-06-08
```

Analyze the microcycle containing that date.

### Explicit date range

```text
analysis microcycle --from 2026-06-08 --to 2026-06-14
```

Analyze the explicit range as a custom microcycle.

### Detailed output

```text
analysis microcycle --full
```

Include detailed sections, intermediate metrics and data-quality diagnostics.

### Machine-readable output

```text
analysis microcycle --json
```

Return structured JSON output.

### Optional exclusions

```text
analysis microcycle --no-wellness
analysis microcycle --no-plan
```

`--no-wellness` excludes wellness/readiness analysis.

`--no-plan` analyzes actual execution only and skips planned-vs-actual comparison.

---

## 10. Input validation rules

The command must reject:

```text
--from without --to
--to without --from
--week combined with --from/--to
--date combined with --from/--to
invalid dates
inverted ranges where from > to
```

The command should warn on:

```text
ranges unusually large for a microcycle
missing athlete timezone
future dates with no planned data
missing wellness data
missing plan data
missing FTP/zones
stale activity cache
```

The command should not crash on empty data.

---

## 11. Data sources

The command may read from:

* Athlete config.
* Athlete timezone.
* Athlete plan.
* Planned workouts.
* Completed activities.
* Recent activities before selected microcycle.
* FTP history.
* Power zones.
* Heart-rate zones.
* Wellness data:

  * sleep;
  * resting HR;
  * HRV;
  * subjective feel;
  * fatigue/soreness if available.
* Local cache.
* External API data already available through existing project architecture.
* Local API definitions/contracts.

External API contracts should be inferred from local definitions where available, not guessed.

---

## 12. Functional requirements

### FR1 — Identify selected microcycle

The command must output:

* microcycle start date;
* microcycle end date;
* timezone;
* whether the microcycle is complete or partial;
* elapsed days;
* remaining days;
* data freshness.

Example:

```text
Microcycle: 2026-06-08 to 2026-06-14
Status: partial, 5 of 7 days elapsed
Timezone: America/New_York
```

---

### FR2 — Summarize planned vs actual execution

When plan data exists, the command must identify:

* planned sessions;
* completed sessions;
* missed sessions;
* extra sessions;
* modified sessions;
* planned load vs actual load;
* planned duration vs actual duration;
* key workout completion;
* compliance by volume;
* compliance by intensity.

When plan data does not exist, the command must continue and mark this section as unavailable.

---

### FR3 — Summarize training load

The command must report available load metrics, including:

* total duration;
* total distance, if available;
* total TSS/load;
* number of activities;
* intensity factor or equivalent, if available;
* comparison against previous 7 days;
* comparison against previous 28 days;
* comparison against previous 90 days, if available.

Optional metrics:

* CTL;
* ATL;
* TSB;
* ACWR;
* monotony;
* strain.

The command must not invent unavailable metrics.

---

### FR4 — Analyze intensity distribution

The command must summarize:

* time in zones;
* Z1/Z2/Z3/Z4/Z5+ distribution;
* number of true intensity sessions;
* number of Z4+ sessions;
* whether the week is polarized, pyramidal, threshold-heavy or intensity-heavy, if supported by available data.

The command should flag excessive high-intensity exposure.

Default coaching guardrail:

```text
More than 2 real Z4+ sessions in a microcycle should be flagged as a potential risk unless explicitly justified by the plan.
```

---

### FR5 — Identify key workouts

The command must identify:

* hard sessions;
* long rides;
* recovery rides;
* missed key workouts;
* failed or partially completed key workouts;
* workouts with unusual HR/power relationship;
* workouts with notable decoupling, if data exists;
* workouts with late-session power fade, if data exists.

Long rides should be treated as major stress events, especially when combined with high intensity, poor sleep, high resting HR, low subjective feel or possible underfueling.

---

### FR6 — Analyze wellness/readiness

The command must analyze readiness using multiple signals.

Required principle:

```text
Do not rely on HRV alone.
```

When available, combine:

* HRV;
* resting HR;
* sleep duration;
* sleep quality;
* subjective feel;
* soreness/fatigue;
* recent training load;
* prior hard sessions;
* long rides;
* illness flags, if available.

The command must lower confidence when wellness data is missing or partial.

Readiness output should include:

* positive signals;
* negative signals;
* neutral/missing signals;
* confidence level.

---

### FR7 — Analyze fatigue and durability

The command should detect:

* high accumulated load;
* unusually dense intensity;
* long-ride fatigue;
* elevated HR at normal power;
* HR drift/decoupling;
* late-ride fading;
* poor recovery between hard sessions;
* potential underfueling signal, if nutrition data exists;
* signs that load is productive vs excessive.

The command should distinguish:

```text
productive overload
excessive fatigue
insufficient stimulus
recovery/absorption
data-limited uncertainty
```

---

### FR8 — Analyze adaptation signal

The command must classify the microcycle’s adaptation signal.

Allowed classifications:

```text
on_track
productive_overload
underloaded
overloaded
recovery_needed
disrupted
data_limited
experimental_uncertain
```

The classification must include:

* rationale;
* supporting evidence;
* confidence;
* main positive signal;
* main risk.

---

### FR9 — Detect data-quality issues

The command must detect and report:

* no activities;
* missing plan;
* missing wellness;
* missing HR;
* missing power;
* missing FTP;
* missing zones;
* duplicate activities;
* stale sync/cache;
* malformed external data;
* unexpected zero values;
* timezone inconsistencies;
* suspiciously high or low TSS/load;
* future dates with no planned data;
* activities outside expected date boundaries.

Data-quality issues must reduce confidence where appropriate.

---

### FR10 — Produce decision-oriented summary

The command must end with a concise diagnostic summary:

* current microcycle status;
* classification;
* confidence;
* main positive signal;
* main risk;
* data-quality warnings;
* whether this analysis is suitable as input to weekly report/adaptive planning.

It must not prescribe a concrete future workout.

Acceptable:

```text
Use this analysis as input for the weekly report or adaptive planning step.
```

Not acceptable:

```text
Do VO2 intervals tomorrow.
```

---

## 13. Output structure

Human-readable output should follow this structure:

```text
1. Microcycle summary
2. Data quality
3. Planned vs actual
4. Load summary
5. Intensity distribution
6. Key sessions
7. Wellness/readiness
8. Fatigue and durability
9. Adaptation signal
10. Risks and anomalies
11. Open questions / missing data
12. Final classification
```

---

## 14. Example human-readable output

```text
Microcycle: 2026-06-08 to 2026-06-14
Status: partial, 5 of 7 days elapsed
Timezone: America/New_York
Classification: productive_overload
Confidence: medium

Summary:
The microcycle is currently tracking as a productive build week. Load is above the recent 7-day baseline but still reasonable against the 28-day context. Intensity exposure is acceptable so far, with one confirmed Z4+ session. Wellness data is incomplete, so readiness confidence is limited.

Data quality:
- Activities: available
- Power: available
- HR: partially available
- Wellness: incomplete
- Plan: available
- Timezone: configured
- Last sync: available

Planned vs actual:
- Planned sessions: 5
- Completed sessions: 4
- Missed sessions: 0
- Remaining sessions: 1
- Extra sessions: 0
- Key workout completion: on track

Load:
- Actual load is moderately above recent 7-day baseline.
- Current load remains within expected build-week range.
- Final weekly load depends on the remaining long ride.

Intensity:
- Confirmed Z4+ sessions: 1
- Intensity density: acceptable
- No evidence yet of excessive high-intensity stacking.

Wellness/readiness:
- Readiness confidence is medium-low because wellness data is incomplete.
- No single HRV-only conclusion was made.

Main positive signal:
Key intensity work was completed and aerobic volume is accumulating as planned.

Main risk:
If the planned long ride is executed at full volume, total weekly load may exceed the intended progression.

Final note:
No plan changes were made. This command is diagnostic only.
```

---

## 15. JSON output contract

When run with:

```text
analysis microcycle --json
```

The command should return structured output similar to:

```json
{
  "command": "analysis microcycle",
  "status": "success",
  "microcycle": {
    "start": "2026-06-08",
    "end": "2026-06-14",
    "timezone": "America/New_York",
    "is_partial": true,
    "elapsed_days": 5,
    "remaining_days": 2
  },
  "classification": {
    "value": "productive_overload",
    "confidence": "medium",
    "rationale": [
      "Load is above recent baseline but within expected build range",
      "Only one confirmed Z4+ session so far",
      "Wellness data is incomplete"
    ]
  },
  "data_quality": {
    "activities": "available",
    "plan": "available",
    "power": "available",
    "heart_rate": "partial",
    "wellness": "incomplete",
    "timezone": "configured",
    "warnings": [
      "Wellness data incomplete"
    ]
  },
  "planned_vs_actual": {
    "available": true,
    "planned_sessions": 5,
    "completed_sessions": 4,
    "missed_sessions": 0,
    "extra_sessions": 0,
    "remaining_sessions": 1
  },
  "load": {
    "duration_seconds": 18000,
    "tss": 320,
    "activity_count": 4,
    "comparison": {
      "previous_7_days": "above",
      "previous_28_days": "within_expected_range"
    }
  },
  "intensity": {
    "z4_plus_sessions": 1,
    "intensity_density": "acceptable",
    "warnings": []
  },
  "wellness": {
    "available": false,
    "confidence_impact": "reduced"
  },
  "risks": [
    {
      "type": "load_progression",
      "severity": "medium",
      "message": "Remaining long ride may push weekly load above intended progression"
    }
  ],
  "side_effects": {
    "mutated_plan": false,
    "mutated_config": false,
    "synced_external_data": false
  }
}
```

Exact field names may evolve while experimental, but once stable they should be treated as a contract.

---

## 16. Error and empty-state behavior

### No activities

Expected behavior:

```text
No completed activities found for this microcycle.
Plan data is available/unavailable.
Analysis is data-limited.
```

The command must not crash.

---

### No plan

Expected behavior:

```text
No athlete plan found. Planned-vs-actual analysis is unavailable.
Continuing with actual activity analysis only.
```

---

### No wellness

Expected behavior:

```text
No wellness data found. Readiness confidence is reduced.
Continuing with activity/load analysis.
```

---

### No FTP or zones

Expected behavior:

```text
FTP or zone configuration is missing. Zone-based intensity analysis is unavailable.
Continuing with duration/load analysis where possible.
```

---

### External API unavailable

Expected behavior:

```text
External data source unavailable: <source>.
Analysis could not be completed from that source.
Retry after restoring connectivity or use cached data if available.
```

The command must not silently convert API failure into “no data”.

---

### Malformed API response

Expected behavior:

```text
Malformed response from <source>.
Analysis stopped for this data source because the response does not match the expected contract.
```

---

### Stale data

Expected behavior:

```text
Warning: activity data may be stale.
Last successful sync: <timestamp>.
```

---

## 17. Security requirements

The command must never print:

* tokens;
* credentials;
* auth headers;
* raw secrets;
* full external API responses containing sensitive data;
* stack traces in normal user-facing output.

Debug output, if supported, must sanitize secrets by default.

Example:

```text
zepp_token: abc123...redacted
```

Not acceptable:

```text
zepp_token: abc123FULLTOKENVALUE
```

---

## 18. UX requirements

The command output should be:

* structured;
* scannable;
* explicit about confidence;
* explicit about missing data;
* clear about whether the week is partial;
* clear about what was analyzed;
* clear that no plan changes were made.

The command should avoid overly narrative coaching language. It is a diagnostic command, not a full coaching report.

Good:

```text
Classification: productive_overload
Confidence: medium
Main risk: remaining long ride may push load above intended progression.
```

Bad:

```text
You are ready to smash intervals tomorrow.
```

---

## 19. Relationship to other commands

### Weekly report

`analysis microcycle` is a lower-level diagnostic layer.

The weekly report may consume this analysis, but the weekly report is broader, more narrative and more user-facing.

### Adaptive planning

`analysis microcycle` may identify readiness and risk signals, but must not change the plan.

Adaptive decisions belong to a separate command or report layer.

### Data sync

`analysis microcycle` should not fetch/sync new external data unless the broader system architecture already guarantees read-only cached access.

### Config commands

`analysis microcycle` should read config but not mutate it.

---

## 20. Acceptance criteria

The feature is acceptable when:

1. `analysis microcycle` runs with no arguments.
2. `analysis micro` works as an alias or is intentionally omitted.
3. The command is read-only.
4. The command identifies the correct current microcycle.
5. The command respects athlete timezone.
6. The command marks in-progress weeks as partial.
7. The command handles no activities without crashing.
8. The command handles missing plan without crashing.
9. The command handles missing wellness without crashing.
10. The command handles missing FTP/zones without crashing.
11. The command reports data-quality warnings.
12. The command does not print secrets.
13. The command distinguishes confidence levels.
14. The command does not prescribe workouts.
15. The command has human-readable output.
16. The command supports JSON output or explicitly marks JSON as unsupported.
17. The command has tests for happy path, empty state and major error states.
18. The command is documented as experimental until stable.

---

## 21. Required tests

Minimum test coverage should include:

### Command discovery

* `analysis microcycle --help`
* `analysis micro --help`, if alias exists

### Default behavior

* current microcycle with complete data
* current microcycle with partial week

### Date selection

* `--date`
* `--week`
* `--from` / `--to`

### Invalid inputs

* invalid date
* inverted range
* `--from` without `--to`
* `--to` without `--from`
* conflicting `--week` and `--from/--to`

### Data states

* no activities
* no plan
* no wellness
* no FTP
* no zones
* missing HR
* missing power
* stale sync/cache
* malformed external response
* external API unavailable

### Safety

* no token leakage
* no config mutation
* no plan mutation
* no external sync side effect

### Output

* human-readable sections present
* JSON output valid when `--json` is used
* confidence reduced when data is incomplete
* partial week clearly marked

---

## 22. Release recommendation

Initial release status:

```text
Experimental
```

Stable release requirements:

* command documented;
* alias behavior decided;
* output format stabilized;
* JSON contract stabilized if supported;
* error behavior tested;
* no secret leakage;
* clear distinction from weekly report and adaptive planning;
* tests cover empty, partial, normal and broken-data states.

---

## 23. Open product decisions

1. Should `analysis micro` remain as an alias or should only `analysis microcycle` be exposed?
2. Should `--json` be part of MVP or added later?
3. Should explicit custom ranges be allowed, or should the command only analyze configured microcycles?
4. Should stale cache cause a warning or a hard failure?
5. Should external API reads be allowed, or should the command only use already-synced local data?
6. Should the command appear in normal CLI help while experimental?
7. Should detailed metrics like ACWR, monotony and strain be included in MVP or deferred?

````

La versión más corta del PRD sería:

```text id="short-definition"
`analysis microcycle` is a read-only diagnostic command that analyzes the current or selected training microcycle. It summarizes planned-vs-actual execution, load, intensity, wellness/readiness, fatigue, adaptation signals, risks and data-quality issues. It must not modify the plan, sync data, prescribe workouts or generate the full weekly report. Until documented and tested as a stable contract, it should remain experimental. `analysis micro` may exist as a short alias.
