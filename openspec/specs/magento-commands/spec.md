# magento-commands Specification

## Purpose
TBD - created by archiving change php-commands-in-go. Update Purpose after archive.
## Requirements
### Requirement: Running the Magento CLI

The tool SHALL run the Magento CLI inside the php container and SHALL answer with its exit code.

#### Scenario: The same invocation

- **WHEN** `hm magento <anything>` runs
- **THEN** the command, its output and its exit code are the shell implementation's

### Requirement: Running Composer

The tool SHALL run Composer inside the php container, and SHALL say beforehand what it will not do.

#### Scenario: The ordinary case

- **WHEN** a Composer subcommand that only reads is run
- **THEN** it runs in the container, with its own exit code

#### Scenario: Creating a project

- **WHEN** `composer create-project` is asked for
- **THEN** it is refused with the usage exit code, pointing at the command that does it

#### Scenario: A worktree that reads somebody else's dependencies

- **WHEN** Composer is asked to write dependencies in a worktree that shares the main checkout's
- **THEN** it is refused with the refusal exit code, naming the checkout they come from

#### Scenario: The invocation that rewrites the host's tree

- **WHEN** `install`, `update`, `require` or `remove` runs on macOS
- **THEN** it is handled by the shell implementation, which owns that sequence whole

### Requirement: Releasing for both platforms

The tool SHALL be released for macOS and for Linux, and the release SHALL fail rather than publish
a platform it could not build.

#### Scenario: A binary for each platform and architecture

- **WHEN** a release is built
- **THEN** there is a binary for darwin and linux, on amd64 and arm64

#### Scenario: What macOS needs

- **WHEN** the darwin binaries are built
- **THEN** cgo is enabled, because the file watcher reaches FSEvents through it

#### Scenario: A platform that stopped building

- **WHEN** any of the four fails to build
- **THEN** nothing is published

### Requirement: A wrapper runs what it says it runs

The tool SHALL pass a command and its arguments to the container as a command and its arguments.
A wrapper that also owns a JSON success envelope SHALL NOT emit that envelope, and SHALL NOT print
its success line, when the program it ran inside the container failed.
(Previously: covered `varnish-on`/`varnish-off`'s VCL edit and cache toggle. This adds `debug-off`'s
xdebug ini edit, the same failure shape against a different service.)

#### Scenario: A command line with options in it

- **WHEN** a wrapper runs a program with arguments inside the container
- **THEN** the program runs and receives them, rather than being looked for as a file whose name
  contains them

#### Scenario: The program fails

- **WHEN** the program inside the container fails
- **THEN** the exit code is the program's own

#### Scenario: The VCL edit fails inside the varnish container

- **GIVEN** `varnish-on` or `varnish-off` is run
- **WHEN** the `sed` edit against the varnish service exits non-zero with no Go-level error
- **THEN** the command exits with that same status, not a mapped code
- **AND** stdout carries no "Varnish cache enabled/disabled!" line; stderr carries the container's
  own output plus one readable line naming the VCL edit as the step that failed
- **AND** under `--json`, stdout carries no success document; stderr carries exactly one envelope
  with `ok: false` and `error.code` equal to that status
- **AND** the restart is not reached

Test: `TestAFailedVCLEditReportsTheContainersCode` (new).

#### Scenario: The full-page-cache toggle fails inside the php container

- **GIVEN** `varnish-on` or `varnish-off` is run
- **WHEN** `cache:enable`/`cache:disable` exits non-zero with no Go-level error
- **THEN** the command exits with that same status
- **AND** stdout carries no success line; stderr carries the container's own output plus one
  readable line naming the cache-toggle step
- **AND** under `--json`, stdout carries no success document; stderr carries exactly one envelope
  with `ok: false` and `error.code` equal to that status
- **AND** for `varnish-off`, neither the purge nor the cache-clean call runs afterward

Test: `TestAFailedCacheToggleStopsBeforeThePurge` (new).

#### Scenario: The xdebug ini edit fails inside the php container

- **GIVEN** `debug-off` is run
- **WHEN** the `sed` edit against the `phpfpm` service exits non-zero with no Go-level error
- **THEN** the command exits with that same status, not a mapped code
- **AND** stdout carries no "xdebug disabled" line; stderr carries the container's own output
  plus one readable line naming the ini edit as the step that failed
- **AND** under `--json`, stdout carries no success document; stderr carries exactly one envelope
  with `ok: false` and `error.code` equal to that status
- **AND** the restart (`Compose(["restart", "phpfpm"])`) is not reached

Test: `TestAFailedDebugOffEditReportsTheContainersStatus` (new; shared with `go-entrypoint`'s
scenario of the same fact, proved once).

#### Scenario: A Go-level error still stops before the restart

- **WHEN** the VCL edit fails with an error rather than a non-zero status
- **THEN** the restart is not reached and the failure is reported with the mapped Docker exit code

Test: `TestAFailedEditStopsBeforeTheRestart` (existing, unchanged — regression, not new).
(Previously: only the `err != nil` branch of the two guards had a scenario; the `status != 0,
err == nil` branch, which answered success, had none.)

Requirement "Turning the page cache off clears what it cached" is untouched by this change and is
not repeated here.
### Requirement: Turning the page cache off clears what it cached

The tool SHALL leave nothing behind that is served from a cache that has been turned off.

#### Scenario: Varnish is turned off

- **WHEN** the page cache is disabled
- **THEN** what it generated is cleared, because pages cached before the change would be served
  until they expired

#### Scenario: The disable step fails before the cleanup runs

- **GIVEN** `varnish-off` is run
- **WHEN** `cache:disable` exits non-zero with no Go-level error
- **THEN** neither the purge nor `cache:clean` runs, so nothing already cached is cleared
- **AND** the failure is reported — the container's own exit code, plus an `ok: false` envelope
  under `--json` naming the disable step — rather than answered as success

Test: `TestAFailedCacheToggleStopsBeforeThePurge` (same test as above — it proves both the guard's
generic behavior and this requirement's guarantee).
(Previously: the guarantee held only when `cache:disable` returned a Go-level error; a non-zero
status with no error skipped the cleanup and reported success.)
