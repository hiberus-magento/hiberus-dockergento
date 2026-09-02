# Worktree environments

## ADDED Requirements

### Requirement: What is written when a branch environment is created

The tool SHALL write a registration and an overlay that either implementation can read.

#### Scenario: The registration

- **WHEN** a branch environment is created
- **THEN** the registration holds its path, branch, profile, address, project and whether it
  shares dependencies

#### Scenario: The overlay

- **WHEN** the overlay is written
- **THEN** every service appears exactly once, none publishes ports, and only the profile's web
  service carries the routing

#### Scenario: A profile with no web service

- **WHEN** the profile keeps no web service
- **THEN** no routing is written, because a router pointing at a service that was removed answers
  with a 404 nobody can explain

### Requirement: Refusing before creating

The tool SHALL check everything it can before anything exists on disk.

#### Scenario: A name that is taken

- **WHEN** the name a branch would take belongs to another branch
- **THEN** it is refused rather than resolved, naming the branch that has it

#### Scenario: Without the proxy

- **WHEN** the project is not routed through the global proxy
- **THEN** creating a branch environment is refused

#### Scenario: Two at once

- **WHEN** two agents create a branch environment at the same moment
- **THEN** they take turns, whichever implementation each is running
