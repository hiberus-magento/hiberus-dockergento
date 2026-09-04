# Exploration: Continuing the Bash → Go migration

Change (umbrella): `continue-go-migration`
Engram topic: `sdd/continue-go-migration/explore` (project `hiberus-dockergento`, observation #2)
Date: 2026-09-03

## Current state

The migration follows a strangler pattern documented in `MIGRATION.md`. `cmd/hm/main.go` builds the real `hm` binary, now the primary entry point. `bin/run` is the original Bash CLI, kept as the bridge for everything not yet ported. `internal/cli/run.go` (`Run()`) switches on the subcommand; anything unmatched falls through to `dockergento/adapters/legacy.Runner.Run`, which execs `bin/run` with the same argv and preserves `HM_REGISTERED*` so bridged calls from inside a worktree keep their registration.

Code layout:

- `dockergento/` — public library: `core` domain, `app` use cases and ports, `adapters/*` (dockerd, registry, fsprops, composecfg, gitvcs, legacy, machine, toolinfo).
- `internal/cli/` — the only private package; terminal driving adapter. `wrappers.go` hosts the "one thing in one container" commands.

## Command inventory (verified against `internal/cli/run.go`, not the table)

`MIGRATION.md` claims 16 of 65 commands are Go. The `run.go` switch shows 14 more commands fully wired to Go but still marked `shell` in the table:

`down, set-host, copy-to-container, copy-from-container, version, purge, npm, n98-magerun, test-unit, test-integration, mysqldump, varnish-on, varnish-off, setup`

All call real domain code (`core.ParseSetup` + `engine.Setup`, `core.DownOptions`, `engine.Exec/Dump/CopyInto/CopyFrom/SetHost`); none are stubs. `db` and `proxy` are correctly partial (`db` routes only `templateSubcommands`, `proxy` only `proxySubcommands`).

Real state: about 30 of 65 commands are Go-wired.

Still shell-only: `ai-*` (5), `cloud`, `cloud-login`, `compatibility`, `config-env`, `create-project`, `db` (snapshot half), `dbeaver`, `debug-on`, `debug-off`, `docker-compose`, `docker-stop-all`, `grunt`, `install`, `launch`, `mcp`, `permissions`, `post-start`, `proxy` (partial), `rebuild`, `sequelace`, `share`, `ssl`, `switch`, `tableplus`, `transfer-db`, `transfer-media`, `tui`, `tunnel`, `update`, `verify`.

No ported command's Bash implementation has been deleted except `worktree.sh` (rewritten as a delegating stub because the registry became a live SQLite database Bash cannot read). Every other ported command keeps its full Bash implementation on disk as the oracle for the Docker-gated parity tests under `tests/integration/go_*_test.sh`.

## The migration rule (`MIGRATION.md`, "Cómo se porta un comando")

1. Use case in `internal/app` against ports, with tests, no Docker.
2. Adapter in `internal/adapters`.
3. Wire in `internal/cli`, remove from the bridge switch.
4. Port the shell integration test so it drives the built binary.
5. Mark it in the `MIGRATION.md` table and bump the counter.

Deletion discipline: ported command → its Go test → then delete the shell implementation and its bash test. Never delete Bash before Go covers the behavior.

`tests/unit/migration_status_test.sh` guards the table but is one-directional: it only asserts that rows marked `go` exist in `internal/cli/`. It never asserts that every case in `run.go`'s switch is marked `go`. That is how the 14-command undercount went undetected.

## Docker communication

`go.mod` pulls `github.com/docker/docker` (daemon SDK), `github.com/docker/compose/v2` and `compose-spec/compose-go/v2` (Compose as a library, ADR-009 bis). `dockergento/adapters/dockerd/exec.go` and `oneoff.go` use the SDK directly (`ContainerExecCreate/Attach/Inspect`, `ContainerCreate/Attach/Start/Wait`). Go never shells out to `docker` or `docker compose`. The one deliberate exception is `dockergento/adapters/gitvcs/vcs.go`, which shells to `git` because go-git cannot model linked worktrees. Bash still builds `$DOCKER_COMPOSE` and shells out for every unported command. Binary cost of embedding Compose: 8.5 MB → 60.6 MB (64.5 MB with SQLite), 70 → 426 modules, accepted per ADR-009 bis (`docs/research/2.0-arquitectura.md`).

## Testing

- `go test ./...` — packages plus `test/e2e/*_test.go`, which builds and runs the binary.
- `./tests/run.sh` — Bash harness: `tests/unit/`, `tests/integration/`, `tests/performance/`. Go runs first (seconds), Bash second (Docker, up to 20 min).
- Gap: no `internal/cli/wrappers_test.go`. The 14 wrapper commands have zero pure-Go unit coverage; only the Docker-gated Bash parity harness exercises them.

## Documentation drift

- `CLAUDE.md`: "entire internal implementation is written in Bash scripts" — false.
- `architecture/02-cli-architecture.md`: "written entirely in Bash" — false.
- `openspec/config.yaml` context: "Implementación: 100 % Bash" and a file list omitting `cmd/`, `dockergento/`, `internal/`, `go.mod`.
- `MIGRATION.md`: counter and table undercount by 14 commands.

## Ranked next-slice candidates

1. **`sync-migration-status`** — reconcile the `MIGRATION.md` table and counter (16 → ~30), add the reverse check to `tests/unit/migration_status_test.sh`, fix the "100 % Bash" claims in `CLAUDE.md`, `architecture/02-cli-architecture.md`, `openspec/config.yaml`. Well under 400 lines, no Docker needed, near-zero risk.
2. **`internal/cli/wrappers_test.go`** — table-driven pure-Go unit tests for the wrapper commands' argument parsing and error paths. Additive, fits the budget, low risk.
3. **Retire superseded Bash and bash tests** for the 14 already-Go commands. `setup.sh` and `down.sh` are the risky part (interactive branching, worktree and proxy overlays, WT-01). 200–400 lines; likely two slices (wrappers batch, then setup/down batch).
4. **Next genuinely unported group** — `ai-*` family (5 files, self-contained) or `docker-compose`/`docker-stop-all`. `launch`, `tunnel`, `share` depend on `hm mysql`-backed domain lookup and should wait.

## Recommendation

Do slice 1 first as a standalone change: it is the honest baseline every later slice's framing depends on. Then slice 2, then slice 3 split as needed, then a fresh exploration for slice 4.

## Risks

- One-directional `migration_status_test.sh` gives false confidence and invites redundant re-porting.
- No pure-Go unit coverage for `internal/cli/wrappers.go`; CI signal depends on Docker availability.
- Four documents disagree with reality in different ways; fix them against `internal/cli/run.go` as the single source of truth.
- `setup`/`down` Bash retirement carries interactive and overlay risk.
