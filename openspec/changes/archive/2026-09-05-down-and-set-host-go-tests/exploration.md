# Exploration: down-and-set-host-go-tests

Change: `down-and-set-host-go-tests`
Engram topic: `sdd/down-and-set-host-go-tests/explore` (project `hiberus-dockergento`, observation #238)
Date: 2026-09-05

Gives the last three Go-wired but untested commands (`down`, `set-host`, `version`) Go tests on the existing `commands` seam. Re-verifies `sdd/remaining-wrappers-go-tests/explore` (#160) at HEAD 7417863: call sites `down.go:61`, `wrappers.go:281` (`RemoveHost`), `wrappers.go:296` (`SetHost`), `version.go:23` (`Installed`); interface today has seven methods.

## Fake shape for `Down`

Add `result string` to the shared `outcome{status, err}` struct; `fakeEngine.Down` reads it the way `Exec` reads `status`. Rejected: a dedicated field on the fake (breaks the by-call-index convention) and a second outcome type. `call` gains `Domain`, `Database`, `Interactive`, `DownOptions core.DownOptions` (`Options` is already `core.ExecOptions`). `core.DownOptions` (`dockergento/core/orchestration.go:44-58`): `Volumes`, `RemoveOrphans`, `Images`, `Timeout *int`; all exported, `cmp.Diff` needs no new options. `core.Installation`/`core.Tooling` are return-only: fake fields `installed`, `tooling`.

## `down` branches to pin

| Input | Effect |
|---|---|
| `-v`/`--volumes`, `--remove-orphans`, `--rmi <all\|local>`, `-t/--timeout N` | set options; continue |
| `--rmi` or `-t` with no value | silently ignored (found, not fixed) |
| `-t nada` | `exitUsage`, `invalid_argument`, "The timeout is a number of seconds: nada", hint `hm down -t 30`; empty log |
| unknown flag | `exitUsage`, "Unknown option: …", hint `hm down [-v]`; empty log |

After flags: `projectOr` (already routed), then `Down(here(), options, interactive)` with `interactive = HM_NON_INTERACTIVE == ""`. Results: `"destroyed"`/`"saved"` → JSON `{destroyed:true, volumes, snapshot}` and no text outside `--json`; `""` → prints "Nothing was destroyed." unconditionally, even under `--json` (found, not fixed; pin literally). Errors through `report()`: `errNothingChosen` → "Nothing was chosen." and `exitInterrupted`; `core.Refusal` → `asRefusal` mapping with the refusal's own code/kind/message/hint (first test in the family to cover it); else `exitDocker`. The interactive Ask/Choose flow lives in `app.Operator.Down`, below the seam.

## `set-host` branches

`--remove` skips `projectOr` and calls `RemoveHost(domain)` only; JSON `{"removed": domain}`; no text otherwise. Default path: `projectOr` then `SetHost(here(), domain, database)` with `--no-database` flipping `database`; JSON `{"domain", "database"}`. Unknown `-` flag → usage error, hint `hm set-host shop.test`, empty log. No positional domain is not a CLI-level error: `""` is forwarded; the engine's `no_domain` refusal is reachable only through a fake outcome.

## `version`

Any argument → usage error, hint `hm version`, empty log. Otherwise `Installed()` alone in the log; JSON fields `version, tag, commits_ahead, commit, branch, detached, dirty, path, binary, docker.{version, compose, compose_command}`; `binary` from the pure `buildOfThisBinary()`, asserted separately. Text uses `orUnknown` for tag/commit and `orMissing` ("not available") for docker/compose. `Installed()` reaches the daemon via `toolinfo.Reader.DockerVersion` → `ServerVersion(context.Background())` with no timeout; `DOCKER_HOST=unix:///nonexistent` fails fast.

## RED safety (the finding that gates design and apply)

During the RED window `wrappers.go:281/296` still call the real engine. `Engine.hosts()` (`dockergento.go:491-500`) reads `HM_HOSTS_FILE`; `app.Hosts.file()` (`hosts.go:181-187`) falls back to the literal `/etc/hosts`. `Hosts.Set`/`Remove` read that file and `write()` (`hosts.go:148-179`) opens it for writing, falling back to `exec.Command("sudo", "cp", …)` wired to the real terminal (`hosts.go:173`). An unrouted RED run of `set-host --remove` could therefore prompt for the developer's password and mutate the real hosts file.

Mitigation, confirmed by reading `Set`/`Remove`/`point`/`write`: pin `HM_HOSTS_FILE` to `filepath.Join(t.TempDir(), "hosts")`. A missing file fails at `os.ReadFile` before any write; an owned file is opened directly without `sudo`. `Hosts` never calls Ask/Choose.

`down` unrouted reaches `presentVolumes` (daemon dial) and could reach the real `Choose`. `choose()` (`select.go:39`) refuses immediately when `HM_NON_INTERACTIVE` is set, independently of the `interactive` bool. Pin `HM_NON_INTERACTIVE=1` and `DOCKER_HOST=unix:///nonexistent`.

Recommendation: extend the shared `answering(t)` helper to pin all three (`HM_HOSTS_FILE`, `HM_NON_INTERACTIVE`, `DOCKER_HOST`) unconditionally, alongside `HM_STATE_DIR`. A `down` subtest that asserts `Interactive: true` overrides `HM_NON_INTERACTIVE` with a nested `t.Setenv` once the site is routed.

## Size at the corrected density (~1.5x)

| Item | Estimate |
|---|---|
| `down` tests | ~150 |
| `set-host` tests | ~120 |
| `version` tests | ~85 |
| fake growth (4 methods, new fields, `outcome.result`) | ~50 |
| routing (4 sites) + interface (4 signatures) | ~13 |
| Total | ~418 |

Over the 400 budget by a small margin if delivered as one unit. Options: two changes (`version` ~104; `down`+`set-host` ~314), or one change with three commits ordered `version`, `set-host`, `down`, a size gate after the second, and `down` as the cut into a follow-up. The orchestrator recommends the latter to avoid a fourth change for 104 lines.

## Conventions

No `t.Parallel()`; literate table style; `-short`; gofmt `./cmd ./internal`. Baselines at HEAD: 44 in `internal/cli`, 204 overall (carried over; apply re-baselines).

## Risks

1. Real `/etc/hosts` and `sudo` reachable in RED without `HM_HOSTS_FILE`. Non-negotiable pin.
2. `down` RED could dial the daemon or reach `Choose` without the other two pins.
3. Combined size over budget; split or cut order needed.
4. Two found-not-fixed quirks to pin literally: `down` prints text under `--json` when nothing was destroyed; `--rmi`/`-t` without a value are silently ignored.
5. No CLI guard for a missing domain in `set-host` and `down`.
