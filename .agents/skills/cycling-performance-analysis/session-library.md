# Session Library (canonical templates)

Source of truth for planned WORKOUT shapes. Load this file only when writing or rewriting workouts.

Pick a template by role, scale duration/reps/IF to the week's load target, run `icu workouts calculate` with the FTP for that environment, then write the calendar with the Intervals-friendly desc. Desc parser, dual local/Intervals format, and `plan` / `rebalance` live in the **intervals-icu** skill.

Resolve live FTP, zones, and language with [athlete.md](./athlete.md). Do not put watt numbers or one athlete's FTP in this file.

**Not** a dump of every historical ride. Prefer these models; invent free-form structure only when no template fits.

Disruption / clinical-rest days do not use this library. Follow the calendar note. See SKILL.md → Disruption week.

## Session Structure Norm

Norm for planned rides, not for disruption-week movement.

Banned default shape: warmup → one constant main block → cooldown.

Required shape (every WORKOUT unless the athlete asked for a continuous free-ride, or the day is disruption/clinical rest):

1. At least two distinct work phases after openers, or one repeat block that alternates targets inside the session's band.
2. Phase changes every ~8–20 min on endurance rides longer than ~45 min.
3. HI / tempo / VO2 / over-under: openers, main sets, controlled spin-down.
4. Variety must not sneak intensity up a zone. Easy stays easy. Z2 peaks stay ≤ high-Z2. Valleys are HR-control floats, not full Z1 naps unless the session is recovery.
5. Name the pattern in the event title.
6. ALTs follow the same norm.
7. Build the varied desc, then calculate load. Do not flatten structure to hit TSS.

## Zone intent (coaching, not Intervals chart alone)

Intervals Ride Z2 is wide. Do not treat the chart ceiling as the long-ride target. Use sport-settings zones, or a tighter ceiling from that athlete's notes / recent durable rides.

| Intent | Power band (%FTP) | Role |
|--------|-------------------|------|
| Recovery Z1 | 45–58% | reset |
| **Durable Z2 (preferred)** | 60–72% with floats | most volume |
| High-Z2 / top chart Z2 | 72–75% | spice inside endurance |
| Tempo / SS (Z3) | 85–92% | controlled quality |
| Threshold / O-U (Z4) | 95–100% / 80–85% unders | threshold economy |
| VO2 (Z5) | 110–120% (30/15 ~112–115%) | 1×/week typical |
| Anaerobic Z6 | 120–150%, 20–60s | punch blocks only |
| Neuromuscular Z7 | max, 5–15s | sprints / event prep |

### Primary Z2 pattern (athlete preference) — `z2_hr_control_waves`

**Default aerobic pattern** unless the athlete asks otherwise:

1. Ride **2–3 min** at mid→high Z2 power (enough that HR drifts up).
2. When HR approaches the top of useful Z2 (or subjectively “pushing into Z3 feel”), **drop 30–40 s** to Z1 / very low Z2 power.
3. Repeat. Goal is hours with HR mostly in durable Z2, not sitting at the chart-top because the zone allows it.

Power sketch (local calc format):

```
- 10m 55-68%
Nx
  - 2m30s 70-74%
  - 35s 52-58%
- 10m 62-50%
```

Scale `N` and optional mid-block “settle” minutes to hit duration/TSS. Valleys are HR-control, not full stop.

---

## Template catalog

Each template lists:

- `id`, `category`, `role`, `priority`
- `title_pattern` for calendar names
- `local_desc` — for `icu workouts calculate` (no `Ramp`/`FTP` tokens required)
- `intervals_desc` — for `events create/update` (use `Ramp` + `FTP` + blank lines + `Nx` blocks)
- `scale_notes`
- `when`

Copy `local_desc` / `intervals_desc` and scale; do not flatten to a single constant main block.

### Z1 — Recovery

#### `z1_easy_undulate`

- **role:** recovery · **priority:** high
- **title:** `Easy Undulate {duration}`
- **when:** post-HI, residual illness, HRV weak, optional sun
- **scale:** change rep count; keep peaks ≤ low Z2

```
# local
- 8m 50-60%
6x
  - 3m 55-65%
  - 1m 48-55%
- 8m 50-52%
```

```
# intervals
- 8m Ramp 50-60% FTP

6x
  - 3m 55-65% FTP
  - 1m 48-55% FTP

- 8m 50-52% FTP
```

#### `z1_spin_optional`

- **role:** recovery optional · **priority:** high
- **title:** `Easy Play Optional`
- **when:** green-light optional day only

```
# local
- 8m 50-60%
5x
  - 3m 58-65%
  - 1m 50-55%
- 8m 52-48%
```

---

### Z2 — Aerobic (core volume)

#### `z2_hr_control_waves`  ★ DEFAULT Z2

- **role:** aerobic · **priority:** critical
- **title:** `Z2 HR-Control Waves`
- **when:** default mid-week Z2, indoor aerobic, absorb weeks
- **scale:** `N` for duration; optional 8–15m settle block mid-ride at 62–66%

```
# local (~75–90m example, N=12)
- 10m 55-68%
12x
  - 2m30s 70-74%
  - 35s 52-58%
- 10m 62-50%
```

```
# intervals
- 10m Ramp 55-68% FTP

12x
  - 2m30s 70-74% FTP
  - 35s 52-58% FTP

- 10m Ramp 62-50% FTP
```

**Execution cue:** watch HR; if still climbing through the valley, lengthen valley to 45–50s or lower work to 68–72%. If HR never approaches useful Z2, work may be too easy — nudge work to 72–74% before adding junk tempo.

#### `z2_cruise_float`

- **role:** aerobic · **priority:** high
- **title:** `Z2 Cruise-Float`
- **when:** slightly longer steady feel; still not flat

```
# local
- 12m 55-70%
6x
  - 8m 66-72%
  - 2m 55-60%
- 10m 62-52%
```

#### `z2_low_mid_rotate`

- **role:** aerobic · **priority:** high
- **title:** `Z2 Low-Mid Rotate`
- **when:** variety day; keep HR honest on “mid” blocks

```
# local
- 10m 55-68%
5x
  - 5m 62-66%
  - 4m 68-72%
  - 1m 52-58%
- 10m 60-50%
```

#### `z2_ladder`

- **role:** aerobic · **priority:** medium
- **title:** `Z2 Ladder`
- **when:** build aerobic without HI; top step is high-Z2 cap only

```
# local
- 12m 55-70%
- 12m 62-66%
- 12m 66-70%
- 12m 70-74%
4x
  - 4m 72-75%
  - 2m 58-62%
- 12m 66-70%
- 12m 60-50%
```

#### `z2_durable_story`  (long)

- **role:** long_endurance · **priority:** critical
- **title:** `Z2 Durable Story {duration}`
- **when:** weekend long; multi-act, includes HR-control waves in the middle
- **scale:** extend settle blocks and wave counts; peaks ≤75% FTP

```
# local (~2h45–3h sketch)
- 15m 55-70%
- 25m 62-66%
8x
  - 2m30s 70-74%
  - 35s 52-58%
- 20m 64-68%
6x
  - 2m30s 70-74%
  - 35s 52-58%
- 15m 66-70%
4x
  - 3m 72-75%
  - 2m 58-62%
- 15m 62-52%
```

**Long-ride HR rule:** average HR for pure durable days should sit in useful Z2, not at the chart-top. If decoupling climbs and HR is stuck high, more/longer valleys.

#### `z2_strict_absorb`

- **role:** aerobic absorb · **priority:** medium
- **title:** `Z2 Absorb`
- **when:** post-overload or post-illness return; still multi-phase, lower work power

```
# local
- 12m 55-65%
10x
  - 2m 65-70%
  - 40s 50-58%
- 12m 58-50%
```

---

### Z3 — Tempo / Sweet spot

Always **openers + main + spin-down**. Historical athlete set: 2×20 tempo, 3×10/12/15 SS, 3×18 SS.

#### `z3_tempo_2x`

- **role:** tempo_threshold · **priority:** high
- **title:** `2x{N} Tempo`
- **doses:** 2×12, 2×15, 2×20 @ 88–92%
- **when:** build light; bridge to threshold; no VO2 that week if load tight

```
# local 2x12
- 10m 55-75%
4x
  - 1m 80-88%
  - 1m 55-60%
- 5m 60-65%
2x
  - 12m 88-92%
  - 5m 55-60%
- 5m 60-65%
3x
  - 1m 75-85%
  - 1m 55-60%
- 12m 60-50%
```

#### `z3_ss_3x`

- **role:** tempo_threshold · **priority:** high
- **title:** `3x{N} Sweet Spot`
- **doses:** 3×10, 3×12, 3×15 @ 88–92% (hist. also 3×18)
- **when:** volume quality without full threshold

```
# local 3x12
- 10m 55-75%
3x
  - 45s 90-100%
  - 75s 55-60%
- 5m 60-65%
3x
  - 12m 90-92%
  - 4m 55-60%
- 12m 60-50%
```

---

### Z4 — Threshold + over-unders

#### `z4_threshold_2x`

- **role:** tempo_threshold · **priority:** medium
- **title:** `2x{N} Threshold`
- **doses:** 2×8–2×12 @ 95–100%
- **when:** threshold economy; not every week

```
# local 2x10
- 10m 55-80%
4x
  - 1m 90-100%
  - 1m 55-60%
- 5m 60-65%
2x
  - 10m 96-100%
  - 5m 55-60%
- 12m 60-50%
```

#### `z4_over_under_2x`

- **role:** tempo_threshold · **priority:** high
- **title:** `2x Over-Unders`
- **when:** broken threshold; pairs well after VO2 week or instead of long threshold
- **hist.:** 3×12 O/U, 3×15 O/U, 2×15 O/U

```
# local
- 10m 55-80%
3x
  - 1m 85-95%
  - 1m 55-60%
- 5m 60-65%
2x
  - 3m 95-98%
  - 2m 80-85%
  - 3m 95-98%
  - 2m 80-85%
  - 3m 95-98%
  - 5m 55-60%
- 12m 60-50%
```

---

### Z5 — VO2 (30/15 family primary)

Default VO2 family is 30/15. Historical doses: re-entry 3×13 → build 3×13 → overload **4×13**; also 2×13+2m, 4×4, 4×5, 5×3.

#### `z5_vo2_3015_reentry`

- **role:** high_intensity · **priority:** high
- **title:** `3x8 30/15 VO2 Re-entry`
- **when:** first VO2 after illness/gap or low CTL

```
# local
- 10m 55-80%
4x
  - 45s 95-105%
  - 75s 55-60%
- 5m 55-60%
8x
  - 30s 112-115%
  - 15s 50%
- 5m 50-55%
8x
  - 30s 112-115%
  - 15s 50%
- 5m 50-55%
8x
  - 30s 112-115%
  - 15s 50%
- 12m 58-45%
```

#### `z5_vo2_3015_std`

- **role:** high_intensity · **priority:** critical
- **title:** `3x{10-13} 30/15 VO2`
- **when:** standard build VO2 (default quality day)

Flatten to three `Nx` 30/15 blocks (Intervals: no nested reps). Example 3×13:

```
# local main pattern per set = 13x (30s 112-115% / 15s 50%), 5m easy between sets
```

#### `z5_vo2_3015_overload`

- **role:** high_intensity · **priority:** high
- **title:** `4x13 30/15 VO2 Overload`
- **when:** only if wellness OK, recent Z2 durable intact, not post-illness week
- **note:** athlete can execute this; cost is recovery — gate with HRV/sleep/RHR

#### `z5_vo2_longrep`

- **role:** high_intensity · **priority:** medium
- **title:** `4x4 VO2` or `4x5 VO2` or `5x3 VO2`
- **when:** variety / outdoor-friendly; secondary to 30/15

```
# local 4x5
- 10m 55-80%
4x
  - 45s 100-110%
  - 75s 55-60%
- 5m 55-60%
4x
  - 5m 112-115%
  - 4m 55-60%
- 12m 58-45%
```

---

### Z6 / Z7 — Optional (low priority)

Not core for current blocks. Use when event demands punch or once every 10–14 days to avoid total neuromuscular loss.

#### `z6_short_repeats` (optional)

- **title:** `Z6 30/30 Micro`
- **when:** crit/final prep; never post-illness priority

```
# local — short session or finisher after Z2
- 10m 55-70%
6x
  - 30s 125-140%
  - 30s 50%
- 10m 55-50%
```

#### `z7_sprints` (optional)

- **title:** `Z7 Sprints 6x10s`
- **when:** neuromuscular touch; full recoveries

```
# local
- 15m 55-70%
6x
  - 10s 150%
  - 3m 50-55%
- 10m 55-50%
```

---

## Weekly assembly cheat-sheet

| Week role | Typical picks |
|-----------|----------------|
| Disruption / clinical rest | Do not pick a library ride. Follow the calendar note. |
| Health / absorb | `z1_*`, `z2_strict_absorb`, `z2_hr_control_waves` short |
| Reentry | waves + cruise-float + short durable story; no overload VO2 |
| Build | durable story + `z5_vo2_3015_std` or `z3_*` + waves |
| Overload | `z5_vo2_3015_overload` **or** heavy SS — not both maxed |
| Deload | undulate + short waves; optional spin |

**Default quality family:** 30/15 over long VO2 reps when choosing one HI day.

## Scaling + rebalance

1. Pick template → scale duration/reps to session TSS budget.
2. `icu workouts calculate --ftp FTP --desc LOCAL` using live outdoor `ftp` or indoor `indoorFtp` from Ride sport settings.
3. Write the event with the Intervals desc plus `moving_time` / `training_load` (see **intervals-icu** dual-format / `--calculate-load` rules).
4. If the **week** is off target but sessions already structured: `icu rebalance` with preserve-structure rather than rewriting patterns from scratch.
5. ALTs use the same library (`z2_hr_control_waves` as default ALT for skipped HI).

## Historical coverage (why these exist)

From ~20 weeks 2026 + 2025 named indoor work: Z2 durable/reset/absorb/float, SS 3×10–18, tempo 2×20, O/U 2–3×, VO2 30/15 up to 4×13, VO2 4×4/4×5/5×3, recovery spins, cadence play, group/Zwift. Library encodes the **repeatable cores**, not every one-off Zwift map name.
