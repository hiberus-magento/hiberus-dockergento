# Design: What `list`, `describe` and `logs` ask of the engine

## Technical Approach

No new mechanism. The seam is `commands` (`engine.go:17-30`, **twelve** methods today), `newEngine`
(`engine.go:36-38`), the compile-time assertion (`engine.go:32`) and the ordered call log in
`fake_engine_test.go`. It grows by exactly three methods and three routed call sites, quoted verbatim
from `dockergento/dockergento.go` and re-verified at HEAD `382247c`:

```go
Environments() ([]core.Environment, error)                          // :140
Describe(dir string, withSecrets bool) (core.Description, error)    // :152
Logs(dir string, services []string, options core.LogOptions) error  // :322
```

Twelve plus three is fifteen, so the proposal's count holds. Everything below is a decision about
how the tests are written and about what the RED window is allowed to reach — which here, again, is
the load-bearing part, because while a call site is unrouted the real engine runs against the
developer's own machine.

One thing was added to the cycle after the proposal: **four hardcoded hints that this cycle's own
tests would otherwise turn into a requirement**. They are fixed first, in a unit of their own, before
any test asserts them. See Decision 6.

### Where the proposal and exploration turned out to be wrong

Both artifacts' line citations check out — unlike the last two cycles, **no drift was found**. One
substantive claim does not.

> Proposal Decision 2 / Exploration §C: "`Describe` and `Logs` both call `e.Resolve(dir)` as their
> first line, and the empty temp cwd pinned by `t.Chdir(t.TempDir())` **fails them** before any
> adapter is touched."

`Resolve` does not fail. `dockergento.go:128-131` says so in as many words — "A directory that is not
a project is not an error … the answer is a project with no name" — and `app/resolve.go:24-26`
repeats it. Outside a repository `gitvcs.Git.Resolve` returns `(dir, false, "", nil)`
(`gitvcs/vcs.go:32`), and a missing `properties.json` is not an error either. So `Resolve` returns a
**nameless project rooted at the temp cwd, with a nil error**, and both handlers carry on.

For `logs` that changes nothing that matters (it stops at the Compose loader, below). For
**`describe` it changes the whole risk assessment**: `app.Describer.Describe` fans out seven
concurrent questions (`app/describe.go:48-70`) *before* it looks at any error, and two of them are
`d.Engine.Containers()` — the same machine-wide `ContainerList` as `list` — and
`d.Tooling.Xdebug(project.Name)`, which does a `ContainerList` and then
`ContainerExecCreate`/`ContainerExecAttach` **inside a matching container**
(`toolinfo/tooling.go:67-124`).

**`describe`'s RED reaches the daemon exactly as `list`'s does.** `DOCKER_HOST` is load-bearing for
two of the three commands, not one. The proposal's *conclusion* — no new pin is needed — survives;
its reasoning does not, and Decision 2's "safe by construction" sentence must not be repeated in a
test file comment.

**And it is worse than "reaches the daemon."** Follow `Xdebug` with the nameless project in hand.
`toolinfo/tooling.go:84-85` matches on

```go
matches := item.Labels["hm.project"] == project ||
	(item.Labels["hm.project"] == "" && item.Labels["com.docker.compose.project"] == project)
```

With `project == ""` the **first** disjunct — `item.Labels["hm.project"] == ""` — is true of every
container that does not carry the label. Combined with the `com.docker.compose.service == "phpfpm"`
test at `:87`, `Xdebug` selects the first unlabelled `phpfpm` container on the machine, which is any
Magento environment not created by this tool or created before the labels existed. It then runs
`ContainerExecCreate` and `ContainerExecAttach` (`:98-112`) **inside it**.

So the risk of `describe`'s RED is not that it reads a container list. It is that **it execs into
somebody else's running container**. The command is a read-only `grep`, which is luck rather than
design — nothing in the CLI under test chose it. `DOCKER_HOST=unix:///nonexistent` is the only thing
between the two, and that is the strongest reason in this document for treating that one pin as
load-bearing rather than tidy.

## Architecture Decisions

### Decision 1 — how the fake answers a method that returns data

`Environments` and `Describe` return values. Two precedents already exist and they are the same
shape: `Resolve` (`fake_engine_test.go:83-92`) answers from `f.project`, and `StopEverything`
(`:168-177`) answers from `f.stopped` — each takes the outcome first, returns the **zero value** with
the error when there is one, and the answer field with `nil` when there is not. `Installed`
(`:144-148`) is the other shape: answers only, consumes no outcome, does not even read `len(f.calls)`.

| Option | Trade-off | Decision |
|---|---|---|
| Answer field + `outcomeFor(number)` (the `Resolve`/`StopEverything` shape) | Two fields, no new mechanism, and a test states a failure exactly where it already states one for `Dump` or `SetHost` | **Chosen** for both |
| The `Installed` shape (answer only, never fails) | Wrong: `list.go:24-27` and `describe.go:26-30` both have an error branch with its own exit code, and an unfailable fake could not reach either | Rejected |
| A `func() ([]core.Environment, error)` hook per method | Lets one test express anything, and destroys the property that makes this fake readable — that `outcomes` is one list indexed by call number | Rejected |

`fakeEngine` gains two fields; `call` gains two. `outcome` is unchanged.

```go
// fakeEngine gains, beside project, properties, installed, tooling and stopped:

	// environments is what Environments answers with, when its own outcome does not fail it —
	// the same shape as project for Resolve. Environments takes no directory, so there is
	// nothing else to answer from.
	environments []core.Environment

	// description is what Describe answers with, on the same terms.
	description core.Description

// call gains, beside Key, Path, Domain, Database and Interactive:

	// WithSecrets is whether describe was asked for the credentials. It is the only argument of
	// that call besides the directory, and it is the one worth a regression: false by default is
	// what keeps a database password out of a description nobody asked to contain one.
	WithSecrets bool

	// LogOptions is everything `hm logs` was asked for, enumerated. Compared whole, so a flag
	// that starts writing a different field shows up as a diff rather than as nothing.
	LogOptions core.LogOptions
```

`call.Services` is **reused** for `Logs` — `Restart` already records services there. Adding fields to
`call` breaks no existing `want` literal: every one of them keyword-constructs, so the new fields take
their zero values on both sides of `cmp.Diff`.

```go
// Environments, like Resolve, answers from a field and can still be refused: an unreachable
// daemon is what list.go:24 branches on. It takes no directory, so the call records only its
// name — the same single-field entry version's Installed leaves.
func (f *fakeEngine) Environments() ([]core.Environment, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Environments"})

	if out := f.outcomeFor(number); out.err != nil {
		return nil, out.err
	}

	return f.environments, nil
}

// Describe answers from f.description on the same terms. WithSecrets is recorded rather than
// acted on: what the credentials are withheld from is app.Describer's decision, and this seam's
// job is to prove the flag arrives unchanged.
func (f *fakeEngine) Describe(dir string, withSecrets bool) (core.Description, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Describe", Dir: dir, WithSecrets: withSecrets})

	if out := f.outcomeFor(number); out.err != nil {
		return core.Description{}, out.err
	}

	return f.description, nil
}

// Logs, like Restart, is asked to do something and returns only whether it could.
func (f *fakeEngine) Logs(dir string, services []string, options core.LogOptions) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Logs", Dir: dir, Services: services, LogOptions: options})

	return f.outcomeFor(number).err
}
```

`Environments` returns a **nil** slice with an error, not `[]core.Environment{}`: that is what the
real `app.Inventory.Environments` returns (`app/environments.go:26-28`), and the handler discards it
either way, so matching the real signature costs nothing and misleads nobody.

### Decision 2 — the RED plan, and the one command with no net

`answering(t)` (`fake_engine_test.go:212-229`) pins six things. **No new pin is needed.** Which pin
carries which unit, read from the code rather than assumed:

| Unit | What RED actually reaches, before routing | Pin that bounds it | Where it stops |
|---|---|---|---|
| `list` | `Environments()` takes no directory: straight to `app.Inventory{…}.Environments()` (`dockergento.go:140-146`) → `dockerd.Engine.Containers()` → `connect()` → `ContainerList` **across the whole machine** | **`DOCKER_HOST` alone.** `dockerd.Endpoint()` gives `DOCKER_HOST` precedence over the context store (`endpoint.go:21-31`), and `client.FromEnv` reads it too, so `unix:///nonexistent` wins over Docker Desktop or Colima | dial `ENOENT`, immediately |
| `describe` | `Resolve(cwd)` **succeeds** (nameless project at the temp cwd), then seven concurrent asks: `Compose.Load` (temp root), `Engine.Containers()`, `Magento.*` file reads, `Tooling.Version()` (`git describe`, cwd), `ComposeVersion()` (`docker compose version --short`), **`Xdebug("")` — which execs into the first unlabelled `phpfpm` container on the machine**, `State.Anonymisation` | **`DOCKER_HOST`** for the two daemon calls, and it is the only thing stopping the exec above; **`HM_STATE_DIR`** for the anonymisation file; **`t.Chdir`** for every file read, which otherwise happens in the package's own checkout | `Compose.Load` returns "no compose file" → `configErr` → `exitProject` |
| `logs` | `projectOr` is already routed, so the fake answers. Then the real `Logs(cwd, …)`: `Resolve(cwd)` succeeds, `orchestrator.open` builds a docker CLI and `Initialize`s it **without dialling**, then `load` (`composelib/orchestrator.go:204-215`) | **`t.Chdir`.** `DOCKER_HOST` is belt-and-braces here; no daemon call is made at all | `"no compose file to read in <temp cwd>"` → `report()` → `exitDocker` |

**Is `DOCKER_HOST` sufficient for `list`?** Yes, and it is the only thing that is. Verified, not
assumed: `Endpoint()` returns `DOCKER_HOST` before consulting `~/.docker/config.json`, and the SDK
never reads docker *contexts* itself. A `unix:///nonexistent` socket fails the dial with `ENOENT`
rather than hanging, so the RED fails fast both online and offline.

What it is **not** sufficient against is a test that never calls `answering(t)`, or a subtest that
overrides the variable. That is a process problem, and it gets a process answer:

**Each unit's RED has exactly one legitimate failure signature. Any other signature means a pin did
not hold — stop the run and investigate before repeating it.**

| Unit | RED must fail with | The alarm |
|---|---|---|
| 0 (hints) | a **hint-equality diff** — `want "hm list"`, `got "hm list --help"` | Nothing to alarm about: unit 0 builds no engine and reaches no adapter. Its refusals return during parsing, which is why it needs no pin at all beyond `t.Chdir(t.TempDir())` |
| `list` | `list = 3, want exitOK` (`exitDocker`) | A **call-log diff** instead — `want [{Environments}]`, `got []` — means `Environments()` *succeeded*, which means it read this machine's live containers |
| `describe` | `describe = 4, want exitOK` (`exitProject`) | Same: a log diff at `exitOK` means the daemon answered |
| `logs` | `logs = 3, want exitOK` (`exitDocker`) | A hang means `open()` reached a socket it should never have reached |

Rejected as a defence: a guard subtest asserting `os.Getenv("DOCKER_HOST") == "unix:///nonexistent"`
straight after `answering(t)` set it. It is an assertion that cannot fail — the exact defect class
this repository has now found five times — and it would give false confidence in place of the
signature table above.

Also rejected: running RED with a `DOCKER_HOST=… go test` prefix. The predecessor dropped that on
purpose (`down-and-set-host-go-tests` design, Decision 5): a prefix protects only the run somebody
remembered to type.

### Decision 3 — the `Dir` a `logs` test must expect, and what the trap actually is

`orchestrate.go:166-167` calls `Logs(here(), …)` — `os.Getwd()` (`engine.go:94-102`) — and not the
`project.Root` the fake already answered from `projectOr` (`engine.go:84`). `inside()` does the
opposite, and `inside_test.go:35,65,122` assert `Dir: root` accordingly.

**The proposal calls this a "silently passing test". It is not.** A `want` of `Dir: root` against a
`got` of the temp cwd fails loudly on `cmp.Diff`. The real hazard is what a reader does with that
failure: the obvious "fix" is to change `orchestrate.go` to pass `project.Root`, which is a behaviour
change this cycle forbids. So the mitigation is not a device against silence — it is an explanation
at the point of failure, plus a fixture that makes the assertion *able* to discriminate.

**The fixture must also name the project.** `projectOr` refuses on an empty name —
`engine.go:85` is `if err != nil || project.Name == ""` — so a fixture that set only `Root` would
answer `project_not_configured` and **every `logs` happy-path row would die at `exitProject` before
`Logs` was ever called**. The log would hold `Resolve` alone. Loud rather than silent, but it belongs
here rather than in whoever debugs it. The fixture is therefore, in full and in both `logs` tests
that reach the engine:

```go
fake.project = core.Project{Name: "shop", Root: "/code/shop"}
```

which is `inside_test.go:24`'s value verbatim. `Name` is what gets past `projectOr`; `Root` is what
makes the `Dir` assertion below able to discriminate. Neither is decorative.

| Option | Trade-off | Decision |
|---|---|---|
| A comment at the assertion **plus** `fake.project.Root = "/code/shop"`, a path that can never equal a `t.TempDir()` cwd | The comment tells the reader the `want` is right and the impulse is wrong; the distinct root is what makes `Dir: cwd` fail if anybody "reconciles" `logs` with `inside` | **Chosen** |
| A named helper, e.g. `whereTheCommandRan() string { return here() }` | An indirection the reader must follow to learn nothing the call site did not already say, and it disappears from the diff exactly where the explanation is needed | Rejected |
| Leaving `f.project.Root` empty | The assertion still passes, but it no longer discriminates against the `project.Root` variant in any way a reader can see | Rejected |

The comment, written once, at the `Logs` entry of the want:

```go
// Dir is the test's working directory, not f.project.Root: orchestrate.go:166 asks about
// here() (os.Getwd()) and never about the project projectOr just resolved. inside_test.go
// asserts Dir: root for the opposite reason — inside() passes project.Root — so this is where
// the two precedents diverge. If this line ever fails, the want is right and the change to
// orchestrate.go is the defect.
```

The three call logs, verified against the handlers:

| Command | Recorded calls |
|---|---|
| `list` | `[{Environments}]` — no `Resolve`, no `Dir`: `list.go` never calls `projectOr` |
| `describe` | `[{Describe, Dir: cwd, WithSecrets: …}]` — `describe.go` never calls `projectOr` either, so `f.project` is irrelevant to it |
| `logs` | `[{Resolve, Dir: cwd}, {Logs, Dir: cwd, Services, LogOptions}]` |

### Decision 4 — `checkServices` stays off the seam, and what that costs a test

Confirmed as the proposal decided it: `commands` stays at fifteen methods and `orchestrate.go:200`
keeps calling the concrete `engine(`. The consequence has to be in `logs_test.go`'s own header, not
discovered later:

> A row that names a service still reaches the **real** `engine(nil, nil, jsonOutput).Configuration(project)`
> inside `checkServices` (`orchestrate.go:198-203`). With `f.project.Root` set to `/code/shop`, the
> compose loader finds nothing there, returns an error, and `:201-203` treats **any** error as "skip
> validation". That is deterministic **only while no test creates a compose file under
> `f.project.Root`** — and since that root does not exist on any machine, nothing can.
>
> `Configuration` is off the seam, so it never appears in `fake.calls`. A want that listed it would
> never match.

### Decision 5 — one new test file per command, and `fake_engine_test.go` grown three times

| File | Action | Holds |
|---|---|---|
| `internal/cli/list_test.go` | Create | `list`'s log, its two renderings, its refusals |
| `internal/cli/describe_test.go` | Create | `describe`'s flag, its two renderings, its refusals |
| `internal/cli/logs_test.go` | Create | `logs`' option table, its `Dir`, its refusals |

Rejected: one `read_only_test.go`. The three share nothing but the fake, they land in three separate
commits, and a single file would be ~750 lines across three unrelated themes — the same measurement
that moved `set-host` out of `wrappers_test.go` last cycle.

`answering(t)` and its doc comment are **not** edited. The six pins already cover these three; the
only thing the comment could gain is the correction in Decision 2 above, and that belongs in the two
test files whose RED actually depends on it, next to the assertion that would catch it.

### Decision 6 — the hints are fixed before the tests can cement them

Four production strings hardcode a hint. **Three of them are wrong twice over; the fourth is wrong
once**, and the distinction matters because it is what makes them one fix rather than two:

| Site | Today | Wrong because |
|---|---|---|
| `list.go:20` | `"hm list --help"` | `--help` is an argument, so `len(args) > 0` refuses it identically — the hint is a closed loop |
| `describe.go:21` | `"hm describe --help"` | `--help` falls into the same `default:` branch. Verified live: `hm --json describe --help` answers `{"type":"invalid_argument","message":"Unknown option: --help","hint":"hm describe --help"}` |
| `doctor.go:24` | `"hm doctor --help"` | `--help` has no `--only=` prefix, so it lands in the same refusal |
| `describe.go:29` | `"hm setup"` | Not circular, but hardcoded; `engine.go:88` emits `binaryName()+" setup"` for the **identical** `project_not_configured` refusal three files away |

And all four hardcode `hm`. `binaryName()` (`engine.go:106-112`) exists because the tool can be
invoked under another name through `COMMAND_BIN_NAME`, so a user who renamed the binary is told to
run a command that does not exist for them.

**Those three are the only `--help` hints in the package** — `describe.go:29` is not one of them; it
says `"hm setup"`, which is the hardcoded-binary-name half of the defect without the circular half.
Counted: thirty-eight `failure()` and `warning()` sites build their hint with `binaryName()+`, and
**not one** of them points at `--help`. The convention is to show the real usage, and it has two
shapes:

| Shape | Precedent |
|---|---|
| A command that takes nothing → the bare command | `version.go:20` → `binaryName()+" version"`; `tools.go:28` → `binaryName()+" masquerade"` |
| A command that takes options → the real usage in brackets | `down.go:57` → `" down [-v]"`; `clean.go:27` → `" clean [--force]"`; `orchestrate.go:142` → `" logs [-f] [service...]"`; `db.go:106` → `" db freeze --name=base"` |

So, reading each command's actual option surface rather than inventing one:

| Site | Becomes | Why that shape |
|---|---|---|
| `list.go:20` | `binaryName()+" list"` | `list` takes **no options at all** (`list.go:18` refuses any argument). Exactly `version`'s case, and exactly `version`'s hint |
| `describe.go:21` | `binaryName()+" describe [--with-secrets]"` | `describe` takes exactly one option (`describe.go:16-18`). Exactly `clean [--force]`'s case |
| `doctor.go:24` | `binaryName()+" doctor [--only=ports]"` | `doctor` takes `--only=<id>` (`doctor.go:17`). `ports` is a real check id — `go_passthrough_test.sh:145` runs `doctor --only=ports`. Follows `db freeze --name=base`'s concrete-example shape |
| `describe.go:29` | `binaryName()+" setup"` | Makes it identical to `engine.go:88`, which is the same refusal |

**Should `--help` become a real per-command flag? No**, and not because it is out of scope but
because it is a different question. `hm --help` is a **tool-level** flag resolved before command
dispatch — `tests/integration/help_test.sh`, `tests/integration/cli_contract_test.sh:60-64`, and a
performance budget on it at `tests/performance/budget_test.sh:35`, which is a different suite
directory and not one of the integration ones. Making it per-command means deciding where the text lives
(`data/command_descriptions.json` already holds descriptions and `opts`), rendering it, and keeping
both halves in step. That is a help-system change. This cycle only stops the hint from lying.

**Does `doctor` come along?** **Yes.** Stated rather than smuggled, because the cycle's own
justification does not cover it: `doctor` gets no engine-seam test here, so nothing this cycle writes
would cement its hint. It comes along for a different reason — the string is character-for-character
the same defect, it is one line, and fixing three of the four instances would leave the fourth
reading like a deliberate exception to a convention it is simply violating. It brings its own test
(below), so it is not an unproved production change.

**Parity, checked rather than assumed.** The shell twin still says `--help`
(`console/commands/describe.sh:85-87` emits `$COMMAND_BIN_NAME describe --help`), so after this the
two halves' hint text **diverges**. That is safe: `go_passthrough_test.sh` runs
`both describe --nonsense` (`:80-83`), `both list --nonsense` (`:99-102`) and `both doctor --nonsense`
(`:189-192`), and every one of them asserts **only the exit code** and that the two halves agree on
it. No suite anywhere compares `.error.hint` for these three. Nothing under `console/` is touched, so
the proposal's non-goal holds; the divergence disappears when the deletion cycle removes the twins.

**Placement: its own unit, first.** Not folded into unit 1.

| Option | Trade-off | Decision |
|---|---|---|
| A unit of its own, before the three test units | The only ordering in which `list_test.go` and `describe_test.go` are *written against the corrected hint* rather than written, then edited. It also keeps units 1–3 test-only, so the proposal's rollback promise — "reverting removes the tests with nothing attached" — stays true of them | **Chosen** |
| Folded into unit 1 with `list` | Mixes a behaviour change into a test-only unit, and pushes unit 1 to ~281 for no gain | Rejected |
| A follow-up change after this cycle | Exactly the failure the fix exists to prevent: units 1 and 2 would ship assertions pinning the circular hint, and the follow-up would have to edit tests written days earlier to say something else. `down-and-set-host-go-tests` is mid-correction for doing this three times | Rejected |

Unit 0 therefore creates `list_test.go`, `describe_test.go` and `doctor_test.go` holding **only** the
unknown-option refusal tests. Those need no fake, no seam and no routing — the refusal returns during
parsing, before any engine call — which is the shape `down_test.go:11-18` already established for
exactly this. Units 1 and 2 then *add* to files that already exist.

**One knowing exception.** `describe.go:29`'s hint fires only when `Describe` returns an error, which
needs the fake, which needs the seam. It therefore ships in unit 0 and is proved in unit 2, one
commit later, inside `TestDescribeReportsAnUnresolvableProject`. The alternative — reaching it
through the unrouted real engine — is the daemon-reaching path of Decision 2 and must never become a
permanent test. Recorded as a bounded exception rather than hidden.

## Data Flow

    list ──────────────────────────────────→ Environments()              (no projectOr, no dir)
    describe [--with-secrets] ─────────────→ Describe(here(), withSecrets)  (no projectOr)
    logs [flags] [service...] ──→ projectOr ──→ Resolve(cwd)
                                      │
                                      ├──→ checkServices ──→ engine(...).Configuration   ← OFF SEAM
                                      └──→ Logs(here(), services, options)
                                                    ↓
                                           newEngine ──┬─→ *dockergento.Engine (production)
                                                       └─→ fakeEngine (ordered log + answers)

## File Changes

| File | Action | Unit | Description |
|---|---|---|---|
| `internal/cli/list.go` | Modify | 0 | `:20` hint → `binaryName()+" list"` |
| `internal/cli/describe.go` | Modify | 0 | `:21` hint → `binaryName()+" describe [--with-secrets]"`; `:29` → `binaryName()+" setup"` |
| `internal/cli/doctor.go` | Modify | 0 | `:24` hint → `binaryName()+" doctor [--only=ports]"` |
| `internal/cli/doctor_test.go` | Create | 0 | `doctor`'s unknown-option refusal — the whole of `doctor` in this cycle |
| `internal/cli/list_test.go` | Create → Modify | 0, then 1 | Unit 0 the refusal; unit 1 the log and the renderings |
| `internal/cli/describe_test.go` | Create → Modify | 0, then 2 | Unit 0 the refusal; unit 2 the flag, the renderings, the error |
| `internal/cli/engine.go` | Modify | 1, 2, 3 | `commands` gains `Environments`, `Describe`, `Logs` — one line each, one per unit |
| `internal/cli/list.go` | Modify | 1 | `:23` routes through `newEngine` |
| `internal/cli/describe.go` | Modify | 2 | `:25` routes through `newEngine` |
| `internal/cli/orchestrate.go` | Modify | 3 | `:166` routes through `newEngine`; **`:200` deliberately unchanged** |
| `internal/cli/fake_engine_test.go` | Modify | 1, 2, 3 | Three methods; `environments` and `description`; `call.WithSecrets` and `call.LogOptions` |
| `internal/cli/logs_test.go` | Create | 3 | Unit 3 |
| `openspec/changes/read-only-commands-go-tests/specs/go-entrypoint/spec.md` | Modify | 0, then 3 | Already exists. Unit 0 adds the hint scenarios; unit 3 completes the two requirements (excluded from the size gate) |

`list.go` and `describe.go` are each touched by two units — once for the hint, once for the routing —
and they are different lines. No file under `console/`, `bin/run`, `data/`, `dockergento/` or
`tests/` changes: the shell twins keep their `--help` hint, as Decision 6 records. `answering` is not
edited. `engine.go:32`'s assertion needs no edit — all three methods already exist on
`*dockergento.Engine`.

## Testing Strategy

| Layer | What | How |
|---|---|---|
| Unit | What each handler asks of the engine | `newEngine` substituted by `fakeEngine`; the whole log compared with `cmp.Diff` |
| Unit | What each handler prints | `--json` decoded into `map[string]any` and compared against a **literal** map; text compared against a **literal** string under `NO_COLOR=1` |
| Integration | That the fake and the engine agree | The unchanged Bash parity suites, which run both halves against a real environment |

Two rules carried from the predecessors and one added here:

1. **Never compare a rendering against another rendering of the same value.** The `--json` want is a
   hand-written literal, as `version_test.go:58-70` does. Re-marshalling the fixture and diffing it
   against stdout compares the binary with itself and passes through any tag rename.
2. **Never write the `want` with production's own format string.** `list`'s text want is a literal
   with the column padding spelled out, not a `fmt.Sprintf` of `list.go:63`'s format.
3. **No `t.Parallel()`.** `newEngine` and `t.Setenv` are process state, and `t.Chdir` panics in a
   parallel test or one with a parallel parent. `rg 't\.Parallel' internal/cli` is still empty.

### Test names: this document's set wins

Eight names diverge between this design and the delta spec (`TestListRefusesAnyArgument` versus
`TestListWithAnArgumentIsAUsageError`, and seven more). One document has to decide rather than leave
it to apply. **This one does, and the names below are authoritative.** Three reasons, in order of
weight:

1. **The spec's family does not compile.** `internal/cli` is one package and already holds
   `TestAnArgumentNobodyDeclaredIsAUsageError` (`version_test.go:128`) and
   `TestASetHostOptionNobodyDeclaredIsAUsageError` (`set_host_test.go`). More
   `…IsAUsageError` names in the same package are one collision away from a build failure, and the
   collision is silent until two files are open at once. Command-prefixed verb-first names cannot
   collide by construction.
2. **`TestWhat<Command>Asks` is unanimous** for the engine-log tests — `TestWhatPurgeAsks`,
   `TestWhatNpmAndMagerunAsk`, `TestWhatTheTestSuitesAsk`, `TestWhatSetHostAsks`,
   `TestWhatVersionReports`. This set already matches it.
3. **`Test<Command>Refuses…` is the live precedent** for a refusal table:
   `TestDownRefusesAMissingRmiValue` and `TestDownRefusesAMissingTimeoutValue`
   (`down_test.go:20,86`) are the most recently written tests in the package and are exactly the
   shape used here.

The spec's instinct is right on one point and it is worth conceding: `list` refusing *any* argument
is the same situation as `version`'s, and `version_test.go` names it
`TestAnArgumentNobodyDeclaredIsAUsageError`. That name is unavailable — reason 1 — so
`TestListRefusesAnyArgument` carries the same meaning under a name that can coexist with it.

### The cases, and what each would catch

Every case below states the regression it catches and how it fails. Cases that could not fail against
a plausible break are listed after the table and are **not planned**.

**Unit 0 — the hint fix (`list_test.go`, `describe_test.go`, `doctor_test.go`)**

Three tests of the same shape, one per command, each a two-row table. No fake, no `answering(t)`,
only `t.Chdir(t.TempDir())` — the refusal returns during parsing, so no engine is ever built. This is
`down_test.go:11-18`'s established shape.

**Both rows pin `COMMAND_BIN_NAME` explicitly**, the default row to `"hm"` and the renamed row to
`"hd"`, the way `version_test.go:86` already does. This is not belt-and-braces: `bin/run:4` exports
`COMMAND_BIN_NAME="hm"`, so a run under the wrapper and a bare `go test` would otherwise disagree
about what `binaryName()` returns, and the default rows would pass or fail by accident rather than
by assertion.

| Test | Case | Regression it catches | How it fails |
|---|---|---|---|
| `TestListRefusesAnyArgument` | `COMMAND_BIN_NAME=hm`, `--help` → `exitUsage`, hint `hm list` | **The circular hint returning.** `--help` is the row that matters: it is the argument whose hint used to name itself | Hint equality. **Fails today** — today's hint is `hm list --help` |
| | with `COMMAND_BIN_NAME=hd`, `--tonteria` → hint `hd list` | The hint being hardcoded again. This row can only pass if `binaryName()` is genuinely called; a literal `"hm list"` fails it | Hint equality. **Fails today** — today's hint is a literal that ignores the variable |
| `TestDescribeRefusesAnUnknownOption` | `COMMAND_BIN_NAME=hm`, `--help` → hint `hm describe [--with-secrets]` | The same closed loop, and the hint no longer naming the one option `describe` has | Hint equality. **Fails today** |
| | `COMMAND_BIN_NAME=hd`, `--tonteria` → hint `hd describe [--with-secrets]` | The same hardcoding | Hint equality. **Fails today** |
| `TestDoctorRefusesAnUnknownOption` | `COMMAND_BIN_NAME=hm`, `--help` → hint `hm doctor [--only=ports]` | The same closed loop; the hint no longer naming a usable option | Hint equality. **Fails today** |
| | `COMMAND_BIN_NAME=hd`, `--tonteria` → hint `hd doctor [--only=ports]` | The same hardcoding | Hint equality. **Fails today** |

Every row asserts the hint read out of the `--json` fault (`contract.Fault.Hint`), not scraped from
the text rendering: the text puts it behind an arrow and a blank line (`output.go:69-72`), and
matching that prose would be a second assertion about formatting dressed up as one about the hint.

The `COMMAND_BIN_NAME` rows are the ones that stay useful after the fix. `version_test.go:86` already
establishes setting that variable in a test, so the mechanism is not new.

**Unit 1 — `list_test.go`**

| Test | Case | Regression it catches | How it fails |
|---|---|---|---|
| `TestWhatListAsks` | happy path, `fake.environments` set | The routing regressing to `engine(`; a `projectOr` being added to `list` | `cmp.Diff` on the log: `want [{Environments}]` |
| `TestListRefusesAnyArgument` | *(already in unit 0)* gains one row: `algo` under `answering(t)` | `list.go:18-21` being loosened so a **positional** reaches the engine | `exitUsage`, then `len(fake.calls) != 0` |
| `TestWhatListPrints` | **empty** inventory | `list.go:55-60` being dropped, so an empty machine prints a bare header | literal-string diff on the two sentences |
| | one ordinary row | a column width or field order change (`%-26s`/`%-9s`/`%-7s`/`%-22s`) | literal-string diff |
| | a row that is orphan, has no metadata, and has empty `Branch` and `Root` | `orDash` (`:93-99`) being removed; either marker at `:73-79` being dropped or reordered; the two markers stopping concatenating | literal-string diff on `-`, `  ⚠ orphan` and `  (no metadata)` |
| | a row with a `Worktree` | `:69-71`'s `name (wt: branch)` composition | literal-string diff |
| `TestWhatListReportsAsJSON` | two environments | a `json:` tag rename (`has_metadata`, `containers.running`); `count` disappearing; the envelope changing | decode to `map[string]any`, `cmp.Diff` against a literal (numbers arrive as `float64`) |
| `TestListReportsAnUnreachableDaemon` | `outcomes[0].err`, text and `--json` | `list.go:25-27` mapping the error to `exitError`, or losing `docker_unavailable` | exit code equality **and** the `type` field of the decoded fault |

**Unit 2 — `describe_test.go`**

| Test | Case | Regression it catches | How it fails |
|---|---|---|---|
| `TestWhatDescribeAsks` | no arguments | **`withSecrets` defaulting to `true`** — `hm describe` volunteering the database password into every log, issue and agent context. The highest-value case in the cycle | `cmp.Diff` on `[{Describe, Dir: cwd, WithSecrets: false}]` |
| | `--with-secrets` | the flag being renamed or stopping setting anything (`describe.go:17-18`) | the same diff, `WithSecrets: true` |
| `TestDescribeRefusesAnUnknownOption` | *(already in unit 0)* gains one row: a bare `algo` under `answering(t)` | `describe.go:19-22` being loosened so a positional reaches the engine | `exitUsage`, then an empty log |
| `TestWhatDescribeReportsAsJSON` | a full `core.Description`, `Credentials` nil | any `json:` tag rename across six nested blocks; **`omitempty` being dropped from `Credentials`**, which would put `"credentials": null` into every document | decode to `map[string]any`, `cmp.Diff` against a literal; the `credentials` key must be **absent**, not null |
| | the same with `Credentials` set to fixture sentinels | the credentials block being nested differently or a field renamed | the same diff |
| `TestWhatDescribePrintsAsText` | `Credentials` nil | `describe.go:91`'s nil guard being removed — which is a nil dereference, so this is also the panic test | literal-string diff; a removed guard panics the subtest |
| | `Credentials` set | the four labels, their order, or the `warning("Credentials")` heading changing | literal-string diff |
| | empty `Version`, `Mode`, and empty `URLs` entries | `orUnknown` (`:102-108`) being removed; `:70-72`'s skip of empty URLs being dropped, which would print six blank lines | literal-string diff |
| `TestDescribeReportsAnUnresolvableProject` | `outcomes[0].err = errors.New("compose file is invalid")` | `describe.go:27-29` swallowing the engine's reason instead of interpolating it | `exitProject`, plus `strings.Contains(stderr, "compose file is invalid")` |
| | the same, with `COMMAND_BIN_NAME=hd` → hint `hd setup` | `describe.go:29` reverting to the hardcoded `"hm setup"`. This is the unit-2 half of Decision 6's knowing exception | hint equality against the decoded fault |

**Unit 3 — `logs_test.go`**

| Test | Case | Regression it catches | How it fails |
|---|---|---|---|
| `TestWhatLogsAsks` | no arguments | `Services` becoming `nil` instead of `[]string{}` (`orchestrate.go:92`); the `Dir` trap of Decision 3 | `cmp.Diff` — `cmp` distinguishes a nil slice from an empty one, so the `want` **must** say `Services: []string{}` |
| | `-f`, `--follow`, `-t`, `--timestamps`, `--no-color`, `--no-log-prefix` | any of `:131-138` writing a different `core.LogOptions` field | whole-`LogOptions` diff |
| | `--tail 100` and `-n 100`, two rows with an **identical** want | `-n` being given a field of its own instead of sharing `Tail` (`:111-112`) | the `-n` row diffs while the `--tail` row passes |
| | `--since 1h`, `--until 5m` | `:113-116` crossing the two | whole-`LogOptions` diff |
| | `--index 2` | `strconv.Atoi`'s result going somewhere else (`:117-124`) | `Index: 2` |
| | `phpfpm nginx` positional | a positional being eaten as an option; the order of services being reordered | `Services: []string{"phpfpm", "nginx"}` |
| | everything at once | flag order mattering when it should not | whole-`LogOptions` diff |
| `TestLogsRefusesAFlagWithNoValue` | table over `--tail`, `-n`, `--since`, `--until`, `--index`, each last | a flag dropping out of `takesValue` (`:96`), which is the exact defect the shell implementation had | `exitUsage`, `missing_value`, empty log |
| `TestLogsRefusesANonNumericIndex` | `--index nada` | `:118-122` accepting it and passing `0` | `exitUsage`, `invalid_argument`, empty log |
| `TestLogsRefusesAnUnknownOption` | `--tonteria` | `:139-143`'s dash test | `exitUsage`, empty log |
| `TestLogsReportsAFailure` | `outcomes[1].err` (the `Logs` call, not the `Resolve`) | `report()` no longer being reached from `:166` | `exitDocker`, `docker_failed`, and a log that ends at `Logs` |

### Cases deliberately not planned

- **`logs` returning a `core.Refusal`.** `composelib.Orchestrator.Logs` returns raw errors; it
  produces no refusal. A row for it would be re-proving `report()`'s refusal branch
  (`orchestrate.go:240-243`), which `set_host_test.go` already covers. It would pass forever and
  catch nothing about `logs`.
- **`unknown_service` through `checkServices`.** Off the seam by Decision 4. It cannot fire against a
  fake project, so a case for it would assert the *skip*, not the validation.
  `tests/integration/logs_launch_version_test.sh:148-153` covers it against the Go binary above the
  `docker info` gate, and survives the deletion cycle.
- **"`describe --with-secrets` does not print a credential the engine did not supply."** There is
  nowhere for the CLI to obtain one; no plausible break produces it. Duplicates the nil-credentials
  text case.
- **Anything of the shape "the exit code is not X"**, or an assertion that reads back an environment
  variable the test itself just set. Both are unfailable.
- **That the *engine* withholds credentials when `withSecrets` is false.** That decision is
  `app/describe.go:156-158`, one layer below this seam. **Finding: `app.Describer` has no unit test
  at all** — `rg 'withSecrets|Credentials' dockergento/app` finds only production code, and the only
  coverage is `tests/integration/introspection_test.sh:152` and `go_passthrough_test.sh:67`. Out of
  scope here; worth a follow-up, recorded under Open Questions.

### Proving the secret is withheld without the test holding one

The fixture's `core.Credentials` is invented by the test and read from nowhere, so nothing is leaked
by construction. To keep that visible to a reader, the four values are **sentinels that cannot be
mistaken for credentials** — `"db-name-fixture"`, `"db-user-fixture"`, `"db-password-fixture"`,
`"db-root-password-fixture"`. The tool's own documented defaults are explicitly **not** to be used:
they would read like a real pair, and a grep for them would hit a test file.

What the seam can then prove, and it is worth saying which is which:

1. **The toggle travels.** `WithSecrets: false` with no flag and `true` with it. This is the case
   that catches a leak-by-default.
2. **The CLI renders and serialises credentials only when the engine supplied them.** The nil case
   proves the absence in both renderings — no `Credentials` heading in the text, no `credentials`
   key in the document.

What it cannot prove is that the engine honours the flag. Said plainly in the test file's header, so
nobody reads case 1 as more than it is.

`core.Description` and `core.Environment` both use **anonymous nested structs** (`core/environment.go:128-131`,
`:207-246`), so a fixture cannot be a single composite literal. Each test file gets one small builder
that assigns field by field, the way `app/describe.go:102-113` itself does. This is priced into the
size table.

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED |
|---|---|---|---|
| Documentation-like paths | **N/A** — no file is classified or executed by this change | — | — |
| Git repository selection | **N/A** — no `git -C` or path selector is composed. The only `git` in reach is the `git describe` the real `Tooling.Version()` spawns during `describe`'s RED, with `Dir` = the pinned temp cwd | Decision 2 | — |
| Commit state | **N/A** — nothing touches the index or the worktree | — | — |
| Push state | **N/A** — no push | — | — |
| PR commands | **N/A** — no PR automation | — | — |

The real adversarial surface is not in that matrix, as it was not last cycle: it is the RED window's
reach into the **live Docker daemon** on a machine running the user's own containers. Two calls, not
one — a machine-wide `ContainerList` from `list`, and from `describe`'s `Xdebug` an
`ContainerExecCreate`/`ContainerExecAttach` **into an unrelated live container**, because a nameless
project matches every container that lacks the `hm.project` label (`toolinfo/tooling.go:84-87`).
Decision 2 answers both with a verified precedence chain, a per-unit failure signature, and a named
alarm. The exec is the row that would belong in a matrix of its own if this change introduced it;
it does not — it inherits it, for the length of one RED run, and pins it shut.

## Migration / Rollout

No migration. No command's arguments, output, `--json` document or exit codes change. Three routing
edits, each one line, each substituting a factory that returns the same object production has always
built (`engine.go:36-38`).

## Size and Commits

**Counting convention: changed lines = additions + deletions, as `git diff --shortstat` reports and
as the review guard defines the 400-line budget.** A one-line replacement therefore counts **2**, not
1. The predecessor's table counted such edits as 1; the difference is six lines across this change,
and it is stated here so the two documents are not silently incomparable.

### Unit 0 — `fix(cli): hints that name a real invocation instead of a --help that refuses`

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `internal/cli/list.go` | Modify | 1 | 1 | 2 |
| `internal/cli/describe.go` | Modify | 2 | 2 | 4 |
| `internal/cli/doctor.go` | Modify | 1 | 1 | 2 |
| `internal/cli/list_test.go` | Create | 30 | 0 | 30 |
| `internal/cli/describe_test.go` | Create | 32 | 0 | 32 |
| `internal/cli/doctor_test.go` | Create | 34 | 0 | 34 |
| **Unit 0** | | **100** | **4** | **104** |

Each test file is one two-row table, but carries a package clause, imports and a header comment,
which is why a single-test file is ~30 lines and not ~15. `doctor_test.go` is the longest of the
three because its header has to say why `doctor` appears in a cycle otherwise about three other
commands.

### Unit 1 — `test(cli): what list asks of the engine, and the one command with no RED net`

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `internal/cli/engine.go` | Modify | 1 | 0 | 1 |
| `internal/cli/list.go` | Modify | 1 | 1 | 2 |
| `internal/cli/fake_engine_test.go` | Modify | 14 | 0 | 14 |
| `internal/cli/list_test.go` | Modify | 200 | 0 | 200 |
| **Unit 1** | | **216** | **1** | **217** |

### Unit 2 — `test(cli): what describe asks of the engine`

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `internal/cli/engine.go` | Modify | 1 | 0 | 1 |
| `internal/cli/describe.go` | Modify | 1 | 1 | 2 |
| `internal/cli/fake_engine_test.go` | Modify | 19 | 0 | 19 |
| `internal/cli/describe_test.go` | Modify | 251 | 0 | 251 |
| **Unit 2** | | **272** | **1** | **273** |

### Unit 3 — `test(cli): what logs asks of the engine`

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `internal/cli/engine.go` | Modify | 1 | 0 | 1 |
| `internal/cli/orchestrate.go` | Modify | 1 | 1 | 2 |
| `internal/cli/fake_engine_test.go` | Modify | 13 | 0 | 13 |
| `internal/cli/logs_test.go` | Create | 231 | 0 | 231 |
| **Unit 3** | | **246** | **1** | **247** |

### Arithmetic

| | + | − | Changed |
|---|---|---|---|
| Unit 0 | 100 | 4 | 104 |
| Unit 1 | 216 | 1 | 217 |
| Unit 2 | 272 | 1 | 273 |
| Unit 3 | 246 | 1 | 247 |
| **Cumulative** | **834** | **7** | **841** |

Checked both ways: 834 + 7 = 841, and 104 + 217 + 273 + 247 = 841.

Against the pre-amendment figure of 799: the hint fix adds **42 net** (three production edits at 8
changed lines, and `doctor_test.go` at 34). The other 62 lines of unit 0 are not new — they are the
`list` and `describe` refusal tests **moved earlier**, out of units 1 and 2, which is why those two
units shrink by exactly 30 and 32.

The `openspec/` delta is excluded by the measuring command and is not in the table:
`git diff --shortstat <parent>..HEAD -- . ':(exclude)openspec'`, measured against **this change's own
parent at the time it is applied**, never a hash written into this document.

### Order, and what each unit can land on its own

The proposal's order is **confirmed and extended by one**: the hint fix, then `list`, `describe`,
`logs`.

- **Unit 0 must go first**, and its position is not a preference. It is the only ordering in which
  `list_test.go` and `describe_test.go` are written against a hint that is already correct. Placed
  anywhere later, units 1 and 2 ship assertions that pin the circular hint as a requirement, and unit
  0 then has to edit tests written the day before to say something else — which is precisely the
  pattern the fix exists to stop.
- The three test units are independent in behaviour — no unit needs another's interface method — but
  they all edit `fake_engine_test.go`, so they are **sequential commits, not parallel branches**.
- `list` first among them because it is the smallest and it carries the `DOCKER_HOST` finding; the
  RED discipline is established on the cheapest unit.
- `describe` second because it shares `list`'s daemon-reaching RED window, so the correction to
  Decision 2 is fresh when its test file is written.
- `logs` last because it is the only one with an off-seam neighbour (`checkServices`) and the `Dir`
  divergence, and it depends on nothing the other three establish.

Unit 0 is the only unit that changes production behaviour. Units 1–3 stay test-only, so the
proposal's rollback promise holds for them unchanged; unit 0 reverts on its own, restoring four
strings.

Each unit leaves the tree green on its own: `go build ./...`, `go test ./... -short`,
`tests/run.sh unit`, `gofmt -l ./cmd ./internal` empty, `go vet ./...` clean, and
`var _ commands = (*dockergento.Engine)(nil)` compiling — at **every** commit, because the interface
grows by exactly the method the routed call site needs and `*dockergento.Engine` already has all
three.

### The budget, and the one unit at risk

Each unit is inside 400 alone. The **cumulative 841 is not**, so the delivery decision belongs before
apply, not mid-apply: under the cached `ask-on-risk` strategy the recommendation is now **four
chained PRs in the order above**, each self-contained, rather than one `size:exception`.

Every recent cycle here overshot its own figure by 1.3–2x. Applied to these: unit 0 135–208, `list`
282–434, `describe` 355–546, `logs` 321–494.

**The hint fix improved the budget picture rather than worsening it.** `describe` was the unit at
risk at 305; moving its refusal test into unit 0 brings it to **273**, so its overshoot band drops
from 397–610 to 355–546 and it now clears 400 at 1.4x instead of 1.3x. `list` at 217 is the safest it
has been. The unit most likely to breach 400 is still `describe`, and if it does, cut in this order
and no other:

1. Merge the two `--json` subtests into one table row-pair.
2. Drop the bare-positional row unit 2 adds to `TestDescribeRefusesAnUnknownOption` — unit 0 already
   proves the branch with two rows.
3. Split `describe_test.go`'s two rendering tests into a fifth commit, leaving the log, the flag and
   the error in unit 2.

Never cut: comments, the `WithSecrets: false` case, the credentials-absent case, or anything in unit
0 — the hint fix is the one part of this cycle that changes what a user sees, and shrinking its
proof to fit a budget would leave a behaviour change less tested than the tests around it. If the
total still exceeds 400 after those three cuts, **stop and report it** rather than shrinking anything
else — the estimate was wrong, and that is worth knowing before the next cycle repeats it.

## Open Questions

- [ ] `logs` passes `here()` while `projectOr` has already resolved the project's root — the same
      `here()`-versus-`project.Root` inconsistency the three predecessors left open, now with a third
      instance. Pinned by Decision 3, not fixed.
- [ ] `app.Describer` has **no unit test**, so `withSecrets` gating the credentials
      (`app/describe.go:156-158`) is proved only by two integration suites. This cycle cannot close
      it — it is a layer below the seam — but somebody should.
- [ ] `checkServices` treats **any** `Configuration` error as "skip validation"
      (`orchestrate.go:201-203`), so a project whose compose files are unreadable silently loses the
      `unknown_service` refusal. Found, not fixed; the natural owner is the deletion cycle, which is
      what removes the parity half.
- [ ] `hm describe --help`, `hm list --help` and `hm doctor --help` are still unknown options; this
      cycle stops their hints pointing back at themselves but does **not** make `--help` a real
      per-command flag. That is a help-system question — where per-command help text lives, how it
      renders, and how both halves stay in step — and Decision 6 argues it is deliberately out of
      scope, not forgotten.
- [ ] After unit 0 the Go and shell hints for those three refusals **diverge** — the twins still say
      `--help` (`console/commands/describe.sh:87`). Verified harmless today, because every parity
      case for them asserts only the exit code (`go_passthrough_test.sh:80-83, 99-102, 189-192`). The
      divergence ends when the deletion cycle removes the twins; until then it is a known, bounded
      difference rather than an accident.
- [ ] `doctor` now has exactly one Go test — its unknown-option refusal — and no engine-seam test at
      all. It is not otherwise in this cycle's scope, and its `Diagnose` call site
      (`doctor.go:29`) still goes through the concrete `engine(`.
