# Exploration: docker-tools-in-go

Change: `docker-tools-in-go`
Engram topic: `sdd/docker-tools-in-go/explore` (project `hiberus-dockergento`, observation #370)
Date: 2026-09-08

Ports `docker-compose` and `docker-stop-all` from Bash to Go. A real migration slice: use case, adapter, CLI wiring, tests on the seam in the same commit, integration test ported, `MIGRATION.md` updated, Bash deleted.

## Bash behaviour today

**`docker-compose`** (`console/commands/docker-compose.sh`, 4 lines): `$DOCKER_COMPOSE "$@"`. Pure passthrough of any compose subcommand, with `$DOCKER_COMPOSE` resolved by `bin/run` (compose v2 or v1, `--project-directory`, `-f` base + platform overlay, worktree overlay, proxy overlay when `USE_PROXY`). Group `tools`, safety `dangerous`. Needs a resolvable project with a valid compose config. No dedicated Bash test; referenced by `command_safety_test.sh:88` and `permissions_test.sh:33`. `internal/cli/globals.go:21` already lists it as `transparent`.

**`docker-stop-all`** (`console/commands/docker-stop-all.sh`, 52 lines): stops every running container on the machine, labelled or not (`docker ps -q`, no `-a`, no label filter). Counts total and "mine" (`com.docker.compose.project` == `COMPOSE_PROJECT_NAME`), reports "This stops N container(s) on this machine." and "M of them do not belong to '<project>'.", asks "Stop them all? [y/N]:" unless non-interactive; no → "Nothing was stopped."; zero running → "No containers running". Swallows docker errors. Still needs a project name because `bin/run` resolves it before the command runs. Covered by `tests/integration/stop_all_guard_test.sh` (92 lines, stubbed docker).

**Critical link**: `dockergento/app/orchestrate.go:63-67`, the already-ported Go `start -s`, does `o.Legacy.Run([]string{"docker-stop-all"})`: it shells back to Bash for this behaviour. Porting `docker-stop-all` means rewiring `start -s` too. `console/commands/start.sh:11` also calls the `.sh` directly for the pure-shell entry point.

## Go side: what exists, what is new

- Routing: `internal/cli/run.go` switch; unmatched falls through to `Shell` → `ports.Legacy.Run` → `bin/run`. Two new `case` lines.
- `docker-compose`: `composelib` (ADR-009 bis) implements only Up, Down, Stop, Logs, Exec against the compose-go library; it cannot run an arbitrary subcommand. A port is a subprocess exec of the resolved compose binary (`toolinfo.Reader.ComposeCommand()`) with `--project-directory` and the `-f` list from `Engine.ComposeFiles(project)` (which today omits the proxy overlay: parity gap to fix) and `Engine.Environment(project)`, inheriting stdio like `legacy/runner.go`. New facade method, new `commands` method, CLI handler returning compose's raw exit code.
- `docker-stop-all`: `ports.ContainerEngine` has `Containers()` (lists all, `All: true`) and `Remove`, no `Stop`. New: `dockerd.Engine.Stop(ids)` (SDK `ContainerStop`), a use case filtering to running containers, counting mine/others, an `Ask` hook (the `Operator` has `Choose` but no `Ask`), a facade method, a `commands` method, and a CLI handler owning the four message shapes. Rewire `Operator.Start`'s `stopOthers` branch to the new capability.

## Porting rule applied

1. Core/app: `ContainerEngine.Stop`, stop-all use case and result type, `Operator.Ask`, rewired `stopOthers`.
2. Adapters: compose passthrough (subprocess), `dockerd.Engine.Stop`.
3. CLI: `docker_compose.go`, `docker_stop_all.go`, two `case` lines.
4. Tests ported: `stop_all_guard_test.sh` scenarios become Go tests against a fake `ContainerEngine` plus one `test/e2e` scenario stopping the harness's own throwaway containers only; `docker-compose` gains read-only cases (`config`, `ps`) in `go_passthrough_test.sh`.
5. `MIGRATION.md`: two rows `shell` → `go`, counter 30 → 32 de 65. `tests/unit/migration_status_test.sh` enforces this in the same commit as the `case` lines.

## Deletion discipline

Same change, after the Go tests: `console/commands/docker-compose.sh` (no other reference), `console/commands/docker-stop-all.sh` (but `start.sh:11` calls it: either delegate that line to the binary or keep a stub like `worktree.sh`), `tests/integration/stop_all_guard_test.sh`. `data/command_descriptions.json` and `bin/run` untouched.

## Risks

1. Scope of `docker-stop-all`: every running container on the machine. Preserve exactly; any narrowing is a separate behaviour change.
2. `tests/integration/worktree_environments_test.sh:172,267,284` use `docker-compose config` as the probe for the Bash bridge; once ported they stop testing the bridge and need another still-shell probe.
3. `start.sh:11` needs a decision (delegate or stub).
4. `Engine.ComposeFiles` omits the proxy overlay; `docker-compose` parity for proxied projects needs it.
5. `Operator.Ask` is a change to an existing wired struct.
6. Porting `docker-compose` as a subprocess wrapper adds Go that shells out to `docker compose`, against the migration's "SDK, not shell-outs" direction; its value is removing four lines of Bash and the `bin/run` dependency for that command.

## Size and split

At measured density (tests ~1.8x raw, production ~1.2x): production ~335 raw; unit tests ~470; e2e ~215; deletions 148; probe swap 20. Well over 400 combined. Recommended: two work units. `docker-stop-all` (SDK `Stop`, use case, `start -s` rewiring, e2e) ~350-400; `docker-compose` (subprocess adapter, facade, CLI, tests, deletion) ~300-350. The explorer proposed `docker-compose` first as the smaller, independent one; the orchestrator recommends `docker-stop-all` first because it uses the SDK, un-bridges an already-ported command, and delivers the migration's stated value.
