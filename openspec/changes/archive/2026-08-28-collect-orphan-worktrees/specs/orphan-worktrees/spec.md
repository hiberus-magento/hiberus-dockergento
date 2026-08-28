# Orphan branch environments

## ADDED Requirements

### Requirement: Collecting branch environments whose worktree is gone

The tool SHALL report registered branch environments whose directory no longer exists, and SHALL
remove them only when collection is asked for.

#### Scenario: Listing

- **WHEN** `hm clean` runs and a registered branch environment has no directory
- **THEN** it is listed, with the path it used to be at

#### Scenario: One that is still there

- **WHEN** a registered branch environment still has its directory
- **THEN** it is not listed, whatever state its containers are in

#### Scenario: Looking changes nothing

- **WHEN** `hm clean` runs without `--force`
- **THEN** the registration is still there afterwards

#### Scenario: Collecting

- **WHEN** `hm clean --force` runs
- **THEN** the environment's containers and volumes are removed by name, and its registration and
  overlay are deleted

#### Scenario: Nothing else is taken with it

- **WHEN** a branch environment is collected
- **THEN** the database snapshots of that project are not touched

### Requirement: Saying where the way out is

The tool SHALL point at the command that collects them when it reports a branch environment as
missing.

#### Scenario: Listing branch environments

- **WHEN** `hm worktree list` shows an environment whose worktree is gone
- **THEN** it names the command that collects it
