# Progress signal

## ADDED Requirements

### Requirement: Something on screen before the work starts

The tool SHALL print a description of any operation that can take longer than a moment before
beginning it, so that nothing is silent while it runs.

#### Scenario: A long silent operation

- **WHEN** a snapshot, a restore, an import, a freeze or a clone begins
- **THEN** a line describing it is printed before the work starts

#### Scenario: Work that prints its own output

- **WHEN** an operation streams output of its own
- **THEN** the label is printed first and the output follows, without an animation over it

#### Scenario: Finishing

- **WHEN** an operation that took more than two seconds finishes
- **THEN** the elapsed time is reported

### Requirement: Animation only where it belongs

The tool SHALL animate only when stdout is a terminal and the output is meant for a person, and
SHALL print plain lines otherwise.

#### Scenario: Piped output

- **WHEN** stdout is not a terminal
- **THEN** no escape sequence is written, and the operation is described by plain lines

#### Scenario: JSON output

- **WHEN** the command is producing JSON
- **THEN** nothing is animated and nothing is written to stdout that is not the document

#### Scenario: Colour turned off

- **WHEN** `NO_COLOR` is set, or `TERM` is `dumb`, or the run is non-interactive
- **THEN** the animation is off and the plain lines are used

#### Scenario: A terminal

- **WHEN** stdout is a terminal and the output is for a person
- **THEN** the line is animated while the work runs and replaced by the result when it ends

### Requirement: Reporting failure without drowning success

The tool SHALL keep the output of a wrapped silent command and SHALL show it only when the command
fails.

#### Scenario: It works

- **WHEN** a wrapped command succeeds
- **THEN** its output is not printed

#### Scenario: It fails

- **WHEN** a wrapped command fails
- **THEN** the failure is reported and the captured output is printed with it

#### Scenario: The exit code survives

- **WHEN** a wrapped command fails
- **THEN** the wrapper returns that command's exit code
