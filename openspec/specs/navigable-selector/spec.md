# navigable-selector Specification

## Purpose
TBD - created by archiving change navigable-selector. Update Purpose after archive.
## Requirements
### Requirement: Choosing from a list with the arrow keys

The tool SHALL let a person move through the options of a question with the arrow keys and take
one with Enter, with the first option preselected.

#### Scenario: Moving and choosing

- **WHEN** the user presses the down arrow and then Enter
- **THEN** the second option is chosen and returned in `REPLY`

#### Scenario: Taking the default

- **WHEN** the user presses Enter without moving
- **THEN** the first option is chosen

#### Scenario: Moving past the end

- **WHEN** the user moves down from the last option
- **THEN** the selection wraps to the first, and moving up from the first wraps to the last

#### Scenario: The editor keys

- **WHEN** the user presses `j` or `k`
- **THEN** the selection moves down or up

#### Scenario: A digit

- **WHEN** the user presses the digit of an option
- **THEN** that option is chosen without pressing Enter

#### Scenario: Escape

- **WHEN** the user presses escape
- **THEN** nothing is chosen and the list stays, because a caller that received an empty answer
  would carry on with nothing chosen

#### Scenario: The question stays on screen

- **WHEN** the selection moves
- **THEN** the list is rewritten in place and nothing scrolls

### Requirement: Using what the person already has

The tool SHALL use `fzf` when it is installed and both ends are a terminal.

#### Scenario: fzf is installed

- **WHEN** a choice is asked for on a machine with `fzf`
- **THEN** `fzf` is used, with the question as its prompt, and its answer is returned

#### Scenario: The user aborts fzf

- **WHEN** `fzf` returns nothing
- **THEN** the command stops with the interrupted exit code rather than continuing with no answer

### Requirement: Still working where nothing can be drawn

The tool SHALL fall back to the numbered list, and SHALL keep refusing to choose when the run is
not interactive.

#### Scenario: A terminal that cannot be drawn on

- **WHEN** `TERM` is `dumb` or unset, or stdout is not a terminal
- **THEN** the numbered list is used, exactly as before

#### Scenario: Not interactive

- **WHEN** the run is non-interactive
- **THEN** the question is refused with the usage exit code, because a choice between options has
  no safe default

