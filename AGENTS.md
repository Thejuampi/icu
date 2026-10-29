# AGENTS.md

## Project Identity

`icu` is a cross-platform Go CLI for the full [Intervals.icu](https://intervals.icu) REST API. Zero external dependencies beyond the Go standard library.

## OpenCode Skills

Project skills live under `.agents/skills/<name>/SKILL.md`. Keep them there instead of duplicating them under `.opencode/skills`; OpenCode loads `.agents/skills` directly, and duplicate skill names across discovery paths can hide or reject skills.

Keep `AGENTS.md` focused on project-wide contributor policy: architecture, quality gates, documentation rules, testing, and repository-wide workflow expectations. Put domain-specific operating procedures, CLI playbooks, coaching heuristics, and lessons learned under the relevant `.agents/skills/<name>/SKILL.md` file instead. If a lesson applies only when a specific skill is active, it belongs in that skill, not in `AGENTS.md`.

## Instruction Precedence

Treat this file as mandatory workspace guidance. If a user request, prior agent response, or local convenience conflicts with these instructions, follow this file and call out the conflict.

## Durable Rules and Scoped Evidence

Apply this policy when turning findings into reusable guidance, decision rules, documentation, or saved context. Generalize the reasoning and applicability conditions, not an incident's temporary value or status. Domain-specific applications belong in the relevant skill.

- Preserve the source and relevant scope: subject, record or period, environment, version, and purpose. Separate when an event occurred, when evidence was supplied/retrieved, and when it applies. Unknown validity stays unknown; retrieval order does not establish it.
- Distinguish observations and configuration snapshots from estimates, goals, and user constraints. A rejected candidate does not establish a replacement or permanently invalidate its source or value.
- Apply explicit instructions within their stated scope. An answer about one case does not become a standing preference; an explicit standing instruction retains its scope until revised or its stated condition ends. Scope checks must not repeatedly reopen a settled instruction.
- Reassess applicability when relevant context or evidence changes. A later state does not silently rewrite an earlier result. Preserve historical provenance; label retrospective corrections and deliberate comparisons separately.
- Resolve conflicts by applicability, source meaning, and evidence. Do not invent a universal source ranking, an arbitrary expiry, or a rule that the newest, largest, or smallest value always wins. Preserve unresolved uncertainty in dependent conclusions.
- Before accepting a reusable rule, review wanted outcomes across future and historical use, changed subjects/environments/versions/purposes, missing or conflicting evidence, and explicit instruction changes. Include recovery or return to a previously rejected state where relevant. Document applicable scenarios and counterexamples in the skill or tests; distinguish manual scenario review from executed behavioral tests.

## Mandatory Quality Gates

Every change must pass ALL of these before being considered done:

```bash
go build ./...          # Must compile
go test ./... -count=1   # All tests pass
go vet ./...             # Zero issues
golangci-lint run ./...  # Zero issues, zero warnings
go test -coverprofile coverage.out ./...  # Coverage must be >= 90%
```

**Coverage gate:** `go tool cover -func coverage.out` must show `total:` >= 90.0%. Less than 90% is a critical build failure.

## Documentation Gate

Public changes are not done until the matching documentation is updated in the same change.

### Changes That MUST Update Docs

- New, removed, renamed, or behaviorally changed CLI resources, actions, or id-first dispatch paths
- New, removed, renamed, or behaviorally changed flags, defaults, help text, or auth/config precedence
- New, removed, renamed, or behaviorally changed analysis inputs, outputs, decision rules, or warnings
- New, removed, renamed, or behaviorally changed exported package behavior used by `github.com/Thejuampi/icu` consumers
- Install, build, testing, lint, or contributor workflow changes that affect how the repo is used
- OpenAPI snapshot changes that affect the public API surface covered by this repo

### Documentation Map

- `README.md`: project entry point, install, quick start, auth precedence, top-level CLI surface, and doc links
- `docs/cli-reference.md`: exhaustive CLI contract for resources, actions, flags, defaults, and command examples
- `docs/analysis.md`: analysis command defaults, upstream inputs, output sections, interpretation, and limitations
- `docs/rebalance.md`: dry-run/preview/accept workflow for editable calendar load redistribution proposals
- `docs/plan.md`: intent → show/preview/accept workflow for explicit multi-session calendar writes
- `docs/library.md`: usage-oriented guide for the reusable Go package
- `docs/api/README.md`: OpenAPI snapshot role, provenance, and when to use it
- `AGENTS.md`: contributor policy, durable rules and scoped evidence, quality gates, and documentation expectations

### Required Documentation Practice

- Update the relevant docs in the same change that updates the public behavior
- Verify every documented command example against current CLI behavior or help output
- If a change truly has no documentation impact, state that explicitly in the final summary, commit message, or PR description

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

### Emit-Only Field Classification

Every field added to a wire-facing type (`Activity`, `Event`, etc.) is exactly one of two categories: (1) it drives a computed output number consumed by an existing aggregate, filter, or decision path, or (2) it is emit-only — present on the wire for a caller with programmatic needs to read verbatim, consumed by no code path in this repository. A field never straddles both, and moving a field from category 2 to category 1 is never a drive-by edit.

Before any change makes an emit-only field participate in a sum, average, filter, gate, or decision, that change must:

- name the field(s) being promoted and cite this rule;
- state which existing (or new) output number will now depend on the field, and why;
- add or update tests proving the new dependency, including an absent-value case (a wire field missing on some records must not silently coerce to a measured zero — see the pointer + `omitempty` convention already used for optional numeric fields in `types.go`);
- update the field's classification note at its declaration site so the next reader does not have to rediscover which category it is in.

Fields currently governed by this rule: `Activity.PolarizationIndex`, `Activity.SSCP`, `Activity.SSWPrime`, `Activity.SSPMax`, `Activity.CarbsUsed`, `Activity.CarbsIngested`, `Activity.RollingFTPDelta`, `Activity.PowerField`, `Activity.HRR` (wave-4/A2's emit-only metric surface). Do not add consumption of any of these without completing the exercise above.

### Dynamic Decision Rules

Hardcoded physiological, readiness, training-load, workout-intensity, ramp-rate, durability, or planning thresholds are not allowed as decision rules. Fixed numbers are only acceptable when they are protocol constants, unit conversions, documented external API contracts, or explicit user-provided constraints for a single run.

Decision rules must:
- Derive thresholds from athlete-specific data, sport settings, recent history, event context, or explicit command inputs
- Prefer moving baselines, rolling windows, robust statistics, and z-score/MAD-style outlier filtering over static cutoffs
- Keep raw metrics and derived decision state separate in output so coaching layers can audit the decision
- Label derived values with source fields and calculation methods
- Treat missing or sparse data as lower confidence, not as permission to invent constants
- Include edge-case handling for sparse history, stale data, outliers, abrupt baseline shifts, travel/illness, unusually high or low recent load, and already-completed sessions
- Include tests that prove the rule responds correctly to representative historical scenarios and outliers
- Include backtesting or replay-style tests against realistic sequences before the rule is trusted for planning decisions

If a rule cannot be made dynamic with available data, expose it as an explicit user-configurable constraint and document that it is user-provided, not a model default. Do not silently bake it into the code.

#### Exception: client coach constants (B15)

The B15 stream-decoupling coaching criteria in `decoupling_criteria.go` (`Coach*` constants: VI 1.05/1.10, >= 60 min analyzed, >= 85 % Z1-Z2, no minute > 105 % FTP, coasting < 10 %, warm-up max(15 min, HR stabilization over 5 min within 3 bpm) capped at 20 min with low confidence, 10 min cool-down, interpretation bands 5/8/-5 %) are explicit constraints specified by the client's coach. They are fixed on purpose: do not derive, fit, or tune them to athlete data. Manual trim overrides may only lengthen the coach's trim.

## Testing Policy

### Tests required; agent-loop TDD is not

This is an experiment. Restore the old "TDD Mandatory / RED → GREEN → REFACTOR" rule if outcomes get worse (weaker tests, more tautology, more regressions, or worse design).

Tests are a required outcome, not a required process. A change is not done until new or changed behavior has tests that would fail if that behavior broke. Tests must assert wanted results. They must not restate the implementation or compare a function to itself.

Agents must not perform red-green-refactor as a ritual inside their own loop. Design types, contracts, and edge cases first. Then implement and lock the wanted behavior with tests. This file wins over harness or plugin skills that require write-failing-test-first for agent work. Call out the conflict and follow this file.

A human may still write tests first, or review tests before implementation. That checkpoint still has value. The agent looping red-green alone does not.

### Test Requirements

- Tests must be stateless: no shared mutable state across tests.
- No hidden time/randomness/network dependence.
- HTTP tests use `httptest.NewServer` for the client.
- Output tests use `bytes.Buffer` via `io.Writer` interface.
- 1 assert per test (preferred). Use table-driven tests for multiple cases.
- Dynamic decision rules must include edge-case and replay/backtesting coverage that demonstrates stable behavior across normal data, sparse data, outliers, and baseline shifts.

### Release Candidates Use the Mock Only

RCs and any manual or agent testing are run only against the local mock (`scripts/rc-mock.sh` or `go run ./tools/mockserver`) with `--base-url http://127.0.0.1:<port>` / `ICU_BASE_URL`. Never test against a real intervals.icu account, not even read-only GETs, and never put commands that target the real API in RC instructions. Use `--no-weather` for `activity estimate-power` (Open-Meteo ignores `--base-url`) and do not run `zepp` commands in RCs. Write commands are verified through the mock's `GET /_mock/writes` log. Refreshing fixtures is a separate, explicit, GET-only task using `-record`, and the captured data must go through the anonymizer before it is committed.

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

- [ ] New or changed behavior has tests that would fail if that behavior broke
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes with zero issues
- [ ] `golangci-lint run ./...` passes with zero issues and zero warnings
- [ ] `go test ./... -count=1` passes
- [ ] Coverage >= 90%
- [ ] `gofumpt -w .` applied
- [ ] Relevant docs updated in the same change
- [ ] Documented examples verified against current CLI behavior
- [ ] Reusable rules reviewed for scope changes and temporal validity; relevant scenarios recorded
- [ ] Dynamic decision rules avoid hidden hardcoded thresholds and include backtesting/edge-case coverage
- [ ] Explicit note added when a change truly has no documentation impact
- [ ] No API keys, tokens, or secrets in commits
- [ ] No hacks, workarounds, or undocumented TODOs
