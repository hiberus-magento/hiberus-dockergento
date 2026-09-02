# web-interface Specification

## Purpose
TBD - created by archiving change web-dashboard. Update Purpose after archive.
## Requirements
### Requirement: Bringing the web interface up

The tool SHALL serve a web interface on this machine — `web`, beside `tui` for the terminal one —
brought up and taken down like the proxy rather than held open in a terminal.

#### Scenario: Starting it

- **WHEN** the web interface is brought up
- **THEN** it prints the address to open, and it is already answering

#### Scenario: Asking about it

- **WHEN** its status is asked for
- **THEN** it reports whether it is running, and where

#### Scenario: Starting it twice

- **WHEN** it is brought up while already running
- **THEN** the one already running is reported, and no second one is started

#### Scenario: Stopping it

- **WHEN** it is taken down
- **THEN** it stops and its port is free

#### Scenario: Stopping what is not running

- **WHEN** it is taken down and it was not running
- **THEN** that is not a failure

#### Scenario: A port that is taken

- **WHEN** the port it would listen on is not free
- **THEN** it refuses, saying so, and suggests another

### Requirement: The same answers through a different door

The web interface SHALL answer with the same documents the command line prints.

#### Scenario: The environments

- **WHEN** the environments are asked for over HTTP
- **THEN** the document is the one `list --json` prints

#### Scenario: Credentials

- **WHEN** a project is described over HTTP
- **THEN** the credentials are absent unless they were asked for

### Requirement: Who may talk to it

The web interface SHALL only answer requests from this machine, and only with the token it printed.

#### Scenario: No token

- **WHEN** a request arrives without the token, or with the wrong one
- **THEN** it is refused

#### Scenario: A name that is not loopback

- **WHEN** a request arrives with a Host that is not loopback
- **THEN** it is refused, because a name on the internet can be pointed at this machine

#### Scenario: Where it listens

- **WHEN** the server is running
- **THEN** it is bound to loopback and to nothing else

### Requirement: The tool's own flags

The tool SHALL accept its own flags before the command name as well as after it, as the shell
implementation does.

#### Scenario: Before the command

- **WHEN** a global flag is given before the command name
- **THEN** it is honoured and the command still runs the ported implementation

#### Scenario: A flag meant for a child process

- **WHEN** a flag follows the name of a command whose output is a child process's
- **THEN** it belongs to that child

#### Scenario: A format already decided

- **WHEN** the command is reached through the shell entry point, which parsed the flags itself
- **THEN** the format it decided is honoured

