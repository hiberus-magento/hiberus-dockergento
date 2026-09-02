# Worktree environments

## ADDED Requirements

### Requirement: What a listing reports

The tool SHALL report the state of a branch environment from what is actually there, not from what
was recorded.

#### Scenario: Running or not

- **WHEN** branch environments are listed
- **THEN** each one's state comes from its containers

#### Scenario: A directory somebody deleted

- **WHEN** the worktree's directory is gone
- **THEN** it is reported as missing, and the command that collects it is named
