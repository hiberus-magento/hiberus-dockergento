# diagnosis Specification

## Purpose
TBD - created by archiving change doctor-in-go. Update Purpose after archive.
## Requirements
### Requirement: Diagnosing a machine and a project

The tool SHALL report what is wrong with the machine and with this project, each finding with the
command that fixes it, and SHALL answer exactly what the shell implementation answered.

#### Scenario: The document

- **WHEN** `hm doctor --json` runs in a project
- **THEN** the checks and the summary are identical to the shell implementation's

#### Scenario: The report

- **WHEN** `hm doctor` runs with colour disabled, and again with colour forced
- **THEN** the text is identical to the shell implementation's, escape sequences included

#### Scenario: Every check, in the same order

- **WHEN** the diagnosis runs twice
- **THEN** the findings come out in the same order, which is the order the shell implementation
  reported them in

#### Scenario: Something broken

- **WHEN** any check reports an error
- **THEN** the command fails, so that a script does not have to read the report to know

#### Scenario: One check on its own

- **WHEN** `--only=<id>` names a check
- **THEN** only that check runs, and answers what it answered as part of the whole

#### Scenario: An option nobody declared

- **WHEN** an unknown option is given
- **THEN** it is a usage error, with the usage exit code

### Requirement: Diagnosing outside a project

The tool SHALL diagnose the machine from a directory that is not a project, because that is the
question somebody has when nothing works anywhere.

#### Scenario: No project here

- **WHEN** the diagnosis runs outside a configured project
- **THEN** the checks about the machine still report, and none of the project ones do

#### Scenario: The ports an environment would need

- **WHEN** there is no configuration to ask which ports are needed
- **THEN** they come from the template the tool ships

### Requirement: A check that cannot answer

The tool SHALL report a check that hangs or fails as a finding of its own, and SHALL not let it
stop the rest of the diagnosis.

#### Scenario: A check that hangs

- **WHEN** a check does not answer within its time limit
- **THEN** it is abandoned, reported as timed out, and every other check still reports

#### Scenario: Nothing to look with

- **WHEN** no tool on the machine can list the ports in use
- **THEN** the check says so, rather than reporting that every port is free

### Requirement: Attributing a port conflict

The tool SHALL name the environment holding a port it needs, and SHALL not name a process it was
not told.

#### Scenario: Another environment

- **WHEN** ports this project needs are published by another environment
- **THEN** one finding names that environment and every port it holds

#### Scenario: This project's own ports

- **WHEN** the ports are held by this project's own containers
- **THEN** there is no conflict

#### Scenario: Something on the host with no name

- **WHEN** the tool that listed the ports names no process
- **THEN** the finding says the ports are held by processes on the host

### Requirement: Recognising the generated agent context

The tool SHALL tell a generated agent context that still describes this project from one that does
not, using the fingerprint the generator recorded.

#### Scenario: A context the generator wrote

- **WHEN** the context was generated from this configuration
- **THEN** it is reported as matching

#### Scenario: A project that moved on

- **WHEN** the configuration has changed since the context was generated
- **THEN** it is reported as an error, because an agent obeys what it reads

#### Scenario: No context at all

- **WHEN** there is no generated context
- **THEN** it is a suggestion, not a failure

