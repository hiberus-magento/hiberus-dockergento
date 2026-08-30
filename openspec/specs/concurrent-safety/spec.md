# concurrent-safety Specification

## Purpose
TBD - created by archiving change survive-several-agents. Update Purpose after archive.
## Requirements
### Requirement: One lock for shared state

The tool SHALL serialise the operations that write state shared between projects, with a lock
that works on every platform it supports.

#### Scenario: Two commands wanting the same lock

- **WHEN** two invocations try to write the same shared state at the same time
- **THEN** one proceeds and the other waits, and neither leaves the state half written

#### Scenario: A lock left by a process that died

- **WHEN** a lock is held by a process that no longer exists
- **THEN** it is broken and the operation proceeds, rather than waiting for a process that will
  never release it

#### Scenario: Waiting has an end

- **WHEN** a lock cannot be acquired within its timeout
- **THEN** the command fails saying what is holding it, rather than hanging

#### Scenario: Released on interrupt

- **WHEN** a command holding a lock is interrupted
- **THEN** the lock is released

### Requirement: Shared files are written atomically

The tool SHALL write every file that another process may read through a uniquely named temporary
and a rename.

#### Scenario: A reader during a write

- **WHEN** a file is being rewritten while another process reads it
- **THEN** the reader sees either the previous content or the new one, never a partial file

#### Scenario: Two writers at once

- **WHEN** two processes write the same file at the same time
- **THEN** their temporaries do not collide, because no temporary has a fixed name

### Requirement: A worktree resolves its own configuration

The tool SHALL resolve the properties of a registered worktree from the worktree itself.

#### Scenario: Reading

- **WHEN** a command runs in a registered worktree
- **THEN** the properties it reads are the worktree's own

#### Scenario: Writing

- **WHEN** a command in a registered worktree saves properties
- **THEN** they are written to the worktree, and the main checkout's are untouched

### Requirement: An agent's environment is labelled as one

The tool SHALL stamp the environments created for an agent, so they can be told apart from the
rest.

#### Scenario: A branch environment on the agent profile

- **WHEN** an environment is created with `--profile=agent`
- **THEN** its containers carry the agent label, and listing the environments shows which ones
  belong to an agent

### Requirement: Names and addresses are checked before they are taken

The tool SHALL refuse to register a branch environment whose compose project name or address is
already in use, and SHALL NOT invent a different name.

#### Scenario: A compose project name already in use

- **WHEN** the name a branch environment would take belongs to an existing project
- **THEN** it is refused, naming the project it would have collided with

#### Scenario: An address already routed

- **WHEN** the address a branch environment would answer on is already routed by another
  environment
- **THEN** it is refused, naming the other one

#### Scenario: Two branches with the same shape of name

- **WHEN** a branch's name reduces to one that is already registered
- **THEN** it is refused, saying which branch already holds it

### Requirement: Entries in the hosts file can be found and removed

The tool SHALL mark the entries it adds to the hosts file, SHALL be able to remove its own, and
SHALL report the ones left behind by environments that no longer exist.

#### Scenario: Adding an entry

- **WHEN** an entry is added to the hosts file
- **THEN** it carries a marker saying which tool added it

#### Scenario: Removing one

- **WHEN** the user asks to remove the entry of a project
- **THEN** only the marked entry for that domain is removed, and anything a person wrote is left
  alone

#### Scenario: Entries whose project is gone

- **WHEN** the collection command runs
- **THEN** the marked entries whose environment no longer exists are listed, with the command that
  removes them, and the file is not modified

