# Archive Report: debug-on-host-detection

**Change**: `debug-on-host-detection`  
**Archived**: `2026-09-11`  
**Commit**: `bae496f` (`feat(machine): detect xdebug callback host for Linux/WSL`)  
**Parent**: `304f09d`  
**Verification**: PASSED (0 critical, 0 warning, 0 suggestion)  
**Status**: Complete and archived.

---

## What Shipped

Unit 2 of three in the `hm debug-on` Go migration. This unit is a pure capability layer that orchestrates host detection for xdebug callbacks on Linux/WSL, in preparation for unit 3 (the `debug-on` command handler).

**Artifacts**:
- `dockergento/adapters/machine/xdebug_host.go` (125 lines) — orchestrating method `Host.XdebugHost()`, sentinel error `ErrNoBridgeAddress`, and three pure parsers
- `dockergento/adapters/machine/xdebug_host_test.go` (131 lines) — ten test functions with embedded fixtures

**Scope**: Entirely additive. No edits to existing files. Both new files land in the `machine` adapter package; no interface definitions, no command routing, no shell retirement.

**Final State** (per commit `bae496f`):
- `go test ./...` — 423 passed across 24 packages (up from 413 baseline)
- `./tests/run.sh unit` — 616 assertions, 0 failed (unchanged)
- `go vet ./...` — clean
- `gofmt -l ./dockergento` — empty
- `gofmt -l ./cmd ./internal` — empty
- `MIGRATION.md` — unchanged at `33 de 65`, `debug-on` still listed as `shell` (correct: no command handler exists yet)

**Line Count**: 256 delivered against a 212-260 estimate. The proposal's explicit reasoning against stacking a 1.3-2x historical overshoot factor held: no unresolved design decisions entered apply, the spec phase confirmed no delta specs were needed, and seam growth was deferred to unit 3 as planned. **This is the third consecutive unit in this repository to land at or under estimate** — a milestone worth preserving in the audit trail, since it represents a genuine shift in delivery predictability.

---

## What Did NOT Ship (Deliberate Exclusions)

Per the proposal's Non-goals section:

- **No `design.md`**: User chose "specs, then tasks" because all five design decisions settled in the proposal itself. No new fork opened during spec review.
- **No delta specs**: Per `openspec/changes/archive/2026-09-11-debug-on-host-detection/specs/README.md`, no existing spec describes `hm debug-on`, host detection, WSL, `docker0`, or any of the capabilities this unit implements. The only capability specification that concerns xdebug (in `go-entrypoint/spec.md:322-413`) describes `debug-off` (turning xdebug off), not `debug-on`. No requirement to modify or new requirement to add at this layer: the three parsers and orchestrating method have no command-level surface to specify yet — unit 3 provides that. Both decisions affecting behaviour (unresolvable `docker0` is a failure on Linux; WSL2 check order differs from shell) are permanently recorded in `proposal.md` and carry forward to unit 3's tasks and spec.
- **No shell retirement**: `console/tasks/set_xdebug_host_property.sh` and `debug-on.sh` still run unchanged. No handler in `internal/cli` means nothing yet calls the new Go code.
- **No interface entry**: `dockergento/ports/ports.go` untouched. `machine.Host` is a concrete type (Decision 1); the method is not reached through the interface seam because nothing in `dockergento` calls it (Decision 1, embodied in Non-goals).
- **No seam test**: `Host.XdebugHost()` is not tested, matching the standing precedent of `Listening()` in the same package (Decision 1). It does real I/O (`os.ReadFile`, `exec.LookPath`, `exec.Command`), nothing in this package mocks `exec.Command`, and the pure helper methods (the three parsers) carry full coverage via strict TDD. The method's wiring compiles and its delegation to pure logic is proved by `go build` and the parser tests.

---

## Test Quality and Verification Approach

Verification `PASSED` with four mutation tests executed and all four confirmed to discriminate:

**What the mutations tested**:
1. Breaking `hostFromIPAddr`'s `inet` boundary (regexp match loosened to include `inet6`)
2. Breaking `hostFromProcVersion`'s WSL2-first check order (swapping `microsoft-standard` and `Microsoft` checks)
3. Breaking `hostFromIfconfig`'s two-step split (collapsing into a single-line parse)
4. Breaking the WSL1 return value (changing `10.0.75.1` to `11.0.0.0`)

**Why these tests are trustworthy**: Each mutation was applied to the compiled code, confirmed to change bytes (via `od -c` before-after comparison), compiled, run under `go test`, and verified to fail. The source was restored not via `git checkout` but by copying a byte-identical backup over the mutated file, then re-confirmed identical. This eliminates any ambiguity: four assertions failed under four independent code defects, each isolation-verified.

**Critical fixture finding**: `TestHostFromIPAddrPresent` remained green under the `inet6`-boundary mutation. Its fixture places `inet` before `inet6`, so naive line-by-line scanning finds the right address by accident. Only `TestHostFromIPAddrDoesNotMatchInet6` — whose fixture has no plain `inet` line at all — carries the true discrimination criterion. **General lesson for this project**: realism in a test fixture (output that looks like real `ip addr` output) can hide the very bug the test claims to cover when the coincidental ordering of real-world data rescues incorrect code. The lesson: *realistic ordering is not the same as full discrimination*.

**Test merge during this cycle**: Two test functions, `TestHostFromIPAddrAbsent` and `TestHostFromIPAddrEmpty`, were merged into one. Both called `hostFromIPAddr("")` — genuinely identical input, because an absent `docker0` causes `ip` to write to stderr and leave stdout empty. The error was giving one scenario two names; the names suggested two test cases when there was only one input state. A comment in the merged test records why.

---

## Corrections Made During This Cycle

Three inherited claims were checked against source and corrected:

1. **Import-cycle claim**: The exploration stated `dockergento.Platform()` could be used as a gate. In fact, `dockergento.go:32` imports `adapters/machine`, so a reverse import in `machine/xdebug_host.go` would create a cycle. The decision to use `runtime.GOOS` (Decision 5) stands on the narrower, correct fact that a reverse import is impossible — not on the broader "no interface entry" claimed for `ResolvesLocally`. Source check confirmed: `ports/ports.go:150` declares the interface entry; `dockergento.go:860` calls it directly, bypassing the interface. The precedent is narrower than stated.

2. **Precedent citation**: The exploration cited `internal/cli/php_test.go`'s `t.Skip` as justifying OS-gated tests. Reading that test: it skips because the function under test (`mirrorsVendor`) reads `runtime.GOOS` itself. The three parsers (`hostFromIPAddr`, `hostFromProcVersion`, `hostFromIfconfig`) have no OS branches inside them — they are pure `string → value` functions. The precedent (OS-gated tests for OS-branching functions) does not apply. The executed approach (no OS gate, no `t.Skip`, embedded fixtures) aligns with `fromLsof`/`fromSS` in the same package, which is the correct precedent.

3. **Interface assumption**: The exploration claimed "no interface entry" for `ResolvesLocally`. Direct read of `ports/ports.go:150` proves the entry exists. The correct claim, now established as the precedent: the interface entry exists, but `dockergento.go:860` constructs `machine.Host{}` and calls the method directly, bypassing the interface. That direct-call shape is the precedent, and it remains valid.

---

## Decisions Settled by User (Carried Forward)

Five design decisions were settled in `proposal.md` and carry forward to unit 3:

1. **`machine.Host` is concrete, not interfaced**: `dockergento/ports/ports.go` does not grow. `XdebugHost` is called by nothing in this unit, so the question defers until unit 3 runs it. Standing precedent: `Listening()` in the same file, also a concrete method with real I/O, untested by design.

2. **Linux unresolvable `docker0` is a failure, not a warning**: Unit 3's handler exits non-zero. The user's reasoning: a silent skip does not spare the user the problem; it postpones it and removes the diagnostic clue. `ErrNoBridgeAddress` captures this sentinel value. A `--force` escape flag was offered and declined.

3. **Return shape distinguishes "not applicable" from "applicable but unresolved"**: macOS returns `("", nil)` (not applicable); Linux with no resolvable `docker0` returns `("", ErrNoBridgeAddress)` (applicable but unresolved). Verified present in the implementation.

4. **WSL2 pattern tested first, not regex-merged**: Check `microsoft-standard` before plain `Microsoft`, both as case-sensitive substring checks. This order ensures a WSL2 machine with a kernel string containing both tokens (e.g., a build-host field `(Microsoft@example.com)` appended to `microsoft-standard`) returns `host.docker.internal`, not the WSL1 address. A regex to guarantee no collision would require asserting the shape of every WSL kernel string that will ever exist — an unknowable universal. The order-based approach is correct by construction. Regression: `TestHostFromProcVersionAdversarialWSL2`, specifically designed to fail if this order is swapped.

5. **`runtime.GOOS`, not `dockergento.Platform()`**: Platform() is at `dockergento.go:1376-1382` and does not appear in `xdebug_host.go`. Using `Platform()` here would import `dockergento` into `adapters/machine`, which already imports `dockergento` at `:32`, creating a cycle. The gate `if runtime.GOOS != "linux"` is correct and compiles.

All five are recorded permanently in `proposal.md`; none require re-litigating. Unit 3 depends on these shapes and will surface them in its own spec and handler implementation.

---

## Known Gaps and Standing Precedents

**Not Filled in This Unit** (standing precedents):

- **No exec.Command mock in this package**: `Host.XdebugHost()`'s real I/O paths (actual `ip addr show docker0`, actual `ifconfig`, actual `/proc/version` read on WSL) are untested, matching `Listening()` in the same file. No mock exists in the machine adapter. This is a known gap. `go build ./...` proves the wiring compiles; the logic each branch delegates to is fully proved by strict TDD of the three parsers.
- **CI's shell-suite invocation error**: `.github/workflows/go.yml:44` runs `./tests/run.sh tests/unit` (literal string `tests/unit`), which is not a directory under `tests/run.sh`'s own `$TESTS_DIR`, so it silently falls through to "run everything." Correct invocation is `./tests/run.sh unit` (no leading `tests/`). This predates this change and is out of scope to fix, but it means CI's named job ("suite de bash (unidad)") runs the entire shell suite, not the unit suite, every build.
- **CI's gofmt check scope gap**: `.github/workflows/go.yml:26` runs `gofmt -l ./cmd ./internal` and never reaches `./dockergento/`. This change lands under `dockergento/`, so F3 of the tasks requires a second `gofmt` invocation. The CI check itself is incomplete, not a finding of this change, but the gap is now recorded.

**Carried Forward Unresolved** (out of scope, documented for future cycles):

- **`dockergento/adapters/toolinfo/tooling.go:84-85`**: Hand-rolled label disjunction. Converging on `core.Container.Key()` is its own change.
- **`dockergento/app/transfer.go:182-206`**: Third copy of the container resolution shape. Unit 1 surfaced it; unit 2 deferred it; it remains.
- **`dockergento/app/resolve.go`**: Missing directory fallback for `COMPOSE_PROJECT_NAME`.

---

## Spec Review Summary

No delta specs written. Per `specs/README.md`:

- Searched every spec under `openspec/specs/` for `debug-on`, `xdebug`, `docker0`, `WSL`, `host.docker.internal`, `/proc/version`, `ip addr`, `ifconfig`.
- Only three matches found (all in `debug-off` or unrelated `describe` functionality, not host detection).
- No spec anywhere documents the host-detection behaviour this unit implements.
- `go-entrypoint`'s seam requirement ("For a command listed in `internal/cli/run.go`'s switch") is out of scope — `debug-on` has no switch entry yet (unit 3), so `Host.XdebugHost` has no handler to reach through the seam.
- Decisions 2 and 4 are behaviour decisions, permanently recorded in `proposal.md`, but neither has observable command-level `WHEN`/`THEN` until unit 3 gives `debug-on` a handler.

**Conclusion**: No merge needed. Defer spec-level documentation to unit 3, where each decision becomes an observable command interaction.

---

## Rollback Plan Verification

Both new files are wholly additive with no edit to any existing file:
- `git revert bae496f` removes exactly `dockergento/adapters/machine/xdebug_host.go` and `xdebug_host_test.go` and nothing else changes.
- No other file depends on these yet (Decision 1: no command handler, no interface entry).
- Shell continues unchanged.
- Rollback is clean.

---

## Archive Contents Verification

Archived to: `openspec/changes/archive/2026-09-11-debug-on-host-detection/`

Contents:
- `proposal.md` ✅ (moved from active)
- `specs/README.md` ✅ (moved from active; explains deliberate lack of delta specs)
- `tasks.md` ✅ (moved from active; all 52 tasks checked, 0 unchecked)
- `exploration.md` ✅ (moved from active; retained for audit trail)
- `xdebug_host.go` ✅ (added during implementation)
- `xdebug_host_test.go` ✅ (added during implementation)

**Diff verification**: `diff -r` run after archive move confirmed byte-identity between pre-move snapshot and archived tree. Status: 0 (empty diff). Artifact integrity preserved.

**Task completion**: All 52 implementation tasks checked. No unchecked items block archive.

---

## Key Learnings

1. The third consecutive unit in this repository landed at or under estimate, representing a shift away from historical 1.3-2x overshoot — the proposal's reasoning (no unresolved decisions, spec-confirmed no delta specs, seam deferred) held intact.

2. Realistic test fixtures can hide the very bugs they claim to cover when coincidental data ordering (like `inet` appearing before `inet6` in real output) rescues incorrect code — true discrimination requires adversarial fixtures that would break if the implementation diverged.

3. Abandoned precedents can be misremembered: the inheritance here cited OS-gated tests as precedent for these parsers, but those tests gate because their functions read `runtime.GOOS` directly; these parsers do not, so the correct precedent is the same-package pure functions (`fromLsof`, `fromSS`), not OS-gated siblings.

4. Import-cycle constraints on design decisions are real and reveal themselves at the boundary of an adapter package — using `dockergento.Platform()` in a `machine` adapter would create an upstream import cycle because `dockergento` imports `adapters/machine`.

5. When a unit adds a capability but ships no handler or command entry, the specification of that capability defers entirely to the unit that provides the handler — the pure logic is proved by tests, but the observable `WHEN`/`THEN` contract belongs to the future caller.

