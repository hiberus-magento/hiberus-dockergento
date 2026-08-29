# Worktree dependencies

## MODIFIED Requirements

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
