# Archive Report: sync-migration-status

**Date**: 2026-09-04  
**Change**: sync-migration-status  
**Mode**: Hybrid (OpenSpec + Engram)  
**Repository**: hiberus-dockergento  
**Project**: hiberus-dockergento  

## Executive Summary

The `sync-migration-status` change has been successfully implemented, verified, and archived. The change reconciles the migration documentation (`MIGRATION.md`) against the Go implementation (`internal/cli/run.go`), introducing a bidirectional verification test and correcting stale documentation claims about the implementation's scope. All 14 tasks completed, verification passed with informational warnings only, and no CRITICAL issues detected.

## Artifact Retrieval

This archive report records all source artifacts for traceability:

| Artifact | Source | Observation ID |
|----------|--------|---|
| Proposal | Engram `sdd/sync-migration-status/proposal` | #16 |
| Spec (delta, go-entrypoint) | Engram `sdd/sync-migration-status/spec` | #23 |
| Design | Engram `sdd/sync-migration-status/design` | #27 |
| Tasks | OpenSpec `openspec/changes/sync-migration-status/tasks.md` | #44 |
| Apply progress | Engram `sdd/sync-migration-status/apply-progress` | #48 |
| Verify report | Engram `sdd/sync-migration-status/verify-report` | #59 |

## Final State Authority

Per the skill's Final-State Authority hierarchy, this report reflects the state AT CLOSE:

1. **Delivered commits** (highest authority): 3 commits on `release/2.0.0`:
   - `1053545` `test(migration): every command the router answers, checked against the table`
   - `86a45ef` `docs: the implementation is Go and Bash, not only Bash`
   - `d39faac` `chore: ignore the local codegraph index` (unrelated, out of scope)

2. **Task completion** (authoritative via tasks artifact): All 14 implementation tasks complete, including commit tasks 1.7 and 2.6 (prepared by apply, committed by orchestrator as per design).

3. **Verification verdict** (per verify-report obs #59): `pass_with_warnings`
   - 0 CRITICAL findings
   - 2 WARNING (non-blocking):
     1. Two ADDED spec scenarios verified manually per project convention (`openspec/config.yaml:85`, `rules.tasks manual-verification`)
     2. Pre-existing `printf | grep -qx` race at `tests/unit/migration_status_test.sh` lines 34/42, out of scope (left untouched per design)

4. **Explicit final-state facts** (from launch prompt, outrank stale snapshots): Test counts and completion verified as delivered.

### Stale vs. Current Snapshot Claims

- **apply-progress.md** (obs #48, written during apply phase): Claims tasks 1.7, 2.6 as "Prepared but NOT committed — orchestrator settles." This is the apply phase's narrower completion criterion (apply does not run `git commit`). The tasks artifact (obs #44) correctly marks both checked (`[x]`), noting "orchestrator settles delivery." Both statements are consistent: apply prepared, orchestrator committed. The delivered commits (`1053545`, `86a45ef`) confirm both commit tasks completed. ✅

- **verify-report.md** (obs #59, written during verify phase): Documents intermediate state at verification time. Current state surpasses this: verify ran against the prepared commits; they now exist and have been delivered. No stale claims detected.

## Task Completion Gate

**Status**: PASSED ✅

All 14 tasks in `openspec/changes/archive/2026-09-03-sync-migration-status/tasks.md` are checked `[x]`:

**Phase 1: Commit 1 work unit** (6 tasks + commit)
- [x] 1.1 RED: reverse-check block added
- [x] 1.2 GREEN: 14 rows flipped `shell`→`go`
- [x] 1.3 Counter fixed: `16 de 65` → `30 de 65`
- [x] 1.4 `MIGRATION.md:187-191` prose corrected to cite `mirrorsVendor`
- [x] 1.5 `MIGRATION.md:282-283` description rewritten for both directions
- [x] 1.6 Full Docker-free suite green (612/612)
- [x] 1.7 Commit prepared (delivered as `1053545`)

**Phase 2: Commit 2 work unit** (5 tasks + commit)
- [x] 2.1 `CLAUDE.md:11` strangler-pattern sentence
- [x] 2.2 `architecture/02-cli-architecture.md:5` strangler-pattern sentence, diagram intact
- [x] 2.3 `openspec/config.yaml` absorbed as-is
- [x] 2.4 Cross-check: no absolute Bash claim in either file
- [x] 2.5 Full Docker-free suite green (612/612)
- [x] 2.6 Commit prepared (delivered as `86a45ef`)

**Phase 3: Manual verification** (1 task)
- [x] 3.1 Manual verification in real checkout (counter `30 de 65`, all 14 rows `go`, both prose fixes read consistently)

## Spec Merge

**Delta spec merged into main spec**: ✅ `openspec/specs/go-entrypoint/spec.md`

| Action | Requirement | Details |
|--------|-------------|---------|
| MODIFIED | "The state of the migration is written down and true" | Added reverse-direction guarantee; now checks that every routed command is tabled `go` (not just the forward direction). Added 4 new scenarios covering the reverse check, partial-command exceptions, counter agreement, and runtime-code isolation. |
| ADDED | "Documentation does not overstate what remains unported" | New requirement ensuring docs describe the strangler pattern accurately and never claim 100% Bash implementation after Go commands are wired. Added 2 new scenarios. |

Both the MODIFIED and ADDED requirements now appear in `openspec/specs/go-entrypoint/spec.md` with all scenarios intact, following the main spec's markdown formatting exactly.

## Verification Recap

Per `verify-report.md` (obs #59), delivered as PASS WITH WARNINGS:

| Metric | Result |
|--------|--------|
| Tasks complete | 14/14 ✅ |
| Build | `go build ./...` → exit 0 ✅ |
| Unit tests | `bash tests/unit/migration_status_test.sh` → RESULT 12 0 ✅ |
| Full Docker-free suite | `tests/run.sh unit` → 612 assertions passed ✅ |
| Go regression | `go test ./... -short` → 172 passed in 22 packages ✅ |
| Spec requirements | 2/2 requirements, 9/9 scenarios ✅ |
| CRITICAL findings | 0 |
| WARNING findings | 2 (non-blocking, documented above) |

**Scope verification** (per design's threat matrix: "not applicable"):
- No files under `console/`, `bin/run`, `internal/`, `dockergento/` modified ✅
- Only test, docs, and config files changed ✅
- No runtime command path touched ✅

## Archive Contents Verification

**Mechanical copy verification** (per Mechanical Copy Contract):

Pre-move snapshot created, archive move performed via `git mv` → fallback `mv`, post-move `diff -r` run:

```
openspec/changes/archive/2026-09-03-sync-migration-status/ → snapshot identical
(empty diff = pass; archive-report.md is additive-only, excluded from comparison)
```

**Archive contents**:
- ✅ `proposal.md` — original proposal artifact
- ✅ `specs/go-entrypoint/spec.md` — original delta spec
- ✅ `design.md` — original design artifact
- ✅ `tasks.md` — original tasks artifact (14/14 complete)
- ✅ `apply-progress.md` — original apply progress snapshot
- ✅ `verify-report.md` — original verify report snapshot
- ✅ `exploration.md` — original exploration artifact
- ✅ `archive-report.md` — this archive report (additive-only)

**Active changes directory**: `openspec/changes/sync-migration-status/` no longer exists ✅

## Source of Truth Updated

The main spec `openspec/specs/go-entrypoint/spec.md` has been updated with:
- The reverse-direction verification guarantee (MODIFIED requirement text)
- 4 new scenarios for reverse check, partial commands, counter agreement, and runtime isolation (MODIFIED requirement)
- 2 new scenarios for strangler pattern documentation and no-absolute-claim verification (ADDED requirement)

This spec is now the single source of truth for the go-entrypoint capability in this repository.

## SDD Cycle Complete

This change has been fully:
- ✅ Proposed (`sync-migration-status` proposal with clear intent and approach)
- ✅ Specified (delta spec defining bidirectional verification and documentation accuracy)
- ✅ Designed (two-phase commit strategy, reverse-check implementation details, prose corrections)
- ✅ Tasked (14 work units with RED→GREEN TDD cycle and manual verification)
- ✅ Implemented (orchestrator delivered two commits, one pre-existing tooling commit)
- ✅ Verified (all tests green, zero CRITICAL findings, two non-blocking WARNINGs)
- ✅ Archived (specs merged, change folder moved, audit trail complete)

## Key Decisions & Learnings

1. **Bidirectional verification necessity**: The reverse check revealed 14 commands fully wired in Go but still tabled as `shell`. A test pointing only at the document → implementation direction would never catch such drift. The reverse direction is essential for audit integrity.

2. **SIGPIPE/pipefail race discovery and mitigation**: The here-string fix (`grep -qx ... <<< ...` instead of `printf ... | grep -qx ...`) for the new reverse-check's membership test was a sound engineering deviation, not a design violation. Correctly scoped (only new code) and disclosed. The pre-existing idiom at lines 34/42 remains untouched, as out of scope, but is flagged for follow-up.

3. **Manual verification for prose requirements**: Two spec scenarios ("Describing how a command reaches its implementation", "No absolute claim remains") are inherently not mechanically testable but are explicitly covered by the project's manual-verification policy (`openspec/config.yaml:85`). Both were manually verified by apply and independently re-verified during verification phase.

4. **Documentation as audit trail**: Keeping `MIGRATION.md` and docs like `CLAUDE.md`/`architecture/02-cli-architecture.md` synchronized with code reality is not busywork — it is essential for the strangler pattern to remain visible to future maintainers. A claim that "the implementation is 100% Bash" after Go commands exist is a hazard signal that the architecture has drifted from its documentation.

## Risks & Follow-ups

| Item | Type | Severity | Status |
|------|------|----------|--------|
| Pre-existing `printf \| grep -qx` race at `tests/unit/migration_status_test.sh:34,42` | Latent hazard | Medium | Follow-up (out of scope for this change; pre-existing, untouched) |
| Stale comment at `internal/cli/php.go:96` | Documentation debt | Low | Follow-up (deliberately left untouched per design; belongs with runtime-code slices) |

No blockers remain for the next change.

## Files Changed (Committed)

Per the orchestrator's final-state facts:

| Commit | Files | Summary |
|--------|-------|---------|
| `1053545` | `tests/unit/migration_status_test.sh`, `MIGRATION.md` | Reverse check + 14-row flip + counter + prose fixes |
| `86a45ef` | `CLAUDE.md`, `architecture/02-cli-architecture.md`, `openspec/config.yaml` | Strangler-pattern docs + config absorption |

(`.gitignore` change in `d39faac` is explicitly out of scope per design.)

## Archive Location

- **OpenSpec archive**: `openspec/changes/archive/2026-09-03-sync-migration-status/`
- **Engram archive report**: topic_key `sdd/sync-migration-status/archive-report`

---

**Archive sealed**: 2026-09-04 at UTC session end  
**Prepared by**: sdd-archive sub-agent  
**Verified by**: Task Completion Gate + Mechanical Copy Contract + Final-State Authority hierarchy
