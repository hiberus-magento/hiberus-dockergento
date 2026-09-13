# Exploration: debug-on-handler

Change: `debug-on-handler` (unit 3 of 3, the last and largest, porting `hm debug-on` to Go)
Engram topic: `sdd/debug-on-handler/explore` (project `hiberus-dockergento`, observation #1019)
Date: 2026-09-11

Units 1 (`capture-container-output`) and 2 (`debug-on-host-detection`) both shipped today and are
archived. This unit composes them into the handler and routes the command.

**Written to disk by the orchestrator.** The `sdd-explore` agent has no Write tool. Every claim
below marked *verified* was re-checked against source by the orchestrator, independently of the
agent's report.

## What this unit ships

`console/commands/debug-on.sh` (38 lines), split by where each part lands:

| Lines | What the shell does | Where it goes |
|---|---|---|
| 8 | `start_service_if_not_running.sh phpfpm` | precondition; see §D |
| 10-12 | Source host detection on Linux | `Engine.XdebugHost()` → `machine.Host.XdebugHost()` (unit 2) |
| 14-17 | `sed`-patch `xdebug.ini`'s `remote_host`/`client_host` | `Exec`, as `debug-off` does |
| 19 | Uncomment `zend_extension` | `Exec` |
| 21-24 | On macOS, `copy-from-container generated` | **already ported** — see §C |
| 26 | `restart.sh phpfpm` | `commands.Restart`; see §E |
| 27-37 | `php -v`, branch v2/v3/absent, `php -i \| grep` the keys, print the banner | `Capture` (unit 1) plus new pure parsing |

## A. The seam grows by two, not one

**Verified.** `internal/cli/engine.go`'s `commands` interface has exactly **16 methods**, and
neither `Capture` nor `XdebugHost` is among them. Both must join here, because this is the unit
that finally gives them a handler — which is what `engine.go:14-16`'s written invariant requires:
the interface "names what the handlers call and nothing else".

**`XdebugHost` needs a facade method first, and that is not optional.** `internal/cli` **never
imports `dockergento/adapters/*`** — verified by grep across the package, zero hits. So the handler
cannot call `machine.Host{}.XdebugHost()` directly the way `dockergento.go:860` calls
`ResolvesLocally`. A new `(*dockergento.Engine).XdebugHost() (string, error)` must be added on the
facade, wrapping the adapter call, and *then* put on the seam.

So: **16 → 18**, not the 16 → 17 some earlier documents assumed.

**The fake needs a new kind of field.** `internal/cli/fake_engine_test.go`'s `outcome` struct is
exactly `{status int; err error}` — verified. Nothing in it carries returned **data**, because no
method on the seam has ever answered with any. `Capture` returns a string, so `outcome` grows a
field for it. `fakeEngine` also needs a default answer for `XdebugHost`, which takes no arguments.

## B. A second pipefail abort, and this one kills a branch somebody wrote

**New finding, not inherited. Verified against source.**

`console/commands/debug-on.sh:2` is `set -euo pipefail`. Line 27 is:

```bash
xdebug_version=$("$COMMANDS_DIR"/exec.sh php -v | grep "Xdebug v")
```

A **bare assignment**, not an `if` condition — so `set -e` applies with no exemption. When Xdebug
is genuinely absent, `grep "Xdebug v"` matches nothing and exits 1, `pipefail` propagates it, and
the script **aborts at line 27**.

Nine lines later, at line 36, sits this:

```bash
else
    echo "Xdebug is not installed or the version is not recognized."
fi
```

**That message cannot print for the case it names.** Somebody wrote an else branch for "Xdebug is
absent", and the script dies before reaching it whenever Xdebug is actually absent.

*Precision, because the first reading was too strong:* the branch is not dead code outright. It
remains reachable for a version string that matches `Xdebug v` but neither `v2.` nor `v3.` — a
hypothetical `Xdebug v4`. So it covers "unrecognized version" and cannot cover "not installed",
despite naming both.

This is the **second** occurrence of this exact shape in the same command. Unit 2 found the first:
the `docker0` lookup at `set_xdebug_host_property.sh`, also a bare assignment, also aborting where
a reader would expect a skip.

**The decision this forces, and it is not the same shape as unit 2's.** In Go the else branch
becomes reachable for the first time, so the port must choose:

- **Replicate the abort.** `hm debug-on` fails when Xdebug is not installed.
- **Make "absent" a normal, non-fatal branch** — enable what can be enabled, then report that
  Xdebug is not installed. This is what the shell's author visibly intended.

Note this pulls the *opposite* way from unit 2's settled decision, and the asymmetry is real rather
than inconsistent. There, the silent path hid a genuine misconfiguration the user could not see, so
failing loudly added information. Here, the shell *fails* and the author clearly meant to *inform* —
and "Xdebug is not installed", printed right after `debug-on` ran, is itself useful information, not
a silence. Deciding this the same way as unit 2 would be consistency for its own sake.

**Open. For the proposal to put to the user.**

## C. The macOS copy branch is already ported — question 2 costs nothing

**Verified.** `copy-from-container` is routed in `internal/cli/run.go:104`, the handler is
`copyFrom()` at `internal/cli/wrappers.go:303`, and `CopyFrom(dir string, paths []string) error` is
already on the `commands` interface at `engine.go:24`.

So `debug-on.sh:21-24` needs no new capability and no separate cycle. The handler calls
`copyFrom([]string{"generated"}, stdout, stderr, jsonOutput)` directly.

**The composition pattern is established precedent, not an invention.**
`internal/cli/wrappers_test.go`'s `TestWhatVarnishAsks/off` already covers a handler calling
sibling handler functions mid-body — `varnish` calls `purge()` and `magento()` — and the test
comments the consequence explicitly, that each goes through its own `projectOr` and so `Resolve`
appears more than once in the fake's call log. Worth knowing before writing the assertions here,
because the same extra `Resolve` calls will appear.

Also verified: `report()`/`inside()` print **nothing** on success, in either text or JSON mode. So
calling `copyFrom` mid-handler adds no stray output to `debug-on`'s own banner.

## D. The precondition at line 8

`start_service_if_not_running.sh phpfpm` probes with `compose exec -T` and, **only on status
exactly 1**, runs `$COMMAND_BIN_NAME start` — the whole environment, not just the service.

Whether the Go path needs an equivalent, and whether starting the entire environment as a side
effect of `debug-on` is behaviour worth preserving, is a question for the proposal. `debug-off`
shipped without one.

## E. `Restart` introduces a refusal that does not exist today

**Verified, and it must be a disclosed acceptance criterion rather than an inherited side effect.**

`console/commands/restart.sh` is literally `stop.sh` then `start.sh`, so `commands.Restart` is the
faithful literal port of `debug-on.sh:26` — unlike `debug-off`, whose shell called
`$DOCKER_COMPOSE restart phpfpm` directly and was genuinely a simpler call.

But `Operator.Restart` (`dockergento/app/orchestrate.go:237-247`) calls
`refuseFromAnUnregisteredWorktree` **unconditionally** — exit 6 — and `Start`'s `afterStarting()`
hands off to `hm post-start` on Linux.

Neither happens today, and the reason is a routing quirk: `bin/run`'s gate (`hm_alters_environment`,
`console/helpers/worktree.sh:85-94`) tests only the **top-level invoked name**, which is `debug-on`,
and that name is absent from the list. `restart.sh` called from inside another script never
re-enters the dispatcher, so it is never gated — even though a user typing `hm restart` is.

So porting to `Restart` makes `hm debug-on` answer exit 6 from an unregistered worktree, where it
never has. That is a real behaviour change and belongs in the proposal as a criterion.

## F. The `--json` envelope is genuinely new design

`debug-off` prints `{"xdebug": false}` — one boolean, and it had no shell precedent either.
`debug-on` prints a version plus several ini values, so the shape owed is richer.

**Checked for a precedent to reuse and found only a thin one.** `describe`'s JSON reports xdebug
state through `core.Tooling.Xdebug` as a single string — `"on"` / `"off"` / `"unknown"`. That is far
less than `debug-on` needs (idekey, host, port, and the version), so it does not dictate a shape.
What it does show is that the existing precedent is minimal, which is an argument that this is real
design work rather than reuse.

**Open. For the proposal to put to the user.**

## G. Parsing `php -v` and `php -i`

Two pure functions' worth of work:

- Extract the Xdebug version from `php -v` output, and classify it v2 / v3 / neither.
- The `php -i | grep -e ... | cut -d= -f1-2` equivalent: select the relevant ini keys and trim each
  to `key=value`. Note the key names **differ by version** — v2 uses `xdebug.remote_host` /
  `xdebug.remote_port`, v3 uses `xdebug.client_host` / `xdebug.client_port`, and both use
  `xdebug.idekey`.

These are `string → value` functions, so they are fixture-testable on any runner with no `t.Skip`,
exactly like unit 2's three parsers.

**Carry unit 2's proven lesson into the fixtures.** A realistic fixture can hide the bug it appears
to cover. Unit 2's `ip addr` fixture listed `inet` before `inet6`, so a parser that wrongly matched
`inet6` still returned the right address **by accident of ordering** — and the test built on that
realistic fixture stayed green under the mutation. Only a fixture containing *only* the excluded
thing carried the criterion.

The equivalents to write here:

- A `php -v` fixture with **no** Xdebug line at all, for the absent case — not merely one where the
  Xdebug line happens to sit late.
- A `php -i` fixture containing **both** `xdebug.remote_host` and `xdebug.client_host`, so a v3
  parse that wrongly accepted a v2 key cannot pass by the v2 key being absent.
- A version string that contains `Xdebug v` but neither `v2.` nor `v3.` — the `v4` case that is the
  only one the shell's else branch can actually reach.

## H. What is provable where

- **At the CLI seam, with `answering(t)` and `fakeEngine`**: what the handler asks of the engine and
  in what order — the `Exec` calls and their arguments, the `Capture` call, the `XdebugHost` call,
  the `Restart`, and the extra `Resolve` calls that `copyFrom` brings with it.
- **As adapter/package fixture tests**: the `php -v` and `php -i` parsers.
- **Not provable here at all**: that the `sed` edits land in the real ini, that PHP-FPM picks them up
  after restart, and what a real PHP image's `php -v` prints for each supported version. The last of
  these means the fixtures are built from documented output shapes rather than from this project's
  own images — a gap worth stating rather than papering over.

## I. Size, and why the recent run of good estimates should not be assumed

**Independent estimate: ~550-850 lines**, well above the ~350-500 the inherited document carried.

The last three units all landed at or under estimate, breaking this repository's 1.3-2x historical
overshoot. That happened for three specific reasons: decisions were settled before apply, no spec
scenarios arrived late, and seam growth was foreclosed.

**Two of those three do not hold here.** Two decisions are open (§B and §F), and the seam grows by
two methods plus a new kind of field on the fake. The third — late spec scenarios — is the one
genuinely under control, and only if §B and §F are settled in the proposal rather than drifting into
apply.

### Recommended slicing, within this one change

| Slice | What | Lands alone? |
|---|---|---|
| 1 · seam growth | `Engine.XdebugHost()` facade, both methods onto `commands`, `outcome`'s data field, `fakeEngine`'s answers | Yes — mirrors unit 1's Slice A/B shape |
| 2 · pure parsing | `php -v` version extraction and classification, the `php -i` key selection, with fixtures | Yes — no caller needed |
| 3 · the handler | `debugOn` itself: platform branch, host resolution, the two `sed` edits, the mac copy, `Restart`, the version branch, the banner, the JSON envelope | No — last, and largest |

Slice 3 is where both open decisions become RED/GREEN behaviour, which is the argument for settling
them before it starts rather than during.

## Open, carried to the proposal

1. **§B** — when Xdebug is absent, does the Go port replicate the shell's abort, or make it the
   informative non-fatal branch the shell's author visibly intended?
2. **§F** — the `--json` envelope's shape.
3. **§D** — whether the Go path needs `start_service_if_not_running`'s equivalent, given it starts
   the whole environment.

## Risks

- No `exec.Command` mock exists anywhere in this repository, so the orchestrating host and capture
  methods stay untestable at that level. Standing precedent (`Listening()`), not a new gap.
- Real `php -v` / `php -i` output for this project's supported PHP images is unverified here, since
  nothing may touch a live daemon. Fixtures come from documented shapes.
- Slice 3 is large enough that its estimate is the least trustworthy number in this document.
