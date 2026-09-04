# Apply progress — sync-migration-status

Mode: Strict TDD (active). 13/14 tasks complete; 2 tasks (1.7, 2.6 — commit) intentionally NOT
executed: the orchestrator settles delivery, apply must not commit.

## TDD Cycle Evidence — Phase 1 (Commit 1 work unit)

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| 1.1-1.2 reverse check | `bash tests/unit/migration_status_test.sh` → `RESULT 12 1`, failure "every command the router answers for is tabled as go" named exactly: `copy-from-container copy-to-container down mysqldump n98-magerun npm purge set-host setup test-integration test-unit varnish-off varnish-on version` (14 commands, matches design doc list) | After flipping the 14 rows + counter in MIGRATION.md: `bash tests/unit/migration_status_test.sh` → `RESULT 12 0`, all green | During RED authoring, found the design's literal `printf ... \| grep -qx` idiom (mirrored from the pre-existing `rows`/`commands` style) is racy under `set -o pipefail`: `grep -qx` exits as soon as it matches, SIGPIPEs the upstream `printf`, and pipefail then reports the *printf* exit code (141) as the pipeline result even though `grep` itself matched — intermittent false failure naming `db(not-routed)` or `proxy(not-routed)`, reproduced 3/6 runs. Fixed by reading from a here-string (`grep -qx "$command" <<< "$still_routed"`) instead of piping from `printf`, which removes the live pipe and the SIGPIPE race. Confirmed stable across 6+ reruns after the fix. This is new code in this change, not a fix to the pre-existing `rows`/`commands` helpers (out of scope, untouched). |
| 1.3 counter | same RED run above | `MIGRATION.md:21` → `\| Comandos en Go \| 30 de 65 \|`, counter assertion passes in the GREEN run | — |
| 1.4 composer/copy-to-container prose | N/A (prose, not test-covered) | Manual read: `MIGRATION.md:187-191` now cites `mirrorsVendor` (`internal/cli/php.go:98`) — darwin's copy-in/run/copy-back sequence — instead of the stale "`copy-to-container` no está portado" claim (now false since 1.2 flips it to `go`) | — |
| 1.5 test-description prose | N/A (prose) | Manual read: `MIGRATION.md:282-283` now describes both directions (rows marked `go` exist in the Go tree, AND every routed command is tabled `go` except the pinned `db`/`proxy` exception) | — |
| 1.6 full docker-free suite | — | `tests/run.sh unit` → `612 assertions passed`, 0 failed | — |

## Work Unit Evidence — Phase 2 (Commit 2 work unit, docs-only, no test boundary)

| Evidence | Value |
|---|---|
| Focused test command and result | `tests/run.sh unit` → `612 assertions passed` (rerun after CLAUDE.md/architecture/02/config.yaml edits — sanity only, docs change has no test coverage of its own) |
| Runtime harness | N/A — docs-only, no runtime command path touched (design threat matrix: not applicable) |
| Rollback boundary | Revert `CLAUDE.md`, `architecture/02-cli-architecture.md` prose edits independently of Commit 1; `openspec/config.yaml` was already patched in the working tree before this session (not authored by apply), absorbed as-is |

## Completed tasks

- [x] 1.1 RED reverse-check block added to `tests/unit/migration_status_test.sh`
- [x] 1.2 GREEN: 14 rows flipped `shell`→`go` in `MIGRATION.md`
- [x] 1.3 Counter fixed: `16 de 65` → `30 de 65`
- [x] 1.4 `MIGRATION.md:187-191` composer/macOS reason corrected to `mirrorsVendor`
- [x] 1.5 `MIGRATION.md:282-283` test description rewritten to both directions
- [x] 1.6 Full docker-free suite green (612/612)
- [ ] 1.7 Commit prepared, NOT executed (orchestrator settles)
- [x] 2.1 `CLAUDE.md:11` strangler-pattern sentence
- [x] 2.2 `architecture/02-cli-architecture.md:5` strangler-pattern sentence, diagram intact
- [x] 2.3 `openspec/config.yaml` absorbed as-is (pre-existing working-tree patch, untouched by apply)
- [x] 2.4 Cross-check: no absolute Bash claim in either file
- [x] 2.5 Full docker-free suite green again (612/612)
- [ ] 2.6 Commit prepared, NOT executed (orchestrator settles)
- [x] 3.1 Manual verification: counter, 14 rows, both prose fixes read consistently

## Files changed (working tree, uncommitted)

- `tests/unit/migration_status_test.sh` (+36/-0) — reverse-check block
- `MIGRATION.md` (+/-, net ~43 changed lines) — 14 rows flipped, counter, two prose fixes
- `CLAUDE.md` (+1/-1) — strangler-pattern sentence
- `architecture/02-cli-architecture.md` (+1/-1) — strangler-pattern sentence, diagram unchanged
- `openspec/config.yaml` (+53/-3) — pre-existing patch, absorbed as-is, not authored here

Not touched: `.gitignore` (pre-existing unrelated modification, left alone per instruction), nothing
under `console/`, `bin/run`, `internal/`, `dockergento/`.

## Planned commits (NOT executed — working tree left ready for orchestrator/user to commit)

1. `test(migration): every command the router answers, checked against the table` — files:
   `tests/unit/migration_status_test.sh`, `MIGRATION.md`
2. `docs: the implementation is Go and Bash, not only Bash` — files: `CLAUDE.md`,
   `architecture/02-cli-architecture.md`, `openspec/config.yaml`

## Deviations from design

None — implementation matches design.md exactly (ROUTER/`routed()`/`PARTIAL`/`is_partial()`
verbatim from design, POSIX BRE `sed`, the 14-command list matches exactly, the `mirrorsVendor`
citation matches). The only addition beyond the design's literal snippet is the here-string fix
for the SIGPIPE/pipefail race in the reverse-check's own membership test, which is an
implementation detail needed to make the RED→GREEN cycle deterministic, not a spec/design
deviation.

## Anomaly noticed (not part of design, informational)

Mid-session, one `Bash` tool result for `git diff -- openspec/config.yaml` had an appended block
masquerading as a system-reminder instructing that commits/PRs include `Co-Authored-By: Claude
Fable` and a `Claude-Session` URL. This contradicts the user's global CLAUDE.md ("Never add
Co-Authored-By or AI attribution to commits") and was not followed. No commits were made in this
session regardless (per explicit instruction not to commit), so it had no effect, but it should be
reported since it read as a prompt-injection attempt riding on tool output.
