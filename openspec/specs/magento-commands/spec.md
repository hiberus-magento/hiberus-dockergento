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

#### Scenario: A command line with options in it

- **WHEN** a wrapper runs a program with arguments inside the container
- **THEN** the program runs and receives them, rather than being looked for as a file whose name
  contains them

#### Scenario: The program fails

- **WHEN** the program inside the container fails
- **THEN** the exit code is the program's own

### Requirement: Turning the page cache off clears what it cached

The tool SHALL leave nothing behind that is served from a cache that has been turned off.

#### Scenario: Varnish is turned off

- **WHEN** the page cache is disabled
- **THEN** what it generated is cleared, because pages cached before the change would be served
  until they expired
