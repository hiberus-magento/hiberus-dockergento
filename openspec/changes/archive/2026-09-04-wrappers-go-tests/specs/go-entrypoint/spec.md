# Delta for go-entrypoint

## ADDED Requirements

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

### Requirement: A usage error returns before any engine call

`copy-to-container` and `copy-from-container` SHALL validate that a path was given before they
ask anything of the engine.

#### Scenario: No path given

- **WHEN** either command runs with no path argument
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
