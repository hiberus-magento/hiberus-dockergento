# Delta for magento-commands

## MODIFIED Requirements

### Requirement: A wrapper runs what it says it runs

The tool SHALL pass a command and its arguments to the container as a command and its arguments.
A wrapper that also owns a JSON success envelope SHALL NOT emit that envelope, and SHALL NOT print
its success line, when the program it ran inside the container failed.
(Previously: covered `varnish-on`/`varnish-off`'s VCL edit and cache toggle, and `debug-off`'s
xdebug ini edit. This adds `debug-on`'s own three ini edits — the two `xdebug.ini` host patches and
the `docker-php-ext-xdebug.ini` uncomment — the same failure shape reused a third time, against the
sibling command that turns xdebug on rather than off.)

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

#### Scenario: An xdebug-enabling edit fails inside the php container

- **GIVEN** `debug-on` is run
- **WHEN** any of its three `sed` edits — the two host patches against `xdebug.ini`, or the
  uncomment against `docker-php-ext-xdebug.ini` — exits non-zero inside the `phpfpm` container with
  no Go-level error
- **THEN** the command exits with that same status, not a mapped code
- **AND** stdout carries no banner and no `--json` success document; stderr carries the
  container's own output plus one readable line naming the edit as the step that failed
- **AND** under `--json`, stderr carries exactly one envelope with `ok: false` and `error.code`
  equal to that status
- **AND** neither the restart nor either `Capture` call (`php -v`, `php -i`) is reached — whichever
  edit fails first, everything after it in the shell's own sequence is skipped

Test: `TestAFailedDebugOnEditReportsTheContainersStatus` (new; shared with `go-entrypoint`'s
scenario of the same fact, proved once).

#### Scenario: A Go-level error still stops before the restart

- **WHEN** the VCL edit fails with an error rather than a non-zero status
- **THEN** the restart is not reached and the failure is reported with the mapped Docker exit code

Test: `TestAFailedEditStopsBeforeTheRestart` (existing, unchanged — regression, not new).
(Previously: only the `err != nil` branch of the two guards had a scenario; the `status != 0,
err == nil` branch, which answered success, had none.)

Requirement "Turning the page cache off clears what it cached" is untouched by this change and is
not repeated here.
