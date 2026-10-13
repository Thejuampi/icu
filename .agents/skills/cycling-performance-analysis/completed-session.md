# Completed-session review from the recording

Use this workflow for any completed-ride review, especially when the athlete asks for the complete session or sample-by-sample analysis. Prefer the recording over precomputed execution labels. Keep this a read-only coaching task.

## Intent and sources

1. Resolve the activity ID and athlete-local date from activity metadata. Read the planned WORKOUT and surrounding day/week/block NOTE events. Use the athlete's explanation when no plan exists; a missing plan reduces confidence in compliance, not confidence that the recorded effort happened. Keep the daily router's narrow scope.
2. Load the **intervals-icu** skill's original-recording download procedure. Obtain the original file and complete streams, including time, power, HR, cadence, and available terrain/movement channels. Save private recordings and analysis artifacts outside tracked repository files. Existing Go dependencies remain unchanged.
3. Decode the original recording with an available parser, checking file integrity when the format supports it. Preserve missing fields. Cross-check overlapping timestamps/channels against streams and report differences. Original files, processed FIT exports, current streams, device summaries, and server summaries are separate sources; label them.
4. If current streams contain an intentionally accepted correction, retain that provenance and use the corrected series for the affected metric. Keep the original for comparison; never mix both versions silently. A first-value fill, dropped sample, or changed timestamp in a processed series must not overwrite original absence.
5. Do not fetch `activity intervals`, `show --intervals`, or `analysis workout` to construct execution context. Do not consume detected reps, warmup/cooldown boundaries, compliance inferred from those reps, or labels such as zone-alignment diagnoses. An explicit request to compare the CLI detector is a separate diagnostic task; its output stays separate from the recording-based review.
6. If the original is unavailable or cannot be decoded with available tools, continue from complete timestamped streams and name the fallback. If only rollups exist, provide a limited summary and mark sample-based sections unavailable. Do not claim the original was verified or fabricate interval structure.

## Resolve the training anchor

Apply [AGENTS.md's durable rules and scoped evidence policy](../../../AGENTS.md#durable-rules-and-scoped-evidence) to the training anchor. The relevant times are the athlete-local recording date, when anchor evidence was supplied/retrieved, and its known effective period. Live settings provide current configuration, not FTP or zone history.

- State the purpose first: reconstruct execution against the session's applicable anchor, prescribe current/future training, or explicitly re-express an old ride against another reference. The last is a labeled comparison, not a correction of the historical record.
- Bind each candidate and the selected anchor to athlete, activity or explicitly named date range, sport, indoor/outdoor environment, value/units, source/version, and known effective period. Retain device identity when relevant. Do not invent historical FTP, zone history, or metadata absent from the source.
- For a completed ride, consider its recorded/applied anchor, contemporaneous plan/settings evidence, and applicable athlete instructions. FTP, LTHR, and zone definitions must have compatible scopes. Thresholds come from applicable settings, notes, or explicit inputs, never a fixed physiological cutoff.
- If an athlete instruction states no broader scope, bind it to the activity currently under discussion. Record an explicit period or standing constraint with its stated scope and any end condition. Keep it labeled as an analysis/prescription constraint rather than measured fitness.
- Device `threshold_power` is a configuration snapshot for its recording. Numeric disagreement alone proves neither obsolescence nor physiological truth. Keep the device and analysis values with the reason for selecting one. Distinguish configured FTP, workout targets, aspirational goals, and modeled/estimated FTP.
- Reassess applicability for another session, environment, athlete, planning period, or material source update. Scope the outcome to that request; an instruction for one activity does not cover every ride on the same date.
- If applicable evidence conflicts or historical coverage is missing, preserve the raw analysis and name the unresolved attribution. Ask only if resolving it materially changes the requested interpretation and the current instructions do not already settle it. An explicitly scoped override remains usable as an analysis constraint; label it as such rather than claiming measured fitness. Keep %FTP/zones/time-above-FTP provisional or unavailable when no defensible basis exists.
- Record the selected anchor, scope, and rationale alongside derived results. Changing the analysis anchor changes %FTP, zone attribution, and quantities such as time/work above FTP; it does not change timestamps, recorded watts, or absolute-power best windows. Do not overwrite stored device/server summaries or write settings from this read-only resolution. Compare periods with their own anchors and label any deliberate common-reference normalization.

When editing these rules or auditing reused evidence, run the scenarios in [anchor-aging-checks.md](./anchor-aging-checks.md). They evaluate scope and temporal attribution; they do not implement a new CLI resolver.

## Timestamp and missing-data integrity

- Align channels by actual timestamps. Never substitute array index for elapsed seconds. Check monotonicity, duplicate timestamps, channel lengths, and sampling cadence before calculating durations or best windows.
- Distinguish elapsed time, device timer time, server moving time, record count, and measured coverage per channel. Explain material differences rather than forcing them to match.
- Match gaps with device timer start/stop events when available. A low speed near a gap is context, not proof of an autopause. Unexplained gaps stay unexplained; no interpolation or holding the previous power across them.
- Preserve a measured zero. Power zero with cadence zero and a functioning meter supports coasting; absent power/cadence remains missing. Power zero with cadence present is a flag to inspect, not automatic meter failure or permission to fill.
- Check cadence, balance, and sensor coverage for dropouts using the **intervals-icu** meter-gap procedure. Never remove real outdoor accelerations merely because an indoor flywheel heuristic flagged them. Identify any exclusion and show its effect on results.
- Means include measured zeros and exclude absent values, with the contributing coverage stated. For irregular sampling, use verified covered time spans and time weighting; do not equate record count with seconds. When duration cannot be established, report counts instead.
- Zone durations may count measured samples as seconds only after establishing one-second coverage. Read zone boundaries from settings and state the inclusivity convention. Power-zone totals must reconcile with measured power coverage, not elapsed time or an upstream summary.

## Reconstruct the full sequence

Start with the complete power / HR / cadence timeline, plus altitude and speed when available. Keep pauses visible. Label display smoothing and its window; it changes the drawing only. Do not smooth across missing timestamps or use the display trace to price load, classify samples, or select best efforts.

Device laps are useful recorded boundaries. Retain their elapsed/timer accounting, but do not call them manual laps, work reps, or recoveries without supporting metadata, athlete intent, or planned structure. A lap can contain preparation, variable work, and a harder finish.

Within those boundaries, reconstruct chronological phases from the combined signals and terrain. State that these are analyst interpretations and give exact elapsed-time bounds. Explain observable transitions: sustained target power, real coasting, a long recovery, repeated accelerations, a change in terrain, or the end of the work block. A momentary crossing below FTP does not by itself end an effort. Avoid replacing the upstream detector with another unexplained cutoff detector.

For relevant phases, show duration/coverage, mean and median power, %FTP, time above FTP, configured power-zone time, HR response, and cadence while pedalling. If excluding zeros for a secondary pedalling-only mean, label its denominator and show the full mean alongside it. Preserve terrain and movement context when interpreting either number.

Select representative accelerations with stated boundaries and continuous best windows appropriate to the question or intended workout. A best window must cover its full elapsed duration with valid power at the verified recording cadence; retain zeros and exclude windows spanning pauses/dropouts. Describe why representative efforts were selected rather than implying they are an exhaustive rep count. If the selected method cannot support a window, mark it unavailable.

## Interpretation limits

- Keep observations separate from explanations. Lower power with sustained HR can reflect fatigue, terrain, pacing, recovery timing, temperature, or sensor behavior; it does not establish the cause on its own.
- Power in Z5/Z6 describes intensity relative to the selected FTP. It does not establish time at VO2max, oxygen kinetics, lactate, or a particular adaptation. A variable threshold session with short surges can still be the week's hard stimulus.
- Respect HR lag. Whole-session power/HR zone mismatch does not diagnose heat, cold, or readiness. Compare relevant phases with similar effort and context.
- Whole-session decoupling across hard work and easy return is not a steady-endurance durability test. Use suitable comparable aerobic phases when available and state uncertainty otherwise.
- HR dropping during an uncontrolled ride return is not a standardized HR-recovery test. Do not invent an HRR value or consume emit-only DTO fields without the `AGENTS.md` promotion requirements.
- A non-maximal ride's best window is observed execution, not a new FTP estimate or proof of a fitness decline. Do not update FTP, W', load, or the calendar from this review alone.
- Missing/sparse channels, stale anchors, sensor exclusions, and uncertain phase intent lower confidence. Do not invent thresholds, physiological values, or a replacement plan.

## Deliverable and verification

Lead with the session's role and what actually happened. Provide chronological phases, the efforts/windows that explain the conclusion, power/HR/cadence response, limitations, and the consequence for the existing week. Preserve the approved plan's purpose before recommending changes.

For detailed inspection, deliver an interactive timeline with per-sample values and selectable ranges, plus the complete sample CSV and original download. A static chart is useful for sharing. The artifact must retain every record and show absent values / pauses; smoothing is optional and visibly labeled. Keep coordinates and unrelated personal metadata out of the displayed report unless needed. Use absolute artifact links in chat.

Each CSV row should retain original timestamp, elapsed time, raw channel values, missing/gap indicators, chosen FTP/%FTP/zone, and separately labeled recorded lap / interpreted phase. Report a record count separately from each channel's measured count. Include method/source metadata for calculations. Do not paste thousands of rows into chat.

Before reporting results:

- Reconcile measured zone coverage, explicit missing samples, pause accounting, and full-session phase coverage using the chosen time convention.
- Cross-check a representative selection of continuous best windows independently; confirm the denominator, exact timestamps, and included zeros.
- Inspect exported charts and verify that interactive range summaries use original samples. Do not call an artifact verified if it was not inspected.
- Keep all calculations and artifacts outside the repository unless the user asks to build reusable CLI support. The workflow does not require new repository code or dependencies.

### Acceptance scenarios

These are review scenarios, not new CLI tests or physiological defaults:

| Evidence | Required result |
| --- | --- |
| Detected intervals are absent or misleading, but complete power/HR/cadence exist | Reconstruct the execution from samples; do not infer that quality work was absent or reuse detector context. |
| An explicit instruction selects the analysis anchor for the session under review | Use it within that scope, retain the device snapshot and dated rationale, and re-evaluate applicability for another session; no permanent device-status or current-FTP inference. |
| A first FIT record has a timestamp but no sensor fields; processed streams contain filled values | Preserve original absence; measured channel count is lower than record count. |
| Timer events identify pauses | Preserve gaps; continuous windows cannot cross them and moving/timer/elapsed totals retain their own meanings. |
| Real coasting and one sensor dropout occur in the same ride | Keep measured zeros; exclude/flag the absent channel without converting it to zero. |
| Recording cadence is irregular | Derive covered time correctly or report sample counts; never count every row as one second by assumption. |
| An intentionally repaired stream differs from the original | Use the declared corrected source for its metric and preserve an audit comparison; never silently mix versions. |
| Hard surges alternate with coasting and the ride ends easily | Report the mixed execution; do not equate Z5 with measured VO2max or global decoupling with endurance durability. |
| Only incomplete streams or rollups are available | State the fallback and coverage; keep unsupported sections unavailable. |
