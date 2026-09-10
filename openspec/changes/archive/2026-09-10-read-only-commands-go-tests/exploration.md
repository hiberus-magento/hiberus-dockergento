# Exploration: read-only-commands-go-tests

Change: `read-only-commands-go-tests`
Engram topic: `sdd/read-only-commands-go-tests/explore` (project `hiberus-dockergento`, observation #670)
Date: 2026-09-09

Five commands are routed in Go, have no runtime caller, and are blocked from losing their shell twin
only by the missing Go test on the `internal/cli` seam: `list`, `describe`, `logs`, `bash`,
`masquerade`. This is step two of `MIGRATION.md`'s three-step rule — port, prove, delete.

## The scope reading, which needs confirming

The name follows the step-two precedents (`wrappers-go-tests`, `remaining-wrappers-go-tests`,
`down-and-set-host-go-tests`), all of which wrote tests and deleted nothing; the deletion came later,
for twelve commands at once, in `retire-ported-shell-commands`. This exploration reads the cycle the
same way: **tests only, deletion out of scope**. The 253 lines of shell those five twins hold are the
payoff this unlocks, not work this cycle does.

## The table, verified

| Command | Handler | Engine method | On `commands`? |
|---|---|---|---|
| `list` | `internal/cli/list.go:23` | `Environments()` | No |
| `describe` | `internal/cli/describe.go:25` | `Describe(here(), withSecrets)` | No |
| `logs` | `internal/cli/orchestrate.go:166-167` | `Logs(here(), services, options)` | No |
| `bash` | `tools.go:10-18` → `execute()` → `orchestrate.go:188-189` | `Exec(...)` | Yes, but through the concrete `engine(...)` |
| `masquerade` | `tools.go:54` | `Anonymise(here())` | No |

`internal/cli/engine.go:17-30` holds twelve methods today. Only `*dockergento.Engine` and
`*fakeEngine` implement `commands`, so nothing else has to move for the interface to grow.

## A. The seam grows by four, not three

| Method | Signature | Fake shape | `call` gains |
|---|---|---|---|
| `Environments` | `Environments() ([]core.Environment, error)` | answer field, like `Resolve` | nothing |
| `Describe` | `Describe(dir string, withSecrets bool) (core.Description, error)` | answer field | `WithSecrets bool` |
| `Logs` | `Logs(dir string, services []string, options core.LogOptions) error` | outcome only, like `Restart` | `LogOptions` |
| `Anonymise` | `Anonymise(dir string) error` | outcome only, like `Dump` | nothing — reuses `Dir` |

`bash` and `exec` need no new method: `Exec` is already there. Five call sites change one line each
from `engine(...)` to `newEngine(...)`. All four methods already exist on `*dockergento.Engine`, so
the compile-time assertion keeps holding without an edit.

## B. `bash` drags `exec` onto the seam, and that is fine

`shell()` is a thin wrapper over `execute()`, which is also the `exec` handler. Routing it seams both.
That is a bonus rather than a hazard, for reasons that were checked rather than assumed:

- `Exec` is already an interface method, already exercised through `inside()` by `inside_test.go`.
- The swap is one line with no production behaviour change — `newEngine` resolves to `engine`.
- `exec` has **no Go test at all** today. This closes a real pre-existing gap, and the proposal
  should declare it rather than let it look like a side effect.
- `exec.sh`'s six by-path callers are untouched: they call the shell file directly, not Go.

For the future deletion cycle: `bash.sh` (8 lines) only delegates to `exec.sh` and can retire alone.
`exec.sh` cannot — `debug-on.sh`, `debug-off.sh` and `create-project.sh` are still genuinely shell and
still route real invocations through it.

## C. The RED window, and the one command that has no net

`answering(t)` (`fake_engine_test.go:212-229`) pins six things because a RED run reaches the real
engine while a call site is unrouted: `HM_STATE_DIR`, `HM_HOSTS_FILE`, `HM_NON_INTERACTIVE=1`,
`DOCKER_HOST=unix:///nonexistent`, `HM_LEGACY_ROOT`, and `t.Chdir(t.TempDir())`.

**This paragraph was wrong, and the design caught it.** It claimed four of the five were safe by
construction because each calls `e.Resolve(dir)` first, so an empty temp cwd would fail them before
any adapter is touched. `Resolve` never fails on a non-project directory — `dockergento.go:127-129`
documents it: "a directory that is not a project is not an error… the answer is a project with no
name". `describe` therefore proceeds into `app/describe.go:58-70`, which fans out seven concurrent
questions and inspects no error until they all return; two of them are `d.Engine.Containers()` and
`d.Tooling.Xdebug(project.Name)`. With an empty name, `toolinfo/tooling.go:84-85` matches any
container lacking an `hm.project` label, so `Xdebug` can exec into an unrelated live `phpfpm`.

The `DOCKER_HOST=unix:///nonexistent` pin is what actually protects every one of them. Left here
rather than deleted, because a wrong reason that reached a right conclusion is worth being able to
find again.

**`list` was called the exception, and it is not — it is only the most obvious case.**
`Environments()` takes no directory at all. It
goes straight to `app.Inventory{...}.Environments()`, whose first act is `i.Engine.Containers()` — a
Docker SDK `ContainerList` across the whole machine, scoped to no project. Its only protection is the
`DOCKER_HOST` pin. On a machine with live containers, an unpinned RED run for `list` reaches the real
daemon.

**A subtlety for whoever writes the tests.** `logs`, `execute` and `masquerade` call their engine
method with `here()` again rather than with the `project.Root` the fake already returned from
`projectOr`. So the recorded `Dir` is the test's cwd, not the fake's root. `inside()` does the
opposite, and `inside_test.go` asserts `Dir: root` accordingly — copying that pattern verbatim here
would be wrong.

**`checkServices` is an open decision, not a settled fact.** `logs`'s unknown-service validation
(`orchestrate.go:198-221`) calls `engine(nil, nil, jsonOutput).Configuration(project)`, which is not on
the interface. It reads compose YAML locally, so it is no RED hazard — but `checkServices` treats any
error as "skip validation" (`:202`), so against a fake project with a fictitious root the
`unknown_service` branch can never fire. Either `Configuration` becomes a fifth method, or that branch
stays untested in Go. Design has to choose.

## D. Size

| Component | Estimate |
|---|---|
| `engine.go`, four signatures | ~6 |
| Five routing fixes | ~6 |
| `fake_engine_test.go` | ~85-100 |
| `list_test.go` | ~140-160 |
| `describe_test.go` | ~200-250 |
| `logs_test.go` | ~180-210 |
| `shell_test.go` / `exec_test.go` | ~110-130 |
| `masquerade_test.go` | ~140-170 |
| **Total** | **~865-935** |

Every recent cycle here overshot its own estimate by 1.3-2x (`down-and-set-host-go-tests` 418→609,
`wrapper-exit-codes` 50-90→184, `down-json-and-flags` 120-180→302), so treat this as a floor.

**Recommended split: five work units, one per command**, each growing the interface by exactly what it
needs, routing exactly its own call site, and leaving the tree green on its own. Order: `list` (~230-260),
`describe` (~280-330), `logs` (~270-310), `bash`+`exec` together (~180-210, joined by `execute()`),
`masquerade` (~220-250).

## E. What the shell twins do

All five read. Four carry `set -euo pipefail`; `logs.sh` has `set -uo pipefail` without `-e`. Checked
for a `varnish-on`-shaped hidden difference and found none: the only failure that could matter is a
command substitution assigned to a variable, which does not trigger `-e` anyway, and the script ends in
`exec`, so the daemon's exit code becomes the script's.

Two real findings:

1. **`masquerade`: Go is stricter than the shell, and nobody wrote it down.** The shell twin never
   reads `$@`; any argument is ignored. The Go handler (`tools.go:25-30`) rejects any dash-prefixed
   argument as `invalid_argument`. `data/command_descriptions.json` documents no `opts` for it. Found,
   not fixed — the test should pin what ships, not quietly restore the shell's laxity.
2. **Two files are called `masquerade.sh`.** `console/commands/masquerade.sh` is the command twin;
   `console/components/masquerade.sh` is a shared helper sourced by `mysql.sh:34` and
   `transfer-db.sh:6`. A future deletion must not touch the second.

`bash.sh`, `list.sh` and `describe.sh` match their Go handlers exactly, including a shared quirk where
`--help` is treated as an unknown option in both halves rather than printing help.

## F. Blockers

None for a test-only cycle. All five already read `go` in `MIGRATION.md` and already have both a file
and a route, which is what `command_safety_test.sh`'s "a file **or** a route" already accepts. The
items that would block a deletion cycle — `skills_test.sh:126`'s comment about `bash.sh`,
`command_safety_test.sh`'s safety rows, `MIGRATION.md` prose — belong to that cycle's own exploration.

## The name was wrong, and the scope now fits it

`bash` and `masquerade` are declared `"safety": "dangerous"` in `data/command_descriptions.json` and
pinned as such by `command_safety_test.sh:108-118`. Only `list`, `describe` and `logs` are `"safe"`.

**Decision (user, 2026-09-09): the three genuinely read-only commands only** — `list`, `describe`,
`logs`. Tests only, no deletion. That makes the cycle's name accurate rather than aspirational, and it
drops the two findings that belonged to the other two: `Anonymise` is not needed, so the seam grows by
**three** methods rather than four, and `masquerade`'s decline-path test with its `os.Stdin`
redirection — the single most awkward piece in the estimate — goes with it.

`bash`, `exec` and `masquerade` get their own cycle. `bash` and `exec` are joined at `execute()` and
have to move together; `exec` has no Go test at all today, so that cycle closes a gap rather than
extending one.

Revised size: three units, ~780 lines — `list` (~230-260), `describe` (~280-330), `logs` (~270-310).
Each under the 400-line budget on its own; the cumulative needs the delivery decision before apply.
