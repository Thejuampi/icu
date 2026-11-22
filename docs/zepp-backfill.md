# Zepp cycling backfill test

This is a **single-ride experiment**, not bulk backfill. The write path is
off unless you pass `--real-write-test`. No command here logs in, refreshes
credentials, or calls Intervals.icu.

## Offline FIT preview

```powershell
icu zepp backfill-test --preview --fit ".\ride.fit"
```

This builds the same workout forms as the canary and reports record counts,
field names, and byte counts (before form escaping) as JSON. It never prints
field values or reads configuration; the account-specific `userid` and
`country` fields are empty in preview. It does not contact either service or
confirm Zepp would accept the upload.
Only one mode may be selected: `--preview` and `--real-write-test` cannot be
combined.

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

## Library previews and mock tests

- `ZeppClient.PlanRideBackfill` is a read-only library preview against the
  run-history feed; `not-in-feed` alone does not establish absence.
- `DecodeZeppRideFIT` and `EncodeZeppRideFIT` use provisional form fields
  based on static inspection of Zepp Android 10.9.0-play (151983). In
  that build, the FIT reader (`knh`) requires both coordinates; `n2g.c`
  selects valid position records and the `oo60.f` Trackdata builder
  aligns GPS, time, altitude and pace to them. Missing altitude is sent
  with the app's sentinel (`-2000000` centimeters), missing pace with
  its no-reading marker, and timestamped sensor readings keep their
  original times. No positions or samples are filled or interpolated.
  The CLI deliberately quantizes each absolute coordinate before taking
  deltas to avoid drift. It does not implement app smoothing, lap/pause
  inference or the complete distance contract.
- The APK request builders (`o5z`, `py60`, `z0l`) put common app fields
  in the POST form body and in the DELETE query. The CLI pins public app
  metadata for this build, uses a static locale/device profile
  (`en_US`, `UTC`, `android_32`), and uses country and user ID from its
  saved Zepp config only in write mode. Those fields do not change web
  authentication and do not establish that the service permits writes.
- The gap behavior was checked offline with a public Wahoo ELEMNT BOLT
  sample file; regression tests use synthetic records, not a rider's
  activity or coordinates.

Run the focused synthetic tests without contacting the live service:

```sh
go test ./... -run 'Test(Backfill|PlanRideBackfill|PreviewZeppRideFIT|RideBackfillCanary|ZeppBackfill|ZeppRideFIT|ZeppFIT)'
```
