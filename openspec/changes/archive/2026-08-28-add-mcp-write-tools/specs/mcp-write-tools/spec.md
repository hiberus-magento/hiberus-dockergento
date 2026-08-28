# MCP write tools

## ADDED Requirements

### Requirement: Bounded write tools, off by default

The tool SHALL offer write tools over the Model Context Protocol only when explicitly enabled, and
SHALL omit them entirely otherwise.

#### Scenario: The server runs read-only

- **WHEN** `hm mcp` runs without `--write` and the client lists the tools
- **THEN** only the read-only tools are returned, and none of the write tools appears

#### Scenario: Calling one that is not offered

- **WHEN** a client calls a write tool on a read-only server
- **THEN** it is answered as an unknown tool, with the tools that do exist named

#### Scenario: The server runs with writes enabled

- **WHEN** `hm mcp --write` runs and the client lists the tools
- **THEN** `cache_clean`, `cache_flush`, `reindex` and `config_set` are returned in addition to
  the read-only ones

#### Scenario: The configuration says so

- **WHEN** the user runs `hm mcp --config --write`
- **THEN** the printed client entry passes `--write` to the server

### Requirement: What the write tools do

The tool SHALL implement each write tool over the corresponding Magento command, and SHALL declare
through the protocol's annotations that they modify the project.

#### Scenario: Flushing the cache

- **WHEN** `cache_flush` is called
- **THEN** the Magento cache is flushed and the command's output is returned

#### Scenario: Cleaning some cache types

- **WHEN** `cache_clean` is called with a list of types
- **THEN** only those types are cleaned

#### Scenario: Reindexing

- **WHEN** `reindex` is called with no argument
- **THEN** every index is rebuilt; with an index name, only that one

#### Scenario: Setting a configuration value

- **WHEN** `config_set` is called with a path and a value
- **THEN** the value is set at default scope

#### Scenario: Something that is not a configuration path

- **WHEN** `config_set` is called with a path that is not of the form `section/group/field`
- **THEN** it is refused without running anything, and the refusal says what a path looks like

#### Scenario: The annotations are declared

- **WHEN** the tools are listed
- **THEN** the write tools declare `readOnlyHint` false and the read tools declare it true

### Requirement: What is not offered

The tool SHALL NOT offer tools for operations whose failure requires a person, however convenient
they would be.

#### Scenario: The excluded operations

- **WHEN** the tool list is inspected, with writes enabled
- **THEN** there is no tool for `setup:upgrade`, for Composer, for compiling dependency injection,
  for importing a database, or for removing an environment
