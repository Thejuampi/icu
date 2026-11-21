# Zepp cycling backfill prototype

This is a read-only library preview, not a CLI upload feature.

- `ZeppClient.PlanRideBackfill` compares caller-supplied Intervals.icu rides
  against the paginated Zepp run-history feed with an already configured
  client. It never logs in. Incomplete responses, auth failures and redirects
  stop the preview. `not-in-feed` means only that no overlap was observed in
  that feed; it is not proof of absence from Zepp or permission to upload.
- `DecodeZeppRideFIT` decodes a local, single-session cycling FIT with bounds
  and CRC checks. `EncodeZeppRideFIT` builds provisional form fields based on
  static format analysis for Zepp Android 10.9.0-play (151983). The known
  cycling series are covered by synthetic fixtures. The complete
  distance, laps and pauses contract is unverified. Neither function does I/O
  beyond reading the caller's FIT reader.
- No POST or DELETE transport is wired. Mock responses cannot establish that
  a saved web session can safely write and undo a workout, or that the feed
  covers all conflicting sports. Do not use this prototype for writes.

Run the focused mock and codec tests with
`go test . -run 'Test(BackfillHistory|PlanRideBackfill|ZeppRideFIT|ZeppFIT)'`.
