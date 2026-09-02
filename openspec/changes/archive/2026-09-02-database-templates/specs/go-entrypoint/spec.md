# Go entry point

## ADDED Requirements

### Requirement: A command whose subcommands are ported one family at a time

The tool SHALL let a command be answered by either implementation depending on its subcommand,
and SHALL answer identically whichever one runs.

#### Scenario: A ported subcommand

- **WHEN** a subcommand that has been ported is asked for
- **THEN** it is answered by the ported implementation, with the same documents, tables, refusals
  and exit codes as before

#### Scenario: One that has not

- **WHEN** a subcommand that has not been ported is asked for
- **THEN** it reaches the shell implementation unchanged
