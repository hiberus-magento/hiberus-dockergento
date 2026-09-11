# Proposal: What `bash`, `exec` and `masquerade` ask of the engine

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as the
predecessors `wrappers-go-tests`, `remaining-wrappers-go-tests`, `down-and-set-host-go-tests`,
`retire-ported-shell-commands` and `read-only-commands-go-tests`.

## Intent

`MIGRATION.md:62-64` states the rule in three steps: **a command ported → its test in Go → then the
shell implementation and its bash test are deleted**, in that order. `read-only-commands-go-tests`
wrote step two for the three of the five stragglers declared `"safety": "safe"`. It deferred the
other three with reasons, on record in its own Non-goals: all three are declared `"dangerous"`
(`data/command_descriptions.json:80`, `:324`, `:450`), `bash` and `exec` are joined at one call site
and cannot move separately, and `masquerade`'s decline path needs `os.Stdin` redirected.

This change writes step two for `bash`, `exec` and `masquerade`. It deletes nothing.

## Scope

### In Scope

- Go tests on the `newEngine` seam (`internal/cli/engine.go:39-41`) for `bash`, `exec` and
  `masquerade`: table-driven, no daemon, no network, no project beyond `t.TempDir()`.
- **One routing line** for `bash`+`exec`: `orchestrate.go:188`, `engine(` → `newEngine(`. `Exec` is
  already on `commands` (`engine.go:21`) and already exercised through `inside()`, so the interface
  does not grow. `shell()` (`tools.go:10-18`) only ever calls `execute()`, so both commands land
  together.
- **One method and one routing line** for `masquerade`: `commands` gains
  `Anonymise(dir string) error`, verified against `dockergento/dockergento.go:452-465`, and
  `tools.go:54` routes through `newEngine`. Fifteen methods become sixteen;
  `var _ commands = (*dockergento.Engine)(nil)` (`engine.go:35`) needs no edit.
- `fakeEngine` gains one outcome-only method in the shape of `Restart`/`Dump`. `call`
  (`fake_engine_test.go:20-46`) gains **nothing** — `Dir` already exists.
- The delta for `go-entrypoint`.

### Non-goals (Out of Scope)

- **Deleting any shell twin.** `console/commands/bash.sh`, `exec.sh` and `masquerade.sh` stay, with
  their bash suites. That is a later cycle, and only two of the three will ever qualify:
  `exec.sh` has live callers — `debug-on.sh` at six sites (`:15`, `:16`, `:19`, `:27`, `:32`, `:34`),
  `create-project.sh` at five (`:43`, `:44`, `:49`, `:57`, `:60`) and `debug-off.sh:7`, none of those
  three routed in `run.go`. **Correcting the exploration's inventory**: `composer.sh:95,98,101` and
  `bash.sh:5,7` call it too, but `composer` and `bash` are both routed (`run.go:67`, `:88`), so those
  five are dead shell calling dead shell, as `magento.sh:4` is. `bash.sh` has no live caller of its
  own — only a comment at `tests/unit/skills_test.sh:126`; `masquerade.sh` has none. `exec.sh` waits
  for `exec-callers-in-go`.
- **`console/components/masquerade.sh`.** Two files carry that name. The component is a helper
  sourced by `mysql.sh:34` and `transfer-db.sh:6`, and is not retireable at all. A later deletion
  touches only `console/commands/masquerade.sh`.
- Any behaviour change. Arguments, exit codes and text stay as they are.
- The `toolinfo` defect in Decision 5, and `app/anonymise.go`'s unconditional `tty: false`.
- Any test that needs a Docker daemon.

## Decision 1 — two units, `bash`+`exec` first

| # | Unit | What it lands on its own |
|---|---|---|
| 1 | `bash` + `exec` | One routing line, no interface growth, no new machinery. Leaves the tree green alone: `Exec` already has a fake implementation and `inside_test.go` already drives it. |
| 2 | `masquerade` | One interface method, one routing line, and the stdin machinery of Decision 2. Independent of unit 1 — `tools.go:54` is a different call site from `orchestrate.go:188`. |

The order is not arbitrary. Unit 1 is the cheapest seam addition of the five cycles so far and is
where the `t.Chdir` pin discipline of Decision 3 gets established; unit 2 carries everything new.
Either unit reverts without the other.

## Decision 2 — the decline path is the point, and the standard helper cannot reach it

`masquerade()` (`tools.go:24-55`) rejects dashed arguments, calls `projectOr`, asks
`ask("…", "Y")`, and only on `""` or `"y"` reaches `Anonymise`. Anything else returns `exitOK`
having touched nothing.

`ask()` (`ask.go:23-56`) checks `HM_NON_INTERACTIVE` **first** (`:29-35`) and returns the suggestion
without reading stdin. `answering(t)` pins that variable to `1` (`fake_engine_test.go:271`), so
under the standard helper the switch can only ever see `"Y"`. **The decline branch is structurally
unreachable.**

A decline test needs three things together:

1. `answering(t)` as usual — its other five pins are still wanted, and `projectOr`'s `Resolve` must
   succeed against `f.project`.
2. A nested `t.Setenv("HM_NON_INTERACTIVE", "")`, which stacks over the helper's own pin and unwinds
   LIFO.
3. `os.Stdin` swapped for a pipe carrying `n\n`. `ask.go:45` reads the real `os.Stdin` directly;
   nothing short of swapping the global works.

`ask_test.go:25-60`'s `askingWith` is the precedent and transfers almost unchanged.
**Strictly only `os.Stdin` must move** — `ask.go:43` writes the question to the real `os.Stderr`,
not to the writer `masquerade()` was handed, so swapping stdout/stderr changes no assertion. Match
`askingWith`'s three-stream swap anyway: it keeps the question out of the test log and keeps the two
files consistent, and it costs nothing. **But see the `Tty` note in Success Criteria** — swapping
`os.Stdin` is not free for `exec`.

A note for the test file rather than a test: `masquerade`'s `input_required` branch
(`tools.go:37-40`) is unreachable. `ask()` only errors when the suggestion is empty, and this call
site passes `"Y"`.

## Decision 3 — two pins, two different hazards, neither covers the other

| Command | Where an unrouted RED stops | What bounds it |
|---|---|---|
| `exec`, `bash` | `composelib.Orchestrator.load` (`orchestrator.go:204-215`) refuses with "no compose file to read" before `service.Exec` runs | **`t.Chdir(t.TempDir())`** (`fake_engine_test.go:274`) |
| `masquerade` | `Database.container` (`app/database.go:121-145`) calls `Engine.Containers()` at `:122` — a machine-wide `ContainerList` | **`DOCKER_HOST=unix:///nonexistent`** (`:272`) |

`Engine.Exec` → `Resolve` (which does **not** fail on a non-project directory: `dockergento.go:128-130`
says so in its own comment) → `orchestrator.Exec` → `open` (`:166-202`) → `load`. `open` builds the
docker CLI before `load`, so `DOCKER_HOST` is not idle there either, but the thing that stops the
call is the missing compose file under the temp root. `Engine.Anonymise` → `Resolve` →
`app.Importer.Anonymise` (`app/anonymise.go:26-30`) → `Database.Ready` reaches the daemon regardless
of cwd. **Do not let either pin stand in for the other.**

`masquerade` has a second, independent layer that `describe`'s RED window lacked:
`app/database.go:128` matches on `candidate.Key()`, and `Key()` (`core/environment.go:112-118`)
falls back to `ComposeProject` when `hm.project` is empty. Compose stamps
`com.docker.compose.project` on everything it creates (`composelib/orchestrator.go:240-248`), so a
nameless project matches essentially nothing, and the `db` service filter closes the rest. **This
does not make `DOCKER_HOST` optional** — it makes the unrouted window survivable, not absent.

## Decision 4 — the `Dir` trap, fourth occurrence

Both `execute()` (`orchestrate.go:188-189`) and `masquerade()` (`tools.go:54`) call the engine with
`here()` (`engine.go:98-105` — `os.Getwd()`), **not** with the `project.Root` the fake answered
`projectOr` with (`engine.go:87`). `inside()` does the opposite, and `inside_test.go` asserts
`Dir: root`. Copying that neighbour produces a green test asserting the wrong value. This is the
fourth handler where the divergence bites; `logs`, `CopyInto` and `CopyFrom` were the earlier three,
and it has nearly landed in three cycles running.

The working template is in the repo: `wrappers_test.go:23-56`'s
`TestWhatCopyingIntoTheContainerAsks` sets `fake.project = core.Project{Name: "shop", Root:
"/code/shop"}`, captures `cwd := here()`, and asserts `Dir: cwd`. **Copy that one.** The root must
differ from any `t.TempDir()` so the assertion discriminates instead of passing by coincidence.

| Command | Recorded calls |
|---|---|
| `exec` | `Resolve{cwd}`, `Exec{cwd, phpfpm, args, terminalOptions(user)}` |
| `bash` | the same, `Command: ["bash"]`, and `terminalOptions("root")` when `-r` was first |
| `masquerade`, confirmed | `Resolve{cwd}`, `Anonymise{cwd}` |
| `masquerade`, declined | `Resolve{cwd}` only |

## Decision 5 — a defect found here, fixed elsewhere

`toolinfo.Reader.Xdebug` (`adapters/toolinfo/tooling.go:67-92`) selects the php container with
`item.Labels["hm.project"] == project || (item.Labels["hm.project"] == "" &&
item.Labels["com.docker.compose.project"] == project)` at `:84-85`.

**Correcting the exploration**: it is not that the fallback is missing — the fallback is there,
written inline. It is that it is ordered as a disjunction on the raw label, so when `project` is
`""` the **first** disjunct `"" == ""` is already true for any container that lacks the label,
whatever compose project it belongs to. `core.Container.Key()` (`core/environment.go:112-118`) picks
the source first and compares once, so an empty project name never matches a compose-created
container. `dockerd/engine.go:91-93` populates both fields from exactly those two labels, so the
substitution is semantics-preserving.

That asymmetry is why `describe`'s unrouted RED could `ContainerExecCreate` into an unrelated live
`phpfpm` while `masquerade`'s cannot. **Fix identified, explicitly out of scope**: replace `:84-85`
with a `Key()`-shaped comparison so a nameless project matches nothing. It touches
`dockergento/`, which this change does not, and it wants its own test.

## Capabilities

### New Capabilities

None. No user-visible behaviour changes.

### Modified Capabilities

- `go-entrypoint`: **"A ported command's engine interaction is provable without Docker"**
  (`openspec/specs/go-entrypoint/spec.md:253-601`) gains scenarios for `bash`, `exec` and
  `masquerade`, including the declined-anonymisation scenario; its "Previously covered" roster
  (`:259-264`) grows by those three and names the one new seam method, `Anonymise(dir)`.
- `go-entrypoint`: **"A usage error returns before any engine call"** (`:602-708`) — the subject
  list at `:604-606` broadens to include `exec` and `masquerade`. `exec` refuses above the engine
  call with `missing_command` (`orchestrate.go:179-182`); `masquerade` refuses any dashed argument
  (`tools.go:25-30`). `bash` has no usage error of its own — it always builds a command.

### Governing but not modified

- `go-entrypoint`: **"The test seam changes nothing a real invocation runs"** (`:757-772`) — the two
  new routings conform; `newEngine` still resolves to `engine` in production (`engine.go:39-41`).
- `go-entrypoint`: **"A command proved in Go has no shell twin"** (`:784-836`) — deliberately **not**
  amended. Its roster and `command_safety_test.sh`'s `RETIRED` list grow in the deletion cycle, and
  `exec` never joins them.
- `anonymisation:78-87` ("Anonymising without a terminal"), `worktree-safety:31-40` and
  `command-safety`'s roster already own the behaviour. Nothing about it changes.

## Approach

Strict TDD, as all four predecessors did: the test is written against a seam that does not route
yet, and the routing line is what turns it green. For `masquerade` the interface method is written
to compile the fake, then the call site moves.

`answering(t)` needs **no new pin**: the six it has cover all three, with the load-bearing one
differing per command per Decision 3.

- **`exec`**: `-r` first sets `User: "root"` and is consumed; no arguments is `missing_command` with
  an empty log; the happy path records `Resolve` then `Exec` with `phpService` (`php.go:13`) and
  `terminalOptions(user)` (`php.go:70-76`); a non-zero status from `Exec` is returned as-is
  (`orchestrate.go:190-192`) rather than mapped.
- **`bash`**: `shell()` builds `["bash"]`, and `-r` is forwarded to `execute()` as `["-r", "bash"]`
  — assert the `Command` the engine receives is `["bash"]` either way, with `User` carrying the
  difference.
- **`masquerade`**: a dashed argument is `invalid_argument` with an empty log; the confirmed path
  records `Resolve` then `Anonymise`; **the declined path records `Resolve` only**, per Decision 2;
  an `Anonymise` error is reported through `report`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/engine.go:17-33` | Modified | `commands` gains `Anonymise(dir string) error` (unit 2) |
| `internal/cli/orchestrate.go:188` | Modified | Routes through `newEngine` (unit 1) |
| `internal/cli/tools.go:54` | Modified | Routes through `newEngine` (unit 2) |
| `internal/cli/fake_engine_test.go` | Modified | One fake method; no new `call` field |
| `internal/cli/tools_test.go` | New | `bash` and `masquerade`; the stdin helper lives here |
| `internal/cli/exec_test.go` | New | `exec`'s tests |
| `openspec/specs/go-entrypoint` | Modified | Delta, two requirements |

## Delivery

| # | Unit | Floor | Honest band |
|---|---|---|---|
| 1 | `bash` + `exec` — one routing line, no interface growth | ~180-210 | ~230-420 |
| 2 | `masquerade` — one method, one routing line, the stdin machinery | ~220-260 | ~285-500 |
|   | **Cumulative** | **~400-470** | **~515-920** |

**These are floors, not estimates.** Every recent cycle in this repo overshot its own figure by
1.3-2x: `down-and-set-host-go-tests` 418 → 609, `wrapper-exit-codes` 50-90 → 184,
`down-json-and-flags` 120-180 → 302. The band is that multiplier applied.

Each unit fits the 400-line review budget **alone**. The cumulative does not, at the floor or at the
band. **The delivery decision therefore belongs before apply, not mid-apply** — under the cached
`ask-on-risk` strategy the recommendation is two chained PRs in the order above, each self-contained,
rather than one `size:exception`.

Measured with `git diff --shortstat <parent>..HEAD -- . ':(exclude)openspec'` against **this
change's own parent** at the time it is applied. A fixed hash written into this document goes stale
the moment another change lands first, and has already silently measured unrelated commits twice in
this repository.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| The masquerade unit ships without a working decline test, leaving unproved the branch that keeps the tool from writing to a database | **High** — `answering(t)` makes the branch silently unreachable, so a naive test passes while asserting the confirm path twice | Decision 2; the criterion below makes the empty call log the assertion |
| A test copies `inside_test.go` and asserts `Dir: root` | **High** — it is the adjacent precedent, and this is the fourth occurrence | Decision 4; copy `wrappers_test.go:23-56` instead, with a `Root` that differs from any temp dir |
| `DOCKER_HOST` is assumed to cover `exec`, or `t.Chdir` to cover `masquerade` | Med | Decision 3's table; neither pin is optional and the two protect different calls |
| An exec test hardcodes `Options: core.ExecOptions{Interactive: true}` and goes red when `go test` happens to run with a terminal, or when a neighbouring test swaps `os.Stdin` | Med | `terminalOptions` reads `isTerminal(os.Stdin)`/`os.Stdout` (`php.go:70-76`); build the expectation by calling `terminalOptions(user)`, never by writing `Tty:` out |
| The `toolinfo` defect gets fixed opportunistically inside this change | Med | Decision 5 states the fix and puts it out of scope; no file under `dockergento/` is touched |
| The cumulative overruns the budget mid-apply | Certain at ~400-470+ | Two chained PRs decided **before** apply, not after a gate trips |
| Interface drift as `Engine` grows | Low | `var _ commands = (*dockergento.Engine)(nil)` (`engine.go:35`) fails loudly at sixteen methods as it did at fifteen |

## Rollback Plan

Up to two commits on `release/2.0.0`; `git revert` restores them in reverse order. Everything is
inside `internal/cli`, two of the four files are new, and the two routing edits are one line each,
so reverting removes the tests with nothing attached. No shell file is touched, so no deletion has
to be undone.

## Dependencies

None. **No impact on existing projects and no migration step for users**: no command's arguments,
output or exit codes change. Proven by the untouched Bash parity suites and `go build ./...`.

## Success Criteria

- [ ] Each of `bash`, `exec` and `masquerade` has a Go test that **fails first**, then passes, under
      `go test ./internal/cli -short` with no daemon, no network and no project beyond `t.TempDir()`.
- [ ] **`masquerade` has a decline test that actually declines.** It runs under `answering(t)` plus a
      nested `t.Setenv("HM_NON_INTERACTIVE", "")` plus an `os.Stdin` pipe carrying `n\n`, and asserts
      the call log is `Resolve` **only**. A masquerade unit whose log shows `Anonymise` in the
      decline case, or that reaches `exitOK` without the stdin swap, has not covered the branch that
      keeps the tool from touching a database.
- [ ] Every test that can reach these handlers runs under `answering(t)`. No subtest overrides
      `DOCKER_HOST` or re-chdirs: `t.Chdir` is what stops `exec`, `DOCKER_HOST` is what stops
      `masquerade`, and each is load-bearing for exactly one of them.
- [ ] Expected `Dir` is read from `here()`/`os.Getwd()` inside the test and is **not**
      `f.project.Root`, whose `Root` is set to something no temp dir can equal. A comment names the
      divergence from `inside_test.go`.
- [ ] Expected `Options` is built by calling `terminalOptions(user)`, not by writing `Tty:` literally.
- [ ] `commands` has exactly sixteen methods, the new one being `Anonymise(dir string) error`.
- [ ] Exactly two call sites route through `newEngine`: `orchestrate.go:188` and `tools.go:54`. No
      other `engine(` site changed — in particular not `orchestrate.go`'s `checkServices`.
- [ ] No file under `console/`, `bin/run`, `data/` or `dockergento/` is modified, and no shell twin
      or bash suite is deleted. `adapters/toolinfo/tooling.go` is untouched.
- [ ] `go test ./... -short` passes; `tests/run.sh unit` passes, unmodified;
      `gofmt -l ./cmd ./internal` is empty; `go vet ./...` is clean. Baselines are re-read at apply,
      never carried from this document.
- [ ] Changed lines measured with the shortstat command above against this change's own parent, with
      the two-PR split applied, and no `size:exception`.
- [ ] Manual verification on a real project: `hm exec ls -lah`, `hm exec -r whoami`, `hm bash -c` via
      `hm bash`, the usage errors `hm exec` and `hm masquerade --nope` (exit 2), and
      `printf 'n\n' | hm masquerade` leaving the database untouched and exiting 0.
