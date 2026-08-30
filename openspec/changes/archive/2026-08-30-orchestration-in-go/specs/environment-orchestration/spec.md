# Environment orchestration

## ADDED Requirements

### Requirement: Driving Compose

The tool SHALL bring environments up and down through the Compose engine in its own process, and
what it creates SHALL be indistinguishable from what the `docker compose` command creates.

#### Scenario: The command sees what the tool created

- **WHEN** an environment is brought up by the tool
- **THEN** `docker compose ps` lists its services

#### Scenario: Neither recreates the other's containers

- **WHEN** the tool and the command are used one after the other, in either order
- **THEN** the configuration hash is the same and nothing is recreated

#### Scenario: A change is still applied

- **WHEN** the compose configuration or a service's image has changed
- **THEN** the affected containers are replaced

### Requirement: Resolving what a project is built from

The tool SHALL load every compose file a project is built from, and SHALL report only the ones the
project declares.

#### Scenario: A project routed through the proxy

- **WHEN** the project has a proxy overlay
- **THEN** it is loaded, so the project does not read as publishing ports it does not publish

#### Scenario: A branch environment

- **WHEN** the project is a worktree with an environment of its own
- **THEN** its overlay is loaded and the proxy's is not

#### Scenario: What a description reports

- **WHEN** a project's configuration is described
- **THEN** the files reported are the two the project declares, not the overlays

### Requirement: Starting an environment

The tool SHALL bring the environment up, and SHALL do the things around it that nobody should have
to remember.

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

### Requirement: Stopping an environment

The tool SHALL stop without removing, and SHALL only take a copy of the database when asked.

#### Scenario: An everyday stop

- **WHEN** `stop` runs with nothing else asked for
- **THEN** the containers are stopped and still there, and no copy is taken

#### Scenario: A copy that could not be taken

- **WHEN** a copy was asked for and it failed
- **THEN** the environment is left running and the failure is reported

### Requirement: Restarting an environment

The tool SHALL apply a changed configuration when restarting.

#### Scenario: The configuration changed

- **WHEN** the compose file has changed and `restart` runs
- **THEN** the change is running afterwards

### Requirement: Reading the logs

The tool SHALL write what the services are saying, exactly as the shell implementation did.

#### Scenario: The same output

- **WHEN** logs are read, of everything or of one service
- **THEN** the prefixes, the colours and the lines are the shell implementation's

#### Scenario: A service this project does not have

- **WHEN** a service is named that the project does not define
- **THEN** it is refused with the service exit code, saying which services there are

#### Scenario: An option that takes a value

- **WHEN** an option that needs a value is given without one
- **THEN** it is a usage error, and the next word is never read as a service name

### Requirement: Running something inside a service

The tool SHALL run a command in the php container and SHALL return that command's own exit code.

#### Scenario: The command's exit code

- **WHEN** the command run inside exits with a code
- **THEN** the tool exits with the same one

#### Scenario: As root

- **WHEN** root is asked for
- **THEN** the command runs as root

#### Scenario: Nothing to run

- **WHEN** no command is given
- **THEN** it is a usage error
