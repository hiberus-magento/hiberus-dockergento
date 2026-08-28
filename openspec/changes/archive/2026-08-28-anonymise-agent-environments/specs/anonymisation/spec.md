# Anonymisation state

## ADDED Requirements

### Requirement: Recording whether the database has been anonymised

The tool SHALL record a successful anonymisation outside the checkout, and SHALL clear that record
whenever the contents of the database are replaced.

#### Scenario: After anonymising

- **WHEN** `hm masquerade` completes successfully
- **THEN** the project is recorded as anonymised, with the date

#### Scenario: After restoring a snapshot

- **WHEN** a snapshot is restored, a template is cloned, a dump is imported or a database is
  transferred
- **THEN** the record is cleared, because what those brought in has not been anonymised

#### Scenario: A project nobody has anonymised

- **WHEN** the state is asked for and nothing was ever recorded
- **THEN** the answer is unknown, and unknown is never treated as anonymised

#### Scenario: Reported by describe

- **WHEN** `hm describe --json` runs
- **THEN** the anonymisation state is part of what it reports

### Requirement: Anonymising an agent's environment by default

The tool SHALL anonymise the database of a branch environment created for an agent, unless told
not to.

#### Scenario: Creating an agent environment

- **WHEN** `hm worktree add <branch> --profile=agent` runs and the database was cloned
- **THEN** the database is anonymised before the environment is handed over, and the result is
  recorded

#### Scenario: Opting out

- **WHEN** `--no-anonymise` is given
- **THEN** the database is left as it is, and the state stays unknown

#### Scenario: Other profiles

- **WHEN** the profile is `lite` or `full`
- **THEN** nothing is anonymised automatically, because that environment is usually a person's own

#### Scenario: Anonymisation fails

- **WHEN** the anonymisation cannot run
- **THEN** the environment is still created, and the failure is reported rather than swallowed

### Requirement: Telling the agent what it is looking at

The tool SHALL state the anonymisation status in the generated agent context, and SHALL warn when
an agent's environment holds data nobody has anonymised.

#### Scenario: The context says it

- **WHEN** the agent context is generated for a project whose database is not anonymised
- **THEN** the block says so and tells the reader to treat every row as real personal data

#### Scenario: The context says when it is safe

- **WHEN** the database was anonymised
- **THEN** the block says so, with the date

#### Scenario: The doctor notices

- **WHEN** an environment created for an agent has a database that was never anonymised
- **THEN** `hm doctor` reports it with the command that fixes it

### Requirement: Anonymising without a terminal

The tool SHALL be able to anonymise from a script, from CI and from an agent.

#### Scenario: No terminal attached

- **WHEN** `hm masquerade` runs with stdin that is not a terminal
- **THEN** it runs, rather than failing because Docker was asked for a terminal that does not
  exist
