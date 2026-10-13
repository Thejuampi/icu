# Anchor aging checks

Use when changing anchor guidance or checking whether a remembered statement still applies. This is a scenario-based review of skill instructions, not an automated model evaluation or a second implementation of the CLI FTP resolver. Run each case against the written rule and explain the selected basis, scope, uncertainty, and invariant raw evidence. Go test coverage does not prove an agent will follow the policy.

## Review method

1. Identify athlete, activity/local date or requested period, sport/environment, and purpose from the case. Separate recording time, evidence time, and effective period.
2. Read the case's evidence without adding a hidden history, physiological estimate, permanent preference, or expiry period.
3. Apply [the anchor rule](./completed-session.md#resolve-the-training-anchor), then compare the proposed response with the independently stated wanted result below.
4. Mark a case failed if its response changes another scope, claims missing validity, reasks a settled instruction, discards conflicting evidence, or changes recorded watts. State whether it was a review judgment or an executed numerical/artifact check; do not report a scenario table as an automated behavioral test.

All numbers below are fictional fixture inputs, not athlete data, physiological thresholds, or model defaults. Each case starts fresh unless it explicitly references another step in the replay. Dates are athlete-local; a named session is a specific activity identity, not every ride on that date.

## Replay across fitness and configuration changes

Athlete A, outdoor Ride R1 on 2026-01-05: its device snapshot is 300 W; the athlete explicitly requests 260 W **for R1**. A later outdoor R2 on 2026-02-10 has an applicable 280 W reference. An older outdoor R0 on 2025-12-15 has contemporaneous evidence for 300 W. Indoor I1 on 2026-02-11 has an applicable 245 W reference. These effective dates are supplied fixture evidence, not inferred from retrieval order.

| Case | Evidence / requested analysis | Wanted result |
| --- | --- | --- |
| A01 — same session | R1 is reviewed again with no new conflicting instruction | Reuse the R1-scoped 260 W basis, preserve the 300 W device snapshot, and avoid repeating the settled question. |
| A02 — later recovery | Review R2 after the supplied change to 280 W | Use R2's applicable 280 W basis. R1's instruction neither supplies the current FTP nor invalidates the new recording. |
| A03 — historical backfill | Review R0 after the R1 statement about its device setting | Use R0's supported 300 W attribution. A later rejection for R1 cannot retroactively invalidate R0. |
| A04 — environment change | Review indoor I1 after outdoor R1/R2 | Use the applicable indoor 245 W basis; no outdoor override crosses environment by implication. |
| A05 — snapshot reread | Review R1 after fetching current 280 W settings | Keep R1's 260 W analysis basis. Retrieving a new setting does not establish a historical effective period or revise R1. |
| A06 — explicit renormalization | Re-express R1 against 280 W for comparison with R2 | Label a common-reference comparison. Preserve R1's historical 260 W attribution; raw watts and absolute-power windows remain unchanged. |
| A07 — retrospective correction | New explicit evidence says R1 should instead be analyzed at 270 W | Revise R1 with the new dated rationale, retaining the earlier basis in provenance. Do not alter R0, R2, I1, or current settings. |

## Independent scope and evidence cases

| Case | Evidence / requested analysis | Wanted result |
| --- | --- | --- |
| A08 — another athlete | Athlete B uses a shared device after A's R1 confirmation | Resolve B's evidence. Neither A's 260 W instruction nor A's configuration diagnosis transfers to B. |
| A09 — another device | A records a later ride with a different or reconfigured device | Evaluate that recording's snapshot and applicable evidence; no inherited obsolete-device flag. |
| A10 — explicit period | The athlete instructs use of 260 W for outdoor rides in a named week | Reuse within that athlete/environment/week. A ride outside the week requires its own applicable basis. |
| A11 — standing constraint | The athlete explicitly requests 260 W for future outdoor prescriptions until revised; live configuration later says 280 W | Honor the stated prescription constraint and distinguish it from live configuration / measured fitness. Do not invent expiry or silently replace it. It does not rewrite historical execution or indoor prescriptions. |
| A12 — historical evidence absent | Only today's 280 W settings and an old device snapshot are available; historical validity is unestablished | Keep the source disagreement and lack of coverage explicit. Raw analysis continues; historical %FTP/zone conclusions remain provisional or unavailable rather than fabricated. |
| A13 — unknown effective date | A newer note was retrieved today, but it never states when its anchor applied | Retrieval order does not make it effective for the ride. Do not choose it solely because it was fetched later. |
| A14 — same-scope conflict | Two records cover the same session/environment with incompatible anchors and no settling instruction | Report the conflict. Ask if material to the requested conclusion; otherwise preserve raw analysis and qualify anchor-dependent results. Do not automatically choose larger, smaller, newer, device, or server. |
| A15 — midnight / timezone | A ride starts on the previous athlete-local date even though its UTC date is the next day; references change on named local dates | Attribute using the supplied local effective dates and actual session context. UTC date or array position cannot shift the applicable reference. |
| A16 — abrupt interruption | Illness/off-bike time occurs after a formerly applicable anchor, with no new measured/configured anchor | Treat suitability as uncertain; do not invent an FTP reduction or promote the earlier per-session instruction to today's fitness. |
| A17 — cached report | A saved R1 report contains 260 W; current planning requests a new period | The saved report is R1-scoped evidence. Resolve current planning from applicable sources and explicit constraints; never copy its anchor into a new plan as a default. |
| A18 — source/version change | Corrected current streams differ from the original, or a new parser/export changes reported fields | Retain source/version provenance; assess scope and integrity again. A correction to watts is distinct from selecting an FTP reference. Do not silently mix versions or let the old source ranking settle the discrepancy. |
| A19 — incompatible zone context | FTP is session-applicable, but available HR/power zones only describe a different period or environment | Retain the valid FTP basis; do not claim the mismatched zone definitions are session-specific. Keep affected zone conclusions provisional/unavailable, or explicitly label a requested comparison. |
| A20 — another ride on the same day | A second outdoor ride has a different activity ID after a one-activity override | Same date/environment alone does not extend a one-activity instruction. Resolve the second ride separately unless the athlete supplied a wider period. |
| A21 — rejection without replacement | The athlete rejects a device snapshot for the ride but supplies no alternative and no other applicable anchor exists | Do not invent a confirmed replacement, default to a remembered number, or turn a goal/estimate into FTP. Continue raw analysis and resolve attribution only if needed. |
| A22 — different quantity | A recent workout target, aspirational FTP goal, or modeled eFTP differs from the applicable configured/session anchor | Preserve each quantity's meaning. Freshness and numerical similarity do not turn a target, goal, or model prediction into the session's FTP. |
| A23 — return to the same number | A later session has independent evidence for 300 W, the same number rejected for R1 | Use the later applicable basis. A rejection attached to R1 is not a blacklist of 300 W or of the device. |
| A24 — later decline | Another later outdoor ride has independent applicable evidence for 240 W | Use its supported 240 W basis. Do not keep the earlier 260/280 W values, select the larger number, or infer a decline solely from age. |

## Numerical and output invariants

Holding the recording version fixed, fixture power 280 W reported against 260 W gives about 107.69% FTP; against 280 W it gives 100%. The recorded power remains 280 W. In a deliberately renormalized comparison, configured zone classification and time/work above FTP may change; absolute-power best windows, timestamps, measured zeros, and missing coverage must not change. Calculate both references independently and keep the comparison labeled. A deliberately corrected recording version is a separate source change, not this invariant's premise.

Inspect any saved report or artifact for unqualified claims such as "current FTP is 260 W" or "the device FTP is obsolete" when the evidence only names R1. It should name the selected reference and scope instead. Do not write current settings, athlete memory, or the calendar as a side effect of resolving these cases.

## Aging findings addressed

The earlier guidance said to reuse an answer and use a confirmed anchor without specifying the answer's applicability. It also framed one device value as obsolete without dating or scoping that conclusion. Those instructions left A02–A05, A08–A10, A17, and A20 open to cross-scope reuse. Historical attribution already had a warning, but it was not enough to constrain a reused confirmation. The revised rule binds evidence to scope, separates effective time from retrieval time, preserves explicit constraints, and keeps unsupported attribution uncertain. Re-run all cases when this guidance changes.
