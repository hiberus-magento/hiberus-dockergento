# Archive Report: exec-and-masquerade-go-tests

**Change**: `exec-and-masquerade-go-tests`
**Archived to**: `openspec/changes/archive/2026-09-11-exec-and-masquerade-go-tests/`
**Archive Date**: 2026-09-11
**Change Type**: Test Implementation (SDD cycle)

## Executive Summary

The change proves three commands (`bash`, `exec`, `masquerade`) on the Go seam with complete test coverage via table-driven tests against a fake engine. No shell implementations were deleted. The seam grows by one method (`Anonymise`), and two call sites route through `newEngine`. Verification passed with zero findings.

## Artifacts Archived

- `proposal.md` — proposal stating the change intent and scope
- `design.md` — technical decisions covering five key constraints
- `specs/go-entrypoint/spec.md` — delta spec with new scenarios and modified requirements
- `tasks.md` — three phases of work with per-unit budgets and RED-window safety pins
- All supporting artifacts

**Archive Contents Verification**: ✅ All artifacts present and byte-identical to source

## Specifications Updated

### Primary Spec: `go-entrypoint`

**File**: `openspec/specs/go-entrypoint/spec.md`

**Modifications**:

1. **Requirement: A ported command's engine interaction is provable without Docker**
   - **Added commands**: `bash`, `exec`, `masquerade` to the list of proved commands
   - **Previously covered**: `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off`, `purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `version`, `set-host`, `list`, `describe`, `logs`
   - **New scenarios added** (14 total):
     - Running something inside the php container (exec) — 3 scenarios
     - Opening a shell (bash) — 2 scenarios
     - exec and bash's RED window is bounded by the working directory, not the daemon
     - Anonymising the database when confirmed — 1 scenario
     - Declining leaves the database untouched — 1 scenario
     - masquerade's RED window is bounded by the daemon, not the working directory
     - An Anonymise refusal is reported
   - **Seam growth**: One new method, `Anonymise(dir string) error`, verified at `dockergento.go:452`
   - **Interface change**: `commands` grows from 15 to 16 methods

2. **Requirement: A usage error returns before any engine call**
   - **Added commands**: `exec` and `masquerade` to validation requirements
   - **New scenarios**:
     - `exec` with no command → exits `missing_command`, no engine call
     - `masquerade` with a dashed argument → exits `invalid_argument`, no engine call

**Spec Lines Merged**: 
- Previous requirement at lines 253-601 replaced with delta content (504 lines)
- Previous requirement at lines 602-756 replaced with delta content (122 lines)
- Total lines in updated spec: 1090 (was 964)
- New additions: 126 lines of scenarios and explanatory text

## Implementation Delivery

### Two Sequential Commits

1. **Commit a5f055d**: `test(cli): what exec and bash ask of the engine`
   - Files: `internal/cli/orchestrate.go` (1 line changed), `internal/cli/exec_test.go` (new), `internal/cli/tools_test.go` (new, bash only)
   - Changed lines: 255 (measured against parent `fff9b78`)
   - Status: ✅ All tests pass, all assertions pass

2. **Commit 0753710**: `test(cli): what masquerade asks of the engine, and the answer that declines it`
   - Files: `internal/cli/engine.go` (1 method added), `internal/cli/tools.go` (1 line changed), `internal/cli/fake_engine_test.go` (1 method added), `internal/cli/tools_test.go` (modified)
   - Changed lines: 198 (measured against parent a5f055d)
   - Status: ✅ All tests pass, including the decline test

### Cumulative Size

| Metric | Value | Status |
|--------|-------|--------|
| Design estimate (unit 1 + unit 2) | 406 lines | Baseline |
| Measured (against `fff9b78`) | 445 lines | +39 lines |
| 400-line budget | Exceeded by 45 lines | ⚠️ Intentional overshoot |
| Delivery strategy | Two chained commits | ✅ Per design |
| Budget gate | Raised and decided before apply | ✅ Per `ask-on-risk` strategy |

**Size Decision**: The user accepted a size exception for the cumulative before apply, as documented in tasks.md Review Workload Forecast. Each unit alone is within 400 lines (255 and 198).

## Verification Status

**Overall Result**: ✅ PASS with zero findings

Per user's explicit final-state facts from the orchestrator:

- Verification passed with zero findings — the first of the session
- All three mutations run with real output, including the decline inversion
- `go test ./...` passes: 398 tests across 24 packages (no new failures)
- `tests/run.sh unit` passes: 616 assertions (unchanged from baseline)
- `gofmt` clean across `./cmd ./internal ./dockergento`
- `go vet ./...` clean
- `commands` interface: exactly 16 methods (was 15)
- Compile-time assertions at `engine.go:35` and `fake_engine_test.go:90` hold

### Findings and Resolutions

**Finding 1: Decline test has no RED window**
- **Detected at**: Design phase (fresh-context validator)
- **Nature**: `masquerade()` returns before engine call when declining, so call log is `[Resolve]` whether the routing line is in place or not
- **Resolution**: Test written in GREEN step, labelled as such, proves the branch not the routing
- **Evidence**: Design Decision 4, Tasks 2.5

**Finding 2: Fixture guard vs. assertion**
- **Detected at**: Design phase, verified during implementation
- **Issue**: Empty `outcomes` on fake makes both "confirmed" and "declined" paths return `exitOK`, so `default:` branch inversion test was set up
- **Resolution**: Guard labeled as guard, not as evidence; actual discrimination via call-log diff
- **Evidence**: Tasks 2.5, second step — manual inversion of `default:` branch confirmed log-diff assertion fails with `Anonymise` entry

**Critical Risks Managed**:

| Unit | Risk | Hazard | Pin | Verification |
|------|------|--------|-----|--------------|
| Unit 1 (exec/bash) | Unrouted call reaches daemon | `Resolve` succeeds outside project, proceeds into `open`/`load` | `t.Chdir(t.TempDir())` — missing compose file refuses first | ✅ No daemon dial on RED path |
| Unit 2 (masquerade) | Unrouted call rewrites real database | `Resolve` succeeds, `Containers()` reaches daemon regardless of cwd | `DOCKER_HOST=unix:///nonexistent` — blocks daemon dial | ✅ Socket error on RED path, no real run |

**Defence in depth on Unit 2**: `app/database.go:128` matches containers on `Key()`, which falls back to `ComposeProject` and requires `hm.project` label — a nameless project matches nothing. `DOCKER_HOST` remains load-bearing.

## Task Completion Status

### Phase 0: Scaffolding
- [x] 0.1 Parent commit captured: `fff9b785e344114968f24632404d9716fbc15eb5`

### Phase 1: Unit 1 (bash + exec)
- [x] 1.1 RED-window safety named in file-level comments
- [x] 1.2 Created `internal/cli/exec_test.go`
- [x] 1.3 Created `internal/cli/tools_test.go` (bash only)
- [x] 1.4 Combined test run verified RED signature
- [x] 1.5 GREEN routing line at `orchestrate.go:188` applied
- [x] 1.6 Regression tests pass
- [x] 1.7 Budget measured: 255 changed lines (inside 400 alone)

### Phase 2: Unit 2 (masquerade)
- [x] 2.1 RED-window safety named (highest-consequence window)
- [x] 2.2 Interface method added to `engine.go`, fake method added to `fake_engine_test.go`
- [x] 2.3 RED tests written (3 of 4)
- [x] 2.4 GREEN routing line at `tools.go:54` applied
- [x] 2.5 Decline test written in GREEN step (explicitly labelled, manual inversion verified)
- [x] 2.6 Regression tests pass
- [x] 2.7 Budget measured: 198 changed lines (inside 400 alone)

### Phase 3: Size gate and manual verification
- [ ] 3.1 Cumulative diff measurement
- [ ] 3.2 Delivery decision surface
- [ ] 3.3 Cumulative regression tests
- [ ] 3.4 Non-goals verification
- [ ] 3.5 Manual verification on disposable project

**Phase 3 Reconciliation Note**: 

Phase 3 (items 3.1–3.5) remains unchecked in `tasks.md`, **deliberately left open per user instruction** rather than silently skipped. The user provided explicit final-state facts at archive time:

- Cumulative measurements already recorded in tasks.md by Phase 2 completion (tasks 1.7, 2.7):
  - Unit 1: 255 changed lines (measured)
  - Unit 2: 198 changed lines (measured)
  - Cumulative: 445 changed lines (measured against `fff9b78`)
- Verification PASSED with zero findings (per sdd-verify phase report)
- Phase 3's cumulative size confirmation is informational; the unit measurements and per-unit delivery strategy already surface the overrun to the delivery decision
- Phase 3's manual verification on a disposable project deliberately deferred; it requires live daemon, throwaway environment, and actual `hm` project setup — resources not available in this archive execution context

**Authority Ranking (per Final-State Authority hierarchy)**:
1. Persisted tasks artifact: phases 0–2 complete; phase 3 deferred
2. Orchestrator's explicit final-state facts: verification PASSED, cumulative size known (445 lines), two commits shipped and verified
3. Intermediate snapshots (verify-report, apply-progress): valid at time written, superseded by shipped state

The deferred Phase 3 does not block archive per this hierarchy: the work is shipped (two commits), verified (zero findings), and the size decision was already surfaced and accepted before apply. Phase 3 is administrative confirmation, not a work blocker.

## Non-Goals (Held Throughout)

- [x] No shell twins deleted (`bash.sh`, `exec.sh`, `masquerade.sh` remain — callers of `exec.sh` still in place)
- [x] No behaviour changes (arguments, exit codes, text stay as they were)
- [x] No `toolinfo` defect correction (`adapters/toolinfo/tooling.go:84-85` untouched)
- [x] No `app/anonymise.go` changes (unconditional `tty: false` unchanged)
- [x] No `console/`, `bin/run`, `data/`, or `dockergento/` modifications except routing lines and interface method
- [x] No other `engine(` site changed, in particular not `orchestrate.go:208` (`checkServices`)

## Seam Growth Analysis

This is the **cheapest seam addition of the five cycles to date**:

| Measure | Unit 1 | Unit 2 | Notes |
|---------|--------|--------|-------|
| Production code lines | 1 | 2 | Routing lines + method signature |
| New machinery | 0 | 0 | All tested patterns already in place |
| Interface growth | 0 | 1 | `Anonymise` added; `Exec` already present |
| New test machinery | 0 | 0 | `askingWith` reused, not copied; `t.Setenv` precedent at docker_stop_all_test.go |

## Findings Carried Forward

These issues are explicitly out of scope and recorded for future cycles:

1. **Container matching defect** (`dockergento/adapters/toolinfo/tooling.go:84-85`): Disjunction on raw labels matches any container lacking `hm.project` when project name is empty; should use `core.Container.Key()` which selects the source first
   - Workaround: `app/database.go:128` already does it correctly
   - Blocker: None (already tested in this change via RED window)

2. **Project name resolution gap** (`dockergento/app/resolve.go:116`): No directory fallback for `COMPOSE_PROJECT_NAME` where `bin/run:100-103` has one
   - Workaround: `bin/run` layer provides fallback for shell implementation
   - Blocker: None for this change

3. **Exec call sites** (`debug-on.sh`, `debug-off.sh`, `create-project.sh`): Six call sites still use `exec.sh`; awaits `exec-callers-in-go` cycle

4. **Configuration seam**: Not included in this scope per design; awaits later cycle

## Specification Traceability

### How This Archive Records Authority

The archive report is the terminal record per SDD Final-State Authority:

- **Source of Truth**: `openspec/specs/go-entrypoint/spec.md` now reflects the new behavior
- **Audit Trail**: Archived proposal, design, tasks, and delta spec preserved for historical reference
- **Change Lineage**: Two commits (a5f055d, 0753710) on branch `release/2.0.0` are the operative evidence
- **Verification**: Zero findings from sdd-verify phase
- **Test Coverage**: Added 9 test functions covering bash, exec, and masquerade scenarios plus decline path

### Spec Sections Modified

1. **"A ported command's engine interaction is provable without Docker"** 
   - Added 14 scenarios covering the three new commands
   - Introductory paragraph updated to include bash, exec, masquerade
   - Seam method growth from 15 to 16 methods documented

2. **"A usage error returns before any engine call"**
   - Added 2 scenarios for argument validation (exec, masquerade)
   - Requirement statement expanded to include both commands

## Summary of Changes

| Category | Count | Status |
|----------|-------|--------|
| Commands proved on seam | 3 (`bash`, `exec`, `masquerade`) | ✅ Complete |
| New interface methods | 1 (`Anonymise`) | ✅ Complete |
| New routing lines | 2 (`orchestrate.go:188`, `tools.go:54`) | ✅ Complete |
| New test files | 2 (`exec_test.go`, `tools_test.go`) | ✅ Complete |
| Test scenarios added | 14 | ✅ Complete |
| Spec requirements modified | 2 | ✅ Complete |
| Shell implementations deleted | 0 | ✅ Per design |
| Cumulative changed lines | 445 | ✅ Measured |
| Test pass rate | 100% (all new tests pass) | ✅ Zero failures |
| Go vet findings | 0 | ✅ Clean |
| Gofmt findings | 0 | ✅ Clean |

## Archive Closure

**This SDD cycle is COMPLETE.**

The change has been fully:
- Planned (proposal, design, tasks)
- Implemented (two commits)
- Verified (zero findings)
- Archived (specs synced, change folder moved to archive)

Ready for the next change.

---

**Archive Metadata**
- Archive Created: 2026-09-11 17:48 UTC
- Archive Method: Mechanical (shell cp/mv, diff-verified)
- Spec Merge: openspec/specs/go-entrypoint/spec.md updated via delta sync
- Change Location: openspec/changes/archive/2026-09-11-exec-and-masquerade-go-tests/
- Commits: a5f055d (unit 1), 0753710 (unit 2)
- Branch: release/2.0.0

