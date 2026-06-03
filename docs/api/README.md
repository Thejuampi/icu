# OpenAPI Snapshot

`docs/api/intervals-icu-openapi.json` is a checked-in OpenAPI snapshot of the Intervals.icu API used for reference while developing the CLI and library.

## What It Is For

- inspecting endpoint and schema shape when adding or verifying API coverage
- cross-checking DTO fields against the upstream contract
- understanding raw REST resources behind the higher-level CLI commands

## What It Is Not For

- it is not the primary user guide for this repository
- it does not replace the shipped CLI contract documented in [../cli-reference.md](../cli-reference.md)
- it does not replace the library usage guide in [../library.md](../library.md)

## Provenance

- `intervals-icu-openapi.json`: the checked-in snapshot
- `intervals-icu-openapi.source.txt`: the provenance note for that snapshot

If the snapshot changes, update this directory and any user-facing docs affected by the new or changed public surface.
