```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:30a8ff43ff0ad13b4880a4bf8706e11670c9ff89cf86e0a689d8420c4bc3588b
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 2/2
scenarios: 13/13
test_command: go test ./... -short
test_exit_code: 0
test_output_hash: sha256:581bafcf3dba8a619fe217e459f86c8dac4a406fa60926547d7f72414af93cdb
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: remaining-wrappers-go-tests
**Version**: N/A (delta spec, no version field)
**Mode**: Strict TDD

### Completeness

| Metric | Value |
|--------|-------|
| Tasks total | 13 |
| Tasks complete | 13 (13 checked, including 2.3 explicitly marked complete-as-deferred) |
| Tasks incomplete | 0 |

The delta spec was reconciled by the orchestrator on 2026-09-05: the two `version`-bound scenarios
("Reporting what is installed", "version with an argument") were removed from
`specs/go-entrypoint/spec.md` and moved, via its Note, to the follow-up change
`down-and-set-host-go-tests`, alongside `proposal.md`'s cut-order record of the same date. The
delta spec now carries 13 scenarios (6 carried + 7 new) across 2 requirements, and every one of
them has a passing covering test in this change's delivered code.

### Build & Tests Execution

**Build**: PASSED
```text
$ go build ./...
(no output, exit 0)
```

**Tests**: 44 passed / 0 failed (package `internal/cli`, focused) — 204 passed / 0 failed (full repo, `go test ./... -short`, 22 packages) — 612 assertions passed (`tests/run.sh unit`, Bash side, unmodified)
```text
$ go test ./internal/cli -short
Go test: 44 passed in 1 packages

$ go test ./... -short
Go test: 204 passed in 22 packages

$ tests/run.sh unit
612 assertions passed
```
Both counts match the orchestrator's stated expectations (44, 204, 612) exactly. Reused unchanged
from the prior verify pass: the working tree's Go code did not change between passes, only
`openspec/changes/remaining-wrappers-go-tests/{specs/go-entrypoint/spec.md,proposal.md}`, which
are excluded from the size/behavior budget and carry no runtime evidence of their own. No project
or Docker command was re-run.

**gofmt**: `gofmt -l ./cmd ./internal` → empty (clean)
**go vet**: `go vet ./...` → clean

**Coverage**: Not available (no coverage tool detected in this project) — informational only, not blocking.

### Spec Compliance Matrix

| Requirement | Scenario | Test | Result |
|---|---|---|---|
| A ported command's engine interaction is provable without Docker | Copying into the container | `wrappers_test.go > TestWhatCopyingIntoTheContainerAsks` | COMPLIANT (carried, unchanged) |
| " | Copying everything into the container | `wrappers_test.go > TestWhatCopyingIntoTheContainerAsks` (subtest) | COMPLIANT (carried, unchanged) |
| " | Copying out of the container | `wrappers_test.go > TestWhatCopyingOutOfTheContainerAsks` | COMPLIANT (carried, unchanged) |
| " | Turning the page cache on | `wrappers_test.go > TestWhatVarnishAsks` | COMPLIANT (carried, unchanged) |
| " | Turning the page cache off cascades | `wrappers_test.go > TestWhatVarnishAsks` (subtest) | COMPLIANT (carried, unchanged) |
| " | Clearing generated code (purge) | `inside_test.go:21 > TestWhatPurgeAsks` | COMPLIANT — asserts the seven literal directories from `wrappers.go:22-25` via `cmp.Diff` |
| " | Running the front-end package manager (npm) | `inside_test.go:54 > TestWhatNpmAndMagerunAsk/npm_passes_its_arguments_straight_through` | COMPLIANT |
| " | Running n98-magerun | `inside_test.go:85 > TestWhatNpmAndMagerunAsk/n98-magerun_joins_its_arguments_through_a_shell` | COMPLIANT — `bash -c "n98-magerun ..."` join asserted |
| " | Running the unit suite | `inside_test.go:112 > TestWhatTheTestSuitesAsk` (3 unit rows: configured, fallback, appended args) | COMPLIANT |
| " | Running the integration suite | `inside_test.go:112 > TestWhatTheTestSuitesAsk` (3 integration rows: configured, doubled-fallback, appended args) | COMPLIANT |
| " | Writing a database dump | `wrappers_test.go:306 > TestWhatMysqldumpAsks` (success / `--json` / refusal subtests) | COMPLIANT |
| A usage error returns before any engine call | No path given (`copy-to-container`/`copy-from-container`) | `wrappers_test.go:92 > TestCopyingWithNoPathIsRefused` | COMPLIANT (carried, unchanged) |
| " | mysqldump with no path | `wrappers_test.go:386 > TestMysqldumpWithNoPathIsRefused` | COMPLIANT |

**Compliance summary**: 13/13 scenarios compliant.

### Correctness (Static Evidence)

| Requirement | Status | Notes |
|---|---|---|
| A ported command's engine interaction is provable without Docker | Implemented | All 11 scenarios under this requirement (as reconciled) have passing covering tests |
| A usage error returns before any engine call | Implemented for its 2 scenarios; one prose loose end | Both scenarios ("No path given", "mysqldump with no path") pass. The requirement's own body text still names `version` ("`copy-to-container`, `copy-from-container`, `mysqldump`, and `version` SHALL validate...") even though the `version` scenario was removed by the reconciliation — a leftover word, not a missing scenario (see SUGGESTION below) |

### Coherence (Design)

| Decision | Followed? | Notes |
|---|---|---|
| Interface gains exactly `Property` and `Dump` (`Installed` deferred) | Yes | `internal/cli/engine.go:23-24`; no `Installed` present |
| Three call sites routed (`wrappers.go:69,77,104`); `version.go:23` untouched | Yes | Confirmed via `git diff 5e9f443..5e3b1af` — only `engine.go`, `wrappers.go`, `fake_engine_test.go`, `inside_test.go`, `wrappers_test.go` changed (5 files total) |
| Other `engine(` call sites untouched | Yes | Full-diff `--stat` shows exactly those 5 files touched; every other `engine(` call site (doctor.go, mysql.go, setup.go, worktree.go, proxy.go, tools.go, down.go, orchestrate.go, describe.go, run.go, db.go, web.go, list.go, version.go, clean.go, worktree_add.go, wrappers.go:281/296) is byte-identical to base |
| Fake fields and logging as designed (`properties` map, `call.Key`/`call.Path`, both appended to the ordered log) | Yes | `fake_engine_test.go:51-55,111-124` |
| No `t.Parallel()` | Yes | `rg t.Parallel` over the three changed/added test files returns zero matches |
| Purge's seven directories literal, equal to `wrappers.go:22-25` | Yes | `inside_test.go:38-45` string matches `wrappers.go:22-25` verbatim |
| Magerun uses `bash -c` | Yes | `wrappers.go:55-56`, asserted at `inside_test.go:95-100` |
| Suites table includes both fallbacks and the doubled `/var/www/html/./vendor/bin/phpunit` with a comment | Yes | `inside_test.go:177-192` (case + comment) |
| Mysqldump success/`--json`/refusal/no-path | Yes | `wrappers_test.go:306-398`, all four covered |
| `Exec` calls carry `phpService` + `terminalOptions("")` | Yes | Every `Exec` want-value in `inside_test.go` uses `Service: phpService, Options: terminalOptions("")`; matches `php.go:57-60`'s `inside()` |

### Task Completion

13/13 tasks checked. Both commits (`aba8cd6`, `5e3b1af`) exist and match the stated file lists
exactly (`git show <sha> --stat`). Task 2.3 is explicitly ticked-as-deferred with the correct
rationale text, now consistent with the reconciled `specs/go-entrypoint/spec.md` Note and
`proposal.md`'s "Applied on 2026-09-05" cut-order record. The manual verification record (task
3.1, Phase 3) exists in `apply-progress.md` and is internally coherent with the deferral (it
explicitly notes `version`'s comparison moves with `version` to `down-and-set-host-go-tests`).

### Commit Hygiene

Commit `aba8cd6` was checked out to an isolated `git worktree` at
`/private/tmp/.../scratchpad/verify-aba8cd6` (removed after the check): `go build ./...` succeeded
and `go test ./internal/cli -short` passed with 39 leaf test-pass events (40 pass events counting
the package-level summary), matching the orchestrator's stated expectation exactly. Commit
`5e3b1af` (HEAD) was verified in place: `go build ./...` and `go test ./... -short` both pass
(204/204). Each commit is self-consistent as claimed in `design.md`'s "Size and Commits" section.

### Scope Check

`git diff --stat 5e9f443..5e3b1af` touches exactly 5 files, all under `internal/cli/`:
`engine.go`, `fake_engine_test.go`, `inside_test.go` (new), `wrappers.go`, `wrappers_test.go`.
Nothing under `dockergento/`, `console/`, `bin/run`, `MIGRATION.md`, or `tests/` appears in the
diff. `here()` vs `project.Root` in `dump()` (`wrappers.go:104`) is unchanged and still uses
`here()`, matching design's open question (intentionally not fixed). The deferred varnish
`report(nil err)` defect is untouched — no line in `wrappers.go:148-170` (the varnish handlers)
changed.

### Deferred scope (reconciled, no longer a scenario-count gap)

`version`/`Installed` is deferred to the follow-up change `down-and-set-host-go-tests`, per the
1.5 size-gate hard-stop in `tasks.md`/`design.md`: Commit 1 measured 247 changed lines (over the
220-line gate) because `inside_test.go` came out to 227 lines against a ~150-line estimate (six
exhaustive table cases plus comments, not four). This is now fully reflected in the artifact set:
`specs/go-entrypoint/spec.md`'s Note records the move of both `version` scenarios and the
`Installed` seam method to the follow-up change, and `proposal.md` records the cut as "Applied on
2026-09-05". Confirmed in the repository: `internal/cli/version_test.go` does not exist,
`Installed` is absent from the `commands` interface and the fake, and `version.go:23` still calls
`engine(` directly (unrouted) — exactly what the reconciled spec now describes as out of scope for
this change, not a missing scenario within it. Final size after both commits: 359 changed lines,
under the 400 budget, no `size:exception` used.

---

## TDD Compliance

| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | Found in apply-progress | "TDD Cycle Evidence" table present, 2 rows (`Property`, `Dump`) |
| All tasks have tests | 2/2 tasks with routed call sites have test files | `inside_test.go`, `wrappers_test.go` additions |
| RED confirmed (tests exist) | 2/2 test files verified | `inside_test.go` and the `wrappers_test.go` mysqldump additions both exist in the tree |
| GREEN confirmed (tests pass) | 2/2 pass on execution | `go test ./internal/cli -short` → 44 passed, no failures |
| Triangulation adequate | Adequate | `Property`: 7 subtests across `TestWhatTheTestSuitesAsk` (2 kinds × configured/fallback/args, minus one shared fallback row); `Dump`: 3 subtests (success, `--json`, refusal) |
| Safety Net for modified files | Present | Baseline `go test ./internal/cli -short` was 28 (re-confirmed via `git stash` in apply-progress) before either commit; both modified files (`wrappers.go`, `engine.go`) had their pre-existing coverage re-run green after each change |

**TDD Compliance**: 6/6 checks passed

The apply-progress RED narrative is independently plausible from the commits' own diffs: before
routing, `Property` reaches `fsprops.Reader.Load` for a nonexistent root (answers `""`), so the
fallback path (`./vendor/bin/phpunit`) diverges from the fake's pinned `bin/phpunit`, and the log
lacks `Property` entries entirely (the real engine doesn't log). Before routing, `Dump` reaches
`e.database().Ready(project)`, which dials Docker; under `DOCKER_HOST=unix:///nonexistent` that
dial fails deterministically (`exitDocker`), and the `Dump` entry is absent from the log since the
call never reached the fake. Both claims are consistent with the routed code (`wrappers.go:69,77,104`)
and the design's stated RED mechanics (`design.md` Decision 5). This assessment is reasoned from
the commit diffs and source, not independently re-run against a real daemon (out of scope for a
read-only verify pass).

---

## Test Layer Distribution

| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit | 16 new leaf tests (1 purge + 3 npm/magerun + 7 test-suites + 4 mysqldump + 1 no-path) | 2 (`inside_test.go` new, `wrappers_test.go` modified) | Go stdlib `testing`, `go-cmp` |
| Integration | 0 new (unchanged `tests/integration/go_wrappers_test.sh` parity suite, not touched) | 0 | Bash/Docker (untouched) |
| E2E | 0 | 0 | — |
| **Total** | **16** | **2** | |

---

## Changed File Coverage

Coverage tooling not detected in this Go project's toolchain configuration (no `-cover` invocation
wired into the project's test runners). Reported instead via execution evidence: `go test
./internal/cli -short` passes 44/44 with the new suites (`inside_test.go`, mysqldump additions)
exercising every line the design attributes to them. Coverage analysis skipped — no coverage tool
detected.

---

## Assertion Quality

Scanned `inside_test.go`, the `wrappers_test.go` additions, and the `fake_engine_test.go` growth
for the banned patterns (tautologies, orphan empty checks, type-only-alone assertions, no-op
loops, smoke-test-only, implementation-detail coupling, mock-heavy ratios).

**Assertion quality**: All assertions verify real behavior. Every test calls the production
handler under test (`purge`, `npm`, `magerun`, `tests`, `dump`) and asserts on its exit code plus
a whole-log `cmp.Diff` against a literal `want` slice built from the parity suite/source, not from
observed output. No tautologies, no empty-collection-only assertions without a companion
non-empty case (every "no calls" assertion, e.g. `TestMysqldumpWithNoPathIsRefused`, has a sibling
test in the same suite asserting a non-empty log for the success path), no ghost loops, and no
mock-heavy ratio concerns (the fake is a hand-written substitute, not a mocking framework, and
every subtest that logs calls also asserts on them).

---

## Quality Metrics

**Linter**: Not available (no linter detected in this project's configured tooling)
**Type Checker**: N/A for Go (`go vet ./...` used instead) → Clean, no errors

---

### Issues Found

**CRITICAL**: None

**WARNING**:
1. Manual purge verification on `rabatrepo` (task 3.1) could not observe an actual file deletion:
   the project's PHP container has no `generated/code`, `generated/metadata`, `var/cache`,
   `var/page_cache`, or `var/view_preprocessed` at `/var/www/html`, so the `rm -rf` ran as a no-op
   on both the pre-change and new builds. Exit-code and stdout/`--json` parity between the two
   builds was confirmed, and the literal command string is already pinned by
   `TestWhatPurgeAsks`'s `cmp.Diff`, but the manual harness did not add independent evidence that
   the command actually deletes files on a real filesystem — that residual risk is only covered by
   the (unmodified) integration parity suite against a real container, which this verify pass did
   not re-run.

**SUGGESTION**:
1. `inside_test.go` measured 227 lines against design's ~150-line estimate (six exhaustive table
   cases in `TestWhatTheTestSuitesAsk` plus literate comments, not four) — this is what triggered
   the 1.5 size-gate cut. Future size estimates for table-driven Go tests with this level of
   comment density should budget closer to the observed ratio (~1.5x) rather than the raw case
   count, to avoid a fixed-scope size gate firing mid-implementation.
2. Requirement "A usage error returns before any engine call" still names `version` in its prose
   ("`copy-to-container`, `copy-from-container`, `mysqldump`, and `version` SHALL validate their
   arguments...") even though the `version` scenario was removed by the reconciliation. This is a
   cosmetic leftover in the requirement statement, not a missing scenario — the Note already
   explains the deferral — but a future editorial pass on `spec.md` could drop "and `version`"
   from that sentence to fully match the reconciled scenario list.

### Verdict

PASS WITH WARNINGS — all 13 spec scenarios in the reconciled delta are compliant with passing
runtime evidence, tasks are 13/13 complete, design decisions are followed exactly, scope is clean
(5 files only, nothing outside `internal/cli/`), commit hygiene holds under isolated rebuild, and
every regression runner matches its expected count exactly (44, 204, 612, gofmt clean, go vet
clean). CRITICAL findings: 0. The one WARNING (inconclusive manual deletion evidence on a
`rabatrepo` project with no generated code) is a residual manual-verification gap already covered
by the untouched Docker-backed integration suite, not a regression in this change. The `version`/
`Installed` deferral to `down-and-set-host-go-tests` is now fully reflected in the artifact set
(spec, proposal, tasks, apply-progress all agree), so it no longer registers as scenario-count
incompleteness.
