# agent-context Specification

## Purpose
TBD - created by archiving change generate-agent-context. Update Purpose after archive.
## Requirements
### Requirement: Writing the project's agent context

The tool SHALL write, into the project, the facts an agent needs before it starts, taken from the
resolved configuration rather than from the files it was configured with.

#### Scenario: Generating it for the first time

- **WHEN** the user runs `hm ai-context` in a project with no `AGENTS.md`
- **THEN** `AGENTS.md` is created with a delimited block containing the project's versions, its
  URLs including the admin's real front name, how to run commands, the exit-code contract, and
  what not to read or run

#### Scenario: A file somebody wrote themselves

- **WHEN** `AGENTS.md` already exists with content of its own
- **THEN** only the delimited block is replaced, and everything outside it is left exactly as it
  was; a file with no block gets one appended

#### Scenario: An existing CLAUDE.md

- **WHEN** the project already has a `CLAUDE.md`
- **THEN** it is not modified, and the line that imports the context is printed for the person to
  place

#### Scenario: No CLAUDE.md yet

- **WHEN** the project has no `CLAUDE.md`
- **THEN** one is created that imports `AGENTS.md` and nothing else

#### Scenario: No secrets are written

- **WHEN** the context is generated for a project whose database password is known to the tool
- **THEN** the password appears nowhere in the generated files

#### Scenario: The MCP server is wired up

- **WHEN** the context is generated
- **THEN** `.mcp.json` contains an entry for this tool's server, merged with any entries already
  there

### Requirement: Declaring what an agent should not read

The tool SHALL declare, in one place, the paths an agent should not read, each with the reason,
and SHALL use that declaration both in the generated context and in the permissions it produces.

#### Scenario: The list is declared once

- **WHEN** the exclusions are inspected
- **THEN** each entry has a path and a reason, and the file is the only place they are listed

#### Scenario: Secrets and customer data are in it

- **WHEN** the list is inspected
- **THEN** it contains at least `app/etc/env.php`, `var/log`, `pub/media/customer`, `vendor`,
  `generated` and `var/cache`

#### Scenario: Permissions refuse them

- **WHEN** the user runs `hm permissions`
- **THEN** the configuration contains a `deny` rule for reading each excluded path

#### Scenario: The context explains them

- **WHEN** the context is generated
- **THEN** the block lists the excluded paths with their reasons, so an agent whose tooling
  enforces nothing still knows

### Requirement: Noticing when the context is stale

The tool SHALL record what the generated context was built from, and SHALL report it as a problem
when the project no longer matches.

#### Scenario: The project changed

- **WHEN** the PHP or database version changes after the context was generated
- **THEN** `hm doctor` reports the context as out of date and names the command that refreshes it

#### Scenario: The context is current

- **WHEN** nothing relevant has changed
- **THEN** the check passes

#### Scenario: No context at all

- **WHEN** the project has no generated context
- **THEN** the check says so as a suggestion rather than a failure, because not every project
  wants one

