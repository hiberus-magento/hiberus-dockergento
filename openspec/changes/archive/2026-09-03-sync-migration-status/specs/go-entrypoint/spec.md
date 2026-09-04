# Delta for go-entrypoint

## MODIFIED Requirements

### Requirement: The state of the migration is written down and true

The repository SHALL carry a document saying where the migration is, and a test SHALL keep it
from drifting in both directions: every row the document marks `go` SHALL exist in the Go tree,
and every command the Go tree fully answers for SHALL be marked `go` in the document.
(Previously: only the forward direction was checked — rows marked `go` had to exist in the Go
tree. A command fully wired in Go but still tabled `shell` passed the suite undetected.)

#### Scenario: Every command is accounted for

- **WHEN** the document is compared with the command declarations
- **THEN** every command appears exactly once, with an owner

#### Scenario: A claim that is not true

- **WHEN** the document says a command is implemented in Go and nothing in the Go tree answers
  for it
- **THEN** the suite fails

#### Scenario: Carrying on later

- **WHEN** somebody opens the document
- **THEN** it says how to build it, how to test it, and which phase is next

#### Scenario: A command is fully wired but still tabled as shell

- **GIVEN** a `case "<command>":` label in `internal/cli/run.go`'s `Run()` switch, other than the
  `_registry` label
- **WHEN** the document tables that command as `shell`
- **THEN** the suite fails and names the command

#### Scenario: A command routed by more than one implementation

- **GIVEN** a command whose subcommands are split between the Go and shell implementations
  (currently `db` and `proxy`)
- **WHEN** the reverse check runs
- **THEN** that command is excluded through an explicit, enumerated exception — never a silent
  skip — and its exclusion is never reported as drift while only some of its subcommands are
  routed

#### Scenario: The counter and the table agree

- **WHEN** the document's declared Go count is compared against rows marked `go`
- **THEN** they match, and the total matches the number of documented commands

#### Scenario: The reconciliation touches no runtime code

- **GIVEN** this fix reconciles the document and the test against `run.go` as the single source
  of truth
- **WHEN** the resulting change is diffed against its base branch
- **THEN** no file under `console/`, `bin/run`, `internal/`, or `dockergento/` is modified — only
  the test file and the affected documents change

## ADDED Requirements

### Requirement: Documentation does not overstate what remains unported

The documents that describe this repository's implementation SHALL NOT claim the implementation
is entirely or 100% Bash once any command is wired in Go.

#### Scenario: Describing how a command reaches its implementation

- **WHEN** a document explains the routing between the two implementations
- **THEN** it describes the strangler pattern: the `cmd/hm` Go binary as the entry point,
  `bin/run` as the bridge for what is not yet ported, and `internal/cli/run.go`'s switch as what
  decides between them

#### Scenario: No absolute claim remains

- **WHEN** `CLAUDE.md` and `architecture/02-cli-architecture.md` are read
- **THEN** neither states or implies the implementation is entirely or 100% Bash
