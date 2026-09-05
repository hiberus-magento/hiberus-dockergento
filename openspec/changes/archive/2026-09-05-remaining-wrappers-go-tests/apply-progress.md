# Apply Progress: What the one-container wrappers, mysqldump and version ask of the engine

## Status: partial (Phase 1 and Phase 2 content complete; both commits pending orchestrator execution; Phase 3 pending manual verification; `version`/`Installed` deferred to `down-and-set-host-go-tests`)

## Deviation from the gate-approved design (report this to the user)

The 1.5 size gate triggered: Commit 1 measured **247 changed lines** (`git diff --shortstat 5e9f443 -- . ':(exclude)openspec'` after `git add -N internal/cli/inside_test.go` → `4 files changed, 245 insertions(+), 2 deletions(-)`), over the 220-line gate. `inside_test.go` came out to 227 lines (six exhaustive table cases in `TestWhatTheTestSuitesAsk` plus literate comments), larger than the ~150-line estimate in tasks.md.

Per the hard constraint's cut order, **`version`/`Installed`/`version.go:23`/`version_test.go` are deferred to the follow-up change `down-and-set-host-go-tests`**. Commit 2 in this change is **mysqldump-only**. This was NOT a mysqldump cut (mysqldump stayed in scope, per the cut order: version first, then mysqldump only if still needed — it was not needed). The `inside` family (purge/npm/n98-magerun/test-unit/test-integration) was never touched or cut. No comment was removed and no code was compressed to fit the budget.

Final total measured after both commits: **359 changed lines** (`5 files changed, 356 insertions(+), 3 deletions(-)`, same command, `inside_test.go` counted via `git add -N`), safely under 400.

## TDD Cycle Evidence

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| `Property` (tests()/wrappers.go:69,77) | `DOCKER_HOST=unix:///nonexistent go test ./internal/cli -short -v -run TestWhatTheTestSuitesAsk` → 7 subtests FAIL: `cmp.Diff` shows `want`'s `Property` entries replaced by an `Exec` entry with `Service:""`/`Command:nil` (the real, unrouted `engine(...).Property` reads a real `fsprops.Reader` for a nonexistent root, answering `""`, so the fallback `./vendor/bin/phpunit` is used instead of the fake's pinned `bin/phpunit`); `TestWhatPurgeAsks`/`TestWhatNpmAndMagerunAsk` PASS unchanged (already routed through `inside`, characterization tests by construction) | Routed `wrappers.go:69,77` from `engine(` to `newEngine(`; `go test ./internal/cli -short -v` → all 39 leaf tests PASS | None needed — matched design exactly |
| `Dump` (dump/wrappers.go:104) | `DOCKER_HOST=unix:///nonexistent go test ./internal/cli -short -v -run TestWhatMysqldumpAsks` → all 3 subtests FAIL: `success` and `--json` exit `3` (`exitDocker`, want `exitOK` — the real, unrouted `engine(...).Dump` reaches `e.database().Ready(project)`, which fails against the unreachable `DOCKER_HOST`); `a_refused_dump_is_reported` FAILS on `cmp.Diff`: `got` has only the `Resolve` call, no `Dump` entry at all (the real `Dump` never reaches the fake); `TestMysqldumpWithNoPathIsRefused` PASSES unchanged (returns before any engine call) | Routed `wrappers.go:104` from `engine(` to `newEngine(`; `go test ./internal/cli -short -v` → all leaf tests PASS, no `DOCKER_HOST` override needed | None needed — matched design exactly |

## Work Unit Evidence

| Evidence | Commit 1 (one-container commands) | Commit 2 (mysqldump) |
|---|---|---|
| Focused test command and exact result | `go test ./internal/cli -short -v -run 'TestWhatPurgeAsks\|TestWhatNpmAndMagerunAsk\|TestWhatTheTestSuitesAsk'` → PASS (9 leaf cases: 1 + 2 + 6) | `go test ./internal/cli -short -v -run 'TestWhatMysqldumpAsks\|TestMysqldumpWithNoPathIsRefused'` → PASS (4 leaf cases: 3 + 1) |
| Runtime harness command/scenario and exact result | `go build -o bin/hm ./cmd/hm` then manual `hm purge` on a running project — **not run in this session** (Phase 3, orchestrator-executed, deferred until after both commits land) | `go build -o bin/hm ./cmd/hm` then manual `hm mysqldump` — **N/A in this change**: `version`'s manual comparison was the Phase 3 harness item for Commit 2, and it is deferred with `version` itself to `down-and-set-host-go-tests`; no manual mysqldump scenario was specified in tasks.md |
| Rollback boundary | Revert `internal/cli/engine.go` (`Property` line only), `internal/cli/wrappers.go:69,77`, `internal/cli/fake_engine_test.go` (`Key`, `properties` field, `Property` method), `internal/cli/inside_test.go` (delete) — self-contained | Revert `internal/cli/engine.go` (`Dump` line only), `internal/cli/wrappers.go:104`, `internal/cli/fake_engine_test.go` (`Path` field, `Dump` method), the mysqldump additions in `internal/cli/wrappers_test.go` (delete `TestWhatMysqldumpAsks`, `TestMysqldumpWithNoPathIsRefused`, and the two added imports) — independent of Commit 1 |

## Verification performed (re-measured this session, not carried over)

Baseline and final counts were both re-measured with `go test ./internal/cli -short -json | rg -c '"Action":"pass","Package":"[^"]*","Test":'` (counts every leaf pass event, parent tests and subtests alike), using `git stash push -u` / `git stash pop` around the baseline measurement to confirm it against the true pre-change tree (`5e9f443`), not assumed from the prompt.

- `go test ./internal/cli -short` leaf passes: **28 (baseline, re-confirmed) → 44 (final) = +16**. Breakdown: `TestWhatPurgeAsks` (1) + `TestWhatNpmAndMagerunAsk` (1 parent + 2 subtests = 3) + `TestWhatTheTestSuitesAsk` (1 parent + 6 subtests = 7) + `TestWhatMysqldumpAsks` (1 parent + 3 subtests = 4) + `TestMysqldumpWithNoPathIsRefused` (1) = 16. All PASS, zero red.
- `go test ./... -short` leaf passes: **188 (baseline, re-confirmed) → 204 (final) = +16**, same delta, confirming no other package was touched.
- `gofmt -l ./cmd ./internal`: empty (both after Commit 1 and after Commit 2).
- `go vet ./...`: clean (both after Commit 1 and after Commit 2).
- `tests/run.sh unit`: 612 assertions passed, unmodified (Bash side untouched, as expected — this change is Go-test-only).
- `var _ commands = (*dockergento.Engine)(nil)` still compiles (implicit in every `go build ./...` run above).
- Size: `git diff --shortstat 5e9f443 -- . ':(exclude)openspec'` (with `git add -N internal/cli/inside_test.go`) → `5 files changed, 356 insertions(+), 3 deletions(-)` = 359 total, under 400.

## Commit-split verification (mandatory given both commits touch `engine.go`, `fake_engine_test.go`, `wrappers.go`)

Because `engine.go`'s two-line interface addition (`Property` then `Dump`) and `fake_engine_test.go`'s two adjoining blocks (the `call` struct's `Key`/`Path` fields, and the `Property`/`Dump` method pair) are single-line-adjacent, plain `git add -p` cannot auto-split them into separate hunks per commit — this was verified directly in this session:

1. Applied three exact partial patches to the git INDEX only (`git apply --cached`) that stage exactly Commit 1's intended lines in `engine.go` (`Property` only), `fake_engine_test.go` (`Key` field, `properties` field+comment, `Property` method only), and `wrappers.go` (lines 69 and 77 only — these two are already clean, separate hunks, no manual split needed), plus `git add internal/cli/inside_test.go` as a whole new file.
2. Confirmed `git diff --cached --stat` at that point matched the original Commit-1-alone measurement exactly: `4 files changed, 245 insertions(+), 2 deletions(-)`.
3. Stashed the remaining (Commit 2) working-tree changes with `git stash push --keep-index -u`, leaving the working tree at exactly the Commit-1 split state, and confirmed `go build ./...` and `go test ./internal/cli -short -v` (28 → 39 tests) both pass in that isolated state.
4. Restored the stash. It produced a benign 3-way merge conflict in `engine.go` and `fake_engine_test.go` (expected: the stash's base assumption is HEAD, not the already-partially-staged working tree) — resolved by hand back to the known-good combined content (both lines/methods present), verified byte-for-byte against the pre-stash file content, no conflict markers remain (`grep -n '<<<<<<<\|=======\|>>>>>>>' internal/cli/engine.go internal/cli/fake_engine_test.go internal/cli/wrappers.go internal/cli/wrappers_test.go` → no matches).
5. `git reset` (unstaged everything back to plain working-tree edits) and re-ran the full final verification battery above — all green, size back to 359.
6. Dropped only the verification stash (`stash@{0}`, message `commit2-part-verify`); the unrelated pre-existing stash from `main` (`stash@{1}`) was left untouched.

**Exact commands for the orchestrator to split and commit** (see the exact patch text embedded in the return envelope's "two commit file/hunk lists" section — apply verbatim with `git apply --cached <patchfile>`, then `git add internal/cli/inside_test.go`, then commit; the remaining unstaged diff becomes Commit 2 automatically, needing only `git add internal/cli/engine.go internal/cli/fake_engine_test.go internal/cli/wrappers.go internal/cli/wrappers_test.go` before the second commit).

## Tasks completed (this session)

- [x] 1.1, 1.2, 1.3, 1.4, 1.5 (Phase 1, full RED→GREEN→regression→size-gate)
- [x] 2.1, 2.2, 2.4, 2.5 (Phase 2, mysqldump RED→GREEN→regression)
- [x] 2.3 marked as explicitly deferred (not implemented in this change — moved to `down-and-set-host-go-tests`)
- [ ] 1.6, 2.6 — commits, orchestrator-executed, NOT done by this agent
- [ ] 3.1 — manual verification, orchestrator-executed, NOT done by this agent (and rescoped: no `version` comparison in this change)

## Files changed (working tree, uncommitted)

- `internal/cli/engine.go` — `Property`, `Dump` added to `commands` interface (2 lines)
- `internal/cli/fake_engine_test.go` — `Key`, `Path` fields on `call`; `properties` field + `Property` method; `Dump` method (25 lines)
- `internal/cli/inside_test.go` — new file, 227 lines (`TestWhatPurgeAsks`, `TestWhatNpmAndMagerunAsk`, `TestWhatTheTestSuitesAsk`)
- `internal/cli/wrappers.go` — 3 call sites routed (`:69`, `:77`, `:104`) from `engine(` to `newEngine(`
- `internal/cli/wrappers_test.go` — `TestWhatMysqldumpAsks`, `TestMysqldumpWithNoPathIsRefused` appended (99 lines incl. 2 new imports)

## Next recommended

`sdd-apply` again is not needed for this change's own scope (Phase 1 and Phase 2 content are done); the two commits and Phase 3 manual verification are orchestrator-executed next steps within this same change. `version`/`Installed` work belongs to a fresh `sdd-tasks`/`sdd-apply` cycle on `down-and-set-host-go-tests`. After both commits land and Phase 3 passes, `sdd-verify` is the next phase for this change.

## Delivery record (orchestrator)

- Commit 1 `aba8cd6` — `test(cli): what the one-container commands ask of the engine`: `internal/cli/engine.go` (Property), `internal/cli/fake_engine_test.go` (Key, properties, Property), `internal/cli/inside_test.go` (new), `internal/cli/wrappers.go` (:69, :77). Staged with `git apply --cached --recount` from the partial patch below, because the Property and Dump additions are line-adjacent in `engine.go` and `fake_engine_test.go`. `4 files changed, 245 insertions(+), 2 deletions(-)`. In isolation: `go build ./...` ok, `go test ./internal/cli -short` 39 passed.
- Commit 2 `5e3b1af` — `test(cli): what mysqldump asks of the engine`: the remaining diff (Dump on the interface and the fake, `call.Path`, `wrappers.go:104`, mysqldump tests in `wrappers_test.go`). `4 files changed, 111 insertions(+), 1 deletion(-)`.
- Total `git diff --shortstat 5e9f443..5e3b1af -- . ':(exclude)openspec'`: `5 files changed, 356 insertions(+), 3 deletions(-)` = 359, under the 400 budget with no exception.

The commit-1 partial patch (engine.go Property line; fake_engine_test.go `Key` field, `properties` field with its comment, `Property` method; wrappers.go :69 and :77 `engine(` → `newEngine(`) is reproducible from `git show aba8cd6 -- internal/cli/engine.go internal/cli/fake_engine_test.go internal/cli/wrappers.go`.

## Manual verification record (task 3.1)

Run by the orchestrator on 2026-09-05 with the user's authorization against `rabatrepo`. The project's containers had exited (255) after a Docker restart, so the first attempt exercised only the refusal path: both `bin/hm` (5e3b1af) and `hm-pre` (5e9f443) answered `docker_failed: service "phpfpm" is not running`, exit 3, byte-identical in text and `--json`. With the user's go-ahead the project was started (`HM_NON_INTERACTIVE=1 hm start`, exit 0, nine services up) and left running, which is how the user had it before the restart.

| Command | Result |
|---|---|
| `hm purge` (pre, then new) | exit 0 on both; identical stdout. |
| `hm --json purge` (pre, then new) | exit 0 on both; identical document. |
| `generated/code`, `generated/metadata`, `var/cache`, `var/page_cache`, `var/view_preprocessed` in the php container | absent before and after on both runs: this project's layout has no generated code at `/var/www/html`, so the `rm -rf` was a no-op there. Parity of the command and its exit is proven; an observable deletion was not available on this project. |

`version` was cut from this change by the size gate, so no `hm version` comparison belongs to this record; it moves with `version` to `down-and-set-host-go-tests`.
