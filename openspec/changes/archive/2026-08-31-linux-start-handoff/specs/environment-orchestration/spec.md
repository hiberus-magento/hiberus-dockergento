# Environment orchestration

## ADDED Requirements

### Requirement: One implementation of starting

The tool SHALL start environments through the same implementation on every platform, and SHALL
hand back only the steps that platform needs afterwards.

#### Scenario: What Linux needs afterwards

- **WHEN** an environment is started on Linux
- **THEN** the platform's own steps run after it is up

#### Scenario: What macOS needs afterwards

- **WHEN** an environment is started on macOS
- **THEN** there is nothing to do, and nothing is started to find that out

#### Scenario: A step that fails

- **WHEN** the steps after starting fail
- **THEN** the failure is reported and the environment is left running

#### Scenario: One copy of those steps

- **WHEN** either implementation brings an environment up
- **THEN** both run the same steps, from the same place

### Requirement: A project with no TLS terminator

The tool SHALL start a project that has no TLS terminator, which is every project routed through
the global proxy.

#### Scenario: Self-routing with nothing to route through

- **WHEN** the project has no TLS terminator running
- **THEN** the self-routing entries are skipped, said out loud, and the start succeeds

#### Scenario: The container the entries are written into

- **WHEN** the php container is not running
- **THEN** that is still reported, because there is nowhere to write them
