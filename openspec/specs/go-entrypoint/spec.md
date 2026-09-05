# go-entrypoint Specification

## Purpose
TBD - created by archiving change go-skeleton-and-bridge. Update Purpose after archive.
## Requirements
### Requirement: Everything not ported runs unchanged

The binary SHALL execute the shell implementation for every command it does not implement,
passing the arguments through untouched and returning its exit code.

#### Scenario: A command the binary does not implement

- **WHEN** the user runs any command through the binary
- **THEN** the shell implementation runs it and its output is identical to running it directly

#### Scenario: The exit code

- **WHEN** the shell implementation exits with a usage error, a refusal or a success
- **THEN** the binary exits with the same code, because they are a contract that callers branch on

#### Scenario: The terminal

- **WHEN** a command asks a question or streams output
- **THEN** it reads and writes the same terminal as it would without the binary

#### Scenario: The shell implementation cannot be found

- **WHEN** the binary cannot locate the shell tree
- **THEN** it says so and exits with the code for something wrong with the tool itself

### Requirement: The project is resolved by the Go layer

The binary SHALL resolve which project a directory belongs to, from the root that ends up being
used.

#### Scenario: A main checkout

- **WHEN** the directory is a project
- **THEN** its name, root, domain, Magento directory and topology are reported from its own
  properties

#### Scenario: A worktree with no environment of its own

- **WHEN** the directory is a git worktree with no registered environment
- **THEN** it resolves against the main checkout, which is what the refusals depend on

#### Scenario: A worktree with an environment

- **WHEN** the directory is a registered worktree
- **THEN** it resolves against itself: its own root, its own properties, and a project name and
  address derived from the parent's

#### Scenario: A directory that is not a project

- **WHEN** the directory has no properties
- **THEN** that is reported as an empty project rather than as an error, because several commands
  run outside one

#### Scenario: Checked against the shell implementation

- **WHEN** both implementations resolve the same project
- **THEN** they agree on the name, the root, the domain and the Magento directory

### Requirement: Saying which binary this is

The binary SHALL be able to report itself and what it resolved, without changing what any real
command does.

#### Scenario: The build

- **WHEN** the user runs `hm-go-version`
- **THEN** the version and the revision it was built from are printed

#### Scenario: The resolution

- **WHEN** the user runs `hm-go-project`
- **THEN** what the Go layer resolved is printed as JSON

#### Scenario: They cannot collide

- **WHEN** the names of these commands are compared with the shell implementation's
- **THEN** none of them is a command the shell implementation has

### Requirement: Installing the binary

The release SHALL publish a binary per platform, and the installation SHALL make it the command,
without taking the shell implementation away from a machine that cannot get it.

#### Scenario: An installation

- **WHEN** the tool is installed
- **THEN** the binary for that platform and architecture is fetched into the checkout and the
  command in the path points at it

#### Scenario: A machine that cannot fetch it

- **WHEN** the binary cannot be downloaded
- **THEN** the command points at the shell implementation and says so, rather than leaving the
  machine without a tool

#### Scenario: Finding the shell tree from where it is installed

- **WHEN** the binary is invoked through the symlink an installation leaves in the path
- **THEN** it finds the shell tree beside it, whatever directory it was invoked from

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

### Requirement: A command whose subcommands are ported one family at a time

The tool SHALL let a command be answered by either implementation depending on its subcommand,
and SHALL answer identically whichever one runs.

#### Scenario: A ported subcommand

- **WHEN** a subcommand that has been ported is asked for
- **THEN** it is answered by the ported implementation, with the same documents, tables, refusals
  and exit codes as before

#### Scenario: One that has not

- **WHEN** a subcommand that has not been ported is asked for
- **THEN** it reaches the shell implementation unchanged

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

### Requirement: A ported command's engine interaction is provable without Docker

For a command listed in `internal/cli/run.go`'s `Run()` switch, the repository SHALL carry a Go
test that verifies what its handler asks of the engine, through a fake substituted for the real
one, requiring no Docker daemon, no network access, and no project beyond what the test itself
builds.

#### Scenario: Copying into the container

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `copy-to-container <path>` runs against a resolved project
- **THEN** the fake records a call to copy into the container with that path

#### Scenario: Copying everything into the container

- **WHEN** `copy-to-container --all` runs
- **THEN** the fake records the same call with the "everything" flag set

#### Scenario: Copying out of the container

- **WHEN** `copy-from-container <path>` runs
- **THEN** the fake records a call to copy out of the container with that path

#### Scenario: Turning the page cache on

- **WHEN** `varnish-on` runs
- **THEN** the fake records, in order, the VCL edit on the varnish service, a restart of the
  varnish service, and the enable call on the php service

#### Scenario: Turning the page cache off cascades

- **WHEN** `varnish-off` runs
- **THEN** the fake records the VCL edit, the restart, and the disable call, followed by the
  purge and the cache-clean that turning it off leaves behind, in that order

#### Scenario: Clearing generated code

- **WHEN** `purge` runs against a resolved project
- **THEN** the fake records one shell command, run as the web user, that removes exactly the
  generated paths the command lists

#### Scenario: Running the front-end package manager

- **WHEN** `npm <args>` runs
- **THEN** the fake records a call into the container that passes those arguments through
  verbatim to `npm`

#### Scenario: Running n98-magerun

- **WHEN** `n98-magerun <args>` runs
- **THEN** the fake records a call that runs `n98-magerun` with the arguments joined into one
  command, through a shell

#### Scenario: Running the unit suite

- **WHEN** `test-unit <args>` runs
- **THEN** the fake records a call that runs `phpunit` against the unit suite's configuration,
  with the arguments appended
- **AND** when the project defines no bin directory, the fallback bin directory is used

#### Scenario: Running the integration suite

- **WHEN** `test-integration <args>` runs
- **THEN** the fake records a call that changes into the integration suite's own directory, then
  runs `phpunit` against its configuration from the resolved working directory and bin
  directory, with the arguments appended
- **AND** when the project defines no working directory, the fallback working directory is used

#### Scenario: Writing a database dump

- **WHEN** `mysqldump <path>` runs against a resolved project
- **THEN** the fake records a request for a dump at that path from the current directory
- **AND** on success the command answers `{"path": <path>}` in JSON
- **AND** when the fake refuses the request, the command exits with the code reserved for
  engine/Docker failures

### Requirement: A usage error returns before any engine call

`copy-to-container`, `copy-from-container`, and `mysqldump` SHALL validate their arguments before
they ask anything of the engine.

#### Scenario: No path given

- **WHEN** either `copy-to-container` or `copy-from-container` runs with no path argument
- **THEN** it fails with the missing-path error, and the fake engine used by its test records
  zero calls

#### Scenario: mysqldump with no path

- **WHEN** `mysqldump` runs with no path argument
- **THEN** it fails with the missing-path error, and the fake engine used by its test records
  zero calls

### Requirement: The test seam changes nothing a real invocation runs

Adding a way to observe a handler's engine calls SHALL NOT change what production runs.

#### Scenario: Production still builds the real engine

- **WHEN** the binary runs any command outside a test
- **THEN** it is served by the real engine, unchanged, and a compile-time check confirms the
  real engine satisfies the interface the tests substitute

#### Scenario: Nothing else moves

- **WHEN** `go build ./...`, `go test ./... -short`, and `tests/run.sh unit` run after the seam
  is added
- **THEN** all three pass, and the Bash parity suites are unmodified

### Requirement: The migration document's truthfulness check does not depend on a pipe's producer finishing

`tests/unit/migration_status_test.sh`'s comparisons SHALL NOT read their result from a pipeline
whose early-exiting consumer can send its producer a broken pipe.

#### Scenario: A match found before the input is exhausted

- **WHEN** the comparison finds what it is looking for before its producer has written
  everything
- **THEN** the suite still reports the comparison's own result, not a broken-pipe exit code

### Requirement: A comment about what stays in shell names its real reason

A comment describing why part of a ported command still runs in shell SHALL name the actual
reason, not a stale one.

#### Scenario: The mac Composer vendor mirror

- **WHEN** the comment above `mirrorsVendor` in `internal/cli/php.go` is read
- **THEN** it says the vendor-mirror flow is what stays in shell, not that `copy-to-container`
  is unported — `copy-to-container` is wired in `run.go`

