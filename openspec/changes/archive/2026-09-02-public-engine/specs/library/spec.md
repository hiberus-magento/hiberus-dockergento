# The tool as a library

## ADDED Requirements

### Requirement: A usable engine

The tool SHALL be usable as a library by another program, without a terminal.

#### Scenario: Another module imports it

- **WHEN** a separate Go module depends on this one
- **THEN** it can import the engine and build against it

#### Scenario: Real answers

- **WHEN** that program asks what a directory's project is, what is running on this machine, and
  what is wrong with it
- **THEN** it gets the same answers the command line gives

#### Scenario: Changing things too

- **WHEN** that program brings an environment up
- **THEN** the environment is running afterwards

#### Scenario: The command line is not part of the API

- **WHEN** another module tries to import the command line
- **THEN** the language refuses it

### Requirement: Sensible defaults, stated exceptions

The engine SHALL work without configuration for a program installed beside the tool, and SHALL let
anything else say what it needs.

#### Scenario: No configuration

- **WHEN** an engine is built with no options
- **THEN** it resolves the tool's own installation from the running executable

#### Scenario: A program that is not the tool

- **WHEN** the caller says where the tool is installed
- **THEN** the defaults and the compose template are read from there

### Requirement: What a project is built from

The engine SHALL own the knowledge of which compose files a project is built from, so that no
consumer has to reconstruct it.

#### Scenario: A project on the proxy

- **WHEN** a project has a proxy overlay
- **THEN** the engine loads it and reports only the files the project declares

#### Scenario: A branch environment

- **WHEN** the project is a worktree with an environment of its own
- **THEN** the engine loads its overlay and not the proxy's
