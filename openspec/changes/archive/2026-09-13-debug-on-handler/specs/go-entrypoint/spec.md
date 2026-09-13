# Delta for go-entrypoint

## MODIFIED Requirements

### Requirement: A ported command's engine interaction is provable without Docker

For a command listed in `internal/cli/run.go`'s `Run()` switch, the repository SHALL carry a Go
test that verifies what its handler asks of the engine, through a fake substituted for the real
one, requiring no Docker daemon, no network access, and no project beyond what the test itself
builds.
(Previously: covered `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`,
`purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`,
`set-host`, `list`, `describe`, `logs`, `bash`, `exec`, `masquerade`, and `debug-off`. This adds
`debug-on`, the nineteenth, and the first that needs two new seam methods at once: `Capture`
(added, uncalled, by `capture-container-output`) and a new `(*dockergento.Engine).XdebugHost()
(string, error)` facade over `machine.Host.XdebugHost` (added, uncalled, by
`debug-on-host-detection`). `commands` grows from sixteen to eighteen — `Restart` is already on it,
exercised today by `varnish-on`/`varnish-off`, and needs no seam change of its own.)

#### Scenario: Copying into the container

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `copy-to-container <path>` runs against a resolved project
- **THEN** the fake records a call to copy into the container with that path

#### Scenario: Copying everything into the container

- **WHEN** `copy-to-container --all` runs
- **THEN** the fake records the same call with the "everything" flag set

#### Scenario: Copying out of the container

- **WHEN** `copy-from-container <path>` runs
- **THEN** the fake records a call to copy out of the container with that path

#### Scenario: Turning the page cache on

- **WHEN** `varnish-on` runs
- **THEN** the fake records, in order, the VCL edit on the varnish service, a restart of the
  varnish service, and the enable call on the php service

#### Scenario: Turning the page cache off cascades

- **WHEN** `varnish-off` runs
- **THEN** the fake records the VCL edit, the restart, and the disable call, followed by the
  purge and the cache-clean that turning it off leaves behind, in that order

#### Scenario: Turning the page cache on stops at a failing step

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `varnish-on` runs and the VCL edit or the `cache:enable` call answers a non-zero status
  with no error
- **THEN** the fake records only the calls up to and including the one that failed — no restart
  and no `cache:enable` call are recorded when the edit is the one that failed
- **AND** the command's return value is that same status

Test: `TestAFailedVCLEditReportsTheContainersCode` (shared code with `varnish-off`, proved once).

#### Scenario: Turning the page cache off stops at a failing step

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `varnish-off` runs and the VCL edit or the `cache:disable` call answers a non-zero
  status with no error
- **THEN** the fake records only the calls up to and including the one that failed — no purge and
  no cache-clean calls are recorded
- **AND** the command's return value is that same status

Test: `TestAFailedCacheToggleStopsBeforeThePurge`.

The exit code, the text shown, and the `--json` document for both scenarios above are governed by
`magento-commands`' "A wrapper runs what it says it runs" and by `cli-output-contract`'s "Errores
estructurados" and "Códigos de salida estables"; this requirement owns only what the fake records.

`TestAFailedEditStopsBeforeTheRestart` (the `err != nil` branch of the same two guards) is
unchanged by this delta and keeps passing.

#### Scenario: Turning xdebug off

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `debug-off` runs against a resolved project
- **THEN** the fake's whole call log holds exactly three entries, in order: a `Resolve` call; an
  `Exec` call with `Dir: project.Root`, `Service: phpService`, `Options: terminalOptions("")`, and
  `Command: []string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
  "/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"}` — the shell's `s/^\zend_extension/`
  loses its stray, inert `\` before `z`; and a `Compose` call with `Dir: project.Root` and
  `Command: []string{"restart", "phpfpm"}`
- **AND** no fourth call is recorded, and in particular no `Start` call — `debug-off` never asks
  the engine to start anything, unlike the shell script's own defensive probe

Test: `TestWhatDebugOffAsks`.

#### Scenario: The restart used is Compose's own, not a stop-and-start

- **GIVEN** `Operator.Restart` refuses unconditionally from an unregistered worktree and may
  recreate containers, while `Operator.Compose` carries neither of those (`app/orchestrate.go:
  231-247, 326-327`)
- **WHEN** `debug-off` runs
- **THEN** the engine call recorded is `Compose(["restart", "phpfpm"])`, never
  `Restart(["phpfpm"])`, so an edit made inside the running container is not undone by a recreate,
  and an unregistered worktree that could run `debug-off` before this port still can, unrefused
- **AND** this is provable by construction, not by a seam assertion of worktree state: the choice
  of method is what `TestWhatDebugOffAsks`'s `Compose` log entry pins

#### Scenario: Turning xdebug off stops at a failing edit

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `debug-off` runs and the `sed` edit answers a non-zero status with no error
- **THEN** the fake records only the `Resolve` and `Exec` calls — no `Compose` call and no `Start`
  call are recorded
- **AND** the command's return value is that same status
- **AND** this is also the seam-visible half of the disclosed behaviour change: with the
  environment stopped, the edit's own `Exec` call is what answers a non-zero status, and
  `debug-off` now fails naming that step instead of starting the whole environment the way the
  shell script's `start_service_if_not_running.sh` probe did

Test: `TestAFailedDebugOffEditReportsTheContainersStatus`.

#### Scenario: A Go-level error on the edit stops before the restart

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `debug-off` runs and the `sed` edit answers an error rather than a non-zero status
- **THEN** the `Compose` call is not reached, and the failure is reported with the mapped Docker
  exit code

Test: `TestADebugOffEditErrorStopsBeforeTheRestart`.

#### Scenario: The restart's own non-zero status is returned as-is

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `debug-off` runs and the `Compose(["restart", "phpfpm"])` call answers a non-zero status
  with no error
- **THEN** the fake's whole call log still holds all three entries — `Resolve`, `Exec`, `Compose`
  — because the restart is the last call attempted, not one that failed to be recorded
- **AND** the command's own return value is that same status, unwrapped — no `failure()` call
  names a step, and no structured `ok: false` object is written under `--json`; the raw status is
  the only signal, on stdout or in the process exit code, exactly as it is for a passthrough
  command
- **AND** this mirrors `dockerCompose()`'s own handling of the identical `(int, error)` pair
  (`internal/cli/docker_compose.go:15-20`: `if err != nil { return report(...) }; return status`)
  and `Operator.Compose`'s own doc, "hands back that subcommand's own exit code"
  (`dockergento.go:298-300`) — and matches the shell twin, where `set -euo pipefail` ends the
  script with `$DOCKER_COMPOSE restart phpfpm`'s own exit status and nothing else
- **AND** this is the one place `debug-off`'s failure handling diverges from the sed edit's: the
  edit's non-zero status is wrapped through `failure()` with a named step and a structured
  `--json` error object (`magento-commands`' "The xdebug ini edit fails inside the php
  container"); the restart's is not — because the edit runs a program inside a container
  (`varnish`'s precedent) and the restart is a Compose passthrough (`docker-compose`'s precedent),
  and those are different things this port keeps different rather than unifying by accident

Test: `TestAFailedDebugOffRestartReturnsTheComposeStatus`.

#### Scenario: A Go-level error on the restart is reported

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `debug-off` runs and the `Compose` call answers an error rather than a non-zero status
- **THEN** the failure is reported through the same fault-mapping `report()` uses elsewhere —
  `exitDocker`, `error.type` `docker_failed` under `--json` — the same path `dockerCompose()`'s own
  `err != nil` branch takes, and the same path `varnish`'s failed `Restart` takes

Test: `TestADebugOffRestartErrorIsReported`.

#### Scenario: Disabling xdebug in JSON

- **WHEN** `debug-off --json` succeeds
- **THEN** the document's `data` carries `{"xdebug": false}`, mirroring `varnish`'s `{"varnish":
  bool}` shape
- **AND** the envelope itself (`schema_version`, `command`, `ok`, `data`) is unchanged and remains
  `cli-output-contract`'s to govern — this scenario owns only the one new key
- **AND** on a failed restart, no document is produced at all, success or error — see "The
  restart's own non-zero status is returned as-is"

Test: `TestWhatDebugOffAsks` (JSON case).

#### Scenario: Turning xdebug on, with a resolved host, Xdebug v2

- **GIVEN** a fake engine substituted for the real one, answering `XdebugHost` with a non-empty
  address and no error, and its first `Capture` call (`php -v`) with an Xdebug v2 version string
- **WHEN** `debug-on` runs against a resolved project on Linux
- **THEN** the fake's whole call log holds, in order: a `Resolve` call; an `XdebugHost` call
  carrying no directory — it asks about the machine, not the project, the same way `Environments`
  and `Installed` ask for nothing beyond themselves; two `Exec` calls against `phpfpm`, each with
  `Options` built from `terminalOptions("")`, patching `/usr/local/etc/php/conf.d/xdebug.ini` —
  first `xdebug.remote_host`, then `xdebug.client_host` — with the resolved address, and with the
  shell's stray, inert `\=` dropped from both patterns (GNU sed already read it as a literal `=`,
  the same undefined-escape shape `debug-off`'s own `\z` was, per Decision 7); a third `Exec` call
  against `phpfpm` patching `/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini` — a **different
  file** from the host patches above — uncommenting `zend_extension` with the shell's stray `\;`
  also dropped; a `Restart` call for `["phpfpm"]`; and a `Capture` call running `["php", "-v"]`
- **AND** because the captured `php -v` output classifies as Xdebug v2, a second `Capture` call
  runs `["php", "-i"]`, and the printed text banner shows, between two separator lines, only the
  `xdebug.idekey`, `xdebug.remote_host` and `xdebug.remote_port` lines the fixture carried — none
  of the v3 keys, even when the fixture given to `php -i` also carries them (the discriminating
  fixture named below)
- **AND** no `Start` call is ever recorded (Decision 3), and no `CopyFrom` call is recorded — this
  run is Linux, not macOS

The `php -i` fixture used for this proof carries **both** `xdebug.remote_host` and
`xdebug.client_host`, so a v2 selection that wrongly also picked up the v3 key could not pass by
the v3 key merely being absent from the fixture.

Test: `TestWhatDebugOnAsks` (Linux, v2 case).

#### Scenario: Turning xdebug on with Xdebug v3

- **GIVEN** the same fake as above, this time answering the first `Capture` call with a v3 version
  string, and the second with the same dual-key `php -i` fixture
- **WHEN** `debug-on` runs
- **THEN** the same ordered call log holds, and the second `Capture` call's output is parsed for
  `xdebug.idekey`, `xdebug.client_host` and `xdebug.client_port` only — proving the v3 selection
  reads the `client_*` pair and never `xdebug.remote_host` or `xdebug.remote_port`, even though
  both are present in the same fixture

Test: `TestWhatDebugOnAsks` (v3 case).

#### Scenario: The restart used is Restart's own, a stop and a start — not Compose's

- **GIVEN** `restart.sh` is literally `stop.sh` then `start.sh`, so `commands.Restart` is the
  faithful port of `debug-on.sh:26` — unlike `debug-off`, whose shell twin called `$DOCKER_COMPOSE
  restart phpfpm` directly
- **WHEN** `debug-on` runs
- **THEN** the engine call recorded is `Restart(dir, ["phpfpm"])`, never `Compose([...,
  "restart", "phpfpm"])` — the opposite choice from `debug-off`'s own, made for the opposite
  reason: `debug-on`'s shell twin genuinely ran a stop then a start, and `Operator.Restart` is what
  reproduces that, including the worktree refusal `Operator.Restart` carries and `Compose` does
  not
- **AND** this is provable by construction, the same way `debug-off`'s spec pins the opposite
  choice: the choice of method is what the ordered call log's `Restart` entry pins, in the same
  test as the scenario above

Test: `TestWhatDebugOnAsks` (same test; this scenario is the log entry's own proof, not a separate
run).

#### Scenario: Turning xdebug on with no host to patch, on macOS

- **GIVEN** `XdebugHost` answers `("", nil)` — not applicable outside Linux, per
  `debug-on-host-detection` — and a fake engine substituted for the real one
- **WHEN** `debug-on` runs on macOS
- **THEN** the call log holds no host-patching `Exec` calls at all — the two `xdebug.ini` patches
  never fire against an empty host — but does hold, in order: the uncomment `Exec` against
  `docker-php-ext-xdebug.ini`, a `CopyFrom(["generated"])` call, and the `Restart` call
- **AND** the line `Copying generated code into host` is printed in warning colour before the
  `CopyFrom` call, written by the handler itself — `copyFrom()` prints nothing on success
  (verified), so nothing else would print it
- **AND** the eventual output's host is reported as absent (`null` in JSON, omitted in text) —
  see "Enabling xdebug in JSON" below

Test: `TestWhatDebugOnAsksOnMac`.

#### Scenario: Xdebug absent or unrecognized is reported, not a failure

- **GIVEN** two `php -v` fixtures: one carrying **no** Xdebug line at all — the case the shell
  script's own `set -euo pipefail` aborted on before its own `else` clause, written nine lines
  below, could ever run for it — and one carrying a version string that matches `Xdebug v` but
  neither `v2.` nor `v3.` (the only case the shell's `else` could reach)
- **WHEN** `debug-on` runs against either fixture
- **THEN** the enabling work already completed is recorded exactly as in the installed case — both
  host patches when a host was resolved, the uncomment edit, the macOS copy when applicable, and
  the `Restart` — and the command still exits `0`
- **AND** no second `Capture` call (`php -i`) is recorded — there are no keys to read back
- **AND** the printed text reports, between the same two separator lines the installed case
  prints, that Xdebug is not installed, in place of the `key=value` lines
- **AND** this is Decision 1, made observable for the first time: the shell's abort meant its own
  `else` branch could never fire for the absent case; in Go the branch is reachable and is chosen
  to be informative rather than fatal, because by this point the edits and the restart have
  already succeeded

Test: `TestWhatDebugOnAsksXdebugAbsent` (both fixtures, one test — the two inputs drive the same
branch; discriminating them at the parser level is a separate, package-level fixture test, not
duplicated here as two handler scenarios).

#### Scenario: The xdebug host cannot be resolved on Linux

- **GIVEN** a fake engine substituted for the real one, answering `XdebugHost` with `("",
  ErrNoBridgeAddress)` — a `docker0` that exists but could not be resolved
- **WHEN** `debug-on` runs on Linux
- **THEN** the fake's whole call log holds exactly two entries — `Resolve` and `XdebugHost` — no
  `Exec`, no `CopyFrom`, no `Restart` and no `Capture` call is recorded
- **AND** the command exits `exitDocker`, and the reported fault's `type` is `docker_failed`,
  carrying `XdebugHost`'s own error text — the same mapping `report()` gives any error that is not
  a `core.Refusal` elsewhere in this package, because the host lookup answers a plain error, not a
  refusal
- **AND** this is `debug-on-host-detection`'s Decision 2 (renumbered Decision 4 in this proposal),
  made observable for the first time: an unresolvable `docker0` is a failure, not a silent skip

Test: `TestDebugOnReportsAnUnresolvableHost`.

#### Scenario: An edit fails before the restart, with no Go-level error

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `debug-on` runs and one of its three edit `Exec` calls — either `xdebug.ini` host patch
  or the `docker-php-ext-xdebug.ini` uncomment, whichever comes first in the log — answers a
  non-zero status with no error
- **THEN** the fake records only the calls up to and including the one that failed — no further
  `Exec`, no `CopyFrom`, no `Restart`, and no `Capture` call is recorded, and in particular no
  `Start` call is ever recorded (Decision 3)
- **AND** the command's return value is that same status
- **AND** this is also the seam-visible half of the disclosed behaviour change: with the
  environment stopped, an edit's own `Exec` call is what answers non-zero, and `debug-on` now
  fails naming that step instead of starting the whole environment the way the shell script's
  `start_service_if_not_running.sh` probe did

Test: `TestAFailedDebugOnEditReportsTheContainersStatus` (shared with `magento-commands`'
scenario of the same fact, proved once).

#### Scenario: A Go-level error on an edit stops before the restart

- **GIVEN** a fake engine substituted for the real one
- **WHEN** one of the same three edit `Exec` calls answers an error rather than a non-zero status
- **THEN** no further call beyond it is recorded — no `CopyFrom`, no `Restart`, no `Capture` — and
  the failure is reported with the mapped Docker exit code

Test: `TestADebugOnEditErrorStopsBeforeTheRestart`.

#### Scenario: The restart refuses from an unregistered worktree

- **GIVEN** a fake engine substituted for the real one, answering `Restart` with the same
  `core.Refusal` an unregistered worktree gets elsewhere (`Kind: "blocked_in_worktree"`, `Code: 6`)
- **WHEN** `debug-on` runs from an unregistered worktree
- **THEN** the fake's whole call log ends at `Restart` — no `Capture` call is ever recorded
- **AND** the command exits `6`, and the reported fault's `type` is `blocked_in_worktree`
- **AND** this is new: `hm debug-on` has never refused from a worktree before this port, because
  `restart.sh`, called from inside `debug-on.sh`, never re-entered `bin/run`'s own gate, and
  `debug-on` is absent from `hm_alters_environment` (`worktree.sh:85-94`, verified) — so no shell
  invocation of `debug-on` was ever gated on it either (proposal Decision 5)

Test: `TestDebugOnRefusesFromAnUnregisteredWorktree`.

#### Scenario: The restart's other failures are reported as Docker failures

- **GIVEN** a fake engine substituted for the real one, answering `Restart` with a plain error, not
  a `core.Refusal`
- **WHEN** `debug-on` runs
- **THEN** the failure is reported through the same fault-mapping `report()` uses elsewhere —
  `exitDocker`, `error.type` `docker_failed` — and no `Capture` call is recorded

Test: `TestADebugOnRestartErrorIsReported`.

#### Scenario: A failed Capture is reported as a named step, not a raw status

- **GIVEN** a fake engine substituted for the real one, whose `Capture` outcome — either the first
  (`php -v`) or the second (`php -i`) — answers a non-zero status or an error
- **WHEN** `debug-on` runs
- **THEN** the failure is reported through `failure()`, naming the step (reading the Xdebug
  version, or reading its settings) rather than being returned as a raw, unwrapped status the way
  `debug-off`'s own restart status is
- **AND** when the first `Capture` call is the one that fails, no second `Capture` call is ever
  recorded
- **AND** this is a decision this spec makes in the absence of any shell precedent — `Capture` had
  no caller before this unit — following the same wrapped-with-a-named-step shape the sed edits
  already use, rather than the raw-passthrough shape `Compose`'s restart uses: a captured
  program's own exit code is not something a caller of `hm debug-on` is meant to branch on the way
  a Compose subcommand's is

Test: `TestAFailedDebugOnCaptureIsReported`.

#### Scenario: The xdebug host environment variable is not consulted

- **GIVEN** nothing in this repository sets `XDEBUG_HOST` except `set_xdebug_host_property.sh`
  itself (verified, unchanged by this port) — a shell-only escape hatch this port drops
- **WHEN** `debug-on` runs with `XDEBUG_HOST` set in the calling process's own environment
- **THEN** the recorded `XdebugHost` call — not an environment read — is still what decides the
  host used; the variable is not read anywhere on this path
- **AND** this is a disclosed behaviour change, not an oversight (proposal Decision 8): the escape
  hatch a developer could reach for by hand is gone, and the only way to change what address gets
  patched is whatever `XdebugHost()` resolves

Test: `TestDebugOnIgnoresXdebugHostEnvironmentVariable`.

#### Scenario: Enabling xdebug in JSON

- **GIVEN** a fake engine as in the v2 and v3 scenarios above
- **WHEN** `debug-on --json` succeeds
- **THEN** the document's `data` carries `{"xdebug": true, "version": <the verbatim php -v
  string>, "host": <the patched address, or null when nothing was patched>, "settings":
  {<the ini keys as actually read, unrenamed — xdebug.idekey/xdebug.remote_host/xdebug.remote_port
  on v2, xdebug.idekey/xdebug.client_host/xdebug.client_port on v3>}}`
- **AND** on macOS, or wherever `XdebugHost` answered no host, `host` is `null`, not an empty
  string — carrying `debug-on-host-detection`'s not-applicable/unresolved distinction outward
  rather than flattening it into one falsy value (proposal Decision 2)
- **AND** the `settings` object is never renamed to a version-independent shape — a v3 run's
  document has no `xdebug.remote_host` key and a v2 run's has no `xdebug.client_host` key, because
  normalising them would assert an equivalence this port never verified
- **AND** the envelope itself (`schema_version`, `command`, `ok`, `data`) is unchanged and remains
  `cli-output-contract`'s to govern — this scenario owns only these four keys

Test: `TestWhatDebugOnAsks` (JSON case, v2 and v3).

#### Scenario: Xdebug absent in JSON

- **GIVEN** Xdebug is absent or its version is unrecognized, as in the text scenario above
- **WHEN** `debug-on --json` succeeds
- **THEN** the document's `data` carries only `{"xdebug": false}` — no `version`, `host` or
  `settings` key at all, because nothing was read to report — mirroring `debug-off`'s own minimal
  `{"xdebug": false}` shape for the symmetric case

Test: `TestWhatDebugOnAsksXdebugAbsent` (JSON case).

#### Scenario: Getting logs

- **GIVEN** a fake engine substituted for the real one, whose `project.Root` is a directory
  distinct from the test's own working directory
- **WHEN** `logs` runs with one or more positional service names
- **THEN** the fake's call log holds, in order, a `Resolve` call and then a `Logs` call
- **AND** the `Logs` call's `Dir` equals the test's own working directory (read from `os.Getwd()`
  inside the test), **not** `f.project.Root` — `orchestrate.go:166-167` calls its engine method
  with `here()` again, the same way `execute` does, and the opposite of `inside()`, which
  `inside_test.go` asserts against `root`
- **AND** the positional arguments are recorded as the requested services, in the order given

Test: `TestWhatLogsAsks`.

#### Scenario: What logs' flags ask for

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `logs` runs with `-f`, `-t`, `--no-color`, `--no-log-prefix`, `--tail 50` (or `-n 50`,
  which sets the same field as `--tail`), `--since 1h`, `--until now`, and `--index 2`
- **THEN** the recorded `Logs` call carries follow, timestamps, no-color and no-log-prefix all on,
  the tail value from whichever of `--tail`/`-n` was given, since and until from their values, and
  the index as a number

Test: `TestWhatLogsAsks` (flag-mapping case).

#### Scenario: A failed logs call is reported as a Docker failure

- **GIVEN** the fake's second outcome — the `Logs` call, not the preceding `Resolve` call —
  carries an error
- **WHEN** `logs` runs
- **THEN** it exits with `exitDocker`, the reported fault's `type` is `docker_failed`, and the call
  log still ends at `Logs` — `Resolve` succeeded before it

Test: `TestLogsReportsAFailure`.

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

#### Scenario: Running something inside the php container (exec)

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `exec <command...>` runs against a resolved project
- **THEN** the fake's whole call log holds, in order, a `Resolve` call and then an `Exec` call
- **AND** the `Exec` call's `Dir` equals the test's own working directory (read from `os.Getwd()`
  inside the test), **not** `f.project.Root` — `orchestrate.go:188-189` calls its engine method
  with `here()`, the same way `execute` (via `logs`) already does, and the opposite of `inside()`,
  which `inside_test.go` asserts against `root`; `TestWhatCopyingIntoTheContainerAsks`
  (`wrappers_test.go:23-56`) is the working template to copy, with `fake.project.Root` set to a
  value no `t.TempDir()` can equal so the assertion discriminates
- **AND** the command's arguments are recorded verbatim as `Command`, `Service` is `phpfpm`
  (`phpService`), and `Options` equals exactly what calling `terminalOptions(user)` itself returns
  — never a literal `Tty: false` or `Tty: true` — because `terminalOptions` (`php.go:70-76`) reads
  `isTerminal(os.Stdin) && isTerminal(os.Stdout)` at call time

Test: `TestWhatExecAsks`.

#### Scenario: -r sets the root user for exec

- **WHEN** `exec` runs with `-r` first
- **THEN** it is consumed from the arguments before the command is built, and the recorded `Exec`
  call carries `Options` built from `terminalOptions("root")` instead of `terminalOptions("")`,
  while the remaining arguments become `Command` unchanged

Test: `TestWhatExecAsks` (root case).

#### Scenario: A failed exec status is returned without being mapped

- **GIVEN** the fake's `Exec` outcome answers a non-zero status with no error
- **WHEN** `exec` runs
- **THEN** the command's own return value is that same status (`orchestrate.go:190-192`), not
  passed through `report`'s `docker_failed` mapping

Test: `TestAFailedExecStatusIsReturnedAsIs`.

#### Scenario: An exec error is reported as a Docker failure

- **GIVEN** the fake's `Exec` outcome answers status `0` together with an error — the other half
  of `orchestrate.go:190-194`'s branch, distinct from the non-zero-status case above: `execute()`
  only returns `status` early when it is non-zero, so a zero status with a non-nil `err` falls
  through into `report`
- **WHEN** `exec` runs
- **THEN** it exits `exitDocker`, and the reported fault's `type` is `docker_failed`, and the call
  log still ends at `Exec` — `Resolve` succeeded before it

Test: `TestExecReportsAFailure`.

#### Scenario: Opening a shell (bash)

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `bash` runs against a resolved project
- **THEN** the fake's whole call log holds, in order, a `Resolve` call and then an `Exec` call
  whose `Command` is exactly `["bash"]`, through the same `execute()` handler `exec` uses, with
  `Dir` equal to the test's own working directory, the same as `exec`'s
- **AND** when `bash` runs with `-r` first, `shell()` (`tools.go:10-18`) forwards it to `execute()`
  as `["-r", "bash"]`, and the recorded `Exec` call's `Command` is still exactly `["bash"]`, while
  `Options` is built from `terminalOptions("root")` instead of `terminalOptions("")`

Test: `TestWhatBashAsks`.

#### Scenario: bash silently discards any argument other than -r

- **GIVEN** `shell()` (`tools.go:10-18`) only ever inspects `args[0]`, comparing it against `"-r"`
  and nothing else
- **WHEN** `bash` runs with an argument other than `-r` — for example `--nonsense`, or a positional
  word — as its first (and only) argument
- **THEN** the recorded `Exec` call's `Command` is still exactly `["bash"]` and `Options` is built
  from `terminalOptions("")`, exactly as with no argument at all: the argument is silently
  discarded rather than reaching the container or producing a usage error
- **AND** this is worth pinning precisely because it is surprising: unlike `exec`'s own argument
  handling, nothing in `bash`'s path refuses an argument it does not recognize

Test: `TestWhatBashAsks` (discarded-argument case).

#### Scenario: exec and bash's RED window is bounded by the working directory, not the daemon

- **GIVEN** `exec_test.go` and `tools_test.go`'s `bash` cases are written, strict-TDD style,
  against `Exec()` before `orchestrate.go:188` is routed through `newEngine`
- **WHEN** that RED half runs under `answering(t)`
- **THEN** `t.Chdir(t.TempDir())` (`fake_engine_test.go:274`) is what stops it — `Resolve` does not
  fail on a directory that is not a project, so an unrouted call proceeds into
  `orchestrator.Exec`'s own `open`/`load` (`orchestrator.go:166-215`), which refuses for want of a
  compose file under the temp cwd, before `service.Exec` runs and before any daemon dial happens
- **AND** `DOCKER_HOST=unix:///nonexistent` does not bound this window by itself — the missing
  compose file is what stops it first
- **AND** every subtest that exercises `exec` or `bash` runs under `answering(t)`, and none
  re-chdirs on its own

Test: `TestWhatExecAsks` and `TestWhatBashAsks` (asserted by construction: neither test file has a
subtest outside `answering(t)`, and a code comment beside the call names why).

#### Scenario: Anonymising the database when confirmed

- **GIVEN** a fake engine substituted for the real one, run under `answering(t)` so `ask()`
  answers its own suggestion (`"Y"`) without reading stdin
- **WHEN** `masquerade` runs against a resolved project
- **THEN** the fake's whole call log holds, in order, a `Resolve` call and then an `Anonymise`
  call, and the `Anonymise` call's `Dir` equals the test's own working directory (read from
  `os.Getwd()` inside the test), **not** `f.project.Root` — `tools.go:54` calls its engine method
  with `here()`, the same way `exec` does

Test: `TestWhatMasqueradeAsks`.

#### Scenario: Declining leaves the database untouched

- **GIVEN** `answering(t)` as usual (its other pins are still wanted, and `projectOr`'s `Resolve`
  must succeed against `f.project`), **plus** a nested `t.Setenv("HM_NON_INTERACTIVE", "")` that
  stacks over the helper's own pin and unwinds LIFO, **plus** `os.Stdin` swapped for a pipe
  carrying `"n\n"` — `ask()` (`ask.go:23-56`) checks `HM_NON_INTERACTIVE` first (`:29-35`) and
  answers the suggestion without reading stdin when it is set, so `answering(t)`'s own pin makes
  this branch structurally unreachable unless all three are present together; `askingWith`
  (`ask_test.go:25-60`) is the precedent, and strictly only `os.Stdin` needs to move, because
  `ask.go:43` writes the question to the real `os.Stderr`, not to a writer `masquerade()` was
  handed
- **WHEN** `masquerade` runs and the piped answer is anything other than `""` or `"y"`
  (case-insensitive, trimmed)
- **THEN** the fake's whole call log holds exactly one entry — the `Resolve` call — and no
  `Anonymise` call is recorded
- **AND** the command exits `exitOK`, because declining is not reported as a failure

Test: `TestADeclinedMasqueradeAsksOnlyToResolve`. A masquerade test whose log shows `Anonymise` in
this case, or that reaches `exitOK` without the `os.Stdin` swap, has not covered the branch that
keeps this tool from writing to a database — the empty half of the log past `Resolve` is the
assertion that matters, not the exit code alone.

#### Scenario: An Anonymise refusal is reported

- **GIVEN** the fake's second outcome — the `Anonymise` call, not the preceding `Resolve` call —
  carries an error
- **WHEN** `masquerade` runs, confirmed
- **THEN** the error is reported through the same fault-mapping `report()` uses elsewhere, and the
  call log still ends at `Anonymise` — `Resolve` succeeded before it

Test: `TestAFailedAnonymiseIsReported`.

#### Scenario: masquerade's RED window is bounded by the daemon, not the working directory

- **GIVEN** `tools_test.go`'s `masquerade` cases are written, strict-TDD style, against
  `Anonymise()` before `tools.go:54` is routed through `newEngine`
- **WHEN** that RED half runs under `answering(t)`
- **THEN** `DOCKER_HOST=unix:///nonexistent` (`fake_engine_test.go:272`) is what stops it —
  `Resolve` does not fail on a directory that is not a project, so an unrouted call proceeds
  through `app.Importer.Anonymise` (`app/anonymise.go:26-30`) into `Database.Ready`, which calls
  `Engine.Containers()` at `app/database.go:121-145` — a machine-wide `ContainerList` that dials
  the daemon regardless of the working directory
- **AND** `t.Chdir` does not bound this window the way it bounds `exec` and `bash` — the compose
  loader is never reached on this path
- **AND** every subtest that exercises `masquerade` runs under `answering(t)`, and none overrides
  `DOCKER_HOST`

Test: `TestWhatMasqueradeAsks` and `TestADeclinedMasqueradeAsksOnlyToResolve` (asserted by
construction: neither subtest overrides `DOCKER_HOST`, and a code comment beside the call names
why).
