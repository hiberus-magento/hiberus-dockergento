# Delta for go-entrypoint

## MODIFIED Requirements

### Requirement: A command proved in Go has no shell twin

Once a command is proved in Go through the engine-interaction test seam (the sixteen retired in
this change: `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`, `purge`,
`npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`, `set-host`,
`list`, `describe`, `logs`, `debug-on`), the shell tree SHALL NOT carry an implementation for
it, and the shell entry point SHALL answer a request for that command by naming the binary as
where it lives, rather than failing as though the command did not exist.
(Previously: named fifteen commands. This adds `debug-on` — `console/commands/debug-on.sh`
(38 lines), plus its sourced task `console/tasks/set_xdebug_host_property.sh` (19), which
disappears as a side effect and needs no retirement mechanism of its own.)

#### Scenario: Nothing under the shell tree implements it

- **GIVEN** a command proved in Go through its own engine-interaction test
- **WHEN** the shell command tree is searched for an implementation of it
- **THEN** none exists — the file was removed, not replaced
- **AND** the router still answers for it, so the absence is a retirement and not a lost command

Pinned by "and what the binary owns outright has no file to drift from" in
`tests/unit/command_safety_test.sh`, which walks the sixteen by name (Previously: fifteen) and
fails in both directions: a resurrected `.sh` and a lost route are each a failure.

#### Scenario: The RETIRED list gains its name only once the file is gone

- **GIVEN** `command_safety_test.sh`'s `RETIRED` array (`:69-71`) and the drift check (`:75-79`),
  which fails the moment a listed name still has a `console/commands/<name>.sh` file
- **WHEN** `debug-on` is added to `RETIRED`
- **THEN** that edit lands in the same commit that deletes `debug-on.sh`, never in an earlier
  commit where the file still exists — reversing the order turns the suite red

Test: "and what the binary owns outright has no file to drift from" in
`tests/unit/command_safety_test.sh` (ordering constraint on an existing, unmodified assertion).

#### Scenario: Asking the shell entry point for a retired command

- **WHEN** the shell entry point is asked to run a command the binary owns and that has no shell
  twin
- **THEN** it answers that the command is now part of the binary and names the invocation to use,
  exiting with the usage-error code (2), without running any Docker validation first

Pinned by the `command_moved_to_binary` cases in `tests/integration/cli_contract_test.sh`,
which gains one such case for `describe`.

#### Scenario: A shell caller needs the binary and it is not installed

- **GIVEN** a shell command that reaches a retired command by asking the binary for it
- **WHEN** the binary is not present in the installation directory
- **THEN** it refuses before doing any of the work, naming the command that needed the binary and
  how to build it, and exits with the error code (1) rather than continuing without it

Pinned by "hm_run_binary refuses when the binary is not installed" in
`tests/unit/exit_codes_test.sh`.
