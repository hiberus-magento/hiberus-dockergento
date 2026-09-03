# Environment cleanup

## ADDED Requirements

### Requirement: The report is a document a program can read

The tool SHALL answer with lists, not with one entry holding everything.

#### Scenario: Several of something

- **WHEN** more than one environment, volume or registration is found
- **THEN** each is its own entry, with its own reason where it has one

#### Scenario: The same order everywhere

- **WHEN** the report is produced on machines with different language settings
- **THEN** the order is the same
