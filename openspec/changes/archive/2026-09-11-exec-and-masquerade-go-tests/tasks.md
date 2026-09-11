# Tasks: What `bash`, `exec` and `masquerade` ask of the engine

Authority order for this file: `design.md` and `specs/go-entrypoint/spec.md` (reconciled with each
other by the orchestrator) overrule `proposal.md` and `exploration.md` wherever they disagree — the
design corrected the exploration twice, a fresh-context validator corrected the design six times, and
every `Test:` name below is the spec's own choice, not an independent one (`internal/cli` is one Go
package; a name collision is a build failure). All line numbers below were re-verified against the
working tree on 2026-09-11 (current `HEAD` `382247c`, the same commit `design.md` cites for
`dockergento.go:452`); no drift was found in any load-bearing citation checked (`orchestrate.go`,
`tools.go`, `engine.go`, `fake_engine_test.go`, `ask.go`, `ask_test.go`, `docker_stop_all_test.go`,
`wrappers_test.go`, `inside_test.go`, `php.go`, `dockergento.go`, `app/anonymise.go`,
`app/database.go`, `dockergento/adapters/composelib/orchestrator.go`). One tangential drift found and
recorded rather than acted on: design's Decision 4 table cites `inside_test.go:21,53,105` as the three
function lines that declare a `root` identifier; the actual lines are `20`/`49`/`104` for the function
declarations (`21`/`50`/`105` for the `root :=` statement inside each) — a documentation-table
citation off by one to three lines, not a claim this file's tasks depend on, since the rule it
supports ("a test function whose handler passes `here()` declares no `root` identifier") holds
unchanged for every function checked.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | Unit 1 (`bash`+`exec`): 252 · Unit 2 (`masquerade`): 154 · **Cumulative: 406** (design's counting convention: a one-line replacement counts as 2 changed lines) |
| 400-line budget risk | **Low per unit** (252 and 154, both comfortably inside 400 alone). **The cumulative is over** (406 > 400) — barely, but over. Applying this repository's observed 1.3–2x overshoot multiplier: unit 1 lands at 328–504, unit 2 at 200–308. **Unit 1 is the one at risk of breaching 400 on its own** if the overshoot lands at the high end |
| Chained PRs recommended | **Yes** — design's own recommendation, made explicitly because the delivery decision belongs before apply, not mid-apply |
| Suggested split | Two self-contained deliveries, strictly in this order: `bash`+`exec` → `masquerade`. Sequential, not parallel: unit 2 modifies `tools_test.go`, which unit 1 creates. This repo's recorded convention on `release/2.0.0` is direct sequential commits rather than PRs; if that still holds, treat the two units as two sequential direct commits instead of two PRs — either way, **not** one combined delivery and **not** a `size:exception` |
| Delivery strategy | `ask-on-risk` (cached) — the cumulative overrun is exactly the risk this strategy asks about |
| Chain strategy | pending — surfaced to the user before apply, per design's "Order, and what each unit leaves green on its own" |

**Decision needed before apply: Yes.** The cumulative (406) exceeds the 400-line review budget by 6
lines. The design's recommendation is two chained deliveries in the fixed order below, each
self-contained, rather than one `size:exception`. This must be raised before Phase 1 starts, not
discovered at Phase 3's size gate.

### Suggested Work Units

| Unit | Goal | Likely delivery | Focused test command | Runtime hazard | Rollback boundary |
|---|---|---|---|---|---|
| 1 | `bash`+`exec` on the seam via one routing line, no interface growth; new `exec_test.go` and `tools_test.go` (bash only) | Commit/PR 1 | `go test ./internal/cli -short -run 'TestWhatExecAsks\|TestExecRefusesWithNoCommand\|TestAFailedExecStatusIsReturnedAsIs\|TestExecReportsAFailure\|TestWhatBashAsks' -v` | An unrouted call is bounded by `t.Chdir(t.TempDir())`, not `DOCKER_HOST` — no daemon dial happens on this path at all | Revert `orchestrate.go:188` (one line), delete `exec_test.go` and `tools_test.go` (both new) |
| 2 | `masquerade` on the seam: `commands` gains `Anonymise(dir string) error`, `tools.go:54` routed, `tools_test.go` modified to add `masquerade`'s four tests including the decline test | Commit/PR 2 (depends on Unit 1: modifies the `tools_test.go` Unit 1 creates) | `go test ./internal/cli -short -run 'TestWhatMasqueradeAsks\|TestADeclinedMasqueradeAsksOnlyToResolve\|TestAFailedAnonymiseIsReported\|TestMasqueradeRefusesADashedArgument' -v` | An unrouted call is bounded by `DOCKER_HOST=unix:///nonexistent`, not `t.Chdir` — it reaches `Database.Ready`'s `Engine.Containers()`, a machine-wide `ContainerList`, regardless of cwd. **This is the highest-consequence RED window in the change: an unbounded run could rewrite a real database's contents** | Revert `engine.go` (`Anonymise`), `tools.go:54`, `fake_engine_test.go` (`Anonymise` growth), `tools_test.go`'s masquerade additions — independent of Unit 1's own tests |

**Sequencing constraint, stated once:** both units touch `internal/cli/tools_test.go` — Unit 1
creates it (holding only `bash`'s test), Unit 2 modifies it (adding `masquerade`'s four tests). They
are **strictly sequential commits**, never parallel branches, regardless of PR-vs-direct-commit
delivery. The interface line (`engine.go`) and the fake method (`fake_engine_test.go`) **must land in
the same commit** as each other inside Unit 2 — `commands` gaining a method `fakeEngine` does not
implement breaks `var _ commands = (*fakeEngine)(nil)` (`fake_engine_test.go:90`) and the test build
with it, for the whole package, not just this change's own tests.

## Phase 0: Scaffolding

- [x] 0.1 Before any edit in Unit 1, run `git rev-parse HEAD` and record the hash. This is **this
      change's own parent commit** for the Phase 3 size check — do not hardcode a fixed hash anywhere
      in this file, a commit message, or a follow-up note; two other artifacts in this repository have
      already gone stale that way this week, and the design's own Size-and-Commits section names this
      explicitly. (Known at the time this file was written: `382247c` — re-capture it fresh at apply
      time; other commits may land on `release/2.0.0` first.)
      Captured at apply time: `fff9b785e344114968f24632404d9716fbc15eb5` (`docs: archive
      retire-read-only-shell-commands, and go-entrypoint counts fifteen`) — several unrelated commits
      landed on `release/2.0.0` after `382247c`, confirming the rule above.

## Phase 1: Unit 1 — `bash` + `exec` (`test(cli): what exec and bash ask of the engine`)

- [x] 1.1 **RED-window safety, named rather than assumed.** State, before writing any subtest, in a
      file-level comment in both new test files: an unrouted `execute()` call reaches `Engine.Exec`
      → `Resolve` (which **succeeds** with a nameless project outside a real repository —
      `dockergento.go:128-130` says so in its own comment) → `orchestrator.Exec`
      (`composelib/orchestrator.go:145`) → `open` (`:166`), which builds and `Initialize`s a docker
      CLI, → `load` (`:204-215`), which refuses for want of a compose file under the temp root and
      returns **before** `service.Exec` ever runs. **`t.Chdir(t.TempDir())`
      (`fake_engine_test.go:274`) is what bounds this window — not `DOCKER_HOST`.** No daemon dial
      happens on this path at all.
      **The one operative consequence of the one thing this design could not verify**: whether
      `docker.Initialize(flags.NewClientOptions())` (`orchestrator.go:179`) performs no dial of its
      own. If a `/nonexistent` socket error appears under this unit's RED instead of `"no compose
      file to read in <temp cwd>"`, that is **not** an alarm — check whether `load` was reached, not
      whether `t.Chdir` held. Both paths still stop before `service.Exec`, so no reach is added
      either way; `DOCKER_HOST` is pinned regardless, so the worst case is a fast `ENOENT`.
      Every subtest that exercises `exec` or `bash` runs under `answering(t)`, and none re-chdirs on
      its own.
- [x] 1.2 RED — create `internal/cli/exec_test.go`. Package header repeats the no-`t.Parallel` rule
      (`t.Setenv`/`t.Chdir` both panic under it, and this change's `askingWith` calls in the sibling
      file add a third reason) and the `Dir`-trap comment (Decision 4), stated **per test function**:
      a test function whose handler passes `here()` declares no `root` identifier, so `Dir: root`
      would not compile in this file — `Dir: cwd` is the only thing in scope.
      Fixture per function, copied from `wrappers_test.go:23-56`'s `TestWhatCopyingIntoTheContainerAsks`
      (not from `inside_test.go`): `fake.project = core.Project{Name: "shop", Root: "/code/shop"}`,
      `cwd := here()` read **after** `answering(t)`. `ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")`
      (the convention already at `wrappers_test.go:164,237,271,350`) — `Tty` never appears in any
      comparison in this file; `Options` is built by calling `terminalOptions(user)` and compared
      with `ignoreTty`, never written as a literal `Tty:` value.
      `TestWhatExecAsks`, a table:
      - `ls -lah` → `want [{Resolve, cwd}, {Exec, cwd, phpfpm, ["ls","-lah"], terminalOptions("")}]`
      - `-r whoami` → `-r` consumed before the command is built; `want Command: ["whoami"]`,
        `Options: terminalOptions("root")`
      - `ls -r` (`-r` not first) → `-r` is **not** stripped when it is not `args[0]`; `want Command:
        ["ls","-r"]`, `Options: terminalOptions("")`
      `TestExecRefusesWithNoCommand`, two rows, no fake needed beyond the usual `answering(t)`:
      - no arguments → `exitUsage`, kind `missing_command`, **empty** log
      - `-r` alone → same three assertions (`-r` stripped **before** the emptiness check)
      `TestAFailedExecStatusIsReturnedAsIs` — `outcomes[0] = outcome{status: 2}` → `code == 2`, not 3
      (`orchestrate.go:190-192` returns the status as-is, without `report()`).
      `TestExecReportsAFailure` — `outcomes[0] = outcome{status: 0, err: errors.New(...)}`, the other
      half of the same branch (`:190-194`: only a non-zero status returns early, so a zero status
      with a non-nil error falls through to `report`) → `exitDocker`, kind `docker_failed`, log ending
      at `Exec` (`Resolve` succeeded first).
      Verify RED: `go test ./internal/cli -short -run 'TestWhatExecAsks|TestExecRefusesWithNoCommand|TestAFailedExecStatusIsReturnedAsIs|TestExecReportsAFailure' -v`.
      **The one legitimate failure signature**, on every subtest that expects success: `exec … = 3,
      want exitOK` (`exitDocker` from `report()`), the error reading `"no compose file to read in
      <temp cwd>"`. **Any other signature is an alarm** — in particular, `exitOK` with a call-log diff
      (`want [Resolve, Exec]`, `got [Resolve]`) means the real `Exec` **succeeded**, i.e. a compose
      project was found and a command ran inside a live container; a non-zero code that is not 3 is
      the same alarm, worse — `orchestrate.go:190-192` would be returning a real container's exit
      status; a **hang** means `open` reached a socket. **Stop the run and investigate before
      repeating it.**
- [x] 1.3 RED — create `internal/cli/tools_test.go`, holding only `bash`'s test in this unit (it will
      be modified in Unit 2 to add `masquerade`). Same header obligations as 1.2 (no-`t.Parallel`,
      the per-function `Dir`-trap comment); same fixture shape.
      `TestWhatBashAsks`, a table:
      - no arguments → `want [{Resolve, cwd}, {Exec, cwd, phpfpm, ["bash"], terminalOptions("")}]`
      - `-r` → `Command` still `["bash"]`, `Options: terminalOptions("root")`
      - `-c 'echo hi'` → `shell()` (`tools.go:10-18`) builds `["bash"]` and appends nothing, so the
        arguments are **silently dropped**; pin this rather than endorse it — `want Command: ["bash"]`
      - `-r -c 'echo hi'` → only `args[0] == "-r"` is honoured, the rest still dropped; `Command:
        ["bash"]`, `User: "root"`
      This is the RED half of `bash`'s test — it is bounded by the same `t.Chdir` pin as `exec`,
      because `shell()` only ever calls `execute()`.
      Verify RED: `go test ./internal/cli -short -run TestWhatBashAsks -v`. Same one legitimate
      failure signature and same alarm class as 1.2 — `bash`'s call reaches `execute()`, the same
      routing line.
- [x] 1.4 Run 1.2 and 1.3's tests together and confirm every failure carries the one legitimate
      signature above. Any other shape — a panic, a wrong exit code, a compile error unrelated to the
      routing — means a test itself is wrong; stop and fix it before proceeding.
- [x] 1.5 GREEN — `internal/cli/orchestrate.go:188`: `engine(` → `newEngine(`. This single line puts
      both `exec` and `bash` on the seam, because `shell()` only ever calls `execute()`.
      `orchestrate.go:208`'s `checkServices` (`Configuration`, off the seam by design) is
      **deliberately left unchanged**.
      Verify GREEN: re-run 1.4's command → **PASSES**, every subtest.
- [x] 1.6 Regression: `go test ./internal/cli -short -v` (whole package, green); `go test ./... -v`
      (no `-short`; 24-package baseline 378, plus this unit's new tests, no red);
      `./tests/run.sh unit` (exactly `unit`, not `tests/unit` — the latter silently runs everything;
      616-assertion baseline, unchanged by this unit — it touches no shell file); `gofmt -l ./cmd
      ./internal ./dockergento` empty; `go vet ./...` clean; `commands` still has fifteen methods
      (`Exec` was already on it); `var _ commands = (*dockergento.Engine)(nil)` (`engine.go:35`)
      still compiles unedited.
- [x] 1.7 Budget watch (informational, not a gate). If this unit's measured diff approaches or exceeds
      400 changed lines, cut in this order and no other (design, "The budget"):
      1. Merge `TestExecReportsAFailure` into `TestAFailedExecStatusIsReturnedAsIs` as a second row —
         note before taking it that the two prove **different branches** (`:190-192` returns a status
         without consulting `report()`, `:194` consults it), so the merged test must keep both rows
         distinct inside it.
      2. Drop the `-r -c 'echo hi'` row from `TestWhatBashAsks` — the `-c` row already proves
         arguments are dropped and the `-r` row already proves the user.
      3. Move `bash` into Unit 2 instead, making it three commits rather than two.
      **Never cut**: the comments, the `-r`-not-first row, the empty-log assertions, or anything in
      Unit 2. If the unit still exceeds 400 after these three cuts, **stop and report it** rather than
      shrinking anything else — the estimate was wrong and that is worth knowing before the next cycle
      repeats it.
      Record the measurement here once available: `git diff --shortstat <parent-from-0.1>..HEAD --
      . ':(exclude)openspec'`.
      Measured (working tree against `fff9b78`, untracked test files included via
      `git add -N`): `3 files changed, 254 insertions(+), 1 deletion(-)` — **255 changed lines**
      total, comfortably inside 400 alone and close to the design's own 252 estimate. No cut needed.
- [ ] 1.8 Commit: `test(cli): what exec and bash ask of the engine`.
      Files: `internal/cli/orchestrate.go`, `internal/cli/exec_test.go` (new),
      `internal/cli/tools_test.go` (new, `bash` only).
      Left unchecked deliberately: the apply executor does not commit in this cycle — the user
      commits directly on `release/2.0.0`. All content for this commit is staged and green.

## Phase 2: Unit 2 — `masquerade` (`test(cli): what masquerade asks of the engine, and the answer that declines it`)

- [x] 2.1 **RED-window safety, named rather than assumed — the highest-consequence window in this
      change.** State, in a file-level comment beside the masquerade tests in `tools_test.go`: an
      unrouted `Engine.Anonymise` (`dockergento.go:452`) call reaches `Resolve` (same nameless
      success) → `app.Importer.Anonymise` (`app/anonymise.go:26-30`) → `Database.Ready`
      (`app/database.go:48`) → `container` (`:121-145`) → `Engine.Containers()` — a machine-wide
      `ContainerList` that dials the daemon **regardless of cwd**. `t.Chdir` does **not** bound this;
      **`DOCKER_HOST=unix:///nonexistent` (`fake_engine_test.go:272`) is the only thing between an
      unrouted RED run and a real daemon.**
      Note the second, independent layer, and that it is defence in depth, not a reason to drop the
      pin: `app/database.go:128` matches on `candidate.Key()`, which falls back to `ComposeProject`
      when `hm.project` is empty (`core/environment.go:112-118`), and Compose stamps
      `com.docker.compose.project` on everything it creates — a nameless project matches essentially
      nothing. **`DOCKER_HOST` stays load-bearing regardless.**
      Every subtest that exercises `masquerade` runs under `answering(t)`, and none overrides
      `DOCKER_HOST`.
- [x] 2.2 Extend `internal/cli/engine.go`'s `commands` interface (currently fifteen methods,
      `:17-33`) with `Anonymise(dir string) error`, matching `dockergento.go:452`'s signature exactly
      — `var _ commands = (*dockergento.Engine)(nil)` (`:35`) needs no edit, since the real engine
      already has this method. **In the same step**, add to `internal/cli/fake_engine_test.go` the
      outcome-only method in the shape of `Restart`/`Dump`:
      ```go
      func (f *fakeEngine) Anonymise(dir string) error {
          number := len(f.calls)
          f.calls = append(f.calls, call{Method: "Anonymise", Dir: dir})

          return f.outcomeFor(number).err
      }
      ```
      No new `call` field (`Dir` already exists), no `outcome` change, and `answering`'s six pins and
      its doc comment are **not** edited. These two additions cannot be split across steps or commits:
      `commands` gaining a method `fakeEngine` does not implement breaks
      `var _ commands = (*fakeEngine)(nil)` (`:90`) and the whole package's test build with it.
      Confirm the package still builds (`go build ./...` / `go vet ./internal/cli`) before writing
      any new test against it.
- [x] 2.3 RED — modify `internal/cli/tools_test.go` to add three of `masquerade`'s four tests (**not**
      the decline test — see 2.5). Same fixture shape as Unit 1 (`fake.project = core.Project{Name:
      "shop", Root: "/code/shop"}`, `cwd := here()` after `answering(t)`), same per-function `Dir`-
      trap comment, same `ignoreTty` where `Options` is asserted.
      `TestWhatMasqueradeAsks` — no arguments, confirmed by `answering(t)`'s own pin (`ask()` answers
      its suggestion `"Y"` without reading stdin) → `want [{Resolve, cwd}, {Anonymise, cwd}]`.
      `TestMasqueradeRefusesADashedArgument` — table over `--nope` and `-y` → `exitUsage`, kind
      `invalid_argument`, **empty** log. This one needs no RED window at all: `tools.go:25-30`
      refuses before any engine is built, the same shape as `down_test.go`'s refusals — it should
      already pass with no fake beyond `answering(t)`, and its assertion does not change between the
      RED and GREEN steps of this unit.
      `TestAFailedAnonymiseIsReported` — `outcomes = [{}, {err: errors.New(...)}]` (the second call,
      not `Resolve`) → `exitDocker`, kind `docker_failed`, log ending at `Anonymise`.
      Verify RED: `go test ./internal/cli -short -run 'TestWhatMasqueradeAsks|TestMasqueradeRefusesADashedArgument|TestAFailedAnonymiseIsReported' -v`.
      **The one legitimate failure signature**, on every subtest that expects success: `masquerade =
      3, want exitOK` (`exitDocker`), the error naming the `/nonexistent` socket. **Any other code is
      an alarm, escalating in this order — stop immediately and do not re-run**:
      - **exit 5 / `service_not_running`** (`app/database.go:139-144`): `Containers()` answered — the
        daemon was reached and `DOCKER_HOST` did not hold, but no container matched.
      - **exit 1 / `anonymisation_failed`** (`app/anonymise.go:70-77`): a container *did* match,
        `credentials()` exec'd into it, and `OneOff.Run` **actually ran `hiberusmagento/masquerade`
        against a database** — it reports failure only because the image exited non-zero.
      - **`exitOK` with a call-log diff** (`want [Resolve, Anonymise]`, `got [Resolve]`): the same run
        *succeeded*. **This and the previous bullet are the worst outcomes available in this cycle and
        are indistinguishable in consequence: personal data in a database nobody put in scope was
        rewritten. Stop immediately and do not re-run** — the user's own project lives on this
        machine.
      - A **hang** means the dial reached something real.
- [x] 2.4 GREEN — `internal/cli/tools.go:54`: `engine(` → `newEngine(`.
      Verify GREEN: re-run 2.3's command → **PASSES**, all three tests.
- [x] 2.5 **Write the decline test now, in this GREEN step, explicitly labelled as having no RED.**
      `masquerade()` returns `exitOK` at `tools.go:51` **before** the engine call it declines, and
      `projectOr` is already routed — so the call log is `[Resolve]` whether `tools.go:54` is routed
      or not. Presenting this as a red-turning-green pair would be false; it proves a branch, not a
      routing. Add `TestADeclinedMasqueradeAsksOnlyToResolve` to `internal/cli/tools_test.go`:
      - `fake := answering(t)`; `fake.project = core.Project{Name: "shop", Root: "/code/shop"}`.
      - A nested `t.Setenv("HM_NON_INTERACTIVE", "")` **over** the helper's own pin. This has a
        precedent in this package: `docker_stop_all_test.go:21-24` does exactly this, comment and
        all — echo that comment here. `ask()` (`ask.go:29-35`) checks `HM_NON_INTERACTIVE` first and
        answers the suggestion without reading stdin while it is set, so under the helper's own `"1"`
        the switch at `tools.go:48-52` could only ever see `"Y"`. `t.Setenv` unwinds LIFO: the
        override registers second and is undone first, restoring the pin for every later test.
      - `cwd := here()`, read **after** `answering(t)`.
      - **Call `askingWith` (`ask_test.go:25`) — do not copy it.** It is an unexported helper in the
        same package that already takes `*testing.T` and a closure and restores all three streams
        through `t.Cleanup`. `askingWith(t, "n\n", func() { code = masquerade(...) })` — `ask.go:45`
        reads the real `os.Stdin` directly, not the writer `masquerade()` was handed, so nothing
        short of swapping the global reaches the decline branch.
      - **A fixture guard, labelled as a guard and nothing more**: `if code != exitOK { t.Fatalf(...) }`
        runs **before** the diff. It catches `exitProject` from a project whose `Name` never got set —
        it cannot tell declining from anonymising, because with no `outcomes` configured the fake's
        `Anonymise` returns `nil` and `report()` answers `exitOK` too, so **both sides of an inverted
        `default:` branch are `exitOK`**. Do not describe this check as evidence about the branch in
        any comment or commit message.
      - **The assertion that actually discriminates**: `want := []call{{Method: "Resolve", Dir:
        cwd}}`, compared with `cmp.Diff` against `fake.calls`. An `Anonymise` entry here means the
        answer was not read.
      - The stderr assertion, needed because the log alone cannot tell "declined" from "returned
        early without asking": `strings.Contains(stderr, "anonymise your database")`.
      One-time confidence check, not a formal gate (this test has no natural RED window to prove it
      can fail): temporarily invert or remove the `default:` branch at `tools.go:48-52`, confirm the
      log-diff assertion fails with an `Anonymise` entry in `got`, then revert the change before
      committing anything. Do not leave this inversion in any commit.
      Verify: `go test ./internal/cli -short -run TestADeclinedMasqueradeAsksOnlyToResolve -v` →
      **PASSES**. This step produces no RED signature to check against — a failure here before the
      manual inversion above would mean the fixture is wrong, not that the test works.
- [x] 2.6 Regression: `go test ./internal/cli -short -v` (whole package, green); `go test ./... -v`
      (no `-short`; 24 packages, green, this unit's new tests included); `./tests/run.sh unit`
      (exactly `unit`; 616-assertion baseline unchanged — no shell file touched); `gofmt -l ./cmd
      ./internal ./dockergento` empty; `go vet ./...` clean; `commands` now has exactly sixteen
      methods, the new one being `Anonymise(dir string) error`; both compile-time assertions
      (`engine.go:35`, `fake_engine_test.go:90`) still compile.
- [x] 2.7 Budget watch (informational, not a gate). Unit 2 is estimated at 154 changed lines, well
      inside 400 even at the top of its overshoot band (200–308) — no cut is expected to be needed.
      If it is approached anyway, the design names no cut order for this unit specifically because
      none of its content is cuttable: the decline test is the whole reason this change exists.
      Record the measurement here once available: `git diff --shortstat <parent-from-0.1>..HEAD --
      . ':(exclude)openspec'`.
      Measured (working tree against `a5f055d`, unit 1's own commit): `4 files changed, 193
      insertions(+), 5 deletions(-)` — **198 changed lines** for unit 2 alone, comfortably inside
      the 320-line ledger ceiling and close to the design's own 154 estimate. No cut needed.
      Cumulative against `fff9b78` (unit 1's captured parent, both units applied): `6 files
      changed, 443 insertions(+), 2 deletions(-)` — 445 changed lines total.
- [ ] 2.8 Commit: `test(cli): what masquerade asks of the engine, and the answer that declines it`.
      Files: `internal/cli/engine.go`, `internal/cli/tools.go`, `internal/cli/fake_engine_test.go`,
      `internal/cli/tools_test.go` (modified — `masquerade`'s four tests added).

## Phase 3: Size gate, cumulative verification, and the delivery decision

- [ ] 3.1 Measure the cumulative diff against **this change's own parent commit captured in 0.1**
      (never a hash written into this file): `git diff --shortstat <parent-from-0.1>..HEAD --
      . ':(exclude)openspec'`. Design's floor is 406 changed lines (unit 1: 252, unit 2: 154), already
      over the 400-line budget at the floor, before any overshoot multiplier is applied. This number
      confirms the Review Workload Forecast's "Decision needed before apply: Yes" — it is reported to
      whoever makes the delivery decision, not acted on unilaterally.
- [ ] 3.2 **Surface the delivery decision explicitly, before this change is delivered, under the
      cached `ask-on-risk` strategy**: the cumulative exceeds the 400-line budget; the recommendation
      is two chained deliveries in the order Unit 1 → Unit 2, each self-contained, rather than one
      `size:exception`. Do not deliver as a single unit without this being raised and answered.
- [ ] 3.3 Cumulative regression, both units applied: `go test ./... -v` (no `-short`; 24-package
      baseline 378, plus this cycle's new tests, no red); `./tests/run.sh unit` (exactly `unit`, not
      `tests/unit`; 616-assertion baseline, unchanged — no shell file touched anywhere in this change);
      `gofmt -l ./cmd ./internal ./dockergento` empty; `go vet ./...` clean;
      `var _ commands = (*dockergento.Engine)(nil)` and `var _ commands = (*fakeEngine)(nil)` both
      compile; `commands` has exactly sixteen methods.
- [ ] 3.4 Confirm the non-goals held end to end: no file under `console/`, `bin/run`, `data/`, or
      `dockergento/` was modified anywhere in this change except the two one-line routing edits and
      the one-line interface addition already listed per unit; no shell twin or bash suite deleted;
      `adapters/toolinfo/tooling.go` untouched; `answering(t)` and its doc comment in
      `fake_engine_test.go` not edited beyond the one method/field addition named in Unit 2; exactly
      two call sites route through `newEngine` in this change — `orchestrate.go:188` and
      `tools.go:54` — and no other `engine(` site changed, in particular not `orchestrate.go`'s own
      `checkServices` (`:208`).
- [ ] 3.5 Manual verification on a real, disposable project (never `rabatrepo` or any project this
      user actually runs — this unit's own RED-window analysis is about exactly this risk):
      - `hm exec ls -lah`
      - `hm exec -r whoami`
      - `hm bash -c 'echo hi'` (confirms the arguments are silently discarded, as pinned) and plain
        `hm bash`
      - the usage errors `hm exec` (no command) and `hm masquerade --nope`, both exit 2
      - `printf 'n\n' | hm masquerade` — leaves the database untouched, exits 0
      - `hm masquerade` confirmed (only on a project whose database's contents do not matter) — exits
        0, database anonymised

## Notes

- Out of scope, no tasks: deleting any shell twin (`bash.sh`, `exec.sh`, `masquerade.sh` and their
  bash suites stay — `exec.sh` has live callers in `debug-on.sh`, `debug-off.sh` and
  `create-project.sh` and waits for `exec-callers-in-go`); the `toolinfo.Reader.Xdebug` disjunction
  defect (`adapters/toolinfo/tooling.go:84-85`) — verified real, explicitly out of scope, touches
  `dockergento/`; `app/anonymise.go:65`'s unconditional `tty: false`; a `TestWhatATerminalIsAskedFor`
  closing the gap that `terminalOptions` itself has no test of its own; a `--yes` row on
  `masquerade` (`globals.go:45-46` consumes it before the command sees its arguments — the row would
  assert `globals()`'s behaviour under `masquerade`'s name); `masquerade`'s `input_required` branch
  (`tools.go:37-40`, structurally unreachable since this call site always passes `"Y"` — a code
  comment, not a test); a `core.Refusal` row on either command (already covered by
  `TestARefusedHostEditIsReported` in `set_host_test.go`); a guard subtest reading back
  `DOCKER_HOST` or `HM_NON_INTERACTIVE` (unfailable — it reads back what the test itself just wrote);
  a `bash` usage error (`shell()` has none — every argument that is not a leading `-r` is discarded,
  so a refusal test would assert nothing).
- `commands` grows from fifteen methods to sixteen in Unit 2 only; Unit 1 adds no method.
- No documentation task beyond the `go-entrypoint` spec delta already committed at
  `openspec/changes/exec-and-masquerade-go-tests/specs/go-entrypoint/spec.md`: no command's
  arguments, `--json` document, or exit codes change in either unit.
- Verified during this phase, not carried from the design unchecked: every line citation in this file
  was re-read against the current working tree before being written into a task. One tangential
  drift was found (see the header note above) and none of the load-bearing safety-critical citations
  — the `t.Chdir`/`DOCKER_HOST` pin lines, the `report()` branch lines, the `app/anonymise.go` and
  `app/database.go` call chain, and `composelib/orchestrator.go`'s `open`/`load` — had drifted from
  `design.md`'s own citations.
