# ai-tooling-inventory Specification

## Purpose
TBD - created by archiving change track-ai-tooling-versions. Update Purpose after archive.
## Requirements
### Requirement: Recording where each resource came from

The tool SHALL record, for every skill or agent it installs, a content digest, the repository it
came from, and the branch or tool version it corresponds to.

#### Scenario: Installing from the bundled skills

- **WHEN** `hm ai-pull` installs the skills that came with the tool
- **THEN** each one is recorded with the repository name, the tool's version, the date, and a
  digest of its contents

#### Scenario: The digest works on this machine

- **WHEN** a resource is installed on macOS, where `sha256sum` does not exist
- **THEN** a real digest is recorded, not an empty string

#### Scenario: A skill is a directory

- **WHEN** the digest of a skill is taken
- **THEN** it covers every file in it, and adding, renaming or removing one changes the result

### Requirement: Reporting what is installed

The tool SHALL report every installed skill and agent with its origin and its state, and SHALL
change nothing.

#### Scenario: Everything as installed

- **WHEN** `hm ai-doctor` runs just after a pull
- **THEN** every resource is reported as current, with the repository it came from

#### Scenario: One that was edited by hand

- **WHEN** an installed skill has been changed since it was installed
- **THEN** it is reported as modified, with the warning that the next pull overwrites it and that
  renaming it is what keeps it

#### Scenario: One that came with an older version of the tool

- **WHEN** the installed copy differs from the one the tool now carries
- **THEN** it is reported as outdated, without any network request

#### Scenario: One nobody installed

- **WHEN** a skill directory is present that the tool never installed
- **THEN** it is reported as custom

#### Scenario: One that is gone

- **WHEN** a tracked resource no longer exists on disk
- **THEN** it is reported as missing

#### Scenario: No configuration yet

- **WHEN** the project has never run `hm ai-init`
- **THEN** the command says so and exits successfully, rather than failing

#### Scenario: As data

- **WHEN** stdout is not a terminal, or `--json` is given
- **THEN** the inventory is printed as JSON with a schema version

