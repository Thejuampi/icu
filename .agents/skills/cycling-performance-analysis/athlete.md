# Athlete context

Read these from the CLI for the configured athlete.

1. `icu config show` → default `athlete_id`
2. Current Ride sport settings (`icu sports get Ride`, or `sportSettings` on `icu analysis coaching`) for current prescriptions:
   - Outdoor: `ftp`
   - Indoor / `VirtualRide`: `indoorFtp` if set, else `ftp`
   - LTHR, max HR, HR zones, power zones
   - These are current configuration, not an FTP/zone history. For completed rides, resolve the session's dated anchor using [completed-session.md](./completed-session.md#resolve-the-training-anchor).
3. Timezone from the athlete profile. Use it for `YYYY-MM-DD` dates.
4. Note language: existing NOTE events, then athlete locale, then chat language.

`icu workouts calculate --ftp N` calculates with the supplied value; it does not fetch live FTP or establish when that value applies. Resolve `N` for the session's environment and purpose before calling it.

Retain the athlete, session/date range, sport/environment, purpose, and evidence date with any athlete-supplied override. An instruction about one ride does not become a standing FTP preference. An explicitly stated standing constraint retains its stated scope until revised; report it as a constraint rather than a fitness measurement. A device value rejected for one recording remains a snapshot to assess separately for other recordings.
