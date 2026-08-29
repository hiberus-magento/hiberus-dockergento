# Database client launchers

## ADDED Requirements

### Requirement: Opening a client on this project's database

The tool SHALL open a named database client connected to the current project's database, using
the connection details of the resolved configuration.

#### Scenario: The client is installed

- **WHEN** the user runs `hm tableplus` in a project whose database publishes a port
- **THEN** TablePlus is opened with a connection to that port, user, password and database

#### Scenario: The client is not installed

- **WHEN** the client cannot be opened
- **THEN** the connection string is printed with the name of what was missing, rather than an
  error alone

#### Scenario: Printing instead of opening

- **WHEN** `--print` is given
- **THEN** the connection string is printed and nothing is opened

#### Scenario: The details come from the project

- **WHEN** the project uses a database name or credentials of its own
- **THEN** they appear in the connection, because they are read from the resolved configuration
  rather than assumed

### Requirement: Not opening a client that cannot connect

The tool SHALL refuse to open a client when the database publishes no port, and SHALL name the
command that opens one.

#### Scenario: A project routed through the proxy

- **WHEN** the database service publishes no host port
- **THEN** the command stops, explains that the proxy removed it, and names `hm tunnel db`

#### Scenario: A tunnel is already open

- **WHEN** `--port` names a port
- **THEN** the connection uses it, without looking for a published one

#### Scenario: The environment is not running

- **WHEN** the database service is not running
- **THEN** the command says so with the service exit code
