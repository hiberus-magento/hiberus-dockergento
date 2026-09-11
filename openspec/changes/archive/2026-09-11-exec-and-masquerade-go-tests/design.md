# Design: What `bash`, `exec` and `masquerade` ask of the engine

## Technical Approach

No new mechanism, and this time barely any new machinery either. The seam is `commands`
(`engine.go:17-33`, **fifteen** methods today), `newEngine` (`engine.go:39-41`), the compile-time
assertion (`engine.go:35`) and the ordered call log in `fake_engine_test.go`. It grows by exactly
**one method and two routed call sites**:

```go
Anonymise(dir string) error   // dockergento/dockergento.go:452 — verified at HEAD 382247c
```

Fifteen plus one is sixteen, so the proposal's count holds, and `var _ commands =
(*dockergento.Engine)(nil)` needs no edit because `*dockergento.Engine` already has the method.

`bash` and `exec` need no interface growth at all: `Exec` is already on `commands`
(`engine.go:21`) and already driven through `inside()` (`inside_test.go`), and `shell()`
(`tools.go:10-18`) only ever calls `execute()`. One line — `orchestrate.go:188`, `engine(` →
`newEngine(` — puts both commands on the seam.

Everything below is a decision about how the tests are written and what the RED window is allowed to
reach. Here the load-bearing parts are three: the decline path, which is structurally unreachable
under the standard helper; the `Tty` field, which cannot be asserted as a literal; and the two
different pins that stop two different unrouted calls.

### Where the proposal and exploration turned out to be wrong or incomplete

Every cycle this week found the artifact ahead of it wrong about something. This one found five
things, four of them in our favour.

**1. The nested `HM_NON_INTERACTIVE` override is not new machinery — it already ships.**
Proposal Decision 2 and Exploration §B both present the nested `t.Setenv("HM_NON_INTERACTIVE", "")`
as something to be established. It is established: `docker_stop_all_test.go:21-24` does exactly it,
over `answering(t)`, with a comment explaining why:

```go
fake := answering(t)
// answering(t) pins HM_NON_INTERACTIVE for its RED-safe default; this test is about the
// interactive path, so it clears the pin back to what a caller's own terminal looks like.
t.Setenv("HM_NON_INTERACTIVE", "")
```

Only the `os.Stdin` pipe is new to this cycle, and see finding 2. The comment above is the phrasing
the masquerade test should echo, so the two read as the same idea.

**2. `askingWith` should be reused, not copied.** Both artifacts say it "transfers almost
unchanged", which reads as an instruction to copy it into the new file. It does not need copying:
`ask_test.go:25-60` is an unexported helper in the same package, already takes `*testing.T` and a
`func()`, already restores the three streams through `t.Cleanup`, and already feeds the answer from a
goroutine. The masquerade test **calls** it. Zero new machinery, and the answer to the three-stream
question below becomes "whatever `askingWith` already does", which is the cheapest correct answer.

**3. `cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")` already exists, and neither artifact mentions
it.** `wrappers_test.go:164`, `:237`, `:271` and `:350` all build the diff option and pass it to
`cmp.Diff`. This is a stronger answer to the `Tty` problem than the proposal's success criterion
("build the expectation by calling `terminalOptions(user)`"), and it is already the convention in the
one file that had the same problem. See Decision 2.

**4. The decline test has no RED, and neither artifact says so.** `masquerade()` returns `exitOK` at
`tools.go:51` **before** the engine call it declines. `projectOr` is already routed, so the log holds
`{Resolve, Dir: cwd}` whether `tools.go:54` routes or not. The decline test is therefore **green
before the routing line lands and green after it**. This does not weaken it — it proves a branch, not
a routing — but it must be written in the GREEN step and labelled, exactly as
`down-and-set-host-go-tests` ruled for `TestDownAsksInteractivelyWhenNobodySaidOtherwise`. The
proposal's success criterion "each of `bash`, `exec` and `masquerade` has a Go test that fails first"
is still met, by `TestWhatMasqueradeAsks`.

**5. The proposal is right and the exploration wrong on both points where they disagree, verified.**
Exploration §C says the `Dir` divergence has bitten in "`logs` and `CopyInto`", two handlers; the
proposal says three, adding `CopyFrom`. The proposal is right — `wrappers.go:252` passes `here()` too.
And the proposal's `toolinfo` correction is exact: `tooling.go:84-85` does carry the fallback inline,
and the defect is that it is a disjunction on the raw label, so `project == ""` makes the **first**
disjunct true for any container lacking `hm.project`. `core.Container.Key()`
(`core/environment.go:112-118`) picks the source first and compares once. Confirmed, and confirmed
out of scope: no file under `dockergento/` is touched.

**What could not be verified.** That `docker.Initialize(flags.NewClientOptions())`
(`composelib/orchestrator.go:179`) performs no dial. It is asserted by the `read-only-commands`
design and is consistent with `load` (`:196`) running after it, but proving it needs a run with a
blocked socket rather than a reading. It is belt-and-braces either way: `DOCKER_HOST` is pinned, so
the worst case is a fast `ENOENT` rather than a hang.

**Its one operative consequence, because Decision 3 makes error text load-bearing.** If `Initialize`
*did* dial, unit 1's legitimate RED would carry a `/nonexistent` socket error instead of
`"no compose file to read in <temp cwd>"` — which is **unit-2-shaped text appearing under unit 1's
signature**. Under this document's own rule that the error text rather than the exit code
distinguishes a legitimate RED from an alarm, that would read as a pin failure and stop the run for
nothing. So: a `/nonexistent` dial error under unit 1 is **not** an alarm — it is this assumption
being wrong, and the thing to check is whether `load` was reached, not whether `t.Chdir` held. Both
paths still stop before `service.Exec`, so no reach is added either way.

## Architecture Decisions

### Decision 1 — the decline test's machinery, written out

Three things together, and the standard helper supplies only the first.

| # | Piece | Why nothing smaller works |
|---|---|---|
| 1 | `answering(t)` as usual | Its other five pins are still wanted, and `projectOr` (`engine.go:87-92`) refuses on an empty `project.Name`, so the fake must answer `Resolve` |
| 2 | A nested `t.Setenv("HM_NON_INTERACTIVE", "")` | `ask()` checks the variable **first** (`ask.go:29-35`) and returns the suggestion without reading stdin. Under the helper's own `"1"` the switch at `tools.go:48-52` can only ever see `"Y"`, so the decline branch is unreachable |
| 3 | `askingWith(t, "n\n", …)` | `ask.go:45` reads the **real** `os.Stdin` — `bufio.NewReader(os.Stdin)` — not the writer `masquerade()` was handed. Nothing short of swapping the global reaches it |

**Why the `t.Setenv` override is safe, stated because it is the part that looks fragile.**
`t.Setenv(k, v)` records the value `k` held at the moment it is called and registers a `t.Cleanup`
restoring exactly that. Cleanups registered on one `*testing.T` run in **reverse registration
order** when that test ends. `answering(t)` registers first: restore `HM_NON_INTERACTIVE` to what it
was outside the test, normally unset. The test's own `t.Setenv("HM_NON_INTERACTIVE", "")` registers
second: restore it to `"1"`. On the way out the second runs first and puts `"1"` back, then the first
runs and unsets it. The variable is empty only between the override and the end of that one test;
every sibling starts under the helper's pin again, and nothing leaks into the next test file.

Two conditions make that true and both already hold in this package: the two calls are on the **same**
`*testing.T` (or the override on a child of it), and no test calls `t.Parallel` — `t.Setenv` and
`t.Chdir` both panic in a parallel test or one with a parallel parent, and `rg 't\.Parallel'
internal/cli` is empty.

**Three streams or only stdin?** Strictly only `os.Stdin` must move: `ask.go:43` writes the question
to the real `os.Stderr`, and `masquerade()`'s own `stderr` argument is `io.Discard` in these tests, so
no assertion depends on either. The choice is therefore about cost, not correctness.

| Option | Trade-off | Decision |
|---|---|---|
| Call the existing `askingWith` (swaps all three) | No new helper, no duplicated pipe plumbing, the question stays out of the test log, and the returned `stderr` enables the one assertion the call log cannot make (below). Costs one inert coupling: `os.Stdout` becomes a pipe for the length of the subtest, which flips `isTerminal(os.Stdout)` and therefore `terminalOptions(...).Tty` — inert here because `masquerade` never calls it, and harmless to neighbours because Decision 2 removes `Tty` from every comparison | **Chosen** |
| A new stdin-only helper in `tools_test.go` | Smallest global surface, but duplicates ~20 lines of pipe plumbing that already exist ten lines away in the same package, and gives up the captured stderr | Rejected |
| A new stdin+stderr helper | The minimal-necessary set, but it is `askingWith` minus one line, written again under a second name. Two helpers doing the same thing is how the next reader picks the wrong one | Rejected |

The code, in full:

```go
// Declining is the branch that keeps the tool from writing to a database, and it is the one the
// standard helper cannot reach: answering(t) pins HM_NON_INTERACTIVE, and ask() (ask.go:29-35)
// answers with the suggestion without reading stdin while it is set — so the switch at
// tools.go:48 could only ever see "Y".
func TestADeclinedMasqueradeAsksOnlyToResolve(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	// Clearing the pin stacks over the helper's own: t.Setenv records the value it displaced
	// ("1") and restores it on a cleanup that runs before the helper's, because cleanups unwind
	// in reverse registration order. The pin is back in place for every later test and absent
	// only inside this one. docker_stop_all_test.go:22-24 does the same thing for the same
	// reason.
	t.Setenv("HM_NON_INTERACTIVE", "")

	cwd := here()

	var code int

	// askingWith (ask_test.go:25) swaps the three standard streams for pipes and feeds the
	// answer into os.Stdin, which is what ask.go:45 reads — the writer masquerade() was handed
	// is not consulted, so nothing less than moving the global works.
	_, stderr := askingWith(t, "n\n", func() {
		code = masquerade(nil, io.Discard, io.Discard, false)
	})

	// A fixture guard, not evidence about the branch. It catches exitProject from a project
	// whose Name never got set, and it runs before the diff so a broken fixture does not read
	// as a broken branch. It cannot tell declining from anonymising: with no outcome
	// configured the fake's Anonymise returns nil and report() answers exitOK too, so both
	// sides of the break are exitOK. The log below is the only thing that discriminates.
	if code != exitOK {
		t.Fatalf("masquerade declined = %d, want exitOK", code)
	}

	// Resolve and nothing else. An Anonymise entry here means the answer was not read.
	want := []call{{Method: "Resolve", Dir: cwd}}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine after declining (-want +got):\n%s", diff)
	}

	// The log alone cannot tell "declined" from "returned early without asking": both leave
	// exactly [Resolve]. This is the assertion that distinguishes them.
	if !strings.Contains(stderr, "anonymise your database") {
		t.Fatalf("stderr = %q, want the question that was answered", stderr)
	}
}
```

`cwd := here()` is read **after** `answering(t)` has already `t.Chdir`'d, per the convention
`down-and-set-host-go-tests` established and every test in the package now follows.

### Decision 2 — the `Tty` expectation: excluded by name, not computed and not written

`terminalOptions` (`php.go:70-76`) fills three fields, and one of them is not a property of the code
under test:

```go
Tty: isTerminal(os.Stdin) && isTerminal(os.Stdout)
```

A literal `Tty: true` or `Tty: false` is wrong twice over, as the proposal says: it depends on
whether `go test` was run from a terminal or from CI, and — in `tools_test.go`, where `bash`'s
assertions sit beside the masquerade decline test — a neighbouring subtest replaces `os.Stdin` and
`os.Stdout` with pipes.

| Option | Trade-off | Decision |
|---|---|---|
| `Options: terminalOptions(user)` **plus** `cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")` | `Tty` is removed from the comparison entirely, so no stream swap anywhere — this cycle's or a later one's — can reach the assertion. `User` still discriminates (`"root"` versus `""`), which is the break `-r` can actually suffer. Already the convention at `wrappers_test.go:164,237,271,350` | **Chosen** |
| `Options: terminalOptions(user)` alone, as `inside_test.go:39` does | Correct today, because both sides call the same function in the same subtest and therefore agree. But it is correct by coincidence of timing rather than by construction: it survives a stream swap only because the `want` is built after it. One reordering away from a test that fails for a reason nobody can read | Rejected |
| `core.ExecOptions{User: "root", Interactive: true}` + `ignoreTty` | Strictly the strongest: `User` and `Interactive` become literals that can fail against a change inside `terminalOptions` itself. Rejected on scope, not on merit — it introduces a third convention into a package where nine `want` sites already use the second, and the gap it closes is about `terminalOptions`, not about `exec`. Recorded as an Open Question instead | Rejected |
| A literal `Tty:` of any value | Decides the assertion by how the suite was invoked | Rejected |

**Can the `exec` and `masquerade` tests live in the same file?** The question is real but the file is
the wrong boundary — Go runs tests and subtests within a package sequentially in source order unless
`t.Parallel` is called, so a stream swap in one test is visible to another only if it outlives that
test. `askingWith` restores the three streams through `t.Cleanup` (`ask_test.go:47`), which runs when
the function that called it ends. So the swap cannot reach a neighbour whether the neighbour is in the
same file or not, and with `ignoreTty` it would not change an assertion even if it did.

What actually protects this is therefore **package-wide, not file-local**: `t.Cleanup` scoping plus
the standing no-`t.Parallel` rule. Both new files repeat that rule in their headers, because a future
`t.Parallel()` would break the masquerade test, the `t.Chdir` in `answering`, and every `t.Setenv` in
the package at once.

The layout below separates them anyway, for a different reason — Decision 5 — and note that the file
`masquerade` shares is `tools_test.go` with **`bash`**, not with `exec`.

### Decision 3 — two pins, two hazards, and one legitimate RED signature each

Neither pin covers the other. Traced, not assumed:

| Unit | What an unrouted RED reaches | Where it stops | The pin that stops it |
|---|---|---|---|
| 1: `exec`, `bash` | `Engine.Exec` (`dockergento.go:333`) → `Resolve(temp cwd)`, which **succeeds** with a nameless project (`dockergento.go:128-130` says so in its own comment) → `orchestrator.Exec` (`composelib/orchestrator.go:145`) → `open` (`:166`), which builds and initialises a docker CLI → `load` (`:204`) | `load` finds no compose file under the temp root and returns `"no compose file to read in <temp cwd>"` (`:213-215`) **before** `service.Exec` runs | **`t.Chdir(t.TempDir())`** (`fake_engine_test.go:274`) |
| 2: `masquerade` | `Engine.Anonymise` (`dockergento.go:452`) → `Resolve`, same nameless success → `app.Importer.Anonymise` (`app/anonymise.go:26-30`) → `Database.Ready` (`app/database.go:48`) → `container` (`:121`) → `Engine.Containers()` (`:122`) → `dockerd.Engine.Containers` → `connect()` (`dockerd/engine.go:30`) — a machine-wide `ContainerList` | The dial fails with `ENOENT` | **`DOCKER_HOST=unix:///nonexistent`** (`fake_engine_test.go:272`) |

`t.Chdir` does **not** bound unit 2: `Anonymise` reaches the daemon regardless of the working
directory, because `Containers()` takes no project. `DOCKER_HOST` does **not** bound unit 1: nothing
in unit 1's path dials before `load` refuses. **Do not let either stand in for the other.** This is
the correction from three cycles ago restated in its consequence: `Resolve` does not fail on a
non-project directory, so `t.Chdir` bounds things only by what fails downstream of it, and downstream
of it for `Anonymise` there is nothing.

`masquerade` has a second, independent layer, and it is worth naming so nobody mistakes it for a
reason to drop the pin. `app/database.go:128` matches on `candidate.Key()`, which falls back to
`ComposeProject` when `hm.project` is empty (`core/environment.go:112-118`), and Compose stamps
`com.docker.compose.project` on everything it creates. Matching would require a container carrying
`com.docker.compose.service=db` while carrying **neither** project label — which Compose never
produces. That makes the window survivable, not absent. `DOCKER_HOST` stays load-bearing.

**Each unit's RED has exactly one legitimate failure signature. Any other signature means a pin did
not hold — stop the run and investigate before repeating it.**

| Unit | RED must fail with | The alarm, and what it means |
|---|---|---|
| 1 (`exec`, `bash`) | `exec … = 3, want exitOK` — `exitDocker` from `report()` (`orchestrate.go:194`), the error being `"no compose file to read in <temp cwd>"` | **`exitOK` with a call-log diff** (`want [Resolve, Exec]`, `got [Resolve]`) means the real `Exec` **succeeded** — a compose project was found and a command ran inside a live container. **A non-zero code that is not 3** is the same alarm, worse: `orchestrate.go:190-192` is returning a real container's exit status. **A hang** means `open` reached a socket |
| 1, status row | `exec = 3, want 2` | Same table as above |
| 2 (`masquerade`, confirmed) | `masquerade = 3, want exitOK` — `exitDocker`, the error naming the `/nonexistent` socket | **Any code other than 3**, stated as a class the way unit 1's row is, because each member means the dial did not fail and the escalation is ordered: **exit 5 / `service_not_running`** (`app/database.go:139-144`) means `Containers()` answered — the daemon was reached and `DOCKER_HOST` did not hold, but no container matched. **Exit 1 / `anonymisation_failed`** (`app/anonymise.go:70-77`) means a container *did* match, `credentials()` exec'd into it, and `OneOff.Run` **actually ran `hiberusmagento/masquerade` against a database** — it reports failure only because the image exited non-zero. **`exitOK` with a log diff** (`want [Resolve, Anonymise]`, `got [Resolve]`) is the same run having *succeeded*. The last two are the worst outcomes available in this cycle and are indistinguishable in their consequence: personal data in a database nobody put in scope was rewritten. Stop immediately and do not re-run. **A hang** means the dial reached something real |
| 2 (`masquerade`, declined) | **Nothing — it has no RED.** See finding 4 | A RED *failure* here means the fixture is wrong, not that the test works |
| Either, refusal rows | **Nothing — green in RED.** `exec` with no arguments (`orchestrate.go:179-182`) and `masquerade` with a dashed argument (`tools.go:25-30`) both return before any engine is built | Same shape as `down_test.go`'s refusals and the read-only cycle's unit 0 |

Note that units 1 and 2 both RED at exit code 3. They arrive there through different calls stopped
by different pins, and the distinguishing evidence is the error text, not the code.

Rejected as a defence, for the ninth time: a guard subtest asserting `os.Getenv("DOCKER_HOST") ==
"unix:///nonexistent"` after `answering(t)` set it, or `os.Getenv("HM_NON_INTERACTIVE") == ""` after
the decline test cleared it. Both read back what the test itself just wrote and cannot fail. The
signature table above is the defence.

Also rejected: running RED under a `DOCKER_HOST=… go test` prefix. Dropped on purpose two cycles ago
— a prefix protects only the run somebody remembered to type.

### Decision 4 — the `Dir` trap, fourth occurrence, and the convention that already prevents it

`execute()` (`orchestrate.go:188-189`) and `masquerade()` (`tools.go:54`) both call the engine with
`here()` — `os.Getwd()` (`engine.go:98-105`) — and **not** with the `project.Root` the fake just
answered `projectOr` with. `inside()` (`php.go:59-60`) does the opposite, and `inside_test.go:35,65,
91,124…` assert `Dir: root` accordingly. This is the fourth handler where it bites; `logs`,
`CopyInto` and `CopyFrom` were the three before.

**The fixture, copied from `wrappers_test.go:36-49` rather than from `inside_test.go`:**

```go
fake := answering(t)
fake.project = core.Project{Name: "shop", Root: "/code/shop"}
cwd := here()
```

Both fields are load-bearing and neither is decorative. `Name` is what gets past `projectOr`
(`engine.go:88` refuses an empty name), without which every happy-path row would die at `exitProject`
with a log of `[Resolve]` alone. `Root` is `/code/shop` — a path that exists on no machine and can
never equal a `t.TempDir()` — which is what makes the assertion **discriminate**: if anybody
"reconciles" `execute()` with `inside()` by passing `project.Root`, the diff reads `/code/shop`
against the temp path instead of passing by coincidence.

**How to stop a future reader copying the wrong neighbour.** Two devices, one social and one
mechanical.

The comment, written once per new file at the first `Dir: cwd`:

```go
// Dir is the test's working directory, not f.project.Root: orchestrate.go:188 asks about
// here() (os.Getwd()) and never about the project projectOr has just resolved. inside_test.go
// asserts Dir: root for the opposite reason — inside() passes project.Root — so this is where
// the two precedents diverge. If this line ever fails, the want is right and the change to
// orchestrate.go is the defect.
```

The mechanical device is a convention the repository already follows perfectly without having named
it. **Stated per test function, not per file** — `wrappers_test.go` sits on both sides of it, which
is precisely why the file is the wrong unit: **a test function whose handler passes `here()` declares
no `root` identifier.** Checked across every existing case —

| Test function | Handler passes | `root` declared? |
|---|---|---|
| `wrappers_test.go:23` `TestWhatCopyingIntoTheContainerAsks` | `here()` (`wrappers.go:227`) | No — `core.Project{…}` inline, `Dir: cwd` |
| `wrappers_test.go:58` `TestWhatCopyingOutOfTheContainerAsks` | `here()` (`wrappers.go:252`) | No — inline, `Dir: cwd` |
| `docker_stop_all_test.go:20` `TestWhatDockerStopAllAsks` | `here()` (`docker_stop_all.go:23`) | No |
| `wrappers_test.go:165` `TestWhatVarnishAsks` | `project.Root`, via `inside` | **Yes** — `root := "/code/shop"` |
| `wrappers_test.go:238` `TestAFailedEditStopsBeforeTheRestart` | `project.Root`, via `inside` | **Yes** |
| `wrappers_test.go:272`, `:351` | `project.Root`, via `inside` | **Yes** |
| `inside_test.go:21,53,105` (`TestWhatPurgeAsks`, `TestWhatNpmAndMagerunAsk`, `TestWhatTheTestSuitesAsk`) | `project.Root` (`php.go:60`) | **Yes** — and the `Dir: root` lines are `:35`, `:65`, `:121-122`, not `:124`, which is a `terminalOptions("")` line |

Every function, both sides, no exception. `exec_test.go`'s four functions and `tools_test.go`'s five
therefore construct the project inline and declare no `root`. A reader who reaches for `Dir: root`
finds no such identifier in that function and has to stop and look, which is the whole point;
`Dir: cwd` is then the only thing in scope that compiles. Stated in both file headers so it survives
as a rule rather than an accident — and stated as a rule about functions, so a later cycle adding an
`inside`-shaped test to either file is not forced to break it.

The four call logs, read from the handlers:

| Command | Recorded calls |
|---|---|
| `exec` | `{Resolve, Dir: cwd}`, `{Exec, Dir: cwd, Service: phpService, Command: args, Options: terminalOptions(user)}` |
| `bash` | the same, `Command: ["bash"]` always, `Options: terminalOptions("root")` when `-r` came first |
| `masquerade`, confirmed | `{Resolve, Dir: cwd}`, `{Anonymise, Dir: cwd}` |
| `masquerade`, declined | `{Resolve, Dir: cwd}` only |

### Decision 5 — the unit boundary and the file boundary do not coincide

`bash` and `masquerade` are neighbours in `tools.go`, but `bash` reaches the engine only through
`execute()` (`tools.go:14,17`), so **`bash` cannot be tested before unit 1's routing line lands**.
`bash` is therefore unit 1 work living in the file unit 2 will grow.

| File | Action | Unit | Holds |
|---|---|---|---|
| `internal/cli/exec_test.go` | Create | 1 | `exec`'s log, its `-r` handling, its usage error, its status passthrough, its failure |
| `internal/cli/tools_test.go` | Create → Modify | 1, then 2 | Unit 1 `bash`; unit 2 `masquerade` and the decline test |

Rejected: one `exec_and_masquerade_test.go`. The two units land in two commits and two PRs, and a
single file would put a file that unit 1 must not touch in unit 2's diff. Also rejected: putting
`exec` into `tools_test.go` beside `bash` — `execute()` lives in `orchestrate.go`, which already has
`logs_test.go` named after its command rather than its file, and `exec`'s four tests are the bulk of
unit 1.

`fake_engine_test.go` grows once, in unit 2, by one outcome-only method in the shape of `Restart`
(`:148-153`) and `Dump` (`:180-185`). `call` gains **nothing** — `Dir` already exists (`:22`).
`answering` and its doc comment are **not** edited: the six pins already cover all three commands,
and the per-command correction about which pin is load-bearing belongs in the two test files whose
RED depends on it, next to the assertion that would catch it.

## Data Flow

    bash [-r] ──→ shell() ──┐
                            ├──→ execute() ──→ projectOr ──→ Resolve(cwd)
    exec [-r] <cmd...> ─────┘                       │
                                                    └──→ Exec(here(), phpfpm, args,
                                                             terminalOptions(user))
    masquerade ──→ projectOr ──→ Resolve(cwd)
                        │
                        └──→ ask("…", "Y") ──┬── "" | "y" ──→ Anonymise(here())
                                             └── anything else ──→ exitOK, engine untouched
                                                          ↓
                                                 newEngine ──┬─→ *dockergento.Engine (production)
                                                             └─→ fakeEngine (ordered log)

## File Changes

| File | Action | Unit | Description |
|---|---|---|---|
| `internal/cli/orchestrate.go` | Modify | 1 | `:188` routes through `newEngine`. **`:208`'s `checkServices` deliberately unchanged** |
| `internal/cli/exec_test.go` | Create | 1 | `exec`'s four tests |
| `internal/cli/tools_test.go` | Create | 1 | `bash`'s one test |
| `internal/cli/engine.go` | Modify | 2 | `commands` gains `Anonymise(dir string) error` |
| `internal/cli/tools.go` | Modify | 2 | `:54` routes through `newEngine` |
| `internal/cli/fake_engine_test.go` | Modify | 2 | One method, `Anonymise`. No new `call` field, no `outcome` change, no `answering` edit |
| `internal/cli/tools_test.go` | Modify | 2 | `masquerade`'s four tests |
| `openspec/changes/exec-and-masquerade-go-tests/specs/go-entrypoint/spec.md` | Modify | 1, then 2 | The delta, excluded from the size gate |

No file under `console/`, `bin/run`, `data/`, `dockergento/` or `tests/` is modified. No shell twin
and no bash suite is deleted. `adapters/toolinfo/tooling.go` is untouched despite Decision 5 of the
proposal naming its defect.

## Interfaces / Contracts

```go
// engine.go — commands gains one line, taking it from fifteen methods to sixteen.
Anonymise(dir string) error

// fake_engine_test.go — outcome-only, the shape Restart and Dump already have: there is
// nothing to read back, so there is no answer field and f.outcomes carries the only variation.
func (f *fakeEngine) Anonymise(dir string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Anonymise", Dir: dir})

	return f.outcomeFor(number).err
}
```

Both must land in the **same commit**: `commands` gaining a method that `fakeEngine` does not have
breaks `var _ commands = (*fakeEngine)(nil)` (`fake_engine_test.go:90`) and the test build with it.
`var _ commands = (*dockergento.Engine)(nil)` (`engine.go:35`) needs no edit — `Anonymise` is already
on the real engine at `dockergento.go:452` with exactly this signature.

## Testing Strategy

| Layer | What to test | Approach |
|---|---|---|
| Unit | What each handler asks of the engine | `newEngine` substituted by `fakeEngine`; the whole log compared with `cmp.Diff` and `ignoreTty` |
| Unit | Which branch a handler took | Exit code, the `kind` of the fault, and the **length** of the log — an empty log is what proves a refusal returned above the engine |
| Unit | That a question was asked | The stderr `askingWith` captures, for the one case the call log cannot distinguish |
| Integration | That the fake and the engine agree | The unchanged Bash parity suites, which run both halves against a real environment |

Three rules carried from the predecessors:

1. **No `t.Parallel()`.** `newEngine` and `t.Setenv` are process state, `t.Chdir` panics in a parallel
   test or one with a parallel parent, and this cycle adds a third reason: `askingWith` swaps process
   globals.
2. **Never write a `want` with production's own format string**, and never compare a rendering
   against another rendering of the same value.
3. `cwd := here()` is read **after** `answering(t)`, never before.

### The cases, and the break each one catches

Every case below states the regression it catches and how it fails. Cases that could not fail against
a plausible break are listed after the tables and are **not planned**.

**Test names are the delta spec's, verbatim.** `specs/go-entrypoint/spec.md:609-620` states that
every `Test:` name in the delta is this design's rather than an independent choice, and gives the
reason: `internal/cli` is one Go package, the `…IsAUsageError` family already has three members
contorting their prefixes to stay distinct, and a fourth is one collision from a build failure. The
first draft of this document invented five names that contradicted it. **The spec's win**, and the
five that moved are recorded here so the change is visible rather than silent:

| Was (first draft) | Is (spec's, adopted) |
|---|---|
| `TestExecReturnsTheCommandsOwnStatus` | `TestAFailedExecStatusIsReturnedAsIs` |
| `TestExecRefusesACommandThatIsNotThere` | `TestExecRefusesWithNoCommand` |
| `TestDecliningAnonymisationAsksTheEngineForNothing` | `TestADeclinedMasqueradeAsksOnlyToResolve` |
| `TestMasqueradeReportsAFailedAnonymisation` | `TestAFailedAnonymiseIsReported` |
| `TestMasqueradeRefusesAnyOption` | `TestMasqueradeRefusesADashedArgument` |

`TestWhatExecAsks`, `TestWhatBashAsks` and `TestWhatMasqueradeAsks` already agreed.

**Two planned behaviours have no scenario in the delta yet**, and the spec's name list is one short
because of the first. Both rows stay; the spec is being asked to add the scenarios.

| Behaviour | Where it is planned | Spec status |
|---|---|---|
| An `Exec` **error** becoming `exitDocker`/`docker_failed` — the delta covers only the non-zero **status** passthrough, which is a different branch (`orchestrate.go:190-192` versus `:194`) | `TestExecReportsAFailure` | Settled: the spec added the scenario "An exec error is reported as a Docker failure" and named it `TestExecReportsAFailure`. This document proposed a different name and adopted the spec's, per the rule that a `Test:` line is contractual where it is written |
| `bash` silently discarding every argument that is not a leading `-r` (`tools.go:11-17`) | the `-c 'echo hi'` and `-r -c 'echo hi'` rows of `TestWhatBashAsks` | No scenario. The name is already the spec's and does not move |

If the spec adopts a different name for the first, **the spec wins and this table is what gets
edited** — the rule is one document decides, and for `Test:` names that document is the spec.

**Unit 1 — `exec_test.go`**

| Test | Case | Break it catches | How it fails |
|---|---|---|---|
| `TestWhatExecAsks` | `ls -lah` | The routing regressing to `engine(`; `phpService` (`php.go:13`) changing; arguments reordered; the `Dir` trap of Decision 4 | Log diff: `want [{Resolve, cwd}, {Exec, cwd, phpfpm, ["ls","-lah"], terminalOptions("")}]` |
| | `-r whoami` | `-r` no longer consumed at `orchestrate.go:174-177`, which would leave `Command: ["-r","whoami"]` and `User: ""` — a root command silently running as the web user | Log diff on **both** `Command` and `Options.User` |
| | `ls -r` (`-r` not first) | `-r` being matched anywhere in `args` instead of only at `args[0]`, which would eat a real argument of the command being run | `want Command ["ls","-r"], User ""`; got would be `["ls"]` and `root` |
| `TestExecRefusesWithNoCommand` | no arguments | `orchestrate.go:179-182` being removed or loosened, so a bare `hm exec` reaches the engine | `exitUsage`, kind `missing_command`, and an **empty** log |
| | `-r` alone | That `-r` is stripped (`:174-177`) **before** the emptiness check (`:179`), so `hm exec -r` is `missing_command` too. Catches anyone moving the emptiness check above the strip | Same three assertions |
| `TestAFailedExecStatusIsReturnedAsIs` | `outcome{status: 2}` | `orchestrate.go:190-192` being replaced by a plain `report(...)`, which would turn a `grep` that matched nothing into `exitDocker` and break every script piping through `hm exec` | `code == 2`, not 3 |
| `TestExecReportsAFailure` **(name proposed to the spec)** | `outcome{err: …}` at call 1 | `report()` (`:194`) no longer being reached, or the error being swallowed into `exitOK` | `exitDocker`, kind `docker_failed`, log ending at `Exec` |

**Unit 1 — `tools_test.go` (`bash`)**

| Test | Case | Break it catches | How it fails |
|---|---|---|---|
| `TestWhatBashAsks` | no arguments | `shell()` (`tools.go:11`) sending something other than `["bash"]`; the routing | Log diff, `Command: ["bash"]`, `Options: terminalOptions("")` |
| | `-r` | `tools.go:13-15` dropping the `-r` prepend, so `hm bash -r` opens a shell as the web user while the person believes they are root | `Options.User`: `"root"` versus `""` |
| | `-c 'echo hi'` | Today `shell()` builds `["bash"]` and appends nothing, so `hm bash -c 'echo hi'` **silently drops** the arguments. The row pins that rather than endorsing it, and catches anyone who starts forwarding them without a decision being taken | Log diff: `want Command ["bash"]` |
| | `-r -c 'echo hi'` | That only `args[0] == "-r"` is honoured, and that the rest is still dropped | `Command ["bash"]`, `User "root"` |

**Unit 2 — `tools_test.go` (`masquerade`)**

| Test | Case | Break it catches | How it fails |
|---|---|---|---|
| `TestWhatMasqueradeAsks` | no arguments, confirmed by `answering`'s own pin (`ask` returns `"Y"`) | The routing regressing to `engine(`; `Anonymise` being given `project.Root` instead of `here()` | Log diff: `want [{Resolve, cwd}, {Anonymise, cwd}]` |
| `TestADeclinedMasqueradeAsksOnlyToResolve` | `n\n` on the pipe, the pin cleared | `tools.go:48-52`'s `default:` branch being removed or inverted, so answering "no" anonymises the database anyway | **The call-log diff, and only that**: `got` would carry `{Anonymise, cwd}`. See the note below — the exit code cannot discriminate here |
| | the same test's stderr check | A guard added between `projectOr` and `ask` that returns `exitOK` without asking. The log cannot see it: it is `[Resolve]` either way | `strings.Contains(stderr, "anonymise your database")` |
| `TestMasqueradeRefusesADashedArgument` | `--nope` | `tools.go:25-30` being loosened, letting an unrecognised option through to a destructive command | `exitUsage`, kind `invalid_argument`, **empty** log |
| | `-y` | The guard testing `--` instead of `-`, which would let single-dash options through | Same three assertions |
| `TestAFailedAnonymiseIsReported` | `outcomes = [{}, {err: …}]` | `report()` (`:54`) no longer being reached, or a failed anonymisation reporting success — which would tell a script the personal data is gone when it is not | `exitDocker`, kind `docker_failed`, log ending at `Anonymise` |

**The exit code cannot discriminate in the decline row, and the first draft of this document said it
could.** Trace the inverted branch: with no `outcomes` configured, `fakeEngine.Anonymise` returns
`nil`, `report()` (`orchestrate.go:233-235`) answers `exitOK`, and the decline branch returns
`exitOK` at `tools.go:51` too. **Both sides of the break are `exitOK`.** The `code != exitOK` check in
Decision 1's listing stays — it is a fixture guard that catches `exitProject` from a bad
`fake.project`, and it must `t.Fatalf` before the diff so a broken fixture does not read as a broken
branch — but it is **not** evidence about the branch, and this table must not list it as such.

This is the tenth appearance of the unfailable-assertion class in four days, and the first one
**inside the document that catalogues it**. The lesson is specific and worth keeping: the defect did
not arrive as a bad assertion, it arrived as a **good assertion described as reinforcing a second
one**. "Fails on X **and** Y" reads as strength and is never re-traced, which is exactly how the
class survives review. A `How it fails` cell names the one thing that discriminates; anything else in
the test is a guard and is labelled a guard.

### Cases deliberately not planned

- **A `--yes` row on `masquerade`.** Verified: `globals.go:45-46` consumes `--yes` and sets
  `HM_NON_INTERACTIVE=1` **before** the command sees its arguments. `masquerade` never receives it,
  so the row would assert `globals()`'s behaviour under `masquerade`'s name — and, worse, would pass
  identically whether `masquerade` handled it or not.
- **`masquerade`'s `input_required` branch (`tools.go:37-40`).** Structurally unreachable: `ask()`
  errors only when the suggestion is empty (`ask.go:30-34`), and this call site passes `"Y"`. A test
  would need production changed to reach it. Recorded in the file header rather than as a test, as the
  proposal asks.
- **A `core.Refusal` row on either command.** `report()`'s refusal branch (`orchestrate.go:248-251`)
  is already covered by `TestARefusedHostEditIsReported` in `set_host_test.go`. A row here would
  re-prove shared code and catch nothing about these commands — and the seam cannot prove that
  `app/anonymise.go:71-76` produces the refusal in the first place, because the fake is what answers.
- **Any assertion on `Options.Tty`.** Excluded by name per Decision 2. A test that pinned it would
  pass or fail by how `go test` was invoked.
- **A `bash` usage error.** `shell()` has none — it always builds a command, and every argument that
  is not a leading `-r` is discarded. A `TestBashRefuses…` would assert nothing.
- **A guard subtest reading back `DOCKER_HOST` or `HM_NON_INTERACTIVE`.** Unfailable: it reads what
  the test itself just wrote. The temptation is strongest in the decline test, which is why it is
  named here.
- **`stdout == ""` from `askingWith` in the decline test.** `masquerade` writes nothing to the real
  `os.Stdout` — its writers are arguments — so the assertion is nearly unfailable, and the property it
  gestures at (the question goes to stderr, not stdout) is already `ask_test.go:82-107`'s subject.

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED |
|---|---|---|---|
| Documentation-like paths | **N/A** — no file is classified or executed by this change | — | — |
| Git repository selection | **N/A** — no `git -C` or path selector is composed. No `git` runs in either RED path | — | — |
| Commit state | **N/A** — nothing touches the index or the worktree | — | — |
| Push state | **N/A** — no push | — | — |
| PR commands | **N/A** — no PR automation | — | — |
| Shell/subprocess composition | **Applicable** — `execute()` composes an argv run inside a container, and `-r` selects the user it runs as | Decision 4's log assertions compare `Command` and `Options.User` whole | `TestWhatExecAsks`' three rows |
| Process integration (routing) | **Applicable** — two call sites move onto a substitutable factory | `newEngine` still resolves to `engine` in production (`engine.go:39-41`); `routing_test.go` and the parity suites drive the real binary | Signature table, Decision 3 |

The real adversarial surface is not in that matrix, as it was not in either predecessor: it is the RED
window's reach into a live Docker daemon on a machine running the user's own containers. This cycle's
worst case is the worst one yet — not reading a container list, and not an exec into somebody else's
container, but **`hiberusmagento/masquerade` rewriting the personal data of a database that was never
in scope**, via `app/anonymise.go:65`. Decision 3 answers it with a verified stop, a second
independent layer at `Key()`, one named failure signature and one named alarm. The `Key()` layer is
recorded as defence in depth, not as a licence to drop `DOCKER_HOST`.

## Migration / Rollout

No migration. No command's arguments, output, `--json` document or exit codes change. Two routing
edits, each one line, each substituting a factory that returns the same object production has always
built (`engine.go:39-41`). No user-visible behaviour changes, so there is no rollout step and no
impact on existing projects.

## Size and Commits

**Counting convention: changed lines = additions + deletions, as `git diff --shortstat` reports and
as the review guard defines the 400-line budget.** A one-line replacement counts **2**, not 1 — the
convention `read-only-commands-go-tests` established and the one this table uses throughout.

### Unit 1 — `test(cli): what exec and bash ask of the engine`

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `internal/cli/orchestrate.go` | Modify | 1 | 1 | 2 |
| `internal/cli/exec_test.go` | Create | 170 | 0 | 170 |
| `internal/cli/tools_test.go` | Create | 80 | 0 | 80 |
| **Unit 1** | | **251** | **1** | **252** |

`exec_test.go` carries four tests: a four-row table (~60), the two-row refusal (~30), the status
passthrough (~28), the failure (~26), and ~26 of package clause, imports and the header that states
the `Dir` divergence, the no-`root` convention and the no-`t.Parallel` rule. `tools_test.go` is one
four-row table plus the same header obligations, which is why 80 and not 45.

### Unit 2 — `test(cli): what masquerade asks of the engine, and the answer that declines it`

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `internal/cli/engine.go` | Modify | 1 | 0 | 1 |
| `internal/cli/tools.go` | Modify | 1 | 1 | 2 |
| `internal/cli/fake_engine_test.go` | Modify | 10 | 0 | 10 |
| `internal/cli/tools_test.go` | Modify | 141 | 0 | 141 |
| **Unit 2** | | **153** | **1** | **154** |

`tools_test.go` grows by the confirm test (~30), the decline test with its three explanatory comments
(~45), the two-row refusal (~32), the failure (~28) and ~6 of header addition. **Revised +3** from
the first draft's 42: the fixture-guard comment grew from two lines to five when it had to explain
that `code != exitOK` cannot discriminate between declining and anonymising. That is a comment paying
for the defect in the table above, and it is not cuttable — it is the only thing standing between a
later reader and re-adding the assertion.

### Arithmetic

| | + | − | Changed |
|---|---|---|---|
| Unit 1 | 251 | 1 | 252 |
| Unit 2 | 153 | 1 | 154 |
| **Cumulative** | **404** | **2** | **406** |

Checked both ways: 404 + 2 = 406, and 252 + 154 = 406. Per-unit, checked the same way: 251 + 1 = 252
and 2 + 170 + 80 = 252; 153 + 1 = 154 and 1 + 2 + 10 + 141 = 154. `tools_test.go`'s own breakdown:
30 + 45 + 32 + 28 + 6 = 141.

Against the proposal's figures: unit 1 lands at 252 against a floor of ~180-210, inside its own
honest band of ~230-420; unit 2 at 154 against a floor of ~220-260, **below** its floor, because the
proposal priced the stdin machinery as new work and finding 2 removes it. The cumulative 406 sits at
the bottom of the proposal's ~400-470 floor. Adopting the spec's five test names moves no figure —
the names differ in length, not in line count.

The `openspec/` delta is excluded by the measuring command and is not in the table:
`git diff --shortstat <parent>..HEAD -- . ':(exclude)openspec'`, measured against **this change's own
parent at the time it is applied**, never a hash written into this document.

### Order, and what each unit leaves green on its own

The proposal's order is **confirmed**: `bash`+`exec` first, `masquerade` second.

- **Unit 1 first** because it is the cheapest seam addition of the five cycles — one routing line, no
  interface growth, no new machinery — and because it is where the `Dir` comment, the no-`root`
  convention and the `ignoreTty` form are established on work that carries nothing else.
- **Unit 2 second** because it carries everything new: the sixteenth method, the fake's method, the
  nested pin override and the stdin pipe.
- They are **sequential commits, not parallel branches**: unit 2 modifies `tools_test.go`, which unit
  1 creates.

Each unit leaves the tree green alone — `go build ./...`, `go test ./... -short`, `tests/run.sh unit`,
`gofmt -l ./cmd ./internal` empty, `go vet ./...` clean, and both compile-time assertions compiling:

- **Unit 1** changes one production line and adds two test files. `commands` stays at fifteen methods;
  `Exec` is already on it and already implemented by both the real engine and `fakeEngine`, so nothing
  can fail to compile. Reverting it removes two files and restores one line.
- **Unit 2** takes `commands` to sixteen. `*dockergento.Engine` already has `Anonymise`
  (`dockergento.go:452`) so `engine.go:35` compiles; `fakeEngine` gains it in the same commit so
  `fake_engine_test.go:90` compiles. The interface line and the fake method **cannot be split across
  commits**.

### The budget

Each unit is comfortably inside 400 alone (252 and 154). The **cumulative 406 is not**, so the
delivery decision belongs before apply, not mid-apply: under the cached `ask-on-risk` strategy the
recommendation is **two chained PRs in the order above**, each self-contained, rather than one
`size:exception`.

Applying this repository's observed 1.3-2x overshoot: unit 1 328-504, unit 2 200-308. **Unit 1 is the
one at risk.** If it breaches 400, cut in this order and no other:

1. Merge `TestExecReportsAFailure` into `TestAFailedExecStatusIsReturnedAsIs` as a second row.
   Note the consequence before taking it: the two prove **different branches** — `:190-192` returns a
   status without consulting `report()`, `:194` consults it — so the merged test needs both rows kept
   distinct inside it, and the spec scenario item 3 asks for still has to name something.
2. Drop the `-r -c 'echo hi'` row from `TestWhatBashAsks` — the `-c` row already proves arguments are
   dropped and the `-r` row already proves the user.
3. Move `bash` into unit 2, making it three commits rather than two.

Never cut: the comments, the `-r`-not-first row, the empty-log assertions, or anything in unit 2 — the
decline test is the whole reason this change exists. If unit 1 still exceeds 400 after those three
cuts, **stop and report it** rather than shrinking anything else; the estimate was wrong and that is
worth knowing before the next cycle repeats it.

## Open Questions

- [ ] **Two planned behaviours are waiting on delta scenarios**: an `Exec` **error** becoming
      `exitDocker`/`docker_failed` (the delta covers only the non-zero status passthrough, a
      different branch), and `bash` discarding every argument that is not a leading `-r`. Both rows
      are planned here, and the spec has since added both scenarios. It named the first
      `TestExecReportsAFailure`; this design had proposed a different name and was edited to match,
      which is the rule working rather than an exception to it.
- [ ] **`terminalOptions` has no test, and by construction cannot fail one today.** All nine existing
      `want` sites build `core.ExecOptions` by calling the production function
      (`inside_test.go`, `wrappers_test.go`), and this cycle adds four more. A change setting
      `Interactive: false` at `php.go:73` would pass every test in the package. `php_test.go` exists
      but covers only `mirrorsVendor`/`writesDependencies`. Out of scope here — the gap is about
      `terminalOptions`, not about `exec` — but a small `TestWhatATerminalIsAskedFor` would close it.
- [ ] `exec` and `masquerade` pass `here()` while `projectOr` has already resolved the project's root
      — the fourth and fifth instances of the `here()`-versus-`project.Root` inconsistency the four
      predecessors left open. Pinned by Decision 4, not fixed.
- [ ] **`hm bash -c 'anything'` silently discards its arguments** (`tools.go:11-17` builds `["bash"]`
      and never appends). Pinned by `TestWhatBashAsks` rather than fixed, because fixing it is a
      behaviour change this cycle forbids — but a user typing `hm bash -c 'php -v'` gets an
      interactive shell and no explanation.
- [ ] `toolinfo.Reader.Xdebug` (`tooling.go:84-85`) compares the raw `hm.project` label as a
      disjunction instead of calling `core.Container.Key()`, so an empty project name matches every
      unlabelled container. Verified here, **explicitly out of scope** (Decision 5 of the proposal);
      it touches `dockergento/` and wants a test of its own.
- [ ] `app/anonymise.go:65` passes `tty: false` unconditionally while the shell helper passes `-t -i`
      when both streams are terminals. Below this seam and unreachable from it.
- [ ] `console/commands/bash.sh`, `exec.sh` and `masquerade.sh` and their bash suites all survive this
      cycle. Only two of the three will ever qualify for deletion: `exec.sh` has live callers in
      `debug-on.sh` (six sites), `debug-off.sh` and `create-project.sh`, none of them routed in
      `run.go`, so it waits for `exec-callers-in-go`.
