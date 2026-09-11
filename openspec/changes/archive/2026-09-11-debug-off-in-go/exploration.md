# Exploration: debug-off-in-go

Change: `debug-off-in-go` (explored as `xdebug-commands-in-go`, narrowed after the sizing below)
Engram topic: `sdd/xdebug-commands-in-go/explore` (project `hiberus-dockergento`, observation #932)
Date: 2026-09-11

**Decision (user, 2026-09-11): `debug-off` only.** The sizing in §F is why — `debug-on` needs a new
`Capture` method on the `commands` interface, a generalisation of the container resolution behind
it, and a host-detection adapter that is new Go rather than composition of what exists. It gets its
own cycle. This one is the first step-one port of the series, and the first of anything should be
the cheap one.

Everything below was written while both were in scope. `debug-on`'s findings are kept rather than
trimmed, because that cycle inherits them.

**Step one of `MIGRATION.md`'s rule, not step two.** The last five cycles wrote Go tests for commands
already ported. These two are still pure shell and have no `case` in `internal/cli/run.go`:
`console/commands/debug-on.sh` (38 lines) and `debug-off.sh` (9).

`create-project` is out of scope by the user's decision — 119 lines, interactive, writes host files
and runs `composer create-project` in the container. It gets its own cycle. **The consequence has to
be said plainly: porting these two does not free `exec.sh`**, because `create-project.sh` still calls
it at `:43`, `:44`, `:49`, `:57` and `:60`.

## A. What the two do

**`debug-off`**: start phpfpm if not running → `sed -i -e 's/^\zend_extension/;zend_extension/g'` on
`docker-php-ext-xdebug.ini`, re-commenting the line that loads `xdebug.so` → `$DOCKER_COMPOSE restart
phpfpm` **directly** → print `"  xdebug disabled"`.

**`debug-on`**: start phpfpm if not running → on Linux, if `$XDEBUG_HOST` is unset, source
`set_xdebug_host_property.sh`, which detects the host address: WSL1 via `/proc/version` matching
`Microsoft` → `10.0.75.1`; WSL2 via `microsoft-standard` → `host.docker.internal`; otherwise
`ip addr show docker0` or `ifconfig docker0` to read the bridge's own IP → if resolved, `sed` both
`xdebug.remote_host` (xdebug 2) and `xdebug.client_host` (xdebug 3) → uncomment `zend_extension` →
**on macOS only**, `hm_run_binary copy-from-container generated`, because the code xdebug regenerates
lives in a named volume there → `restart.sh phpfpm` → capture `php -v`, grep `"Xdebug v"`, print a
banner, then branch on `v2.`/`v3.` to grep-and-cut the matching `idekey`/`remote_*`/`client_*` lines
out of `php -i`, or say it is not installed.

## B. How much the engine already does

`Exec` and `Restart` are already on the `commands` interface (16 methods), and the shape
`Exec(sed…)` → `Restart([service])` is exactly what the ported `varnish-on`/`varnish-off` already do
at `internal/cli/wrappers.go:134-189`. So the edit-and-restart half needs no new engine surface.

**`start_service_if_not_running` needs no Go analogue.** Compose's `up -d` — which is what
`engine.Start` runs — is a no-op for a service already running and unchanged, so calling `Start`
unconditionally subsumes the shell's explicit probe.

The catch: **`Start` is not on the interface.** It exists on `*dockergento.Engine`
(`dockergento.go:218`), but `internal/cli/orchestrate.go:37-41`'s `start()` still calls the concrete
`engine(` — the same unrouted class as `checkServices`'s `Configuration()`. So keeping the defensive
start costs one interface method; dropping it is a behaviour change (these commands would fail
instead of starting the service) and must be a decision, not a silent omission.

## C. The part that does not map, and the real work of the cycle

`debug-on` pipes container output through `grep` and `cut`. Three exec mechanisms exist in this
codebase and **none of them returns captured text for an arbitrary service through `commands`**:

| Mechanism | Where | What it answers |
|---|---|---|
| `composelib.Orchestrator.Exec` | `adapters/composelib/orchestrator.go:145` — backs `commands.Exec` | `(int, error)` only. Streams to the `*os.File` streams fixed once at `engine()` construction (`internal/cli/engine.go:50`) |
| `dockerd.Runner` | `adapters/dockerd/exec.go` — `Run`, `Capture`, `Feed` | writes to an `io.Writer`; used by `app.Database.Capture` for `mysqldump`, but only ever resolves the `db` container |
| `toolinfo.Reader.Xdebug` | `adapters/toolinfo/tooling.go:67-124` | a third, bespoke raw exec that already greps `^zend_extension` in the same file, for `describe`/`doctor` |

**The new work is a `Capture(dir, service string, command []string) (string, int, error)`-shaped
method**, growing `commands` to 17, backed by generalising `Database.container()`'s resolution —
`Containers()` plus `core.Container.Key()` and `ComposeService`, **not** the raw label disjunction at
`toolinfo/tooling.go:84-85`, which is already recorded as a defect. With that in place,
`grep … | cut -d= -f1-2` becomes ordinary Go string work and is unit-testable with no container in
sight.

## D. Behaviour to preserve, and behaviour to question

**Preserve**: `set -euo pipefail` semantics — the first failing step aborts with its own status,
which is exactly the class the `varnish-on` port silently dropped; the ini paths and setting names;
every message; the macOS-only copy branch and the Linux-only host detection as genuinely separate
platform paths; and `toolinfo.Reader.Xdebug`, which stays untouched.

**Question, found rather than assumed:**

1. **The twins restart differently.** `debug-on` uses `restart.sh phpfpm` — stop, start, proxy check,
   Linux permission fix. `debug-off` uses a bare `$DOCKER_COMPOSE restart phpfpm` — none of that.
   That is unexplained drift of the same class as the `varnish-on` defect. The design has to
   **choose**: replicate `debug-off` literally, which costs nothing because `commands.Compose` is
   already a raw passthrough, or unify both through `Restart`, which is a disclosed behaviour change
   that incidentally fixes the inconsistency. Not a silent pick either way.
2. `debug-off`'s `s/^\zend_extension/` carries a stray backslash before `z`. Inert, because GNU sed
   treats an unrecognised escape as the literal character. Cosmetic, but not to be copied without
   noticing.
3. If `$XDEBUG_HOST` never resolves — no `ip`, no `ifconfig`, no WSL match — `debug-on` skips the
   host patch **silently**. Worth a design note; possibly worth a message.

## E. What a test can assert at the seam

`debug-off`, if replicated literally: `[Resolve, Exec(sed re-comment), Compose(["restart","phpfpm"])]`,
plus `Start` if kept. If unified: `Restart(phpfpm)` in place of `Compose`.

`debug-on` on Linux with the host resolved: `[Resolve, Exec(sed remote_host), Exec(sed client_host),
Exec(sed uncomment), Restart(phpfpm), Capture(php -v), Capture(php -i)]`, plus `CopyFrom` on macOS,
plus `Start` if kept.

**Not provable at this seam**, and to be recorded at the call site the way `checkServices` already is:
which `XDEBUG_HOST` branch fired — that needs its own unit test over `/proc/version` and `ip addr`
content; the platform switch, which is asserted by controlling `runtime.GOOS`; and the `php -v` /
`php -i` parsing, which becomes an ordinary string-function test once `Capture` returns text.

## F. Size and units

**Two units, and the second may itself need splitting at design time.**

| Unit | What it needs | Estimate |
|---|---|---|
| `debug-off` | `Exec` plus `Compose` or `Restart`, possibly `Start` | ~150-220 |
| `debug-on` | a new `Capture` method, adapter generalisation, a **new host-detection adapter** (WSL and `docker0`, running on the host rather than in a container), version and ini parsing | ~350-550 |

`debug-on` is the heaviest single unit of the series so far, and the host detection is genuinely new
Go rather than composition of what exists. Whether it splits into capture-and-parsing versus
host-detection-and-macOS-copy is a design decision, not one to settle here.
