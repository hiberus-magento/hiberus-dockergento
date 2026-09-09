# Tasks: The two `down` options that take a value and accept none

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~120-180 (proposal's own estimate; sibling `wrapper-exit-codes` estimated 50-90 and landed at 184 with the same shape) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Both guards + tests + delta spec | PR 1 (only PR) | `go test ./internal/cli/ -run TestDown -v` | `hm down --rmi` / `hm down --rmi all` on a **disposable** test project directory — never `rabatrepo` (9 live containers) | `git revert` — `internal/cli/down.go` (2 guards), `internal/cli/down_test.go` (new), delta spec; nothing else depends on it |

## Phase 1: Scaffolding

- [x] 1.1 Before any edit, run `git rev-parse HEAD` and keep that hash for the size check in Phase 6 — do not hardcode a fixed hash in this file or the commit; two other changes have already gone stale that way today. (Parent: `091e53def0f8059aa0d92ddf3f4190b957e4fd35`)
- [x] 1.2 Create `internal/cli/down_test.go` (new). Package `cli`, imports `encoding/json`, `io`, `strings`, `testing`, `github.com/google/go-cmp/cmp`. File-level comment: no `answering(t)`, no fake engine, no Docker — every refusal here returns during parsing, before `projectOr` (`internal/cli/engine.go:55` in `down.go`) is ever reached, except the one test in Phase 4 that deliberately reaches it. State explicitly it diverges from `version_test.go:128-152` / `set_host_test.go:190-202`, which use `answering(t)`'s fake+chdir+env fixture; this file needs only the `t.Chdir(t.TempDir())` half, applied per-test, never the fake.

## Phase 2: RED/GREEN — `--rmi` missing value

- [x] 2.1 RED — add `TestDownRefusesAMissingRmiValue` (table: text and `--json`), each: `t.Chdir(t.TempDir())`, `stdout, stderr := &strings.Builder{}, &strings.Builder{}`, `code := down([]string{"--rmi"}, stdout, stderr, jsonOutput)`. Text subtest asserts `code == exitUsage`, `stdout.String() == ""`, stderr contains `"--rmi needs a value"` and `"  → hm down --rmi all"`. `--json` subtest `json.Unmarshal`s stderr into the `set_host_test.go:165-172`-shaped envelope struct and `cmp.Diff`s against `{Code: 2, Type: "missing_value", Message: "--rmi needs a value", Hint: "hm down --rmi all"}`, plus `stdout.String() == ""`.
- [x] 2.2 Run `go test ./internal/cli/ -run TestDownRefusesAMissingRmiValue -v` and paste the failure. Expected RED: `code == 4` (`exitProject`), not 2 — with no guard, the loop falls through to `projectOr`, which fails resolution against the temp dir and answers `project_not_configured`/exit 4 before ever reaching the assertion's expected `missing_value`/exit 2. That is failing for the right reason: the guard does not exist yet, not a typo in the test. (Confirmed: `down --rmi = 4, want exitUsage` / `down --rmi --json = 4, want exitUsage`.)
- [x] 2.3 GREEN — edit `internal/cli/down.go:31-35`: above the existing `if at+1 < len(args) { ... }`, add `if at+1 >= len(args) { return failure(stderr, jsonOutput, "down", exitUsage, "missing_value", "--rmi needs a value", binaryName()+" down --rmi all") }`, the verbatim shape of `internal/cli/web.go:53-56`. Re-run 2.2's command, confirm PASS. (Confirmed: 3 passed.)

## Phase 3: RED/GREEN — `-t`/`--timeout` missing value

- [x] 3.1 RED — add `TestDownRefusesAMissingTimeoutValue` (table over `-t` and `--timeout`, each with text and `--json` subtests), same buffer/chdir setup as 2.1, asserting `error.type == "missing_value"`, message `argument+" needs a value"` (the flag as typed — `orchestrate.go:102-105`'s generic shape, not a hardcoded string), hint `"hm down -t 30"`, stdout empty.
- [x] 3.2 Same commit, add `TestDownStillRefusesANonNumericTimeout` (`hm down -t nada`): this one is **not** a RED test — it pins the existing `invalid_argument` behavior (`down.go:42-44`) so the new guard cannot regress it. Run it first and confirm it already PASSes against unmodified `down.go`, then re-run after 3.3 and confirm it still PASSes. (Confirmed pre-guard PASS: 1 passed; confirmed post-guard PASS below.)
- [x] 3.3 Run `go test ./internal/cli/ -run TestDownRefusesAMissingTimeoutValue -v`, paste the failure. Expected RED, same reason as 2.2: falls through to `projectOr`, answers exit 4/`project_not_configured` instead of exit 2/`missing_value`. (Confirmed: `down -t = 4, want exitUsage` / `down --timeout = 4, want exitUsage` and their `--json` counterparts; 7 failed.)
- [x] 3.4 GREEN — edit `internal/cli/down.go:36-48`: replace `if at+1 < len(args) { ... }` with the guard placed **above** the `strconv.Atoi` call: `if at+1 >= len(args) { return failure(stderr, jsonOutput, "down", exitUsage, "missing_value", argument+" needs a value", binaryName()+" down -t 30") }`, then `at++` and the existing `strconv.Atoi`/`invalid_argument` block unconditionally (no longer wrapped in the old `if`). Re-run 3.3's and 3.2's commands, confirm both PASS — the missing case now answers `missing_value` and the non-numeric case still answers `invalid_argument`, never collapsed into one type. (Confirmed: 8 passed.)

## Phase 4: Regression and the stated gap

- [x] 4.1 Add `TestDownStillAcceptsAGivenValue` (table: `--rmi all`; `-t 30`; `-v --remove-orphans --rmi all -t 30`). **Named safety-pin step**: each case starts with `t.Chdir(t.TempDir())` before calling `down(...)` — this is what makes `projectOr` (`internal/cli/engine.go:77-92`, failing at the `newEngine(nil, nil, jsonOutput).Resolve(directory)` call and the `project.Name == ""` check at `:84-89`) fail deterministically, stopping the run before `down.go:61`'s `engine(stdout, stderr, jsonOutput).Down(...)` — the concrete, non-seamed call that would otherwise reach a real Docker daemon (the machine running this has `rabatrepo` live with 9 containers). Assert exactly `code == exitProject` (4) and `error.type == "project_not_configured"`, never "any code other than 2".
- [x] 4.2 In the same test, immediately above the assertion, add the gap comment verbatim in intent: `// This does not prove --rmi all stores "all" in core.DownOptions.Images, nor that -t 30 stores 30 — Down is off the commands seam (engine.go:17-30) until down-go-tests grows it (archive/2026-09-05-down-and-set-host-go-tests/design.md:291-296). Asserting the exact resolution failure is the strongest statement available without it. Stated gap, not an oversight.`
- [x] 4.3 Add `TestADownOptionNobodyDeclaredIsAUsageError` (`--tonteria`, from `t.Chdir(t.TempDir())`): pins the existing `default:` branch — `exitUsage`, `invalid_argument`, `"Unknown option: --tonteria"`, hint `"hm down [-v]"` — confirming neither guard touched it.
- [x] 4.4 Run `go test ./internal/cli/ -run TestDown -v`, confirm all of Phase 2-4's tests PASS together. (Confirmed: 15 passed — note the `-run TestDown` regex does not match `TestADownOptionNobodyDeclaredIsAUsageError`, since "TestADown" does not contain the substring "TestDown"; verified it separately, 1 passed.)

## Phase 5: Spec and handoff discoverability

- [x] 5.1 Verify `openspec/changes/down-json-and-flags/specs/environment-lifecycle/spec.md` already carries the "An option that takes a value refuses when none follows" and "A missing value and a bad value stay distinguishable" scenarios naming these exact test names — no edit needed, this step only confirms the names in 2.1/3.1/3.2 match what the spec cites. (Confirmed: spec.md:21-46 names `TestDownRefusesAMissingRmiValue`, `TestDownRefusesAMissingTimeoutValue`, `TestDownStillRefusesANonNumericTimeout` verbatim — match.)
- [x] 5.2 In `internal/cli/down_test.go`, near `TestDownStillAcceptsAGivenValue`, add a short comment pointing the next reader at the delta spec's "Handoff Note" (`specs/environment-lifecycle/spec.md:72-79`): `archive/2026-09-05-down-and-set-host-go-tests/design.md:306` and `:340-342` still plan to pin `--rmi`/`-t` last as silently-ignored `core.DownOptions{}`; `down-go-tests` must read the Handoff Note before extending this file, because that plan is now stale. **Decision**: do not edit the archived design.md itself — it sits outside this change's edit authority and archived documents are not retroactively rewritten; the breadcrumb goes in the file `down-go-tests` will actually open and extend, which is more discoverable than a note in a frozen document nobody browses when starting new work.

## Phase 6: Verification and commit

- [x] 6.1 `gofmt -l ./cmd ./internal ./dockergento` — expect empty output. (Confirmed: empty.)
- [x] 6.2 `go vet ./...` — expect clean. (Confirmed: clean.)
- [x] 6.3 `go test ./...` — expect the 303-across-24-packages baseline plus this change's new `internal/cli` tests, all passing. (Confirmed: 319 passed in 24 packages — 16 new.)
- [x] 6.4 `./tests/run.sh unit` (exactly `unit`, not a path like `tests/unit` — that silently runs the whole suite) — expect the 616 baseline, unaffected (no Bash side of `down` changed). (Confirmed: 616 assertions passed.)
- [x] 6.5 `git diff --shortstat <parent from 1.1>..HEAD -- . ':(exclude)openspec'` — sanity-check against the ~120-180 estimate. (Not yet committed; measured with `git diff --shortstat 091e53d -- . ':(exclude)openspec'` after `git add -N` for the new file: 2 files changed, 290 insertions(+), 12 deletions(-) — 302 changed lines. Above the 120-180 estimate, closer to but above the `wrapper-exit-codes` sibling's 184; still comfortably inside the 400-line budget.)
- [x] 6.6 Manual verification, **from a temporary directory that is not a project**, never from one with a live environment. All four cases are observable there, because both new refusals return before the engine is ever reached and a directory with no project stops the rest at resolution:
      - `hm down --rmi` → exit 2, `missing_value`
      - `hm down -t` → exit 2, `missing_value`
      - `hm down -t nada` → exit 2, `invalid_argument` (unchanged)
      - `hm down --rmi all` → exit **4**, `project_not_configured` — the parse loop accepted the value and moved on
      Run each with and without `--json` and confirm stdout stays empty in both.
      Do **not** run `hm down --rmi all` against a real environment to "check it still destroys": it destroys whatever is active in the current directory and removes its images, and it proves nothing this change touches — the guards return long before the engine. Whether the parsed value actually reaches `core.DownOptions` is the gap the spec declares and `down-go-tests` owes. The user's `rabatrepo` project with 9 containers is running on this machine.
      (Confirmed all four from a `mktemp -d` non-project directory, `--no-json` and `--json` each: exit 2/`missing_value` for `--rmi` and `-t`; exit 2/`invalid_argument` for `-t nada`; exit 4/`project_not_configured` for `--rmi all`. stdout empty in all eight invocations. `rabatrepo` untouched — never invoked against a project directory.)
- [ ] 6.7 One commit on `release/2.0.0` covering `internal/cli/down.go`, `internal/cli/down_test.go`, and the delta spec. Suggested message: `fix(cli): refuse down --rmi and -t/--timeout given without a value` — English, imperative, no AI attribution. (Not performed — the user commits, per instructions for this session.)
