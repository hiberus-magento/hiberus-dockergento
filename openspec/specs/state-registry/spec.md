# state-registry Specification

## Purpose
TBD - created by archiving change state-registry. Update Purpose after archive.
## Requirements
### Requirement: What the registry holds

The tool SHALL record what somebody decided about the projects on this machine, and SHALL NOT
record what exists.

#### Scenario: A branch environment

- **WHEN** a branch environment is registered
- **THEN** its name, path, branch, profile, address and whether it shares dependencies are recorded

#### Scenario: A worktree with no registration

- **WHEN** a worktree that was never registered is asked about
- **THEN** the answer is that there is none, and that is not an error

#### Scenario: Registering the same one again

- **WHEN** a branch environment already recorded is registered again
- **THEN** it is updated, not duplicated

#### Scenario: Two environments with the same name

- **WHEN** two branch environments would answer to the same compose project name
- **THEN** the second is refused

#### Scenario: What exists is not recorded

- **WHEN** the environments on the machine are listed
- **THEN** they come from the container labels, not from the registry

### Requirement: Both topologies

The registry SHALL model the classic and the orchestrated topologies from the outset.

#### Scenario: A classic project

- **WHEN** a project is classic
- **THEN** it has no allocations

#### Scenario: An orchestrated project

- **WHEN** a worktree of an orchestrated project is given its resources
- **THEN** it receives a schema, an index prefix, three Redis databases and a vhost

### Requirement: Handing out slots

The tool SHALL give each worktree of a project resources that no other worktree of that project
has, even when several are created at once.

#### Scenario: Several at once

- **WHEN** several worktrees are allocated concurrently
- **THEN** each receives a different slot

#### Scenario: Asking twice

- **WHEN** a worktree that already has an allocation asks again
- **THEN** it receives the one it has

#### Scenario: A slot that was freed

- **WHEN** a worktree is removed and another is created
- **THEN** the freed slot is used again

#### Scenario: Removing a worktree

- **WHEN** a worktree is removed
- **THEN** its allocation goes with it

#### Scenario: Running out

- **WHEN** a project has as many worktrees as the shared Redis has room for
- **THEN** the next one is refused with a reason

### Requirement: The state of the data

The registry SHALL record whether an environment's data has been anonymised, per environment.

#### Scenario: Nobody has said

- **WHEN** nothing was recorded
- **THEN** the answer is unknown, which is never treated as safe

#### Scenario: A branch does not inherit

- **WHEN** a project's data was anonymised and a branch environment's was not
- **THEN** the branch answers unknown

#### Scenario: Data that was replaced

- **WHEN** the record is cleared
- **THEN** the answer is unknown again

### Requirement: What was already written down

The tool SHALL bring across the state the shell implementation recorded, without losing any of it.

#### Scenario: Importing

- **WHEN** the registry is opened on a machine with existing registrations
- **THEN** they are brought in with everything they recorded

#### Scenario: Importing again

- **WHEN** the import runs a second time
- **THEN** nothing is duplicated

#### Scenario: A record that cannot be read

- **WHEN** one registration is unreadable
- **THEN** the others are still brought in

### Requirement: One registry, read by everything

The tool SHALL keep one record of what was decided about this machine, and every entry point
SHALL read it.

#### Scenario: The command and the diagnostic

- **WHEN** a branch environment is registered and the diagnostic is asked what the registry holds
- **THEN** it reports that environment

#### Scenario: The entry point people type

- **WHEN** a registry command is run through the shell entry point
- **THEN** the answer is the same one the binary gives

### Requirement: A command bridged from a branch environment stays in it

The tool SHALL make the registration available to the part of it that cannot read the registry.

#### Scenario: An unported command run from a branch environment

- **WHEN** a command that is not ported is run from a registered branch environment
- **THEN** it resolves to that environment's project, services and code, and not to the main one

#### Scenario: The shell entry point run on its own

- **WHEN** the shell entry point is run directly from a registered branch environment
- **THEN** it resolves to that environment as well

### Requirement: What earlier versions recorded is kept

The tool SHALL bring across registrations written by earlier versions, without anybody being asked
to migrate.

#### Scenario: A machine upgrading with environments on it

- **WHEN** the registry is read on a machine whose branch environments were registered by an
  earlier version
- **THEN** those environments are listed

#### Scenario: Reading again

- **WHEN** the registry is read repeatedly
- **THEN** nothing is duplicated

### Requirement: Forgetting leaves nothing behind

The tool SHALL clear everything that records a branch environment when it is forgotten.

#### Scenario: Removing one that an earlier version registered

- **WHEN** a branch environment is removed
- **THEN** neither the registry nor what earlier versions wrote still holds it, and it is not
  listed again

### Requirement: The overlay is a file

The tool SHALL keep the compose overlay of a branch environment on disk.

#### Scenario: Starting a branch environment

- **WHEN** a branch environment is started
- **THEN** its overlay is a compose file Docker can load
