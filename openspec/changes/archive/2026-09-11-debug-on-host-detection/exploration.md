# Exploration: debug-on-host-detection

Change: `debug-on-host-detection` (unit 2 of 3 porting `hm debug-on` to Go; unit 1, `Capture`,
shipped as `capture-container-output`, archived at
`openspec/changes/archive/2026-09-11-capture-container-output/`)
Engram topic: `sdd/debug-on-host-detection/explore` (project `hiberus-dockergento`, observation #995)
Date: 2026-09-11

Inherits `openspec/changes/archive/2026-09-11-capture-container-output/exploration.md` §B-E in full.
This document verifies every claim in those sections against today's tree, records the wiring fork
§B left open as closed by user decision, and lays out §D's Linux abort-vs-skip question with
consequences rather than a chosen side.

**Written to disk by the orchestrator.** The `sdd-explore` agent has no Write tool, so it produced
this content into Engram and the orchestrator wrote it here. Two orchestrator edits were made to
the agent's text, both marked in place: a mid-sentence self-correction in the abort trace was
resolved into its conclusion, and one universal claim about WSL kernel strings was downgraded to
what the evidence actually supports. Nothing else was changed.

## Verification of inherited claims

| Claim (§) | Verdict | Evidence |
|---|---|---|
| `console/commands/debug-on.sh` is 38 lines | **Held** | Read in full, 38 lines exactly |
| `console/tasks/set_xdebug_host_property.sh` is 19 lines | **Held** | Read in full, 19 lines exactly |
| §B: `Host.Listening()` (`machine.go:54-68`) is untested real I/O; `fromLsof`/`fromSS` (`:72-120`) are pure and fixture-tested | **Held** | `Listening()` has no test; `fromLsof`/`fromSS` are pure `string → []core.Listener` functions with dedicated fixture tests |
| §B: `Platform()` gates on `darwin→mac`, at `dockergento.go:1361-1367` | **Partially held — line number drifted** | Body and mapping unchanged, but it now sits at `dockergento.go:1376-1382`; unit 1's additions shifted it down. Re-verified by the orchestrator against source |
| §B: open fork — whether the host method joins `ports.Machine`/`commands`, or becomes a second seam | **Closed by user decision** — see below | — |
| §C: platform predicates provable only on the matching OS, `t.Skip`-gated the way `dockergento/adapters/.../php_test.go:15-25` does | **Wrong on path, and wrong on applicability to this unit** — see below | The file is `internal/cli/php_test.go`, not under `dockergento/adapters/` |
| §D.1: Linux `pipefail` abort is reachable, macOS skip is correct | **Held, re-derived independently** | See "The Linux abort, re-verified" |
| §D.2: `debug-on.sh:19` carries no stray backslash | **Held** | Read at line 19, matches exactly |
| MIGRATION.md: `debug-off` counted Go, `debug-on` still shell | **Held** | `MIGRATION.md:260-261` |
| Unit 1 (`Capture`) shipped and stayed off the `commands` seam | **Held** | `Engine.Capture` at `dockergento.go:469-480`; the `commands` interface has no `Capture` entry and the compile assertion still holds |

### The one correction worth stating plainly

§C's citation for the `t.Skip` precedent points at the wrong package, and that is not a path typo —
it changes what the citation can support.

The file is `internal/cli/php_test.go:15-25`. It skips on non-darwin because the function under
test, `mirrorsVendor` (`internal/cli/php.go:99-101`), **reads `runtime.GOOS` itself**. The test is
OS-gated because the production code branches on the live runtime, not on an injected parameter.

That is not the shape this unit needs. `dockergento/adapters/machine` has **no** `t.Skip` or
`runtime.GOOS` anywhere. Its established precedent for platform difference is the
`memory_darwin.go`/`memory_linux.go` pair — two files selected by Go's filename build-tag
convention, each unconditionally correct for its own OS, with no runtime check and no test in
either. That is parallel to `Listening()`, not to `mirrorsVendor`.

For the three parsers this unit needs, the right template is `fromLsof`/`fromSS`: each takes a
**string** as its only input and returns a value, with no OS check inside it. A function shaped
`hostFromProcVersion(content string) (host string, ok bool)` is provable on any CI runner, Linux or
macOS, from a literal fixture — no `t.Skip`, because the platform never enters the function. The
`t.Skip` pattern applies only to code that itself branches on `runtime.GOOS`/`Platform()`, and if
this unit follows `Listening()`'s precedent and leaves the gating method untested, there is no test
to skip in the first place.

**Recommendation: three pure parsers with ordinary fixture tests, no skip anywhere, and the
orchestrating method left untested exactly like `Listening()`.**

## The open fork — closed

§B asked whether the new method joins `ports.Machine`/`commands` or becomes a second seam. The user
settled it: **the adapter is concrete, the seam stays deferred to unit 3** — the same shape unit 1
used for `Capture`. Concretely:

- The new method lives only on `machine.Host` in `dockergento/adapters/machine`.
- Nothing is added to `ports.Machine` (`dockergento/ports/ports.go`). Precedent for a concrete
  `Host` method with no interface entry already exists: `dockergento.go:860` calls
  `(machine.Host{}).ResolvesLocally(...)` directly, bypassing `ports.Machine` entirely. Verified
  against source by the orchestrator.
- Nothing is added to `internal/cli`'s `commands` interface, so the compile assertion and
  `fake_engine_test.go`'s fake need no changes, and `internal/cli/engine.go:14-16`'s invariant —
  "names what the handlers call and nothing else" — stays true, because this unit adds no handler.

This mirrors unit 1 exactly: `Capture` landed on `*dockergento.Engine` with no caller and no seam
entry. Unit 3 wires both capabilities into `internal/cli` alongside the handler that calls them.
Recorded here so unit 3's exploration does not re-open it.

## The Linux abort, re-verified

`console/tasks/set_xdebug_host_property.sh`, read in full (19 lines, `set -euo pipefail` at line 2):

```bash
XDEBUG_HOST=""
if [ "$MACHINE" == "linux" ]; then
    if grep -q Microsoft /proc/version; then # WSL
        XDEBUG_HOST=10.0.75.1
    elif grep -q microsoft-standard /proc/version; then #WSL2
        XDEBUG_HOST=host.docker.internal
    else
        if [ "$(command -v ip)" ]; then
            XDEBUG_HOST=$(ip addr show docker0 | grep "inet\b" | awk '{print $2}' | cut -d/ -f1)
        else
            XDEBUG_HOST=$(ifconfig docker0 | grep "inet addr" | cut -d ':' -f 2 | cut -d ' ' -f 1)
        fi
    fi
fi
export XDEBUG_HOST="$XDEBUG_HOST"
```

**Which failures are fatal and which are not, precisely.** `set -e` exempts a command used as an
`if` condition. So the two `grep -q ... /proc/version` calls and the `[ "$(command -v ip)" ]` test
are all safe when they fail — that is the normal control flow, and a non-WSL Linux box passes
through both `grep`s failing without incident.

The line that kills is the bare assignment:

```bash
XDEBUG_HOST=$(ip addr show docker0 | grep "inet\b" | awk '{print $2}' | cut -d/ -f1)
```

It is inside an `if` **body**, not an `if` condition, and being in a body carries no exemption.
Under `pipefail`, a pipeline's status is the rightmost non-zero exit among its stages. If `docker0`
does not exist or carries no address — rootless Docker, a renamed bridge, a custom network — then
`ip` writes its complaint to stderr and nothing to stdout, and `grep "inet\b"` finds no match and
exits 1. The downstream `awk` and `cut` succeed trivially on empty input, but `pipefail` still
reports the pipeline failed, because `grep` was the rightmost stage that actually failed. `set -e`
then aborts.

*(Orchestrator edit: the agent's text reasoned aloud toward this conclusion mid-sentence,
briefly misreading the assignment as an `if` condition before correcting itself. The conclusion it
reached is the one stated above, and the wobble is removed rather than preserved.)*

`debug-on.sh` brings this file in with `source`, not a subshell, so the abort propagates out of
`debug-on.sh` itself: the whole command dies before the restart, the copy-from-container, or the
summary banner. The failure is not confined to a corner of the script — it takes the command down
before any of it has produced value.

The `ifconfig` fallback branch has identical structure and identical exposure. The inherited
document only walked the `ip` branch.

macOS is unaffected: the outer `if [ "$MACHINE" == "linux" ]` never enters, `XDEBUG_HOST` stays
`""`, and `debug-on.sh:14`'s own `if [[ "${XDEBUG_HOST:-}" != "" ]]` guard correctly skips the
`sed` patch. That half of the inherited note is correct as stated.

### The decision, laid out rather than made

Two faithful ports are possible, and neither is transcription:

**A — Replicate the abort.** The Go method returns an explicit failure when neither `ip` nor
`ifconfig` can produce an address for `docker0` on Linux, and unit 3's handler exits non-zero with
a clear message, exactly where the shell would have died.

*Cost:* a Linux user with a nonstandard bridge gets `hm debug-on` refusing outright where before it
proceeded. That is a regression in the literal sense — though what it proceeded to do was already
broken, since no host means xdebug enabled but unreachable.

**B — Soften to a graceful skip.** The Go method returns "unresolved" without treating it as fatal;
unit 3 enables xdebug, leaves the host at the image default, and says so in the summary instead of
failing.

*Cost:* reintroduces exactly the silence the abort was accidentally preventing. Xdebug turns on,
the container restarts, and breakpoints never connect — with no exit code to flag it. That is a
worse debugging session than a refusal, because nothing announces the problem until the developer
is already confused about why a breakpoint is not hitting.

Neither is more correct in the abstract. They trade a hard stop for a silent no-op, and the answer
depends on how many of this project's Linux users are in the affected minority. This is a product
call for the proposal phase to put to the user.

**Design recommendation regardless of which is chosen:** the Go method must not collapse "resolved
to empty" and "not applicable on this platform" into one signal. Returning
`(host string, resolved bool)` — or an error distinct from "not applicable" — lets unit 3 implement
either A or B from the same unit-2 output. Building that distinction now costs nothing and
forecloses nothing.

## What the shell does, split by unit

`console/commands/debug-on.sh` (38 lines):

| Lines | What it does | Unit |
|---|---|---|
| 8 | `start_service_if_not_running.sh phpfpm` | neither — precondition, covered by existing machinery |
| 10-12 | On Linux with no `$XDEBUG_HOST` already set, source the host-detection script | **Unit 2** |
| 14-17 | If a host resolved, `sed`-patch `xdebug.ini`'s `remote_host`/`client_host` | Unit 3 (consumes unit 2's output) |
| 19 | Uncomment `zend_extension` in `docker-php-ext-xdebug.ini` | Unit 3 |
| 21-24 | On macOS, `copy-from-container generated` | Unit 3 |
| 26 | `restart.sh phpfpm` (→ `commands.Restart`, already on the seam) | Unit 3 |
| 27-37 | `php -v` for the Xdebug version, branch v2/v3/absent, `php -i \| grep` the keys, print the banner | Unit 3 |

`console/tasks/set_xdebug_host_property.sh` (19 lines) is entirely **Unit 2**:

| Lines | What it does |
|---|---|
| 4 | `XDEBUG_HOST=""` default |
| 5 | Gate: only on `$MACHINE == linux` |
| 6-7 | WSL1: `grep -q Microsoft /proc/version` → fixed host `10.0.75.1` |
| 8-9 | WSL2: `grep -q microsoft-standard /proc/version` → fixed host `host.docker.internal` |
| 10-15 | Neither: prefer `ip addr show docker0`, fall back to `ifconfig docker0` if `ip` is absent |
| 19 | `export XDEBUG_HOST` |

## The template, confirmed, and what the new code mirrors

`machine.go`'s shape holds exactly as described: `Listening()` (`:54-68`) is the one method doing
real process I/O (`exec.LookPath`, `exec.Command`) and has no test; `fromLsof` (`:72-94`) and
`fromSS` (`:96-120`) are pure functions with dedicated fixture tests in `machine_test.go` built
from real tool output shapes.

What unit 2 mirrors:

- **File layout**: a new file under `dockergento/adapters/machine/`, e.g. `xdebug_host.go`.
  `machine.go` is already 237 lines covering seven unrelated concerns, and `resolver.go` already
  establishes the same-package sibling file for one cohesive concern — so a new file is the
  established pattern, not a deviation from it.
- **Naming**: one exported method on `Host` for the orchestrating call, mirroring `Listening()`'s
  shape; three unexported pure functions in `fromLsof`/`fromSS`'s register —
  `hostFromProcVersion`, `hostFromIPAddr`, `hostFromIfconfig`.
- **How fixtures are fed**: literal Go string constants inside the test file, one per real tool's
  output shape — exactly `TestReadingLsof`/`TestReadingSS`'s pattern. Not golden files on disk;
  nothing in this package reads fixtures from disk.

## Fixture content, from evidence rather than invention

**`/proc/version` — WSL1** (matches `grep -q Microsoft`):
```
Linux version 4.4.0-43-Microsoft (Microsoft@Microsoft.com) (gcc version 5.4.0 (GCC) ) #1-Microsoft Wed Dec 31 14:42:53 PST 2014
```

**`/proc/version` — WSL2** (matches `grep -q microsoft-standard`):
```
Linux version 5.15.90.1-microsoft-standard-WSL2 (oe-user@oe-host) (x86_64-msft-linux-gcc (GCC) 9.3.0, GNU ld (GNU Binutils) 2.34.0.20200220) #1 SMP Fri Jan 27 02:56:13 UTC 2023
```

**`/proc/version` — ordinary Linux** (matches neither, falls to the `ip`/`ifconfig` branch):
```
Linux version 5.15.0-91-generic (buildd@lcy02-amd64-078) (gcc (Ubuntu 11.4.0-1ubuntu1~22.04) 11.4.0, GNU ld (GNU Binutils for Ubuntu) 2.38) #101-Ubuntu SMP Tue Nov 14 13:30:08 UTC 2023
```

> **Orchestrator edit — a universal claim downgraded.** The agent wrote that the WSL1 check running
> first "only matters if a string could match both, and **by construction it cannot**." That is an
> assertion about every WSL2 kernel version string that exists or will exist, and it is not
> knowable from this repository. What the evidence supports is narrower: *the WSL2 sample above does
> not contain a capital-M `Microsoft` token, so it does not match the WSL1 pattern.* The ordering is
> therefore load-bearing in a way worth naming — a WSL2 string that did carry `Microsoft` anywhere
> (a build-host field, for instance) would be misdetected as WSL1 and handed `10.0.75.1` instead of
> `host.docker.internal`. The Go port can remove the fragility for free by testing the WSL2 pattern
> first, or by matching more precisely than a substring. Worth a proposal-phase decision, not a
> silent transcription of the shell's order.

**`ip addr show docker0`** — the interface line carries the address `awk`/`cut` extract:
```
3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN group default
    link/ether 02:42:0a:5b:2e:1a brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.1/16 brd 172.17.255.255 scope global docker0
       valid_lft forever preferred_lft forever
    inet6 fe80::42:aff:fe5b:2e1a/64 scope link
       valid_lft forever preferred_lft forever
```

**The one place a naive port silently breaks behaviour.** The shell's `grep "inet\b"` deliberately
excludes the `inet6` line: `\b` is a word boundary, and there is none between `inet` and `6`
because both are word characters. A Go equivalent needs the same distinction — match `inet ` with a
trailing space, or `regexp.MustCompile(`^\s*inet\s`)` per line — because a bare
`strings.Contains(line, "inet")` also matches `inet6` and extracts the wrong address. This deserves
an explicit acceptance criterion.

**`ip addr show docker0` — interface absent** (the case that reaches the abort/skip decision):
```
Device "docker0" does not exist.
```
Printed to stderr; stdout is empty, so `grep` matches nothing.

**`ifconfig docker0`** (net-tools legacy format, used when `ip` is absent):
```
docker0   Link encap:Ethernet  HWaddr 02:42:0a:5b:2e:1a
          inet addr:172.17.0.1  Bcast:172.17.255.255  Mask:255.255.0.0
          inet6 addr: fe80::42:aff:fe5b:2e1a/64 Scope:Link
          UP BROADCAST MULTICAST  MTU:1500  Metric:1
```
The shell parse (`grep "inet addr" | cut -d ':' -f 2 | cut -d ' ' -f 1`) splits on `:` first —
field 2 is `172.17.0.1  Bcast`, since `Bcast:` introduces the next colon — then on space, giving
`172.17.0.1`. A Go parser should replicate that two-step split rather than a single regex, or it
will silently diverge on a line whose `Bcast`/`Mask` ordering differs across net-tools versions.

## What is provable here and what is not

- **Provable in this unit, adapter-package tests, no `t.Skip` needed**: all three parsers — pure
  functions over fixture strings, exactly like `fromLsof`/`fromSS`.
- **Not provable here, and not tested at all, matching `Listening()`'s precedent**: the
  orchestrating method that gates on `Platform()`, calls `exec.LookPath("ip")`, and shells out.
  No fixture stands in for a real `docker0` without either a real Linux host with Docker's bridge
  present, or an `exec.Command` mock — which nothing in this package does and which the
  `Listening()` precedent does not attempt either.
- **Not provable at all, here or later**: that the `sed` replacement lands in `xdebug.ini` inside a
  running container, and that PHP-FPM picks it up after restart. Unit 3's concern, and outside
  anything short of a live daemon either way.

## Size

**Independent estimate: ~130-175 lines** (production ~65-89, tests ~65-90).

- Production: one orchestrating method mirroring `Listening()` (17 lines with its doc comment) but
  with more branches — platform gate, two WSL fixed-host cases, `ip`-preferred-over-`ifconfig`
  fallback — call it 25-35; three pure parsers at 12-18 each, 40-54 total.
- Tests: this package's `fromLsof`/`fromSS` ratio is roughly 1:1 with production. Three parsers at
  2-3 fixture cases each land around 20-30 lines per parser.

This lands **close to the inherited 120-180 midpoint rather than clearly under it**, unlike the two
most recent slices (unit 1's A: 62 against 50-65; B: 153 against 160-185, both under). The reason
is structural, not optimism. Closing the wiring fork removes work — no `ports.Machine` growth, no
`commands` growth, no fake field — but this unit has **three** parsers where the template pairs
have **two**, and each carries the same doc-comment weight this codebase spends everywhere. The two
effects roughly cancel. For unit 1's slices the resolved uncertainty *was* the majority of the
extra scope; here it is a smaller share of a total with its own irreducible per-parser cost.

## What a task list needs that the inherited document does not say

- File names: `dockergento/adapters/machine/xdebug_host.go` and `xdebug_host_test.go`. **Check both
  paths are free before writing** — unit 1 clobbered an existing `resolve.go` this way, and was
  saved only by luck of coupling.
- Signatures, settled here so nothing later renegotiates them:
  - `func hostFromProcVersion(content string) (host string, ok bool)`
  - `func hostFromIPAddr(output string) (host string, ok bool)`
  - `func hostFromIfconfig(output string) (host string, ok bool)`
  - `func (Host) XdebugHost() (host string, resolved bool)` — name open to the proposal phase;
    the shape (platform-gated, distinguishing "not applicable" from "applicable but unresolved") is
    what this exploration commits to.
- An explicit acceptance criterion that `hostFromIPAddr` preserves the `inet\b` boundary and does
  not match `inet6`.
- An explicit non-goal: no `ports.Machine`/`commands` change in this unit, repeated in `tasks.md`
  so a reviewer does not go looking for one.
- The abort-vs-skip decision, carried to the proposal phase unresolved.
- The WSL1/WSL2 ordering question raised in the orchestrator note above.
- Re-verify `Platform()`'s line reference wherever it is cited — it moved from `:1361` to `:1376`
  between these two cycles and will move again.

## Ready for proposal

Yes, with two decisions carried forward unresolved: the Linux abort-vs-skip behaviour, and whether
the Go port preserves the shell's WSL1-before-WSL2 check order or removes that fragility.
Everything else — file layout, signatures, fixture content, what is and is not provable, and the
closed wiring fork — is settled enough for `sdd-propose` to proceed.

## Risks

- The abort-vs-skip decision, left undecided into `sdd-tasks`, makes Strict TDD's RED step ambiguous
  for the orchestrating method's failure path. Pin it in the proposal.
- `Platform()`'s line number will drift again before unit 3.
- No adapter-level mock exists for `exec.Command`, so the orchestrating method's Linux branches are
  permanently unverifiable by automated test in this repository's current shape. This matches
  `Listening()`'s existing precedent and is not a new gap this unit introduces.
