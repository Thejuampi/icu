# Recovery-history scenario checks

Use these synthetic scenarios when changing the recovery-history requirement. They are manual behavior-review cases, not executed integration tests or physiological thresholds. No real account reads are needed. Check that an answer names what was actually read, preserves scope, and never supplies invented baseline values.

| Scenario | Expected behavior |
| --- | --- |
| Daily advice, only today's HRV read, historical wellness source available | Read historical HRV, sleep, and resting HR before advising; keep unrelated activity history narrow. |
| Connector has no wellness tool, but calendar and load are readable | State lack of wellness access, not absence of records. Do not claim the planned tool is deployed. Give only qualified provisional advice within the existing evidence and constraints. |
| Tool exists but request fails | Identify failed read and unknown coverage; no claim of reviewed history or normal recovery. |
| Successful range read returns no records | Report no records in that returned range, not no history anywhere; check a relevant earlier period if available. |
| Records contain HRV and resting HR but no sleep fields | Report missing sleep separately; preserve valid measurements and limit dependent conclusions. |
| Today's record is empty, older signals have different latest dates | State each signal's actual latest date; do not infer syncing or use yesterday as a universal fallback. |
| Sparse or stale history, outlier, or device/HRV-method change | Describe limitations and source/method compatibility; no invented fixed thresholds or synthetic normal values. |
| Raw `hrv` has no verified method/device/units; projection marks another field `not_returned` | Do not call HRV RMSSD or infer its units; explain that omitted projected data do not prove absence upstream. |
| Sleep score exists but duration does not | Report score as score and duration unavailable, not a complete sleep review. |
| Old ride reviewed today with later wellness available | Use date-applicable history for historical readiness; label any retrospective comparison. Current advice needs current applicable history. |
| Illness/travel causes a baseline shift | Preserve the earlier baseline's scope; explain uncertainty and clinical constraints rather than fitting a universal cutoff. |
| Later observations return to the earlier baseline | Reassess the current history; neither prior suppression nor prior clearance remains a permanent rule. |
| Different athlete or changed recommendation period | Resolve that athlete's period and baseline independently; do not transfer another person's values or stale conclusions. |
| Completed ride reconstruction without training advice | Continue observed execution analysis; do not imply recovery was checked. Read history before adding next-session advice. |
| Weekly review or plan already has a wellness aggregate | Inspect available historical records and per-signal coverage before recommendations; an aggregate alone does not prove the history check occurred. |

During review, distinguish a wording/source inspection from an executed behavioral test. Repository Go tests validate code, not an agent's compliance with these coaching instructions.
