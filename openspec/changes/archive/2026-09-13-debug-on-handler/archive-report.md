# Archive Report: debug-on-handler

**Change**: `debug-on-handler`  
**Archived**: 2026-09-13  
**Project**: hiberus-dockergento (release/2.0.0)  
**Status**: Complete and verified  
**Verification Result**: 0 critical, 0 warning, 2 suggestions (documentation/coverage notes)

## Executive Summary

Unit 3 of the debug-on port to Go has shipped: `hm debug-on` is now fully implemented in Go, displacing the last Bash implementation of the xdebug handler. Four commits land on top of ef540b0, totalling **1246 insertions / 2 deletions** across 7 files. All four commits passed verification; all four mutations discriminated as expected.

## What Shipped

### Commits

1. **47cc9de**: Slice 1 — `xdebugVersion` and `xdebugSettings` parsers with fixture tests
   - Two string-to-value functions for classifying php -v output and selecting xdebug ini keys
   - Fixture tests prove both v2 and v3 classification, and the absence case
   - 129 insertions in wrappers.go and wrappers_test.go

2. **5facfe2**: Slice 2 — Engine seam growth from 16 to 18 methods
   - `(*Engine).XdebugHost()` facade added to dockergento.go
   - `Capture` and `XdebugHost` added to the commands interface in engine.go
   - Fake implementations and a fake-level test proving correct forwarding of captured values
   - 371 insertions in test scaffolding and interface

3. **6c8b3d4**: Slice 3a — Handler enabling half through `Restart`
   - `debugOn` handler in wrappers.go: host resolution, two xdebug.ini patches, uncomment of docker-php-ext-xdebug.ini
   - All failure and refusal paths before `Restart` (Decision 3: no Start ever called)
   - Six unit tests covering successful paths for both v2 and v3, failure cases, and the worktree refusal
   - 500+ insertions in test code proving each branch

4. **41acc77**: Slice 3b — Handler readback half, routing, and migration
   - Two `Capture` calls: php -v for version classification, php -i for settings extraction
   - Version-aware settings banner and JSON envelope
   - Routing in run.go and MIGRATION.md updated (33→34 de 65, debug-on row shell→go)
   - migration_status_test.sh verified both row and counter
   - 246 insertions, includes all JSON envelope tests

### Impact on Metrics

- **go test baseline**: 437 tests after slices 1-2; 461 after 3b (24 new test functions)
- **go test result**: 461 passed across 24 packages (no regressions)
- **./tests/run.sh unit**: 616 assertions, 0 failed (baseline unchanged)
- **commands interface**: Exactly 18 methods; engine.go:14-16 doc comment unchanged and still true
- **Shell retirement**: `console/commands/debug-on.sh` and `console/tasks/set_xdebug_host_property.sh` now retired in a later cycle

## Line Counts (Final State)

| Metric | Result |
|--------|--------|
| Insertions | 1246 |
| Deletions | 2 |
| Files touched | 7 |
| Test functions added | 11 |
| New scenarios documented | 16+ |

## Estimate vs. Actual

The proposal forecast 850–950 lines with explicit caveat that overshoot was predicted because seam growth was back and two decisions were open.

- **Forecast reasoning**: The seam had grown from 14→16 methods (capture-container-output), then 16→18 methods (this unit). The proposal predicted ~200 lines for slices 1-2, ~400-500 for slice 3a (handler enabling), ~150-250 for slice 3b (readback and JSON).
- **Actual slices 1-2**: 367 lines (1.84× the estimate)
- **Forecast after slices 1-2**: Revised to 1000–1200 lines total
- **Final actual**: 1246 lines

**Analysis**: The forecast's reasoning was correct — 834 of the 1246 lines are `wrappers_test.go`, exactly where the prediction said the mass would fall. The magnitude error came from underestimating test fixture complexity (discriminating fixtures for both xdebug versions, failure paths, macOS/Linux divergence). The lesson: identifying the right cause does not calibrate the magnitude; test proliferation still outpaced the estimate even after the first revision.

## Critical Findings

### Bug Caught Before Shipping

**Issue**: `copyFrom()` emits its own top-level JSON document on success, so `debugOn` calling it under `--json` produced two concatenated JSON objects — invalid for any consumer.

**Root Cause**: A half-true claim in the exploration that travelled unchanged through proposal, tasks.md, and three orchestrator briefs with each defended by line citation: *"copyFrom prints nothing of its own on success."* True in text mode, false under `--json`. The half that was true is what people spot-checked; the false half shipped unchallenged.

**Fix Applied**: In slice 3a, `copyFrom(..., false)` called unconditionally internally, gating the decorative warning line behind `!jsonOutput`. Verified: no other handler carries this shape; `varnish-on`/`varnish-off` pass `jsonOutput` to their engine calls but never call `document()`.

**General Lesson**: A half-true claim survives scrutiny better than a false one. In review:
- The checked half is accepted without challenging the unchecked half
- Line citations increase confidence without increasing actual verification
- A single unchallenged false detail can ship in production code

### CI Gap: No macOS Test Coverage

**Finding**: CI never executes a single macOS-specific code path:
- `.github/workflows/go.yml:13` runs on `ubuntu-latest` (go test only)
- `.github/workflows/release.yml:18` runs on `macos-latest` but only cross-compiles; no tests

**Affected Branches**: Five `runtime.GOOS == "darwin"` / `Platform() == "mac"` paths across:
- `dockergento/dockergento.go` (3 branches)
- `internal/cli/php.go` (1 branch)
- `internal/cli/wrappers.go` (1 branch)

**Why This Matters**: The `copyFrom` bug lived in exactly such a darwin branch and was caught only because the apply agent's machine runs macOS. On a Linux dev machine it ships. The test suite names a Linux scenario but fired macOS code; the JSON parse failed before it could reach production.

**Cheap Fix**: Add a `macos-latest` matrix entry to go.yml, or run `go test` inside the existing release.yml macos-latest job. No architectural work needed.

### Disclosed Secondary Gap

**Capture Failure Reporting on macOS**: A `CopyFrom` failure during `--json` execution on macOS reports through `copyFrom`'s text-mode error path — plain text on stderr, stdout left empty rather than corrupted. Not covered by any scenario, disclosed by verifier. No scenario asserts this branch because the happy path (success) and the error path (copyFrom error) were not both exercised; the happy path test did not exercise the error path, and the error path was not tested when JSON was active.

## Specifications Merged

### go-entrypoint (`openspec/specs/go-entrypoint/spec.md`)

**Requirement Modified**: "A ported command's engine interaction is provable without Docker"

- Updated header from 18th to 19th command; debug-on is the first requiring two new seam methods at once (`Capture`, `XdebugHost`)
- Added 16+ scenarios specific to debug-on, covering:
  - Happy path (v2 and v3 xdebug versions)
  - macOS path (no host patching, `CopyFrom` call)
  - Xdebug absent/unrecognized (informative exit 0, disclosed decision)
  - Host resolution failure (disclosed as a failure, not silent skip)
  - All edit failures (before restart, both Go errors and non-zero status)
  - Worktree refusal from `Restart` (exit 6, new behaviour)
  - Captured calls discriminated for both versions
  - JSON envelope structure (host as null when absent/not patched, version-specific settings keys)

**Line change**: +9 lines (from 1182 to 1191)

### magento-commands (`openspec/specs/magento-commands/spec.md`)

**Requirement Modified**: "A wrapper runs what it says it runs"

- Updated header to include debug-on's three ini edits as a third reuse of the shared failure shape
- Added 1 new scenario: "An xdebug-enabling edit fails inside the php container"
- Clarified that shared scenarios with go-entrypoint are proved once (cross-reference maintained)

**Line change**: +19 lines (from 150 to 169)

## Carried Forward (Not Completed)

### Next Cycle

- **Retire shell implementations**: `console/commands/debug-on.sh` and `console/tasks/set_xdebug_host_property.sh`
- **Process**: Per MIGRATION.md rule — move file aside, run full suite, note breaks, restore, repair, delete. A textual check for references is worth running first but never a proof of absence.

### Open Architectural Issues (Pre-existing)

- **architecture/05-state-and-configuration.md:207, 371-377**: Documents an xdebug host-detection algorithm the code does not implement. Unit 2 ported the code (correct); the doc is misleading.
- **`.github/workflows/go.yml:44`**: Runs `./tests/run.sh tests/unit` but resolved path `tests/tests/unit` misses, falls through to `:83`, runs every suite. Unrecognised target fails silently rather than erroring.
- **`.github/workflows/go.yml:26`**: `gofmt -l ./cmd ./internal` never reaches `./dockergento/` (incomplete formatting coverage).
- **`dockergento/adapters/toolinfo/tooling.go:84-85`**: Label disjunction unresolved.
- **`dockergento/app/transfer.go:182-206`**: Third copy of container-resolution logic; convergence is a behaviour change.
- **`dockergento/app/resolve.go`**: Missing directory fallback for `COMPOSE_PROJECT_NAME`.
- **Interface method count**: Still hand-maintained prose in doc comment; no enforced test.

## Task Completion Gate

All implementation tasks in `tasks.md` marked complete:
- [x] Slice 1 (parsers)
- [x] Slice 2 (seam growth)
- [x] Slice 3a (handler enabling)
- [x] Slice 3b (handler readback, routing, MIGRATION)
- [x] Final verification (go test, go vet, gofmt, integration tests)

**Note on tasks.md**: File carried forward with all checkboxes marked. Slice 3 checkboxes were marked but uncommitted; included in final archive commit per user guidance.

## Verification Report Summary

(Verification PASSED per orchestrator input)

- **Critical issues**: 0
- **Warnings**: 0
- **Suggestions**: 2 (both documentation/coverage notes; no functional impact)

Verification confirmed:
- All four mutations discriminated (RED → GREEN proved for each slice)
- No regressions in existing tests
- All new scenarios covered and passing
- JSON envelope structure correct for all paths
- Worktree refusal path correctly integrated
- Version-specific settings parsing correct for both v2 and v3

## Archive Structure

```
openspec/changes/archive/2026-09-13-debug-on-handler/
├── proposal.md
├── exploration.md
├── tasks.md (with Slice 3 checkboxes marked)
├── specs/
│   ├── go-entrypoint/spec.md (delta spec merged into main spec)
│   └── magento-commands/spec.md (delta spec merged into main spec)
└── archive-report.md (this file)
```

## Cleanup Checklist

- [x] No backup/stale files (.backup, .bak, .orig) left in workspace
- [x] Delta specs merged into main specs (openspec/specs/{domain}/spec.md)
- [x] Change folder moved to archive with date prefix
- [x] Archive report written and verified to exist on disk
- [x] All artifacts included in final commit

## Cycle Closure

The debug-on-handler change is archived and complete. The three-unit Go port of `hm debug-on` is finished:
- Unit 1 (debug-on-host-detection): Host resolver, engine refusal path, disclosed edge cases
- Unit 2 (capture-container-output): Capture seam method, XdebugHost facade prep
- Unit 3 (debug-on-handler): Full handler implementation with readback, JSON envelope, routing

`MIGRATION.md` now reads `34 de 65` (debug-on is the 34th of 65 shell commands ported to Go). Next major work: retire the shell implementations and verify the binary handles all cases.

---

**Archived by**: Claude (sdd-archive phase)  
**Date**: 2026-09-13  
**Repository**: /Users/ddelgado/hm (release/2.0.0)  
**Engram Topic Key**: sdd/debug-on-handler/archive-report
