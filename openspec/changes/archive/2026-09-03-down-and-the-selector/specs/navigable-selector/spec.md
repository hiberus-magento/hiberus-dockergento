# Navigable selector

## ADDED Requirements

### Requirement: One selector, whichever half asks

The tool SHALL offer the same list, with the same three ways of asking, from the ported half as
from the shell one.

#### Scenario: A question asked by a ported command

- **WHEN** a ported command needs one of several answers
- **THEN** the person is asked with `fzf` if it is installed, with the arrow keys if the terminal
  can be drawn on, and with the numbered list otherwise

#### Scenario: Nothing is chosen

- **WHEN** the person aborts the list
- **THEN** the command stops rather than continuing with no answer, with the exit code a shell
  uses for an interrupted command, and without a document — there is nothing to report
