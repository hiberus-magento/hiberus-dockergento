# Version management

## ADDED Requirements

### Requirement: The version says what a bug report needs

The tool SHALL report which build is installed, not the nearest tag.

#### Scenario: A checkout past its last tag

- **WHEN** the installed checkout is some commits past a tag
- **THEN** the version says the tag and how many commits, and names the commit

#### Scenario: Uncommitted changes in the installation

- **WHEN** the installation directory has tracked changes
- **THEN** it is reported, and untracked files are not counted

#### Scenario: Docker is not running

- **WHEN** the daemon cannot be reached
- **THEN** it is reported as not available rather than failing the command

#### Scenario: The binary
- **WHEN** the version is reported by the ported half
- **THEN** it also says which build of the binary is running
