# Delta for go-entrypoint

## ADDED Requirements

### Requirement: A command proved in Go has no shell twin

Once a command is proved in Go through the engine-interaction test seam (the twelve retired in
this change: `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`, `purge`,
`npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`, `set-host`), the
shell tree SHALL NOT carry an implementation for it, and the shell entry point SHALL answer a
request for that command by naming the binary as where it lives, rather than failing as though
the command did not exist.

#### Scenario: Nothing under the shell tree implements it

- **GIVEN** a command proved in Go through its own engine-interaction test
- **WHEN** the shell command tree is searched for an implementation of it
- **THEN** none exists — the file was removed, not replaced
- **AND** the router still answers for it, so the absence is a retirement and not a lost command

Pinned by "and what the binary owns outright has no file to drift from" in
`tests/unit/command_safety_test.sh`, which walks the twelve by name and fails in both directions:
a resurrected `.sh` and a lost route are each a failure.

#### Scenario: Asking the shell entry point for a retired command

- **WHEN** the shell entry point is asked to run a command the binary owns and that has no shell
  twin
- **THEN** it answers that the command is now part of the binary and names the invocation to use,
  exiting with the usage-error code (2), without running any Docker validation first

Pinned by the `command_moved_to_binary` cases in `tests/integration/cli_contract_test.sh`.

#### Scenario: A shell caller needs the binary and it is not installed

- **GIVEN** a shell command that reaches a retired command by asking the binary for it
- **WHEN** the binary is not present in the installation directory
- **THEN** it refuses before doing any of the work, naming the command that needed the binary and
  how to build it, and exits with the error code (1) rather than continuing without it

Pinned by "hm_run_binary refuses when the binary is not installed" in
`tests/unit/exit_codes_test.sh`.

## MODIFIED Requirements

### Requirement: A ported command's shell entry point still answers by delegating

When a command is fully answered by the tool but keeps a shell script as its entry point — today
`docker-stop-all`, `docker-compose`, and the branch-environment command (`worktree`) — that script
SHALL delegate to the tool rather than duplicate or lose the behavior, so invoking it through the
shell entry point reaches the same answer. This is a different case from a command whose shell
twin is retired outright: those have no delegating script at all — see "A command proved in Go has
no shell twin."
(Previously: read as universal, with no named scope and no distinction from a command that no
longer has a shell entry point at all.)

#### Scenario: Running a ported docker tool through the shell entry point

- **GIVEN** `docker-stop-all` or `docker-compose` is answered by the tool
- **WHEN** either is invoked through the shell entry point instead of the tool directly
- **THEN** the shell script delegates to the tool, and the answer is the same

#### Scenario: The question still reaches the same terminal

- **WHEN** `docker-stop-all`'s confirmation is asked through the delegated shell entry point
- **THEN** the terminal that asks and reads the answer is the same one the tool would use directly

#### Scenario: Running the branch-environment command through the shell entry point

- **GIVEN** `worktree` is answered by the tool
- **WHEN** it is invoked through its shell entry point instead of the tool directly
- **THEN** the shell script delegates to the tool, and the answer is the same
