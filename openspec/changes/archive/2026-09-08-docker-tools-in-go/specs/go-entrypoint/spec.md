# Delta for go-entrypoint

## ADDED Requirements

### Requirement: A ported command's shell entry point still answers by delegating

When a command is fully answered by the tool, its former shell implementation SHALL delegate to
the tool rather than duplicate or lose the behavior, so invoking it through the shell entry point
reaches the same answer.

#### Scenario: Running a ported docker tool through the shell entry point

- **GIVEN** `docker-stop-all` or `docker-compose` is answered by the tool
- **WHEN** either is invoked through the shell entry point instead of the tool directly
- **THEN** the shell script delegates to the tool, and the answer is the same

#### Scenario: The question still reaches the same terminal

- **WHEN** `docker-stop-all`'s confirmation is asked through the delegated shell entry point
- **THEN** the terminal that asks and reads the answer is the same one the tool would use directly

## MODIFIED Requirements

### Requirement: A ported command's engine interaction is provable without Docker

For a command listed in `internal/cli/run.go`'s `Run()` switch, the repository SHALL carry a Go
test that verifies what its handler asks of the engine, through a fake substituted for the real
one, requiring no Docker daemon, no network access, and no project beyond what the test itself
builds.
(Previously: covered `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`,
`purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`, and
`set-host`. Adds `docker-stop-all` and `docker-compose`.)

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

#### Scenario: Reporting what is installed

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `version` runs, with no project resolved
- **THEN** the fake records exactly one call, asking what is installed and what tooling is present
- **AND** in JSON, the document carries the installation fields (`version`, `tag`, `commits_ahead`,
  `commit`, `branch`, `detached`, `dirty`, `path`), the docker tooling fields (`docker.version`,
  `docker.compose`, `docker.compose_command`), and `binary` from the binary's own build
  information, independent of the fake
- **AND** in text, an empty tag or commit prints "unknown", and an empty docker or compose value
  prints "not available"

#### Scenario: Pointing a domain at this machine

- **WHEN** `set-host <domain>` runs against a resolved project
- **THEN** the fake records a call asking to point that domain here, with the database flag on
- **AND** `--no-database` records the same call with the database flag off
- **AND** in JSON, the document carries `{"domain": <domain>, "database": <flag>}`
- **AND** a refusal returned by the engine is reported with its own exit code, message and hint,
  rather than as a generic Docker failure

#### Scenario: Removing a domain without resolving a project

- **WHEN** `set-host --remove <domain>` runs
- **THEN** no project is resolved first, and the fake records a call asking only to remove that
  domain
- **AND** in JSON, the document carries `{"removed": <domain>}`

#### Scenario: Asking to stop everything

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `docker-stop-all` runs against a resolved project
- **THEN** the fake records exactly one request to stop everything for the resolved directory,
  carrying whether the run is interactive
- **AND** whatever the fake answers back — a decline, a refusal, or a stop — is reported without
  the handler making any other request

#### Scenario: Passing a Compose subcommand through

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `docker-compose <args>` runs against a resolved project
- **THEN** the fake records exactly one request to run Compose for the resolved directory, with
  the arguments passed through verbatim
- **AND** the command exits with the code the fake reports back
