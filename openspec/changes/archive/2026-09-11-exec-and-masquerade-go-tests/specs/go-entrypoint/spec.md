# Delta for go-entrypoint

## MODIFIED Requirements

### Requirement: A ported command's engine interaction is provable without Docker

For a command listed in `internal/cli/run.go`'s `Run()` switch, the repository SHALL carry a Go
test that verifies what its handler asks of the engine, through a fake substituted for the real
one, requiring no Docker daemon, no network access, and no project beyond what the test itself
builds.
(Previously: covered `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`,
`purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`,
`set-host`, `list`, `describe`, and `logs`. This adds `bash`, `exec`, and `masquerade` — the
three commands `data/command_descriptions.json` declares `"safety": "dangerous"` and deferred by
`read-only-commands-go-tests` — and the seam gains one method, `Anonymise(dir string) error`.
`Exec` was already on `commands` and already exercised through `inside()`, so `bash` and `exec`
cost only a routing line: `shell()` (`tools.go:10-18`) only ever calls `execute()`, so one line at
`orchestrate.go:188` puts both on the seam.)

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

### Requirement: A usage error returns before any engine call

`copy-to-container`, `copy-from-container`, `mysqldump`, `version`, `set-host`, `list`,
`describe`, `logs`, `doctor`, `exec`, and `masquerade` SHALL validate their arguments before they
ask anything of the engine.
(Previously: covered `copy-to-container`, `copy-from-container`, `mysqldump`, `version`,
`set-host`, `list`, `describe`, `logs`, and `doctor`. This adds `exec` and `masquerade`. `bash` has
no usage error of its own — it always builds a command and so needs no scenario here.)

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

Test: `TestDoctorRefusesAnUnknownOption`. No fake or seam is needed: the refusal returns during
parsing, the same shape `down_test.go:11-18` already established.

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

#### Scenario: exec with no command

- **WHEN** `exec` runs with no command argument (after any `-r` is consumed)
- **THEN** it fails with `missing_command`, and the fake engine used by its test records zero
  calls

Test: `TestExecRefusesWithNoCommand`.

#### Scenario: masquerade with a dashed argument

- **WHEN** `masquerade` runs with any argument that starts with a dash
- **THEN** it fails with `invalid_argument`, and the fake engine used by its test records zero
  calls

Test: `TestMasqueradeRefusesADashedArgument`.

## Note

Every `Test:` name in this delta is the spec's own choice, not the design's: `internal/cli` is one
Go package, and Go forbids two test functions sharing a name in it, so each name below was picked
here to avoid collision, and the design was reconciled afterward to adopt these same names
verbatim rather than the other way around. `TestAnArgumentNobodyDeclaredIsAUsageError`
(`version_test.go:128`), `TestADownOptionNobodyDeclaredIsAUsageError` (`down_test.go:250`), and
`TestASetHostOptionNobodyDeclaredIsAUsageError` (`set_host_test.go:190`) already occupy the
`…IsAUsageError` family and already contort their prefixes to stay distinct from each other —
that family has exactly three members today. A fourth member is one collision closer to a build
failure over a name, which is why every test added here uses a command-prefixed, verb-first name
instead: `TestWhatExecAsks`, `TestWhatBashAsks`, `TestWhatMasqueradeAsks`,
`TestADeclinedMasqueradeAsksOnlyToResolve`, `TestAFailedExecStatusIsReturnedAsIs`,
`TestExecReportsAFailure`, `TestAFailedAnonymiseIsReported`, `TestExecRefusesWithNoCommand`, and
`TestMasqueradeRefusesADashedArgument` — none of these joins the `…IsAUsageError` family, the same
choice `list`, `describe`, and `logs`'s own refusal tests already made. `bash`'s discarded-argument
scenario needs no new name at all: it is a case of `TestWhatBashAsks`, already listed above.

Five decisions constrain how these tests must be written, not just what they must prove:

- **`Dir` is `here()`, not the project root, for both handlers this delta adds** (Decision 4).
  `execute()` (`orchestrate.go:188-189`, serving both `exec` and `bash`) and `masquerade()`
  (`tools.go:54`) both call the engine with `here()`. `inside()` does the opposite, and
  `inside_test.go` asserts `Dir: root` accordingly — copying that neighbouring precedent produces
  a green test asserting the wrong value; this is the same divergence `logs`, `CopyInto`, and
  `CopyFrom` already carry. `TestWhatCopyingIntoTheContainerAsks` (`wrappers_test.go:23-56`) is the
  working template: it sets `fake.project = core.Project{Name: "shop", Root: "/code/shop"}`,
  captures `cwd := here()`, and asserts `Dir: cwd`, with a root that differs from any `t.TempDir()`
  so the assertion discriminates instead of passing by coincidence — copy that one, not
  `inside_test.go`.
- **The decline path is the point, and the standard helper cannot reach it on its own** (Decision
  2). `answering(t)` pins `HM_NON_INTERACTIVE=1`, and `ask()` (`ask.go:23-56`) checks that first
  (`:29-35`) and answers its suggestion without reading stdin — so `masquerade`'s decline branch is
  structurally unreachable under the standard helper alone. A decline test needs `answering(t)`
  as usual, **plus** a nested `t.Setenv("HM_NON_INTERACTIVE", "")` that stacks over the helper's
  own pin and unwinds LIFO, **plus** `os.Stdin` swapped for a pipe carrying `"n\n"` — `askingWith`
  (`ask_test.go:25-60`) is the precedent and transfers almost unchanged; strictly only `os.Stdin`
  needs to move. The empty call log past `Resolve` is the assertion that matters, not the exit
  code alone.
- **Two pins, two different hazards, neither covers the other** (Decision 3). `t.Chdir(t.TempDir())`
  (`fake_engine_test.go:274`) is what stops an unrouted `exec`/`bash` — the missing compose file
  under the temp cwd refuses inside `composelib.Orchestrator.load` before any dial happens.
  `DOCKER_HOST=unix:///nonexistent` (`fake_engine_test.go:272`) is what stops an unrouted
  `masquerade` — it reaches `Database.Ready`'s `Engine.Containers()`, a machine-wide
  `ContainerList`, regardless of cwd. Neither pin stands in for the other, the same way `list`'s
  and `describe`'s daemon pin does not extend to `logs`.
- **`Options` must be built by calling `terminalOptions(user)`, never written as a literal `Tty:`
  value.** `terminalOptions` (`php.go:70-76`) computes `Tty` from `isTerminal(os.Stdin) &&
  isTerminal(os.Stdout)` at call time, so a hardcoded expectation goes red depending on how
  `go test` is invoked, and worse once a neighbouring `masquerade` subtest swaps `os.Stdin` for a
  pipe.
- **`masquerade`'s `input_required` branch (`tools.go:37-40`) stays a code comment, not a test.**
  `ask()` only errors when the suggestion is empty, and this call site passes `"Y"`, so the branch
  is unreachable through this handler; the test file notes this rather than asserting it.

This delta changes nothing under `console/`, `bin/run`, `data/`, or `dockergento/` beyond what the
proposal already lists as in scope; no shell twin is deleted. `commands` grows to exactly sixteen
methods (Previously: fifteen), the new one being `Anonymise(dir string) error`. Exactly two call
sites route through `newEngine` in this change — `orchestrate.go:188` and `tools.go:54` — no other
`engine(` site changed, in particular not `orchestrate.go`'s own `checkServices`.
