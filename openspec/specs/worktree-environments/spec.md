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

The tool SHALL give a new branch environment its dependencies without installing them again, and
SHALL do so in a way that leaves the autoloader resolving against the worktree.

#### Scenario: Dependencies on a bind-mounted platform

- **WHEN** the environment is created on Linux and the branch's `composer.lock` matches the main
  checkout's
- **THEN** the main checkout's `vendor/` and `node_modules/` are mounted read-only into the
  worktree's containers at their own paths, and nothing is linked or copied

#### Scenario: The autoloader resolves against the worktree

- **WHEN** the resolved configuration of the worktree is inspected
- **THEN** the code mount is intact and the dependency mount sits on top of it, so that a file
  under `vendor/` resolves its base directory to the worktree

#### Scenario: A branch that changes dependencies

- **WHEN** the branch's `composer.lock` differs from the main checkout's
- **THEN** nothing is mounted, the environment is still created, and the tool says to run
  `hm composer install` in the worktree

#### Scenario: The decision is recorded

- **WHEN** a worktree is registered
- **THEN** the registration says whether its dependencies are shared or its own

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

### Requirement: Not writing to somebody else's dependencies

The tool SHALL refuse to run Composer against dependencies shared with the main checkout, and
SHALL say why.

#### Scenario: Composer in a worktree with shared dependencies

- **WHEN** `hm composer install` runs in a worktree whose dependencies are shared
- **THEN** it is refused with an explanation and the way to give that worktree its own, rather
  than failing on a read-only filesystem

#### Scenario: Composer in a worktree with its own dependencies

- **WHEN** the worktree's dependencies are its own
- **THEN** Composer runs normally

### Requirement: What a listing reports

The tool SHALL report the state of a branch environment from what is actually there, not from what
was recorded.

#### Scenario: Running or not

- **WHEN** branch environments are listed
- **THEN** each one's state comes from its containers

#### Scenario: A directory somebody deleted

- **WHEN** the worktree's directory is gone
- **THEN** it is reported as missing, and the command that collects it is named

