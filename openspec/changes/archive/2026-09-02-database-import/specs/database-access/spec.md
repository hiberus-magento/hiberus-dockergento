# Database access

## ADDED Requirements

### Requirement: Importing a dump

The tool SHALL replace the contents of the project's database with a dump, and SHALL do what has
to happen around that.

#### Scenario: The dump goes in

- **WHEN** a dump is imported
- **THEN** its contents are in the database afterwards

#### Scenario: Clauses naming another user

- **WHEN** the DEFINER clauses are asked to be removed
- **THEN** the objects that named another user belong to whoever imported them

#### Scenario: Asking for that, in the documented order

- **WHEN** the option that removes them is given before the file
- **THEN** the import happens

#### Scenario: The record of the data

- **WHEN** a dump is imported
- **THEN** any record of the data having been anonymised is cleared

#### Scenario: Anonymising

- **WHEN** anonymisation is asked for
- **THEN** it runs after the import, and is recorded only if it succeeded

#### Scenario: A file that is not there

- **WHEN** the dump does not exist
- **THEN** it is said, with the same exit code as before

#### Scenario: The store still pointing somewhere else

- **WHEN** the import succeeds and the store cannot be configured
- **THEN** it is reported as such, saying the data is in and the addresses are not

### Requirement: Saying that something is happening

The tool SHALL say when a step will take a while, and SHALL only animate where animation belongs.

#### Scenario: A terminal

- **WHEN** the output is a terminal, in text, with colour allowed and nobody asking for silence
- **THEN** the step is animated

#### Scenario: Anywhere else

- **WHEN** any of those is not true
- **THEN** the step is announced once and its outcome once

### Requirement: Asking for what cannot be worked out

The tool SHALL ask when it needs a value it cannot derive, and SHALL never hang when there is
nobody to ask.

#### Scenario: Nobody to ask

- **WHEN** there is no terminal, or non-interactive mode is on
- **THEN** the suggestion is used

#### Scenario: Nobody to ask and nothing to guess

- **WHEN** there is no suggestion either
- **THEN** it fails with a message that says what to pass instead
