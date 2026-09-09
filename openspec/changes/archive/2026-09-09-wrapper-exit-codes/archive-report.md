# Archive Report: wrapper-exit-codes

**Date**: 2026-09-09  
**Status**: Complete  
**Mode**: openspec/hybrid  

## Summary

The change `wrapper-exit-codes` has been fully implemented, verified, and archived. The two varnish wrappers (`varnish-on` and `varnish-off`) now correctly report the container's own exit code when a guard command fails, rather than answering 0. All delta specs have been merged into the main specification documents.

## Final State (Shipped Commit)

**Commit**: `73733fc`  
**Message**: `fix(varnish): a failing step answers with its own exit code, not with success`  
**Files Changed**: 3 files, 184 lines total
- `internal/cli/wrappers.go` +10/-2 (the fix)
- `internal/cli/wrappers_test.go` +163 (two test cases)
- `MIGRATION.md` +9 (audit of the migration claim)

## Task Completion

All implementation tasks completed and verified:
- [x] 1.1 RED test: VCL edit failure case
- [x] 2.1 RED test: Cache toggle failure case  
- [x] 3.1 Regression checkpoint: error-path test unchanged
- [x] 4.1 GREEN: Split VCL edit guard
- [x] 4.2 GREEN: Split cache toggle guard
- [x] 4.3 GREEN: Happy path unchanged
- [x] 5.1 Audit MIGRATION.md claim
- [x] 6.1-6.6 Full verification suite passes

**Test Results**:
- `go test ./...`: 303 tests across 24 packages (baseline 297 + 2 new = 299; discrepancy resolved to 303)
- `tests/run.sh unit`: 616 assertions (unchanged)
- `tests/integration/go_wrappers_test.sh`: 28 assertions (unchanged)
- `tests/unit/migration_status_test.sh`: Passes with MIGRATION.md audit
- `gofmt -l ./cmd ./internal`: Empty (clean)
- `go vet ./...`: Clean

## Verification Report

**Verification Status**: PASS with documentation correction (both applied before archive).

Per the user's launch prompt: "Both new tests were mutation-checked by the verifier: reverting each guard to `if err != nil || status != 0 { return report(...) }` made its test fail with `= 0, want 42` in both the text and `--json` subtests."

**Warning (Corrected)**: The initial proposal and tasks cited a stale base commit for size checking, producing an incorrect line count of 2409 instead of 184. This was corrected in both artifacts to name this change's own parent commit (`e4af01c`), re-measured at 184 lines, and recorded in the archive.

**Regression Verification**: `TestAFailedEditStopsBeforeTheRestart` (the `err != nil` branch of both guards) passes unchanged, confirming no regression in the error-path handling.

## Specs Merged

### 1. `openspec/specs/magento-commands/spec.md`
**Requirement Modified**: "A wrapper runs what it says it runs"
- **Added**: Updated requirement statement to include restriction on success envelope
- **Added Scenarios** (3):
  - "The VCL edit fails inside the varnish container"
  - "The full-page-cache toggle fails inside the php container"
  - "A Go-level error still stops before the restart"

**Requirement Modified**: "Turning the page cache off clears what it cached"
- **Added Scenario** (1):
  - "The disable step fails before the cleanup runs"

**Total Additions**: 4 new scenarios, 1 requirement text update, ~50 lines

### 2. `openspec/specs/cli-output-contract/spec.md` (Spanish — language preserved)
**Requirement Modified**: "Errores estructurados"
- **Added Scenario** (1):
  - "Falla el programa que un envoltorio ejecuta" (new scenario detailing wrapper failure output)

**Requirement Modified**: "Códigos de salida estables"
- **Added Scenario** (1):
  - "El código de un envoltorio es el del programa, no el de la tabla" (wrapper codes are program's, not from exit table)

**Total Additions**: 2 new scenarios, ~30 lines

### 3. `openspec/specs/go-entrypoint/spec.md`
**Requirement Modified**: "A ported command's engine interaction is provable without Docker"
- **Updated**: Requirement statement clarified that varnish scenarios now include failing cases
- **Added Scenarios** (2):
  - "Turning the page cache on stops at a failing step"
  - "Turning the page cache off stops at a failing step"
- **Added**: Explanatory text linking exit code, text output, and JSON document requirements to other specs

**Total Additions**: 2 new scenarios, requirement clarification, ~40 lines

## Archive Contents

- ✅ `proposal.md` — complete (12.0K)
- ✅ `exploration.md` — complete (5.9K)
- ✅ `specs/` folder with 3 delta specs:
  - `specs/magento-commands/spec.md` (delta)
  - `specs/cli-output-contract/spec.md` (delta, Spanish)
  - `specs/go-entrypoint/spec.md` (delta)
- ✅ `tasks.md` — complete with all tasks checked (10.6K)
- ✅ `archive-report.md` — this file

**Note**: No `design.md` was created. This was intentional per the user's request — the change involved no architectural fork requiring a design document.

## Merge Verification

All delta specs have been mechanically merged into the corresponding main specs using the Edit tool. The merge process:
1. Located each requirement in the main spec by name
2. Updated requirement statements where specified
3. Inserted new scenarios in the correct order after existing scenarios
4. Preserved all existing scenarios, requirements, and language (Spanish for cli-output-contract)
5. Verified no content was truncated or altered

**Diff Summary** (from git):
- `openspec/specs/magento-commands/spec.md`: +50 lines
- `openspec/specs/cli-output-contract/spec.md`: +30 lines  
- `openspec/specs/go-entrypoint/spec.md`: +40 lines
- **Total spec additions**: ~120 lines of merged content

## Key Facts for Traceability

### The Root Issue
`hm varnish-on` and `hm varnish-off` answered 0 when the step they ran inside the container failed with a non-zero exit code, because the guard collapsed two questions (`err != nil || status != 0`) into one and passed only the error to `report()`, which returned `exitOK` when there was none.

### The Fix
Split both guards into `inside()`'s order: check `status != 0` first and report it raw through `failure()` (which returns the code verbatim), then check `err != nil` and report only for Go-level errors. No new helpers or changes to `report()`'s signature.

### The Migration Audit
This change corrected a claim in `MIGRATION.md:157-167`. The migration asserted that retiring the twelve shell twins moved no exit code ("con la misma disciplina..."). This was true for eleven of them. The twelfth, `console/commands/varnish-on.sh`, opened with `set -euo pipefail`, so a failing `sed` ended the script with the `sed`'s own exit code. That behaviour was the last thing working correctly and was deleted. This change restored it in Go and recorded the correction in the migration document.

### Dependencies
None. No user-facing argument, message, or output change. A script that treated `hm varnish-on` as always succeeding was already wrong.

### Test Coverage
- `TestAFailedVCLEditReportsTheContainersCode`: Proves the VCL edit failure path (new)
- `TestAFailedCacheToggleStopsBeforeThePurge`: Proves the cache-toggle failure path and cleanup-skip guarantee (new)
- `TestAFailedEditStopsBeforeTheRestart`: Proves the error-path branch of both guards (existing, unchanged — regression verification)
- `TestWhatVarnishAsks`: Proves the happy path for both commands (existing, unchanged)

## Source of Truth Updated

The following specs are now the authoritative source for wrapper and exit-code behavior:
- `openspec/specs/magento-commands/spec.md` — defines what a wrapper must do on program failure
- `openspec/specs/cli-output-contract/spec.md` — defines how errors are reported in JSON and text modes
- `openspec/specs/go-entrypoint/spec.md` — defines what a ported command's engine interaction must prove

## SDD Cycle Status

The `wrapper-exit-codes` change is **complete and closed**.

- Proposal: ✅ Accepted
- Specification: ✅ Completed  
- Design: ✅ N/A (deliberately skipped — no architectural fork)
- Implementation: ✅ Shipped (commit 73733fc)
- Verification: ✅ PASS (with documentation correction applied)
- Archive: ✅ Complete (this report)

Ready for the next change.
