# AGENTS.md

## Project Identity

`icu` is a cross-platform Go CLI for the full [Intervals.icu](https://intervals.icu) REST API. Zero external dependencies beyond the Go standard library.

## Instruction Precedence

Treat this file as mandatory workspace guidance. If a user request, prior agent response, or local convenience conflicts with these instructions, follow this file and call out the conflict.

## Mandatory Quality Gates

Every change must pass ALL of these before being considered done:

```bash
go build ./...          # Must compile
go test ./... -count=1   # All tests pass
go vet ./...             # Zero issues
golangci-lint run ./...  # Zero issues, zero warnings
go test -coverprofile=coverage.out ./...  # Coverage must be >= 90%
```

**Coverage gate:** `go tool cover -func=coverage.out | grep total` must show >= 90.0%. Less than 90% is a critical build failure.

## Architecture Non-Negotiables

### Functional Core + Imperative Shell

- **Functional core** (pure, deterministic, no IO): `types.go`, `auth.go`, `urls.go`, `output.go`
- **Imperative shell** (IO, HTTP, CLI dispatch): `client.go`, `main.go`, all `cmd_*.go`

Pure functions must:
- Accept explicit inputs, return explicit outputs
- Never access `os.Getenv`, filesystem, network, clock, or randomness
- Be unit-testable with table-driven tests

### SOLID

- Single Responsibility: each file handles one resource (e.g., `cmd_athlete.go` = athlete commands only)
- Open/Closed: new endpoints are added by registering commands, not modifying existing code
- Liskov: DTOs (`types.go`) are plain data; no behavior attached
- Interface Segregation: `Client` exposes focused methods (`Get`, `Put`, `Post`, `Delete`, `Download`, `UploadFile`)
- Dependency Inversion: CLI commands depend on `Client` interface, not HTTP details

### DRY

If the same URL pattern, flag parsing, or error handling appears twice, extract it. Current shared helpers live in `common.go`, `commands.go`, and the `registerActivityDetail` / `registerActivityCurve` helpers in `cmd_activity.go`.

### KISS + YAGNI

- No external dependencies beyond stdlib. Period.
- No ORMs, no frameworks, no code generation.
- Every flag and command must map to an actual intervals.icu API endpoint.
- No speculative features.

## Testing Policy

### TDD Mandatory

RED → GREEN → REFACTOR. Write the failing test first, then implement.

### Test Requirements

- Tests must be stateless: no shared mutable state across tests.
- No hidden time/randomness/network dependence.
- HTTP tests use `httptest.NewServer` for the client.
- Output tests use `bytes.Buffer` via `io.Writer` interface.
- 1 assert per test (preferred). Use table-driven tests for multiple cases.

### Mutation Testing

```bash
go install github.com/zimmski/go-mutesting/cmd/go-mutesting@latest
go-mutesting ./...
```

Run on the functional core (`auth.go`, `urls.go`, `output.go`). Mutation score must not decrease.

## Code Quality

- `gofumpt -w .` before committing (enforced by golangci-lint).
- All linters in `.golangci.yml` must pass with zero issues and zero warnings.
- Never disable a linter rule to work around lazy code. Fix the code instead.
- No commented-out code, no `// TODO` without a tracking issue.
- Error values must be checked. Use `_` only when intentionally discarding with a comment explaining why.

## Definition of Done

All of the following must be true:

- [ ] Failing tests written first (TDD)
- [ ] Implementation makes tests green
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes with zero issues
- [ ] `golangci-lint run ./...` passes with zero issues and zero warnings
- [ ] `go test ./... -count=1` passes
- [ ] Coverage >= 90%
- [ ] `gofumpt -w .` applied
- [ ] No API keys, tokens, or secrets in commits
- [ ] No hacks, workarounds, or undocumented TODOs
