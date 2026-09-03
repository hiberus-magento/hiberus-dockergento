# Environment lifecycle

## ADDED Requirements

### Requirement: Deleting the data is asked about

The tool SHALL NOT delete an environment's volumes without saying what it is about to delete and
offering to save a copy first.

#### Scenario: Removing an environment with its volumes

- **WHEN** the environment is removed with its volumes, interactively, and there are volumes to
  delete
- **THEN** they are named, and the answers offered are to save a copy first, to destroy without
  saving, or to do nothing

#### Scenario: The copy cannot be taken

- **WHEN** saving a copy first is chosen and the copy fails
- **THEN** nothing is destroyed, and the command says so

#### Scenario: Nothing to lose

- **WHEN** the environment has no volumes left to delete
- **THEN** nothing is asked

#### Scenario: Not interactive

- **WHEN** the run is not interactive
- **THEN** the volumes are deleted without a question, because the flag was explicit

### Requirement: Removing an environment is Compose's own command

The tool SHALL do what Compose does, and SHALL NOT remove more than it was asked to.

#### Scenario: Containers of a project no longer in the file

- **WHEN** an environment is removed without asking for orphans to go
- **THEN** they are left alone

#### Scenario: A branch environment being erased

- **WHEN** a branch environment is removed
- **THEN** its orphans go with it, because the environment is being erased rather than stopped
