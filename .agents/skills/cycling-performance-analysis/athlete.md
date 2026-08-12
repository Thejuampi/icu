# Athlete context

This skill is shared. Do not store one person's FTP, zones, ID, watt targets, or standing medical notes here.

Resolve live values from the `icu` CLI at the start of the job. Personal limits live on that athlete's calendar notes and recent sessions.

## Resolve from the CLI

1. `icu config show` for the default `athlete_id`. Use it unless the user names another athlete.
2. Ride sport settings (`icu sports get Ride`, or `sportSettings` on `icu analysis coaching`):
   - Outdoor FTP: `ftp`
   - Indoor FTP: `indoorFtp` when set, otherwise `ftp`
   - LTHR, max HR, HR zones, power zones
3. Timezone: athlete profile `timezone`. Use it for explicit `YYYY-MM-DD` dates.
4. Locale / note language: match existing calendar NOTE events, then the athlete profile locale, then the user's chat language.

If a field is missing, say so. Do not invent a substitute watt number.

## Power is relative

- Prescribe and write sessions in **%FTP** (and HR when the session is HR-capped).
- Never hardcode watts in this skill, in a template, or as a standing calendar target.
- Outdoor `Ride` uses `ftp`. `VirtualRide` / indoor uses `indoorFtp` when set.
- `icu workouts calculate --ftp FTP` takes the live FTP for that environment.
- If the athlete asks "how many watts is that?", multiply the %FTP by the live FTP and name the source. Do not persist the product in the skill or as the written target.

## Personal ceilings

Intervals Ride Z2 is often wider than useful durable work.

- Default to sport-settings zones.
- If calendar notes or recent durable rides set a tighter HR ceiling than the chart, use that ceiling for this athlete and say it came from notes or history.
- Do not copy that ceiling into this file.

Clinical, skin, and return-to-bike rules also live on the calendar. See SKILL.md → Disruption week.
