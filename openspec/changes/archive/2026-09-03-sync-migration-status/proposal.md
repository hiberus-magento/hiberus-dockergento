# Proposal: Synchronize the migration status with reality

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by `MIGRATION.md` and ADR-009 bis in `docs/research/2.0-arquitectura.md`.

## Intent

`MIGRATION.md:21` declares `| Comandos en Go | 16 de 65 |`, and exactly 16 table rows say `go`. `internal/cli/run.go` routes 32 commands (33 `case` labels minus `_registry`). Fourteen fully wired commands are still tabled as `shell`. `tests/unit/migration_status_test.sh:54-60` only checks one direction — rows marked `go` must exist in `internal/cli/` — so the undercount is invisible to the suite. Two documents still claim the implementation is entirely Bash.

## Scope

### In Scope

- `tests/unit/migration_status_test.sh`: add the reverse check — every `case "<cmd>":` in `Run()` must be tabled `go`. Written first; must fail against today's table.
- `MIGRATION.md`: flip 14 rows to `go` (`down`, `set-host`, `copy-to-container`, `copy-from-container`, `version`, `purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `varnish-on`, `varnish-off`, `setup`) and fix line 21 to the verified count. Spanish, per the file's convention.
- `MIGRATION.md`: and the two places where it contradicts itself, corrected in the same pass — line 190 says `copy-to-container` is not ported while line 148 records porting it, and lines 281-282 describe the check as one-directional, which it stops being.
- `CLAUDE.md:11` and `architecture/02-cli-architecture.md:5`: replace the "entirely Bash" claims with one accurate strangler paragraph (`cmd/hm` binary, `bin/run` bridge, `internal/cli/run.go` switch).
- Absorb the already-patched `openspec/config.yaml` into this change's commit.

### Out of Scope (Non-goals)

- `internal/cli/wrappers_test.go`; deleting any Bash implementation; porting any new command; editing `run.go`.
- Marking `db` / `proxy` as `go`: both are conditionally routed (`templateSubcommands`, `proxySubcommands`).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `go-entrypoint`: requirement "The state of the migration is written down and true" gains the reverse-direction guarantee.

## Approach

`internal/cli/run.go` is the single source of truth; every document is reconciled against it, never against another document. The new check parses anchored `case "<cmd>":` labels, excludes `_registry`, and asserts each is `go`.

Open design decision: `migration_status_test.sh:47` admits only `shell` or `go` as owners, so `db`/`proxy` need either a documented exception list in the reverse check or a third vocabulary value applied to both test and table. Design phase decides.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `tests/unit/migration_status_test.sh` | Modified | Reverse check (TDD first) |
| `MIGRATION.md` | Modified | 14 rows + counter (line 21) |
| `CLAUDE.md` | Modified | Line 11 strangler paragraph |
| `architecture/02-cli-architecture.md` | Modified | Line 5 strangler paragraph |
| `openspec/config.yaml` | Modified | Absorb existing working-tree patch |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Hardcoded `db`/`proxy` exception drifts once fully ported | Med | Comment the disappearance condition, as the `_registry` row already does |
| Parsing Go source with grep/awk is portability-sensitive (BSD vs busybox already bit this suite) | Med | Anchored POSIX pattern, no tabs in the expression |
| Counter and table diverge again | Low | Existing assertions at lines 62-69 already couple them |

## Rollback Plan

Single commit on `release/2.0.0`; `git revert` restores it. Only documents and one Bash test change — no runtime code path is touched, so nothing to un-deploy.

## Dependencies

None. No effect on existing projects and no migration step for users.

## Success Criteria

- [ ] The new reverse check fails against the current `MIGRATION.md`, then passes after the table fix.
- [ ] `tests/run.sh tests/unit` passes (Docker-free path).
- [ ] No `case` in `Run()` except `_registry` is tabled `shell`, apart from the documented `db`/`proxy` exception.
- [ ] No file claims a 100% Bash implementation.
- [ ] Changed lines stay well under the 400-line review budget.
