# Dockergento skills

## ADDED Requirements

### Requirement: Skills bundled with the tool

The tool SHALL carry the skills that describe its own commands, one per area of work, in a
`skills/` directory laid out as the AI tooling already expects.

#### Scenario: The skills are present

- **WHEN** the repository is inspected
- **THEN** `skills/` contains a directory per skill, each with a `SKILL.md` carrying a name and a
  description in its front matter

#### Scenario: They are named so they can be filtered

- **WHEN** a person installs only the `dockergento` skill type
- **THEN** every bundled skill matches, because each one is named `dockergento-<area>`

#### Scenario: They cover the areas of work, not one command each

- **WHEN** the set is inspected
- **THEN** there is a skill for the environment and its lifecycle, one for the database, one for
  debugging, and one for branch environments and agent tooling

### Requirement: The skills cannot describe commands that do not exist

The test suite SHALL check every command mentioned in every bundled skill against the tool's own
command declarations, and SHALL fail when a skill names a command, an option or a service that
does not exist.

#### Scenario: A command that was removed or renamed

- **WHEN** a skill mentions `hm <name>` and no `console/commands/<name>.sh` exists
- **THEN** the test fails, naming the skill and the command

#### Scenario: An option that was never declared

- **WHEN** a skill uses an option that is not declared for that command in
  `command_descriptions.json` and is not a global flag
- **THEN** the test fails, naming the option

#### Scenario: A service that is not in the stack

- **WHEN** a skill names a service that is not a service of the compose template
- **THEN** the test fails

#### Scenario: The shell command takes no command

- **WHEN** a skill writes `hm bash <something>` other than `-r`
- **THEN** the test fails, because that argument is dropped and an interactive shell opens instead

### Requirement: Installing the bundled skills

The tool SHALL install its own skills from the installed copy of itself rather than downloading
them, so that the skills a person has describe the version they are running.

#### Scenario: Pulling the AI tools

- **WHEN** the user runs `hm ai-pull`
- **THEN** the bundled skills are installed from the installed tool's own directory, with no
  network request for them, and the configured remote repositories are downloaded as before

#### Scenario: A custom skill of the same name

- **WHEN** a skill directory of the same name exists and was not installed by the tool
- **THEN** it is preserved unless `--force` is given, exactly as for a downloaded skill

#### Scenario: The version is the installed one

- **WHEN** the installed tool is older than the branch the skills are developed on
- **THEN** the skills installed are the ones that came with it, not the ones on the branch
