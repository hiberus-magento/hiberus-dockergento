# Exploration: retire-ported-shell-commands

Change: `retire-ported-shell-commands`
Engram topic: `sdd/retire-ported-shell-commands/explore` (project `hiberus-dockergento`, observation #464)
Date: 2026-09-09

MIGRATION.md's rule is: a command is ported, then it gets its Go test, then its shell implementation and its bash test are deleted. The first two steps are done for twelve commands. The third has never been executed for any of them.

## The twelve, verified

All twelve route to Go in `internal/cli/run.go` before the bridge is reached, and all twelve have a Go test on the seam: `copy-to-container`, `copy-from-container`, `varnish-on`, `varnish-off` and `mysqldump` in `wrappers_test.go`; `purge`, `npm`, `n98-magerun`, `test-unit` and `test-integration` in `inside_test.go`; `version` in `version_test.go`; `set-host` in `set_host_test.go`. Their shell twins come to 403 lines. No other command qualifies today: `docker-stop-all` and `docker-compose` went through this in the previous change, and the rest of the ported commands still have no Go test of their own.

## What stands in the way

**`hm setup` runs the Bash twins on purpose.** `dockergento/app/setup.go:516-533` builds a list of install steps and runs every one through `Legacy.Run`, which is `bin/run`. Five of those steps are commands that are already ported: `start`, `mysql`, `magento`, `restart` and `set-host`. The comment at `:507-509` says why: three of the steps are not ported yet, and running the other three through the same door keeps the order in one place. So today a Go `hm setup` executes shell implementations, and deleting `set-host.sh` breaks it.

**Four callers reach the scripts by path, not through `bin/run`.** `console/tasks/magento_installation.sh:32` runs `set-host.sh`; `console/commands/composer.sh:39` and `:51` run `copy-to-container.sh`; `console/commands/debug-on.sh:22` runs `copy-from-container.sh`. Because they use `"$COMMANDS_DIR"/<script>`, no amount of routing inside `bin/run` helps them: they need either the file or a one-line change each.

**`hm --help` lists what has a file, not what the tool offers.** `console/helpers/print_help.sh:42` globs `console/commands/*.sh` for the names and then joins them against `data/command_descriptions.json` for the descriptions. The JSON already carries all 65 commands, so it is the better source; the glob is why a command that becomes Go-only would quietly vanish from the help.

**Two test files assert against the shell side.** `tests/integration/go_wrappers_test.sh` has 17 test cases, of which 6 run the shell and the binary and compare them; those die with the shell and must be rewritten as binary-only assertions rather than left to pass while checking nothing. `tests/integration/concurrent_safety_test.sh:105-109` asserts the literal contents of `set-host.sh`; the same fact is already pinned by `dockergento/core/hosts_test.go`, so those two assertions go.

## The options weighed

| Option | Lines | What happens to the files | What it costs |
|---|---|---|---|
| Delete outright | ~403 removed, ~60 added | gone | fix 4 by-path callers, fix `--help`, resolve the `setup.go` step |
| Delegation stubs | ~403 removed, ~240 added | 12 files remain, ~20 lines each | nothing else changes; every future port adds another stub |
| Teach `bin/run` to delegate | ~403 removed, ~25 added | gone | touches the dispatcher all 65 commands share, and does not help the 4 by-path callers |
| Untangle `setup` first | larger | unchanged for now | attacks the root cause; deletion becomes trivial afterwards |

The explorer recommended delegation stubs, on the grounds that they need no change to any dependent and match the precedent set by `worktree.sh`, `docker-stop-all.sh` and `docker-compose.sh`. The orchestrator disagreed on the arithmetic: the dependents are four one-line edits, and stubs cost more changed lines (~643) than deletion (~463) while leaving twelve files behind that grow by one with every future port.

**Decision (user, 2026-09-09): delete outright.** The four callers are pointed at the binary, `print_help.sh` takes its names from `data/command_descriptions.json`, and the `setup.go` step is resolved rather than worked around.

## Risks

1. `hm setup` is the hard one: its install list must keep working, and the fix must not quietly reorder the steps.
2. Six parity assertions lose their shell side and must be rewritten, not deleted silently.
3. `hm --help` changes its source of truth; the output should stay the same, which is testable.
4. A project's own `config/hm/commands` is unaffected, and no CI workflow invokes `bin/run` for any of the twelve.

## Size

About 403 lines removed and 60 added, so roughly 463 changed against a 400-line budget. Two work units: the three commands with live shell-side callers and the `setup.go`/`--help` groundwork first, then the remaining nine with the `go_wrappers_test.sh` rewrite.
