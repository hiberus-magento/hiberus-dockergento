# Design: What `set-host` and `version` ask of the engine

## Technical Approach

No new mechanism, again. The seam is `commands` (`engine.go:17`, seven methods today), `newEngine`
(`engine.go:31`), the compile-time assertion (`engine.go:27`) and the ordered call log in
`fake_engine_test.go`. It grows by exactly three methods and three routed call sites, quoted verbatim
from `dockergento/dockergento.go`:

```go
Installed() (core.Installation, core.Tooling)      // :468
SetHost(dir, domain string, database bool) error   // :479
RemoveHost(domain string) error                    // :489
```

`down` is **not** in this change. Its estimate did not fit under the 400-line budget once the working
directory pin was priced (decision 1), so the cut the proposal planned for mid-apply is applied here
instead, up front: `Down` (`dockergento.go:235`), `down.go:61`, `outcome.result`, `call.Interactive`
and `call.DownOptions` all move to a follow-up change named `down-go-tests`. Everything already
decided about them is kept verbatim in the last section, so that change's design can lift it rather
than re-derive it.

Everything below is a decision about how the tests are written against the seam, and about what the
RED window is allowed to reach — which here is the load-bearing part, because while a call site is
unrouted the real engine runs against the developer's own machine and working tree.

## Architecture Decisions

### Decision 1: the RED window is bounded in `answering`, not in the tests that need it

`answering` (`fake_engine_test.go:130`) pins `HM_STATE_DIR` today. It gains a working directory and
four more variables, all unconditional, all in commit 1 so every later RED run in the package — this
change's and `down-go-tests`' — is already covered:

```go
t.Chdir(t.TempDir())
t.Setenv("HM_STATE_DIR", t.TempDir())
t.Setenv("HM_HOSTS_FILE", filepath.Join(t.TempDir(), "hosts"))
t.Setenv("HM_NON_INTERACTIVE", "1")
t.Setenv("DOCKER_HOST", "unix:///nonexistent")
t.Setenv("HM_LEGACY_ROOT", t.TempDir())
```

| Pin | What it stops, with the citation |
|---|---|
| **`t.Chdir`** | **The one that would have damaged the working tree.** `projectOr` is already routed, so the fake answers a *named* project and `set-host` goes on to the real `SetHost(here(), …)`. That resolves `here()` — `os.Getwd()`, the package directory — and `app/resolve.go:28` keeps `root = dir` outside a worktree, so `app/hosts.go:49` calls `Properties.Set(project.Root, "DOMAIN", …)` and `fsprops/properties.go:90-121` does `os.MkdirAll` + `os.WriteFile` + `os.Rename` on `<root>/config/docker/properties.json`. Unpinned, a RED run writes `internal/cli/config/docker/properties.json` into the checkout. A temporary cwd makes `here()` and `projectOr` resolve somewhere disposable instead. |
| `HM_STATE_DIR` | The real `engine()` opens its registry there; unset, that is the developer's own `~/.hm`. Already present. |
| `HM_HOSTS_FILE` | `Hosts.file()` falls back to the literal `/etc/hosts` (`app/hosts.go:181-187`), and `write()` copies over it through `exec.Command("sudo", "cp", …)` wired to `os.Stdin`/`os.Stdout` (`:173-176`). Pointed at a `t.TempDir()` with no `hosts` file in it, `Set` and `Remove` both fail at `os.ReadFile` (`:95`, `:120`) before anything is written. |
| `HM_NON_INTERACTIVE` | `choose()` refuses immediately when it is set (`select.go:39-44`), independently of any `interactive` bool. Nothing in this change reaches a question — it is here for `version`'s RED and for every RED run after it, `down-go-tests` included, and it costs one line. |
| `DOCKER_HOST` | `Installed()` reaches `ServerVersion(context.Background())` with no deadline of its own, so an unreachable socket is what makes `version`'s RED fail instead of wait. |
| `HM_LEGACY_ROOT` | Reached only when `point()` returns nil (decision 2): `Hosts.Set` then runs `magento config:set` through `legacy.Runner`, which execs `<root>/bin/run` (`legacy/runner.go:47`) and takes its root from `HM_LEGACY_ROOT` when `ShellRoot` is empty (`:89-96`) — and `internal/cli/engine.go:42-68` never sets `ShellRoot`. A developer with that variable exported would have a RED run shell out to the Bash half. An empty temporary directory makes the exec fail with ENOENT. |

Verified inert for what already exists: no current caller of `answering` reads any of the four
variables (`rg 'HM_NON_INTERACTIVE|DOCKER_HOST|HM_HOSTS_FILE|HM_LEGACY_ROOT' internal/cli` finds only
production files and `select_test.go:61`, which sets its own and never calls `answering`).

| Option | Trade-off | Decision |
|---|---|---|
| Working directory and all four variables in the helper, unconditionally | One place; a test written later cannot forget. Costs ≈56 changed lines in the tests that already exist (below), which is what pushed `down` out of this change | **Chosen** |
| `t.Chdir` only in the one `set-host` subtest whose RED reaches `Properties.Set` | One line, no existing test touched, `down` stays — and exactly the "protects only the run somebody remembered to type" failure mode the other pins were centralised to avoid. A budget is not a reason to make a safety pin optional | Rejected |
| Pins in each test that needs them | Same failure mode, spread wider | Rejected |

**Consequence for the tests that already exist.** `t.Chdir` moves the process working directory for
the duration of each test, so `here()` no longer equals the directory the test binary started in.
Ten places capture `cwd` *before* `answering(t)` and compare the log against it —
`inside_test.go:22,55,113` and `wrappers_test.go:25,64,123,173,249,283,307` — and every one of them
would fail. Each is the same four-line block:

```go
cwd, err := os.Getwd()
if err != nil {
    t.Fatalf("os.Getwd() = %v, want no error", err)
}
```

It is replaced by `cwd := here()` placed *after* the `answering(t)` call that moved the directory,
which for a table-driven test means inside the subtest. `here()` rather than `os.Getwd()`: it is the
same call the handler under test makes (`engine.go:90-97`), it cannot disagree with it, and it needs
no error branch. `os` then has no other use in either file, so its import goes too. Ten blocks
removed, fourteen lines added, two imports dropped: **≈56 changed lines, all in commit 1**, because
the pin and the tests it breaks have to land together. The commit message says so — a reviewer
opening that diff sees seven unrelated tests edited, and the reason has to be in the message rather
than only here.

`t.Chdir` also forbids `t.Parallel` in the test or any parent. The package has never used it —
`rg 't\.Parallel' internal/cli` is empty — and the testing strategy below says why it cannot start.

### Decision 2: `shop.test` stays the test domain, and the lookup it costs is bounded

`Hosts.Set` reaches `point()`, which asks `Resolver.ResolvesLocally(domain)` —
`net.LookupHost` (`machine/resolver.go:22`) — *before* the `os.ReadFile` that `HM_HOSTS_FILE` guards.
So the RED window of the `set-host` default path performs one DNS lookup.

| Option | Trade-off | Decision |
|---|---|---|
| `shop.test` | `.test` is reserved by RFC 6761, so a resolver answers NXDOMAIN without forwarding, and offline it fails immediately. `ResolvesLocally` is false, `point()` goes on to `os.ReadFile` and stops there. One read-only lookup, nothing mutated, and the domain is the one the tool's own hints use (`wrappers.go:273`, `app/hosts.go:116`) | **Chosen** |
| `localhost` | Resolves to loopback deterministically and with no query — but that makes `ResolvesLocally` **true**, so `point()` returns nil and `Set` goes *on* to the `magento config:set` branch. It buys a shorter lookup by opening the deeper path, and it makes the shipped assertion read as though somebody wanted to point Magento at `localhost` | Rejected |
| A name chosen to be unresolvable everywhere | Same NXDOMAIN as `.test`, less readable, and no stronger a guarantee | Rejected |

The proposal's "no network" criterion is about the suite as shipped, and it holds exactly: in GREEN
the fake answers `SetHost` and no lookup happens at all. The single lookup exists only while the call
site is unrouted, it mutates nothing, and it fails fast both online and offline. The one case where
it *does* resolve locally — a developer who has `shop.test` in their own `/etc/hosts`, which this
tool puts there — lands on the `legacy.Runner` branch, which the `HM_LEGACY_ROOT` pin bounds.

`--remove` is clean by construction: `RemoveHost` never resolves a project (`dockergento.go:489`),
and `Hosts.Remove` refuses an empty domain and then reads the pinned file (`app/hosts.go:110-121`).
No cwd, no lookup, no properties file.

### Decision 3: two new test files; the two that exist are edited only for the cwd pin

| File | Holds | Lines |
|---|---|---|
| `version_test.go` (new) | `version`'s document, its text block, its usage error | ≈85 |
| `set_host_test.go` (new) | both `set-host` paths, the refusal, the usage error | ≈120 |

The proposal put `set-host` into `wrappers_test.go` "beside the copy tests". Measured, that file is
**398 lines**, not the ~360 the estimate assumed; adding ≈120 makes it ≈518 across four unrelated
themes. `setHost` is also the only wrapper with two exclusive paths and a `projectOr` it skips, which
is a header's worth of explanation of its own. Own file. Rejected: one `hosts_test.go` shared with a
future `set-host` sibling that does not exist yet.

`wrappers_test.go` and `inside_test.go` are still modified, but only by decision 1's `cwd := here()`
adjustment. No case is added to either.

### Decision 4: the cases, taken from the handlers and the branch tables

| Test | Cases |
|---|---|
| `TestWhatVersionReports` | `--json`: decode the envelope, delete `binary`, `cmp.Diff` the rest against a literal map (numbers arrive as `float64`), then `binary == buildOfThisBinary()` separately — `version.go:31`, and what the parity suite already does. Text: a detached checkout with nothing underneath, which is the only coverage of `orUnknown` ("unknown (detached checkout)", `version.go:42`) and `orMissing` ("not available", `:90-96`), down to the `hm switch --list` footer. Log is `{Method: "Installed"}` alone — no `Resolve`, because `version` asks about no project |
| `TestAnArgumentNobodyDeclaredIsAUsageError` | `--tonteria` **and** a bare `extra`: `version.go:18-21` returns on the first argument whatever it looks like, so the positional is a usage error too. `exitUsage`, empty log |
| `TestWhatSetHostAsks` | `shop.test` → `Resolve(cwd)` + `SetHost{Dir: cwd, Domain: "shop.test", Database: true}`, `cwd` read with `here()` after `answering`; `--no-database` flips it; `--json` → `data` `{"domain","database"}` |
| `TestRemovingAHostAsksNothingAboutTheProject` | `--remove shop.test` with `fake.project` **left at its zero value**: the log is `[{Method:"RemoveHost", Domain:"shop.test"}]` and the code is `exitOK`. A routed `projectOr` against a nameless project would have answered `exitProject`, so this is what proves `wrappers.go:280-290` skips it. Plus `--json` → `{"removed": …}`, and `--remove` alone → `Domain: ""` forwarded, the missing guard pinned rather than fixed |
| `TestARefusedHostEditIsReported` | outcome `core.Refusal{Kind:"no_domain", Code:2, Message:"There is no domain to remove", Hint:"hm set-host --remove shop.test"}` (`app/hosts.go:111-118`) → that code, that message, that hint on stderr, rather than a generic `exitDocker`. It is this change's only exercise of `report()`'s refusal branch (`orchestrate.go:236-239`), and the delta spec's "Pointing a domain at this machine" scenario now carries the AND clause it answers |
| `TestASetHostOptionNobodyDeclaredIsAUsageError` | `-x` → `exitUsage`, empty log (`wrappers.go:271-274`) |

The `version` text subtest pins `NO_COLOR=1` (`palette.go:11`) and `COMMAND_BIN_NAME=hm`
(`engine.go:102`), so neither the terminal nor the invocation name decides the assertion.

### Decision 5: what RED actually fails on, per commit

The pins land first, so both steps below are bounded before they start. RED is a failing assertion in
each, never a hang and never a write outside the test.

- **`version`** (commit 1) — the real `Installed()` spawns `git` against the test binary's own
  directory and dials the daemon; `DOCKER_HOST` makes that dial fail rather than wait. The log has no
  `Installed` entry and the document carries this checkout's description instead of the fake's.
- **`set-host`** (commit 2) — `--remove` reaches `Hosts.Remove`, which reads the pinned
  `HM_HOSTS_FILE`; that file does not exist inside the fresh `t.TempDir()`, so `os.ReadFile` fails
  and `write()` is never reached. The default path first writes `DOMAIN` into a `properties.json`
  under the *temporary* cwd, then looks `shop.test` up (decision 2), then fails at the same
  `os.ReadFile`. Either way `report()` answers `exitDocker` and the log has no `SetHost`/`RemoveHost`
  entry.

**RED and GREEN command**: `go test ./internal/cli -short`. The predecessor's
`DOCKER_HOST=unix:///nonexistent` prefix is now redundant and is dropped on purpose — a prefix
protects only the run somebody remembered to type, and the helper protects every run. GREEN adds
`go test ./... -short`.

### Decision 6: `MIGRATION.md` is not touched

`MIGRATION.md:91-93` already states the rule in general terms — a Go-wired command is proved against
`newEngine` rather than Docker, with no daemon, no network and no real project. Two names added to a
document that deliberately lists none would be noise, and the table already marks both `go`. The
documentation this change owes is the `answering` doc comment, which explains the working directory
and all four variables with the citation for each; it ships with commit 1 and is never cut for size.

## Data Flow

    version ────────────────────────────→ Installed()             (no projectOr)
    set-host --remove ──────────────────→ RemoveHost(domain)      (no projectOr)
    set-host <domain> ──→ projectOr ──→ Resolve ──→ SetHost(here(), domain, database)
                                                       ↓
                                              newEngine ──┬─→ *dockergento.Engine (production)
                                                          └─→ fakeEngine (ordered log + answers)

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/cli/engine.go` | Modify | `commands` gains `Installed`, `SetHost`, `RemoveHost` |
| `internal/cli/version.go` | Modify | `:23` routes through `newEngine` |
| `internal/cli/wrappers.go` | Modify | `:281` and `:296` route through `newEngine` |
| `internal/cli/fake_engine_test.go` | Modify | Three methods; the `installed`/`tooling` answers; `call.Domain`, `call.Database`; the temporary cwd, the four variables and the doc comment in `answering` |
| `internal/cli/wrappers_test.go` | Modify | Seven `cwd` captures move after `answering` as `cwd := here()`; the `os` import goes |
| `internal/cli/inside_test.go` | Modify | The same, three captures |
| `internal/cli/version_test.go` | Create | `version`'s two tests |
| `internal/cli/set_host_test.go` | Create | `set-host`'s four tests |

`internal/cli/down.go` is **not** touched. No file under `console/`, `bin/run`, `dockergento/` or
`tests/` changes.

## Interfaces / Contracts

```go
// call gains, beside Key and Path:
Domain   string
Database bool

// fakeEngine gains, for the method that only ever answers:
installed core.Installation
tooling   core.Tooling
```

`Installed` logs `{Method: "Installed"}` and consumes no outcome, as `Property` does. `SetHost` logs
`{Method: "SetHost", Dir, Domain, Database}` and `RemoveHost` logs `{Method: "RemoveHost", Domain}`;
both consume an outcome, the way `Dump` does. `outcome` is unchanged in this change — `result` is
`Down`'s, and it travels with it.

## Testing Strategy

| Layer | What | How |
|---|---|---|
| Unit | What each handler asks of the engine | `newEngine` substituted by `fakeEngine`; the whole log compared with `cmp.Diff` |
| Unit | What each handler prints | `--json` decoded from a buffer; text compared under `NO_COLOR=1` |
| Integration | That the fake and the engine agree | Unchanged Bash parity suites, which run both halves against a real environment |

No `t.Parallel()`, now for two reasons: `newEngine` and `t.Setenv` are process state, which is what
the package already assumed, and `t.Chdir` panics in a parallel test or one with a parallel parent.

## Threat Matrix

| Boundary | Applicability |
|---|---|
| Documentation-like paths | N/A — no file is classified or executed |
| Git repository selection | N/A — no `git -C` or path selector is composed; the only `git` in reach is the one the real `Installed()` spawns during RED, bounded by decision 5 |
| Commit state | N/A — nothing touches the index or the worktree |
| Push state | N/A — no push |
| PR commands | N/A — no PR automation |

The real adversarial surface of this change is not in that matrix: it is the RED window's reach into
the working tree (`properties.json`), `sudo cp /etc/hosts`, `bin/run`, a DNS resolver and the daemon
socket. Decisions 1 and 2 answer all five, each with a citation, and everything they need ships in
commit 1 before any RED run.

## Migration / Rollout

No migration. No command's arguments, output, `--json` document or exit codes change.

## Size and Commits

≈317 changed lines, under the 400 budget with the `down` cut already applied. No `size:exception`.

| File | Action | Lines | Commit |
|---|---|---|---|
| `internal/cli/version_test.go` | Create | ≈85 | 1 |
| `internal/cli/wrappers_test.go` | Modify | ≈39 (28 removed, 10 added, 1 import) | 1 |
| `internal/cli/inside_test.go` | Modify | ≈17 (12 removed, 4 added, 1 import) | 1 |
| `internal/cli/fake_engine_test.go` | Modify | ≈30 (`Installed`, the two answers, the pins and their comment) | 1 |
| `internal/cli/engine.go` | Modify | 1 (`Installed`) | 1 |
| `internal/cli/version.go` | Modify | 1 | 1 |
| `internal/cli/set_host_test.go` | Create | ≈120 | 2 |
| `internal/cli/fake_engine_test.go` | Modify | ≈18 (`SetHost`, `RemoveHost`, `call.Domain`, `call.Database`) | 2 |
| `internal/cli/engine.go` | Modify | 2 (`SetHost`, `RemoveHost`) | 2 |
| `internal/cli/wrappers.go` | Modify | 2 | 2 |

| # | Commit | Lines | Running total |
|---|---|---|---|
| 1 | `test(cli): what version asks of the engine, and where RED is allowed to reach` | ≈173 | ≈173 |
| 2 | `test(cli): what set-host asks of the engine` | ≈142 | ≈315 |

Commit 1 carries `Installed`, `version.go:23`, the two answer fields, the temporary cwd, the four
variables, their doc comment, the ten `cwd := here()` adjustments and `version_test.go`. Its message
must say why seven tests it does not otherwise touch are edited: the working-directory pin moved the
ground under them. Commit 2 carries `SetHost`, `RemoveHost`, `wrappers.go:281,296`, `call.Domain`,
`call.Database` and `set_host_test.go`.

Both are self-consistent: `go test ./internal/cli -short` and `go test ./... -short` pass at each,
and `var _ commands = (*dockergento.Engine)(nil)` (`engine.go:27`) compiles at each.

**Size gate, now a sanity check.** After commit 2, measure with
`git diff --shortstat <base>..HEAD -- . ':(exclude)openspec'` and nothing added to it. **Nothing
remains to cut** — `down` is already gone, `version` was deferred once and is never cut again, and
comments, blank lines and test cases are never cut to fit. So if the total exceeds 400, stop and
report it rather than shrinking anything: the estimate was wrong by ≈85 lines and that is worth
knowing before the next change repeats it.

## Open Questions

- [ ] `set-host` passes `here()` while the resolved project's root is available — the same
      `here()`-versus-`project.Root` inconsistency the two predecessors left open. It is what made
      the working-tree write in decision 1 possible, so it is now a follow-up with a reason.
- [ ] `set-host` forwards an empty domain with no CLI-level guard, leaving the refusal to
      `app/hosts.go:40-47` and `:110-118`. Pinned, not fixed, as the proposal's non-goals say.

## Carried to `down-go-tests`

Everything below was decided for `down` and verified at HEAD `7417863`. It is kept verbatim so the
follow-up change lifts it rather than re-deriving it. Nothing here is implemented by this change.

**Seam growth it needs.** `commands` gains `Down(dir string, options core.DownOptions, interactive bool) (string, error)`
(`dockergento/dockergento.go:235`); `down.go:61` moves from `engine(` to `newEngine(`. `outcome`
gains `result string`, read the way `Exec` reads `status`; `call` gains `Interactive bool` and
`DownOptions core.DownOptions`. `core.DownOptions` is all exported (`core/orchestration.go:44-58`),
so `cmp.Diff` needs no new option. The fake's `Down` logs
`{Method: "Down", Dir, DownOptions, Interactive}` and answers `(out.result, out.err)`.

**Estimate.** `down_test.go` ≈150, fake growth ≈13, `engine.go` 1, `down.go` 1 — ≈165 changed lines,
one commit, comfortably inside a budget of its own. The six pins it depends on already ship here, so
it inherits them at no cost.

**The cases.**

| Test | Cases |
|---|---|
| `TestWhatDownAsks` | Table: nothing; `-v`; `--volumes`; `--remove-orphans`; `--rmi all`; **`--rmi` last** and **`-t` last** → `core.DownOptions{}`, silently ignored (`down.go:31-38`), pinned rather than fixed; `-v --remove-orphans --rmi all -t 30` → `{Volumes: true, RemoveOrphans: true, Images: "all", Timeout: &thirty}` with `thirty := 30` a local — `cmp` follows pointers, so no option is needed for `*int`. Every row's log is `Resolve(cwd)` + `Down{Dir: cwd, DownOptions: want, Interactive: false}` |
| `TestDownAsksInteractivelyWhenNobodySaidOtherwise` | nested `t.Setenv("HM_NON_INTERACTIVE", "")` → `Interactive: true` (`down.go:59`) |
| `TestWhatDownPrints` | `result:"destroyed"` + `--json` → `{"destroyed":true,"volumes":false,"snapshot":false}`; `result:"saved"` + `-v --json` → `snapshot:true`; `result:""` → `"Nothing was destroyed.\n"` under `NO_COLOR=1`; **`result:"" with --json` → the same sentence on stdout and no document**, `down.go:67-71` reached before the `jsonOutput` branch — found, not fixed, and the assertion says so in a comment |
| `TestHowDownFails` | `errNothingChosen` (`select.go:29`) → `exitInterrupted` and `"\nNothing was chosen.\n"` (`orchestrate.go:230-234`); `core.Refusal{Kind:"snapshot_failed", Code:1, Message:"The snapshot failed, so nothing was destroyed", Hint:"hm down -v   # to destroy anyway"}` (`app/orchestrate.go:358-363`) → code 1, its own message and hint, **not** `exitDocker`; anything else → `exitDocker` |
| `TestADownOptionNobodyDeclaredIsAUsageError` | `--tonteria` → "Unknown option: --tonteria", hint `hm down [-v]`; `-t nada` → "The timeout is a number of seconds: nada", hint `hm down -t 30`. Both `exitUsage` with an empty log |

**What `down`'s RED fails on.** `projectOr` is already routed, so the fake answers a project; then
the real `Engine.Down` resolves the temporary cwd into a nameless project (a directory that is not a
project is not an error, `dockergento.go:125-135`). `presentVolumes` only reads — `Configuration` is
a `composecfg.Loader.Load` over files on disk (`dockergento.go:1132-1135`) — and returns nil because
there are none. `refuseFromAnUnregisteredWorktree` returns nil for a project that is not in a
worktree (`app/orchestrate.go:249-252`). The run then stops inside `Orchestrator.open`: a Docker CLI
is constructed and initialised without dialling anything, and `load` refuses with **"no compose file
to read in `<temporary cwd>`"** (`composelib/orchestrator.go:205-228`). That, not the unreachable
socket, is what makes `down`'s RED safe and immediate. `Choose` is unreachable twice over —
`len(volumes) == 0` (`app/orchestrate.go:326`) and the `HM_NON_INTERACTIVE` pin. The log has no
`Down` entry.

**Ordering instruction for its apply.** `TestDownAsksInteractivelyWhenNobodySaidOtherwise` clears
`HM_NON_INTERACTIVE`, which removes one of the pins for the length of that subtest. Write it in the
GREEN step, after `down.go:61` already calls `newEngine`. It has no RED step and needs none: the
routing it would prove is proved by every other row of `TestWhatDownAsks`, which does run RED first,
and the only thing this subtest adds on top of them is the value of one boolean. Once the site is
routed the fake records the boolean and returns, so `Operator.Down` — the only caller of `Choose`
(`app/orchestrate.go:326,350`) — is never reached at all.

**Spec.** The five `down` scenarios are removed from this change's delta spec and belong to
`down-go-tests`.

**Its own open questions.**

- [ ] `down` prints "Nothing was destroyed." on stdout even under `--json`, where a program reading
      the document finds a sentence in front of it. Pinned there; whether the sentence should move to
      stderr is a follow-up of its own.
- [ ] `--rmi` and `-t` as the last argument are silently ignored, unlike `hm logs`, which refuses a
      value-taking flag with nothing after it (`orchestrate.go:97-101`). The tests make the
      inconsistency visible; reconciling it is not that change either.
