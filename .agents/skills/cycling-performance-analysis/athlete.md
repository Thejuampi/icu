# Athlete profile — Thejuampi (`i445643`)

These are athlete constraints from Ride sport settings and recent durable rides. They are not model defaults and not Intervals zone-chart ceilings.

Read this file before scaling templates or talking about Z1/Z2. Do not copy these numbers into other skill files.

## Anchors

| Anchor | Value | Source |
|--------|-------|--------|
| Outdoor FTP | 270 W | Ride sport settings `ftp` |
| Indoor FTP | 285 W | Ride sport settings `indoorFtp` |
| LTHR | 178 bpm | Ride sport settings |
| Max HR | 198 bpm | Ride sport settings |
| Ride HR Z1 (chart) | ≤143 bpm | Intervals Ride zones |
| Ride HR Z2 (chart) | 144–158 bpm | Intervals Ride zones |
| True easy | ≤125 bpm | Recent recovery / easy work |
| Durable Z2 HR ceiling | ≤140 bpm | Recent durable rides |
| Default Z2 template | `z2_hr_control_waves` | Athlete preference |
| Favorite VO2 | 30/15 | Athlete preference; overload up to 4×13 when healthy |
| Timezone | America/New_York | Athlete profile |
| Calendar NOTE language | Spanish | Existing NOTE events |
| Chat language | Match the user | Session |

## FTP application

- Outdoor `Ride`: 270 W.
- `VirtualRide` / indoor: 285 W.
- Do not use 285 W for outdoor load calc.
- `ftpApplied` `confidence: medium` is usable. Mention it only when the session environment looks wrong.

## Zone intent (this athlete)

Intervals Ride Z2 goes to 158 bpm. That is not the long-ride target.

- True easy: ≤125 bpm, conversational whisper.
- Durable Z2: cruise about 130–140 bpm. Cut a valley before chasing 150+.
- High-Z2 / chart-top Z2: 140–150 bpm, short caps only.

Templates and %FTP bands live in [session-library.md](./session-library.md).

## Standing constraints

- Saddle-skin recurrence is a real limiter. Off-saddle weeks are tissue healing, not a deload.
- Clinical instructions (dentist, physician) beat training notes.
- Bike return needs an explicit gate already on the calendar. The date is negotiable; the gate conditions are not.

Do not encode a one-off clinical cap (for example a post-extraction HR number) here. That cap lives on the calendar note for that week.
