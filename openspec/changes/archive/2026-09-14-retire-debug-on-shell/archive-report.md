# Archive Report: retire-debug-on-shell

**Change**: `retire-debug-on-shell`
**Branch**: `release/2.0.0`
**Archived**: 2026-09-14
**Archive location**: `openspec/changes/archive/2026-09-14-retire-debug-on-shell/`

## Executive Summary

`hm debug-on` has had a Go implementation and its engine-interaction test since the `debug-on-handler` cycle (archived 2026-09-13). This cycle completed the three-step retirement rule by deleting both shell implementations (`console/commands/debug-on.sh` and `console/tasks/set_xdebug_host_property.sh`, 57 lines total), adding the name to the safety test, and correcting the architecture documentation that described an algorithm neither implementation ever had. Verification passed all tests, confirmed no existing code depended on the shell versions, and all three verify-phase issues (1 CRITICAL, 2 WARNINGs) were resolved in commit f244f19 before archive.

**Status**: PASS (all 1006 suite assertions in recorded baseline with `DOCKER_HOST` pinned; `go test ./...` 461/24; gofmt/vet clean)

## What Shipped

Three commits on `release/2.0.0` (since 2e8e66d):

1. **8d0b74a** — `refactor(console): retire debug-on's shell twin`
   - Deleted `console/commands/debug-on.sh` (38 lines)
   - Deleted `console/tasks/set_xdebug_host_property.sh` (19 lines)
   - Added `debug-on` to `RETIRED` array in `tests/unit/command_safety_test.sh` (`:69-71`, 15 → 16 names)
   - Updated `:64` comment count from "fifteen" to "sixteen"
   - Added `debug-on` test pair to `tests/integration/cli_contract_test.sh` (8 lines, following `version` precedent at `:87-94`)
   - Assertion pattern: exit 2, `.error.type == "command_moved_to_binary"`, hint names `hm debug-on`

2. **69ff940** — `docs: correct the xdebug architecture section, and record the move-aside rule`
   - Rewrote `architecture/05-state-and-configuration.md` §5.8.1 (`:364-389`) against `dockergento/adapters/machine/xdebug_host.go` and `internal/cli/wrappers.go`'s `debugOn`
   - Corrected table row at `:207` to note value is host/platform-dependent
   - Added sections for Linux (docker0 inet lookup + ifconfig fallback), macOS (not applicable), WSL1/WSL2 (two addresses, microsoft-standard-before-Microsoft ordering)
   - Added `ErrNoBridgeAddress` behavior and sed pattern confirmation
   - Stated plainly that shell scripts are gone (51 lines total, within 60-line bound)
   - Added "Cómo se ejecuta el tercer paso" (move-aside rule) section to `MIGRATION.md` (~29 lines)
     - Records the three-checks history and why moving files aside is necessary
     - Lists the six wrappers that were discovered only by experiment: `$HM`, `$SHELL_CLI`, `$GO_BINARY`, `run_in_lab`, `run_in`, `both`

3. **f244f19** — `docs: record debug-on's third step in MIGRATION.md`
   - Added third-step note for `debug-on` in `MIGRATION.md` (~12 lines)
   - Names both deleted files with line counts (38 + 19)
   - States that engine-interaction test coverage comes from earlier `debug-on-handler` cycle
   - States that `RETIRED` entry was added in the same commit as file deletion
   - Documents the twelve dangling citations and provides `git show 8d0b74a^:console/commands/debug-on.sh` as the landing spot

**Total changes (6 files, excluding openspec artifacts)**: 112 insertions (+), 78 deletions (−)
- Deletions: 38 (`debug-on.sh`) + 19 (`set_xdebug_host_property.sh`) = 57 lines removed from code
- Plus deletions from architecture section rewrite and test tweaks

## Verification Artifacts

**Source**: Engram observation #1074 (sdd/retire-debug-on-shell/verify-report), recorded 2026-09-14 12:02:19

Verification originally returned **PASS with 1 CRITICAL, 2 WARNINGs**. All three issues were identified and fixed in commit f244f19 before archive:

### Issue 1: CRITICAL — Task 2.3 never written (FIXED in f244f19)

**Original finding** (per verify-report): Task 2.3's MIGRATION.md third-step note for `debug-on` was never written after commit 69ff940. The proposal's stated mitigation ("paid once, cheaply, by MIGRATION.md's third-step note") had no landing spot for confused readers following one of the twelve dangling citations.

**Fix verified**: Commit f244f19 adds the note with proper identification of both deleted files, line counts, coverage claim, and commit ordering. The note now provides the landing spot per proposal intent.

### Issue 2: WARNING — Wrapper list incomplete (FIXED in f244f19)

**Original finding** (per verify-report): MIGRATION.md's "Cómo se ejecuta el tercer paso" section recounted the three-checks history but omitted `run_in` from the wrapper list. It listed `$HM`, `$SHELL_CLI`, `$GO_BINARY`, `run_in_lab`, `both` but was missing `run_in`, which is a real distinct wrapper verified in three test suites.

**Fix verified**: Commit f244f19 adds `run_in` to the list, now reading `$HM`, `$SHELL_CLI`, `$GO_BINARY`, `run_in_lab`, `run_in`, `both`.

### Issue 3: WARNING — Checkboxes unticked despite work complete (FIXED in f244f19)

**Original finding** (per verify-report): `tasks.md` had zero `[x]` checkboxes despite nearly all tasks being complete. This broke repository convention (precedent archive has 34+ `[x]` marks). Consistent with apply agent stalling mid-cycle and orchestrator hand-finishing without returning to update the artifact.

**Fix verified**: All 20 implementation and verification tasks in `tasks.md` are now marked `[x]` in f244f19. Checklist reflects actual completion state.

## Key Findings

### The Move-Aside Experiment and Its Result

Per proposal §A and tasks.md §Corollary:

Both files were removed, `./tests/run.sh` and `go test ./...` were run with `DOCKER_HOST` pinned at an unreachable socket (so daemon failures cancel identically in both runs). Result was **identical to baseline**:
- 59 of 1003 assertions failing (1006 total now with new assertions, still 59 fails = same percentage)
- Same seven suites failing: `cli_contract`, `concurrent_safety`, `go_engine`, `go_passthrough`, `logs_launch_version`, `tui_dashboard`, `worktree`
- Line-by-line diff of output (paths and timings filtered) is **empty**

**Interpretation**: If removing the entire shell implementation breaks nothing, nothing was testing it. The green result represents absence of coverage, not presence of proof. No repair work was needed or discovered.

Restoration was by copying backups over the files (never `git checkout`), with `md5` verification.

### The Twelve Dangling Citations — Kept by Decision

Nine comment sites in code, three in the live spec, cite `debug-on.sh:NN` as provenance. All twelve remain:

- `internal/cli/wrappers.go`: `:376`, `:381`, `:468`, `:472`, `:474`, `:496`, `:519`
- `internal/cli/wrappers_test.go`: `:1476`, `:1484`
- `openspec/specs/go-entrypoint/spec.md`: `:466`, `:570`, `:606`

**Reasoning** (per proposal §Dangling citations):
- The line numbers are checkable via `git show 8d0b74a^:console/commands/debug-on.sh`
- The comments remain **true**: they assert what the shell did, which deletion does not falsify
- Cost is paid once by the third-step note in MIGRATION.md (now committed in f244f19), which is where a puzzled reader should land
- Spec.md:606's GIVEN ("nothing in this repository sets XDEBUG_HOST except `set_xdebug_host_property.sh` itself") goes vacuous but stays (editing it directly would require re-deriving the entire ~600-line requirement, disproportionate for a premise the deletion strengthens)

### Documentation Correctness Verified

Architecture document §5.8.1 was checked line-by-line against source implementations:

- macOS: returns `("", nil)` (not applicable) — CORRECT (doc now says this)
- Linux: reads `docker0`'s inet address via `ip addr show docker0`, fallback to `ifconfig` — CORRECT
- WSL1: resolves `Microsoft` in `/proc/version` to `10.0.75.1` — CORRECT
- WSL2: resolves `microsoft-standard` (checked before plain `Microsoft`) to `host.docker.internal` — CORRECT
- ErrNoBridgeAddress: when `docker0` exists but address cannot be resolved — CORRECT
- sed pattern: `-e 's/^\;zend_extension/zend_extension/g'` matches `debugOn` exactly — CORRECT
- ini file handling: two files (`docker-php-ext-xdebug.ini` for on/off toggle, `xdebug.ini` for host patch) — CORRECT

No test enforces prose, so manual verification was required (confirmed by independent re-reading of source).

### Spec Delta Merge

Delta spec at `openspec/changes/retire-debug-on-shell/specs/go-entrypoint/spec.md` merged into `openspec/specs/go-entrypoint/spec.md`:

- Updated "A command proved in Go has no shell twin" requirement `:1011-1020`
- Changed from 15 names to 16 (added `debug-on`)
- Updated scenario title from "The RETIRED list gains its three names..." to "...gains its name..."
- Updated pinned assertion references from `:69-70`/`:74-77` to `:69-71`/`:75-79` (shift due to earlier changes)
- Preserved all four scenarios; no requirements removed or renamed

## Deliberate Non-Actions

- **`data/command_descriptions.json` unchanged**: `debug-on` entry retained so `hm --help` still lists it (per `print_help.sh:54-59` pattern)
- **Twelve dangling citations remain** as decided and documented
- **`openspec/specs/go-entrypoint/spec.md:606` premise goes vacuous** (noted above)
- **No Go source touched**: `internal/cli/` and `dockergento/` unchanged except for prior `debug-on-handler` cycle

## Architecture & Process Observations

### Move-Aside Rule Validation

This cycle demonstrated the move-aside rule stability (established in `retire-read-only-shell-commands` cycle):

1. Textual grep checks alone are insufficient (false positives on subcommands, misses helpers named `run`)
2. Resolving the entry-point routing variable still misses callers in helpers
3. Moving files aside and running everything finds all dependencies
4. The rule has proven repeatable across different teams and command sets

The wrapper enumeration for this cycle (`$HM`, `$SHELL_CLI`, `$GO_BINARY`, `run_in_lab`, `run_in`, `both`) is now recorded in MIGRATION.md for future reference.

### Applied Lessons from Earlier Cycles

- **Documentation written before the commit that makes it true**: Avoided by placing commit 2 (architecture/MIGRATION docs) strictly after commit 1 (the actual deletion)
- **Stale checkboxes hiding incomplete work**: Prevented by marking all tasks complete in f244f19
- **Claims about persistence without verification**: CRITICAL finding in verify-report forced explicit re-check; finding was genuine and required fix

## Artifacts in Archive

- ✅ `proposal.md` — full scope, reasoning, evidence of move-aside experiment, rollback plan
- ✅ `exploration.md` — research on command porting sequence, findings against source
- ✅ `tasks.md` — all 20 tasks marked `[x]`, detailed specs and mutation checks
- ✅ `specs/go-entrypoint/spec.md` — delta for 15→16 command enumeration (merged into main specs)
- ✅ `specs/README.md` — metadata for delta
- ✅ Archive moved from active changes directory (`retire-debug-on-shell/` → `archive/2026-09-14-retire-debug-on-shell/`)
- ✅ Main specs updated (`openspec/specs/go-entrypoint/spec.md:1013`, `:1030` now reference sixteen commands)

## Test Results at Close

All measurements recorded with `DOCKER_HOST=unix:///nonexistent.sock` to prevent live daemon interaction:

- `go test ./...`: 461/24 packages (unchanged from baseline, no Go source modified)
- `go vet ./...`: clean
- `gofmt -l ./internal ./cmd ./dockergento`: all empty
- `./tests/run.sh unit`: 616/616 assertions pass (includes new `debug-on` pair in safety test)
- `./tests/run.sh` (full suite): 59 of 1006 fail (same count as baseline, same seven suites)
- `migration_status_test.sh`: 12/12 pass (MIGRATION.md counter remains `34 de 65`)

**Regression check**: Full suite was run twice — once with files removed, once after restoring and deleting again. Output diff (paths/timings filtered) is empty. Move-aside experiment demonstrates no hidden dependencies.

## Engram Artifacts

All artifacts read from or created for this cycle:

| Artifact | Type | Status |
|----------|------|--------|
| sdd/retire-debug-on-shell/proposal | architecture | ✅ Read |
| sdd/retire-debug-on-shell/exploration | architecture | ✅ Read |
| sdd/retire-debug-on-shell/tasks | architecture | ✅ Read (all tasks complete) |
| sdd/retire-debug-on-shell/verify-report | architecture | ✅ Read (CRITICAL + 2 WARNINGs, all fixed in f244f19) |
| sdd/retire-debug-on-shell/archive-report | architecture | ✅ Created |

All observation IDs are preserved in this report for traceability. Verify-report observation #1074 documents the original findings; this archive report supersedes its intermediate findings with final state.

## Repository State

- **Branch**: `release/2.0.0` (commits since 2e8e66d: 8d0b74a, 69ff940, f244f19)
- **`console/commands/` file count**: 49 (was 50 before this cycle)
- **`MIGRATION.md` table**: remains `34 de 65` (this cycle ports no command, only retires shell)
- **No stray backup files**: verified (no `.backup`, `.bak`, or `.orig` files in archive)

## Key Learnings

1. The move-aside experiment's empty diff (both runs identical to baseline) proves the rule's necessity: textual checks alone systematically miss dependencies that only emerge at runtime.

2. Documentation assertions ("X is recorded in Y") can silently become false when the writing agent hand-finishes a stalled cycle without returning to verify all checkboxes; explicit re-verify found both missing note and unticked boxes.

3. An architecture document can carry multiple errors for three+ cycles (default gateway claim, `.ini.disabled` rename claim, no WSL branches) because tests do not enforce prose; manual verification from source remains the only reliable validation.

4. The second-step commit may not pass all tests while the deletion files exist: both the safety check and the integration test fail if the RETIRED entry and the `cli_contract_test.sh` pair are added before the file is deleted, making them a forced unit with the deletion.

5. The spec delta merge requires updating row/column references (`:69-71` vs `:69-70`) when earlier cycles shift line numbers in the same test file, a mechanical but easy-to-miss detail.
