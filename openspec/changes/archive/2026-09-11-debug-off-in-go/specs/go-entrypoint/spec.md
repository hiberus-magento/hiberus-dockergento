# Delta for go-entrypoint

## MODIFIED Requirements

### Requirement: A ported command's engine interaction is provable without Docker

For a command listed in `internal/cli/run.go`'s `Run()` switch, the repository SHALL carry a Go
test that verifies what its handler asks of the engine, through a fake substituted for the real
one, requiring no Docker daemon, no network access, and no project beyond what the test itself
builds.
(Previously: covered `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`,
`purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`,
`set-host`, `list`, `describe`, `logs`, `bash`, `exec`, and `masquerade`. This adds `debug-off`,
the eighteenth, which needs no new seam method: `Exec` and `Compose` were already on `commands`,
already exercised by `varnish-on`/`varnish-off` and `docker-compose` respectively.)

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

#### Scenario: Clearing generated code

- **WHEN** `purge` runs against a resolved project
- **THEN** the fake records one shell command, run as the web user, that removes exactly the
  generated paths the command lists

#### Scenario: Running the front-end package manager

- **WHEN** `npm <args>` runs
- **THEN** the fake records a call into the container that passes those arguments through
  verbatim to `npm`

#### Scenario: Running n98-magerun

- **WHEN** `n98-magerun <args>` runs
- **THEN** the fake records a call that runs `n98-magerun` with the arguments joined into one
  command, through a shell

#### Scenario: Running the unit suite

- **WHEN** `test-unit <args>` runs
- **THEN** the fake records a call that runs `phpunit` against the unit suite's configuration,
  with the arguments appended
- **AND** when the project defines no bin directory, the fallback bin directory is used

#### Scenario: Running the integration suite

- **WHEN** `test-integration <args>` runs
- **THEN** the fake records a call that changes into the integration suite's own directory, then
  runs `phpunit` against its configuration from the resolved working directory and bin
  directory, with the arguments appended
- **AND** when the project defines no working directory, the fallback working directory is used

#### Scenario: Writing a database dump

- **WHEN** `mysqldump <path>` runs against a resolved project
- **THEN** the fake records a request for a dump at that path from the current directory
- **AND** on success the command answers `{"path": <path>}` in JSON
- **AND** when the fake refuses the request, the command exits with the code reserved for
  engine/Docker failures

#### Scenario: Reporting what is installed

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `version` runs, with no project resolved
- **THEN** the fake records exactly one call, asking what is installed and what tooling is present
- **AND** in JSON, the document carries the installation fields (`version`, `tag`, `commits_ahead`,
  `commit`, `branch`, `detached`, `dirty`, `path`), the docker tooling fields (`docker.version`,
  `docker.compose`, `docker.compose_command`), and `binary` from the binary's own build
  information, independent of the fake
- **AND** in text, an empty tag or commit prints "unknown", and an empty docker or compose value
  prints "not available"

#### Scenario: Pointing a domain at this machine

- **WHEN** `set-host <domain>` runs against a resolved project
- **THEN** the fake records a call asking to point that domain here, with the database flag on
- **AND** `--no-database` records the same call with the database flag off
- **AND** in JSON, the document carries `{"domain": <domain>, "database": <flag>}`
- **AND** a refusal returned by the engine is reported with its own exit code, message and hint,
  rather than as a generic Docker failure

#### Scenario: Removing a domain without resolving a project

- **WHEN** `set-host --remove <domain>` runs
- **THEN** no project is resolved first, and the fake records a call asking only to remove that
  domain
- **AND** in JSON, the document carries `{"removed": <domain>}`

#### Scenario: Asking to stop everything

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `docker-stop-all` runs against a resolved project
- **THEN** the fake records exactly one request to stop everything for the resolved directory,
  carrying whether the run is interactive
- **AND** whatever the fake answers back — a decline, a refusal, or a stop — is reported without
  the handler making any other request

#### Scenario: Passing a Compose subcommand through

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `docker-compose <args>` runs against a resolved project
- **THEN** the fake records exactly one request to run Compose for the resolved directory, with
  the arguments passed through verbatim
- **AND** the command exits with the code the fake reports back

#### Scenario: Listing every environment on the machine

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `list` runs with no argument
- **THEN** the fake's whole call log holds exactly one entry — an `Environments` call — with no
  `Resolve` before it, because `Environments()` takes no directory and `list.go` never calls
  `projectOr`

Test: `TestWhatListAsks`.

#### Scenario: `list` and `describe` both reach the daemon during their own RED window

- **GIVEN** `list_test.go` and `describe_test.go` are written, strict-TDD style, against
  `Environments()` and `Describe()` before `list.go:23` and `describe.go:25` are routed through
  `newEngine`
- **WHEN** that RED half of each suite runs under `answering(t)`
- **THEN** `answering(t)`'s `DOCKER_HOST=unix:///nonexistent` pin (`fake_engine_test.go:272`) is
  what stops **both** from reaching a real daemon — `Resolve` does not fail on a directory that is
  not a project (`dockergento.go:128-131`), so an unrouted `describe` proceeds into
  `app.Describer.Describe`'s seven concurrent questions before any error is inspected, and two of
  them — `Engine.Containers()` and `Tooling.Xdebug(project.Name)` — reach the daemon exactly as
  `list`'s `Environments()` does
- **AND** `logs` needs no daemon pin for its own RED window: its unrouted path stops at the Compose
  loader, below where any dial happens
- **AND** every subtest that exercises `list` or `describe` runs under `answering(t)`, and none
  overrides `DOCKER_HOST`

Test: `TestWhatListAsks` and `TestWhatDescribeAsks` (asserted by construction: neither test file
has a subtest outside `answering(t)`, and a code comment beside each call names why).

#### Scenario: What list prints as text

- **GIVEN** a fake engine answering with a fixed `[]core.Environment`, including one orphaned
  entry, one with `HasMetadata` false, one with an empty branch and root, and one with a worktree
- **WHEN** `list` runs
- **THEN** the text table shows one row per environment, marking the orphan with "⚠ orphan",
  marking the metadata-less one with "(no metadata)", printing "-" for the empty branch and root,
  and composing the worktree row's name as "name (wt: branch)"

Test: `TestWhatListPrints`.

#### Scenario: An empty list of environments

- **GIVEN** the fake answers `Environments` with zero entries
- **WHEN** `list` runs with no `--json`
- **THEN** the text says no environments were found and suggests `hm setup`

Test: `TestWhatListPrints` (empty-inventory case).

#### Scenario: What list reports as JSON

- **GIVEN** a fake engine answering with a fixed set of two environments
- **WHEN** `list --json` runs
- **THEN** the document decodes to `{"environments": [...], "count": 2}` with the same data,
  compared against a literal, not a re-marshalled fixture

Test: `TestWhatListReportsAsJSON`.

#### Scenario: A failed list is reported as an unreachable daemon

- **GIVEN** the fake's `Environments` outcome carries an error
- **WHEN** `list` runs, in both text and `--json`
- **THEN** it exits with `exitDocker`, and the reported fault's `type` is `docker_unavailable`

Test: `TestListReportsAnUnreachableDaemon`.

#### Scenario: Describing a project

- **GIVEN** a fake engine substituted for the real one
- **WHEN** `describe` runs
- **THEN** the fake's whole call log holds exactly one entry — a `Describe` call, with `Dir` equal
  to the test's own working directory and `WithSecrets` false — and no `Resolve` call precedes it,
  because `describe.go` never calls `projectOr`, so the fake's `project` field plays no part

Test: `TestWhatDescribeAsks`.

#### Scenario: describe --with-secrets

- **WHEN** `describe --with-secrets` runs
- **THEN** the recorded `Describe` call carries `WithSecrets` true instead of false, and nothing
  else about the log changes

Test: `TestWhatDescribeAsks` (with-secrets case).

#### Scenario: What describe prints as text

- **GIVEN** a fake engine answering with a fixed `core.Description` covering the project (name,
  root, domain, worktree, status), URLs, Magento version and mode, xdebug, services, and paths,
  with no credentials, and with an empty version, mode, and some empty URL entries
- **WHEN** `describe` runs
- **THEN** the text rendering shows, in order, the Environment, URLs, Magento, Services and Paths
  sections, using `orDash` for the empty status, domain or worktree, `orUnknown` for the empty
  Magento version and mode, skips the empty URL entries rather than printing blank lines, and shows
  no Credentials section

Test: `TestWhatDescribePrintsAsText`.

#### Scenario: describe --with-secrets prints credentials in text

- **GIVEN** the same fixed description, this time with `Credentials` populated
- **WHEN** `describe --with-secrets` runs
- **THEN** the text rendering adds a Credentials section with the database name, user, password
  and root password

Test: `TestWhatDescribePrintsAsText` (with-secrets case).

#### Scenario: What describe reports as JSON

- **GIVEN** the same fixed `core.Description`, with `Credentials` nil
- **WHEN** `describe --json` runs
- **THEN** the document decodes to the same fields nested under `description`, compared against a
  literal, and carries **no** `credentials` key at all — not a null one

Test: `TestWhatDescribeReportsAsJSON`.

#### Scenario: describe --with-secrets reports credentials in JSON

- **GIVEN** the same fixed description, this time with `Credentials` populated by fixture values
- **WHEN** `describe --with-secrets --json` runs
- **THEN** the document carries the `credentials` object with those values

Test: `TestWhatDescribeReportsAsJSON` (with-secrets case).

#### Scenario: A failed describe is reported as an unresolvable project

- **GIVEN** the fake's `Describe` outcome carries an error
- **WHEN** `describe` runs
- **THEN** it exits with `exitProject`, the reported fault's `type` is `project_not_configured`,
  and the message includes the engine's own error text rather than swallowing it

Test: `TestDescribeReportsAnUnresolvableProject`.

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
