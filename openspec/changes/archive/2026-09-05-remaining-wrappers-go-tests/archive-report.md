# Archive Report: remaining-wrappers-go-tests

**Change**: remaining-wrappers-go-tests  
**Archived**: 2026-09-05  
**Artifact Store**: hybrid  
**Archive Path**: `openspec/changes/archive/2026-09-05-remaining-wrappers-go-tests/`

## Executive Summary

The change "remaining-wrappers-go-tests" has been fully planned, implemented, verified, and archived. All 13 implementation tasks are complete. Two commits were delivered on release/2.0.0: `aba8cd6` (what the one-container commands ask of the engine) and `5e3b1af` (what mysqldump asks of the engine). The delta spec has been merged into `openspec/specs/go-entrypoint/spec.md`, adding 6 new scenarios to the engine-interaction requirement and 1 new scenario to the usage-error requirement, for a total of 13/13 scenarios compliant.

## Artifact Inventory

| Artifact | Status | Location |
|----------|--------|----------|
| proposal.md | ✓ Archived | `openspec/changes/archive/2026-09-05-remaining-wrappers-go-tests/proposal.md` |
| specs/go-entrypoint/spec.md | ✓ Archived (delta) | `openspec/changes/archive/2026-09-05-remaining-wrappers-go-tests/specs/go-entrypoint/spec.md` |
| design.md | ✓ Archived | `openspec/changes/archive/2026-09-05-remaining-wrappers-go-tests/design.md` |
| tasks.md | ✓ Archived | `openspec/changes/archive/2026-09-05-remaining-wrappers-go-tests/tasks.md` |
| apply-progress.md | ✓ Archived | `openspec/changes/archive/2026-09-05-remaining-wrappers-go-tests/apply-progress.md` |
| verify-report.md | ✓ Archived | `openspec/changes/archive/2026-09-05-remaining-wrappers-go-tests/verify-report.md` |

## Specification Merge

### Main Spec Updated

**File**: `openspec/specs/go-entrypoint/spec.md`

**Requirements Modified**: 2

1. **"A ported command's engine interaction is provable without Docker"**
   - Previous: 5 scenarios (copy-to-container, copy-to-container --all, copy-from-container, varnish-on, varnish-off)
   - Added: 6 new scenarios (clearing generated code/purge, running npm, running n98-magerun, running unit suite, running integration suite, writing database dump/mysqldump)
   - Final: 11 scenarios total (all carried scenarios preserved, 6 new scenarios appended)

2. **"A usage error returns before any engine call"**
   - Previous: 1 scenario (no path given for copy-to/copy-from)
   - Updated body: now covers `copy-to-container`, `copy-from-container`, AND `mysqldump`
   - Added: 1 new scenario (mysqldump with no path)
   - Final: 2 scenarios total

**Total Scenarios in Delta Spec**: 13 (11 in requirement 1 + 2 in requirement 2)  
**All Scenarios Status**: ✓ Compliant per verify-report (13/13 passing)

## Task Completion Gate: PASS

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 1 (one-container commands) | 6/6 tasks complete | 1.1–1.6 checked; commit `aba8cd6` delivered |
| Phase 2 (mysqldump) | 6/6 tasks complete | 2.1–2.6 checked; task 2.3 explicitly marked deferred (version moved to follow-up); commit `5e3b1af` delivered |
| Phase 3 (manual verification) | 1/1 task complete | 3.1 executed on rabatrepo; refusal path and purge parity confirmed |
| **TOTAL** | **13/13** | All implementation tasks checked; no stale unchecked tasks; no blockers |

**Source of Truth**: `openspec/changes/archive/2026-09-05-remaining-wrappers-go-tests/tasks.md` shows all checkboxes marked.

## Verification Report Summary

**Verdict**: PASS WITH WARNINGS (per verify-report observation, issued at verification time)

| Metric | Value |
|--------|-------|
| Requirements compliant | 2/2 (100%) |
| Scenarios compliant | 13/13 (100%) |
| Critical findings | 0 |
| Warnings | 1 (manual deletion not observed on rabatrepo due to empty generated directories; covered by untouched integration suite) |
| Suggestions | 2 (size estimate ratio learning for table tests; prose cleanup on requirement 2 wording) |
| Build | PASS (`go build ./...` succeeds) |
| Tests (focused) | PASS (44 tests in `internal/cli`, +16 from baseline of 28) |
| Tests (full) | PASS (204 tests in 22 packages, +16 from baseline of 188) |
| Tests (integration) | PASS (612 assertions in `tests/run.sh unit`, unmodified Bash parity suite) |
| Code quality | PASS (`gofmt` empty, `go vet` clean) |

Per verify-report (obs. #217 per orchestrator context), all 13 scenarios in the reconciled delta are compliant, all tasks are complete, and no CRITICAL blockers prevent archive.

## Delivery Record

| Commit | Message | Files | Scope | Verification |
|--------|---------|-------|-------|--------------|
| aba8cd6 | test(cli): what the one-container commands ask of the engine | `internal/cli/engine.go`, `internal/cli/fake_engine_test.go`, `internal/cli/inside_test.go` (new), `internal/cli/wrappers.go` | Property interface method; inside tests for purge/npm/n98-magerun/test-unit/test-integration | Isolated rebuild: `go build ./...` ok, `go test ./internal/cli -short` 39 passed |
| 5e3b1af | test(cli): what mysqldump asks of the engine | `internal/cli/engine.go`, `internal/cli/fake_engine_test.go`, `internal/cli/wrappers.go`, `internal/cli/wrappers_test.go` | Dump interface method; mysqldump tests | Full suite: `go test ./... -short` 204 passed, `tests/run.sh unit` 612 passed |

**Total changed lines (both commits)**: 359 (5 files changed, 356 insertions+, 3 deletions−)  
**Budget**: 400-line default (Medium risk, no exception)  
**Status**: Under budget ✓

## Deferred Scope (Reconciled)

**Version and Installed method** were deferred to the follow-up change `down-and-set-host-go-tests` per the hard size gate in task 1.5:

- Reason: Commit 1 measured 247 changed lines (over the 220-line gate) because `inside_test.go` came out to 227 lines (six exhaustive table cases plus literate comments), exceeding the ~150-line estimate
- Cut order applied: `version` first (self-contained), then `mysqldump` only if still needed (not needed after version cut)
- Status: Reflected in reconciled spec (`specs/go-entrypoint/spec.md` Note), proposal (`proposal.md` cut record dated 2026-09-05), and tasks (`task 2.3` marked deferred)
- Result: Final size 359 lines, safely under 400 budget; `inside` family (purge/npm/magerun/test-unit/test-integration) untouched; no comments deleted

## Engram Artifact References

The following Engram observations were consulted and are recorded for traceability:

- Exploration (obs. #160): initial discovery and scope analysis
- Proposal (obs. #166): change rationale, scope, and approach
- Spec (obs. #172): delta spec with 13 scenarios
- Design (obs. #176): design decisions and implementation approach
- Tasks (obs. #195): 13 implementation tasks with size gate
- Apply-progress (obs. #203): RED→GREEN cycle evidence and commit delivery record
- Verify-report (obs. #217): PASS WITH WARNINGS, 13/13 scenarios compliant, 0 CRITICAL

## Files Under openspec/ Changed by Archive Operation

- ✓ Merged into `openspec/specs/go-entrypoint/spec.md` (2 requirements modified, 13 total scenarios)
- ✓ Moved `openspec/changes/remaining-wrappers-go-tests/` → `openspec/changes/archive/2026-09-05-remaining-wrappers-go-tests/`
- ✓ Active changes directory no longer contains this change

## Mechanical Copy Contract Verification

- Source snapshot captured before move operation ✓
- Move executed via shell (`mv` fallback after git mv unavailable) ✓
- Post-move `diff -r` readback performed (verbatim output below) ✓
- Archive-report excluded from diff comparison as per contract ✓
- Empty diff is the only passing evidence ✓

**Diff-r readback output** (source snapshot vs archive, archive-report excluded):
```
(empty — no differences detected)
```

## Closure

**SDD Cycle**: Complete  
**Archive Status**: Closed  
**Ready for Next Change**: Yes  
**Recommended Next Phase**: `down-and-set-host-go-tests` (deferred scope: version, set-host, down)

The change has been fully delivered, verified, and archived. The source of truth (`openspec/specs/go-entrypoint/spec.md`) has been updated with the merged delta spec and now accurately reflects the final state of this change's contribution to the go-entrypoint component.
