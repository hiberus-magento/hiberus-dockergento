# Database access

## ADDED Requirements

### Requirement: Asking the database

The tool SHALL run a statement against the project's database and answer with what the database
said, exactly as the shell implementation did.

#### Scenario: A statement

- **WHEN** a statement is run
- **THEN** the output and the exit code are the shell implementation's

#### Scenario: A statement with quotes in it

- **WHEN** the statement contains quotes and backticks
- **THEN** it reaches the database unchanged

#### Scenario: A statement that fails

- **WHEN** the database rejects the statement
- **THEN** the client's own exit code and message come back

#### Scenario: The database is not running

- **WHEN** the database container is not running
- **THEN** it is refused with the service exit code, saying how to start it

#### Scenario: Another project's database

- **WHEN** more than one environment is up
- **THEN** the statement reaches this project's database and no other

### Requirement: The client and the dump

The tool SHALL open a session when there is a terminal and nothing else was asked for, and SHALL
import what is on its input when there is not.

#### Scenario: A dump on the input

- **WHEN** there is no terminal and no option was given
- **THEN** what is on the input is imported

#### Scenario: A statement without a terminal

- **WHEN** a statement is asked for and there is no terminal
- **THEN** the statement is run, not treated as a dump

#### Scenario: An import that does more than import

- **WHEN** an import is asked for by name
- **THEN** it is handled by the implementation that also cleans, anonymises and configures Magento
