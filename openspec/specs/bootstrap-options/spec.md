# bootstrap-options Specification

## Purpose
TBD - created by archiving change bootstrap-options. Update Purpose after archive.
## Requirements
### Requirement: The documented options are accepted

The tool SHALL accept every option it documents for `setup`, in both its short and long forms.

#### Scenario: Long options

- **WHEN** `hm setup --domain=shop.test --project-name=shop --root-directory=. --force` is run
- **THEN** each value is taken, and none of them is treated as an unknown option

#### Scenario: A value after a space

- **WHEN** an option is given as `--domain shop.test`
- **THEN** it is read the same way as `--domain=shop.test`

#### Scenario: The short forms are unchanged

- **WHEN** `-d`, `-D`, `-p`, `-r`, `-i`, `-u` or `-f` are used
- **THEN** they behave exactly as before

#### Scenario: An option nobody declared

- **WHEN** an unknown option is given
- **THEN** the command exits with the usage code and says which option it was

### Requirement: The names Warden uses

The tool SHALL accept `--clean-install` and `--db-dump` as names for the install and dump options.

#### Scenario: Coming from Warden

- **WHEN** `hm setup --clean-install` or `hm setup --db-dump=dump.sql` is run
- **THEN** it means the same as `--install` and `--dump=dump.sql`

### Requirement: A dump that is not there stops the command

The tool SHALL refuse a dump path that does not exist, before creating anything.

#### Scenario: A path that does not exist

- **WHEN** `--dump=/no/such/file.sql` is given
- **THEN** the command exits with the usage code and names the file, rather than warning and
  continuing into a question

#### Scenario: A path that does

- **WHEN** the file exists
- **THEN** it is used

### Requirement: Bootstrapping without a person

The tool SHALL complete a setup without asking anything when the run is non-interactive and the
database mode has been given.

#### Scenario: A clean install with no questions

- **WHEN** `hm setup --yes --clean-install` is run
- **THEN** nothing is asked: the project name, the domain and the root directory take their
  defaults, and the database mode was given

#### Scenario: Non-interactive with no database mode

- **WHEN** `hm setup --yes` is run without `--install` or `--dump`
- **THEN** it refuses with the usage code, because that question has no safe default

#### Scenario: install takes the long form too

- **WHEN** `hm install --use-default` is run
- **THEN** it behaves as `hm install -u`

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
