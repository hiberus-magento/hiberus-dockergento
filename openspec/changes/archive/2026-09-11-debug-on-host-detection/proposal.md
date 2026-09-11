# Proposal: the xdebug host, detected in Go

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis. Same rationale as `capture-container-output` and its
predecessors.

Unit 2 of three. Unit 1 (`Capture`) shipped today
(`openspec/changes/archive/2026-09-11-capture-container-output/`). Unit 3 is the `debug-on` handler,
which consumes both and is where either capability joins the `commands` seam.

## Intent

`console/tasks/set_xdebug_host_property.sh` (19 lines) works out which address inside a Linux
container reaches the developer's IDE, so xdebug can call back. `debug-on.sh:10-12` sources it;
`:14-17` patches `xdebug.ini` with the result. Nothing in Go can answer that question today.

This cycle builds only the answer, in `dockergento/adapters/machine`. It writes no handler, routes
no command, and does not retire a single shell file: `debug-on.sh` still runs, still sources that
task, and still does its own detection until unit 3 replaces the command.

## Scope

### In Scope

- One exported method on `machine.Host` that orchestrates detection: platform gate, `/proc/version`,
  `ip` preferred over `ifconfig`.
- Three unexported pure parsers — `hostFromProcVersion`, `hostFromIPAddr`, `hostFromIfconfig`.
- Fixture tests for the three parsers, in this package's own style.
- New files `dockergento/adapters/machine/xdebug_host.go` and `xdebug_host_test.go`. **Both paths
  verified free** (the package holds only `machine.go`, `resolver.go`, `memory_darwin.go`,
  `memory_linux.go`, `machine_test.go`). Unit 1 nearly clobbered an occupied `resolve.go` and was
  saved by luck of coupling; the check precedes the write, not the build.

### Out of Scope / Non-goals

- **`dockergento/ports/ports.go`.** `ports.Machine` does not grow. Decision 1.
- **`internal/cli`, entirely.** No `commands` method, no handler, no `fakeEngine` change, no touched
  compile assertion. This unit adds no handler, so `engine.go:14-16`'s written invariant — the
  interface "names what the handlers routed through `newEngine` call and nothing else" — stays true
  without effort.
- **No shell file is retired.** `debug-on.sh` and `set_xdebug_host_property.sh` both keep running
  unchanged. `MIGRATION.md:21` stays `33 de 65` and `:261` keeps `debug-on` on the shell side.
- **`toolinfo.Reader.Xdebug`** (`adapters/toolinfo/tooling.go:67-124`). Its `phpfpm` match at
  `:84-85` is a hand-rolled label disjunction — a defect now recorded across four cycles. Named here
  so the overlap does not read as an oversight; converging it on `core.Container.Key()` is its own
  change.
- **`Transfer.container`** (`app/transfer.go:182-206`), the third copy of the resolution shape unit 1
  surfaced. Untouched.
- The `sed` patch, the macOS copy branch, `php -v`/`php -i` parsing, the banner, `--json`, exit-code
  mapping — all unit 3.

## Decisions

### 1. The adapter is concrete; the seam is deferred (settled by the user)

The method lives only on `machine.Host`. Nothing is added to `ports.Machine`, nothing to `commands`.

**Precedent, verified against source**: `dockergento.go:860` constructs `machine.Host{}` and calls
`ResolvesLocally` on it directly, bypassing `ports.Machine` entirely — even though that method *does*
have an interface entry (`ports/ports.go:150`). What the call site proves is not "a `Host` method may
skip the interface" but the narrower and sufficient claim: **a concrete `machine.Host` call is an
established way for this codebase to reach the machine.** The exploration's phrasing of this
precedent was wrong; the decision it supports is not.

This mirrors unit 1 exactly, where `Capture` landed on `*dockergento.Engine` with no caller and no
seam entry. Unit 3 wires both.

### 2. On Linux, an unresolvable `docker0` is a failure, not a warning (settled by the user)

The method reports it as a failure. Unit 3's handler will exit non-zero saying the IP of `docker0`
could not be resolved and that xdebug would have no way to call back.

This is what the shell does today, though by accident: `set -euo pipefail` at
`set_xdebug_host_property.sh:2`, with the pipeline at `:12` inside an `if` **body** (no `set -e`
exemption), `grep "inet\b"` exiting 1 on empty stdout, `pipefail` propagating it. `debug-on.sh`
`source`s the file, so the abort kills the whole command. The `ifconfig` branch at `:14` is exposed
identically.

**Why a failure and not a silent skip.** A skip does not spare the affected user the problem, it
postpones it and removes the clue. "Xdebug enabled but unreachable" presents as "my breakpoint does
not fire", with nothing pointing at the cause. The affected minority — rootless Docker, a renamed
bridge, a custom network — loses a command that appeared to work while leaving xdebug broken.

**No `--force`-style escape flag.** Offered and declined: extra surface to document, test and
maintain on a command that has no flags today.

### 3. The signature distinguishes "not applicable" from "applicable but unresolved"

This is the design constraint the exploration commits to and Decision 2 depends on. macOS never runs
detection and has nothing to patch; Linux with no `docker0` address is a failure. Collapsing both
into one empty answer is the mistake to avoid — unit 3 could not then render the right thing for
each.

**Recommended shape**, mirroring this package's own precedent rather than inventing one:

```go
func (Host) XdebugHost() (host string, err error)
```

- macOS → `("", nil)`: not applicable, nothing to patch, no failure.
- Linux, resolved → `("172.17.0.1", nil)` or the WSL fixed host.
- Linux, unresolved → `("", ErrNoBridgeAddress)`.

`Listening()` (`machine.go:54-68`) already returns `nil, ErrNoPortTool`, and that sentinel's own doc
comment (`:41-44`) states this cycle's Decision 2 in advance: *"a check that cannot look must not
report that everything is free."* A `(host string, resolved bool)` pair is equally acceptable to the
user's decision, but it needs a third state bolted on to carry the distinction; the error carries it
for free and matches the neighbour.

### 4. The WSL2 pattern is tested first — a deliberate improvement, not a port bug

The shell tests `grep -q Microsoft` before `grep -q microsoft-standard`. A WSL2 kernel string
carrying a capital-M `Microsoft` token anywhere — a build-host field, for instance — matches the WSL1
pattern first and is handed the fixed address `10.0.75.1` instead of `host.docker.internal`.
Breakpoints then silently fail.

**Recommendation: swap the order — test `microsoft-standard` first, keep both as case-sensitive
substring checks.** Rationale:

- It is the minimal fix. One ordering swap, provable by one fixture: a WSL2 string containing
  `Microsoft` must resolve to `host.docker.internal`.
- Matching "more precisely" instead would require anchoring a regex against the shape of every WSL
  kernel string that exists or will exist. The exploration explicitly downgraded exactly that
  universal claim as unknowable from this repository; a precise pattern would re-assert it.
- `microsoft-standard` is the WSL2 kernel's own name and is strictly the more specific token.
  Most-specific-first is the correct discipline for a substring cascade, and it makes the
  fallthrough correct by construction rather than by luck of sampling.
- The inverse risk is smaller, not merely different: a WSL1 string would have to contain
  `microsoft-standard` to be misdetected, and the WSL1 kernel is `4.4.0-43-Microsoft`.

Recorded as a **behavioural improvement over the shell**, not a transcription error, so a reviewer
comparing the two does not file it as a port bug.

### 5. `Platform()` cannot be used here — it would be an import cycle

Inherited §B recommended gating on `dockergento.Platform()` rather than raw `runtime.GOOS`. **That is
not implementable once Decision 1 puts the method in `dockergento/adapters/machine`.** `Platform()`
lives in package `dockergento` (`dockergento.go:1376-1382`, re-verified today — it moved from `:1361`
since the last cycle), and that package imports `adapters/machine` (e.g. `dockergento.go:188`,
`:1082`). The reverse import is a cycle. No adapter in this repo imports the root package.

The gate is therefore `runtime.GOOS != "linux"`, which is **exactly equivalent**: `Platform()` only
rewrites `darwin→mac` and returns `runtime.GOOS` otherwise, so `Platform() == "linux"` iff
`runtime.GOOS == "linux"`. This adds the first `runtime` import under `dockergento/adapters/`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This change adds a machine-adapter capability with no command, no output, no exit code and no
caller; no existing spec's scope reaches it, exactly as unit 1's `specs/README.md` concluded for
`Capture`. The spec phase should confirm rather than assume — in particular that no spec documents
`hm debug-on`'s current host-detection behaviour, which Decision 2 and Decision 4 would then change
at the spec level in **unit 3**, not here.

## Approach

| Layer | Change |
|---|---|
| `dockergento/ports` | **nothing** — Decision 1 |
| `dockergento/adapters/machine/xdebug_host.go` | New: `Host.XdebugHost`, the sentinel, three parsers |
| `dockergento/adapters/machine/xdebug_host_test.go` | New: fixture tests for the three parsers |
| `dockergento/dockergento.go` | **nothing** — no facade method; unit 3 calls the adapter |
| `internal/cli` | **nothing** |
| `console/`, `bin/run`, `MIGRATION.md` | **nothing** |

A new file rather than growing `machine.go` (237 lines, seven unrelated concerns): `resolver.go`
already establishes the same-package sibling file for one cohesive concern, and it carries no test
file either.

### What proves what

| Claim | Proved by |
|---|---|
| `hostFromProcVersion` returns `10.0.75.1` for WSL1, `host.docker.internal` for WSL2, nothing for ordinary Linux | fixture strings, literal in the test file, from real kernel version lines |
| A WSL2 string carrying a capital-M `Microsoft` token still resolves to `host.docker.internal` | the adversarial fixture Decision 4 exists for |
| `hostFromIPAddr` extracts `172.17.0.1` from real `ip addr show docker0` output | fixture |
| **`hostFromIPAddr` does not match the `inet6` line** | dedicated fixture case — see Success Criteria |
| `hostFromIfconfig` extracts `172.17.0.1` from net-tools `inet addr:172.17.0.1  Bcast:...` | fixture |
| Empty or error-only output yields no host from either parser | fixture with empty input |
| `Host.XdebugHost` itself | **not tested**, exactly like `Listening()`. It does real I/O — `os.ReadFile("/proc/version")`, `exec.LookPath`, `exec.Command` — and nothing in this package mocks `exec.Command`. Said plainly rather than implied |

**No `t.Skip` anywhere in this unit, and that is not an omission.** The three parsers take a `string`
and return a value; the platform never enters them, so they are provable on any CI runner, macOS or
Linux, exactly like `fromLsof`/`fromSS`. Inherited §C proposed `t.Skip`-gating on the precedent of
`php_test.go:15-25`, which is both the wrong path (`internal/cli/php_test.go`, not under
`dockergento/adapters/`) and the wrong shape: that test skips because `mirrorsVendor` reads
`runtime.GOOS` **itself**. Following it here would produce OS-gated tests for code that has no OS
branch inside it. This package contains no `t.Skip` and no `runtime.GOOS` today; its precedent for
platform difference is the `memory_darwin.go`/`memory_linux.go` build-tag pair, both untested.

Strict TDD, RED first on the three parsers.

### Two parsing details a naive port breaks

1. **`inet\b`.** The shell's `grep "inet\b"` excludes the `inet6` line: `\b` is a word boundary and
   there is none between `inet` and `6`. `strings.Contains(line, "inet")` matches `inet6` and
   extracts a link-local IPv6 address. Match `inet` followed by a space, or
   `regexp.MustCompile(`^\s*inet\s`)` per line.
2. **The `ifconfig` two-step split.** `cut -d ':' -f 2` then `cut -d ' ' -f 1` — replicate both steps
   rather than collapsing them into one regex. Field 2 of the colon split is `172.17.0.1  Bcast`
   precisely because `Bcast:` introduces the next colon; a single regex tuned to that line silently
   diverges on net-tools versions that order `Bcast`/`Mask` differently.

## Size

Derived bottom-up, counting additions and deletions, with this repo's doc-comment weight:

| Piece | Lines |
|---|---|
| `XdebugHost` (doc, `runtime.GOOS` gate, `/proc/version` read, `ip`-over-`ifconfig` fallback) | ~35 |
| `ErrNoBridgeAddress` sentinel, in `ErrNoPortTool`'s shape (`machine.go:41-48`) | ~9 |
| `hostFromProcVersion` | ~14 |
| `hostFromIPAddr` (+ package-level regexp) | ~18 |
| `hostFromIfconfig` | ~17 |
| file header and imports | ~6 |
| fixture constants (4 `/proc/version`, `ip addr`, `ifconfig`, empty) | ~30 |
| test functions, ~7 at 8-12 lines each | ~75 |
| **Total** | **~185-215, centre ~200** |

**Higher than the exploration's 130-175, and that is a correction rather than an overshoot factor.**
The exploration did not count the sentinel (Decision 3), the adversarial WSL fixture (Decision 4), or
the fact that this package's real test-to-production ratio is nearer 1.25:1 than 1:1 —
`fromLsof`+`fromSS` are 48 lines against a ~60-line test file.

**Which historical pattern applies: the recent one, not the 1.3-2x overshoot.** I do **not** stack
the historical factor on top of ~200. The three things that produced this repo's overshoots are all
absent here: decisions arriving unresolved into apply (all three are settled before this proposal),
scenario growth added late by the spec phase (there is no delta spec and no user-visible behaviour to
scenario), and seam/fake growth (Decision 1 forecloses it). That is the same profile as unit 1's two
slices, which came in at 1.23x and 0.85x. Expect ~200, budget to 250.

**Recommendation: one commit.** 200 is half the 400-line review budget, and the exploration is right
that there is no natural internal boundary — the parsers and the method that calls them are one
concern, and unit 1's A/B split worked only because slice A was a standalone refactor with its own
proof. `sdd-tasks` should forecast this **Low**. Contingency if it crosses ~250 in practice: slice A
= the three parsers plus their fixture tests (~150, fully provable, compiles and lands alone), slice
B = `XdebugHost` plus the sentinel (~55). Named as a fallback, not a plan — slice B alone would be an
untested commit under strict TDD, which is a poor thing to ask a reviewer to accept on its own.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| A naive `strings.Contains(line, "inet")` matches `inet6` and ships a wrong address | Med | Explicit success criterion with its own fixture; it is the one place a plausible port silently breaks behaviour |
| The signature is wrong for unit 3, with no caller to have proved it | Med | Decision 3 pins the shape against `Listening()`'s precedent; changing it is cheap while nothing calls it. The cost the deferral accepts |
| `XdebugHost` is untested I/O and its Linux branches drift | Med | Matches `Listening()`'s standing precedent exactly. Not a new gap — no `exec.Command` mock exists in this package and none is introduced here. The branches are thin; the parsing they delegate to is proved |
| A reviewer reads the WSL order swap as a port bug | Med | Decision 4 names it a deliberate improvement, with the failure it prevents |
| A reviewer hunts for the missing `ports.Machine`/`commands` entry, or for a retired shell file | Med | Both stated in Non-goals and repeated in `tasks.md` |
| `Platform()`'s line reference drifts again before unit 3 | High | It already moved `:1361` → `:1376`. Decision 5 makes the citation moot for this unit; unit 3 must re-verify rather than copy |
| Decision 2 regresses a Linux minority with a nonstandard bridge | Low | Accepted deliberately, with reasoning recorded. What it regresses is a command that appeared to work while leaving xdebug unreachable. No behaviour changes for any user until unit 3 ships the handler |

## Rollback Plan

One commit on `release/2.0.0`, **purely additive: two new files and no edit to any existing file.**
`git revert` removes both. Nothing calls the new code, no command is routed, no shell file changed,
no user-visible behaviour exists to restore.

## Dependencies

None. Unit 1 is merged, but this unit does not touch it and could have landed first.
**No impact on existing projects and no migration**: no command changes, no state written, no files
created in any project, no `bin/run` path touched.

## Success Criteria

- [ ] `hostFromIPAddr` extracts `172.17.0.1` from real `ip addr show docker0` output and **does not
      match the `inet6` line** — a dedicated failing-first test, not a comment.
- [ ] `hostFromIfconfig` extracts `172.17.0.1` from the net-tools line by the same two-step
      colon-then-space split the shell uses.
- [ ] `hostFromProcVersion` answers `10.0.75.1` for WSL1, `host.docker.internal` for WSL2, and
      nothing for ordinary Linux — **including for a WSL2 string that contains `Microsoft`**.
- [ ] `Host.XdebugHost` distinguishes not-applicable (macOS) from applicable-but-unresolved (Linux,
      no `docker0`), so unit 3 can render each differently.
- [ ] `dockergento/ports/ports.go` has **no diff**. `internal/cli/` has **no diff**; `commands` still
      has sixteen methods and its doc comment is unchanged and still true.
- [ ] `console/`, `bin/run` and `MIGRATION.md` have **no diff**. `MIGRATION.md:21` still says
      `33 de 65` and `:261` still lists `debug-on` as `shell`.
- [ ] No `t.Skip` and no `runtime.GOOS` appears in any test added by this change.
- [ ] `go build ./...`, `tests/run.sh` green, `gofmt -l ./cmd ./internal` **and** `gofmt -l
      ./dockergento` empty (CI's check stops short of `./dockergento`, where this whole change
      lands), `go vet ./...` clean.
- [ ] `tests/run.sh unit` still reports 616 shell assertions, 0 failed — no shell code was touched.
