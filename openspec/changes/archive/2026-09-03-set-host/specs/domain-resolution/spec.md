# Domain resolution

## ADDED Requirements

### Requirement: Nothing is added that buys nothing

The tool SHALL NOT write an entry for a name that already resolves to this machine.

#### Scenario: A wildcard resolver for the TLD

- **WHEN** the name already resolves to a loopback address
- **THEN** the hosts file is left alone, and it is said

#### Scenario: The same name twice

- **WHEN** the hosts file already resolves the name, whoever wrote the line
- **THEN** no second line is added

### Requirement: Only what this tool wrote is this tool's to remove

The tool SHALL leave alone every line it did not write.

#### Scenario: A line somebody wrote by hand

- **WHEN** an entry with no marker is asked to be removed
- **THEN** nothing is removed, and it is said

#### Scenario: Removing one it wrote

- **WHEN** an entry this tool added is removed
- **THEN** it is gone and the rest of the file is exactly as it was
