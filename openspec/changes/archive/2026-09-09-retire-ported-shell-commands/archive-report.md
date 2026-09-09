# Archive Report: retire-ported-shell-commands

**Date Archived**: 2026-09-09  
**Change Name**: retire-ported-shell-commands  
**Archive Location**: `openspec/changes/archive/2026-09-09-retire-ported-shell-commands/`  
**Branch**: release/2.0.0  
**Repository**: hm (Hiberus Dockergento)

---

## Executive Summary

The third step of `MIGRATION.md`'s porting rule has been executed for the first time. Twelve commands have had their shell implementations deleted after being proved in Go and all call sites rewired to use the binary. The change shipped as five commits with a combined ~1024 changed lines across four implementation units and one defect fix, all verified green with 297 Go tests, 616 shell unit assertions, and 106 integration tests.

---

## Final State Authority and Verification

This archive report reflects the change AT CLOSE, per the SDD Final-State Authority hierarchy. The user provided explicit final-state facts confirming:

- All five commits shipped and verified green on release/2.0.0
- Test counts: 297 Go tests across 24 packages; 616 shell unit assertions; 106 integration tests
  - Integration suites: help (14), cli_contract (20), concurrent_safety (9), go_setup (35), go_wrappers (28, improved from 19)
- 403 lines of shell deleted
- All 12 retired commands still appear in `hm --help` with unchanged descriptions
- Verification caught and fixed one defect in the help coverage test (`tests/integration/help_test.sh`)
- Two design-phase tautology cases (n98-magerun argument joining, version oracle) were rewritten and mutation-checked

The persisted tasks artifact (`openspec/changes/archive/2026-09-09-retire-ported-shell-commands/tasks.md`) shows all 28 implementation tasks marked complete (`[x]`).

---

## Outcome and Scope

### Deliverable

**What was deleted**: 12 shell implementations (403 lines) under `console/commands/`:
- `set-host` (109 lines)
- `version` (85 lines)
- `copy-to-container` (64 lines)
- `copy-from-container` (46 lines)
- `varnish-off` (21 lines)
- `varnish-on` (18 lines)
- `mysqldump` (13 lines)
- `purge` (11 lines)
- `n98-magerun` (10 lines)
- `test-unit` (10 lines)
- `npm` (4 lines)
- `test-integration` (4 lines)

**What was rewritten**: Four by-path callers rewired to use the binary through a new `hm_run_binary` helper:
- `console/tasks/magento_installation.sh:32` — for `set-host`
- `console/commands/composer.sh:39, :51` — for `copy-to-container` (macOS only)
- `console/commands/debug-on.sh:22` — for `copy-from-container` (macOS only)

**What changed**: Six test cases, two help components, one bridge message, setup's install door, and six parity test rewrites.

**What stayed**: 33 shell commands untouched, the custom-commands pass in help unmodified, every command's name, arguments, prompts, exit codes and output format preserved.

---

## Implementation Size and Budget

| Unit | Components | Estimate | Actual | Per-unit Status |
|------|------------|----------|--------|-----------------|
| 1 | selfrun adapter + wiring + bridge case | 170 | 170 | Under budget |
| 2 | binary helper + 3 callers + help + bin/run + command_safety + setup door + concurrent_safety tests | 254 | 244 | Under budget |
| 3 | 3 call sites + 2 file deletions | 117 | 118 | Under budget |
| 4 | 9 deletions + 6 parity rewrites + drift test + MIGRATION.md | 340 | 389 | Under budget |
| **Total** | — | ~881 | ~1024 | ✗ Exception accepted |

**Size Exception**: Cumulative diff is ~1024 changed lines against the 400-line per-unit budget. Each of the four units stayed under 400 lines individually. The user accepted a size exception for the cumulative total before apply, measured with `git diff --shortstat <base>..HEAD -- . ':(exclude)openspec'`.

---

## Five Commits Shipped

Per the user's launch prompt, the change shipped as five commits on release/2.0.0:

1. **5f536d7** — Unit 1, setup install door becomes the binary. New `dockergento/adapters/selfrun` adapter (273 lines). `TestTheInstallDoorIsTheBinary` and `TestTheInstallStepsRunInOneOrderThroughOneDoor` added.

2. **92444a1** — Unit 2, groundwork and `set-host` deletion. New `console/helpers/binary.sh`, `bin/run`'s exit-2 `command_moved_to_binary` branch, `print_help.sh` unions files + declared-no-file names with sorted `.sh` suffix. Four call sites rewired. `command_safety_test.sh` relaxed to "file or route." Tests extended (244 lines).

3. **7d001a9** — Unit 3, `copy-to-container` and `copy-from-container` deletions. Three by-path call sites rewired (118 lines).

4. **583fb0c** — Unit 4, nine remaining deletions, parity rewrites, drift test. Six `go_wrappers_test.sh` cases rewritten against independent oracles (npm recorder, n98-magerun arg counting, test-unit invocation recording, mysqldump restore-and-count, version git oracle, version text against git oracle). `MIGRATION.md` updated (389 lines).

5. **04bac1e** — Defect fix: `tests/integration/help_test.sh` iterated only `console/commands/*.sh`, so it never asked about commands with no file. Changed to iterate declared names; mutation-checked (eleven of twelve missing commands surfaced when union deleted).

---

## Design Phase Quality

The design phase identified two fresh tautology cases during verification:

1. **n98-magerun argument joining** — A shell recorder logging `"$@"` would render `bash -c "n98-magerun arg1 arg2"` the same way whether arguments were passed separately (3 args) or joined (1 arg after `-c`). Task 4.5 rewritten to check explicit argument count via printf-based recorder.

2. **version --json oracle** — A comparison of JSON between two CLI implementations cannot catch a value wrong in both. Task 4.8 rewritten to validate each field against `git describe`, `git rev-parse --short HEAD`, and git status as independent oracles.

Both were mutation-checked during apply: breaking the binary's behavior causes the new assertions to fail.

The same tautology class surfaced a third time in verification: `tests/integration/help_test.sh` assumed all commands had files, so it never tested declared-only commands. Fixed in commit 04bac1e (task 4.9 implied).

**Unplanned but necessary**: `tests/unit/skills_test.sh` also held the file-only assumption and needed the same file-or-route fallback. Not mentioned in design or tasks; appears in the verification report.

---

## Specs Merged

Three delta specs merged into main specs:

### 1. go-entrypoint (`openspec/specs/go-entrypoint/spec.md`)

**ADDED**: New requirement "A command proved in Go has no shell twin" (3 scenarios):
- Nothing under the shell tree implements it (pinned by `command_safety_test.sh`'s drift case)
- Asking the shell entry point for a retired command (pinned by `cli_contract_test.sh`'s `command_moved_to_binary` cases)
- A shell caller needs the binary and it is not installed (pinned by `exit_codes_test.sh`'s binary_missing case)

**MODIFIED**: "A ported command's shell entry point still answers by delegating" — scoped to entry points that are kept (vs. read universally). Distinction clarified: delegating stubs (`docker-stop-all`, `docker-compose`, `worktree`) vs. retired commands with no shell entry point at all. Added note "(Previously: read as universal...)". Added third scenario for `worktree` delegation.

### 2. command-discovery (`openspec/specs/command-discovery/spec.md`)

**MODIFIED**: "Ningún comando desaparece" scenario — changed comparison from command files to declared commands in `data/command_descriptions.json`. Added "(Previously: comparaba los comandos listados contra los ficheros de comandos existentes...)" note explaining the old behavior.

### 3. bootstrap-options (`openspec/specs/bootstrap-options/spec.md`)

**ADDED**: New requirement "The install steps run in order through one door" (3 scenarios):
- The declared order is kept (pinned by `setup_test.go`'s order test)
- A ported step is answered natively (pinned by `TestTheInstallDoorIsTheBinary`)
- An unported step still reaches the bridge (pinned by `go_setup_test.sh`'s "step the binary does not route reaches the bridge" case)

**Readback verification**: All three main spec files modified correctly. Delta changes appended/merged without removing existing content.

---

## Archive Contents

```
openspec/changes/archive/2026-09-09-retire-ported-shell-commands/
├── proposal.md              (2.2K — full proposal with intent, scope, approach, risks, rollback)
├── design.md                (3.4K — detailed design with decisions, rationale, architecture)
├── tasks.md                 (33K — 28 tasks, all marked [x] complete)
├── exploration.md           (4.9K — exploration work and context)
├── specs/
│   ├── go-entrypoint/spec.md        (delta spec for go-entrypoint)
│   ├── command-discovery/spec.md    (delta spec for command-discovery)
│   └── bootstrap-options/spec.md    (delta spec for bootstrap-options)
├── archive-report.md        (this file, added at archive time)
└── .openspec.yaml           (config file)
```

**Mandatory diff -r readback**: Verified empty — source snapshot and archived destination are byte-identical.

---

## Final Verification Status (per user's launch prompt)

- ✅ **Go tests**: 297 across 24 packages, all green
- ✅ **Shell unit assertions**: 616 total, all green
- ✅ **Integration suites**: help (14), cli_contract (20), concurrent_safety (9), go_setup (35), go_wrappers (28 — improved from 19)
- ✅ **Build**: `go build -o bin/hm ./cmd/hm` succeeds
- ✅ **Format/lint**: `gofmt -l ./cmd ./internal ./dockergento` empty, `go vet ./...` clean
- ✅ **Help output**: Byte-identical to pre-change baseline
- ✅ **All 12 commands appear in help**: With unchanged descriptions
- ✅ **Regressions caught and fixed**: Help coverage test (commit 04bac1e)
- ✅ **No shell twins remain**: Grep for `copy-to-container.sh|copy-from-container.sh|set-host.sh` finds nothing

---

## Carried Forward (Open Items, Not Blockers)

The archive records these as follow-ups for the next cycle:

1. **`hm --help` substring matching** — The help coverage check matches command names as substrings, so `version` passes trivially because the word appears in help text elsewhere. Pre-existing, not introduced here.

2. **`architecture/04-command-translation.md:33` stale reference** — Still cites `console/commands/npm.sh` as an example; stale after this deletion. Should be updated to show a command with no file.

3. **`dockergento/app/setup.go:516-530` unported steps** — Still runs `install` and `ssl` through the bridge. These are the only two install steps not yet ported to Go.

4. **`down` unrouted in `internal/cli/run.go`** — The command is not routed anywhere in the binary's switch. Tracked in MIGRATION.md but not yet ported.

5. **Three legitimate delegation stubs** — `docker-stop-all`, `docker-compose`, `worktree` correctly keep their shell files as entry points that delegate. These were explicitly deferred ("the three delegating stubs" in design.md).

---

## Rollback Plan

Two commits on release/2.0.0:
- `git revert 04bac1e` (defect fix)
- `git revert 583fb0c` (unit 4)
- `git revert 7d001a9` (unit 3)
- `git revert 92444a1` (unit 2)
- `git revert 5f536d7` (unit 1)

In that reverse order, restores all twelve shell files, four call sites, help glob-only logic, and `bin/run` message. Reverting any subset up to 92444a1 (unit 2) leaves the tree consistent.

---

## Migration Completion

This change executes the third step of `MIGRATION.md`'s porting rule for the first time, completing the cycle for twelve commands: `set-host`, `version`, `copy-to-container`, `copy-from-container`, `varnish-off`, `varnish-on`, `mysqldump`, `purge`, `n98-magerun`, `test-unit`, `npm`, `test-integration`.

The table in `MIGRATION.md` already marks all twelve as `go` — this archive changes the ground truth: they no longer have shell twins to delete in a future cycle.

---

## Archive Status

**Status**: ✅ **COMPLETE**

- All artifacts present and verified
- All 28 tasks marked complete
- All three delta specs merged into main specs
- Change folder moved to archive with date prefix
- Mandatory diff -r readback passed (byte-identical)
- No unchecked implementation tasks
- No CRITICAL issues in verification
- Ready for next change cycle

**Next Recommended**: Begin next SDD change or continue organic development.

---

## SDD Cycle Close

This change followed the full SDD cycle:
1. ✅ Proposal: intent, scope, approach, risks, rollback
2. ✅ Spec: three delta specs prepared for merge
3. ✅ Design: detailed work breakdown with decisions and rationale
4. ✅ Tasks: 28 implementation tasks, strict TDD throughout
5. ✅ Apply: five commits shipped, all tests green, defect caught and fixed
6. ✅ Verify: 297 Go, 616 shell, 106 integration tests passing
7. ✅ Archive: specs merged, change folder archived, audit trail complete

---

**Archived by**: sdd-archive executor  
**Archive Date**: 2026-09-09  
**Repository State**: release/2.0.0, clean  
**Final Commit on Archive Date**: 04bac1e (defect fix from verification)
