# Database templates

## ADDED Requirements

### Requirement: Freezing the data directory as a template

The tool SHALL save the project's database data directory as a named, reusable template, stopping
the database server while it copies so the copy is consistent.

#### Scenario: Freezing while the environment is running

- **WHEN** the user runs `hm db freeze --name=base` with the environment up
- **THEN** the tool says the database will be unavailable while it copies, stops the database
  service, copies its data directory into `hm-template-<project>-base`, and starts the service
  again

#### Scenario: Freezing with the environment down

- **WHEN** the user runs `hm db freeze` and no container of the project is running
- **THEN** the copy is made without starting or stopping anything

#### Scenario: The name is already taken

- **WHEN** a template with that name already exists
- **THEN** the tool refuses and says which name is taken, unless `--force` is given

#### Scenario: There is nothing to freeze

- **WHEN** the project has no data volume, or it is empty
- **THEN** the tool reports it and exits without creating an empty template

### Requirement: Cloning a template into the project

The tool SHALL build the project's data directory from a template, replacing whatever is there,
and SHALL refuse to do so while the environment is running or under an incompatible database
image.

#### Scenario: Cloning into a stopped environment

- **WHEN** the user runs `hm db clone base` with the environment down
- **THEN** the project's data volume is replaced by a copy of the template and the tool reports
  what was cloned and from where

#### Scenario: Cloning while the environment is running

- **WHEN** any container of the project is running
- **THEN** the tool refuses, naming the command that brings the environment down, and exits with
  the blocked code

#### Scenario: Cloning replaces existing data

- **WHEN** the project already has a data volume with data in it
- **THEN** the tool requires the user to confirm by typing the project name before replacing it,
  unless the run is forced or non-interactive with an explicit force

#### Scenario: The template comes from a different database version

- **WHEN** the template records a database image different from the one the project runs
- **THEN** the tool refuses, naming both images, unless `--force` is given

#### Scenario: A template of another project

- **WHEN** the user runs `hm db clone shop/base` from a different project
- **THEN** the template of the project `shop` named `base` is used

#### Scenario: The template does not exist

- **WHEN** the named template cannot be found
- **THEN** the tool reports it, lists the templates that do exist, and exits with the usage code

### Requirement: Listing and deleting templates

The tool SHALL list the templates on the machine with the information needed to choose one, and
SHALL delete a named template on request.

#### Scenario: Listing templates

- **WHEN** the user runs `hm db templates`
- **THEN** each template is listed with its full `<project>/<name>` address, its size, the
  database image it came from and when it was made

#### Scenario: Listing as data

- **WHEN** the user runs `hm db templates --json`, or stdout is not a terminal
- **THEN** the same information is printed as JSON with a schema version

#### Scenario: Deleting a template

- **WHEN** the user runs `hm db drop base`
- **THEN** the tool asks for confirmation, deletes the volume, and reports the space recovered

#### Scenario: Deleting a template that is in use

- **WHEN** the volume of the template is attached to a running container
- **THEN** the deletion is refused with an explanation instead of a Docker error
