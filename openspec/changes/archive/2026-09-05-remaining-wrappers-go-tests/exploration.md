# Exploration: remaining-wrappers-go-tests

Change: `remaining-wrappers-go-tests`
Engram topic: `sdd/remaining-wrappers-go-tests/explore` (project `hiberus-dockergento`, observation #160)
Date: 2026-09-05

Gives the nine remaining Go-wired but untested commands (`down`, `set-host`, `version`, `purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`) Go tests on the `commands` seam introduced by the archived `wrappers-go-tests` change (`internal/cli/engine.go`, `internal/cli/fake_engine_test.go`, `internal/cli/wrappers_test.go`).

## Per-handler table

| Command | Handler | Engine methods called | Call sites to route | In `commands` today? |
|---|---|---|---|---|
| `purge`, `npm`, `n98-magerun` | `wrappers.go:27/38/49` | none directly; `projectOr` + `inside` (already routed) | none | n/a |
| `test-unit`, `test-integration` | `wrappers.go:63 tests` | `Property(project, "BIN_DIR")` (:69); integration also `Property(project, "WORKDIR_PHP")` (:77); then `inside` | `wrappers.go:69`, `:77` | no: `Property` missing |
| `mysqldump` | `wrappers.go:94 dump` | `Dump(here(), args[0])` (:104) after `projectOr` (:100) | `wrappers.go:104` | no: `Dump` missing |
| `set-host` | `wrappers.go:259 setHost` | `RemoveHost(domain)` (:281, `--remove` branch, no `projectOr`); `SetHost(here(), domain, database)` (:296) after `projectOr` (:292) | `wrappers.go:281`, `:296` | no: both missing |
| `version` | `version.go:17 version` | `Installed()` (:23); no `projectOr` by design | `version.go:23` | no: `Installed` missing |
| `down` | `down.go:20 down` | `projectOr` (:55), then `Down(here(), options, interactive)` (:61) | `down.go:61` | no: `Down` missing |

Seven call sites switch from `engine(` to `newEngine(` across three files. Signatures, verified in `dockergento/dockergento.go`:

```go
func (e *Engine) Down(dir string, options core.DownOptions, interactive bool) (string, error) // :235
func (e *Engine) Dump(dir, path string) error                                                  // :322
func (e *Engine) Installed() (core.Installation, core.Tooling)                                 // :468
func (e *Engine) SetHost(dir, domain string, database bool) error                              // :479
func (e *Engine) RemoveHost(domain string) error                                               // :489
func (e *Engine) Property(project core.Project, key string) string                             // :1239
```

`down` and `dump` pass `here()` rather than the resolved `project.Root`, the same inconsistency the predecessor noted for `copyInto`/`copyFrom`. Out of scope to reconcile; tests will assert `Dir == cwd`.

## Interface growth and cmp.Diff feasibility

`commands` grows from 5 to 11 methods. The new argument types are plain structs with exported fields (`core.DownOptions`: `Volumes`, `RemoveOrphans`, `Images`, `Timeout *int`), so `cmp.Diff` needs no new options beyond the existing `cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")`. `core.Installation` and `core.Tooling` are return values only and never enter the call log.

Two new methods return no error (`Installed`, `Property`), so the outcome-by-call-index scheme does not fit them; the fake needs answer fields (`properties map[string]string`, `installed`, `tooling`). `Down` returns `(string, error)`, the one shape mismatch against `outcome{status, err}`; the design decides the mechanism.

`call` gains fields: `Domain`, `Database`, `Interactive`, `DownOptions core.DownOptions`, `Key`, `Path`.

## Behaviour to pin (from the Bash parity suites)

- `purge`: `Resolve` then `Exec(sh -c "rm -rf …")` with the exact generated list.
- `npm`, `n98-magerun`: argument pass-through; magerun joins args into `bash -c "n98-magerun …"`.
- `test-unit` / `test-integration`: unit runs `binDir + "/phpunit --config ./dev/tests/unit/phpunit.xml.dist"`; integration runs `cd ./dev/tests/integration && $WORKDIR_PHP/$BIN_DIR/phpunit --config phpunit.xml`; args appended; property fallbacks `./vendor/bin` and `/var/www/html` when `Property` answers "".
- `mysqldump`: no path is a usage error before any engine call; a path calls `projectOr` then `Dump`; JSON `{"path": …}`; engine refusal maps to `exitDocker`.
- `version`: no project needed; JSON shape from `Installed()` plus the pure `buildOfThisBinary()`; text uses `orMissing`/`orUnknown`; unknown option is a usage error with no engine call.
- `down`: flag parsing (`-v/--volumes`, `--remove-orphans`, `--rmi`, `-t/--timeout`, non-numeric timeout and unknown flag are usage errors) happens before `projectOr`; `interactive` is `HM_NON_INTERACTIVE == ""`. The Ask/Choose flow lives in `dockergento/app.Operator.Down` (`orchestrate.go:350`), below the seam, so the fake only answers the three `(string, error)` results: `"destroyed"`/`"saved"` → JSON `{destroyed, volumes, snapshot}`; `""` → "Nothing was destroyed."; a `core.Refusal` → `report()`'s `asRefusal` branch, which no test in this family exercises yet.
- `set-host`: `--remove` skips `projectOr` and calls `RemoveHost` only, JSON `{"removed": domain}`; otherwise `projectOr` then `SetHost(here(), domain, database)`, JSON `{domain, database}`.

## Conventions

No `t.Parallel()` in `internal/cli`. Literate style. `-short`. gofmt scope `./cmd ./internal`. Baseline re-confirmed at HEAD 5e9f443 by the orchestrator: `go test ./internal/cli -short` 28 passed.

## Docs

`MIGRATION.md`'s "Cómo se escriben" bullet describes the seam generically and needs no update. The command table already marks the nine as `go`.

## Size, measured, and the two-change split

Predecessor density: 43 lines per test, 6 to 10 lines per fake method body. Estimated test bodies: purge 25, npm 25, n98-magerun 25, test-unit + test-integration 65, mysqldump 55, version 55, down 100, set-host 80.

- **Change A, `remaining-wrappers-go-tests`**: the `inside` family (`purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`) plus `mysqldump` and `version`. No branching logic beyond argument construction and output shaping. Interface gains `Property`, `Dump`, `Installed`. About 300 to 305 lines.
- **Change B, follow-up**: `down` and `set-host`, the two with real branching (`down`'s three-way result and refusal mapping; `set-host`'s `--remove` fork) and the one open shape decision (`Down`'s result string). Interface gains `Down`, `SetHost`, `RemoveHost`. About 232 to 238 lines.

Combined, about 530 to 545 lines: one change would overrun the 400-line budget by more than the predecessor did. The split removes the risk instead of asking for another exception.

## Risks and open questions for design

1. `Down`'s `(string, error)` needs a fake mechanism (result field vs separate outcome type). Change B.
2. No test exercises `report()`'s `asRefusal` mapping; `down` is the natural first place. Change B.
3. `here()` vs `project.Root` in `down` and `dump`: tests assert `cwd`; call it out so reviewers do not read it as an oversight.
4. `set-host --remove` must not set `fake.project`; it is an intended success path without a project.
5. Baseline counts confirmed at HEAD (28 in `internal/cli`).
