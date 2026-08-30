# project-description Specification

## Purpose
TBD - created by archiving change describe-in-go. Update Purpose after archive.
## Requirements
### Requirement: Reading the Compose configuration

The tool SHALL resolve a project's Compose configuration with the library Compose uses, and SHALL
agree with what the `docker compose config` command answers.

#### Scenario: The same answer as the command

- **WHEN** a project's files are resolved both ways
- **THEN** the project name, the services, their images, their published ports and their
  environment are the same

#### Scenario: An overlay for the other platform

- **WHEN** one of the files does not exist on this machine
- **THEN** it is skipped, as the shell implementation skips it

#### Scenario: No files at all

- **WHEN** none of the files exists
- **THEN** it is an error naming the directory, not an empty project

#### Scenario: The order of the services

- **WHEN** the configuration is read repeatedly
- **THEN** the services come out in the same order every time

### Requirement: Describing a project

The tool SHALL report what defines a project, and SHALL answer exactly what the shell
implementation answered.

#### Scenario: The document

- **WHEN** `hm describe --json` runs in a project
- **THEN** the document is identical to the shell implementation's, field for field

#### Scenario: The table

- **WHEN** `hm describe` runs with colour disabled
- **THEN** the text is identical to the shell implementation's

#### Scenario: Credentials

- **WHEN** `--with-secrets` is given
- **THEN** the database credentials are included, and they are absent from every other answer

#### Scenario: Somewhere that is not a project

- **WHEN** the directory has no configuration
- **THEN** it fails with the project exit code and the same message

#### Scenario: The properties a project never set

- **WHEN** a project's file does not mention a property the tool has a default for
- **THEN** the default is used, as the shell implementation does

#### Scenario: A directory that is not a project does not inherit the defaults

- **WHEN** a directory has no properties file of its own
- **THEN** it resolves to no project, rather than to the tool's defaults

