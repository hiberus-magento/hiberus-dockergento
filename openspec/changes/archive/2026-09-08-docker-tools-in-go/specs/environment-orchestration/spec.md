# Delta for environment-orchestration

## ADDED Requirements

### Requirement: Passing an arbitrary Compose subcommand through

The tool SHALL run a Compose subcommand it does not implement itself, against exactly the files
and environment the current project resolves to, and SHALL hand back that subcommand's own exit
code.

#### Scenario: The project's own configuration

- **GIVEN** a resolved project
- **WHEN** a Compose subcommand is run through the tool
- **THEN** it runs against the same compose files and environment the project resolves to (base,
  platform overlay, worktree overlay in a worktree, proxy overlay when the project has one)

#### Scenario: The subcommand's own exit code

- **WHEN** the Compose subcommand finishes
- **THEN** the tool exits with that same code, unchanged

#### Scenario: No project to resolve

- **WHEN** the directory is not a project
- **THEN** the passthrough is refused before any subcommand runs

#### Scenario: The same terminal

- **WHEN** the subcommand reads or writes
- **THEN** it uses the same terminal it would use if Compose were invoked directly

#### Scenario: No Compose binary installed

- **WHEN** no compose binary is installed
- **THEN** the command is refused with a clear message and the Docker-failure exit code, before
  anything runs

#### Scenario: What the runner is asked to run

- **GIVEN** a fake compose runner substituted for the real one
- **WHEN** a Compose subcommand is run through the tool
- **THEN** the fake records the resolved compose file list, the project directory, and the
  environment it was given, alongside the arguments

### Requirement: Stopping every running container through the engine

The tool SHALL stop the containers it finds running by asking the container engine to stop
exactly those ids, and SHALL report back what actually stopped.

#### Scenario: The ids asked for are the ids found running

- **GIVEN** a fake container engine substituted for the real one
- **WHEN** a machine-wide stop is asked for and confirmed
- **THEN** the fake records one request whose ids are exactly the containers found running at
  that moment, and the ids it reports back as stopped are what the requirement's count reflects

## MODIFIED Requirements

### Requirement: Starting an environment

The tool SHALL bring the environment up, and SHALL do the things around it that nobody should have
to remember.
(Previously: stopping other machine-wide containers for `-s` shelled out to the shell
implementation; the tool now performs that stop in-process.)

#### Scenario: A project that needs the proxy

- **WHEN** the project is routed through the global proxy and it is not running
- **THEN** the proxy is started first, and left running afterwards

#### Scenario: Something else holding the proxy's ports

- **WHEN** another environment holds port 80 or 443
- **THEN** the start is refused, naming that environment, with the refusal exit code

#### Scenario: Dependencies bound from the host

- **WHEN** the dependencies of a macOS environment come from a bind mount
- **THEN** the start is refused, naming the mount and what to do about it

#### Scenario: Starting one service

- **WHEN** a service is named
- **THEN** only what was asked for is started, and the whole-environment checks do not run

#### Scenario: Stopping the rest before starting

- **WHEN** starting is asked to stop what else is running first
- **THEN** the same confirmation and messages that stopping everything asks for run before the
  environment comes up, with no shell process involved
- **AND** declining the question is not a failure: nothing is stopped and the environment still
  comes up, exactly as the shell implementation did

### Requirement: One implementation of starting

The tool SHALL start environments through the same implementation on every platform, and SHALL
hand back only the steps that platform needs afterwards.
(Previously: stopping the rest before starting reached the shell implementation on every
platform; it is now the same in-process call on every platform, same as the rest of starting.)

#### Scenario: What Linux needs afterwards

- **WHEN** an environment is started on Linux
- **THEN** the platform's own steps run after it is up

#### Scenario: What macOS needs afterwards

- **WHEN** an environment is started on macOS
- **THEN** there is nothing to do, and nothing is started to find that out

#### Scenario: A step that fails

- **WHEN** the steps after starting fail
- **THEN** the failure is reported and the environment is left running

#### Scenario: One copy of those steps

- **WHEN** either implementation brings an environment up
- **THEN** both run the same steps, from the same place

#### Scenario: One copy of stopping the rest, too

- **WHEN** stopping the rest before starting runs on either platform
- **THEN** it is the same in-process call, not a platform-specific shell invocation
