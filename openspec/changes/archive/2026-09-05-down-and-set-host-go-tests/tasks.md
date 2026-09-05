# Tasks: What `version` and `set-host` ask of the engine

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ≈315 (`version_test.go` new ≈85, `wrappers_test.go` ≈39 cwd-only, `inside_test.go` ≈17 cwd-only, `fake_engine_test.go` ≈30+≈18, `engine.go` 1+2, `version.go` 1, `set_host_test.go` new ≈120, `wrappers.go` 2), excluding `openspec/changes/` |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single delivery, two work-unit commits |
| Delivery strategy | single-pr (no open PRs — direct commit onto `release/2.0.0`); no `size:exception` needed at this estimate |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Six `answering(t)` pins + cwd-read refactor in existing tests, `Installed` on the interface and fake, `version_test.go`, `version.go:23` routed | Commit 1 (direct to `release/2.0.0`) | `go test ./internal/cli -short -v -run 'TestWhatVersionReports\|TestAnArgumentNobodyDeclaredIsAUsageError'` | `go build -o bin/hm ./cmd/hm` then manual `hm version` / `hm version --json` (Phase 3) | Revert `internal/cli/engine.go` (`Installed`), `internal/cli/version.go:23`, `internal/cli/fake_engine_test.go` (pins + `Installed` growth), `internal/cli/version_test.go`; the cwd-read edits in `wrappers_test.go`/`inside_test.go` are inert without the pins and revert with them |
| 2 | `SetHost`/`RemoveHost` on the interface and fake, `set_host_test.go`, `wrappers.go:281,296` routed | Commit 2 (direct to `release/2.0.0`) | `go test ./internal/cli -short -v -run 'TestWhatSetHostAsks\|TestRemovingAHostAsksNothingAboutTheProject\|TestARefusedHostEditIsReported\|TestASetHostOptionNobodyDeclaredIsAUsageError'` | `go build -o bin/hm ./cmd/hm` then manual `hm set-host --remove nothing.test` / `hm set-host nothing.test --no-database` with `HM_HOSTS_FILE` pointed at a temp file (Phase 3) | Revert `internal/cli/engine.go` (`SetHost`, `RemoveHost`), `internal/cli/wrappers.go:281,296`, `internal/cli/fake_engine_test.go` (`SetHost`/`RemoveHost` growth), `internal/cli/set_host_test.go` — independent of Commit 1 |

## Phase 1: Commit 1 — safety pins, cwd refactor, and `version` (RED → GREEN)

- [x] 1.1 Refactor (pins, no RED expected): in `internal/cli/fake_engine_test.go`'s `answering(t)`, add a doc comment beside each of the six pins stating its reason, adding code for the five new ones: `HM_HOSTS_FILE=filepath.Join(t.TempDir(), "hosts")` (`app/hosts.go:186` falls back to the system hosts file; `:173` shells to `sudo cp`), `HM_NON_INTERACTIVE=1` (`select.go:39` makes `choose()` refuse), `DOCKER_HOST=unix:///nonexistent` (an unrouted toolinfo dial has no deadline), `t.Chdir(t.TempDir())` (`app/hosts.go:49` writes `properties.json` under the resolved root = cwd), `HM_LEGACY_ROOT=t.TempDir()` (`legacy/runner.go:94` would exec `bin/run` if a developer's own hosts file already resolves the test domain). `HM_STATE_DIR` already exists; document its reason too.
  Verify: `gofmt -l internal/cli/fake_engine_test.go` empty; `go build ./...` succeeds; whole package still green: `go test ./internal/cli -short` passes at the pre-existing 44-test baseline (this is a refactor step — RED is not expected here).
- [x] 1.2 Refactor (cwd adjustment, mechanical): in `internal/cli/wrappers_test.go` (lines 25, 64, 123, 173, 249, 283, 307) and `internal/cli/inside_test.go` (lines 22, 55, 113), move each `cwd := os.Getwd()` capture to read `cwd := here()` (or `os.Getwd()`) AFTER `answering(t)` runs, since `answering` now changes the working directory via `t.Chdir`. Drop the now-unused `os` import from either file if nothing else in it needs `os`.
  Verify: same green run as 1.1 — `go test ./internal/cli -short` stays at 44, no new failures from the pins or the cwd move (still a refactor step, no RED expected).
- [x] 1.3 RED: create `internal/cli/version_test.go` with `TestWhatVersionReports` (decode the `--json` document, delete `binary`, `cmp.Diff` the rest against a literal built from fake `installed`/`tooling` answers, then assert `binary == buildOfThisBinary()` separately; assert the text form including the `orUnknown` "unknown" and `orMissing` "not available" placeholders; assert the call log is exactly `[{Method:"Installed"}]`) and `TestAnArgumentNobodyDeclaredIsAUsageError` (any argument → `exitUsage`, empty call log).
  Verify RED: `go test ./internal/cli -short -run TestWhatVersionReports` → **FAILS**: `version.go:23` still calls `engine(` directly, so the real `Installed` spawns `git` and dials the pinned dead socket instead of the fake, and the call log lacks the `Installed` entry, so `cmp.Diff` reports a mismatch.
- [x] 1.4 GREEN: extend `commands` in `internal/cli/engine.go` with `Installed() (core.Installation, core.Tooling)`; route `internal/cli/version.go:23` from `engine(` to `newEngine(`.
  Verify GREEN: `go test ./internal/cli -short -v` → **PASSES**, both new `version_test.go` functions and every pre-existing test in the package.
- [x] 1.5 Regression: `go test ./internal/cli -short` (44 baseline, all green, new tests add to the count); `go test ./... -short` (204 baseline, no new red, 22 packages); `gofmt -l ./cmd ./internal` empty; `go vet ./...` clean.
  Measured: `internal/cli` at 44 baseline pre-change, 52 after Commit 1's two new tests (TestWhatVersionReports has 2 subtests, TestAnArgumentNobodyDeclaredIsAUsageError has 2); 22 packages, all green; gofmt and go vet both clean.
- [x] 1.6 Commit (orchestrator-executed): `test(cli): what version asks of the engine, and where RED is allowed to reach`. Message must explain the cwd pin and why ten otherwise-untouched tests changed.
  Files: `internal/cli/engine.go`, `internal/cli/version.go`, `internal/cli/fake_engine_test.go`, `internal/cli/version_test.go`, `internal/cli/wrappers_test.go`, `internal/cli/inside_test.go`.

## Phase 2: Commit 2 — `set-host` (RED → GREEN)

- [x] 2.1 RED: create `internal/cli/set_host_test.go` with `TestWhatSetHostAsks` (default → `Database:true`; `--no-database` → `Database:false`; `--json` → `{"domain","database"}`; `Dir` equals cwd via `here()`), `TestRemovingAHostAsksNothingAboutTheProject` (`--remove shop.test` → `fake.project` stays zero, call log is exactly `[{Method:"RemoveHost", Domain:"shop.test"}]`; JSON `{"removed":"shop.test"}`), `TestARefusedHostEditIsReported` (fake `SetHost` outcome returns a `core.Refusal` → reported with the refusal's own code/message/hint via `asRefusal`, not `exitDocker`), `TestASetHostOptionNobodyDeclaredIsAUsageError` (`-x` → `exitUsage`, empty log). Test domain `shop.test` (RFC 6761); never `localhost`, which resolves locally and diverts `Hosts.Set` into the legacy branch.
  Verify RED: `go test ./internal/cli -short -run 'TestWhatSetHostAsks|TestRemovingAHostAsksNothingAboutTheProject'` → **FAILS**: `wrappers.go:281`/`:296` still call `engine(` directly, so the real `Hosts.Set`/`Remove` reach `os.ReadFile` on the missing pinned temp hosts file, and the call log lacks `SetHost`/`RemoveHost`.
- [x] 2.2 GREEN: extend `commands` in `internal/cli/engine.go` with `SetHost(dir, domain string, database bool) error` and `RemoveHost(domain string) error`; route `internal/cli/wrappers.go:281` (`RemoveHost`) and `:296` (`SetHost`) from `engine(` to `newEngine(`.
  Verify GREEN: `go test ./internal/cli -short -v` → **PASSES**, all four new `set_host_test.go` functions and every prior test in the package.
- [x] 2.3 Regression: `go test ./internal/cli -short` (green, no red); `go test ./... -short` (green, no red, 22 packages); `tests/run.sh unit` (612 baseline, unmodified — scoped Bash unit form); `gofmt -l ./cmd ./internal` empty; `go vet ./...` clean; confirm `var _ commands = (*dockergento.Engine)(nil)` still compiles.
  Measured: `internal/cli` at 61 tests total (52 + 9 new from Commit 2's four test functions, one with 3 subtests), all green; 22 packages green; `tests/run.sh unit` reports 612 assertions passed, unchanged; gofmt and go vet both clean; the assertion still compiles (confirmed by successful `go build ./...` and `go vet ./...`).
- [x] 2.4 Size gate (sanity only, nothing left to cut): measure `git diff --shortstat 7417863..HEAD -- . ':(exclude)openspec'`. Must be ≤400 changed lines (estimate c1 ≈173 + c2 ≈142 ≈315). If it exceeds 400, stop and report rather than shrink anything.
  **Measured: `8 files changed, 499 insertions(+), 110 deletions(-)` = 609 changed lines, 209 over the 400 budget** (estimate ≈315 was wrong by ≈294, not the ≈85 the design's own worst case named). Per this task's own instruction and the design's Size-and-Commits section, nothing was shrunk to fit. Reported to the orchestrator as a blocker; no commit made. See apply-progress.md for the per-file breakdown and the approximate per-commit split.
- [x] 2.5 Commit (orchestrator-executed): `test(cli): what set-host asks of the engine`.
  Files: `internal/cli/engine.go`, `internal/cli/wrappers.go`, `internal/cli/fake_engine_test.go`, `internal/cli/set_host_test.go`.

## Phase 3: Manual verification (real project, non-destructive)

- [x] 3.1 Build the binary from the pre-change ref (`7417863`) and from HEAD after Commit 2 (`go build -o bin/hm ./cmd/hm`). Against a running project, compare `hm version` and `hm version --json` between the two builds field-for-field (aside from the `binary` field's own expected build-time differences).
  Verify: manual — output matches on both builds.
- [x] 3.2 Against the same project, with `HM_HOSTS_FILE=<tmp>` pointed at a TEMPORARY file, compare `hm set-host --remove nothing.test` and `hm set-host nothing.test --no-database` between the pre-change and HEAD builds: same exit codes, same JSON under `--json`. The real hosts file (system "/etc/hosts") MUST NOT be touched — verify by its unchanged mtime after both runs.
  Verify: manual — exit codes and JSON identical on both builds; system "/etc/hosts" mtime unchanged.

## Notes

- Out of scope, no tasks: `down` (all of it — carried to the follow-up `down-go-tests`), `here()` vs `project.Root`, the varnish `report(nil err)` defect, any `MIGRATION.md` edit, any Bash deletion.
- No documentation task added: no command's arguments, `--json` document, or exit codes change — test-only slice on an existing, already-documented seam.
</content>
