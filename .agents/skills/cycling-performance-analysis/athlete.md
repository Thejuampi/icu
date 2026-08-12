# Athlete context

Read these from the CLI for the configured athlete.

1. `icu config show` → default `athlete_id`
2. Ride sport settings (`icu sports get Ride`, or `sportSettings` on `icu analysis coaching`):
   - Outdoor: `ftp`
   - Indoor / `VirtualRide`: `indoorFtp` if set, else `ftp`
   - LTHR, max HR, HR zones, power zones
3. Timezone from the athlete profile. Use it for `YYYY-MM-DD` dates.
4. Note language: existing NOTE events, then athlete locale, then chat language.

Write session targets as %FTP. If notes or recent durable rides set a tighter HR ceiling than the chart, use that.

`icu workouts calculate --ftp` uses the live FTP for that environment.
