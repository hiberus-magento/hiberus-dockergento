```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:84d6dc1ea680138eeca295644623d04ecaaa20a3d1d50e9e32f6a36a6d418507
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 5/5
scenarios: 10/10
test_command: go test ./internal/cli -short
test_exit_code: 0
test_output_hash: sha256:be9d8ff67aa63422ba3d0707cdfc8ab11dc7eb6783fcfd32f83ec6ffc22f647e
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: wrappers-go-tests
**Version**: N/A
**Mode**: Strict TDD
**Evidence commit**: 1dad4721444d195c516db3ce2dcf73f1a79c59f7 (`release/2.0.0`, working tree clean except untracked `openspec/changes/wrappers-go-tests/`)
**Delivery**: three commits — `1327ae8` (seam + fake + tests + MIGRATION.md), `8ce9879` (php.go comment), `1dad472` (here-strings)

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 14 |
| Tasks complete | 14 (per `tasks.md`, all checked) |
| Tasks incomplete | 0 |

Note: `apply-progress.md`'s "Pending Tasks (4/14)" and "Remaining Tasks" sections are stale — they predate the orchestrator executing commits 1.7/2.3/3.3 and the manual verification 4.1 — but the appended "Manual verification record (task 4.1)" section and `tasks.md`'s checkbox state (the canonical tracker) both confirm all 14 are done. WARNING, not CRITICAL: the artifact is internally inconsistent but not misleading about actual completion.

### Build & Tests Execution
**Build**: Passed
```text
$ go build ./...
(no output, exit 0)
```

**Tests**: 28 passed / 0 failed / 0 skipped (`internal/cli`); 188 passed in 22 packages (`go test ./... -short`); 612 assertions passed (`tests/run.sh unit`, includes `migration_status_test.sh` → `RESULT 12 0`)
```text
$ go test ./internal/cli -short
Go test: 28 passed in 1 packages

$ go test ./... -short
Go test: 188 passed in 22 packages

$ tests/run.sh unit
612 assertions passed

$ gofmt -l ./cmd ./internal
(empty)

$ go vet ./...
Go vet: No issues found
```

**Coverage**: Not available — no coverage tool detected in this repo's toolchain.

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| A ported command's engine interaction is provable without Docker | Copying into the container | `internal/cli/wrappers_test.go:22` `TestWhatCopyingIntoTheContainerAsks/a_named_path` | ✅ COMPLIANT |
| A ported command's engine interaction is provable without Docker | Copying everything into the container | `internal/cli/wrappers_test.go:34` `TestWhatCopyingIntoTheContainerAsks/--all_first` | ✅ COMPLIANT |
| A ported command's engine interaction is provable without Docker | Copying out of the container | `internal/cli/wrappers_test.go:61` `TestWhatCopyingOutOfTheContainerAsks` | ✅ COMPLIANT |
| A ported command's engine interaction is provable without Docker | Turning the page cache on | `internal/cli/wrappers_test.go:179` `TestWhatVarnishAsks/on` | ✅ COMPLIANT |
| A ported command's engine interaction is provable without Docker | Turning the page cache off cascades | `internal/cli/wrappers_test.go:207` `TestWhatVarnishAsks/off` | ✅ COMPLIANT |
| A usage error returns before any engine call | No path given | `internal/cli/wrappers_test.go:90` `TestCopyingWithNoPathIsRefused` (both subcases) | ✅ COMPLIANT |
| The test seam changes nothing a real invocation runs | Production still builds the real engine | `internal/cli/engine.go:25` `var _ commands = (*dockergento.Engine)(nil)` — compile-time assertion enforced by every `go build`/`go vet`/`go test` invocation above | ✅ COMPLIANT |
| The test seam changes nothing a real invocation runs | Nothing else moves | `go build ./...`, `go test ./... -short`, `tests/run.sh unit` all pass; commit `1327ae8` touches zero Bash suite files (`git show --stat 1327ae8`) | ✅ COMPLIANT |
| The migration document's truthfulness check does not depend on a pipe's producer finishing | A match found before the input is exhausted | `tests/unit/migration_status_test.sh:34,42` (here-strings) exercised by `RESULT 12 0`, including "every command is in the table" / "and the table invents none" | ✅ COMPLIANT |
| A comment about what stays in shell names its real reason | The mac Composer vendor mirror | `internal/cli/php.go:91-99` — comment-only change; verified via diff evidence (the orchestrator's check explicitly admits "a test or diff evidence" for this scenario) rather than a runtime test, since no behavior exists to execute | ✅ COMPLIANT (diff evidence) |

**Compliance summary**: 10/10 scenarios compliant. 9/10 via passing runtime tests; 1/10 (the comment-only requirement, which has no behavior to execute) via diff evidence, per this verify task's explicit instruction that a scenario may map to "a test or diff evidence." The generic hard rule ("compliant only when a covering test passed at runtime") is honoured for all 9 behavioral scenarios; this one exception is disclosed as a WARNING below, not silently upgraded.

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| REQ-01 engine interaction provable without Docker | ✅ Implemented | `commands` interface (5 methods), `fakeEngine` (`fake_engine_test.go`), 7 test functions in `wrappers_test.go` |
| REQ-02 usage error precedes engine call | ✅ Implemented | `TestCopyingWithNoPathIsRefused` asserts `len(fake.calls) == 0` |
| REQ-03 seam changes nothing in production | ✅ Implemented | `newEngine` default wraps unchanged `engine()`; compile-time interface check |
| REQ-04 migration check pipe robustness | ✅ Implemented | Two `grep -qx ... <<<` here-strings replacing piped `grep -qx` |
| REQ-05 comment names its real reason | ✅ Implemented | `mirrorsVendor` named, no remaining claim that `copy-to-container` is unported |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| `commands` interface, exactly 5 methods | ✅ Yes | `Resolve, Exec, Restart, CopyInto, CopyFrom` — signatures match `dockergento/dockergento.go:129,271,293,508,529` exactly |
| `var _ commands = (*dockergento.Engine)(nil)` | ✅ Yes | `engine.go:25` |
| `newEngine` package-level factory var | ✅ Yes | `engine.go:29-31` |
| 5 call sites moved (`projectOr`, `wrappers.go:150,219,244`, `php.go:59`) | ✅ Yes | Confirmed by diff; `varnish`/`copyInto`/`copyFrom`/`inside`/`projectOr` all route through `newEngine` |
| 39 call sites kept concrete (calling `engine()` directly) | ✅ Yes | Counted 39 remaining direct `engine(...)` handler call sites across `internal/cli` (excluding the `newEngine` factory body and comments) |
| Ordered call-log fake (`[]call`), compared with `cmp.Diff` | ✅ Yes | `fake_engine_test.go`; `go-cmp v0.7.0` already a direct dependency (`go.mod:10`) |
| `answering` pins `HM_STATE_DIR` via `t.Setenv` | ✅ Yes | `fake_engine_test.go:108` |
| No `t.Parallel()` | ✅ Yes | 0 matches in `wrappers_test.go`/`fake_engine_test.go` |
| MIGRATION.md bullet, in Spanish, existing voice | ✅ Yes | "Contra `newEngine`, no contra Docker." — bold lead phrase + explanation, matching adjacent bullets |
| `php.go` comment names `mirrorsVendor`, drops the "unported" claim | ✅ Yes | Rewritten comment confirmed; cross-checked `copyInto` wiring at `run.go:103` |
| `migration_status_test.sh:34,42` use here-strings | ✅ Yes | Confirmed via diff, following the pre-existing pattern at `:101` |

### TDD Compliance
| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | ✅ | `apply-progress.md` "TDD Cycle Evidence" table present |
| All tasks have tests | ✅ | 1.1–1.4 covered by `wrappers_test.go`/`fake_engine_test.go`; 2.1/3.1–3.2 are comment/shell-robustness tasks with documented "no test possible"/existing-suite-as-safety-net rationale, consistent with their nature |
| RED confirmed (tests exist) | ✅ | Both new test files exist and are exercised by the runners above |
| GREEN confirmed (tests pass) | ✅ | 28/28 in `internal/cli`, 188/188 overall, both re-run independently in this verify session |
| Triangulation adequate | ✅ | 3 cases in `TestWhatCopyingIntoTheContainerAsks`, 2 in `TestWhatVarnishAsks`, 2 each in `TestCopyingWithNoPathIsRefused`/`TestACopyTheEngineRefusesIsReported` — confirmed by reading `wrappers_test.go` |
| Safety Net for modified files | ✅ | `wrappers.go`/`php.go`/`engine.go` pre-change compiled and existing suites passed per apply-progress; re-confirmed post-change by the regression sweep above |
| RED claim (logical confirmation) | ✅ | With only `projectOr` routed (task 1.1's first half), the four handler call sites in `wrappers.go`/`php.go` still call the real `engine()` directly; `projectOr` is the sole routed call, so `fake.calls` can only ever hold the `Resolve` entry it produces before any handler-level `Exec`/`Restart`/`CopyInto`/`CopyFrom` call — which bypasses the fake entirely and reaches the real engine instead. This matches the apply-progress RED excerpt (`cmp.Diff` showing `got` = `[]call{{Resolve, ...}}` against a `want` holding the full choreography) and is confirmed here by static reasoning over the final diff's call-site structure, without re-running an intermediate uncommitted state. |

**TDD Compliance**: 7/7 checks passed

---

### Test Layer Distribution
| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit | 7 new (+ 21 pre-existing = 28 total in package) | `internal/cli/wrappers_test.go`, `internal/cli/fake_engine_test.go` | Go `testing`, `go-cmp` |
| Integration | 0 | — | not applicable to this change |
| E2E | 0 (pre-existing `test/e2e` suite untouched; ran manually against Docker as part of task 4.1, not part of this change's automated suite) | — | Docker (manual only) |
| **Total** | **7 new** | **2 new files** | |

---

### Changed File Coverage
Coverage analysis skipped — no coverage tool detected in this repo's toolchain (no `go test -cover` convention established in `CLAUDE.md` or `tests/run.sh`).

---

### Assertion Quality
No violations found. All new assertions in `wrappers_test.go` call production handlers (`copyInto`, `copyFrom`, `varnish`) and compare either exit codes against named `exit*` constants or the fake's ordered `calls` log against a literal `want []call` via `cmp.Diff` — no tautologies, no assertions outside production-code calls, no ghost loops over possibly-empty collections, no CSS/implementation-detail coupling (Go, N/A), and the one intentional `cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")` is justified in design (reads the test process's own stdin) rather than masking a real assertion. Mock/assertion ratio: one fake per test file, multiple `cmp.Diff`/exit-code assertions per test — well under the 2× mock-heavy threshold.

**Assertion quality**: ✅ All assertions verify real behavior

---

### Quality Metrics
**Linter**: Not available — no linter configured/detected for this Go module beyond `go vet` (clean) and `gofmt` (clean, both re-run above).
**Type Checker**: ✅ No errors (`go build ./...`, `go vet ./...` both clean; Go's compiler is the type checker)

### Issues Found

**CRITICAL**: None

**WARNING**:
1. REQ-05's scenario ("The mac Composer vendor mirror") has no runtime-executable test — it asserts on comment prose, which has no behavior to execute. It is marked COMPLIANT above via diff evidence per this verify task's explicit instruction, not via the generic "covering test passed at runtime" rule. This repo's own `migration_status_test.sh` shows prose-vs-source assertions ARE technically writable here (via grep/string-match against the source), so a future change could close this gap with a real test if the project wants stricter enforcement of comment claims; today it is accepted as a documented exception, not a defect in this delivery.
2. `apply-progress.md`'s "Pending Tasks (4/14)" / "Remaining Tasks" sections are stale relative to `tasks.md` (14/14 checked) and the appended manual-verification record — the artifact was not fully updated after the orchestrator executed the three commits and the manual check. Recommend the orchestrator refresh those sections before archive, though `tasks.md` (the canonical tracker) is accurate and no work is actually missing.
3. `copy-to-container --all` is not exercised against a real Docker daemon by any automated or manual check in this delivery: the manual verification (task 4.1) explicitly skipped it (11 GB media), and `test/e2e/transfer_test.go` only copies directories/files by path, not the `--all` flag. The unit test (`TestWhatCopyingIntoTheContainerAsks/--all_first`) fully proves the handler-to-engine mapping (`CopyInto(..., all=true)`), which is what REQ-01's scenario requires and is what this change is scoped to prove — but end-to-end confidence for `--all` specifically against a real container remains unconfirmed. Residual risk, not a spec violation.
4. The manual varnish check (task 4.1, against `rabatrepo`) could not reach the purge + `cache:clean` cascade that `varnish-off` triggers, because that project has no `bin/magento` under `/var/www/html`; both binaries (pre-change and post-change) failed identically at the same step (`exitDocker`), so behavior-preservation is demonstrated for the VCL-edit and restart legs but not for the cascade's tail. The unit test (`TestWhatVarnishAsks/off`) fully covers the cascade's engine-call choreography (which is what REQ-01's scenario requires), including the purge and `cache:clean` calls via the fake — but no environment with a real Magento install exercised the cascade's tail against real Docker in this delivery. Residual risk, not a spec violation.

**SUGGESTION**:
1. `TestACopyTheEngineRefusesIsReported` was strengthened during apply beyond the task's literal `-run` filter expectation, adding a `cmp.Diff` call-log assertion because the original exit-code-only assertion passed trivially in RED (coincidental `exitDocker` from an unrouted real engine hitting a nonexistent Docker socket, not real fake substitution). This is disclosed transparently in `apply-progress.md`'s "Deviations from Design" section and verified present in the final test file (`wrappers_test.go:161-163`) — a strengthening that improves test validity, not a scope or behavior change. No action needed.
2. The two known, deliberately-deferred defects from design's Open Questions (`wrappers.go:155,166`'s `report(..., err)` call when `status != 0` and `err == nil`, and the `here()` vs `project.Root` inconsistency between copy and varnish handlers) remain present and untouched, exactly as the design and tasks specify — confirmed via `wrappers.go:155` still reading `if err != nil || status != 0 { return report(stderr, jsonOutput, command, err) }`. Tracked as follow-up work, not this change's scope.

### Verdict
**PASS WITH WARNINGS**

All 14 tasks complete, all three commits present and matching their stated file lists, 10/10 spec scenarios compliant (9 via passing runtime tests, 1 comment-only requirement via diff evidence per this task's explicit instruction, disclosed as WARNING 1), all design decisions honoured (interface shape, 5-moved/39-concrete call-site split, ordered call-log fake, `HM_STATE_DIR` pinning, no `t.Parallel()`, MIGRATION.md bullet, corrected comment, here-strings), the accepted `size:exception` (457 vs 400 lines) is disclosed and reasoned in both design and apply-progress, the deferred `report(nil err)` defect is confirmed untouched (not silently fixed), no read-only-area file (`dockergento/`, `console/`, `bin/run`) was touched, and all regression runners (`go test ./internal/cli -short` → 28 passed, `go test ./... -short` → 188 passed in 22 packages, `tests/run.sh unit` → 612 assertions, `gofmt -l` → empty, `go vet ./...` → clean) pass exactly as expected. Warnings are residual-risk disclosures (stale progress-doc bookkeeping, and two real-Docker manual-verification gaps for `--all` and the varnish cascade tail) that do not block delivery because the spec's actual requirement — the engine interaction being provable without Docker — is fully met by the unit tests.
