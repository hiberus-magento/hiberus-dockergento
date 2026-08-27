# worktree-environments Specification

## Purpose
TBD - created by archiving change add-worktree-environments. Update Purpose after archive.
## Requirements
### Requirement: Creating a branch environment

The tool SHALL create, from the main checkout, a git worktree with its own compose project, its
own subdomain and its own data, and SHALL register it outside the checkout so that no tracked
file changes.

#### Scenario: Adding a worktree for a branch

- **WHEN** the user runs `hm worktree add feature-x` from the main checkout
- **THEN** a git worktree for that branch is created, registered under
  `~/.hm/worktrees/<project>/feature-x.json` with its path, profile, branch and compose project
  name, and no file inside the repository is modified

#### Scenario: The project is not routed through the proxy

- **WHEN** the project does not use the global proxy
- **THEN** the tool refuses and names the command that sets it up, because every branch
  environment publishing its own ports is the collision the proxy exists to prevent

#### Scenario: Adding from inside a worktree

- **WHEN** the command is run from a worktree rather than the main checkout
- **THEN** it is refused, naming the main checkout

#### Scenario: A name already registered

- **WHEN** an environment with that name already exists
- **THEN** the tool refuses and says how to reach it, rather than creating a second one

### Requirement: Profiles decide what runs

The tool SHALL support the profiles `lite`, `agent` and `full`, and SHALL express a profile by
removing the services it does not include from the resolved configuration.

#### Scenario: The agent profile

- **WHEN** an environment is created with `--profile=agent`
- **THEN** its resolved configuration contains phpfpm, nginx, db, search and redis, and does not
  contain varnish, hitch, the mail catcher or rabbitmq

#### Scenario: The lite profile

- **WHEN** an environment is created with `--profile=lite`
- **THEN** its resolved configuration contains phpfpm and no other service

#### Scenario: The full profile

- **WHEN** an environment is created with `--profile=full`
- **THEN** its resolved configuration contains the same services as the main environment

#### Scenario: An unknown profile

- **WHEN** a profile that does not exist is given
- **THEN** the tool refuses and lists the three that do

### Requirement: A registered worktree resolves against itself

When the current directory is a worktree with a registered environment, the tool SHALL resolve the
project root, the compose project name and the compose files against that worktree, and SHALL NOT
apply the refusals that protect the main environment.

#### Scenario: Commands run against the branch environment

- **WHEN** a command is run from a registered worktree
- **THEN** the compose project name is `<project>-<name>` and the bind mounts resolve to the
  worktree's own directory

#### Scenario: Lifecycle commands are allowed there

- **WHEN** `hm start`, `hm stop` or `hm down` is run from a registered worktree
- **THEN** it acts on that worktree's environment and is not refused

#### Scenario: An unregistered worktree is unchanged

- **WHEN** a command that recreates or destroys an environment is run from a worktree with no
  registered environment
- **THEN** it is refused exactly as before, naming the main checkout

### Requirement: Dependencies and data are copied, not rebuilt

The tool SHALL give a new branch environment its dependencies without installing them again and
its database without importing a dump.

#### Scenario: Dependencies on a bind-mounted platform

- **WHEN** the environment is created on Linux
- **THEN** `vendor/` and `node_modules/` in the worktree are links to the main checkout's, and
  nothing appears as modified in git

#### Scenario: Dependencies on macOS

- **WHEN** the environment is created on macOS, where the code lives in a named volume
- **THEN** the main environment's code volume is copied into the new environment's

#### Scenario: The database comes from a template

- **WHEN** the project has a database template
- **THEN** it is cloned into the new environment before it starts

#### Scenario: No template exists

- **WHEN** the project has no template
- **THEN** the environment is still created, and the tool says which command would have given it
  data instead of silently sharing the main database

### Requirement: Listing and removing branch environments

The tool SHALL list the branch environments of the current project and SHALL remove one on
request, taking the environment and the worktree with it.

#### Scenario: Listing

- **WHEN** the user runs `hm worktree list`
- **THEN** each environment is shown with its name, branch, profile, state and address

#### Scenario: Listing as data

- **WHEN** stdout is not a terminal, or `--json` is given
- **THEN** the same information is printed as JSON with a schema version

#### Scenario: Removing

- **WHEN** the user runs `hm worktree remove feature-x`
- **THEN** the tool asks for confirmation, destroys the environment with its volumes, removes the
  git worktree, and deletes the registration

#### Scenario: Removing a worktree with uncommitted work

- **WHEN** the worktree has uncommitted changes
- **THEN** the tool refuses unless forced, because the code is the one thing here that cannot be
  rebuilt

