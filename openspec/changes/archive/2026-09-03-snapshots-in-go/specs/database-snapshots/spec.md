# Database snapshots

## ADDED Requirements

### Requirement: A copy is usable or it is not there

The tool SHALL NOT leave an incomplete copy where a complete one is expected.

#### Scenario: The copy is interrupted

- **WHEN** taking a copy fails or is interrupted
- **THEN** no snapshot by that name is listed, and nothing half-written is left in its place

### Requirement: A copy holds the data and nothing else

The tool SHALL NOT write anything into a copy but what the database dumped.

#### Scenario: The dumper warns about something

- **WHEN** the command that writes the copy also writes a warning about itself
- **THEN** the warning is not in the copy

#### Scenario: A database large enough to take minutes

- **WHEN** copying takes longer than a query would
- **THEN** it is not cut off

### Requirement: Restoring replaces rather than merges

The tool SHALL empty the database before restoring a copy into it.

#### Scenario: Something was created after the copy was taken

- **WHEN** a copy is restored over a database that has changed since
- **THEN** what was created afterwards is gone, and the database is what the copy holds

#### Scenario: What the copy holds was never anonymised here

- **WHEN** a copy is restored
- **THEN** the record of the data having been anonymised is cleared

### Requirement: Destroying is confirmed by naming what is destroyed

The tool SHALL require the name of what is at stake, not a letter.

#### Scenario: Restoring

- **WHEN** a restore is confirmed with anything other than the project's name
- **THEN** nothing is restored

#### Scenario: Clearing every project

- **WHEN** clearing the copies of every project on the machine is confirmed with one project's
  name
- **THEN** nothing is deleted
