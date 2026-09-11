# Tasks: `debug-off` stops going through bash

Authority order for this file, stated once because no design phase runs for this change: the two
specs (`specs/go-entrypoint/spec.md`, `specs/magento-commands/spec.md`) and `proposal.md` carry the
whole behavioural contract; `exploration.md` is read only for the parts the proposal did not have
to re-derive. Where they disagree, the proposal wins — it corrected the exploration on two points
that changed the answer: the scope narrowed from two commands to one (`debug-on` split off to its
own cycle, decided by the user on 2026-09-11), and `start_service_if_not_running.sh` was found not
to be `up -d <service>` but a probe that starts the *whole* environment, which is why the defensive
start is dropped rather than replicated. Both corrections are already folded into the specs; this
file adds no third opinion.

All line numbers below were re-verified against the working tree on 2026-09-11 (current `HEAD`
`382247c`): `internal/cli/wrappers.go` (`varnish` ends `:204`, the copy section begins `:206`),
`internal/cli/run.go` (`varnish-off` case `:122-123`, `setup` case `:124`), `MIGRATION.md`
(counter `:21`, `debug-off` row `:260`), `internal/cli/wrappers_test.go` (insertion point between
`:427` and `:429`), `internal/cli/routing_test.go` (the `cases` table, whole file 42 lines),
`internal/cli/fake_engine_test.go` (`commands` already has `Exec` and `Compose`, so nothing here
needs a new method), `dockergento/app/orchestrate.go` (`Restart`'s doc comment `:231-236`, the
method `:237-247`, `Compose`'s no-refusal comment `:326-327`), `dockergento/dockergento.go`
(`Engine.Compose` facade `:305-318`), `console/helpers/worktree.sh` (`hm_alters_environment`
`:85-94`, `debug-off` absent from its case), `console/commands/debug-off.sh` (9 lines, unchanged),
`data/command_descriptions.json` (`debug-off` entry `:239-243`, group `tools`, safety `caution`,
unchanged), `tests/unit/migration_status_test.sh` and `tests/unit/command_safety_test.sh` (read in
full — see the suites note below). No drift found against any citation the specs or proposal make.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~150-210 total: `wrappers.go` +~48 (handler + comments), `run.go` +2, `wrappers_test.go` +~115 (three test functions), `routing_test.go` +1 (one table row), `MIGRATION.md` 2 lines changed. Excludes `openspec/` |
| 400-line budget risk | Low — comfortably inside 400 even at this repo's observed 1.3-2x overshoot |
| Chained PRs recommended | No |
| Suggested split | None — one work unit, one commit, direct to `release/2.0.0` |
| Delivery strategy | `single-pr` / direct commit, per this repo's own convention (no PRs opened on this branch) |
| Chain strategy | none |

Decision needed before apply: No. This is the proposal's own estimate (~120-180 authored lines,
spec deltas excluded) plus this file's own count of comment lines; I believe it holds — the
handler reuses `Exec` and `Compose`, both already on `commands` and on `fakeEngine`, so there is no
interface growth and no new fake method, which is exactly what kept the estimate's upper end from
growing in the proposal's own reasoning. Measure with `git diff --shortstat <parent>..HEAD --
. ':(exclude)openspec'` against this change's own parent (task 4.1), never a hash copied from an
earlier change.

**Why the task shape differs from the last five cycles**: those wrote Go tests against commands
already routed in Go. This one is `MIGRATION.md:62`'s step one — the `case` in `run.go` does not
exist yet, so there is production code to write, not only tests against something already built.
`openspec/changes/archive/2026-09-08-docker-tools-in-go/tasks.md` is the most recent port cycle in
this repository and is the template this file follows for the RED/GREEN split, at a fraction of its
size (two new engine methods and two commands there; zero new engine surface and one command here).

## Phase 0: Scaffolding

- [x] 0.1 Before any edit, run `git rev-parse HEAD` and record the hash. This is **this change's own
      parent commit** for the size measurement in Phase 4 — do not hardcode a fixed hash anywhere
      else in this file, a commit message, or a follow-up note; other commits may land on
      `release/2.0.0` first. (Known at the time this file was written: `382247c`.)

## Phase 1: RED — a port's RED, not a reroute's

There is no `case "debug-off"` in `run.go` today, unlike every command the last five cycles wrote
seam tests for. That changes what RED means here: it is not only "the handler exists but the seam
is still wired to the real engine", it is "the handler does not exist at all, and even once it
does, nothing routes to it yet." Both halves are written and confirmed failing before either is
fixed.

- [x] 1.1 RED (contract) — in `internal/cli/wrappers_test.go`, insert after
      `TestAFailedCacheToggleStopsBeforeThePurge` (ends `:427`) and before
      `TestOutsideAProjectNothingIsAsked` (`:429`), three new test functions. No new imports are
      needed — `encoding/json`, `errors`, `strings`, `cmp`, `cmpopts` are all already imported
      (`:3-14`) for the varnish tests these are modelled on.

      `TestWhatDebugOffAsks`, two subtests:
      - `"text"`: `fake.project = core.Project{Name: "shop", Root: root}` with `root :=
        "/code/shop"`; call `debugOff(io.Discard, io.Discard, false)`; want `exitOK`; want log
        (compared with `cmp.Diff` and `ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")`,
        the convention already at `wrappers_test.go:164,237,271,350`):
        ```go
        want := []call{
            {Method: "Resolve", Dir: cwd},
            {Method: "Exec", Dir: root, Service: phpService,
                Command: []string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
                    "/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
                Options: terminalOptions("")},
            {Method: "Compose", Dir: root, Command: []string{"restart", "phpfpm"}},
        }
        ```
        **Then, separately from the `cmp.Diff`, assert `len(fake.calls) == 3` explicitly, with a
        failure message naming what a fourth call would mean** ("no Start call"). This is not
        redundant with the diff: a `cmp.Diff` that fails on an unexpected fourth entry reads as an
        ordinary mismatch, and the whole reason this scenario exists — the shell's own defensive
        `start_service_if_not_running.sh` probe was dropped on purpose — deserves an assertion that
        says so, not one that says so only in a comment beside a diff a future reader might not read.
      - `"json"`: same fixture, `debugOff(stdout, io.Discard, true)`; decode `stdout` into
        `{ Data struct { Xdebug bool `+"`json:\"xdebug\"`"+` } `+"`json:\"data\"`"+` }`; want
        `Data.Xdebug == false`. This is the JSON half of the same spec scenario
        (`go-entrypoint/spec.md:124-132`), not a separate test — the spec names one test for both.

      `TestAFailedDebugOffEditReportsTheContainersStatus`, mirroring
      `TestAFailedVCLEditReportsTheContainersCode` (`:270-344`) exactly: `fake.outcomes =
      []outcome{{}, {status: 42}}`; want `code == 42`; want log `[Resolve, Exec]` only (**no**
      `Compose`); want stdout to carry no `"xdebug disabled"`; want stderr to contain `"ini edit"`.

      `TestADebugOffEditErrorStopsBeforeTheRestart`, mirroring `TestAFailedEditStopsBeforeTheRestart`
      (`:236-264`) exactly: `fake.outcomes = []outcome{{}, {err: errors.New("docker: no")}}`; want
      `code == exitDocker`; want log `[Resolve, Exec]` only.

      All three names are the spec's own (`go-entrypoint/spec.md:87,113,122`;
      `magento-commands/spec.md:61`), checked against every existing `func Test` in `internal/cli`
      (grepped fresh for this file) — none collide.
      Verify RED: `go test ./internal/cli -short -run 'TestWhatDebugOffAsks|TestAFailedDebugOffEditReportsTheContainersStatus|TestADebugOffEditErrorStopsBeforeTheRestart' -v`
      → **FAILS to compile**: `undefined: debugOff`. This is the first half of a port's RED: not a
      wrong answer, no answer at all yet.

- [x] 1.2 RED (routing) — in `internal/cli/routing_test.go`, add one row to `TestTheRouterAnswersForTheDockerTools`'s
      `cases` table (`:16-22`): `{"debug-off", []string{"debug-off"}}`.
      **Naming note, stated rather than silently worked around**: this test's own name says "the
      docker tools", and `debug-off` is not one by `data/command_descriptions.json`'s grouping
      (`group: tools`, same group as `create-project`, not a Docker-daemon passthrough the way
      `docker-stop-all`/`docker-compose` are). Reusing the table anyway rather than adding a new
      function: the file's own doc comment (`:8-13`) already describes the assertion generically —
      "a command that is not yet a case in run.go's switch falls through to the legacy runner" — and
      two conceptually different commands already share this exact table. A renamed, more general
      test name is a one-line improvement available to whoever reviews this, not a blocker.
      Verify RED: `go test ./internal/cli -short -run TestTheRouterAnswersForTheDockerTools -v` →
      **FAILS for the right reason, and this is the second half of a port's RED**: not a
      misrouted case to fix (there was never one), an **absent** one. `Run(["debug-off"], ...)`
      falls through to `run.go:143`'s `engine(stdout, stderr, false).Shell(here(), args)` — which
      calls `engine(` directly, never `newEngine`, so the fake this test substituted is asked
      nothing at all — then execs the pinned `HM_LEGACY_ROOT/bin/run` under a fresh temp directory
      that does not exist, fails, and returns 3. `fake.calls` stays at length 0, so the subtest's own
      `len(fake.calls) == 0` branch fires, exactly as it already does for `docker-compose` before its
      own case existed.

- [x] 1.3 Confirm both 1.1 and 1.2 fail for the stated reasons and no other: 1.1 a compile error
      naming `debugOff`, nothing else; 1.2 a `t.Fatalf` naming an empty call log, not a panic, not a
      hang, not a nonzero-but-not-3 exit reaching a real daemon or the developer's own shell tree.
      `answering(t)`'s six pins (`fake_engine_test.go:275-292`) are what bound this window the same
      way they already bound `docker-stop-all`'s and `docker-compose`'s own RED windows — no new pin
      is needed because `debug-off`'s unrouted path is the same `Shell()` fallback both of those used.

## Phase 2: GREEN — the port itself

- [x] 2.1 Production: in `internal/cli/wrappers.go`, insert a new section after `varnish`'s closing
      brace (`:204`) and before the copy-functions comment block (`:206`):
      ```go
      //
      // debug-off, alone.
      //
      // It shares phpService and terminalOptions with the varnish handlers above and needs no new
      // engine surface: Exec and Compose are both already on commands.
      //

      func debugOff(stdout, stderr io.Writer, jsonOutput bool) int {
          project, code := projectOr(stderr, jsonOutput, "debug-off")
          if code != 0 {
              return code
          }

          engine := newEngine(stdout, stderr, jsonOutput)

          // The shell script's own pattern carries a stray `\` before `z`
          // (`s/^\zend_extension/`), which GNU sed treats as the literal character rather than an
          // escape — the pattern already matched only `^zend_extension`. Dropped here on purpose,
          // not by oversight (proposal Decision 3).
          status, err := engine.Exec(project.Root, phpService,
              []string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
                  "/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
              terminalOptions(""))
          if status != 0 {
              return failure(stderr, jsonOutput, "debug-off", status, "command_failed",
                  "The xdebug ini edit failed in the phpfpm container", "")
          }
          if err != nil {
              return report(stderr, jsonOutput, "debug-off", err)
          }

          // Compose's own restart, not Operator.Restart: the edit lives inside the container
          // filesystem rather than in the compose file, a recreating restart is the wrong
          // primitive for it, and Operator.Restart refuses unconditionally from an unregistered
          // worktree (app/orchestrate.go:231-247) while Compose carries neither (:326-327) —
          // debug-off is not in hm_alters_environment, and this call must not become the first
          // thing that refuses it there (proposal Decision 1). The choice of method, not a
          // worktree-state assertion, is what proves this: see the note in Phase 3.
          status, err = engine.Compose(project.Root, []string{"restart", "phpfpm"})
          if status != 0 {
              return failure(stderr, jsonOutput, "debug-off", status, "command_failed",
                  "The restart failed after the xdebug ini edit", "")
          }
          if err != nil {
              return report(stderr, jsonOutput, "debug-off", err)
          }

          if jsonOutput {
              return document(stdout, stderr, "debug-off", map[string]any{"xdebug": false})
          }

          fmt.Fprint(stdout, good("  xdebug disabled\n"))

          return exitOK
      }
      ```
      **The Compose-failure branch (`status != 0` after a successful edit) is not pinned by any
      named test** — the specs give two failure scenarios and both are on the edit, not the
      restart. It exists for the same reason the edit's own guard does (parity with `varnish`'s
      shape, `wrappers.go:155-158`) and is called out here as a design choice beyond what is
      tested, not as a silent addition.
      Verify: `go build ./...` succeeds.

      **Apply-time deviation from this task's own code sample, per the spec that gained two
      scenarios after this file was written**: the Compose branch above is *not* what the
      handler implements. `go-entrypoint/spec.md`'s "The restart's own non-zero status is
      returned as-is" scenario (added after this task list) requires the restart's status to be
      returned unwrapped — no `failure()` call, no named step, mirroring `dockerCompose()`
      (`docker_compose.go:15-20`: `if err != nil { return report(...) }; return status`). The
      implemented branch is:
      ```go
      status, err = engine.Compose(project.Root, []string{"restart", "phpfpm"})
      if err != nil {
          return report(stderr, jsonOutput, "debug-off", err)
      }
      if status != 0 {
          return status
      }
      ```
      This *is* now pinned, by two tests the spec names that this task list does not:
      `TestAFailedDebugOffRestartReturnsTheComposeStatus` and
      `TestADebugOffRestartErrorIsReported`. The spec is authoritative per this file's own
      Authority order note; the task's original code sample is superseded, not the spec.

- [x] 2.2 Wire it: in `internal/cli/run.go`, add after the `varnish-off` case (`:122-123`) and
      before the `setup` case (`:124`):
      ```go
      case "debug-off":
          return debugOff(stdout, stderr, jsonOutput)
      ```
      This is the one `case` line the RED in 1.2 was waiting for.

- [x] 2.3 Verify GREEN: `go test ./internal/cli -run 'TestWhatDebugOffAsks|TestAFailedDebugOffEditReportsTheContainersStatus|TestADebugOffEditErrorStopsBeforeTheRestart|TestAFailedDebugOffRestartReturnsTheComposeStatus|TestADebugOffRestartErrorIsReported|TestTheRouterAnswersForTheDockerTools' -v`
      → **PASSES**, all six (the spec's own five debug-off tests, two more than this task named,
      per its "gained two scenarios after tasks.md was written" note — plus the extended routing
      table, including its `docker-stop-all` and `docker-compose` rows, still green). Then the
      whole package: `go test ./internal/cli -v` → **PASSES**, no regression in the existing
      baseline (this repo has no `-short` build tag gating these tests; both are run with and
      without it, same result).

- [x] 2.4 MIGRATION, same commit as 2.1/2.2 (`tests/unit/migration_status_test.sh` fails otherwise
      — it checks both directions): in `MIGRATION.md`, change line 260 from
      `` | `debug-off` | tools | 3 | shell | `` to `` | `debug-off` | tools | 3 | go | ``, and line
      21 from `| Comandos en Go | 32 de 65 |` to `| Comandos en Go | 33 de 65 |`.
      Verify: `bash tests/unit/migration_status_test.sh` (or through `tests/run.sh`) passes — in
      particular "what is marked as Go exists in the Go tree" (greps `internal/cli/` for
      `"debug-off"`, which the new `case` line now satisfies) and "the count at the top matches the
      table".

## Phase 3: What this port cannot prove at the seam, said rather than left implicit

- [x] 3.1 **The worktree guarantee is provable only by construction, not by a seam assertion of
      worktree state** (`go-entrypoint/spec.md:97-99`). `fakeEngine` has no notion of a worktree —
      nothing in `internal/cli`'s tests can put the fake "inside an unregistered worktree" and watch
      it refuse or not. What a reader checks instead: that the recorded call is `Compose`, never
      `Restart` — `TestWhatDebugOffAsks`'s log entry (1.1) pins the *method*, and `Operator.Compose`
      (`app/orchestrate.go:328-330`) is the one of the two that carries no
      `refuseFromAnUnregisteredWorktree` call, while `Operator.Restart` (`:237-247`) calls it
      unconditionally. A reader who wants to double-check this without running anything reads those
      two methods side by side, not a test fixture — the guarantee is a property of which method the
      handler calls, and that is fixed at review time, not at test time. No task follows from this
      beyond stating it, because there is nothing further to build: the manual check in 5.1 is the
      one place this is exercised against a real worktree.

## Phase 4: Regression, size, and the commit

- [x] 4.1 Regression, full: `go test ./...` (baseline recorded at **398 across 24 packages**; this
      run measures **408 across the same 24 packages** — the five debug-off test functions plus the
      one extended routing-table row, no red anywhere, no dropped baseline); `./tests/run.sh unit`
      (**exactly `unit`, not `tests/unit`** — the latter silently runs everything; baseline **616**,
      unchanged by this change, confirmed still **616** — no shell file is touched, `debug-off.sh`
      stays exactly as it is); `gofmt -l ./cmd ./internal ./dockergento` empty (confirmed); `go vet
      ./...` clean (confirmed); `commands` (`internal/cli/engine.go:17-34`) still has exactly
      sixteen methods — this change adds none; `var _ commands = (*dockergento.Engine)(nil)`
      (`engine.go:36`) still compiles unedited.

- [x] 4.2 **Which suites mention `debug-off`, and why they must run.** A fresh grep for the literal
      string `debug-off` across `tests/` and `test/e2e/` before this change returns **zero** matches
      in test source — unlike a retirement cycle, there is no existing bash suite to find, migrate,
      or delete for this command; `console/commands/debug-off.sh` has never had one
      (`proposal.md`'s own Scope section: "There is no bash test for `debug-off`"). What *does* run
      because it iterates over every command generically, without naming this one: `tests/unit/migration_status_test.sh`
      (validates the row-and-counter change from 2.4, from both directions — "what is marked as Go
      exists in the Go tree" and "every command the router answers for is tabled as go") and
      `tests/unit/command_safety_test.sh` (validates that `debug-off` still declares a safety level,
      still has both a file *and* a route — which is legitimate: it is not on that suite's `RETIRED`
      list at `:69-71`, and nothing in this change adds it there, because nothing is deleted). The
      suites this change itself *creates* to name `debug-off` directly are **five** Go tests, not
      four — the spec gained two Compose-restart-failure scenarios after this task list was
      written — plus the extended routing table: those are the ones a later cycle that touches this
      command again must find and run, the same way this cycle found `varnish`'s.

- [x] 4.3 Size measurement (reporting only, nothing to cut — the estimate is comfortably inside
      budget): `git diff --shortstat <parent-from-0.1>..HEAD -- . ':(exclude)openspec'`. Expected
      ≈150-210. If it lands materially above 220, stop and report rather than trim anything named
      above — every line in Phase 1 and 2 is either the spec's own pinned assertion or the one
      undocumented branch already called out as such in 2.1.

      **Measured**: parent `2b73edba5447b3e557147978f9470869e1d49f7e` (this change's own parent —
      `382247c` from this file's header had already been superseded by other commits landing on
      `release/2.0.0` first, exactly as this file's own header warned). `git diff --shortstat` = **5
      files changed, 302 insertions(+), 2 deletions(-)**. This is materially above the 150-210
      estimate and above 220, entirely because the spec grew two scenarios
      (`TestAFailedDebugOffRestartReturnsTheComposeStatus`,
      `TestADebugOffRestartErrorIsReported`, ~90 of the added lines) after this estimate was
      written — not from scope added beyond the spec. Reported per this task's own instruction
      rather than trimmed; still within the orchestrator's stated ledger ceiling of 320.

- [x] 4.4 (landed as `c2b9031`) Commit (orchestrator-executed), one commit, one work unit:

      ```
      feat(cli): port debug-off to Go

      debug-off now runs through the same commands seam varnish-on/varnish-off already use: an Exec
      that re-comments zend_extension in the xdebug ini inside phpfpm, then a Compose restart of
      phpfpm, no new engine surface needed.

      Two things change on purpose, not by drift:

      - The restart is Compose's own (["restart", "phpfpm"]), not Operator.Restart. debug-off is not
        in hm_alters_environment, so it is not refused from an unregistered worktree today, and
        Operator.Restart would refuse unconditionally and may recreate containers — the wrong
        primitive for an edit that lives inside the container filesystem, not the compose file.

      - The shell script's defensive start (start_service_if_not_running.sh, which starts the whole
        environment when phpfpm's own probe answers exactly status 1) is dropped, not replicated.
        Start is not on the commands interface, and replicating it conditionally would need a
        service-state probe that isn't either. With the environment stopped, hm debug-off now fails
        naming the failing step instead of silently starting everything.

      Nothing is deleted: console/commands/debug-off.sh keeps running through bin/run untouched,
      and exec.sh keeps its other callers (debug-on.sh, create-project.sh) for their own cycles.

      MIGRATION.md: 32 de 65 -> 33 de 65.
      ```
      Files: `internal/cli/wrappers.go`, `internal/cli/run.go`, `internal/cli/wrappers_test.go`,
      `internal/cli/routing_test.go`, `MIGRATION.md`.

## Phase 5: Manual verification (real project, non-destructive)

- [ ] 5.1 NOT DONE — needs a live daemon and a disposable project; deliberately not attempted, since debug-off edits a PHP ini inside a container and restarts a service. Build the binary (`go build -o bin/hm ./cmd/hm`) and, against a running project: run
      `hm debug-off`, confirm it prints `  xdebug disabled` in green and exits 0; run `hm describe`
      and confirm it now reports xdebug off; run `hm debug-off` a second time and confirm it is a
      no-op that still succeeds. From a directory that is **not** a registered worktree, run
      `hm debug-off` again and confirm it is **not** refused with exit code 6 — this is the one
      place the worktree guarantee named in 3.1 is exercised against a real worktree rather than by
      construction.
      Verify: manual — text, colour, exit code, and `describe`'s xdebug field all match.

- [ ] 5.2 NOT DONE — needs a live daemon and a disposable project; deliberately not attempted, since debug-off edits a PHP ini inside a container and restarts a service. **The disclosed behaviour change, proved deliberately**: stop the environment
      (`hm stop`), then run `hm debug-off` again. Confirm it exits with the status the container
      engine reports (not 0) and names the ini edit as the failing step, and confirm — by watching
      `docker ps` before and after — that it does **not** start the environment the way the shell
      script's own defensive probe used to.
      Verify: manual — nonzero exit, a readable failing-step message, environment still down
      afterward.

- [ ] 5.3 NOT DONE — needs a live daemon and a disposable project; deliberately not attempted, since debug-off edits a PHP ini inside a container and restarts a service. **Nothing is deleted, proved rather than assumed**: confirm `console/commands/debug-off.sh`
      is byte-identical to its pre-change contents (`git diff --stat` shows it untouched); run
      `bin/run debug-off` directly (bypassing the Go binary) against the same running project and
      confirm the shell twin still disables xdebug end to end, unaffected by anything in this
      change.
      Verify: manual — `git diff` shows no change to the file; the shell twin still works.

## Notes

- Out of scope, no tasks, per the proposal's own Scope section: `debug-on` (its own cycle — needs a
  new `Capture` method on `commands`, a host-detection adapter, and container-resolution
  generalisation that this change's zero-new-surface design deliberately avoids); `create-project`;
  deleting `console/commands/debug-off.sh` (spec-legal to defer — it is one of the twenty routed
  commands `command_safety_test.sh` already recognises as legitimately keeping a file); any change
  to `exec.sh` (still called by `debug-on.sh` and `create-project.sh`, neither touched here) or to
  `data/command_descriptions.json` (the entry's group and safety level are unchanged — only its
  route changes, and that is `run.go`'s case line, not the description file).
- `commands` (`internal/cli/engine.go:17-34`) grows by zero methods in this change — the whole
  point of the proposal's two decisions (literal `Compose`, dropped defensive start) was to keep it
  that way.
- No test asserts the Compose-failure branch named in 2.1's comment; it is there for parity with
  `varnish`'s own shape and is named as such rather than left silent.
