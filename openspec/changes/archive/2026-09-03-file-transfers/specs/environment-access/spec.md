# Environment access

## ADDED Requirements

### Requirement: Copying does not destroy what it copies

The tool SHALL refuse to copy a path that is the same file on both sides.

#### Scenario: A path that is a bind mount

- **WHEN** a path that is mounted from the host is copied into the container
- **THEN** it is refused, and where it is mounted is named

### Requirement: A copy arrives where it was asked to

The tool SHALL place what it copies where the caller named, whether or not the destination exists
yet.

#### Scenario: The destination directory is not there

- **WHEN** something is copied into a directory the container does not have
- **THEN** the directory is made and the copy arrives

#### Scenario: A directory copied out over one that is there

- **WHEN** a directory is copied out of the container onto one of the same name
- **THEN** its contents replace what is there rather than nesting inside it

#### Scenario: Nothing is running

- **WHEN** the container is not running
- **THEN** the copy is refused with the service code rather than attempted
