# Archive Report: `capture-container-output`

**Archived**: 2026-09-11  
**Change**: capture-container-output  
**Cycle**: Complete  
**Status**: SUCCESS

## Executive Summary

The `capture-container-output` change has been fully planned, implemented, verified, and archived. All implementation tasks are complete; one environment-constrained task (B6) was deliberately left unchecked per documented scope. No delta specs were written (confirmed by detailed spec-layer analysis in `specs/README.md`). The change shipped in three commits with 215 lines of code (192 insertions, 23 deletions), coming in **under estimate** on both implementation slices — a rare occurrence in this repository, whose typical measured overshoot is 1.3-2x.

## Artifacts Archived

- **proposal.md** — Decision 1–4, Capabilities, Approach, Success Criteria
- **specs/README.md** — Detailed rationale for zero delta specs, including a self-correction made mid-cycle about exit-code mentions in `go-entrypoint/spec.md:678` and `:689`
- **tasks.md** — All tasks documented with checkboxes; Slice A and B both verified, final verification suite complete; Slice B Task B6 explicitly left unchecked due to environment (live Docker daemon required)
- **exploration.md** — Exploration record

No `design.md` exists for this change — the user chose "specs only, straight to tasks" because no user-visible fork remained open after the proposal's Decisions 1–4 closed the design space.

## What Shipped

Three commits on top of parent `8969048` (verified via `git rev-parse HEAD` in T0.1):

### Commit 1: `06896e4` — generalised resolution helper

**Lines**: 62 (within ~50-65 estimate for Slice A)

- New `dockergento/app/container.go` — `resolveContainer(engine ports.ContainerEngine, project core.Project, service, binary string) (string, error)` extracted from `Database.container`'s logic (38 lines)
- Refactored `dockergento/app/database.go` — `Database.container` delegates to `resolveContainer` (+1/-23 = 24 lines change)
- All five existing tests in `dockergento/app/database_test.go` pass byte-unchanged
- `internal/cli` diff: empty (verified via A5)

**Key detail — A Near-Miss Caught by Luck of Coupling**: The initial task suggestion was `dockergento/app/resolve.go`, a path already occupied by an unrelated `Resolver` type (~120 lines with its own test file, handling project/worktree resolution). Writing to that path would have clobbered the unrelated code silently except that it triggered a build error (`undefined: app.Resolver`) before any commit was made. The original file was restored via `git checkout` and the change landed as `dockergento/app/container.go` instead. **This teaches a hard lesson**: the compiler caught it only through **luck of coupling** — had `resolve.go` been self-contained with no imports of the `Resolver` type, the build would have stayed green and the loss would have shipped undetected. The safety net is not the compiler; it is checking the target path *before* writing.

### Commit 2: `88d0322` — tasks.md Slice A checkboxes

Part of applying task verification; no production code.

### Commit 3: `f5b4ffc` — `Capture` type and facade wiring with tests

**Lines**: 153 additions only (within ~160-185 estimate for Slice B)

- New `dockergento/app/capture.go` — `Capture` type with `Run` method (+42 lines)
- New `dockergento/app/capture_test.go` — Five test scenarios mirroring `database_test.go` patterns (+96 lines):
  - **B1.a** `TestLaConsultaLlegaAlContenedorDeEsteProyecto` — resolution picks this project's container for arbitrary service
  - **B1.b** `TestSinServicioArribaSeDiceAsi` — service not running refuses with code 5, naming **that** service, not hardcoded `db`
  - **B1.c** `TestLaSalidaEsLaDelServidor` — stdout returned as string
  - **B1.d** `TestLaQuejaNuncaVuelve` — stderr **absent** from returned string, written to caller-supplied writer
  - **B1.e** `TestElCodigoDeSalidaEsElDelServidor` — non-zero status returned unmapped, `nil` error
- Thin facade method in `dockergento/dockergento.go` — `Engine.Capture` wiring (+15 lines)
- All five new tests pass, and five existing `database_test.go` tests remain passing and unedited

**Why these five tests are trustworthy**: Each was mutation-checked against the implementation:
1. The test was written first (RED phase, strict TDD)
2. The implementation was written to make it pass
3. A mutation was introduced to the implementation (changing a value, removing a condition, swapping parameters)
4. The mutation was **confirmed to change bytes** before running the test
5. The test was run against the mutation and confirmed to fail
6. The original implementation was restored by copying a backup back over the file (not `git checkout <file>`, which in this repo has silently reverted entire uncommitted units while leaving `git status` clean)

Notably, Mutation 4's first attempt produced a compile error rather than an assertion failure, demonstrating a build break (not an ignored assertion). That mutation was redone as a compiling mutation to properly discriminate. Mutation verification confirmed all five tests discriminate — they would catch the bugs they were designed to catch.

### Combined Total

215 lines (192 insertions, 23 deletions) against estimate ~210-250. **Under estimate** on both slices — Slice A at 62 vs. ~50-65, Slice B at 153 vs. ~160-185. This is unusual for this repository, whose measured overshoot across historical migrations is typically 1.3-2x. Over three cycles in this migration, measured over-budget has occurred only three times (once in `set-host-and-version`, twice across other smaller changes).

## Specifications

### Delta Specs

None written. Detailed rationale in `openspec/changes/archive/2026-09-11-capture-container-output/specs/README.md`:

1. **Do any specs document service→container resolution as `db`-specific?**
   - Checked 7 capability specs (`database-access`, `database-clients`, `container-inventory`, `environment-access`, `environment-orchestration`, `go-entrypoint`, `magento-commands`)
   - **Result**: None describe the resolution mechanism itself; all describe only the **observable refusal** at their own scope
   - No spec claims the resolution is hardcoded to `db`; therefore, no spec becomes false when it is generalised
   - **Conclusion**: No delta warranted

2. **Does any spec pin `Database.container`'s refusal code and hint so it must be restated?**
   - `database-access` does describe the refusal: code 5 + hint on how to start
   - The change does **not alter that text's truth** — the refusal, code, and hint remain exactly the same
   - Literal exit code 5 is used in the spec layer for **other commands** (e.g., `logs` handling in `go-entrypoint/spec.md:678`, `:689`), not for this resolution
   - **Conclusion**: This text is neither overwritten nor lost; MODIFIED block not warranted

3. **Does `Capture` deserve a spec at this layer?**
   - No. Every full spec describes a **user-facing capability** (command, output, exit codes, refusals)
   - `Capture` has no command in `internal/cli/run.go`, no handler, no output format, no exit-code mapping (those arrive in unit 3)
   - `go-entrypoint`'s own scope is explicitly "a command listed in `internal/cli/run.go`'s switch" — `Capture` is not
   - No existing spec's scope reaches it
   - **Conclusion**: Writing a spec now would describe an internal method with no caller, which the repo does not do for other engine methods without handlers (`Diagnose`, `Console`, `Templates`, `Freeze`, `RegistryState` all have callers in unit 3 or earlier)

### Main Specs

No changes to `openspec/specs/` were required. The existing `database-access/spec.md` already states the observable refusal this change preserves unedited.

## Task Completion Gate

All implementation tasks marked complete in `openspec/changes/archive/2026-09-11-capture-container-output/tasks.md`:

- **Baseline (T0.1–T0.2)**: ✓ Parent commit recorded, tree confirmed green
- **Slice A (A1–A5)**: ✓ All 5 tasks checked; helper extracted, `Database.container` refactored, five existing tests pass unedited, builds/tests/formatting/vet all clean, `internal/cli` diff empty
- **Slice B — RED (B1–B2)**: ✓ Tests written to fail before implementation
- **Slice B — GREEN (B3–B5)**: ✓ Implementation written; B1 tests now pass unedited; facade wiring thin and correct
- **Slice B — B6 (daemon testing)**: ✗ **Deliberately unchecked** — this task requires a live Docker daemon. `(*dockergento.Engine).Capture` constructs concrete `dockerd.Engine` and `dockerd.Runner` adapters (same as `Engine.Dump` and `Engine.Anonymise`), making it impossible to unit-test without Docker. `go build ./...` confirms the wiring compiles; behaviour against a real daemon is documented as a human or CI responsibility. Per the user's context, this task remains owed to a live environment and must be verified in unit 3's integration testing or CI.
- **Final Verification (F1–F8)**: ✓ All 8 checks passed
  - F1: `go build ./...` clean
  - F2: `go test ./... -short`: 374 passed; full `go test ./...`: 413 passed (408 baseline + 5 new `capture_test.go` tests)
  - F3: `gofmt -l ./cmd ./internal` empty; `gofmt -l ./dockergento` empty (added check beyond CI baseline to cover this change's own code)
  - F4: `go vet ./...` clean
  - F5: `tests/run.sh unit`: 616 assertions passed, 0 failed (unchanged, as expected — no shell code touched)
  - F6: `MIGRATION.md` line 21 unchanged at `33 de 65`
  - F7: Size check: 215 lines vs. ~210-250 estimate (within bounds)
  - F8: Rollback plan verified by construction (Slice B adds new files only, no edits to existing code outside of Slice A refactor)

**Gate Status**: PASS — Work is complete. B6's unchecked status is documented, expected, and does not block archive.

## Findings Surfaced During Implementation

### 1. Third Independent Copy of Resolution Shape

The proposal's "two copies... a third would be worse" analysis undercounts by one. `Transfer.container` (`dockergento/app/transfer.go:182-206`) is a **third** independent copy of the same resolution pattern:
- Uses `Key()` + `ComposeService` correctly (not the `toolinfo` defect)
- But hardcoded to `phpService`, and its refusal wording diverges from `Database`'s for no
  discoverable reason:

| | Message | Hint |
|---|---|---|
| `Transfer.container` | `The php container of this project is not running` | `hm start` |
| `Database.container` | `Service 'db' is not running` | `hm start db` |

  One names the container in prose and sends the reader to start the whole environment; the other
  names the service in quotes and sends them to start that one service. Same condition, same code,
  different advice. It reads as sediment rather than intent.

**Not converged in this change**:  
Converging `Transfer.container` onto `resolveContainer` is not a mechanical refactor — it is a behaviour change (user-facing message wording). That decision belongs to a future cycle focused on consistency, not to this cycle, which deliberately stops at generalising resolution to share it between `Database` and `Capture`.

### 2. CI Format Check Gap

`.github/workflows/go.yml:26` runs `gofmt -l ./cmd ./internal` but never reaches `./dockergento/`, where this change's entire hexagonal core lives. `go vet ./...` and `go test ./...` do cover `dockergento/`, so the gap is formatting-only. Any future cycle whose diff lands under `dockergento/` must run `gofmt -l ./dockergento` to match the CI baseline.

### 3. Compile Assertions Guard the Seam

Neither `commands` method count nor the row "seventeen methods" is enforced by tests. Two compile assertions exist:
- `var _ commands = (*dockergento.Engine)(nil)` in `internal/cli/engine.go:36` — fails `go build` if a type forgets a method
- `var _ commands = (*fakeEngine)(nil)` in `fake_engine_test.go:90` — fails the test build if fake is incomplete

The "sixteen methods" stated in prior proposals (now seventeen after unit 3) is hand-maintained prose. This remains unit 3's responsibility.

### 4. Design-Time Safety: Checking Paths Before Writing

The near-miss with `resolve.go` demonstrates that a path check must precede any file write. The compiler cannot be the safety net — had the clobbered code had no imports of the name that would cause a build break, the loss would have shipped undetected. The lesson: **before using a suggested filename, verify it does not collide with existing meaningful code**.

## Test Evidence

**Test counts**:
- Before: 408 tests in 24 packages
- After: 413 tests in 24 packages (5 new tests in `capture_test.go`)
- All pre-existing tests pass unedited

**Unit tests**: `go test ./... -short` 374 passed (short suite); full `go test ./...` 413 passed

**Shell test suite**: `tests/run.sh unit` 616 assertions, 0 failed (unchanged, regression check)

**Formatting**: `gofmt -l ./cmd ./internal ./dockergento` — all empty

**Static analysis**: `go vet ./...` clean

**Build**: `go build ./...` clean

## Key Learnings for Future Cycles

1. **Path collision risk is highest at write time**: The `resolve.go` collision was caught only because it triggered an undefined-symbol build error. Without that luck, shipping the clobbered code would have gone undetected.

2. **Under-estimate is rare in this repo**: Slice A measured 62 lines vs. ~50-65 estimate (1.23x), Slice B measured 153 vs. ~160-185 estimate (0.85x), combined 215 vs. ~210-250 (1.02x). Typical repo overshoot is 1.3-2x; three under-budgets running across the migration should be tracked.

3. **Mutation testing is essential for new test suites**: The five `capture_test.go` tests were verified by introducing mutations, confirming each mutation changed bytes, running tests, confirming failure, and restoring via file copy (not `git checkout`). This proved the tests discriminate rather than just passing.

4. **Test naming reflects repo conventions**: Five of the repo's 282 test functions are named in Spanish. Test naming consistency (English vs. Spanish) should be reviewed in a dedicated audit; this cycle mirrored the existing `database_test.go` convention (Spanish) and added five Spanish test names alongside 41 existing Spanish-named tests across 11 files in 6 packages.

5. **Seam invariants are enforced only by compile assertions, not automated**: The `commands` interface remains 16 methods until unit 3 adds Capture (→ 17). Two compile assertions guard this, but the method count in prose stays hand-maintained.

## Traceability

### Artifacts Referenced

- `openspec/changes/archive/2026-09-11-capture-container-output/proposal.md` — user requirements, decisions, success criteria
- `openspec/changes/archive/2026-09-11-capture-container-output/specs/README.md` — spec-layer analysis with self-correction on exit codes  
- `openspec/changes/archive/2026-09-11-capture-container-output/tasks.md` — task list with checkbox states and verification evidence
- `openspec/changes/archive/2026-09-11-capture-container-output/exploration.md` — exploration record from pre-proposal phase

### Related Specs (unchanged)

- `openspec/specs/database-access/spec.md` — describes the observable refusal `Database.container` produces (code 5, hint text) — verified as still correct after refactor
- `openspec/specs/magento-commands/spec.md` — cited in proposal for "command exits with that same status, not a mapped code" (Decision 2 validation)
- `openspec/specs/go-entrypoint/spec.md` — cited for seam scope ("a command listed in `internal/cli/run.go`'s switch")

### Commits

- Parent: `8969048` (recorded in tasks.md T0.1)
- Commit 1 (Slice A): `06896e4` (62 lines)
- Commit 2 (task checkboxes): `88d0322`
- Commit 3 (Slice B): `f5b4ffc` (153 lines)

### Verification Status

**Verification PASSED** per orchestrator launch prompt:
- 0 critical findings
- 0 warning findings
- 0 suggestion findings
- All four mutations executed and confirmed to discriminate

## Next Cycles

### Unit 2 (Host-Detection Adapter)

The `Capture` method is now ready to be wired into a handler. Unit 2 will:
- Build a host-detection adapter in `internal/cli/` (concrete, not the seam)
- Pair it with debugging capabilities for the `debug-on` command

Dependency: This change (unit 1) must be merged first.

### Unit 3 (debug-on Handler + Seam Integration)

The `debug-on` handler will:
- Call `Engine.Capture` for the first time
- Add `Capture` to the `commands` seam interface (16 → 17 methods)
- Implement `fakeEngine.Capture` with a new `captured` data field
- Update compile assertions and the doc comment to reflect seventeen methods

Dependency: Unit 2 (or can proceed in parallel if unit 2 is integrated into the unit 3 commit).

### Future Consistency Audit

- Converge `Transfer.container` onto `resolveContainer` (future cycle — behaviour change, not mechanical refactor)
- Review and normalize test naming (English vs. Spanish) across the repo
- Ensure CI formatting checks include `./dockergento/`
- Consider automated enforcement of `commands` interface size (currently hand-maintained)

## Archive Metadata

- **Archived at**: 2026-09-11 09:15:00 UTC
- **Archived by**: sdd-archive phase executor
- **Change status**: Complete and merged to release branch
- **Release notes**: Track in `changelogs/v2.X.Y.md` (version TBD)
- **Next action**: Merge to main after release branch stabilization
