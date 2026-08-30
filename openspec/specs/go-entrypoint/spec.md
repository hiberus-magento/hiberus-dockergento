# go-entrypoint Specification

## Purpose
TBD - created by archiving change go-skeleton-and-bridge. Update Purpose after archive.
## Requirements
### Requirement: Everything not ported runs unchanged

The binary SHALL execute the shell implementation for every command it does not implement,
passing the arguments through untouched and returning its exit code.

#### Scenario: A command the binary does not implement

- **WHEN** the user runs any command through the binary
- **THEN** the shell implementation runs it and its output is identical to running it directly

#### Scenario: The exit code

- **WHEN** the shell implementation exits with a usage error, a refusal or a success
- **THEN** the binary exits with the same code, because they are a contract that callers branch on

#### Scenario: The terminal

- **WHEN** a command asks a question or streams output
- **THEN** it reads and writes the same terminal as it would without the binary

#### Scenario: The shell implementation cannot be found

- **WHEN** the binary cannot locate the shell tree
- **THEN** it says so and exits with the code for something wrong with the tool itself

### Requirement: The project is resolved by the Go layer

The binary SHALL resolve which project a directory belongs to, from the root that ends up being
used.

#### Scenario: A main checkout

- **WHEN** the directory is a project
- **THEN** its name, root, domain, Magento directory and topology are reported from its own
  properties

#### Scenario: A worktree with no environment of its own

- **WHEN** the directory is a git worktree with no registered environment
- **THEN** it resolves against the main checkout, which is what the refusals depend on

#### Scenario: A worktree with an environment

- **WHEN** the directory is a registered worktree
- **THEN** it resolves against itself: its own root, its own properties, and a project name and
  address derived from the parent's

#### Scenario: A directory that is not a project

- **WHEN** the directory has no properties
- **THEN** that is reported as an empty project rather than as an error, because several commands
  run outside one

#### Scenario: Checked against the shell implementation

- **WHEN** both implementations resolve the same project
- **THEN** they agree on the name, the root, the domain and the Magento directory

### Requirement: Saying which binary this is

The binary SHALL be able to report itself and what it resolved, without changing what any real
command does.

#### Scenario: The build

- **WHEN** the user runs `hm-go-version`
- **THEN** the version and the revision it was built from are printed

#### Scenario: The resolution

- **WHEN** the user runs `hm-go-project`
- **THEN** what the Go layer resolved is printed as JSON

#### Scenario: They cannot collide

- **WHEN** the names of these commands are compared with the shell implementation's
- **THEN** none of them is a command the shell implementation has

### Requirement: Installing the binary

The release SHALL publish a binary per platform, and the installation SHALL make it the command,
without taking the shell implementation away from a machine that cannot get it.

#### Scenario: An installation

- **WHEN** the tool is installed
- **THEN** the binary for that platform and architecture is fetched into the checkout and the
  command in the path points at it

#### Scenario: A machine that cannot fetch it

- **WHEN** the binary cannot be downloaded
- **THEN** the command points at the shell implementation and says so, rather than leaving the
  machine without a tool

#### Scenario: Finding the shell tree from where it is installed

- **WHEN** the binary is invoked through the symlink an installation leaves in the path
- **THEN** it finds the shell tree beside it, whatever directory it was invoked from

### Requirement: The state of the migration is written down and true

The repository SHALL carry a document saying where the migration is, and a test SHALL keep it
from drifting.

#### Scenario: Every command is accounted for

- **WHEN** the document is compared with the command declarations
- **THEN** every command appears exactly once, with an owner

#### Scenario: A claim that is not true

- **WHEN** the document says a command is implemented in Go and nothing in the Go tree answers
  for it
- **THEN** the suite fails

#### Scenario: Carrying on later

- **WHEN** somebody opens the document
- **THEN** it says how to build it, how to test it, and which phase is next

