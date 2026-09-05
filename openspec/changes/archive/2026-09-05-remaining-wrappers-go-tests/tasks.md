# Tasks: What the one-container wrappers, mysqldump and version ask of the engine

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ≈355 (`inside_test.go` new ~150, `version_test.go` new ~95, `wrappers_test.go` +~62, `fake_engine_test.go` +~34, `wrappers.go` ~6, `engine.go` ~4, `version.go` ~2), excluding `openspec/changes/` |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single delivery, two work-unit commits; overflow moves to the already-named follow-up `down-and-set-host-go-tests`, not a chained PR |
| Delivery strategy | single-pr (no open PRs in this project — direct commit onto `release/2.0.0`); **no `size:exception` this cycle** |
| Chain strategy | pending (the numeric cut order is the overflow valve, not PR chaining) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | `Property` on the interface, fake growth, `inside_test.go`'s five commands, RED→GREEN, `wrappers.go:69,77` routed | Commit 1 (direct to `release/2.0.0`) | `go test ./internal/cli -short -v -run 'TestWhatPurgeAsks\|TestWhatNpmAndMagerunAsk\|TestWhatTheTestSuitesAsk'` | `go build -o bin/hm ./cmd/hm` then manual `hm purge` on a running project (Phase 3) | Revert `internal/cli/engine.go` (`Property`), `internal/cli/wrappers.go:69,77`, `internal/cli/fake_engine_test.go` (`Property` growth), `internal/cli/inside_test.go` — self-contained, nothing else depends on it |
| 2 | `Dump`/`Installed` on the interface, fake growth, mysqldump tests in `wrappers_test.go`, `version_test.go`, `wrappers.go:104` and `version.go:23` routed | Commit 2 (direct to `release/2.0.0`) | `go test ./internal/cli -short -v -run 'TestWhatMysqldumpAsks\|TestMysqldumpWithNoPathIsRefused\|TestWhatVersionReports\|TestAnOptionNobodyDeclaredIsAUsageError'` | `go build -o bin/hm ./cmd/hm` then manual `hm version` and `hm version --json` (Phase 3) | Revert `internal/cli/engine.go` (`Dump`, `Installed`), `internal/cli/wrappers.go:104`, `internal/cli/version.go:23`, `internal/cli/fake_engine_test.go` (`Dump`/`Installed` growth), the mysqldump additions in `wrappers_test.go`, `internal/cli/version_test.go` — independent of Commit 1 |

## Phase 1: Commit 1 — the one-container commands (RED → GREEN)

- [x] 1.1 RED (interface + fake): in `internal/cli/engine.go`, extend `commands` with `Property(project core.Project, key string) string` (`var _ commands = (*dockergento.Engine)(nil)` stays compiling — production's `dockergento.go:1239` already has it). In `internal/cli/fake_engine_test.go`, add `properties map[string]string` to `fakeEngine`, add `Key string` to `call`, and implement `Property` logging `{Method: "Property", Dir: project.Root, Key: key}` and returning `f.properties[key]` (missing key answers `""`, driving the fallbacks).
  Verify: `gofmt -l internal/cli/fake_engine_test.go` empty; `go build ./...` succeeds — interface grows, no call site routed yet.
- [x] 1.2 RED (tests): create `internal/cli/inside_test.go` (~150 lines) with `TestWhatPurgeAsks` (the seven `generated` directories written literally, `sh -c "rm -rf ..."` through `inside`), `TestWhatNpmAndMagerunAsk` (npm argv passthrough vs magerun's `bash -c "n98-magerun ..."` join), `TestWhatTheTestSuitesAsk` (table over `BIN_DIR=bin` vs `""`→`./vendor/bin` fallback, `WORKDIR_PHP=/app` vs `""`→"/var/www/html" fallback with the doubled "/var/www/html/./vendor/bin/phpunit" pinned and commented, appended args). `want` values come from the parity suite (`tests/integration/go_wrappers_test.sh`), never from observed output; each case compares the whole `calls` log via `cmp.Diff`.
  Verify RED: `DOCKER_HOST=unix:///nonexistent go test ./internal/cli -short -v -run TestWhatTheTestSuitesAsk` → **FAILS**: `cmp.Diff` shows `want`'s two `Property` entries absent from `got` — `wrappers.go:69,77` still call `engine(` directly, bypassing the fake — and the command string mismatches because the real engine's `fsprops.Reader` answers `""` for a nonexistent root, driving `./vendor/bin/phpunit` instead of the fake's pinned `bin/phpunit`. `TestWhatPurgeAsks` and `TestWhatNpmAndMagerunAsk` already PASS here — purge/npm/magerun reach the container only through `inside` (`php.go:57`), already routed through `newEngine` by the predecessor change — so they are characterization tests, not RED, by construction.
- [x] 1.3 GREEN: route `wrappers.go:69` and `:77` (the two `Property` calls inside `tests()`) from `engine(` to `newEngine(`.
  Verify GREEN: `go test ./internal/cli -short -v` → **PASSES** — all of `inside_test.go` plus every pre-existing test in the package (`select_test.go`, `output_test.go`, `php_test.go`, `wrappers_test.go`).
- [x] 1.4 Regression: `go test ./... -short` (188 baseline, all green, no new red); `gofmt -l ./cmd ./internal` empty; `go vet ./...` clean.
  Result: `go test ./internal/cli -short` 28 → 39 (+11, all new tests green); `go test ./... -short` 188 → 199 (+11, same delta, no other package touched); `gofmt -l ./cmd ./internal` empty; `go vet ./...` clean.
- [x] 1.5 Size gate: measure `git diff --shortstat 5e9f443..HEAD -- . ':(exclude)openspec'` (no `wc -l` addend — a new file's lines are already its insertions). If the total exceeds 220, or projecting Commit 2's ≈175 more lines would exceed 400, stop and drop `version` first, then `mysqldump`, into the follow-up `down-and-set-host-go-tests` before starting Phase 2. Never cut the `inside` family (purge/npm/magerun/test-unit/test-integration); never shrink by deleting comments.
  Result: measured with `git add -N internal/cli/inside_test.go` (untracked new file) then `git diff --shortstat 5e9f443 -- . ':(exclude)openspec'` → `4 files changed, 245 insertions(+), 2 deletions(-)` = **247 changed lines**, over the 220 gate (`inside_test.go` came out to 227 lines against the ~150 estimate — six exhaustive table cases plus comments, not four). Per the cut order, **`version`/`Installed`/`version.go:23`/`version_test.go` are deferred to `down-and-set-host-go-tests`**; Commit 2 in this change becomes mysqldump-only. Projected Commit 2 (mysqldump-only: `Dump` on the interface, fake growth, ~62-line `wrappers_test.go` additions, `wrappers.go:104` routed) ≈ 80-100 more lines, keeping the total safely under 400. The `inside` family was not cut; no comment was removed to fit the budget.
- [x] 1.6 Commit (orchestrator-executed): `test(cli): what the one-container commands ask of the engine`.
  Files: `internal/cli/engine.go`, `internal/cli/wrappers.go`, `internal/cli/fake_engine_test.go`, `internal/cli/inside_test.go`.

## Phase 2: Commit 2 — mysqldump (RED → GREEN)

> **Rescoped by the 1.5 size gate**: Commit 1 measured 247 changed lines (over the 220 gate), so
> `version`/`Installed`/`version.go:23`/`version_test.go` are cut into the follow-up
> `down-and-set-host-go-tests`, per the cut order in the Notes below. Commit 2 in this change is
> mysqldump-only.

- [x] 2.1 RED (interface + fake): extend `commands` with `Dump(dir, path string) error`. In `fake_engine_test.go`, add a `Path string` field on `call`; implement `Dump` (logs `{Method: "Dump", Dir: dir, Path: path}`, consumes an `outcome`).
  Verify: `gofmt -l internal/cli/fake_engine_test.go` empty; `go build ./...` succeeds.
  Deferred to `down-and-set-host-go-tests`: `Installed() (core.Installation, core.Tooling)` on the interface, and the fake's `installed core.Installation`/`tooling core.Tooling` fields.
- [x] 2.2 RED (mysqldump tests): append to `wrappers_test.go` `TestWhatMysqldumpAsks` (success logs `Resolve` then `Dump{Dir: <cwd>, Path: filepath.Join(t.TempDir(), "dump.sql")}`; `--json` variant asserts `{"path": ...}`; a refused `Dump` outcome → `exitDocker`) and `TestMysqldumpWithNoPathIsRefused` (no args → `exitUsage`, empty call log).
  Verify RED: `DOCKER_HOST=unix:///nonexistent go test ./internal/cli -short -v -run TestWhatMysqldumpAsks` → **FAILS**: `cmp.Diff` shows no `Dump` entry in `got` — `wrappers.go:104` still calls `engine(` directly — the real `Engine.Dump` re-enters `Resolve` then `e.database().Ready(project)`, which reaches a daemon; `DOCKER_HOST=unix:///nonexistent` fails that unreachable socket deterministically instead of hanging or reaching a live one. `TestMysqldumpWithNoPathIsRefused` already PASSES — it returns before any engine call.
- [x] 2.3 (deferred) RED (version tests): cut into `down-and-set-host-go-tests` by the 1.5 size gate — `internal/cli/version_test.go`, `TestWhatVersionReports`, `TestAnOptionNobodyDeclaredIsAUsageError` are not created in this change.
- [x] 2.4 GREEN: route `wrappers.go:104` (`dump`'s `Dump` call) from `engine(` to `newEngine(`. (`version.go:23` stays on `engine(` — deferred with `version`.)
  Verify GREEN: `go test ./internal/cli -short -v` → **PASSES** — both new test functions plus every prior test in the package, no `DOCKER_HOST` override needed.
- [x] 2.5 Regression: `go test ./... -short` (green, no red); `tests/run.sh unit` (612 baseline, unmodified — scoped form of the Bash unit suites); `gofmt -l ./cmd ./internal` empty; `go vet ./...` clean; confirm `var _ commands = (*dockergento.Engine)(nil)` still compiles.
- [x] 2.6 Commit (orchestrator-executed): `test(cli): what mysqldump asks of the engine`.
  Files: `internal/cli/engine.go`, `internal/cli/wrappers.go`, `internal/cli/fake_engine_test.go`, `internal/cli/wrappers_test.go`.

## Phase 3: Manual verification (real project)

- [x] 3.1 Build the binary from the pre-change ref (`5e9f443`) and from HEAD after Commit 2 (`go build -o bin/hm ./cmd/hm`). Against a running project, run `hm purge` on the same project — **this deletes generated code** (`var/cache/*`, `generated/*`, `pub/static/*`, `var/view_preprocessed/*`, `var/page_cache/*`, `var/generation/*`, `dev/tests/integration/tmp/*`) — and confirm exit code and output are unchanged from the pre-change build. No path over a few MB is touched; nothing moves media.
  (`version`'s manual comparison is deferred with `version` itself to `down-and-set-host-go-tests`, since `version.go:23` is not routed in this change.)
  Verify: manual — `purge` behaves identically on both builds; `git diff --stat 5e9f443..HEAD -- console bin/run dockergento` is empty (no file under `console/`, `bin/run`, or `dockergento/` modified).

## Notes

- Out of scope, no tasks added: `down` and `set-host` (deferred to `down-and-set-host-go-tests`), the `here()` vs `project.Root` inconsistency in `dump` (open question, intentionally not fixed), the deferred varnish `report(nil err)` defect, any `MIGRATION.md` edit (the generic rule already covers these seven commands), any Bash deletion.
- No documentation task added: no command's arguments, `--json` document, exit codes, or `docs/<comando>.md`/`data/command_descriptions.json` entries change — test-only slice on an existing, already-documented seam.
- If Commit 2 threatens the 400-line budget after the Phase 1 size gate, cut in the proposal's order: `version` first (self-contained — drops `Installed`, the `installed`/`tooling` answer fields, and `version.go:23`), then `mysqldump` (drops `Dump`, the `Path` field, and `wrappers.go:104`) — never the `inside` family.
