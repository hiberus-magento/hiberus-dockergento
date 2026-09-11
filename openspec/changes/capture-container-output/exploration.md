# Exploration: capture-container-output

Change: `capture-container-output` (explored as `debug-on-in-go`, narrowed to unit 1 after §E)
Engram topic: `sdd/debug-on-in-go/explore` (project `hiberus-dockergento`, observation #959)

**Decision (user, 2026-09-11): unit 1 only — `Capture` on the seam.** §E is why: the three units
come to 620-900 lines, and unit 3 alone is 350-500 while carrying two unresolved behaviour
decisions. `Capture` is the piece nothing else can be built on, it lands with no caller, and
`create-project` will need it too.

Everything below was written with the whole of `debug-on` in scope and is kept rather than
trimmed, because units 2 and 3 inherit it — including the worktree-refusal finding and the
correction to what the `debug-off` cycle believed about an unresolved host.
Sibling: `openspec/changes/archive/2026-09-11-debug-off-in-go/`, read in full — it is the template
for a step-one port here, and its own exploration kept `debug-on`'s findings deliberately.
Date: 2026-09-11

Every line number the orchestrator measured was verified and none needed correcting.
`console/commands/debug-on.sh` is 38 lines; `console/tasks/set_xdebug_host_property.sh` is 19.
`MIGRATION.md:21` already reads `33 de 65` — `debug-off`'s port is counted.

## The restart is `Restart`, and that is not free

`debug-on.sh:26` calls `restart.sh phpfpm`, which is `stop.sh` then `start.sh` — so
`commands.Restart` is the literal port, exactly as bare `Compose` was literal for `debug-off`.
Each cycle replicates the primitive its own command actually calls. `varnish` already does the
same thing at `internal/cli/wrappers.go:163`.

**What it costs, and this must be a disclosed acceptance criterion rather than an inherited
side effect.** `Operator.Restart` (`app/orchestrate.go:237-247`) calls
`refuseFromAnUnregisteredWorktree` unconditionally — code 6 — and on Linux `afterStarting()`
hands off to `hm post-start`. Neither happens today, and the reason is a routing quirk:
`bin/run`'s gate (`hm_alters_environment`, `console/helpers/worktree.sh:85-94`) tests only the
**top-level invoked name**, which is `debug-on`, and that is absent from the list. `restart.sh`
called from inside a script never re-enters the dispatcher, so it is never gated — even though a
user typing `hm restart` directly would be.

So `debug-off`'s Decision 1 and this one run in opposite directions from the same principle: there
the literal choice *avoided* introducing a refusal, here the literal choice *introduces* one.
`hm debug-on` from an unregistered worktree would start answering code 6 where it never has.

## A. `Capture`, the piece that does not exist

| Mechanism | Shape | Why it does not fit |
|---|---|---|
| `composelib.Orchestrator.Exec` — backs `commands.Exec` | `(int, error)` | streams to `*os.File` fields fixed at `engine()` construction (`internal/cli/engine.go:50`); not redirectable per call |
| `dockerd.Runner.Capture` (`adapters/dockerd/exec.go:72-106`) | `Capture(id, command, out, errors) (int, error)` | right shape, but takes a **container id**; only caller is `app.Database.Capture`, which hardcodes `db` |
| `toolinfo.Reader.Xdebug` (`adapters/toolinfo/tooling.go:67-124`) | bespoke raw exec | resolves `phpfpm` by the raw label disjunction at `:84-85`, a defect recorded across three cycles. Not to be reused, and not to be fixed here |

**What generalises**: `Database.container()` (`app/database.go:121-145`) already resolves correctly
— `Containers()`, then `candidate.Key() != project.Name || candidate.ComposeService != service`,
refusing with code 5 when nothing is running. The work is the same resolution parameterised on an
arbitrary service, best as a shared helper both call, so the two cannot drift.

**Proposed signature**: `Capture(dir, service string, command []string) (string, int, error)` —
buffer at the facade and hand back a string, matching `toolinfo`'s ergonomics rather than making
the CLI plumb `io.Writer`s.

**What the interface growing touches**: `internal/cli/engine.go` (17 methods, and the compile
assertion at `:36` must still hold); `internal/cli/fake_engine_test.go`, where `outcome`
(`:51-54`) needs a field for returned **data** — nothing today answers with any; and whatever
holds the generalised resolution, with `dockergento/app/database_test.go`'s five tests kept passing
unchanged as the proof the refactor is behaviour-preserving.

Worth stating so nobody hunts for it: **no automated test enforces the method count.** The
"sixteen methods" in prior proposals is hand-maintained prose.

## B. Host detection is new Go, and the template already exists

It runs on the machine, not in a container, so there is nothing to compose.
`dockergento/adapters/machine/machine.go` is the shape to copy exactly: `Host.Listening()`
(`:54-68`) does the real I/O — `exec.LookPath`, `exec.Command` — and is **not** unit-tested, while
all its parsing lives in `fromLsof`/`fromSS` (`:72-120`), which are pure and fixture-tested.

So: one method doing the I/O, gated on `Platform()` (`dockergento.go:1361-1367`) rather than raw
`runtime.GOOS`, since `Platform()` already maps `darwin→mac` the way `MACHINE` does; plus
`hostFromProcVersion`, `hostFromIPAddr` and `hostFromIfconfig` as pure functions over content.

**An open fork, deliberately not settled here**: whether that method joins `ports.Machine` and
`commands` — consistent with `newEngine` being the only substitution seam this codebase has — or
becomes a second swappable package var in `internal/cli`, which avoids growing the interface again
but invents a second test-substitution pattern.

## C. What is not provable at the seam

The WSL branches and the `docker0` parsing **are** provable, just not at the CLI seam: they are
pure functions and get adapter-package tests, the way `fromLsof` is proved and `Listening()` is
not. The platform predicates are provable only on the matching OS, `t.Skip`-gated the way
`php_test.go:15-25` already does for `mirrorsVendor`. That the sed patterns match the ini's real
content, and that PHP-FPM reflects the change, is not provable here at all — the same thing
`debug-off` recorded for its own edit.

## D. Behaviour to question, and a correction to what we inherited

1. **The inherited "silent skip" note is wrong on Linux.** The `debug-off` exploration said an
   unresolved `$XDEBUG_HOST` makes `debug-on` skip the host patch silently. Read against
   `set -euo pipefail`, that is true **only** when `$MACHINE != "linux"` — macOS never enters the
   block, so the two `sed` calls are skipped correctly. On real Linux, if `ip addr show docker0`
   fails or its `grep` matches nothing, `pipefail` propagates the failure into the assignment and
   `set -e` **aborts the script**. That is reachable — rootless Docker, a renamed bridge, a custom
   network. So the port has a real decision the inherited note hid: replicate the Linux abort
   faithfully, or soften it into a graceful skip. Either is defensible; neither is transcription.
2. No stray-backslash finding recurs: `debug-on.sh:19`'s `s/^\;zend_extension/zend_extension/g` is
   the exact inverse of `debug-off`'s pattern and carries no dead escape.
3. **The `--json` envelope has no shell precedent**, as `debug-off` found for its own
   `{"xdebug": false}`. But `debug-on` prints several ini values rather than one boolean, so the
   shape owed is richer — version, and the filtered settings. Deferred to the spec phase on
   purpose, the way `debug-off` deferred its own.

## E. Size and units

| Unit | What it needs | Estimate | Lands alone? |
|---|---|---|---|
| 1 · `Capture` on the seam | the method, facade wiring, generalised resolution, the fake's growth, tests | ~150-220 | Yes — no caller needed |
| 2 · host-detection adapter | new files under `adapters/machine`, pure parsers with fixtures, the wiring fork above | ~120-180 | Yes — independent of unit 1 |
| 3 · the handler | composes both, plus the macOS copy branch, `php -v`/`php -i` parsing, the v2/v3/absent banner, the JSON envelope, and the widest failure matrix of any wrapper here | ~350-500 | No — last |

**Realistic total ~620-900**, not the ~350-550 the inherited estimate carried. `debug-on` has four
largely independent gates — host resolved or not, mac or linux, xdebug v2 / v3 / absent, json or
text — where `varnish` has one. `debug-off` itself was estimated at 150-210 and landed at 302,
entirely from scenarios the spec added late; this one branches wider. Plan for several commits
along the split above, not the single commit `debug-off` shipped as.
