# container-inventory Specification

## Purpose
TBD - created by archiving change docker-adapter-and-list. Update Purpose after archive.
## Requirements
### Requirement: Talking to the daemon through its API

The tool SHALL obtain the state of the containers from the Docker API, in one query, and SHALL
find the daemon where the Docker CLI finds it.

#### Scenario: The endpoint is in the environment

- **WHEN** `DOCKER_HOST` is set
- **THEN** that is the daemon used

#### Scenario: The endpoint is in the context store

- **WHEN** `DOCKER_HOST` is unset and the configuration names a current context
- **THEN** the daemon of that context is used, which is where Colima, Docker Desktop and Rancher
  keep it

#### Scenario: Nothing says where it is

- **WHEN** neither the environment nor the configuration names one
- **THEN** the library's own default is used, and nothing is invented

#### Scenario: The daemon is not reachable

- **WHEN** the daemon does not answer
- **THEN** the command fails with the Docker exit code and says so

### Requirement: Grouping containers into environments

The tool SHALL report the environments on the machine, built from the containers' labels rather
than from a file that could disagree with them.

#### Scenario: A project's containers are one environment

- **WHEN** several containers share a project
- **THEN** they are reported as one environment, with how many of them are running

#### Scenario: Half of them running

- **WHEN** some containers of an environment are running and some are not
- **THEN** its state is partial, which is its own answer and not a rounding of the other two

#### Scenario: An environment whose directory is gone

- **WHEN** the directory an environment was created in no longer exists
- **THEN** it is reported as an orphan, and its branch is not asked for

#### Scenario: An environment created before the labels existed

- **WHEN** a compose project has a `phpfpm` service but none of this tool's labels
- **THEN** it is still reported, marked as having no metadata: something missing from an inventory
  is something nobody ever cleans up

#### Scenario: Somebody else's containers

- **WHEN** a compose project is not this tool's and has no `phpfpm` service
- **THEN** it is not reported

#### Scenario: A label missing from one container

- **WHEN** one container of an environment lacks a label the others carry
- **THEN** the value the others carry is kept

#### Scenario: The order does not depend on the machine

- **WHEN** the environments are listed on two machines with different locales
- **THEN** the order is the same, because it is by bytes and not by collation

