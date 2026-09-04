# Exploration: pure-Go unit tests for the Go-wired command entry points

Change: `wrappers-go-tests`
Engram topic: `sdd/wrappers-go-tests/explore` (project `hiberus-dockergento`, observation #73)
Date: 2026-09-04

## Current state

`internal/cli/run.go`'s `Run()` wires 14 commands to Go handlers that have no pure-Go unit tests: `down, set-host, copy-to-container, copy-from-container, version, purge, npm, n98-magerun, test-unit, test-integration, mysqldump, varnish-on, varnish-off, setup`. Their only test signal is the Docker-gated Bash integration suite under `tests/integration/go_*_test.sh`, skipped wherever `go` or a Docker daemon is missing.

The existing `internal/cli/*_test.go` files (`select_test.go`, `output_test.go`, `php_test.go`) test only engine-free logic: `move`, `renderOptions`, `choose`, `isTerminal`, `mirrorsVendor`, `writesDependencies`, `report`.

## The seam finding

`internal/cli/engine.go:20` declares `func engine(stdout, stderr io.Writer, jsonOutput bool) *dockergento.Engine`, which builds a concrete engine through `dockergento.New(dockergento.Options{...})` on every call. It is a package function, not a variable, not an interface, not a parameter. No fake exists in `internal/cli`.

One layer down, `dockergento/ports/ports.go` declares 18 interfaces (`Properties`, `VCS`, `Registry`, `ContainerEngine`, `Orchestrator`, `ContainerRunner`, `FileTransfer`, and others) with fakes used by `dockergento/app/*_test.go`. That seam does not reach the CLI layer.

Consequence: without a new seam, pure-Go tests of the 14 handlers can only cover argument parsing and usage errors that return before any engine call, plus the Docker-free `Resolve()` "not a project" path used by `projectOr()`.

## Per-command surface

| Command | Handler | Pure-testable today | Docker-only | Existing coverage |
|---|---|---|---|---|
| `down` | `down.go:20` | unknown option, non-numeric timeout | `Down()` and "nothing destroyed" branch | `go_down_test.sh` (11 cases) |
| `set-host` | `wrappers.go:259` | unknown option | `SetHost`/`RemoveHost` | indirect only, via `go_setup_test.sh:138` |
| `copy-to-container` | `wrappers.go:206` | missing path | `CopyInto()` | none anywhere |
| `copy-from-container` | `wrappers.go:233` | missing path | `CopyFrom()` | none anywhere |
| `version` | `version.go:17` | unknown option; `buildOfThisBinary`, `orMissing` are pure | `Installed()` | `go_wrappers_test.sh` |
| `purge`, `npm`, `n98-magerun` | `wrappers.go:27/38/49` | "not a project" path | `Exec()` inside container | `go_wrappers_test.sh` |
| `test-unit`, `test-integration` | `wrappers.go:63` | "not a project" path | `Property()` + `Exec()` | `go_wrappers_test.sh` |
| `mysqldump` | `wrappers.go:94` | missing path | `Dump()` | `go_wrappers_test.sh` |
| `varnish-on`, `varnish-off` | `wrappers.go:134` | "not a project" path | `Exec`/`Restart` sequence, purge and cache-clean cascade | none anywhere |
| `setup` | `setup.go:20` | nothing new: `core.ParseSetup` already tested in `dockergento/core/setup_test.go` | `Setup()` + `Resolve()` | `go_setup_test.sh` (~20 cases) |

New finding: `copy-to-container`, `copy-from-container`, `varnish-on` and `varnish-off` have zero test coverage of any kind in the repository.

## Conventions

- Literate style: a prose comment block explaining the why, then one function per behaviour. Direct `t.Fatalf`/`t.Errorf`, no assertion libraries.
- `t.Setenv` is used for env-dependent branches; `internal/cli` tests do not run with `t.Parallel()`.
- `t.TempDir()` for filesystem-touching tests.
- CI runs `gofmt -l ./cmd ./internal`, so `internal/cli` is linted.
- Baseline: `go test ./... -short` passed 172 tests in 22 packages.

## The two debts

1. `internal/cli/php.go:91-97`, comment above `mirrorsVendor` (line 98): line 96 says the mac Composer flow "depends on `copy-to-container`, which is not ported". Wrong since `run.go:102-103` wires it to `copyInto()`. The accurate reason is that the whole vendor-mirror procedure is still one Bash-side flow gated by `mirrorsVendor()`.
2. `tests/unit/migration_status_test.sh:34` and `:42` use `printf ... | grep -qx` and `commands | grep -qx` under `set -uo pipefail` (line 9). `grep -q` exits early and can SIGPIPE the producer. Fix with here-strings, matching the reverse check added in `sync-migration-status`.

## Size and split

- Usage-error and "not a project" tests for the 13 non-setup commands: about 300 to 380 lines across two or three new files, given the literate comment style.
- `setup`: nothing new worth writing.
- Two debts: under 20 lines combined.
- Total: roughly 320 to 400 lines, tight against the 400-line budget.

The explorer recommends one change, no setup/down split, and explicitly recommends NOT adding an engine seam in this slice because it touches runtime code and is a design decision of its own. The orchestrator disagrees on value and puts the choice to the user: usage-error-only tests give little confidence for retiring Bash, which is the stated purpose of this slice.

## Risks

- Usage-error tests leave the Docker-dependent core of the four uncovered commands unverified.
- Literate comment density may push the diff past 400 lines.
- `set-host`'s valid-domain dispatch stays untested by any suite either way.
