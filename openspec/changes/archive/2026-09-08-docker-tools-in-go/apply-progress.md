# Apply Progress: The two docker tools stop going through bash

**Mode**: Strict TDD
**Status**: Phase 1 and Phase 2 functionally complete and fully green. Phase 3 (manual verification)
and both commit tasks (1.14, 2.13) are explicitly the orchestrator's — left unticked, nothing
committed.

## Foundation steps (no RED expected, per design)

| Task | What | Verify |
|---|---|---|
| 1.1 | `ContainerEngine.Stop`, `dockerd.Engine.Stop` (bounded concurrency 8, 30s/container), `core.MachineStop`, `engine` fake gains `Stop` | `go build ./...` succeeded; also required switching the `engine` test fake from value to pointer receiver, which rippled into `dockergento/app/database_test.go` and `dockergento/app/doctor_test.go` (not in the original file list — see Deviations) |
| 2.1 | `ports.ComposeRunner`, `core.ComposeFiles.Paths` (extracted from `composelib.load`), `dockergento/adapters/composecli/runner.go` | `go build ./...` succeeded; `gofmt` clean |

## TDD Cycle Evidence

| # | Test(s) | RED (exact failure) | GREEN |
|---|---|---|---|
| 1.2/1.3 | `TestConfirmingStopsThem`, `TestTheIdsAskedForAreTheIdsFoundRunning`, `TestTheDoesNotBelongLineIsConditional`, `TestSomeContainersRefuseToStop`, `TestStartingWithMinusSAsksTheSameQuestion` (2 subtests), `TestAnAPICallerIsNeverAsked`, `TestStoppingTheRestBeforeStarting`, `TestStoppingTheRestIsTheSameCallOnEitherPlatform` (2 subtests) | `dockergento/app/orchestrate_test.go:402: unknown field Ask in struct literal of type Operator` / `operator.StopEverything undefined` | `go test ./dockergento/app -short -v` → 14 PASS (8 tests + subtests); full package 162 passed (baseline 148) |
| 1.5/1.6 | `TestWhatDockerStopAllAsks`, `TestWhatDockerStopAllAsksNonInteractively`, `TestAFailedStopIsReportedAsADockerFailure`, `TestDockerStopAllAnswersADocumentWhenNobodyIsWatching`, `TestTheRouterAnswersForTheDockerTools/docker-stop-all` | `internal/cli/docker_stop_all_test.go:22: fake.stopped undefined` / `undefined: dockerStopAll` / `unknown field Interactive in struct literal of type call`; routing subtest failed because the unrouted command fell through and asked the fake nothing (0 calls, contradicting the "reached the handler" assertion) | `go test ./internal/cli -short -v` → all 4 contract tests + routing subtest PASS |
| 2.2/2.3 | `TestWhatTheComposeRunnerIsAskedToRun` | `dockergento/app/orchestrate_test.go:650: unknown field Compose in struct literal of type Operator` / `operator.Compose undefined` | PASS; full `dockergento/app` package green |
| 2.5/2.6 | `TestWhatDockerComposeAsks`, `TestComposeExitCodeIsPassedThrough`, `TestDockerComposeOutsideAProjectIsRefused`, `TestAMissingComposeBinaryIsRefused`, `TestTheRouterAnswersForTheDockerTools/docker-compose` | `internal/cli/docker_compose_test.go:24: undefined: dockerCompose` | All PASS; `TestTheRouterAnswersForTheDockerTools` both subtests PASS |

## Work Unit Evidence

| Evidence | Commit 1 (`docker-stop-all`) | Commit 2 (`docker-compose`) |
|---|---|---|
| Focused test command | `go test ./internal/cli ./dockergento/... -short -run 'DockerStopAll\|StopEverything\|ConfirmingStopsThem\|SomeContainersRefuseToStop\|StartingWithMinusS\|AnAPICallerIsNeverAsked\|StoppingTheRest\|TheIdsAskedFor\|TheDoesNotBelongLine\|TheRouterAnswers'` → PASS | `go test ./internal/cli ./dockergento/... -short -run 'DockerCompose\|ComposeExitCode\|ComposeRunnerIsAskedToRun\|AMissingComposeBinary\|TheRouterAnswers'` → PASS |
| Runtime harness | `go test ./test/e2e/... -run StopAll -short` → 3 tests SKIP (correctly gated by `NeedsDocker`'s `-short` check; Docker daemon is live on this dev machine, so the confirmed-stop and even the container-creating scenarios were deliberately **not** run for real — see Risks) | `tests/integration/go_passthrough_test.sh` / `tests/integration/worktree_environments_test.sh` — written (compose config/ps comparison, terminal-stream case, `compatibility` probe swap), syntax-checked with `bash -n`, but **not executed** (same safety boundary — these bring up real Docker environments and this session must not create/stop containers) |
| Rollback boundary | Revert: `dockergento/ports/ports.go` (`Stop`), `dockergento/adapters/dockerd/engine.go` (`Stop`), `dockergento/core/orchestration.go` (`MachineStop`), `dockergento/app/orchestrate.go` (`Ask`, `StopEverything`, `Start` signature), `dockergento/app/orchestrate_test.go`, `dockergento/app/environments_test.go` + `database_test.go` + `doctor_test.go` (engine fake pointer receiver), `dockergento/dockergento.go` (`StopEverything`, `Ask` wiring, `StartOptions.Interactive`), `internal/cli/engine.go`, `internal/cli/orchestrate.go`, `internal/cli/docker_stop_all.go(.test)`, `internal/cli/routing_test.go`'s stop-all case, `internal/cli/run.go`'s case, `console/commands/docker-stop-all.sh`, restore `tests/integration/stop_all_guard_test.sh`, `MIGRATION.md` row, `test/e2e/stop_all_test.go` | Revert: `dockergento/ports/ports.go` (`ComposeRunner`), `dockergento/adapters/composecli/runner.go`, `dockergento/core/environment.go` (`Paths`), `dockergento/adapters/composelib/orchestrator.go`, `dockergento/app/orchestrate.go` (`Compose`/`ComposeRunner` field), `dockergento/dockergento.go` (`Compose` facade), `internal/cli/engine.go`, `internal/cli/docker_compose.go(.test)`, `internal/cli/routing_test.go`'s compose case, `internal/cli/run.go`'s case, `console/commands/docker-compose.sh`, `tests/integration/worktree_environments_test.sh` probe swap, `tests/integration/go_passthrough_test.sh` compose cases, `MIGRATION.md` row |

## Regression (measured at session end, both commits applied)

- `go test ./internal/cli -short` → PASS
- `go test ./... -short` → PASS (225 top-level tests: 204 passed, 21 skipped [Docker-gated `e2e.NeedsDocker`], 0 failed, across 22 buildable packages; baseline was 220 passed/22 packages)
- `tests/run.sh unit` → 612/612 assertions passed (unchanged from baseline; the deleted guard suite lived under `tests/integration`, not `tests/unit`)
- `gofmt -l ./cmd ./internal` → empty
- `go vet ./...` → clean
- `var _ commands = (*dockergento.Engine)(nil)` → still compiles
- `rg "docker-stop-all" dockergento/app/orchestrate.go` → only comments/refusal-kind string remain; no `Legacy.Run` call

## Size measurement (task 1.13 / 2.12)

Measured with `git diff --shortstat 382247c..HEAD -- . ':(exclude)openspec'` plus untracked new
files (git diff does not include untracked files):

| Scope | Tracked diff | New files (all additions) | Total |
|---|---|---|---|
| Commit 1 alone | 512 insertions / 161 deletions | `docker_stop_all.go` 35, `docker_stop_all_test.go` 125, `routing_test.go` (single-case) 41, `test/e2e/stop_all_test.go` 102 = 303 | **976** |
| Commit 1 without the e2e file (design's stated fallback) | 512 / 161 | 201 | **874** |
| Commit 2 alone (incremental) | tracked+untracked delta over commit 1 | `docker_compose.go` 21, `docker_compose_test.go` 84, `composecli/runner.go` 77, `routing_test.go` +1 line | **~396** (matches the ≈360–420 forecast) |
| **Both commits, current working tree** | 701 insertions / 185 deletions | 486 (7 new files) | **1372** |

**Risk flagged for the orchestrator (not resolved unilaterally):** task 1.13 says "if commit 1
alone exceeds 700, stop and report to the orchestrator rather than shrink anything." Commit 1
measures 976 (or 874 without the e2e file) — over the 700-line escalation threshold either way.
The design's own suggested fallback (move `test/e2e/stop_all_test.go` to a third commit) does
**not** fully resolve it, because the actual Go test additions (`docker_stop_all_test.go` at 125
lines, the 8 new `orchestrate_test.go` scenarios at ~277 changed lines) are substantially larger
than the ~300-line total the design estimated for all of Commit 1's tests. The combined total
(1372) is also above the pre-approved ≈970–1090 window. Nothing was cut to fit — per
`work-unit-commits` and the review workload guard, budget pressure is never resolved by deleting
tests, comments, or docs. This is reported as a risk requiring an orchestrator decision (accept an
expanded `size:exception`, or split into three commits as the design's own fallback describes,
which then makes Phase 3's confirmed-stop manual verification mandatory rather than conditional).

## Deviations from design

1. **Field/method name collision (bug in the design as literally written).** Design and tasks.md
   both say `Operator` gains a field `Compose ports.ComposeRunner` **and** a method
   `Compose(project, files, environment, args) (int, error)`. Go does not allow a type to have a
   field and method with the same name (confirmed with a throwaway compile check). Deviation: the
   field is named `ComposeRunner` instead; the method keeps the name `Compose`, which is what the
   facade and CLI actually call. Documented at the field's declaration in `orchestrate.go`.
2. **`engine` test fake needed a pointer receiver**, not mentioned in the tasks.md file list for
   1.1: giving it a `Stop` method that records calls requires `*engine`, which rippled into two
   files tasks.md doesn't name — `dockergento/app/database_test.go` and
   `dockergento/app/doctor_test.go` (their `engine{...}` construction sites became `&engine{...}`).
   Mechanical, no behavior change; included in Commit 1's file list above and in the reported diff.
3. **Integration/e2e scripts were written but not executed for real**, beyond `-short`/syntax
   checks. Docker is live on this development machine with real containers; the hard constraint
   for this apply session forbids ever running `docker-stop-all` in a way that could confirm a
   stop, and more broadly forbids creating or stopping containers from this session. `go test
   ./test/e2e/... -run StopAll -short` was run (all 3 tests SKIP via `NeedsDocker`'s `-short`
   gate — no Docker interaction). `bash -n` syntax-checked both integration `.sh` files. Real
   execution is explicitly deferred to the orchestrator's Phase 3 manual verification.
4. **`docker-stop-all`'s CLI handler does not use `projectOr`** (unlike `docker-compose`): the
   shell implementation ran machine-wide from any directory, project or not, so the Go handler
   calls `Resolve` directly and proceeds regardless of whether a project was found — matching the
   "docker-compose outside a project is refused, docker-stop-all is not" contrast implied by the
   two commands' own test lists (only `docker-compose` has a `TestXOutsideAProjectIsRefused`
   scenario).

## Task-by-task status

All of Phase 1 (1.1–1.13) and Phase 2 (2.1–2.12) are complete and green; see `tasks.md` for the
per-task checkboxes. Left unticked, as instructed: **1.14** and **2.13** (commits — orchestrator's),
**3.1** and **3.2** (manual verification — orchestrator's, and 3.2 is destructive if answered `y`).

## Delivery record (orchestrator, 2026-09-08)

The measured size came to 1372 changed lines against a forecast of 970–1090. Nothing was cut: the
user reaffirmed the `size:exception` and chose a three-commit split over the two the tasks planned,
so each commit stays reviewable on its own.

| Commit | What | Size | Verified in isolation |
|---|---|---|---|
| `80c9e8c` | `feat: docker-stop-all in Go, and start -s stops asking the shell` | 18 files, 695+/69− | `go build ./...`, `go vet ./...`, `go test ./internal/cli ./dockergento/app -short` all clean at that commit |
| `9dab3dc` | `test(stop-all): the bash guard retires, and the proofs that need a daemon` | 3 files, 120+/92− | `go build ./...` and `go vet ./test/e2e` clean at that commit |
| `68b1747` | `feat: docker-compose in Go, and the last of the two shell tools` | 17 files, 373+/25− | full regression, below |

Splitting commit 1 from commit 3 needed surgery, because both ports add lines to the same five
files with no blank line between them: `internal/cli/{engine.go,fake_engine_test.go,routing_test.go,run.go}`
and `MIGRATION.md` were staged whole and then had the Compose additions reverse-applied from the
patches the apply phase left behind, and `dockergento/{ports/ports.go,dockergento.go,app/orchestrate.go,app/orchestrate_test.go}`
had their Compose blocks removed from the working copy, staged, and the working copy restored. The
Compose method and the field it reads share a closing brace with the code above them in the diff, so
patching the diff itself would have produced a commit that did not compile; editing whole files and
letting `git` compute the rest is what made each commit independently green.

Regression at `68b1747`: `go build ./...` clean, `go test ./... -short` 246 passed across 23
packages (baseline 220 across 22), `gofmt -l ./cmd ./internal` empty, `go vet ./...` clean,
`tests/run.sh unit` 612 assertions.

Runtime ledger: attempt `dtg-apply-1` settled `passed` with 1372 changed lines recorded and the
budget flag raised; the objective was then reset on the maintainer's standing acceptance of the
exception.

Still open: tasks 3.1 and 3.2, the manual verification, which only the developer can authorise
because a confirmed run of this command stops every container on the machine.

## Manual verification record (task 3.1; 3.2 deliberately not run)

Run by the orchestrator on 2026-09-08 with the user's authorisation, comparing `bin/hm` at `68b1747`
against a binary built from `382247c`. Nine containers of the user's `rabatrepo` project were up
throughout and were never stopped. Three throwaway containers (`hm-manual-check-1..3`, `alpine sleep
900`) were created for the check and removed afterwards; the machine ended with the same nine
containers running it started with.

| Check | Result |
|---|---|
| `docker-stop-all` answering `n`, both builds | exit 0 on both, nothing stopped: twelve containers before and after. |
| The counting | identical on both: "This stops 12 container(s) on this machine." and "3 of them do not belong to 'rabatrepo'." |
| The messages | identical text on stderr on both; the shell version padded them with blank lines and the Go version does not. |
| `docker-compose config --format json` | byte-identical documents, 16124 bytes, exit 0 on both. |
| `docker-compose ps` | byte-identical output, exit 0 on both. |
| A subcommand Compose does not know | exit 1 from both: Compose's own code, passed through. |
| The confirmed stop (task 3.2) | **not run.** The user's own project was up and a confirmed run stops every container on the machine. Recorded as not exercised: `dockerd.Engine.Stop` against a real daemon is proved by nothing automated either, since the e2e that would is opt-in. |

### A defect this check found, not introduced by this change

With the run interactive and the output captured, the JSON document is not machine-readable: the
question lands on standard output in front of it, so `jq` refuses the result. The announcements go
to standard error correctly; only the question does not.

The cause is `internal/cli/ask.go:37`, which writes every prompt to standard output, and it predates
this change: `clean` and `masquerade` ask the same way. The design assumed this matched the shell
implementation. It does not — `console/components/input_info.sh:232` uses `read -rp`, and bash's
`read -p` writes its prompt to standard error.

Not fixed here, for scope: `ask` is shared machinery and changing where it writes changes every
command that asks. It is worth its own change, and it is small. Until then the spec scenario "A
machine-readable answer" holds only when nothing is asked, which is what `--yes` guarantees.

### The defect is fixed, outside this change

`79e827c` `fix(cli): a question belongs on the error stream, not in the answer` moves the prompt to
standard error, with a test that swaps the three standard streams and proves the document stays
clean. It is its own commit because `ask` is shared: `clean` and `masquerade` are improved by the
same line. With it in place, `printf 'n\n' | hm docker-stop-all > file` yields a document `jq`
reads, and the question and the messages arrive on standard error where a person still sees them.

So the scenario "A machine-readable answer" now holds whether or not a question is asked.
