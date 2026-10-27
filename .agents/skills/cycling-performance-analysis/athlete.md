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

Apply [the repository evidence policy](../../../AGENTS.md#durable-rules-and-scoped-evidence) to athlete context. Retain athlete, session/date range, sport/environment, purpose, and evidence/effective dates with any FTP override. Report a prescribed reference as a constraint rather than a fitness measurement; assess each recording's device snapshot within its own context.
