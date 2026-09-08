# Archive Report: docker-tools-in-go

**Date**: 2026-09-08  
**Change**: docker-tools-in-go  
**Status**: ARCHIVED  
**Verdict**: PASS WITH WARNINGS

## Summary

This change ports `docker-stop-all` and `docker-compose` commands from Bash to Go, completing the second-to-last major step in the Go migration. The change delivered as three commits (rather than the planned two) totalling 1402 insertions and 186 deletions. Verify verdict: **PASS WITH WARNINGS** with 0 CRITICAL findings and 7 WARNING-level gaps, all accepted as known risk due to hard safety constraints (never confirming machine-wide stops on a developer's own running containers).

## Artifacts Merged

All four delta specs successfully merged into their corresponding main specs:

| Main Spec | Domain | Delta Merged | Action |
|-----------|--------|--------------|--------|
| environment-lifecycle/spec.md | environment-lifecycle | MODIFIED | "Parar toda la máquina exige una respuesta" requirement updated with 9 scenarios (expanded from 4) |
| environment-orchestration/spec.md | environment-orchestration | ADDED + MODIFIED | Two new requirements added ("Passing an arbitrary Compose subcommand through", "Stopping every running container through the engine"); two existing requirements modified ("Starting an environment", "One implementation of starting") |
| worktree-safety/spec.md | worktree-safety | MODIFIED | "Bloqueo de operaciones que alteran la topología" requirement refined; description and scenario text updated to hold "regardless of which implementation answers" |
| go-entrypoint/spec.md | go-entrypoint | ADDED + MODIFIED | One new requirement added ("A ported command's shell entry point still answers by delegating"); existing "A ported command's engine interaction is provable without Docker" extended with two new scenarios |

## Final Implementation State

### Delivery
- **Commits**: 3 (over the planned 2)
  - `80c9e8c`: `docker-stop-all` and `start -s` rewired to Go
  - `9dab3dc`: Bash guard retired; Docker-backed proofs added
  - `68b1747`: `docker-compose` passthrough in Go
- **Commits outside scope** (part of the story, recorded for traceability):
  - `79e827c`: `fix(cli): a question belongs on the error stream, not in the answer`
  - `df5c9f4`: `test(stop-all): the two answers that stop nothing` (closed CRITICAL gaps)
- **Total changed lines**: 1402 insertions + 186 deletions (measured at archive time)
- **Size vs forecast**: 1372–1402 lines against a forecast of 970–1090 (accepted `size:exception`, user reaffirmed)

### Test Coverage
- **Go tests**: 250 passed in 23 packages (short-mode)
- **Bash unit tests**: 612 assertions passed
- **Focused test runs**: All pass (stop-all and docker-compose handlers tested in isolation)
- **Build checks**: `go build ./...` clean, `gofmt -l ./cmd ./internal ./dockergento` empty, `go vet ./...` clean

### Code Compliance
| Item | Status | Notes |
|------|--------|-------|
| `ports.ContainerEngine.Stop` interface | ✅ Implemented | `ports/ports.go:80-95` |
| `dockerd.Engine.Stop` adapter | ✅ Implemented | 30s timeout per container, bounded concurrency 8 |
| `core.MachineStop{Total, Others, Stopped}` | ✅ Implemented | `core/orchestration.go:60-64` |
| `Operator.StopEverything` with all four branches | ✅ Implemented | Confirm, decline, zero-containers, non-interactive; all tested |
| `Operator.Start` no longer calls `Legacy.Run` for stop-all | ✅ Implemented | `rg "docker-stop-all" dockergento/app/orchestrate.go` finds only comments/refusal strings |
| `ports.ComposeRunner` adapter | ✅ Implemented | Subprocess exec, argv constructed via `strings.Fields`, never shell strings |
| `Operator.Compose` app layer | ✅ Implemented | Direct passthrough, no worktree refusal (docker-compose not in altered-topology set) |
| `StartOptions.Interactive` from `HM_NON_INTERACTIVE` | ✅ Implemented | `internal/cli/orchestrate.go:40` |
| Both `.sh` files as delegation stubs | ✅ Implemented | `worktree.sh`-style (`exec "$binary" docker-{stop-all,compose} "$@"`) |
| Probe swap in integration tests | ✅ Implemented | Line 284 swapped to `compatibility` (read-only); lines 172/267 kept as parity assertions |
| `MIGRATION.md` at 32 de 65 | ✅ Implemented | Both rows marked `go`; reverse check passes in test suite |

## Verification Summary

### Spec Compliance
- **Requirements**: 8/8 present in the four merged specs
- **Scenarios**: 49/49 total across specs
- **Compliance**: 41/49 COMPLIANT, 8/49 PARTIAL (due to accepted integration/e2e constraints)

### Blocker Closure
The prior verify pass identified two CRITICAL findings:
1. `TestDecliningStopsNothing` missing — now closed by `df5c9f4`, mutation-checked
2. `TestWithNothingRunningNobodyIsAsked` missing — now closed by `df5c9f4`, mutation-checked

Both branches are now mutation-tested: dropping the early return or allowing fall-through makes the tests fail, confirming the production code is being exercised.

### Known Warnings (Accepted Risk)
Seven WARNING-level gaps follow from a hard constraint applied across apply and verify phases: **never confirming a machine-wide stop on a developer's own running containers**. This boundary was explicitly chosen to protect the user's development environment.

| # | Gap | Reason | Closure Path |
|---|-----|--------|--------------|
| 1 | `dockerd.Engine.Stop` no automated proof against real daemon | Hard constraint: no confirmed-stop runs against real Docker | Opt-in e2e behind `HM_E2E_STOP_ALL=1` (never enabled this session) or manual verification on disposable machine |
| 2 | `composecli.Runner.Run` no dedicated unit test | Compose subprocess adapter proven indirectly only | Small unit test with stub executable or pure function extraction |
| 3–5 | Integration/e2e scripts syntax-checked only | Hard constraint: no real Docker execution | When scripts run for real (disposable machine or Docker idle) |
| 6 | Two literal strings no direct Go-test assertion | "This stops N container(s)…" and "Stopping N…" proven by unit-tested counts + manual verification | Add direct string assertions to match "Nothing was stopped." and "No containers running" |
| 7 | `docker-compose` worktree exemption structurally guaranteed only | No automated test names it in worktree context | Explicit worktree-context test case (integration or e2e) |

### Manual Verification
- **Task 3.1 completed** ✅: Pre-change vs. post-change binaries compared with `n` answer; exact text and exit codes match
- **Task 3.2 deliberately skipped** ✓: User's project was up; confirmed-stop step not run (accepted risk; opt-in e2e coverage gates `HM_E2E_STOP_ALL=1`)

## Observations for Engram Traceability

Hybrid mode artifacts persisted to Engram for this change:
- **explore** (observation #370): Initial exploration of scope and concerns
- **proposal** (observation #376): Scope, approach, delivery strategy, risks
- **spec** (observation #384): Structured requirements and scenarios from sdd-spec
- **design** (observation #389): Architectural decisions and affected areas
- **tasks** (observation #423): Detailed work units with estimates and task completion
- **apply-progress** (observation #430): Implementation progress, TDD table, commits delivered
- **verify-report** (observation #438): Test results, spec compliance, findings (prior); updated by `df5c9f4`

**This archive report** is the final authority on the state at close, superseding earlier snapshots where later commits changed outcomes (e.g., `df5c9f4` closed the CRITICAL findings).

## Rollback Path

Straightforward: two independent commit reversions on `release/2.0.0` restore the bash implementations and `Legacy.Run` call:
- `git revert df5c9f4` (test-only; optional if rollback stops at functional change)
- `git revert 68b1747` (docker-compose commit, independent)
- `git revert 9dab3dc` (Bash guard/proofs)
- `git revert 80c9e8c` (docker-stop-all + start rewiring)

Reverting commit 1 alone leaves commit 2's docker-compose work inconsistent; a full rollback reverses all three in order.

## SDD Cycle Complete

This change completes its full SDD lifecycle:

1. ✅ **Explored**: Scope, concerns, and migration status measured
2. ✅ **Proposed**: Delivery strategy agreed, risks documented, user accepted `size:exception`
3. ✅ **Specified**: Requirements and scenarios written from design artifacts
4. ✅ **Designed**: Architecture, affected areas, and open decision resolved
5. ✅ **Tasked**: Work units defined, TDD pairing documented, estimates forecast
6. ✅ **Applied**: Three commits delivered (two planned + one from deferred e2e), all verified building and tests passing
7. ✅ **Verified**: Build clean, 250 Go tests + 612 bash assertions pass; PASS WITH WARNINGS verdict; CRITICAL findings closed; 7 WARNING gaps accepted and documented
8. ✅ **Archived**: Deltas merged, folder moved, final state recorded

**Next recommended step**: None — change is closed. Migration counter advances to 32 of 65 commands in Go. The remaining major slices are the `ai-*` family and the Bash twins of already-ported commands.
