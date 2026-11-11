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

Once a command is proved in Go through the engine-interaction test seam (the twenty-one retired in
this change: `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`, `purge`,
`npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`, `set-host`,
`list`, `describe`, `logs`, `debug-on`, `debug-off`, `bash`, `magento`, `docker-compose`,
`restart`), the shell tree SHALL NOT carry an implementation for it, and the shell entry point
SHALL answer a request for that command by naming the binary as where it lives, rather than failing
as though the command did not exist.
(Previously: named twenty commands. This adds `restart` — `console/commands/restart.sh` (4 lines),
once its four direct shell callers went through the binary. Before that it added `docker-compose` —
`console/commands/docker-compose.sh` (20 lines), which only exec'd the binary. Before that it
added `magento` — `console/commands/magento.sh` (3 lines), once `install.sh` and three tasks that
called it directly went through the binary. Before that it added `bash` —
`console/commands/bash.sh` (8 lines). Before that it added `debug-off` —
`console/commands/debug-off.sh` (9 lines). Before that it added `debug-on` —
`console/commands/debug-on.sh` (38 lines), plus its sourced task
`console/tasks/set_xdebug_host_property.sh` (19), which disappears as a side effect and needs no
retirement mechanism of its own.)

#### Scenario: Nothing under the shell tree implements it

- **GIVEN** a command proved in Go through its own engine-interaction test
- **WHEN** the shell command tree is searched for an implementation of it
- **THEN** none exists — the file was removed, not replaced
- **AND** the router still answers for it, so the absence is a retirement and not a lost command

Pinned by "and what the binary owns outright has no file to drift from" in
`tests/unit/command_safety_test.sh`, which walks the twenty-one by name (Previously: twenty) and
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

### Requirement: A comment about what stays in shell names its real reason

A comment describing why part of a ported command still runs in shell SHALL name the actual
reason, not a stale one.

#### Scenario: The mac Composer vendor mirror

- **WHEN** the comment above `mirrorsVendor` in `internal/cli/php.go` is read
- **THEN** it says the vendor-mirror flow is what stays in shell, not that `copy-to-container`
  is unported — `copy-to-container` is wired in `run.go`

