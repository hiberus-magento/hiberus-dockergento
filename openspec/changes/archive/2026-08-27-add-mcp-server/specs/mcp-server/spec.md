# MCP server

## ADDED Requirements

### Requirement: Speaking the protocol

The tool SHALL implement the Model Context Protocol over stdin and stdout as newline-delimited
JSON-RPC, and SHALL print nothing on stdout that is not a protocol message.

#### Scenario: Initialising

- **WHEN** the client sends an `initialize` request
- **THEN** the server answers with its protocol version, its capabilities and its name and
  version, and accepts the `notifications/initialized` that follows without answering it

#### Scenario: Listing the tools

- **WHEN** the client sends `tools/list`
- **THEN** every tool is returned with a name, a description and a JSON Schema for its arguments

#### Scenario: An unknown method

- **WHEN** the client calls a method the server does not implement
- **THEN** it answers with a JSON-RPC error of code -32601 and keeps running

#### Scenario: A malformed line

- **WHEN** a line arrives that is not valid JSON
- **THEN** the server answers with a parse error and keeps running, rather than exiting

#### Scenario: Wrapped commands cannot corrupt the stream

- **WHEN** a wrapped command writes to stderr or fails
- **THEN** nothing of it reaches stdout, and the tool call returns an error result the model can
  read

### Requirement: Read-only tools over the existing commands

The tool SHALL offer tools that answer questions about the project and the machine, each
implemented over a command that already produces JSON, and SHALL offer no tool that changes the
environment.

#### Scenario: Describing the project

- **WHEN** `describe_project` is called
- **THEN** the result contains what `hm describe --json` reports: versions, services, URLs and
  state

#### Scenario: Listing environments

- **WHEN** `list_environments` is called
- **THEN** every environment on the machine is returned with what it belongs to

#### Scenario: Reading logs

- **WHEN** `service_logs` is called with a service name
- **THEN** the last lines of that service's log are returned, capped, and an unknown service name
  produces an error result naming the services that exist

#### Scenario: Diagnostics

- **WHEN** `check_environment` is called
- **THEN** the checks `hm doctor` runs are returned with their results

#### Scenario: No tool changes anything

- **WHEN** the tool list is inspected
- **THEN** none of the tools maps to a command classified as dangerous, except the database
  query, which is constrained to reading

### Requirement: The database tool reads and cannot write

The tool SHALL accept a single read-only statement, SHALL refuse anything else before running it,
and SHALL cap what it returns.

#### Scenario: A select

- **WHEN** `database_query` is called with `SELECT value FROM core_config_data LIMIT 5`
- **THEN** the rows are returned with their column names

#### Scenario: A statement that writes

- **WHEN** the statement is an `UPDATE`, `DELETE`, `DROP`, `INSERT` or anything else that is not a
  read
- **THEN** it is refused without being run, and the refusal says what is allowed

#### Scenario: A second statement smuggled in

- **WHEN** the argument contains more than one statement, or a comment hiding one
- **THEN** it is refused without being run

#### Scenario: Writing a file through the database

- **WHEN** the statement contains `INTO OUTFILE` or `INTO DUMPFILE`
- **THEN** it is refused, because those write files on the host as the database user

#### Scenario: A result too large to be useful

- **WHEN** the query returns more rows than the cap
- **THEN** the cap is applied and the answer says the result was truncated

### Requirement: Wiring it up

The tool SHALL print the configuration entry an MCP client needs, and SHALL NOT write it anywhere.

#### Scenario: Printing the configuration

- **WHEN** the user runs `hm mcp --config`
- **THEN** the JSON entry for this project is printed, with the absolute command and the project
  directory, and no file is modified
