# State registry

## ADDED Requirements

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
