# Delta for magento-commands

## MODIFIED Requirements

### Requirement: A wrapper runs what it says it runs

The tool SHALL pass a command and its arguments to the container as a command and its arguments.
A wrapper that also owns a JSON success envelope SHALL NOT emit that envelope, and SHALL NOT print
its success line, when the program it ran inside the container failed.

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

#### Scenario: A Go-level error still stops before the restart

- **WHEN** the VCL edit fails with an error rather than a non-zero status
- **THEN** the restart is not reached and the failure is reported with the mapped Docker exit code

Test: `TestAFailedEditStopsBeforeTheRestart` (existing, unchanged — regression, not new).
(Previously: only the `err != nil` branch of the two guards had a scenario; the `status != 0,
err == nil` branch, which answered success, had none.)

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
