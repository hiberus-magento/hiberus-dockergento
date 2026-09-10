# Delta for go-entrypoint

## ADDED Requirements

### Requirement: A refusal's hint names an invocation the reader can run

When `list`, `describe`, or `doctor` refuse an argument or report a failure, the hint SHALL name an
invocation that runs, and SHALL be built from `binaryName()` so `COMMAND_BIN_NAME` is honoured.
`cli-output-contract:42-45` only requires that a `hint` field exist; it does not require the field
to name something runnable, which is what left four sites hardcoding a hint that pointed straight
back into the same refusal.

#### Scenario: list's hint names a real invocation

- **GIVEN** `list` takes no options at all
- **WHEN** `list` runs with an unknown argument, including `--help`
- **THEN** the hint reads `<binary> list`, built from `binaryName()` — not the previous
  `hm list --help`, which fed the reader back into the identical refusal
- **AND** under `COMMAND_BIN_NAME=hd`, the hint reads `hd list`

Test: `TestListRefusesAnyArgument`.

#### Scenario: describe's hint names its one real option

- **WHEN** `describe` runs with an unknown option, including `--help`
- **THEN** the hint reads `<binary> describe [--with-secrets]`, naming the one option `describe`
  actually takes
- **AND** under `COMMAND_BIN_NAME=hd`, the hint reads `hd describe [--with-secrets]`

Test: `TestDescribeRefusesAnUnknownOption`.

#### Scenario: describe's project_not_configured hint matches engine.go's

- **GIVEN** the engine's `Describe` call fails
- **WHEN** `describe` reports `project_not_configured`
- **THEN** the hint reads `<binary> setup` — the same hint `engine.go:88`'s `projectOr` emits for
  the identical refusal three files away — rather than a hardcoded `"hm setup"`
- **AND** under `COMMAND_BIN_NAME=hd`, the hint reads `hd setup`

Test: `TestDescribeReportsAnUnresolvableProject` (the fix ships in unit 0; this is the unit-2 proof,
because reaching this branch needs the fake).

#### Scenario: doctor's hint names its real option

- **WHEN** `doctor` runs with an unknown option, including `--help`
- **THEN** the hint reads `<binary> doctor [--only=ports]` — `ports` is a real check id — rather
  than `--help`
- **AND** under `COMMAND_BIN_NAME=hd`, the hint reads `hd doctor [--only=ports]`

Test: `TestDoctorRefusesAnUnknownOption`. `doctor` gets no engine-seam test in this cycle; this is
its only Go test.

#### Scenario: --help stays an unknown option, not a per-command flag

- **GIVEN** `hm --help` is a tool-level flag resolved before command dispatch
- **WHEN** `list --help`, `describe --help`, or `doctor --help` run
- **THEN** each still refuses as an unknown option with the corrected hint — this requirement only
  stops the hint from naming an invocation that fails the same way; it does not give `--help`
  per-command meaning

No test — a boundary statement about scope, not new behavior. The shell twins are unaffected and
still say `--help` in their own hint; `go_passthrough_test.sh` asserts only the exit code for these
three refusals, so the divergence is harmless until the deletion cycle removes the twins.

## MODIFIED Requirements

### Requirement: A ported command's engine interaction is provable without Docker

For a command listed in `internal/cli/run.go`'s `Run()` switch, the repository SHALL carry a Go
test that verifies what its handler asks of the engine, through a fake substituted for the real
one, requiring no Docker daemon, no network access, and no project beyond what the test itself
builds.
(Previously: covered `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`,
`purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`, and
`set-host`. This adds `list`, `describe`, and `logs` — the three commands
`data/command_descriptions.json` declares `"safety": "safe"` — and the seam gains the three
methods each of them needs: `Environments()`, `Describe(dir, withSecrets)`, and
`Logs(dir, services, options)`.)

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
- **THEN** `answering(t)`'s `DOCKER_HOST=unix:///nonexistent` pin (`fake_engine_test.go:218`) is
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

#### Scenario: What still covers the unknown-service refusal

- **GIVEN** `logs`' unknown-service validation (`orchestrate.go:197-221`) calls `Configuration`,
  which the seam deliberately does not carry (Decision 1) — against a fake project with a
  fictitious root, `checkServices` cannot load compose YAML, treats that as "skip validation", and
  the `unknown_service` branch never fires in an `internal/cli` test
- **WHEN** the same refusal is asked for through the built binary instead of the fake
- **THEN** `tests/integration/logs_launch_version_test.sh:148-153` proves it: it runs the Go
  binary, asserts exit 5, and asserts the refusal names `phpfpm`, sitting above the `docker info`
  gate at `:159` so it needs no daemon
- **AND** `tests/integration/go_orchestration_test.sh:176-181` additionally compares the Go and
  shell refusals as sorted JSON — this comparison is lost once `logs.sh` is deleted, so the cycle
  that deletes it inherits both `Configuration` and this comparison

Covered by (existing, unmodified): `tests/integration/logs_launch_version_test.sh`,
`tests/integration/go_orchestration_test.sh`. No `internal/cli` test asserts this branch.

### Requirement: A usage error returns before any engine call

`copy-to-container`, `copy-from-container`, `mysqldump`, `version`, `set-host`, `list`,
`describe`, `logs`, and `doctor` SHALL validate their arguments before they ask anything of the
engine.
(Previously: covered `copy-to-container`, `copy-from-container`, `mysqldump`, `version`, and
`set-host`. This adds `list`, `describe`, `logs`, and `doctor` — `doctor` gets no engine-seam test
elsewhere in this cycle, so this refusal is its only Go test.)

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

Test: `TestListRefusesAnyArgument`.

#### Scenario: describe with an unknown option

- **WHEN** `describe` runs with an option other than `--with-secrets`
- **THEN** it fails with a usage error, and the fake engine used by its test records zero calls

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

Test: `TestLogsRefusesAnUnknownOption`.

#### Scenario: logs missing a value

- **WHEN** `logs` runs with `--tail`, `-n`, `--since`, `--until`, or `--index` as its last
  argument, with nothing after it
- **THEN** it fails with `missing_value`, and the fake engine used by its test records zero calls

Test: `TestLogsRefusesAFlagWithNoValue`.

#### Scenario: logs with a non-numeric --index

- **WHEN** `logs --index abc` runs
- **THEN** it fails with `invalid_argument`, and the fake engine used by its test records zero
  calls

Test: `TestLogsRefusesANonNumericIndex`.

## Note

Every `Test:` name in this delta is the design's, not an independent choice: `internal/cli` is one
Go package, and Go forbids two test functions sharing a name in it. `TestAnArgumentNobodyDeclaredIsAUsageError`
(`version_test.go:128`), `TestADownOptionNobodyDeclaredIsAUsageError` (`down_test.go:250`), and
`TestASetHostOptionNobodyDeclaredIsAUsageError` (`set_host_test.go:190`) already occupy the
`…IsAUsageError` family and already contort their prefixes to stay distinct from each other. A
fourth, fifth, and sixth member in that family is one collision closer to a build failure over a
name, which is why `list`, `describe`, and `logs`'s refusal tests are named
`TestListRefusesAnyArgument`, `TestDescribeRefusesAnUnknownOption`, and the three
`TestLogsRefuses…` names instead — command-prefixed, verb-first names that cannot collide by
construction. `list` refusing any argument is exactly `version`'s situation, and
`TestListRefusesAnyArgument` says so under a name that can coexist with
`TestAnArgumentNobodyDeclaredIsAUsageError` rather than repeat it.

Four decisions from the design constrain how these tests must be written, not just what they must
prove:

- **The hardcoded hints are fixed first, in their own unit, before any test can cement them**
  (Decision 6 — see the new requirement above). Placed later, `list_test.go` and `describe_test.go`
  would ship assertions pinning the circular `--help` hint, and the fix would then have to edit
  tests written days earlier to say something else.
- **`Configuration` stays off the seam** (Decision 4 / proposal Decision 1). Because of it, a
  `logs <service>` test still reaches the real `engine()` inside `checkServices`, which fails to
  load compose YAML at the fake project's fictitious root and skips validation — deterministic
  only while no test creates a compose file under `f.project.Root`. `logs_test.go` must say so in
  a comment, not leave it to be rediscovered.
- **`list` and `describe` share the same RED-window net, and it is not "safe by construction"**
  (Decision 2). `Resolve` does not fail on a directory with no project — it returns a nameless
  project with a nil error — so an unrouted `describe` reaches the daemon through
  `Engine.Containers()` and `Tooling.Xdebug()` exactly as an unrouted `list` reaches it through
  `Environments()`. `DOCKER_HOST=unix:///nonexistent` is what protects **both**. `logs` is the one
  genuinely safe by construction: its unrouted path stops at the Compose loader, below any dial.
  Each unit's RED has exactly one legitimate failure signature (a hint-equality diff for unit 0; an
  `exitOK`-vs-expected-refusal exit code for units 1–3) — any other signature, in particular a
  call-log diff at `exitOK`, means a pin did not hold and the run must stop for investigation
  rather than be repeated.
- **`logs`' `Dir` is the test's cwd, not `f.project.Root`** (Decision 3). `orchestrate.go:166-167`
  calls `Logs` with `here()`, the same as `execute`, not with the project the fake resolved.
  Copying `inside_test.go`'s `Dir: root` pattern verbatim would silently assert the wrong value;
  the expected `Dir` must be read from `os.Getwd()` inside the test itself.

This delta changes nothing under `console/`, `bin/run`, `data/`, or `dockergento/`; no shell twin
is deleted, and the shell twins keep their own `--help` hint text even where Go's now differs.
`commands` grows to exactly fifteen methods, and `orchestrate.go:200`'s call to the concrete
`engine(` — the one behind `Configuration` — is unchanged. Unit 0 (the hint fix) is the only unit
that changes production behavior; units 1–3 stay test-only.
