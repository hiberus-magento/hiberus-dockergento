# Magento commands

## ADDED Requirements

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
