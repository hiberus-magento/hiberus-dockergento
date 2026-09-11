# Delta for go-entrypoint

## MODIFIED Requirements

### Requirement: A command proved in Go has no shell twin

Once a command is proved in Go through the engine-interaction test seam (the fifteen retired in
this change: `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`, `purge`,
`npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`, `set-host`, `list`,
`describe`, `logs`), the shell tree SHALL NOT carry an implementation for it, and the shell entry
point SHALL answer a request for that command by naming the binary as where it lives, rather than
failing as though the command did not exist.
(Previously: named twelve commands. This adds `list`, `describe`, `logs` — `console/commands/
list.sh` (61 lines), `describe.sh` (92), `logs.sh` (70), 223 total.)

#### Scenario: Nothing under the shell tree implements it

- **GIVEN** a command proved in Go through its own engine-interaction test
- **WHEN** the shell command tree is searched for an implementation of it
- **THEN** none exists — the file was removed, not replaced
- **AND** the router still answers for it, so the absence is a retirement and not a lost command

Pinned by "and what the binary owns outright has no file to drift from" in
`tests/unit/command_safety_test.sh`, which walks the fifteen by name (Previously: twelve) and
fails in both directions: a resurrected `.sh` and a lost route are each a failure.

#### Scenario: The RETIRED list gains its three names only once the files are gone

- **GIVEN** `command_safety_test.sh:69-70`'s `RETIRED` list and the drift check at `:74-77`,
  which fails the moment a listed name still has a `console/commands/<name>.sh` file
- **WHEN** `list`, `describe`, and `logs` are added to `RETIRED`
- **THEN** that edit lands in the same commit that deletes their three shell files, never in an
  earlier commit where the files still exist — reversing the order turns the suite red

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

### Requirement: What still covers the unknown-service refusal

`logs`' unknown-service validation (`orchestrate.go:197-221`) calls `Configuration`, which the
seam deliberately does not carry (Decision 1), so `checkServices` cannot fire this branch in an
`internal/cli` test against a fake project. Integration coverage SHALL prove it instead.

#### Scenario: The refusal is proven through the built binary

- **GIVEN** the `unknown_service` branch cannot fire in an `internal/cli` test
- **WHEN** the same refusal is asked for through the built binary instead of the fake
- **THEN** `tests/integration/logs_launch_version_test.sh:162-167`'s `logs nonexistent` call is
  routed in this change's commit 1 from `run_in_lab`/`bin/run` onto `run_in_lab_go`/`GO_BINARY`,
  after which it asserts exit 5 and that the refusal names `phpfpm`, sitting above the
  `docker info` gate at `:173` so it needs no daemon
- **AND** the main spec's `:583-585` claim is corrected in two parts, not one: the line citation
  drifted from `:148-153` to `:162-167`, **and** the claim itself — "it runs the Go binary" — was
  not true of either citation before this change. Today, that call site still drives `bin/run`,
  i.e. the shell implementation still resident in `logs.sh`; the sentence written to justify
  excluding `Configuration` from the seam described a guarantee that did not yet exist. It becomes
  true only because this cycle's own commit 1 repairs the call site to use the binary — the
  coverage begins to exist here, it is not merely re-cited
- **AND** `tests/integration/go_orchestration_test.sh:165-181`'s Go/shell comparison is retired in
  this change, because `logs.sh` — the shell half of that comparison — stops existing
- **AND** what replaces it is a Go-only assertion against the container's own fixed output: exit 5
  and `.error.type == "unknown_service"`, not a comparison against `bin/run`

Covered by: `tests/integration/logs_launch_version_test.sh` (modified — its `logs nonexistent`
call site is routed to the binary, and its citation in this spec corrected) and
`tests/integration/go_orchestration_test.sh` (modified — parity retired, Go-only assertion kept).
No `internal/cli` test asserts this branch.

### Requirement: A usage error returns before any engine call

`copy-to-container`, `copy-from-container`, `mysqldump`, `version`, `set-host`, `list`,
`describe`, `logs`, and `doctor` SHALL validate their arguments before they ask anything of the
engine.

#### Scenario: No path given

- **WHEN** either `copy-to-container` or `copy-from-container` runs with no path argument
- **THEN** it fails with the missing-path error, and the fake engine used by its test records
  zero calls

#### Scenario: mysqldump with no path

- **WHEN** `mysqldump` runs with no path argument
- **THEN** it fails with the missing-path error, and the fake engine used by its test records
  zero calls

#### Scenario: version with an argument

- **WHEN** `version` runs with any argument
- **THEN** it fails with a usage error, and the fake engine used by its test records zero calls

#### Scenario: set-host with an unknown option

- **WHEN** `set-host` runs with an option it does not recognize
- **THEN** it fails with a usage error, and the fake engine used by its test records zero calls

#### Scenario: list with any argument

- **WHEN** `list` runs with any argument
- **THEN** it fails with a usage error, and the fake engine used by its test records zero calls
- **AND** this holds both with no fake at all (the refusal returns during parsing) and, once
  routed, under `answering(t)`
- **AND** (new) at the integration level, `introspection_test.sh:57-59` (`list --nope`) also
  asserts `.error.type == "invalid_argument"` on stderr, not only exit 2 — see "A wrong-reason
  assertion is corrected to prove the fact it names", case 1
- **AND** (new) `go_passthrough_test.sh:99-102`'s retired `list --nonsense` parity keeps the same
  pair as a Go-only fact — case 10 of the same requirement

Test: `TestListRefusesAnyArgument`.

#### Scenario: describe with an unknown option

- **WHEN** `describe` runs with an option other than `--with-secrets`
- **THEN** it fails with a usage error, and the fake engine used by its test records zero calls
- **AND** (new) `go_passthrough_test.sh:80-83`'s retired `describe --nonsense` parity keeps exit 2
  and `.error.type == "invalid_argument"` as a Go-only fact once the shell comparison is gone —
  case 9 of "A wrong-reason assertion is corrected to prove the fact it names"
- **AND** (new) `introspection_test.sh:70-72` (`describe --nope --json`) is a behaviour difference
  exposed by routing, not a wrong-reason pass: today it asserts exit **4**, because `bin/run` runs
  `validate_docker_compose.sh` (`bin/run:289-299`) before `describe.sh` ever parses `--nope`. Once
  routed to the binary, this exact ordering requirement — arguments validated before any engine
  call — means the same invocation answers exit **2** and `.error.type == "invalid_argument"`
  instead; the expectation moves with the entry point, not around it — see "A validation-order
  difference exposed by routing" under "A wrong-reason assertion is corrected to prove the fact it
  names"

Test: `TestDescribeRefusesAnUnknownOption`.

#### Scenario: doctor with an unknown option

- **WHEN** `doctor` runs with an option other than `--only=<id>`
- **THEN** it fails with `invalid_argument`, before any project is resolved and before `Diagnose`
  is asked for anything

Test: `TestDoctorRefusesAnUnknownOption`.

#### Scenario: logs with an unknown option

- **WHEN** `logs` runs with an option it does not recognize
- **THEN** it fails with `invalid_argument`, and the fake engine used by its test records zero
  calls
- **AND** (new) this holds even for an option placed *after* the command name: `logs` is in the
  `transparent` map (`internal/cli/globals.go:17-21`), so an undeclared flag there reaches the
  handler's own option table (`orchestrate.go:139-143`) instead of being forwarded to Compose.
  `logs_launch_version_test.sh:211-213` (`logs --json db`) currently asserts exit **1**, because
  the shell implementation forwards the unrecognized flag to Compose, which is what refuses it.
  Routed to the binary, the same invocation is the binary's own usage error: exit **2** and
  `.error.type == "invalid_argument"` — case 12 of "A wrong-reason assertion is corrected to prove
  the fact it names"

Test: `TestLogsRefusesAnUnknownOption`.

#### Scenario: logs missing a value

- **WHEN** `logs` runs with `--tail`, `-n`, `--since`, `--until`, or `--index` as its last
  argument, with nothing after it
- **THEN** it fails with `missing_value`, and the fake engine used by its test records zero calls
- **AND** (new) at the integration level, `logs_launch_version_test.sh:169-171` (`logs --tail`
  with no value) also asserts `.error.type == "missing_value"` on stderr, not only exit 2 — case 4
  of "A wrong-reason assertion is corrected to prove the fact it names"

Test: `TestLogsRefusesAFlagWithNoValue`.

#### Scenario: logs with a non-numeric --index

- **WHEN** `logs --index abc` runs
- **THEN** it fails with `invalid_argument`, and the fake engine used by its test records zero
  calls

Test: `TestLogsRefusesANonNumericIndex`.

## ADDED Requirements

### Requirement: A wrong-reason assertion is corrected to prove the fact it names

Once `list.sh`, `describe.sh`, and `logs.sh` are retired, `bin/run`'s `command_moved_to_binary`
refusal (exit 2, empty stdout) SHALL NOT let an assertion written against the live shell command
keep passing for an unrelated reason, across all twelve sites this affects — in eight suites, not
seven: `introspection_test.sh`, `logs_launch_version_test.sh` (seven `logs` call sites — `:163`,
`:170`, `:182`, `:189`, `:196`, `:205`, `:212` — plus `describe` at `:119`, not eight),
`project_name_test.sh`, `concurrent_safety_test.sh`, `mail_catcher_test.sh`,
`go_passthrough_test.sh`, `go_orchestration_test.sh`, and `worktree_test.sh` (previously unnamed;
`HM="$COMMAND_BIN_DIR/bin/run"` at `:12` drives `describe` at `:66`, `:73`, `:115`, `:151` and
`list` at `:119`). This includes two shapes new to this repository's test suites: `assert_equals`
comparing two calls to the same helper that both degrade to `""`, and `assert_json_field` with an
empty expected value, which `jq -r` also answers for a refused command. Each assertion in scope
SHALL either be tightened to assert a fact a refusal cannot satisfy, or, where an adjacent
assertion on the same stdout already fails loudly on empty output, routed to the binary without
further tightening — except where routing itself is not available, in which case tightening SHALL
happen with the entry point unchanged.

#### Scenario: Routed and tightened

- **AND** case 1 — `introspection_test.sh:57-59` (`list --nope`): asserts exit 2 **and**
  `.error.type == "invalid_argument"`
- **AND** case 2 — `introspection_test.sh:115-116` (`.data.magento.version == ""`): gains a
  `'.data.magento | has("version")' == "true"` guard, so an empty document (a refusal) cannot
  satisfy it — the empty value must come from a real, running command missing `composer.lock`
- **AND** case 4 — `logs_launch_version_test.sh:169-171` (`logs --tail` with no value): asserts
  exit 2 **and** `.error.type == "missing_value"`
- **AND** case 12 — `logs_launch_version_test.sh:211-213` (`logs --json db`): currently asserts
  exit 1, because the shell implementation forwards the undeclared flag to Compose, which refuses
  it. `logs` is in the `transparent` map (`internal/cli/globals.go:17-21`), so the binary's own
  option table (`orchestrate.go:139-143`) sees the flag first instead; routed, the assertion
  becomes exit 2 **and** `.error.type == "invalid_argument"` — the binary's own usage error, not
  Compose's

#### Scenario: Tightened only, because the entry point cannot move

- **GIVEN** `bin/run:103` derives `COMPOSE_PROJECT_NAME` from the directory when the property is
  absent, while `dockergento/app/resolve.go:116` reads only `properties["COMPOSE_PROJECT_NAME"]`
  with no such fallback — name derivation is not ported, confirmed in both files, and is out of
  scope for this change
- **AND** case 7 — `project_name_test.sh:96-98` ("a configured name is inherited by a copy") and
  case 8 — `project_name_test.sh:114-115` ("a worktree does not invent a second environment") both
  exercise fixtures with no configured name, so routing `hm_name()` to the binary would make it
  answer `""` for both sides of each comparison — a wrong-reason pass of the same kind these cases
  exist to remove, not a fix for one
- **WHEN** these two assertions are corrected
- **THEN** `hm_name()`'s entry point stays `bin/run`, and each assertion is tightened instead: case
  7 asserts both sides equal the literal `hmnametest`; case 8 asserts both sides equal the literal
  `principal` — the helper's own command changes from `describe --json` to `db list --json`
  (`db.sh:171-174`), a different shell command carrying the same resolved variable, not a
  routing to the binary

#### Scenario: Routed only, because an adjacent same-stdout assertion already fails loudly

- **AND** case 3 — `introspection_test.sh:165-166` (`assert_not_contains "$STDOUT"
  '"schema_version"'`): needs no tightening because `:161-163`'s `assert_contains "$STDOUT"
  "URLs"`, on the same `$STDOUT`, already fails on empty output
- **AND** case 5 — `logs_launch_version_test.sh:192-193` (`assert_not_contains "$STDOUT"
  "hello-from-phpfpm"`): the pairing positive is `:188-190`'s `assert_contains "$STDOUT"
  "hello-from-db"` on the same run
- **AND** case 6 — `logs_launch_version_test.sh:208-209` (`assert_not_contains "$STDOUT"
  "schema_version"`): the pairing positive is `:204-206`'s `assert_contains "$STDOUT"
  "hello-from-db"` on the same run
- **AND** case 11 — `worktree_test.sh:73-74` (`assert_json_field "$STDOUT" '.data.project.worktree'
  ""`, run from the main checkout): the same empty-expected shape as case 2, this time surviving
  by adjacency rather than by a guard — `:76-77`'s `assert_json_field "$STDOUT" '.data.project.name'
  "$PROJECT"`, on the same `$STDOUT`, fails on a refusal or an empty document

#### Scenario: Retired, because the shell half of the comparison stops existing

- **AND** case 9 — `go_passthrough_test.sh:80-83` (`both describe --nonsense`): the shell/Go
  comparison is dropped; the Go-only facts it keeps are documented under "A usage error returns
  before any engine call"
- **AND** case 10 — `go_passthrough_test.sh:99-102` (`both list --nonsense`): same disposition

#### Scenario: A validation-order difference exposed by routing, not a wrong-reason pass

- **GIVEN** `introspection_test.sh:70-72` (`describe --nope --json`), which today asserts exit
  **4** because `bin/run` runs `validate_docker_compose.sh` (`bin/run:289-299`) before
  `describe.sh` ever parses `--nope`
- **WHEN** this call is routed to the binary along with the rest of `introspection_test.sh`
- **THEN** the assertion moves to exit **2** and `.error.type == "invalid_argument"`, matching the
  ordering "A usage error returns before any engine call" already requires — this is a real
  behaviour difference the routing exposes, not an assertion that was passing for the wrong reason

Test: each case is named by its own `test_case` label and file:line above; no new Go test is
added, and no assertion in scope compares two invocations of the same helper to each other once
this requirement is satisfied.
