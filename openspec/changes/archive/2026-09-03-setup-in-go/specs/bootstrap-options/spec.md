# Bootstrap options

## ADDED Requirements

### Requirement: The environment is written before anything is installed

The tool SHALL write a project's compose files and properties before starting or installing
anything.

#### Scenario: A project with a Magento in it

- **WHEN** a project with a composer.lock is set up
- **THEN** its compose files are written with the images that Magento needs, and its properties
  record the name, the address and where the code is

#### Scenario: A compose file this tool wrote

- **WHEN** setup is run again on a project whose compose file is this tool's
- **THEN** the file is left as it is, because rewriting it recreates the containers

#### Scenario: A compose file from somewhere else

- **WHEN** setup is run on a project whose compose file is not this tool's
- **THEN** the environment is written

### Requirement: A project behind the proxy carries the overlay, and only while it uses it

The tool SHALL write the routing overlay for a project that uses the global proxy, and SHALL
remove it from one that does not.

#### Scenario: The project stops using the proxy

- **WHEN** a project that was routed through the proxy is set up again with the proxy off
- **THEN** the overlay is gone, because one left behind would take the published ports away and
  leave the project answering on nothing

### Requirement: What cannot be answered without asking

The tool SHALL refuse a choice it cannot make, and SHALL NOT ask about one it can.

#### Scenario: No database mode, and nobody to ask

- **WHEN** setup runs without a dump and without a clean install, and nothing can be asked
- **THEN** it is refused with the usage code

#### Scenario: The mail catcher, and nobody to ask

- **WHEN** setup runs where nothing can be asked and the project has not chosen one
- **THEN** the default is used and nothing is asked
