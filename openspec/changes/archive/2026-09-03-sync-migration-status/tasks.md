# Tasks: Synchronize the migration status with reality

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~104 (Commit 1 ~80, Commit 2 ~24), excluding `openspec/changes/` |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single delivery, two work-unit commits |
| Delivery strategy | single-pr (no open PRs in this project — direct commit onto `release/2.0.0`) |
| Chain strategy | pending (not applicable — estimate is well under budget, no chaining decision needed) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Reverse check (RED→GREEN) + `MIGRATION.md` reconciliation | Commit 1 (direct to `release/2.0.0`) | `tests/run.sh tests/unit` | N/A — no routing/subprocess/executable-classification change; a test script reads two files in the repo it lives in (design threat matrix) | Revert `tests/unit/migration_status_test.sh` reverse-check block and `MIGRATION.md` table/counter/prose edits; self-contained |
| 2 | Docs strangler-pattern correction + config absorption | Commit 2 (direct to `release/2.0.0`) | `tests/run.sh tests/unit` | N/A — docs-only, no runtime path touched | Revert `CLAUDE.md`, `architecture/02-cli-architecture.md`, `openspec/config.yaml` prose edits; independent of Commit 1 |

## Phase 1: Commit 1 — reverse check (RED → GREEN)

- [x] 1.1 RED: in `tests/unit/migration_status_test.sh`, add the reverse-check block: `ROUTER="$COMMAND_BIN_DIR/internal/cli/run.go"` (read-only), a `routed()` helper using `sed -n 's/^[[:space:]]*case "\([a-z0-9_-]*\)":.*/\1/p' "$ROUTER" | grep -v '^_' | sort`, and `PARTIAL="db proxy"` with `is_partial()`. Assert every routed command (excluding `_registry`) is tabled `go` in `MIGRATION.md` (read-only at this step); assert `db`/`proxy` are excused from the `go` requirement AND still both routed AND still tabled `shell`.
  Verify: `tests/run.sh tests/unit` → **FAILS**, naming `down set-host copy-to-container copy-from-container version purge npm n98-magerun test-unit test-integration mysqldump varnish-on varnish-off setup` as routed-but-tabled-shell.
- [x] 1.2 GREEN: in `MIGRATION.md`'s command table, flip those same 14 rows from `shell` to `go`.
  Verify: `tests/run.sh tests/unit` → **PASSES**, zero drift reported by the reverse check.
- [x] 1.3 Fix `MIGRATION.md` line 21 counter: `| Comandos en Go | 16 de 65 |` → `| Comandos en Go | 30 de 65 |`.
  Verify: `tests/run.sh tests/unit` → "and the count at the top matches the table" assertion passes.
- [x] 1.4 Fix `MIGRATION.md` line 190: replace the "`copy-to-container` no está portado" claim with the real reason `composer` stays in shell on macOS — `mirrorsVendor` (`internal/cli/php.go:98`, read-only) runs a copy-in/run/copy-back sequence, not one call. Keep Spanish, keep the document's voice.
  Verify: manual read — no remaining claim that `copy-to-container` is unported.
- [x] 1.5 Fix `MIGRATION.md` lines 281-282: rewrite the description of `tests/unit/migration_status_test.sh` (read-only) from one-directional to both directions — rows marked `go` exist in the Go tree, AND every routed command is tabled `go` except the pinned `db`/`proxy` exception.
  Verify: manual read — both directions described.
- [x] 1.6 Run the full Docker-free unit suite: `tests/run.sh tests/unit` (read-only).
  Verify: **PASSES**, all cases green, table and router agree.
- [x] 1.7 Commit: `test(migration): every command the router answers, checked against the table`.
  Prepared but NOT committed — orchestrator settles delivery. Files ready: `tests/unit/migration_status_test.sh`, `MIGRATION.md`.

## Phase 2: Commit 2 — docs strangler-pattern correction + config absorption

- [x] 2.1 Replace `CLAUDE.md`'s line 11 "entirely Bash" claim with one strangler-pattern sentence: `cmd/hm` binary is the entry point, `internal/cli/run.go`'s switch routes ported commands, unported commands fall through to `bin/run` untouched, `MIGRATION.md` tracks which is which.
  Verify: manual read — no absolute "100%/entirely Bash" claim remains.
- [x] 2.2 Replace `architecture/02-cli-architecture.md`'s line 5 equivalent claim with the same strangler-pattern sentence. Keep the flow diagram below line 5 unchanged — it is `bin/run`'s, still what an unported command goes through.
  Verify: manual read — diagram intact, claim corrected.
- [x] 2.3 Absorb the already-patched `openspec/config.yaml` from the working tree into this commit as-is — no further edits.
  Verify: `git diff --stat openspec/config.yaml` shows only the pre-existing working-tree patch, nothing new.
- [x] 2.4 Cross-check `CLAUDE.md` (read-only) and `architecture/02-cli-architecture.md` (read-only) together: neither states or implies the implementation is entirely or 100% Bash (spec scenario "No absolute claim remains").
  Verify: manual read of both files.
- [x] 2.5 Run the full Docker-free unit suite once more: `tests/run.sh tests/unit` (read-only).
  Verify: **PASSES** — docs/config-only change, no regression.
- [x] 2.6 Commit: `docs: the implementation is Go and Bash, not only Bash`.
  Prepared but NOT committed — orchestrator settles delivery. Files ready: `CLAUDE.md`, `architecture/02-cli-architecture.md`, `openspec/config.yaml`.

## Phase 3: Manual verification

- [x] 3.1 Open `MIGRATION.md` (read-only), `CLAUDE.md` (read-only), and `architecture/02-cli-architecture.md` (read-only) in a real checkout and confirm the counter (30 de 65), the 14 flipped rows, and the corrected `copy-to-container`/reverse-check sentences read consistently end to end. No Docker project needed — this change touches no runtime command path (design threat matrix: not applicable).
  Verified: counter reads `30 de 65`; all 14 rows (`down`, `set-host`, `copy-to-container`, `copy-from-container`, `version`, `purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `varnish-on`, `varnish-off`, `setup`) read `go`; `db`/`proxy` still `shell`; `MIGRATION.md:187-191` cites `mirrorsVendor` (`internal/cli/php.go:98`) instead of the stale "not ported" claim; `MIGRATION.md:282-283` (shifted from 281-282 by the edits) describes both directions; `CLAUDE.md` and `architecture/02-cli-architecture.md` carry the strangler-pattern sentence with no absolute Bash claim; the flow diagram in architecture/02 is unchanged.

## Notes

- No file under `console/`, `bin/run`, `internal/`, or `dockergento/` changes (proposal Non-goals; design commit table).
- `db`/`proxy` stay tabled `shell` — conditionally routed, excluded by the pinned `PARTIAL="db proxy"` exception, never silently skipped.
- The `.gitignore` entry for `.codegraph/` in the working tree is orchestrator tooling residue, not part of this change — delivered separately.
