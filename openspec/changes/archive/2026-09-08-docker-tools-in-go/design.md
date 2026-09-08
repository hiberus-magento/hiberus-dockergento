# Design

## The question belongs to the use case, not to the handler

`hm start -s` and `hm docker-stop-all` must ask the same thing, in the same words, before stopping
anybody's containers. So the confirmation lives once, in `app.Operator.StopEverything`, and
`Operator.Start` calls it where it used to call `o.Legacy.Run` (`orchestrate.go:63-67`). A handler
that asked, and a `Start` that did not, is two commands with one name.

It is shaped exactly like `Down` (`orchestrate.go:319-327`), the other command that asks before it
is too late: the caller passes `interactive`, the use case announces through `o.say` and asks
through a new `Ask` hook, and the caller is left with a result and, at most, an error.

| | |
|---|---|
| Hook | `Operator.Ask func(question, suggestion string) (string, error)`, wired from `e.options.Ask` in `operator()` (`dockergento.go:1337`), nil-guarded like `Choose` |
| Question | `o.Ask("Stop them all? [y/N]:", "")` — no trailing space in the argument: `prompt()` adds it (`ask.go:37` passes `text+" "`), so what is rendered is the spec's `"Stop them all? [y/N]: "`. Do not "fix" one of the two. An empty suggestion keeps the prompt the shape `confirm` printed (`prompt()` renders a non-empty one as `[n]`, `palette.go:43-50`), and an empty answer already means no |
| Accepted | `y` or `Y`, exactly as `docker-stop-all.sh:41-47`. Anything else, and any error, stops nothing |
| Result | `core.MachineStop{Total, Others, Stopped}` in `core/orchestration.go`, beside `DownOptions` |
| Not asked | `interactive == false` (what `--yes` sets, `globals.go:46`) stops without asking, as the shell does |
| Declined | no error: `Start` carries on and brings the environment up, exactly as `start.sh:10-12` does when `docker-stop-all.sh` exits 0 having stopped nothing |

`StartOptions` gains `Interactive`, and the start handler sets it the way `down.go:59` does:
`Interactive: os.Getenv("HM_NON_INTERACTIVE") == ""`, at `internal/cli/orchestrate.go:36-37`.
Without that line `hm start -s` would stop the machine silently where bash asked.
`internal/api/server.go:185` leaves it false: an API caller never gets a question.

## Five sentences and a question, said in one place

The messages are the use case's, not the handler's — announced through `o.say`, which is how
`Start` (`orchestrate.go:75`) and `Down` (`orchestrate.go:341-348`) already speak. Put them in the
CLI handler instead and `hm start -s` goes silent where `bin/run start -s` spoke, because `Start`
does not go through that handler.

| Said | When | By |
|---|---|---|
| `No containers running` | `Total == 0`; nothing is asked and the exit is 0 | app |
| `This stops N container(s) on this machine.` | interactive, before the question | app |
| `M of them do not belong to '<project>'.` | **only when M > 0** | app |
| `Stop them all? [y/N]: ` | interactive | app, through `Ask` |
| `Nothing was stopped.` | the answer was not `y`/`Y` | app |
| `Stopping N container(s)` | immediately before stopping | app |
| `{"total": N, "others": M, "stopped": bool}` | JSON output only | CLI |

The conditional line is pinned by `TestTheDoesNotBelongLineIsConditional` in
`dockergento/app/orchestrate_test.go`, against a recording `Announce` — an app test rather than a
CLI one, for the reason above, and it answers the environment-lifecycle scenario "The 'does not
belong' line is conditional". The CLI handler is then thin enough that its own test pins only what
the seam records: the directory, the interactivity, the exit code and the document.

## What the daemon is asked

`ports.ContainerEngine` gains `Stop(ids []string) (failed []string, err error)`, implemented in
`dockerd` beside `Remove` (`engine.go:108`) with `ContainerStop` and `container.StopOptions{}` — the
daemon's own default grace, which is what `docker stop` uses.

It does not stop at the first refusal. `docker stop a b c` asks all three and reports what it could
not do, so a container that will not die does not spare the rest; the ids that failed come back so
they can be named. `err` is only for never reaching the daemon at all. This is why the signature is
not `Remove`'s: `Remove` forces, and a forced removal that fails leaves nothing worth naming.

Eight at a time, not one after another: the docker CLI stops the list concurrently, `hm start -s`
pays this on every start, and a machine holding four environments of nine services would otherwise
serialise thirty-six shutdowns. The deadline is per container (30 s) rather than one budget for the
batch, and `failed` is rebuilt in the order the ids came in, so the message is the same every time.

Tolerance is copied, not improved: a `Containers()` that fails is "No containers running" and exit
0, because `docker ps -q 2>/dev/null` was. Failures to stop become one error naming the ids, which
`report()` (`orchestrate.go:236-241`) turns into `docker_failed` and exit 3 — where bash forwarded
docker's own status. The ids travel in the error rather than in `MachineStop`, because a field the
handler can never print is a field nobody maintains.

Counting is `Running` containers whose `ComposeProject` is not `project.Name` — the same label the
shell filtered on (`docker-stop-all.sh:26-28`), asked once instead of twice.

## Compose is a CLI, so it is run as one

`composelib` implements operations, not a command line, so the passthrough is a subprocess: a new
`dockergento/adapters/composecli` with a `Runner{Command string}`, the same deliberate exception
`gitvcs` makes for `git` and `legacy.Runner` makes for `bin/run`. No new dependency — it is
`os/exec` over a binary that is already required.

It is reached through a port, like everything else the domain needs from outside
(`ports/ports.go:1-6`: "implemented by an adapter and by a fake in the tests"). Without one, what
the runner was asked to run could only be proved by running compose:

```go
// ComposeRunner is the Compose command line itself, for the subcommands this tool does not
// implement. Separate from Orchestrator, which performs operations through the library: this one
// hands an arbitrary subcommand to the binary and answers with the code it exited with.
type ComposeRunner interface {
	// Run executes a Compose subcommand from dir, against these files and this environment, wired
	// to this process's terminal.
	Run(dir string, files []string, environment map[string]string, args []string) (int, error)
}
```

`Operator` gains `Compose ports.ComposeRunner` and a thin `Compose(project core.Project, files
[]string, environment map[string]string, args []string) (int, error)`. The facade resolves the
paths — `Paths` needs an `exists`, and `Operator` has no filesystem — and the use case is what the
fake substitutes for. `Engine.Compose` builds it from `composecli.Runner{Command: reader.ComposeCommand()}`,
so the CLI seam stays `Compose(dir string, args []string) (int, error)`: the directory and the
arguments are the handler's business, and the files and the environment are proved one layer down.

`toolinfo.Reader.ComposeCommand()` (`tooling.go:243`) is resolved by the facade and split with
`strings.Fields`, so `docker compose` becomes an argv and never a shell string. Empty is a
`core.Refusal{Kind: "compose_missing", Code: 3}`. Arguments are appended untouched; stdio is
inherited like `legacy/runner.go:48-50`; the exit code is compose's own.

The files are `ComposeFiles(project).Load`, resolved the way `composelib.load` already resolves
them — relative against `project.Root`, missing ones skipped (`orchestrator.go:205-224`). That rule
moves to `core.ComposeFiles.Paths(root string, exists func(string) bool) []string`, which returns
the ordered list and nothing else; the "no compose file to read in %s" error stays where it is
(`orchestrator.go:226-228`), and the passthrough raises the same one, so `up` and a subcommand fail
identically on a project with nothing to load. Both callers use `Paths` because two copies of that
rule is how the passthrough starts disagreeing with `up`. `--project-directory project.Root` is added
where `bin/run:169` passed none, deliberately: it is what makes the relative bind mounts resolve
against the project rather than the caller's directory, and it is what every other ported command
already does. The project name arrives through `COMPOSE_PROJECT_NAME` in `Environment(project)`, as
in the shell.

Both commands resolve the project first (`projectOr`), so neither reaches Docker from a directory
that is not a project. Only `docker-stop-all` then calls `refuseFromAnUnregisteredWorktree`
(`orchestrate.go:249`): it is in `hm_alters_environment` (`console/helpers/worktree.sh:87`) and
`docker-compose` is not.

## The shell files stay, as delegations

Both `.sh` files become `worktree.sh:18-26` stubs: the `binary_missing` refusal, then
`exec "$binary" <command> "$@"`. `console/commands/start.sh:11` is left untouched — it calls the
stub, which is the one delegation point; a second `$binary` call site there would duplicate the
refusal for nothing. `tests/integration/stop_all_guard_test.sh` is deleted in commit 1: it proves
logic that no longer lives in bash.

`worktree_environments_test.sh:284` swaps its bridge probe to `compatibility` — read-only, no
`case` in `run.go`, and excluded from `validate_command` (`bin/run:193`), so it still falls through
to the fake shell tree that ignores its arguments. Line 172 and line 267 stay: they become parity
assertions on the new handler, reached through the stub and directly.

## What RED means for a command that does not exist yet

The seam recipe assumes a wired handler. These are not, so RED is defined in two layers, both
genuine and both safe under `answering(t)`'s pins:

| Layer | RED | GREEN |
|---|---|---|
| Contract | `TestWhatDockerStopAllAsks` (`internal/cli/docker_stop_all_test.go`) against a handler whose body is `return exitError`: the expected call log (`Resolve`, `StopEverything`) is empty | the handler asks the engine |
| Routing | `TestTheRouterAnswersForTheDockerTools` (`internal/cli/routing_test.go`, new): `cli.Run([]string{"docker-stop-all", "--yes"}, …)` records nothing, because the fallthrough execs `HM_LEGACY_ROOT/bin/run`, which the pin makes fail with 3 | the `case` line in `run.go` |

Writing only the routing test would go RED for the wrong reason and stay green afterwards whatever
the handler asked. Writing only the contract test would leave the `case` line untested. Commit 2
adds the `docker-compose` case to the same routing test and its own
`internal/cli/docker_compose_test.go`.

`commands` (`engine.go:17`) gains `StopEverything(dir string, interactive bool) (core.MachineStop, error)`
and `Compose(dir string, args []string) (int, error)`; `call` gains `Interactive bool` and reuses
`Command`; `fakeEngine` answers `StopEverything` from a `stopped core.MachineStop` field, the way
`Resolve` answers from `project`, and consumes an outcome for the error.

## Nothing this suite did not create

An automated test cannot know the machine is otherwise idle, so it never assumes it. The safety
rule: **the suite asserts only on containers it created, and never runs the confirmed stop unless
somebody asked for it.**

| Scenario | How | Runs |
|---|---|---|
| Refused from an unregistered worktree, code 6 | its own throwaway container is still running afterwards | by default |
| Answered `n`: "Nothing was stopped." | `RunWithInput(t, "n\n", …)`, same assertion | by default |
| Answered `y` / `--yes` | asserts its own containers stopped | **only with `HM_E2E_STOP_ALL=1`** |

Say the third one plainly: with that variable set, the scenario runs the real command and **every
running container on the machine is stopped, including the developer's own and anything else that
happens to be up**. It cannot be otherwise — that is what the command does, and narrowing it is out
of scope. So it is opt-in, it is skipped by `e2e.NeedsDocker` as well, and it must never be enabled
on a shared or development daemon. It is the automated form of the manual verification, meant for a
throwaway machine.

Everything else is a fake. `dockerd.Stop` has no unit test for the same reason `Remove` has none —
there is no daemon double — so the opt-in e2e is its **only** automated coverage. If
`test/e2e/stop_all_test.go` is deferred to relieve commit 1, `dockerd.Stop` ships with none, and the
manual verification MUST then exercise the confirmed stop against throwaway containers before the
commit is called done.

## Where each scenario is proved

A decision with no test named is a decision somebody will implement differently. The two fakes are
the existing `engine` (`dockergento/app/environments_test.go:10`, which gains `Stop` returning a
`failed` list) and a new `composeRunner` recording `dir`, `files`, `environment` and `args`, beside
the `orchestrator` and `shell` fakes in `dockergento/app/orchestrate_test.go` — package `app`, so
the use cases are exercised with no daemon and no compose binary.

| Scenario | Test | Where |
|---|---|---|
| Confirming stops them | `TestConfirmingStopsThem` — the `Ask` fake answers `y`, every running id is asked to stop, and `MachineStop.Stopped` is that count | `dockergento/app/orchestrate_test.go` |
| The ids asked for are the ids found running | `TestTheIdsAskedForAreTheIdsFoundRunning` | `dockergento/app/orchestrate_test.go` |
| The "does not belong" line is conditional | `TestTheDoesNotBelongLineIsConditional` | app, recording `Announce` |
| Some containers refuse to stop | `TestSomeContainersRefuseToStop` — the fake answers `failed`, every id is still asked, and the error names them | app |
| …and what the caller does with that | `TestAFailedStopIsReportedAsADockerFailure` — exit 3, `docker_failed`, the ids in the message | `internal/cli/docker_stop_all_test.go` |
| Starting with `-s` asks the same question | `TestStartingWithMinusSAsksTheSameQuestion` — `Start` with `stopOthers` and `interactive` records the same question text, and none when not interactive | app |
| An API caller is never asked | `TestAnAPICallerIsNeverAsked` — `interactive == false` with an `Ask` that fails the test if called; the API side is proof by construction, `server.go:185-188` builds `StartOptions` without `Interactive` | app |
| Stopping the rest before starting | `TestStoppingTheRestBeforeStarting` — the engine fake records the stop before `Up`, and `shell.ran` is empty | app |
| One copy of stopping the rest, too | `TestStoppingTheRestIsTheSameCallOnEitherPlatform` — the same assertion with `Platform` set to `mac` and to `linux` | app |
| What the runner is asked to run | `TestWhatTheComposeRunnerIsAskedToRun` — the fake records the resolved file list, the project directory, the environment and the arguments | app |
| The project's own configuration | same test, on the file list alone: base, platform overlay, worktree overlay when in one, proxy overlay when present, in that order, which is `ComposeFiles.Paths` and therefore what `up` loads | app |
| A machine-readable answer | `TestDockerStopAllAnswersADocumentWhenNobodyIsWatching` | `internal/cli/docker_stop_all_test.go` |
| Asking to stop everything | `TestWhatDockerStopAllAsks` — the seam records the request for the resolved directory carrying whether the run is interactive, and nothing else | `internal/cli/docker_stop_all_test.go` |
| Passing a Compose subcommand through | `TestWhatDockerComposeAsks` — dir and arguments verbatim, `--format json` after the command name included | `internal/cli/docker_compose_test.go` |
| The subcommand's own exit code | `TestComposeExitCodeIsPassedThrough` — the fake answers a non-zero status and the handler returns it unchanged, with no envelope | `internal/cli/docker_compose_test.go` |
| No project to resolve | `TestDockerComposeOutsideAProjectIsRefused` — `projectOr` refuses first, the seam records nothing | `internal/cli/docker_compose_test.go` |
| No Compose binary installed | `TestAMissingComposeBinaryIsRefused` — the facade resolves an empty command and the refusal `compose_missing` (code 3) comes back before the runner is asked anything | `internal/cli/docker_compose_test.go` |
| The same terminal (compose) | test case "compose keeps the caller's streams" in `tests/integration/go_passthrough_test.sh`: `config --format json` through the binary and through the shell produce the same document on stdout, which only holds if stdio was inherited rather than captured | integration |
| Running a ported docker tool through the shell entry point | test cases "the shell entry point still answers for docker-compose" (`config --format json`) and "the shell entry point still answers for docker-stop-all" (with nothing running) in `go_passthrough_test.sh`, both through `bin/run`, both reaching the binary through the delegation stub | integration |
| The question still reaches the same terminal | test case "the question is asked on the caller's own terminal" in `tests/integration/go_passthrough_test.sh`: `HM_NON_INTERACTIVE=` cleared explicitly, `printf 'n\n' \| bin/run docker-stop-all`, and the answer is "Nothing was stopped." or "No containers running" — both are exit 0 and both prove the delegated `exec` kept the caller's streams. Safe by construction: nothing but `y`/`Y` stops anything, and clearing the variable is what stops an inherited `--yes` from turning the probe into a machine-wide stop | integration |

`docker-compose` also gets read-only cases in `go_passthrough_test.sh`: `config --format json` and
`ps`, compared shell against binary.

One thing that is neither changed nor fixed here: a piped run that is still interactive prints the
question through `ask` on stdout (`ask.go:37`), as it does for every other question and as
`confirm` did. `--yes` is what a program reading the document passes.

## Behaviour changes declared

Four differences from the shell implementation, deliberate and none of them silent:

| # | Change | Why |
|---|---|---|
| a | `docker-stop-all` answers `{"total", "others", "stopped"}` when the output is JSON — including off-terminal, since `wanted()` is `!isTerminal` | every ported command answers a document when nobody is watching; that contract is the tool's now, and `go_passthrough_test.sh:124-125` already pins it for others |
| b | Failures to stop exit 3 (`docker_failed`) with the ids named, where bash forwarded docker's own status | `report()` is the failure contract of every ported command; inventing a passthrough code for one of them is the drift |
| c | Compose that cannot be found is `core.Refusal{Kind: "compose_missing", Code: 3}` | `bin/run` resolved `$DOCKER_COMPOSE` before the command ran and failed its own way; the tool now says which thing is missing |
| d | `--project-directory project.Root` where `bin/run:169` passed none outside a worktree | relative bind mounts must resolve against the project, not the caller's directory — what `composelib` already does for every ported command |

And one that is not a change, stated because the port could easily have made it one: stopping
**continues past a container that fails**, as `docker stop a b c` does. The announcements are
painted with `good()` rather than the shell's `print_warning` yellow, which is already true of
`Down`'s warning: parity is the text and the codes.

## mac and linux

`docker-compose` stays `transparent` (`globals.go:21`): no envelope, compose owns stdout.

Platform difference is one, and it is already there: `ComposeFiles` picks
`docker-compose.dev.mac.yml` or `.linux.yml` through `Platform()` (`dockergento.go:1199-1206`), so
the passthrough loads what the platform loads, exactly as `$DOCKER_COMPOSE_FILE_MACHINE` did.
Stopping containers is the same call on both.

## File changes

The proposal's Affected Areas, with the additions this design decided:
`dockergento/core/orchestration.go` (`MachineStop`), `dockergento/core/environment.go` and
`dockergento/adapters/composelib/orchestrator.go` (`ComposeFiles.Paths`, one rule two callers),
`dockergento/ports/ports.go` (`ContainerEngine.Stop` and the new `ComposeRunner`),
`dockergento/adapters/composecli/runner.go`, `dockergento/app/orchestrate.go` (`Operator.Compose`
and its port field, wired in `operator()` at `dockergento.go:1337` beside `Legacy`),
`internal/cli/docker_stop_all_test.go`, `internal/cli/routing_test.go`,
`internal/cli/docker_compose_test.go` and
`test/e2e/stop_all_test.go`. The compose passthrough also touches
`tests/integration/go_passthrough_test.sh`. `MIGRATION.md` moves one row and the counter per
commit: `31 de 65`, then `32 de 65`.

`data/command_descriptions.json`, `docs/docker-stop-all.md` and `bin/run` are untouched: nothing
about either command's arguments, safety or output changes.

## Threat matrix

| Boundary | Applicability | Response | RED test |
|---|---|---|---|
| Subprocess argument composition | Applicable | compose is `exec`'d as argv from `strings.Fields(command)` plus the files and the caller's arguments; never a shell string, so a path with a space and an argument like `--format json` survive | the seam test asserts the exact `Command` slice handed over; `go_passthrough_test.sh` runs `config --format json` |
| Router selection | Applicable | two `case` lines; everything else still falls through, and the `.sh` entry points still reach the same handler through the stubs | the routing RED above, plus `migration_status_test.sh:87-95` (a routed command must be tabled `go`) and `:54-60` (a row saying `go` must exist in the Go tree) |
| Documentation-like paths | N/A | no file is classified or executed by name | — |
| Git repository selection | N/A | no VCS command is added; the worktree refusal reads resolved state | — |
| Commit / push state, PR commands | N/A | no VCS or PR automation in this change | — |

## Size

Measured with `git diff --shortstat 382247c..HEAD -- . ':(exclude)openspec'`, additions plus
deletions, at this repository's density (production ~1.2x, tests ~1.8x):

| Commit | Estimate |
|---|---|
| 1 · `docker-stop-all` (production ~200; Go tests ~300 — the six named app tests and the routing test add ~50 over the previous estimate; stub +18/-51, guard suite -92) | ~610–670 |
| 2 · `docker-compose` (production ~135 — the port and `Operator.Compose` add ~25; tests ~170 with the `composeRunner` fake and the terminal probe; stub +18/-4, probe swap and compose cases ~40) | ~360–420 |

Honestly over the proposal's ~700 and well over 400: the accepted `size:exception` covers it, and
the split keeps each commit a reviewable slice. If the gate after commit 1 measures materially
above this, `test/e2e/stop_all_test.go` is the one piece additive and independent enough to become a
third commit — at the cost recorded above, which is `dockerd.Stop` with no automated coverage until
it lands.

## Open questions

None blocking.
