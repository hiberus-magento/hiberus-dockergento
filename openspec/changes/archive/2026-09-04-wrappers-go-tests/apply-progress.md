# Apply Progress: A test seam for the wrappers

Change: `wrappers-go-tests`. Mode: **Strict TDD**. Artifact store: hybrid (file + Engram).

## Completed Tasks (10/14)

- [x] 1.1 `internal/cli/engine.go`: `commands` interface, `var newEngine`, `var _ commands = (*dockergento.Engine)(nil)`, `projectOr` routed
- [x] 1.2 `internal/cli/fake_engine_test.go`: `call`, `outcome{status, err}`, `fakeEngine`, `answering` helper
- [x] 1.3 RED: `internal/cli/wrappers_test.go`, 7 test functions, routing only `projectOr`
- [x] 1.4 GREEN: routed `wrappers.go:150,219,244` and `php.go:59` through `newEngine`
- [x] 1.5 `MIGRATION.md` bullet added to "Cómo se escriben"
- [x] 1.6 Regression: `go test ./... -short`, `tests/run.sh unit`, `gofmt -l`, `go vet ./...` all clean
- [x] 2.1 `internal/cli/php.go:91-99` comment rewritten — names `mirrorsVendor`'s copy-in/run/copy-back choreography as what stays in shell, not `copy-to-container` (wired to `copyInto`)
- [x] 2.2 Regression: `go build ./... && go vet ./... && gofmt -l ./cmd ./internal` clean
- [x] 3.1 `tests/unit/migration_status_test.sh:34` → here-string
- [x] 3.2 `tests/unit/migration_status_test.sh:42` → here-string, `bash tests/unit/migration_status_test.sh` → `RESULT 12 0`

## Pending Tasks (4/14 — deliberately left to orchestrator/manual)

- [ ] 1.7 Commit 1 (orchestrator executes; do NOT commit here)
- [ ] 2.3 Commit 2 (orchestrator executes; do NOT commit here)
- [ ] 3.3 Commit 3 (orchestrator executes; do NOT commit here)
- [ ] 4.1 Manual verification against a real Dockergento project — no live project/Docker daemon available in this apply session; deferred, not a commit task

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1–1.4 | `internal/cli/wrappers_test.go`, `internal/cli/fake_engine_test.go` | Unit | ✅ package pre-change: `go build ./...` clean, existing `select_test.go`/`output_test.go`/`php_test.go` passing before edits | ✅ Written (7 funcs, all referencing production behaviour not yet routed) | ✅ Passed (28/28 in `internal/cli` after routing) | ✅ 3 cases in `TestWhatCopyingIntoTheContainerAsks` (named path / `--all` first / `--all` not first), 2 cases in `TestWhatVarnishAsks` (on/off), 2 cases each in `TestCopyingWithNoPathIsRefused` and `TestACopyTheEngineRefusesIsReported` | ➖ None needed — production code is 5 one-line call-site substitutions, already minimal |
| 1.5 | N/A (doc) | N/A | N/A (new content) | N/A — doc-only, no test | N/A | Triangulation skipped: purely structural, one Spanish bullet in the existing voice | N/A |
| 2.1 | `internal/cli/php_test.go` (pre-existing, unmodified) | N/A | ✅ 3/3 (`TestTheInvocationsThatRewriteTheHostsTree`, `TestEverythingElseRunsInTheContainer`, `TestOnlyTheFourWriteDependencies`) still pass — comment-only edit, no assertion on comment text | N/A — comment-only, no behaviour, no test possible | N/A | Triangulation skipped: comment-only, no branching | ➖ None needed |
| 3.1–3.2 | `tests/unit/migration_status_test.sh` (pre-existing Bash suite, unmodified as a test — production shell edited) | Shell/Bash | ✅ baseline `bash tests/unit/migration_status_test.sh` → `RESULT 12 0` before the fix | N/A — approval-testing style: existing suite is the safety net, no new assertion added per design (robustness fix, not new behaviour) | ✅ `RESULT 12 0` after the fix — same 12 assertions, 0 failures | Triangulation skipped: mechanical substitution of an already-proven pattern (line 101) | ➖ None needed |

### RED excerpt (task 1.3)

Command: `DOCKER_HOST=unix:///nonexistent go test ./internal/cli -short -v -run 'TestWhatCopyingIntoTheContainerAsks|TestWhatCopyingOutOfTheContainerAsks|TestWhatVarnishAsks|TestACopyTheEngineRefusesIsReported|TestAFailedEditStopsBeforeTheRestart'`

```
Go test: 0 passed, 12 failed in 1 packages
  [FAIL] TestWhatCopyingIntoTheContainerAsks/a_named_path
     wrappers_test.go:46: copy-to-container [app/code] = 3, want exitOK
  [FAIL] TestACopyTheEngineRefusesIsReported/copy-to-container
     wrappers_test.go:162: asked of the engine (-want +got):
     []cli.call{
  [FAIL] TestAFailedEditStopsBeforeTheRestart
     wrappers_test.go:276: asked of the engine after a failed edit (-want +got):
     []cli.call{
     	{Method: "Resolve", Dir: "/Users/ddelgado/hm/internal/cli"},
     -	{
     -		Method:  "Exec",
     -		Dir:     "/code/shop",
     -		Service: "varnish",
     -		...
     -	},
     }
```

All 12 sub-tests across the 5 targeted functions failed: the `code != exitOK`/`exitDocker` checks failed with `3` (real `exitDocker`, unrouted `engine()` hit `DOCKER_HOST=unix:///nonexistent` and errored), and the two `cmp.Diff`-based tests (`TestACopyTheEngineRefusesIsReported`, `TestAFailedEditStopsBeforeTheRestart`) showed `got` holding `Resolve` alone against a `want` with the full choreography, confirming the fake was bypassed. Note: `TestACopyTheEngineRefusesIsReported` initially had no call-log assertion and passed trivially in RED (a real Docker failure coincidentally also returns `exitDocker`) — strengthened with a `cmp.Diff` on `fake.calls` so it is a real RED/GREEN test, not a false positive from exit-code coincidence.

`TestCopyingWithNoPathIsRefused` and `TestOutsideAProjectNothingIsAsked` were excluded from the RED `-run` filter by design (they return before any unrouted call site) and were confirmed passing at this step: `go test ./internal/cli -short -v -run 'TestCopyingWithNoPathIsRefused|TestOutsideAProjectNothingIsAsked'` → `4 passed`.

### GREEN excerpt (task 1.4)

Command: `go test ./internal/cli -short -v` (via `rtk proxy` for unfiltered output)

```
=== RUN   TestWhatCopyingIntoTheContainerAsks
--- PASS: TestWhatCopyingIntoTheContainerAsks (0.01s)
    --- PASS: TestWhatCopyingIntoTheContainerAsks/a_named_path (0.00s)
    --- PASS: TestWhatCopyingIntoTheContainerAsks/--all_first (0.00s)
    --- PASS: TestWhatCopyingIntoTheContainerAsks/--all_not_first (0.00s)
=== RUN   TestWhatVarnishAsks
--- PASS: TestWhatVarnishAsks (0.00s)
    --- PASS: TestWhatVarnishAsks/on (0.00s)
    --- PASS: TestWhatVarnishAsks/off (0.00s)
=== RUN   TestAFailedEditStopsBeforeTheRestart
--- PASS: TestAFailedEditStopsBeforeTheRestart (0.00s)
=== RUN   TestOutsideAProjectNothingIsAsked
--- PASS: TestOutsideAProjectNothingIsAsked (0.00s)
PASS
ok  	github.com/hiberus-magento/hiberus-dockergento/internal/cli	0.658s
```

28/28 tests passed in `internal/cli` (7 new + 21 pre-existing), no `DOCKER_HOST` override needed — the fake fully intercepts once all five call sites are routed.

## Work Unit Evidence

| Evidence | Unit 1 (Commit 1) | Unit 2 (Commit 2) | Unit 3 (Commit 3) |
|---|---|---|---|
| Focused test command and exact result | `go test ./internal/cli -short -v` → 28 passed | `go build ./... && go vet ./... && gofmt -l ./cmd ./internal` → all clean, no output | `bash tests/unit/migration_status_test.sh` → `RESULT 12 0` |
| Runtime harness command/scenario and exact result | `go build -o bin/hm ./cmd/hm` succeeds (verified via `go build ./...`); manual `copy-to-container`/`copy-from-container`/`varnish-on`/`varnish-off` against a real project is task 4.1, **not run** — no live Docker daemon/project in this apply session | N/A — comment-only, no runtime path changes | N/A — shell test script reading its own repo, no routing/subprocess change |
| Rollback boundary | Revert `internal/cli/engine.go`, `internal/cli/wrappers.go`, `internal/cli/php.go`'s `inside` routing hunk only, `internal/cli/fake_engine_test.go`, `internal/cli/wrappers_test.go`, MIGRATION.md bullet — self-contained | Revert `internal/cli/php.go:96-98` comment hunk only — independent of Unit 1's routing hunk in the same file | Revert `tests/unit/migration_status_test.sh:34,42` — independent of Units 1–2 |

## Regression (full sweep, all three work units applied together)

- `go build ./...` → clean
- `go test ./... -short` → all packages `ok` (`dockergento`, its adapters, `dockergento/app`, `dockergento/core`, `internal/cli`, `test/e2e`); observed `188 passed in 22 packages`; `internal/cli` at 28 tests (was ~21 before this change)
- `tests/run.sh unit` → `612 assertions passed`, includes `migration_status_test.sh` `RESULT 12 0`
- `gofmt -l ./cmd ./internal` → empty (clean)
- `go vet ./...` → clean

## Files Changed

| File | Action | What Was Done |
|------|--------|----------------|
| `internal/cli/engine.go` | Modified | Added `commands` interface, `var _ commands = (*dockergento.Engine)(nil)`, `var newEngine`; routed `projectOr` |
| `internal/cli/wrappers.go` | Modified | Routed `varnish`, `copyInto`, `copyFrom` call sites through `newEngine` |
| `internal/cli/php.go` | Modified | Routed `inside` through `newEngine` (Unit 1 hunk); rewrote `mirrorsVendor` comment (Unit 2 hunk, non-overlapping) |
| `internal/cli/fake_engine_test.go` | Created | `call`, `outcome`, `fakeEngine`, `answering` test-only helper |
| `internal/cli/wrappers_test.go` | Created | 7 test functions covering copy-into/out, usage errors, engine refusals, varnish on/off + failure, outside-a-project |
| `MIGRATION.md` | Modified | One Spanish bullet in "Cómo se escriben" |
| `tests/unit/migration_status_test.sh` | Modified | Two `grep -qx` pipes → here-strings (lines 34, 42) |

## Deviations from Design

None — implementation matches design. One test strengthened beyond the task's literal `-run` expectation: `TestACopyTheEngineRefusesIsReported` gained a `cmp.Diff` call-log assertion (design's testing-strategy table only names the exit-code scenario) because without it the test passed trivially in RED — a coincidental `exitDocker` from the real unrouted engine hitting `DOCKER_HOST=unix:///nonexistent`, not real fake substitution. This is a strengthening, not a scope change: same handlers, same scenarios, same file.

## Issues Found

None beyond the two already-documented Open Questions in design.md (`report(nil err)` defect at `wrappers.go:155,166`, and the `here()` vs `project.Root` inconsistency) — both explicitly out of scope and untouched.

## Remaining Tasks

- [ ] 1.7 Commit 1 — `test(cli): what the copy and varnish commands ask of the engine` (orchestrator)
- [ ] 2.3 Commit 2 — `docs(cli): say why the mac Composer flow is still shell` (orchestrator)
- [ ] 3.3 Commit 3 — `test(migration): read the command lists without a broken pipe` (orchestrator)
- [ ] 4.1 Manual verification against a real Dockergento project (deferred — no live project available in this session)

## Workload / PR Boundary

- Mode: single PR, three work-unit commits, direct to `release/2.0.0`
- Estimated review budget impact: 457 changed lines total (`git diff --shortstat` on the 5 modified tracked files: 32 insertions + 9 deletions = 41, plus `internal/cli/fake_engine_test.go` 117 + `internal/cli/wrappers_test.go` 299 = 416; 41 + 416 = 457), over the 400-line budget. The user accepted a `size:exception` for this single-PR delivery rather than chaining. Applying the design's full cut order (merge the two varnish failure tests into one table ≈18 lines, drop the `--all`-not-first case ≈1 line, move `copy-from-container`'s test to a second slice ≈35 lines) would recover only ≈54 lines — not enough to land under 400 without cutting into behaviour coverage — so the exception was taken instead of a partial, less-legible cut.

## Status

10/14 tasks complete. Ready for orchestrator to execute the three commits (1.7, 2.3, 3.3) and, separately, for manual verification (4.1) against a real project before/after delivery.

## Manual verification record (task 4.1)

Run by the orchestrator on 2026-09-04 with the user's authorization, against the only environment with containers running: `rabatrepo` (mac, volume mode, `/var/www/html` is a volume; `app/`, `config/`, `composer.*` and others are bind mounts). Two binaries were compared: `bin/hm` built from HEAD (1dad472) and `hm-pre` built from 21559ab, the commit before this change.

| Command | Result |
|---|---|
| `hm --json copy-to-container .gitignore` | Identical JSON and exit 0 on both binaries. `.gitignore` chosen because it exists on the host and not in the container; removed from the container afterwards. |
| `hm --json copy-from-container .gitignore` | Identical JSON and exit 0 on both; host file unchanged (same md5). |
| `hm --json copy-to-container` / `copy-from-container` (no path) | Byte-identical JSON on both, exit 2, `missing_path`. |
| `hm --json copy-to-container composer.json` | Both refuse with code 6 `path_is_bound` (bind mount in this project). |
| `hm --json varnish-off` then `varnish-on` | Both binaries exit 3 at the `bin/magento cache:*` step because this project has no `bin/magento` under `/var/www/html`. The VCL edit and the Varnish restart happen on both; the purge + `cache:clean` cascade does not. VCL left as found (`#return(pass)`, Varnish on). |
| `hm copy-to-container --all` | Not run: `pub/media` is 11 GB. No automated Docker test covers `--all` either (`test/e2e/transfer_test.go` copies directories and files by path). Documented gap; the unit test covers the handler → `CopyInto(all=true)` mapping only. |
| `go test ./test/e2e -run 'TestCopy…'` | PASS against Docker with the new binary (21 s): directory and file copies in and out, bind-mount refusal, three refusals. |

Observations outside this change's scope:

- A `chown … Operation not permitted` line on stderr appears only when the binary's install root resolves `data/properties.json` (`USER_PHP=app`); a binary built outside the tree skips `own()`. Same on both binaries once both run from `bin/`. Not a seam regression.
- `hm copy-to-container --json` (global flag after the subcommand) is treated as a path to copy on both binaries: pre-existing UX defect.
- `hm list` shows three stale `hm-e2e-copias-*` registrations pointing at deleted temp dirs, left by an earlier harness; this run added none.
