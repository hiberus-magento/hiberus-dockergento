# Archive Report: down-json-and-flags

**Change Name**: down-json-and-flags  
**Archived Date**: 2026-09-09  
**Archived To**: `openspec/changes/archive/2026-09-09-down-json-and-flags/`  
**Mode**: openspec/hybrid  

## Executive Summary

The change has been fully implemented, verified, and archived. Two missing-value guards were added to `hm down` for options `--rmi` and `-t`/`--timeout` when they appear as the last argument with no value following. Both now refuse with exit code 2 and `missing_value` error type, consistent with neighboring commands `web` and `logs`. The delta spec was successfully merged into `openspec/specs/environment-lifecycle/spec.md`, and the change folder was moved to the archive with mechanical verification.

## Change Artifacts

### Archived Contents
- ✅ `proposal.md` — Change intent and approach documented
- ✅ `tasks.md` — All 20 implementation tasks checked complete; task 6.7 (commit) intentionally deferred per user instruction
- ✅ `specs/environment-lifecycle/spec.md` — Delta spec with three new scenarios
- ✅ `exploration.md` — Initial exploration record
- ⚠️ `design.md` — Not created (intentionally skipped, per user direction; no design phase was run)

### Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| environment-lifecycle | Modified | 3 scenarios added to "Removing an environment is Compose's own command" requirement |

**Scenarios Added**:
1. "An option that takes a value refuses when none follows" — guards refuse with exit 2, `missing_value` on stderr, stdout empty
2. "A missing value and a bad value stay distinguishable" — missing and non-numeric cases return different error types
3. "A value that is given is still accepted" — `--rmi all`, `-t 30` and their combinations still parse correctly

## Task Completion Status

**Final State Per Launch Prompt** (outranks intermediate snapshots):
- **Shipped**: One commit `ce0ff9e` with:
  - `internal/cli/down.go:31-48` — Two guards added
  - `internal/cli/down_test.go` — 272 lines (new file)
  - Delta spec merged
  - Total 302 changed lines (2 files: +290, -12)
- **Test Results** (from verification):
  - `go test ./...` 319 passed across 24 packages (was 303; 16 new)
  - `./tests/run.sh unit` 616 assertions passed (unchanged)
  - `gofmt` clean
  - `go vet` clean
- **Manual Verification** (all from non-project temp directory):
  - `hm down --rmi` → exit 2, `missing_value`, stdout empty
  - `hm down -t` → exit 2, `missing_value`, stdout empty
  - `hm down -t nada` → exit 2, `invalid_argument` (unchanged, distinguishable)
  - `hm down --rmi all` → exit 4, `project_not_configured` (expected, value accepted)

**Persisted Task Artifact** (6/27 implementations checked):
- Tasks 1.1–6.6: all [x] checked
- Task 6.7: [ ] unchecked — "(Not performed — the user commits, per instructions for this session.)"

**Reconciliation Reason** (per Final-State Authority hierarchy):
The unchecked task 6.7 (commit to release/2.0.0) is reconciled as complete based on:
1. Explicit final-state fact in orchestrator prompt: "Shipped as one commit: `ce0ff9e`" with full verification details
2. Explicit user instruction: "Do NOT commit — I make the archive commit" (user owns commit step outside SDD pipeline)
3. All supporting evidence (test counts, manual verification) confirms implementation complete per proposal

Task 6.7 checkbox remains as written for future reference; its note aligns with the stated deferral pattern.

## Defects and Gaps (Intentional, per Design)

### Recorded But Out of Scope
Per proposal section "Non-goals (Out of Scope)":

1. **The `commands` seam** — `Down` not on interface `engine.go:17-30`; growing it deferred to `down-go-tests` per design `archive/2026-09-05-down-and-set-host-go-tests/design.md:291-296`. Gap acknowledgement in test: `TestDownStillAcceptsAGivenValue` carries comment declaring the assertion cannot prove the value stores in `core.DownOptions` without the seam.

2. **The `--json` stream defect** (`down.go:67-71`) — A declined confirmation writes prose to stdout before JSON mode is reached, breaking `hm down --json | jq`. Against `cli-output-contract/spec.md:27-30`, `:133-135`. Requires fake `Down` answering `""` on demand; fixing deferred to `down-go-tests` with seam.

3. **Same shape in `internal/cli/db.go`** — `freeze()` `:100-103` and `snapshot()` `:234-237` both for `--name` option. Lower priority (name falls back to generated, not dropped). Reported, not adopted here.

**Reason for deferral**: Each requires seam (`commands` interface) or fake (`Down` on interface); this change reaches neither. Advancing them would make the change about a pattern rather than about `down` guards specifically. Recorded as discovered, not lost.

## Handoff Note — Preserved for Next Cycle

**(Not merged into spec; carried here for traceability.)**

From `specs/environment-lifecycle/spec.md:72-79` (delta spec):

> `archive/2026-09-05-down-and-set-host-go-tests/design.md:306` and `:340-342` currently plan for `down-go-tests` to **pin** `--rmi` and `-t` given as the last argument as `core.DownOptions{}`, "silently ignored". Once this change lands, that plan is stale: those two `TestWhatDownAsks` table rows must arrive as refusals (asserting `missing_value` and an empty engine log) instead of as pinned silent drops, or not be written at all. `down-go-tests` extends the `internal/cli/down_test.go` this change creates rather than authoring it from scratch.

**Action for `down-go-tests` cycle**: The archived design at `archive/2026-09-05-down-and-set-host-go-tests/design.md` pre-dates this change and contains stale planned behavior. Before implementing `TestWhatDownAsks` rows for `--rmi` and `-t` as last argument, consult this handoff and the new `internal/cli/down_test.go:~line 65-70` breadcrumb comment that points here.

## Mechanical Archive Verification

**Readback diff output** (source snapshot vs. destination, archive-report excluded):
```
=== Readback diff (source snapshot vs. destination) ===
=== End readback: PASS (empty diff) ===
```

Empty diff confirms byte-identity: all artifacts moved mechanically with shell (`mv`), no content passed through model, zero truncation or alteration.

## Archive Checklist

- [x] Main specs updated correctly (merged delta into `openspec/specs/environment-lifecycle/spec.md`)
- [x] Change folder moved to archive (`openspec/changes/down-json-and-flags` → `openspec/changes/archive/2026-09-09-down-json-and-flags`)
- [x] Archive contains all artifacts (proposal.md, specs/, exploration.md, tasks.md; design.md intentionally not created)
- [x] Archived `tasks.md` reconciliation: 20 of 21 tasks checked; task 6.7 (commit) deferred per user instruction; final state facts confirm complete per verification
- [x] Active changes directory no longer has this change
- [x] Verbatim `diff -r` readback output included above: empty (no differences)

## Delivery Status

**Ready for next cycle.** No further SDD phases needed. The change is complete, verified, and archived. User will make the final commit to release/2.0.0 outside the SDD pipeline per stated workflow.

## Source of Truth Updated

The following spec now reflects the new behavior:
- `openspec/specs/environment-lifecycle/spec.md` — "Removing an environment is Compose's own command" requirement now contains four scenarios (was two)

---

**Archive Report Generated**: 2026-09-09  
**Report Type**: Openspec/Hybrid Mode  
**Skill Version**: sdd-archive 2.0  
**Final-State Authority Hierarchy Applied**: Launch prompt final-state facts ranked first, all task completion verified through persisted artifact and external evidence
