# Tasks: `debug-on-host-detection`

Read together with `proposal.md` (Decisions 1-5, Approach, Success Criteria) and `specs/README.md`
(why no delta spec is written for this unit). No `design.md` exists — the user chose "specs, then
tasks" because no user-visible fork remained open once the proposal settled Decisions 1-5; the
concrete shapes below are this document's own design detail, not a separate design phase.

Unit 2 of three porting `hm debug-on` to Go. Unit 1 (`Capture`) shipped as
`capture-container-output` (archived at
`openspec/changes/archive/2026-09-11-capture-container-output/`).

**Scope reminder, not re-litigated here:** `dockergento/ports/ports.go` has no diff.
`internal/cli/`, `console/`, `bin/run` and `MIGRATION.md` have no diff. No shell file is retired;
`debug-on.sh` and `set_xdebug_host_property.sh` keep running unchanged. Everything in this unit
lands in two new files under `dockergento/adapters/machine/`.

## Baseline (do first, sequential)

- [x] **T0.1** Record this change's own parent commit: `git rev-parse HEAD` on `release/2.0.0`
      before the first edit. Use it as `<parent>` in every size/diff check below via
      `git merge-base HEAD <parent>` at check time — never a hash copied from this document, which
      goes stale. *(Verifiable here.)*
- [x] **T0.2** Confirm the tree is currently green: `go build ./...`, `go test ./...`, `gofmt -l
      ./cmd ./internal`, `gofmt -l ./dockergento`, `go vet ./...`, `tests/run.sh unit`. Record the
      Go test count and shell assertion count as the "before" baseline the later checks compare
      against. Expected: 413 Go tests across 24 packages, 616 shell assertions, 0 failed — if either
      baseline differs from what this document assumes, note the actual figures and use those.
      *(Verifiable here.)*
- [x] **T0.3** Verify both new file paths are free before writing either one:
      `dockergento/adapters/machine/xdebug_host.go` and
      `dockergento/adapters/machine/xdebug_host_test.go`. Confirmed absent as of this phase — the
      package holds only `machine.go`, `machine_test.go`, `resolver.go`, `memory_darwin.go`,
      `memory_linux.go`. Re-check at implementation time rather than trusting this document, the way
      unit 1's `resolve.go` collision should have been caught before the `Write`, not after.

## RED — parser tests first (Strict TDD)

Requirement link: proposal's "What proves what" table and Success Criteria (`hostFromIPAddr`
excludes `inet6`; `hostFromIfconfig` replicates the two-step split; `hostFromProcVersion` resolves
WSL1/WSL2/ordinary Linux including the adversarial case). These three functions are pure
`string → value`, provable on any CI runner — no `t.Skip`, no `runtime.GOOS`, matching
`fromLsof`/`fromSS`'s precedent in the same package, not `internal/cli/php_test.go`'s OS-gated
skip (wrong package, wrong shape — `mirrorsVendor` reads `runtime.GOOS` itself; these parsers do
not).

- [x] **R1** Create `dockergento/adapters/machine/xdebug_host_test.go` (package `machine`) with
      literal fixture constants — not golden files on disk, matching
      `TestReadingLsof`/`TestReadingSS`'s pattern in `machine_test.go`. Use the exploration's real
      sample content verbatim, not invented samples:
  - [x] **R1.a** `/proc/version` — WSL1 (`4.4.0-43-Microsoft`), asserting
        `hostFromProcVersion` returns `("10.0.75.1", true)`.
  - [x] **R1.b** `/proc/version` — WSL2 (`5.15.90.1-microsoft-standard-WSL2`), asserting
        `hostFromProcVersion` returns `("host.docker.internal", true)`.
  - [x] **R1.c** `/proc/version` — ordinary Linux (`5.15.0-91-generic`), asserting
        `hostFromProcVersion` returns `("", false)`.
  - [x] **R1.d** **Adversarial fixture (Decision 4).** A WSL2-shaped `/proc/version` string that
        also contains a capital-M `Microsoft` token (e.g. a build-host field like
        `(Microsoft@example.com)` appended to a `microsoft-standard` kernel string), asserting
        `hostFromProcVersion` still returns `("host.docker.internal", true)` — i.e. the
        `microsoft-standard` check runs, and wins, ahead of the plain `Microsoft` check. This is the
        one test that would fail if the parser kept the shell's original WSL1-first order.
  - [x] **R1.e** Empty `/proc/version` content, asserting `hostFromProcVersion` returns
        `("", false)`.
  - [x] **R1.f** `ip addr show docker0` — present, full multi-line output including both the
        `inet 172.17.0.1/16 ...` line and the `inet6 fe80::... ` line, asserting `hostFromIPAddr`
        returns `("172.17.0.1", true)`.
  - [x] **R1.g** **The `inet6` boundary (Success Criterion, own dedicated case, not a comment).**
        A fixture whose only `inet`-prefixed line is the `inet6` one (i.e. no plain `inet ` line at
        all — simulating a `docker0` that only shows a link-local IPv6 line, or reordered output),
        asserting `hostFromIPAddr` returns `("", false)` — proving the parser does not fall back to
        matching `inet6` when no `inet ` line exists.
  - [x] **R1.h** `ip addr show docker0` — absent (`Device "docker0" does not exist.` on stderr,
        empty stdout passed to the parser), asserting `hostFromIPAddr` returns `("", false)`.
  - [x] **R1.i** Empty string input to `hostFromIPAddr`, asserting `("", false)`.
  - [x] **R1.j** `ifconfig docker0` — net-tools legacy format (`inet addr:172.17.0.1  Bcast:...
        Mask:...`), asserting `hostFromIfconfig` returns `("172.17.0.1", true)` via the two-step
        colon-then-space split, not a single regex.
  - [x] **R1.k** Empty string input to `hostFromIfconfig`, asserting `("", false)`.
- [x] **R2** Confirm R1's tests fail to compile (the three functions do not exist yet) —
      `go test ./dockergento/adapters/machine/...` — the RED half of strict TDD. Do not proceed to
      GREEN before confirming this.

## GREEN — implement the three parsers

- [x] **G1** Implement `hostFromProcVersion` in `dockergento/adapters/machine/xdebug_host.go`
      (package `machine`), signature:

      ```go
      func hostFromProcVersion(content string) (host string, ok bool)
      ```

      Check `microsoft-standard` **before** plain `Microsoft`, both as case-sensitive
      `strings.Contains` substring checks (Decision 4 — do not "improve" this into a regex; the
      swap is correct by construction, a regex would require asserting the shape of every WSL
      kernel string that will ever exist). On a `microsoft-standard` match return
      `("host.docker.internal", true)`; on a `Microsoft` match (checked second) return
      `("10.0.75.1", true)`; otherwise `("", false)`.
- [x] **G2** Implement `hostFromIPAddr`:

      ```go
      func hostFromIPAddr(output string) (host string, ok bool)
      ```

      Preserve the shell's `grep "inet\b"` word boundary — match a line beginning (after leading
      whitespace) with `inet` followed by a space, e.g.
      `regexp.MustCompile(`^\s*inet\s+(\S+)`)` applied per line, then strip the `/NN` CIDR suffix
      the way the shell's `cut -d/ -f1` does. Must **not** match `inet6` (R1.f and R1.g are the
      acceptance criteria). Empty or no-match input returns `("", false)`.
- [x] **G3** Implement `hostFromIfconfig`:

      ```go
      func hostFromIfconfig(output string) (host string, ok bool)
      ```

      Replicate the shell's two-step split rather than a single regex: find the `inet addr` line,
      split on `:` and take field 2 (matching `cut -d ':' -f 2`), then split that on space and take
      field 1 (matching `cut -d ' ' -f 1`). Doing this in two steps, not one pattern, is deliberate —
      net-tools versions order `Bcast`/`Mask` differently, and a single regex tuned to the sample
      output would silently diverge on those. Empty or no-match input returns `("", false)`.
- [x] **G4** Run `go test ./dockergento/adapters/machine/...`; all of R1's assertions must pass
      **without editing them**. If an assertion needs changing to pass, that is a signal the
      implementation is wrong, not the test — R1 already encodes the agreed behaviour from the
      proposal's Decisions and Success Criteria.

## The orchestrating method (untested by design, matching `Listening()`)

Requirement link: proposal Decision 1 (concrete `machine.Host` method, no `ports.Machine` entry),
Decision 2 (Linux unresolved `docker0` is a failure), Decision 3 (signature distinguishes
"not applicable" from "applicable but unresolved"), Decision 5 (`runtime.GOOS`, not
`dockergento.Platform()` — using `Platform()` here is an import cycle, since package `dockergento`
imports `adapters/machine` at `dockergento.go:32` and no adapter in this repo imports the root
package back).

- [x] **M1** Add the sentinel error in `xdebug_host.go`, mirroring `ErrNoPortTool`'s exact shape
      (`machine.go:41-48`):

      ```go
      // ErrNoBridgeAddress is returned on Linux when docker0 exists but no tool on this machine
      // could resolve its address. A check that cannot look must not report that everything is
      // fine — [state what the caller loses: xdebug has no way to call back].
      var ErrNoBridgeAddress = errNoBridgeAddress{}

      type errNoBridgeAddress struct{}

      func (errNoBridgeAddress) Error() string { return "no address found for the docker0 bridge" }
      ```

      Fill in the bracketed doc-comment sentence in the adapter's own words; keep the shape
      (exported zero-value sentinel backed by an unexported empty struct type) identical to
      `ErrNoPortTool`.
- [x] **M2** Implement `Host.XdebugHost`:

      ```go
      func (Host) XdebugHost() (host string, err error)
      ```

      - Gate: `if runtime.GOOS != "linux" { return "", nil }` — **not** `dockergento.Platform()`
        (Decision 5; this also adds the first `runtime` import under `dockergento/adapters/`).
      - Read `/proc/version` (`os.ReadFile`); if `hostFromProcVersion` resolves (WSL1 or WSL2),
        return that host with a `nil` error.
      - Otherwise, prefer `ip`: if `exec.LookPath("ip")` succeeds, run
        `ip addr show docker0` and parse with `hostFromIPAddr`. If that resolves, return the host.
      - If `ip` is unavailable, fall back to `ifconfig docker0` and `hostFromIfconfig`, same
        pattern.
      - If neither tool resolves an address, return `("", ErrNoBridgeAddress)` — the Linux
        "applicable but unresolved" case Decision 2 and Decision 3 require.
      - macOS never reaches past the gate: `("", nil)`, "not applicable," distinct from the Linux
        unresolved case by construction (Decision 3's whole point — do not collapse the two).
- [x] **M3** Do **not** write a test for `Host.XdebugHost` itself. State this explicitly in the
      commit or PR description so a reviewer does not read the gap as an oversight: it does real
      I/O (`os.ReadFile`, `exec.LookPath`, `exec.Command`), nothing in this package mocks
      `exec.Command`, and `Listening()` (`machine.go:54-68`) is the standing precedent for leaving
      exactly this shape of method untested while its pure helpers are fully covered.
- [x] **M4** `go build ./...` — confirms `XdebugHost` compiles and wires to the three parsers, which
      is the only thing provable about it in this environment.

## Final verification (sequential, after the above)

- [x] **F1** `go build ./...` — clean.
- [x] **F2** `go test ./...` — all packages pass. Compare the count against T0.2's baseline: expect
      baseline + the number of new cases added in R1 (11 as listed above: R1.a-R1.k), and no other
      count moved.
- [x] **F3** `gofmt -l ./cmd ./internal` **and** `gofmt -l ./dockergento` — both empty. CI's own
      check (`.github/workflows/go.yml:26`) only runs the first; the second is on the implementer
      because this whole change lands under `dockergento/`.
- [x] **F4** `go vet ./...` — clean.
- [x] **F5** `tests/run.sh unit` — exactly `unit`, never `tests/unit` (the latter is not a directory
      under `tests/run.sh`'s own `$TESTS_DIR`, so it silently falls through to the "run everything"
      branch — verified by reading `tests/run.sh`'s target-dispatch logic directly). This change
      touches no shell code, so this is a regression check: expect 616 assertions passed, 0 failed,
      unchanged from baseline.
- [x] **F6** Confirm no `t.Skip` and no `runtime.GOOS` appears anywhere in
      `xdebug_host_test.go` — `grep -n 't.Skip\|runtime.GOOS' dockergento/adapters/machine/xdebug_host_test.go`
      returns nothing. (`runtime.GOOS` is expected and correct in `xdebug_host.go` itself, per M2 —
      this check is scoped to the test file only.)
- [x] **F7** Confirm `dockergento/ports/ports.go` has no diff:
      `git diff --stat <parent-from-T0.1>..HEAD -- dockergento/ports/ports.go` empty.
- [x] **F8** Confirm `internal/cli/` has no diff:
      `git diff --stat <parent-from-T0.1>..HEAD -- internal/cli` empty, and `commands` still has
      sixteen methods with its doc comment unchanged (same sixteen named in unit 1's tasks.md B7:
      `Resolve`, `Environments`, `Describe`, `Exec`, `Restart`, `CopyInto`, `CopyFrom`, `Property`,
      `Dump`, `Installed`, `SetHost`, `RemoveHost`, `StopEverything`, `Compose`, `Logs`, `Anonymise`).
- [x] **F9** Confirm `console/`, `bin/run` and `MIGRATION.md` have no diff:
      `git diff --stat <parent-from-T0.1>..HEAD -- console bin/run MIGRATION.md` empty.
      `MIGRATION.md:21` still reads `33 de 65`; `:261` still lists `debug-on` as `shell`.
- [x] **F10** Size check against this change's own parent:
      `git diff --shortstat $(git merge-base HEAD <parent-from-T0.1>)..HEAD -- .
      ':(exclude)openspec'`. Compare against this document's own ~195-230 estimate (below). If it
      lands past ~300, treat that as a signal to reconsider the one-commit recommendation, not a
      reason to pad the estimate after the fact.
- [x] **F11** Confirm the Rollback Plan holds: both new files are wholly additive with no edit to
      any existing file, so `git revert` of the single commit removes exactly
      `xdebug_host.go` and `xdebug_host_test.go` and nothing else changes.

## Not verifiable here (needs a live Linux host with Docker, or a real WSL machine)

- **M2/M4** — `Host.XdebugHost`'s Linux branches (`ip addr show docker0` succeeding or failing
  against a real `docker0` interface, `ifconfig` as a genuine fallback when `ip` is truly absent,
  and the WSL fixed-host branches against a real `/proc/version` on WSL1/WSL2) are not exercisable
  by automated test in this repository's current shape — no `exec.Command` mock exists in this
  package, matching `Listening()`'s own standing gap. `go build ./...` confirms the wiring compiles;
  the parsing logic each branch delegates to is fully proved by R1/G1-G3. A human with the right
  hardware (or CI with a Linux runner and Docker's default bridge present) would need to run it
  manually to confirm end-to-end behaviour — not required for this unit to ship, since nothing
  calls `XdebugHost` yet (Decision 1).

## Line estimate and commit recommendation

**Independent estimate: ~195-230 lines** (production ~95-110, tests ~100-120), derived
bottom-up rather than copied from the proposal's own ~185-215:

| Piece | Lines |
|---|---|
| `XdebugHost` (doc comment, `runtime.GOOS` gate, `/proc/version` read, `ip`-over-`ifconfig` fallback, three return paths) | ~30-38 |
| `ErrNoBridgeAddress` sentinel, in `ErrNoPortTool`'s exact shape | ~8-9 |
| `hostFromProcVersion` (doc + body, WSL2-first ordering) | ~13-16 |
| `hostFromIPAddr` (doc + body + package-level regexp) | ~16-20 |
| `hostFromIfconfig` (doc + body, two-step split) | ~15-18 |
| file header, package doc, imports | ~5-7 |
| fixture constants (WSL1, WSL2, ordinary Linux, adversarial WSL2, `ip addr` present, `ip addr` absent, `ifconfig` present) | ~35-42 |
| test functions (11 cases per R1, ~7-10 lines each including the two multi-assertion parser groups) | ~90-110 |
| **Total** | **~212-260**, centre ~230 |

This lands slightly above the proposal's own ~185-215/centre-200, mainly because R1 above spells
out **eleven** discrete assertions rather than the proposal's rougher "~7 test functions" — the
`inet6`-boundary case (R1.g) and the "absent" cases (R1.h, R1.i, R1.k) are each their own
function-level assertion here rather than folded into a shared one, since Strict TDD reads better
with one assertion failing for one stated reason. This is a documentation-granularity difference,
not a scope disagreement with the proposal.

**Recommendation: one commit, agreeing with the proposal.** ~230 (this estimate) to ~200 (the
proposal's) is comfortably under half the 400-line review budget either way, and there is no real
internal boundary to cut at: the three parsers and the method that calls them are one concern, and
splitting "parsers + tests" from "method + sentinel" (the proposal's own named contingency) would
leave the second commit's `XdebugHost` and `ErrNoBridgeAddress` with no test of their own — an
untested commit under Strict TDD, which is a weak thing to hand a reviewer on its own when nothing
about the whole change is large enough to require it. Unit 1's A/B split was justified because
slice A was a standalone refactor with its own existing-test proof; nothing here has that shape.
Treat the contingency split as a fallback if F10 crosses ~300 in practice, not as a plan to prefer.

## Verification against source, done during this phase

- **`dockergento.go:32`** imports `adapters/machine`; **`dockergento.go:1376-1382`** is `Platform()`,
  confirmed unchanged since the proposal's own re-check (`darwin → mac`, else `runtime.GOOS`
  passthrough) — Decision 5's import-cycle claim and its `runtime.GOOS` equivalence both hold.
- **`dockergento/adapters/machine/`** holds exactly `machine.go`, `machine_test.go`, `resolver.go`,
  `memory_darwin.go`, `memory_linux.go` — neither `xdebug_host.go` nor `xdebug_host_test.go` exists;
  T0.3's "verify free" step is confirmed true as of this phase, not merely assumed.
- **`machine.go:41-68`** — `ErrNoPortTool`'s shape and `Listening()`'s untested-real-I/O precedent
  read exactly as the proposal describes; M1 and M3 above cite it directly rather than at second
  hand.
- **`.github/workflows/go.yml:26`** runs `gofmt -l ./cmd ./internal` only — confirmed it does not
  reach `./dockergento`, so F3's second `gofmt` invocation is genuinely load-bearing and not a
  restatement of an existing CI check.
- **`tests/run.sh`'s target dispatch** (its `target="${1:-}"` branch, read in full): `unit` matches
  `[ -d "$TESTS_DIR/$target" ]` because `tests/unit/` exists as a real directory holding 31 suite
  files (confirmed by listing it); `tests/unit` as a literal argument does **not** match that
  branch (there is no `tests/tests/unit` directory) and falls through to the "no target" branch,
  which runs every `*_test.sh` under `tests/`. The task instruction's claim about this argument is
  correct, independently re-derived from the script's logic rather than taken on faith. Worth
  flagging as a finding, not a task: **CI's own workflow file runs `./tests/run.sh tests/unit`
  (`go.yml:44`)**, which by this same logic runs the *entire* shell suite, not "only unit," every
  time CI builds. That is pre-existing and outside this unit's scope, but is exactly the kind of
  drift a future cycle should fix, since the job's own name ("suite de bash (unidad)") says
  "unit" while its actual argument does not select unit.
- **`MIGRATION.md:21`** reads `33 de 65`; **`:261`** lists `debug-on` as `shell` — both confirmed
  by direct read, matching F9's expectation.

## Findings surfaced while writing this document (not tasks — do not act on these here)

1. **CI's shell-suite step does not do what its name says**, per the source note above
   (`go.yml:44` passes `tests/unit`, not `unit`, so it silently runs the full suite instead of just
   the unit suites). This predates this change, is not caused by it, and is out of scope to fix
   here — recorded so a reader of this document does not conclude nobody noticed.
2. **The bracketed sentence in `ErrNoBridgeAddress`'s doc comment (M1) is deliberately left for the
   implementer to word**, rather than fixed here verbatim, because `ErrNoPortTool`'s own comment
   states the consequence in the adapter's voice ("a check that cannot look must not report that
   everything is free") and this sentinel's consequence — xdebug enabled but unreachable — deserves
   the same treatment rather than a copy-pasted phrase forced to fit.
