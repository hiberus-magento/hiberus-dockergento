# State registry

## ADDED Requirements

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
