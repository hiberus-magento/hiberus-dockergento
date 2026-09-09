# Proposal: The third step of the porting rule, executed for the first time

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as the
predecessors `remaining-wrappers-go-tests`, `down-and-set-host-go-tests` and `docker-tools-in-go`.

## Intent

`MIGRATION.md:62-66` states the rule in three steps: **a command ported → its test in Go → its shell
implementation and its bash test are deleted**, in that order, and nothing in shell is deleted before
its behaviour is covered in Go. Four changes in a row have done the first two steps. The third has
never been executed. Twelve commands are routed in Go, tested in Go, and still carry a full shell
twin: 395 lines that nobody runs and that every reader has to decide whether to trust.

This change is not tidying. While both halves exist, the shell one is still reachable — `hm setup`
runs five of these commands through `bin/run` today (`setup.go:516-533`) — so "ported" is not yet
true of any of them.

## Scope

### In Scope

- **Delete twelve implementations** under `console/commands/`: `set-host` (109), `version` (85),
  `copy-to-container` (64), `copy-from-container` (46), `varnish-off` (21), `varnish-on` (18),
  `mysqldump` (13), `purge` (11), `n98-magerun` (10), `test-unit` (10), `npm` (4),
  `test-integration` (4).
- **Four by-path callers point at the binary**: `console/tasks/magento_installation.sh:32`,
  `console/commands/composer.sh:39` and `:51`, `console/commands/debug-on.sh:22`. One shared helper
  that locates `$COMMAND_BIN_DIR/bin/hm` and refuses like `worktree.sh:18-26` does
  (`binary_missing`), rather than the same eight lines copied three times.
- **`hm --help` stops depending on files.** `print_help.sh:42` globs `console/commands/*.sh`; the
  native names become the declared entries of `data/command_descriptions.json` (all 65 already)
  **union** the files on disk, sorted as `<name>.sh` so the glob's documented order survives
  (`cloud-login` before `cloud`, `:38-40`). Still one `jq` (`:52-53`). The custom-commands pass keeps
  globbing: a project's own commands are not in that file.
- **`hm setup` keeps one door, and the door becomes the binary.** See Approach.
- **`bin/run <retired>` says where the command went** instead of `Command not found`
  (`bin/run:72-82`).
- **Tests that assert against the shell side are rewritten, not dropped**: six parity cases in
  `tests/integration/go_wrappers_test.sh` (`:95`, `:98`, `:137`, `:179`, `:190`, `:201`);
  `tests/integration/concurrent_safety_test.sh:105-109`, whose guarantee now lives in
  `dockergento/core/hosts.go:19` and is pinned by `TestOnlyWhatThisToolAddedIsRemoved`
  (`hosts_test.go:13`) and `TestTheEntryIsItsOwnLine` (`hosts_test.go:66`); and
  `tests/unit/command_safety_test.sh:52-57`,
  which today requires a `.sh` for every declared entry and would report twelve orphans.
- `MIGRATION.md`: the deletion discipline recorded as executed. The table stays as it is — all twelve
  already read `go`.

### Non-goals (Out of Scope)

- **No other command's `.sh` is touched.** `docker-compose.sh`, `docker-stop-all.sh` and
  `worktree.sh` stay as delegations; the other 33 shell commands stay whole.
- **`bin/run` keeps serving the 33.** It is not taught to delegate to the binary, and nothing about
  its dispatch changes beyond one message on the not-found path.
- **No behaviour change**: no command's arguments, prompts, `--json` document or exit codes move.
- Porting anything new, or untangling `setup` beyond the door it already has.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `go-entrypoint`: "A ported command's shell entry point still answers by delegating"
  (spec.md:371-386) currently reads as universal; it must be scoped to the entry points that are
  kept, and joined by a requirement that a command proved in Go has no shell twin and that the shell
  entry point names where it went.
- `command-discovery`: "Ningún comando desaparece" (spec.md:27-29) compares the help against
  *existing command files*; a Go-only command has none. The comparison becomes against declared
  commands. "Comando sin grupo declarado" (:19-21) and "Comandos personalizados" (:23-25) must keep
  holding — which is why the names are a union and not a replacement.
- `bootstrap-options`: nothing there pins `setup`'s install list; the nearest requirement is "The
  environment is written before anything is installed" (spec.md:75). It gains one: the install steps
  run in one order through one door, and that door answers for ported commands too.

`project-installation` is **not** touched: its three requirements are the admin password, the second
factor and the closing summary, none of which this reaches.

## Approach

| Area | What happens |
|---|---|
| `console/commands/` | Twelve files removed. Nothing replaces them. |
| by-path callers | New `console/helpers/binary.sh`, sourced by the three files; each call site becomes one line. |
| `print_help.sh` | `names` built from the JSON's keys ∪ the glob, sorted by `<name>.sh`. Output byte-identical. |
| `bin/run:72-82` | On a miss, if the name is declared in `command_descriptions.json`, fail `command_moved_to_binary` with "`<name>` is part of the `hm` binary now" and the hint `hm <name>`; still exit 2, still before any Docker work. |
| `dockergento/app/setup.go` | `Setup.Legacy` is a `ports.Legacy` (`ports.go:310-314`). Keep the list at `:516-530` exactly as it is, and hand `Setup` a runner that invokes **this binary** instead of `bin/run` — the same shape as `legacy.Runner` (`runner.go:41-63`), wired at `dockergento.go:601`. The comment at `:507-509` stays true word for word: one door, one order. What is still shell reaches `bin/run` through the binary's own router, one hop later. |
| `go_wrappers_test.sh` | The six comparisons become binary-only assertions of the same fact: the argv `npm`/`n98-magerun` reach the container with, the `phpunit --config` line, the row count in the dump, the `version --json` document's shape. Never `assert_equals "$X" "$X"`. |

Strict TDD: RED first for the setup door (a fake `Legacy` seeing `set-host` and asserting which
runner it came from), for the help names, and for the `bin/run` message.

## Delivery

Two work units, each self-consistent, each carrying its own tests.

| # | Work unit | Estimate |
|---|---|---|
| 1 | The three with live shell-side callers (`set-host`, `copy-to-container`, `copy-from-container`, 219 lines) plus all the groundwork: the binary helper, `print_help.sh`, `bin/run`'s message, `command_safety_test.sh`, `setup.go`'s door, `concurrent_safety_test.sh` | ~315 |
| 2 | The remaining nine (176 lines) plus the `go_wrappers_test.sh` rewrite and `MIGRATION.md` | ~215 |

~530 changed lines in total against a 400 budget, measured with
`git diff --shortstat <base>..HEAD -- . ':(exclude)openspec'`. **Each unit is inside the budget on
its own**; cumulatively it is not, so `sdd-tasks` must forecast it and the delivery decision is taken
before apply, not during.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `console/commands/` ×12 | Removed | The twelve shell twins |
| `console/helpers/binary.sh` | New | Locate the binary, refuse like `worktree.sh` |
| `console/tasks/magento_installation.sh:32`, `console/commands/composer.sh:39,:51`, `console/commands/debug-on.sh:22` | Modified | Through the binary |
| `console/helpers/print_help.sh:41-46` | Modified | Names from the data file, union with the glob |
| `bin/run:72-82` | Modified | One message on the not-found path |
| `dockergento/app/setup.go`, `dockergento/dockergento.go:594-610` | Modified | The door becomes the binary |
| `dockergento/adapters/…` | New | A runner that invokes this executable |
| `tests/integration/go_wrappers_test.sh` | Modified | Six parity cases rewritten |
| `tests/integration/concurrent_safety_test.sh:105-109` | Modified | Removed with a note pointing at `core/hosts_test.go` |
| `tests/unit/command_safety_test.sh:52-57` | Modified | A file **or** a route in `internal/cli/run.go` |
| `MIGRATION.md` | Modified | The third step recorded as executed |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `hm setup` breaks or silently reorders its steps | High if careless | The list at `setup.go:516-530` is not edited at all; only the runner behind it changes, with a Go test on the order |
| A parity test survives as a vacuous assertion | Med | Each of the six is rewritten to assert the same observable fact against the binary; a reviewer checklist item, not a rename |
| `hm --help` changes its output | Med | Union of sources, sorted by `<name>.sh`; proven by a captured before/after diff and by `help_test.sh` unchanged |
| `command_safety_test.sh` reports twelve orphans | Certain if forgotten | Named in unit 1, in the same commit as the first deletions |
| Someone scripted `console/commands/<name>.sh` directly | Low | Documented in the changelog; `bin/run <name>` and `hm <name>` both keep working |
| The self-runner recurses or loses the terminal | Low | `setup` is not one of its own steps; stdio inherited exactly as `legacy.Runner` does (`runner.go:48-51`) |

## Impact on existing projects and migration

**No migration step for users.** Every one of the twelve keeps its name, arguments, prompts, output
and exit code, reached through `hm <command>`; `bin/run <command>` now refuses with a message that
names the binary instead of running a duplicate. A project's own `config/hm/commands` is untouched —
it is a different directory, `bin/run:47` reads it separately and the custom pass of the help still
globs it. The only observable difference for a person is that message.

## Rollback Plan

Two commits on `release/2.0.0`; `git revert` in reverse order restores the twelve files, the four
call sites, the help glob and the `bin/run` message. Reverting unit 2 alone leaves unit 1 consistent,
because the groundwork does not depend on the nine.

## Success Criteria

- [ ] `hm --help` is byte-identical to the pre-change binary's, proven by capturing it before the
      first commit and diffing after the last: `bin/run --help > /tmp/help.before` → `diff`.
- [ ] All 65 declared commands appear in the help, including the twelve with no file.
- [ ] `hm setup` runs its steps in the order of `setup.go:516-530`, with `set-host` answered by Go —
      pinned by a Go test over the step list, not by reading the code.
- [ ] `bin/run version` exits 2 saying the command is part of the binary now, with the hint
      `hm version`, and no Docker validation runs first.
- [ ] `rg -l "copy-to-container.sh|copy-from-container.sh|set-host.sh" console/ bin/` finds nothing.
- [ ] `tests/run.sh` green, including `command_safety_test.sh`, `help_test.sh`,
      `migration_status_test.sh`, `concurrent_safety_test.sh` and `go_wrappers_test.sh`; every one of
      the six rewritten cases still fails when the binary's behaviour is broken on purpose.
- [ ] `go test ./... -short`, `gofmt -l ./cmd ./internal` empty, `go vet ./...` clean.
- [ ] Manual verification, safe by construction: `hm --help` compared between the old and new builds
      (reads nothing), and — only if the developer already has a project up — `hm composer install`
      on macOS, where the vendor mirror is what exercises `copy-to-container`, and `hm debug-on`,
      which is what exercises `copy-from-container`. Neither is run against a project the developer
      cannot rebuild.
