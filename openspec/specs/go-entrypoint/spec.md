# go-entrypoint Specification

## Purpose
TBD - created by archiving change go-skeleton-and-bridge. Update Purpose after archive.
## Requirements
### Requirement: Everything not ported runs unchanged

The binary SHALL execute the shell implementation for every command it does not implement,
passing the arguments through untouched and returning its exit code.

#### Scenario: A command the binary does not implement

- **WHEN** the user runs any command through the binary
- **THEN** the shell implementation runs it and its output is identical to running it directly

#### Scenario: The exit code

- **WHEN** the shell implementation exits with a usage error, a refusal or a success
- **THEN** the binary exits with the same code, because they are a contract that callers branch on

#### Scenario: The terminal

- **WHEN** a command asks a question or streams output
- **THEN** it reads and writes the same terminal as it would without the binary

#### Scenario: The shell implementation cannot be found

- **WHEN** the binary cannot locate the shell tree
- **THEN** it says so and exits with the code for something wrong with the tool itself

### Requirement: The project is resolved by the Go layer

The binary SHALL resolve which project a directory belongs to, from the root that ends up being
used.

#### Scenario: A main checkout

- **WHEN** the directory is a project
- **THEN** its name, root, domain, Magento directory and topology are reported from its own
  properties

#### Scenario: A worktree with no environment of its own

- **WHEN** the directory is a git worktree with no registered environment
- **THEN** it resolves against the main checkout, which is what the refusals depend on

#### Scenario: A worktree with an environment

- **WHEN** the directory is a registered worktree
- **THEN** it resolves against itself: its own root, its own properties, and a project name and
  address derived from the parent's

#### Scenario: A directory that is not a project

- **WHEN** the directory has no properties
- **THEN** that is reported as an empty project rather than as an error, because several commands
  run outside one

#### Scenario: Checked against the shell implementation

- **WHEN** both implementations resolve the same project
- **THEN** they agree on the name, the root, the domain and the Magento directory

### Requirement: Saying which binary this is

The binary SHALL be able to report itself and what it resolved, without changing what any real
command does.

#### Scenario: The build

- **WHEN** the user runs `hm-go-version`
- **THEN** the version and the revision it was built from are printed

#### Scenario: The resolution

- **WHEN** the user runs `hm-go-project`
- **THEN** what the Go layer resolved is printed as JSON

#### Scenario: They cannot collide

- **WHEN** the names of these commands are compared with the shell implementation's
- **THEN** none of them is a command the shell implementation has

### Requirement: Installing the binary

The release SHALL publish a binary per platform, and the installation SHALL make it the command,
without taking the shell implementation away from a machine that cannot get it.

#### Scenario: An installation

- **WHEN** the tool is installed
- **THEN** the binary for that platform and architecture is fetched into the checkout and the
  command in the path points at it

#### Scenario: A machine that cannot fetch it

- **WHEN** the binary cannot be downloaded
- **THEN** the command points at the shell implementation and says so, rather than leaving the
  machine without a tool

#### Scenario: Finding the shell tree from where it is installed

- **WHEN** the binary is invoked through the symlink an installation leaves in the path
- **THEN** it finds the shell tree beside it, whatever directory it was invoked from

### Requirement: The state of the migration is written down and true

The repository SHALL carry a document saying where the migration is, and a test SHALL keep it
from drifting in both directions: every row the document marks `go` SHALL exist in the Go tree,
and every command the Go tree fully answers for SHALL be marked `go` in the document.
(Previously: only the forward direction was checked — rows marked `go` had to exist in the Go
tree. A command fully wired in Go but still tabled `shell` passed the suite undetected.)

#### Scenario: Every command is accounted for

- **WHEN** the document is compared with the command declarations
- **THEN** every command appears exactly once, with an owner

#### Scenario: A claim that is not true

- **WHEN** the document says a command is implemented in Go and nothing in the Go tree answers
  for it
- **THEN** the suite fails

#### Scenario: Carrying on later

- **WHEN** somebody opens the document
- **THEN** it says how to build it, how to test it, and which phase is next

#### Scenario: A command is fully wired but still tabled as shell

- **GIVEN** a `case "<command>":` label in `internal/cli/run.go`'s `Run()` switch, other than the
  `_registry` label
- **WHEN** the document tables that command as `shell`
- **THEN** the suite fails and names the command

#### Scenario: A command routed by more than one implementation

- **GIVEN** a command whose subcommands are split between the Go and shell implementations
  (currently `db` and `proxy`)
- **WHEN** the reverse check runs
- **THEN** that command is excluded through an explicit, enumerated exception — never a silent
  skip — and its exclusion is never reported as drift while only some of its subcommands are
  routed

#### Scenario: The counter and the table agree

- **WHEN** the document's declared Go count is compared against rows marked `go`
- **THEN** they match, and the total matches the number of documented commands

#### Scenario: The reconciliation touches no runtime code

- **GIVEN** this fix reconciles the document and the test against `run.go` as the single source
  of truth
- **WHEN** the resulting change is diffed against its base branch
- **THEN** no file under `console/`, `bin/run`, `internal/`, or `dockergento/` is modified — only
  the test file and the affected documents change

### Requirement: A command whose subcommands are ported one family at a time

The tool SHALL let a command be answered by either implementation depending on its subcommand,
and SHALL answer identically whichever one runs.

#### Scenario: A ported subcommand

- **WHEN** a subcommand that has been ported is asked for
- **THEN** it is answered by the ported implementation, with the same documents, tables, refusals
  and exit codes as before

#### Scenario: One that has not

- **WHEN** a subcommand that has not been ported is asked for
- **THEN** it reaches the shell implementation unchanged

### Requirement: Documentation does not overstate what remains unported

The documents that describe this repository's implementation SHALL NOT claim the implementation
is entirely or 100% Bash once any command is wired in Go.

#### Scenario: Describing how a command reaches its implementation

- **WHEN** a document explains the routing between the two implementations
- **THEN** it describes the strangler pattern: the `cmd/hm` Go binary as the entry point,
  `bin/run` as the bridge for what is not yet ported, and `internal/cli/run.go`'s switch as what
  decides between them

#### Scenario: No absolute claim remains

- **WHEN** `CLAUDE.md` and `architecture/02-cli-architecture.md` are read
- **THEN** neither states or implies the implementation is entirely or 100% Bash


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

### Requirement: The test seam changes nothing a real invocation runs

Adding a way to observe a handler's engine calls SHALL NOT change what production runs.

#### Scenario: Production still builds the real engine

- **WHEN** the binary runs any command outside a test
- **THEN** it is served by the real engine, unchanged, and a compile-time check confirms the
  real engine satisfies the interface the tests substitute

#### Scenario: Nothing else moves

- **WHEN** `go build ./...`, `go test ./... -short`, and `tests/run.sh unit` run after the seam
  is added
- **THEN** all three pass, and the Bash parity suites are unmodified

### Requirement: The migration document's truthfulness check does not depend on a pipe's producer finishing

`tests/unit/migration_status_test.sh`'s comparisons SHALL NOT read their result from a pipeline
whose early-exiting consumer can send its producer a broken pipe.

#### Scenario: A match found before the input is exhausted

- **WHEN** the comparison finds what it is looking for before its producer has written
  everything
- **THEN** the suite still reports the comparison's own result, not a broken-pipe exit code

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

### Requirement: A comment about what stays in shell names its real reason

A comment describing why part of a ported command still runs in shell SHALL name the actual
reason, not a stale one.

#### Scenario: The mac Composer vendor mirror

- **WHEN** the comment above `mirrorsVendor` in `internal/cli/php.go` is read
- **THEN** it says the vendor-mirror flow is what stays in shell, not that `copy-to-container`
  is unported — `copy-to-container` is wired in `run.go`

