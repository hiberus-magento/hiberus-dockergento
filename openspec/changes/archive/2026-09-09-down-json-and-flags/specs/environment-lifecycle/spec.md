# Delta for environment-lifecycle

## MODIFIED Requirements

### Requirement: Removing an environment is Compose's own command

The tool SHALL do what Compose does, and SHALL NOT remove more than it was asked to.
(Previously: `--rmi` and `-t`/`--timeout` given as the last argument, with no value after them,
were silently dropped rather than refused.)

#### Scenario: Containers of a project no longer in the file

- **WHEN** an environment is removed without asking for orphans to go
- **THEN** they are left alone

#### Scenario: A branch environment being erased

- **WHEN** a branch environment is removed
- **THEN** its orphans go with it, because the environment is being erased rather than stopped

#### Scenario: An option that takes a value refuses when none follows

- **GIVEN** `hm down` is run with `--rmi` or with `-t`/`--timeout` as the last argument, with
  nothing after it
- **WHEN** the parse loop reaches that flag
- **THEN** the command exits 2, the environment is not touched, and in text mode stderr carries
  the legible message naming the flag plus the `  → ` hint while stdout is empty
- **AND** in `--json` mode stderr carries exactly one JSON envelope with `ok: false`,
  `error.code` 2, `error.type` "missing_value", a message and a hint, and stdout is still empty

Test: `TestDownRefusesAMissingRmiValue`, `TestDownRefusesAMissingTimeoutValue` (new, in
`internal/cli/down_test.go`; each with a text and a `--json` subtest, called directly against
`down(...)` with plain buffers — the refusal returns before `projectOr` and before the engine
call, so no fake and no Docker is involved).

#### Scenario: A missing value and a bad value stay distinguishable

- **GIVEN** `-t`/`--timeout` is the flag being refused
- **WHEN** the value is missing versus when a value is present but not a number (e.g.
  `hm down -t nada`)
- **THEN** the missing case answers `error.type` "missing_value" and the non-numeric case
  answers `error.type` "invalid_argument" — the two refusals are never collapsed into one type

Test: `TestDownRefusesAMissingTimeoutValue`, `TestDownStillRefusesANonNumericTimeout` (new; the
second pins the existing `invalid_argument` behavior so it cannot regress alongside the new
guard).

#### Scenario: A value that is given is still accepted

- **GIVEN** `--rmi all` or `-t 30`, alone or combined with `-v` and `--remove-orphans`
- **WHEN** `down` parses those arguments
- **THEN** the parse loop runs to the end without refusing, and the command reaches project
  resolution — from a directory that is not a project, that is exit 4 with `error.type`
  "project_not_configured", which is the exact answer asserted, never "some code other than 2"
- **AND** an option nobody declared still fails exactly as before, with the existing
  `invalid_argument` "Unknown option" usage error and exit 2

Test: `TestDownStillAcceptsAGivenValue`, `TestADownOptionNobodyDeclaredIsAUsageError` (new; both
call `down(...)` directly with plain buffers from a temporary working directory, so resolution
fails predictably at `engine.go:84-89` and the run stops before `down.go:61` ever reaches a real
engine).

**What this scenario does not prove, deliberately.** That `--rmi all` still *stores* `"all"` in
`core.DownOptions.Images`, and `-t 30` still stores `30`, is not observable from outside `down()`
while `Down` is off the `commands` interface. An assertion phrased as "did not refuse" would pass
against a fix that adds the guard and forgets the assignment — which is the original defect wearing
a different hat. Asserting the exact resolution failure is the strongest statement available
without the seam; proving the value reaches the engine is owed to `down-go-tests`, whose
`TestWhatDownAsks` reads it back off the fake's recorded `DownOptions`. This is a stated gap, not
an oversight.

## Handoff Note (informational — not a requirement, not merged by archive)

`archive/2026-09-05-down-and-set-host-go-tests/design.md:306` and `:340-342` currently plan for
`down-go-tests` to **pin** `--rmi` and `-t` given as the last argument as
`core.DownOptions{}`, "silently ignored". Once this change lands, that plan is stale: those two
`TestWhatDownAsks` table rows must arrive as refusals (asserting `missing_value` and an empty
engine log) instead of as pinned silent drops, or not be written at all. `down-go-tests` extends
the `internal/cli/down_test.go` this change creates rather than authoring it from scratch.
