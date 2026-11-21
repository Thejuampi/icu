# Zepp cycling backfill test

This is a **single-ride experiment**, not bulk backfill. The write path is
off unless you pass `--real-write-test`. No command here logs in, refreshes
credentials, or calls Intervals.icu.

## One-ride test

Use a FIT you already have on disk. Verify that the phone app is signed in
and that the CLI config contains credentials from your existing
`icu zepp login --web` session. This command reads the saved config only.
It does not infer the login mode from the token, use environment token
overrides, or accept a Zepp URL override. It never asks you to log in.

```powershell
icu zepp backfill-test --real-write-test --fit ".\ride.fit"
```

The command checks the paginated run-history feed for overlap before any
write. If clear, it uploads the detail and summary, checks the new track in
history, deletes the identified workout by track ID, and checks that the
track ID is gone. A successful run reports `undo_verified: true`. Confirm
the phone still works afterward. A history read by the CLI cannot verify
the phone's session for you.

**Limitations:** Zepp's production upload and delete permissions for saved
web sessions are unverified. The run-history feed's coverage across sports
and its freshness are not established. Run one test at a time. A failed request, ambiguous
readback, or invalidated session can leave a partial or visible workout.
The command stops on authentication failures without retrying or logging
in. If it reports an uncertain write, don't rerun: use the reported track ID
to inspect the phone history first. It will not delete a workout it cannot
identify confidently, including when history does not return the expected source.

## Offline preview and mock tests

- `ZeppClient.PlanRideBackfill` is a read-only library preview against the
  run-history feed; `not-in-feed` alone does not establish absence.
- `DecodeZeppRideFIT` and `EncodeZeppRideFIT` use provisional form fields
  based on static format analysis of Zepp Android 10.9.0-play (151983).
  The complete distance, laps, and pauses contract is unverified.

Run the focused synthetic tests without contacting the live service:

```sh
go test ./... -run 'Test(Backfill|PlanRideBackfill|RideBackfillCanary|ZeppBackfill|ZeppRideFIT|ZeppFIT)'
```
