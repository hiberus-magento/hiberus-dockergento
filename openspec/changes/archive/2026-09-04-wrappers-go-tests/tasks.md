# Tasks: A test seam for the wrappers

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ≈315 (`engine.go` ≈20, `wrappers.go` 6, `php.go` 10, `fake_engine_test.go` ≈80, `wrappers_test.go` ≈190, `migration_status_test.sh` 8, `MIGRATION.md` 2), excluding `openspec/changes/` |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single delivery, three work-unit commits |
| Delivery strategy | single-pr (no open PRs in this project — direct commit onto `release/2.0.0`) |
| Chain strategy | pending (estimate is well under budget, no chaining decision needed) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Seam + fake + 7 wrapper tests, RED→GREEN, MIGRATION.md bullet | Commit 1 (direct to `release/2.0.0`) | `go test ./internal/cli -short -v` | `go build -o bin/hm ./cmd/hm` then manual `copy-to-container`/`copy-from-container`/`varnish-on`/`varnish-off` against a real project (Phase 4) | Revert `internal/cli/engine.go` routing, `internal/cli/wrappers.go`, `internal/cli/php.go` routing, `internal/cli/fake_engine_test.go`, `internal/cli/wrappers_test.go`, the MIGRATION.md bullet — self-contained, no other file depends on the seam |
| 2 | Correct the `mirrorsVendor` comment | Commit 2 (direct to `release/2.0.0`) | `go build ./... && go vet ./...` (comment-only, nothing asserts on comment text) | N/A — comment-only edit, no runtime path changes | Revert `internal/cli/php.go:91-97`; independent of Commit 1 |
| 3 | Here-strings replace the two `grep -qx` pipes | Commit 3 (direct to `release/2.0.0`) | `tests/run.sh unit` | N/A — shell test script reading two files in its own repo, no routing/subprocess/executable-classification change (design threat matrix) | Revert `tests/unit/migration_status_test.sh:34,42`; independent of Commits 1–2 |

## Phase 1: Commit 1 — the seam, the fake, and the seven tests (RED → GREEN)

- [x] 1.1 In `internal/cli/engine.go`, add the `commands` interface (`Resolve`, `Exec`, `Restart`, `CopyInto`, `CopyFrom` — signatures from `dockergento/dockergento.go:129,271,293,508,529` (read-only)), `var newEngine = func(stdout, stderr io.Writer, jsonOutput bool) commands { return engine(stdout, stderr, jsonOutput) }`, and `var _ commands = (*dockergento.Engine)(nil)`. Route ONLY `projectOr` (`internal/cli/engine.go:58`) through `newEngine`; leave `internal/cli/wrappers.go:150,219,244` (read-only at this step) and `internal/cli/php.go:59` (read-only at this step) calling `engine()` directly.
  Verify: `go build ./...` succeeds — package compiles, no other call site moved yet.
- [x] 1.2 Create `internal/cli/fake_engine_test.go`: `call` and `outcome{status, err}` types, `fakeEngine` with an ordered `calls []call` log and `outcomes []outcome` indexed by call number (past-the-end = success), implementing `commands`. Add the `answering` helper: pins `HM_STATE_DIR` to `t.TempDir()` via `t.Setenv`, installs the fake into `newEngine`, restores the original with `t.Cleanup`.
  Verify: `gofmt -l internal/cli/fake_engine_test.go` is empty.
- [x] 1.3 RED: Create `internal/cli/wrappers_test.go` with the seven functions from design's Testing Strategy table: `TestWhatCopyingIntoTheContainerAsks` (named paths; `--all` first; `--all` not first — `all` stays false, both still passed as paths), `TestWhatCopyingOutOfTheContainerAsks` (paths as given, `Dir` = `here()`), `TestCopyingWithNoPathIsRefused` (into and out of: `exitUsage`, empty call log), `TestACopyTheEngineRefusesIsReported` (`CopyInto`/`CopyFrom` error → `exitDocker`), `TestWhatVarnishAsks` (on: Resolve, `sed` on `varnish` as root, Restart, `cache:enable full_page`; off: same with `cache:disable`, then Resolve + `rm -rf` (purge) and Resolve + `cache:clean`), `TestAFailedEditStopsBeforeTheRestart` (outcome 2 fails → log ends at the `sed`), `TestOutsideAProjectNothingIsAsked` (empty `core.Project` → `exitProject`, log is `Resolve` alone). Compare each fake's whole `calls` log against a literal `want []call` with `cmp.Diff`; ignore `core.ExecOptions.Tty` via `cmpopts.IgnoreFields`. Varnish assertions carry the literal `#skip-varnish` sed expressions.
  Verify RED: `DOCKER_HOST=unix:///nonexistent go test ./internal/cli -short -v -run 'TestWhatCopyingIntoTheContainerAsks|TestWhatCopyingOutOfTheContainerAsks|TestWhatVarnishAsks|TestACopyTheEngineRefusesIsReported|TestAFailedEditStopsBeforeTheRestart'` → **FAILS**: `cmp.Diff` reports `got` holding `Resolve` alone (the only routed call site) against a `want` holding the full choreography, because the four still-unrouted handlers call the real `engine()` and bypass the fake entirely. `DOCKER_HOST=unix:///nonexistent` makes that bypassed real call fail immediately on a nonexistent socket instead of reaching a live daemon or hanging — both the Docker SDK's `client.FromEnv` (`dockergento/adapters/dockerd/daemon.go:128`, read-only) and this repo's own `Endpoint()` (`dockergento/adapters/dockerd/endpoint.go:22`, read-only) read `DOCKER_HOST` first, confirmed by reading both files, so the override is honoured deterministically on any machine, with or without a real daemon. `TestCopyingWithNoPathIsRefused` and `TestOutsideAProjectNothingIsAsked` already pass at this step — both return before reaching any unrouted call site — so they are intentionally excluded from the `-run` filter above; running them is optional but harmless.
- [x] 1.4 GREEN: Route the four remaining call sites through `newEngine` — `internal/cli/wrappers.go:150` (`varnish`), `:219` (`copyInto`), `:244` (`copyFrom`), `internal/cli/php.go:59` (`inside`).
  Verify GREEN: `go test ./internal/cli -short -v` → **PASSES**, all seven new tests plus the package's existing tests (`select_test.go`, `output_test.go`, `php_test.go`), no `DOCKER_HOST` override needed since the fake fully intercepts.
- [x] 1.5 In `MIGRATION.md`'s "Cómo se escriben" section (lines 72-90), add one bullet stating that a Go-wired command's engine interaction is tested against `newEngine`, not Docker.
  Verify: manual read — bullet present, in Spanish, matching the section's existing bullet voice (bold lead phrase, then explanation).
- [x] 1.6 Regression: `go test ./... -short` (baseline 172 tests / 22 packages, expect 179 tests, all green); `tests/run.sh unit`; `gofmt -l ./cmd ./internal` empty; `go vet ./...` clean.
  Verify: all four commands pass; no diff to any Bash parity suite.
- [x] 1.7 Commit: `test(cli): what the copy and varnish commands ask of the engine`.
  Files: `internal/cli/engine.go`, `internal/cli/wrappers.go`, `internal/cli/php.go`, `internal/cli/fake_engine_test.go`, `internal/cli/wrappers_test.go`, `MIGRATION.md`.

## Phase 2: Commit 2 — the comment names its real reason

- [x] 2.1 Rewrite the comment above `mirrorsVendor` in `internal/cli/php.go:91-97`: state that the vendor-mirror flow gated by `mirrorsVendor` (macOS-only copy-in/run/copy-back around Composer) is what stays in shell, not that `copy-to-container` is unported — `copy-to-container` is wired to `copyInto` in `run.go` (read-only cross-check).
  Verify: manual read — no remaining claim that `copy-to-container` is unported; `mirrorsVendor` named as the real reason.
- [x] 2.2 Regression: `go build ./... && go vet ./... && gofmt -l ./cmd ./internal` (comment-only change, nothing asserts on comment text).
  Verify: build clean, vet clean, gofmt empty.
- [x] 2.3 Commit: `docs(cli): say why the mac Composer flow is still shell`.
  Files: `internal/cli/php.go`.

## Phase 3: Commit 3 — the pipe that could lose to a signal

- [x] 3.1 In `tests/unit/migration_status_test.sh:34`, replace `printf '%s\n' "$listed" | grep -qx "$command"` with a here-string form (`grep -qx "$command" <<< "$listed"`), following the pattern already used at `:101` (read-only reference).
- [x] 3.2 In `tests/unit/migration_status_test.sh:42`, replace `commands | grep -qx "$command"` with the equivalent here-string form, same pattern.
  Verify: `tests/run.sh unit` (or standalone `bash tests/unit/migration_status_test.sh`) → last line `RESULT 12 0` — same 12 assertions, 0 failures, as before the fix; this is a robustness fix, not a new assertion.
- [x] 3.3 Commit: `test(migration): read the command lists without a broken pipe`.
  Files: `tests/unit/migration_status_test.sh`.

## Phase 4: Manual verification (real project)

- [x] 4.1 Build the binary (`go build -o bin/hm ./cmd/hm`) and, against a real Dockergento project with containers running, run `hm copy-to-container <path>`, `hm copy-to-container --all`, `hm copy-from-container <path>`, `hm varnish-on`, `hm varnish-off` in sequence. Confirm exit codes, `--json` documents, and observable behavior (VCL edit, varnish restart, cache toggle, and `varnish-off`'s purge + `cache:clean`) are unchanged from before this change — the seam is behaviour-preserving by construction (same object built by the same function, one wider static type at the call site), and this is the check that proves it against Docker, not the fake.
  Verify: manual — all five commands behave exactly as on `release/2.0.0` before this change.

## Notes

- Out of scope, no tasks added: `internal/cli/wrappers.go:155,166`'s `report(nil err)` defect (read-only), the `here()` vs `project.Root` inconsistency, the nine other commands deferred to `remaining-wrappers-go-tests`, any Bash deletion.
- If Commit 1 threatens the 400-line budget, cut in this order (design's Size and Commits): merge the two varnish failure tests into one table, then drop the `--all`-not-first case, then move `copy-from-container`'s test to a second slice. Never cut by deleting comments.
- No file under `console/`, `bin/run`, or `dockergento/` (read-only areas for this change) is modified — proposal Non-goals, design File Changes table.
