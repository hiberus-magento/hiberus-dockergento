```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:75c388eb869c5298c5cb7de7a8a0f8306ab7e253cbc6925eb602ea99163c6573
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 2/2
scenarios: 9/9
test_command: tests/run.sh unit
test_exit_code: 0
test_output_hash: sha256:265a6eac5b908700d8c5a2bbdb365dacf15bcb06b183d74294fcf95c96a43514
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: sync-migration-status
**Version**: N/A (delta spec, single domain `go-entrypoint`)
**Mode**: Strict TDD

Delivered as two commits on `release/2.0.0`: `1053545` (`test(migration): every command the
router answers, checked against the table`) and `86a45ef` (`docs: the implementation is Go and
Bash, not only Bash`). Verified against `git diff 156457e..86a45ef`. Current `HEAD` is `d39faac`
(`.gitignore` for `.codegraph/`), explicitly out of scope for this change per design/tasks.

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 14 |
| Tasks complete | 14 |
| Tasks incomplete | 0 |

Tasks 1.7 and 2.6 ("Commit: ...") are checked `[x]` in `tasks.md` with the note "Prepared but NOT
committed — orchestrator settles delivery," which is the task's own completion criterion (apply
prepares, orchestrator commits). `apply-progress.md`'s own checklist shows those two rows
unchecked from apply's narrower scope (apply itself did not run `git commit`). Both descriptions
are consistent, not contradictory. Both prepared commits now exist verbatim: `1053545` and
`86a45ef` match the planned messages exactly.

### Build & Tests Execution
**Build**: PASSED
```text
$ go build ./...
(no output, exit 0)
```

**Tests**: 12 passed / 0 failed (`migration_status_test.sh`, RESULT 12 0) — 612 passed / 0 failed
(`tests/run.sh unit`, full Docker-free suite)
```text
$ bash tests/unit/migration_status_test.sh
RESULT 12 0

$ tests/run.sh unit
612 assertions passed
```

**Go regression**: `go test ./... -short` → 172 passed in 22 packages, exit 0.

**Coverage**: not available (`coverage_threshold: 0` in `openspec/config.yaml`, no coverage tool
wired) → ➖ Not available.

**Command discrepancy (INFO, non-blocking)**: `openspec/config.yaml`'s `rules.verify.test_command`
is the unscoped `tests/run.sh` (which also runs Docker-dependent integration/performance suites).
The orchestrator explicitly directed the Docker-free scoped form `tests/run.sh unit`, matching what
`tasks.md`'s own "Focused test command" column specifies for both work units and what apply
actually ran. This is consistent with the design's threat matrix ("not applicable — no
routing/subprocess/executable-classification change") and not a deviation introduced by this
change.

### RED Claim Replay (independent confirmation)
Replayed the reverse-check `routed()` extraction against `internal/cli/run.go` (current, unchanged
by this diff) and `MIGRATION.md` as of `156457e` (pre-change): produced the exact same 14
routed-but-tabled-shell commands apply-progress reported — `copy-from-container
copy-to-container down mysqldump n98-magerun npm purge set-host setup test-integration test-unit
varnish-off varnish-on version`. RED claim independently confirmed true.

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|---|---|---|---|
| The state of the migration is written down and true | Every command is accounted for | `migration_status_test.sh > every command is in the table / and the table invents none / each one says who owns it` | ✅ COMPLIANT |
| The state of the migration is written down and true | A claim that is not true | `migration_status_test.sh > what is marked as Go exists in the Go tree` | ✅ COMPLIANT |
| The state of the migration is written down and true | Carrying on later | `migration_status_test.sh > it says how to build and how to test / and points at where the decisions are` | ✅ COMPLIANT |
| The state of the migration is written down and true | A command is fully wired but still tabled as shell | `migration_status_test.sh > every command the router answers for is tabled as go` | ✅ COMPLIANT |
| The state of the migration is written down and true | A command routed by more than one implementation | `migration_status_test.sh > the partial exception is still routed and still tabled shell` | ✅ COMPLIANT |
| The state of the migration is written down and true | The counter and the table agree | `migration_status_test.sh > and the count at the top matches the table / and so does the total` | ✅ COMPLIANT |
| The state of the migration is written down and true | The reconciliation touches no runtime code | `git diff 156457e..86a45ef --stat` (manual, no automated guard) | ✅ COMPLIANT |
| Documentation does not overstate what remains unported | Describing how a command reaches its implementation | manual read: `CLAUDE.md`, `architecture/02-cli-architecture.md` | ⚠️ PARTIAL (manual-only, no automated test — see Issues) |
| Documentation does not overstate what remains unported | No absolute claim remains | manual read: `CLAUDE.md`, `architecture/02-cli-architecture.md` | ⚠️ PARTIAL (manual-only, no automated test — see Issues) |

**Compliance summary**: 9/9 scenarios evidenced true; 7/9 by passing automated test, 2/9 by
project-sanctioned manual verification only.

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|---|---|---|
| `routed()` sed BRE extraction | ✅ Implemented | `[[:space:]]` form, not `-E`, matches design rationale (BSD/busybox/Alpine divergence) verbatim |
| `_registry` excluded | ✅ Implemented | `grep -v '^_'` — underscore-prefix rule, not a name list |
| `PARTIAL="db proxy"` pinned both sides | ✅ Implemented | Two assertions: excused from `go`, and still routed + still `shell` |
| `shell\|go` vocabulary untouched | ✅ Implemented | No third owner value introduced anywhere in `MIGRATION.md` |
| Counter `30 de 65` | ✅ Implemented | `MIGRATION.md:21` |
| `MIGRATION.md` line 190 fix | ✅ Implemented | Now cites `mirrorsVendor` (`internal/cli/php.go:98`), no "no está portado" claim remains |
| `MIGRATION.md` lines 281-282 fix | ✅ Implemented | Now at 282-285 (shifted by earlier edits), describes both directions |
| One-sentence doc replacements | ✅ Implemented | `CLAUDE.md:11`, `architecture/02-cli-architecture.md:5` — single-line diffs each |
| Flow diagram intact | ✅ Implemented | Diff for `architecture/02-cli-architecture.md` touches only line 5; nothing below it changed |
| Scope: no `console/`, `bin/run`, `internal/`, `dockergento/` | ✅ Implemented | Confirmed via `git diff 156457e..86a45ef --stat`: only `CLAUDE.md`, `MIGRATION.md`, `architecture/02-cli-architecture.md`, `openspec/config.yaml`, `tests/unit/migration_status_test.sh` |
| Router case count (design's "33 case labels, `_registry`@51..`restart`@134") | ✅ Confirmed | `rg -c 'case "' internal/cli/run.go` → 33; line 51/134 match exactly (read-only, unmodified by this change) |

### Coherence (Design)
| Decision | Followed? | Notes |
|---|---|---|
| Router is the sole source, read-only | ✅ Yes | `internal/cli/run.go` untouched in the diff |
| BRE + `[[:space:]]` over `-E` | ✅ Yes | Verbatim |
| `PARTIAL`/`is_partial()` pinned from both sides | ✅ Yes | Verbatim, plus a robustness fix (see Deviation) |
| Thirty of sixty-five arithmetic | ✅ Yes | 32 routed − 2 partial = 30; matches 16 pre-existing `go` rows + 14 flipped |
| Two MIGRATION.md prose fixes | ✅ Yes | Both present, correct citations |
| Commit split (test+data vs docs) | ✅ Yes | `1053545` / `86a45ef` match the design's commit table exactly |
| `.gitignore` entry rides separately | ✅ Yes | Confirmed: `.gitignore` change is in `d39faac`, not in either change commit |

### Disclosed Deviation Judgment
`apply-progress.md` discloses replacing a `printf ... | grep -qx` pipe (the project's existing
idiom, used verbatim at `migration_status_test.sh:34` and `:42`) with a here-string
(`grep -qx "$command" <<< "$still_routed"`) at line 101, citing a `pipefail`/SIGPIPE race: `grep -q`
can exit as soon as it matches, closing its stdin pipe end while the upstream `printf` may still be
writing; under `set -o pipefail` (active at `migration_status_test.sh:8`) the pipeline's reported
exit becomes `printf`'s SIGPIPE code (141) even though `grep` matched, intermittently tripping the
`||` branch. This is a real, known bash pipeline hazard for early-exiting readers, not a fabricated
justification, and the here-string fix (no forked writer, no pipe) is a standard, correct
mitigation. It changes only new code introduced by this task (line 101); the pre-existing idiom at
lines 34/42 is untouched, exactly as apply-progress states, so it is out of scope for this change.
Judgment: sound engineering deviation, correctly disclosed, does not violate design intent (design
gave a `PARTIAL`/`is_partial()` snippet but did not literally specify the membership-check plumbing
this line implements). No spec or design violation. Classified WARNING, not CRITICAL — not a
regression, but the pre-existing idiom at lines 34/42 shares the theoretical hazard and is left
unaddressed (correctly out of scope here, worth a follow-up).

### Issues Found

**CRITICAL**: None

**WARNING**:
1. Spec scenarios "Describing how a command reaches its implementation" and "No absolute claim
   remains" (ADDED requirement, `specs/go-entrypoint/spec.md:60-75`) have no automated covering
   test — verified only by manual read (by apply and independently replayed here). Per strict-verify
   philosophy this would default to CRITICAL `UNTESTED`; downgraded to WARNING because
   `openspec/config.yaml:85` (`rules.tasks`) explicitly mandates a manual-verification task for this
   project ("Incluye siempre una tarea de verificación manual con un proyecto real"), `tasks.md`
   task 3.1 implements exactly that, and prose-content assertions ("no absolute Bash claim remains")
   are not mechanically testable by any test framework in this repo. Both files were independently
   re-read here and the claim holds.
2. The pre-existing `printf | grep -qx` idiom at `tests/unit/migration_status_test.sh:34` and `:42`
   shares the same theoretical SIGPIPE/`pipefail` hazard identified and fixed at line 101 in this
   change. Out of scope for this change (untouched, pre-existing), but worth a follow-up ticket.

**SUGGESTION**: None

### Verdict
**PASS WITH WARNINGS** — all 14 tasks complete and correspond to real commits; both commits build
and diff exactly as designed; 12/12 focused + 612/612 full Docker-free suite + 172/172 Go tests all
green; zero scope leakage into `console/`, `bin/run/`, `internal/`, `dockergento/`; the RED claim
independently replayed and confirmed true; the disclosed here-string deviation is sound and
correctly scoped. Warnings are informational (manual-only coverage for two prose scenarios,
justified by explicit project convention; a pre-existing latent hazard elsewhere, out of scope).
Recommend proceeding to `sdd-archive`.
