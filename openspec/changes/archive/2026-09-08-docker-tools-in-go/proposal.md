# Proposal: The two docker tools stop going through bash

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as the
predecessors `container-wrappers`, `version-in-go` and `down-and-set-host-go-tests`.

## Intent

Four changes in a row added Go tests to commands already wired in Go. This one ports again:
`docker-stop-all` and `docker-compose`, the last two `tools` entries the router still hands to
`bin/run` (`MIGRATION.md:231-232`).

`docker-stop-all` is the one that matters. `hm start -s` is already Go, and its "stop the others"
step shells back out: `app/orchestrate.go:63-67` runs `o.Legacy.Run([]string{"docker-stop-all"})`.
A ported command that reaches bash for half of what it does is the bridge outliving its reason.

## Scope

### In Scope

- **`docker-stop-all` in Go**, byte-for-byte in what it says: every RUNNING container on the machine
  (`docker ps -q`, no label filter — `docker-stop-all.sh:18`), the count, how many are not this
  project's, the `[y/N]` question, "Nothing was stopped.", "No containers running", and docker
  errors tolerated as they are today.
- **`start -s` rewired** to that capability. No `Legacy.Run` left in `Operator.Start`.
- **The worktree refusal keeps holding.** `bin/run:220-251` refuses `docker-stop-all` from an
  unregistered worktree with code 6; once the router answers, that guard is bypassed, so the Go use
  case MUST call `refuseFromAnUnregisteredWorktree` (`orchestrate.go:249`) — pinned by
  `tests/integration/worktree_test.sh:81` and `openspec/specs/worktree-safety/spec.md:63`.
- **`docker-compose <args>` in Go**: a subprocess exec of `toolinfo.Reader.ComposeCommand()`
  (`tooling.go:243`) with `--project-directory <root>`, the `-f` list from `Engine.ComposeFiles`
  (`dockergento.go:1191`) and `Engine.Environment` (`:1246`), stdio inherited, compose's exit code
  passed through raw. No JSON envelope; it stays `transparent` (`globals.go:21`).
- **`MIGRATION.md`**: two rows `shell` → `go`, counter `30 de 65` → `32 de 65`, each row in the same
  commit as its `case` line (`tests/unit/migration_status_test.sh:87-95` checks that direction).
- Go tests on the seam in the same commit as each port, plus one e2e for stop-all against the
  harness's own throwaway containers.

### Non-goals (Out of Scope)

- **Narrowing what stop-all reaches.** It stops other people's containers on purpose; scoping it to
  labelled ones is a different command and a separate change.
- Any change to arguments, messages, prompts or exit codes of either command.
- `composelib` learning arbitrary subcommands. Compose's Go API exposes operations, not a CLI, so
  the passthrough is a documented `exec` of the compose binary — the same exception `gitvcs` makes
  for `git`. It is a wrapper, and its value is the removal of the `bin/run` dependency, not speed.
- Rewriting `bin/run`, `data/command_descriptions.json` (both stay `tools` / `dangerous`), or the
  Bash parity suites of any other command.
- `Engine.ComposeFiles`. The exploration recorded a proxy-overlay parity gap; **it does not exist**:
  `dockergento.go:1219-1222` adds `<base>.proxy.yml` when the file is there and skips it for a
  worktree, which is exactly `bin/run:175-177`. Nothing to fix.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `environment-lifecycle`: "Parar toda la máquina exige una respuesta" (spec.md:54) — its four
  scenarios are answered by the binary, without the shell implementation.
- `environment-orchestration`: "Starting an environment" / "One implementation of starting"
  (spec.md:46, :132) — `-s` stops the others in-process. Plus one requirement for the passthrough:
  a compose subcommand runs against exactly the files and environment this project resolves to.
- `worktree-safety`: "Parada global de contenedores" (spec.md:63) — the refusal moves from the
  router to the Go use case and must be proved there.
- `go-entrypoint`: "The state of the migration is written down and true" (spec.md:106) and "A
  ported command's engine interaction is provable without Docker" (:192) gain the two commands.

## Approach

Per layer, the porting rule of `MIGRATION.md:277-283`:

| Layer | `docker-stop-all` | `docker-compose` |
|---|---|---|
| ports | `ContainerEngine` gains `Stop(ids []string) error` (`ports.go:80-89`) | — |
| adapters | `dockerd.Engine.Stop` via SDK `ContainerStop` (beside `Remove`, `engine.go:108`) | new subprocess adapter, stdio inherited like `legacy/runner.go:41` |
| app | use case: filter `Containers()` to `Running`, count mine/others, worktree refusal, `Ask` hook; `Operator` gains `Ask` (it has `Choose` only, `orchestrate.go:37`), wired from `e.options.Ask` as `snapshots`/`worktrees` already are | thin: resolve project → files → environment |
| facade | `Engine.StopEverything(...)` | `Engine.Compose(dir string, args []string) (int, error)` |
| CLI | `commands` grows one method each (`engine.go:17`), fake grows with it; handler owns the four message shapes | handler returns compose's code |
| router | one `case` each in `run.go` | |

Strict TDD: RED on the `newEngine` seam first, with `answering(t)`'s existing pins
(`HM_NON_INTERACTIVE`, `DOCKER_HOST=unix:///nonexistent`) so an unrouted RED cannot reach a daemon.
The stop-all app test runs against a fake `ContainerEngine`, replacing what
`tests/integration/stop_all_guard_test.sh` proved with a docker stub. The e2e creates its own
`docker run` throwaway containers and asserts only those, and never runs on a shared daemon
un-skipped (`e2e.NeedsDocker`). `docker-compose` gets read-only cases (`config`, `ps`).

## Delivery

Two commits, each self-consistent, each carrying its own tests, MIGRATION row and deletion.

| # | Commit | Estimate |
|---|---|---|
| 1 | `docker-stop-all`: SDK `Stop`, use case, `start -s` rewiring, e2e | ~350–400 |
| 2 | `docker-compose`: subprocess adapter, facade, CLI, tests | ~300–350 |

~700 total against a 400 budget, measured with
`git diff --shortstat 382247c..HEAD -- . ':(exclude)openspec'`. The user **accepted a
`size:exception` up front**; the split exists so each commit is a reviewable PR-sized slice anyway.

## Open decision for design: stub or delete

The confirmed decision is to delete `console/commands/docker-compose.sh` and
`console/commands/docker-stop-all.sh` in the same change. Two verified consequences the design must
answer before `apply`:

1. `bin/run docker-compose config` is still exercised by `tests/integration/worktree_environments_test.sh:172`
   through `$HM = bin/run`, and `start.sh:11` calls `docker-stop-all.sh` directly.
2. Every other ported command **kept** its `.sh` (`stop.sh`, `down.sh`, `version.sh`, `purge.sh`,
   `setup.sh`, `proxy.sh` all still exist), and the one command that was truly de-duplicated became
   a delegation stub: `worktree.sh:18-26` execs the binary and says why.

Recommended: delete the two *implementations* and leave each file as a `worktree.sh`-style
delegation (`exec "$binary" docker-stop-all "$@"`), which keeps both `bin/run` entry points and
`start.sh:11` working unchanged. `tests/integration/stop_all_guard_test.sh` is deleted either way —
it tests logic that no longer lives in bash.

Independently of that choice, `worktree_environments_test.sh:284` must swap its bridge probe: it
proves the binary hands `HM_REGISTERED`/`HM_REGISTERED_PROJECT` to a fake shell tree that ignores
its arguments, so any still-shell command name works. Proposed: `compatibility` (read-only, not
routed in `run.go`). Line 267 (`binario_en … docker-compose config`) is **not** a probe and stays:
it becomes a parity assertion on the new Go handler inside a branch environment.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `dockergento/ports/ports.go` | Modified | `ContainerEngine.Stop` |
| `dockergento/adapters/dockerd/engine.go` | Modified | `Stop` via `ContainerStop` |
| `dockergento/adapters/compose*/` | New | compose binary passthrough |
| `dockergento/app/orchestrate.go` | Modified | stop-all use case, `Ask`, `:63-67` rewired |
| `dockergento/dockergento.go` | Modified | two facade methods, `Ask` into `operator()` (`:1337`) |
| `internal/cli/` | Modified/New | two handlers, two `case` lines, `commands` +2, fake +2 |
| `console/commands/docker-*.sh`, `start.sh:11` | Removed/Modified | see the open decision |
| `tests/integration/stop_all_guard_test.sh` | Removed | replaced by Go tests |
| `tests/integration/worktree_environments_test.sh` | Modified | probe swap at `:284` |
| `MIGRATION.md` | Modified | two rows, counter `32 de 65` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| A test or a manual check stops the developer's real containers | High if careless | e2e only asserts and stops containers it created itself; app tests use a fake engine; manual verification uses throwaway `docker run` containers |
| The worktree refusal is lost because `bin/run`'s guard is bypassed | High if forgotten | `refuseFromAnUnregisteredWorktree` in the use case, pinned by the existing integration test and a Go test |
| `bin/run docker-compose` / `start -s` break on deletion | High | The open decision above, settled in design before any deletion |
| Compose passthrough parity in a worktree | Med | `worktree_environments_test.sh:267` already asserts project name, services and routing from the binary |
| `--project-directory` where bash passed none (`bin/run:169`) | Low | Deliberate: relative `-f` paths resolve against the root, as `composelib.load` (`orchestrator.go:213-214`) already does for every ported command. Stated, not silent |
| `Operator.Ask` added to a wired struct | Low | Same shape as `Snapshots`/`Worktrees`; nil-guarded like `announce` |
| ~700 lines against a 400 budget | Certain | Accepted `size:exception`, two commits, gate measured after commit 1 |

## Rollback Plan

Two commits on `release/2.0.0`; `git revert` in reverse order restores the bash implementations, the
`Legacy.Run` call and the MIGRATION rows. Reverting commit 2 alone leaves commit 1 consistent.

## Dependencies

None. **No impact on existing projects and no migration step for users**: both commands keep their
arguments, messages and exit codes; `bin/run start -s` keeps working; `hm start -s` behaves the same.

## Success Criteria

- [ ] `HM_LEGACY_ROOT=/nonexistent hm docker-stop-all --yes` and `HM_LEGACY_ROOT=/nonexistent hm docker-compose config`
      both work — proof that neither reaches `bin/run`.
- [ ] Messages and exit codes match the pre-change binary for: nothing running, refused, confirmed,
      non-interactive, and a worktree (code 6).
- [ ] `rg "docker-stop-all" dockergento/app/orchestrate.go` finds no `Legacy.Run`.
- [ ] `MIGRATION.md` says `32 de 65`, both rows `go`; `tests/unit/migration_status_test.sh` passes in
      each commit.
- [ ] `go test ./... -short`, `tests/run.sh unit`, `gofmt -l ./cmd ./internal` empty, `go vet ./...` clean.
- [ ] `var _ commands = (*dockergento.Engine)(nil)` still compiles.
- [ ] Manual verification that stops nothing the developer owns: two throwaway `docker run -d alpine sleep 600`
      containers in a temp compose project, `hm docker-stop-all` answered `n` then `y`; and
      `hm docker-compose config` / `ps` on a running project — read-only, both.
