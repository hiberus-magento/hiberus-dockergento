# Project commands

A project can add commands of its own: an executable script at `config/hm/commands/<name>.sh`,
under the project's root, is run by `hm <name> [args]`. For example, `hm reindex`:

```bash
mkdir -p config/hm/commands
cat > config/hm/commands/reindex.sh <<'SH'
#!/usr/bin/env bash
# hm reindex [indexer...]: reindex, then flush the cache.
# Self-contained: it uses only what hm puts in its environment, and hm itself.
set -euo pipefail

echo "Reindexing ${COMPOSE_PROJECT_NAME} (${DOMAIN})"

# DOCKER_COMPOSE is the Compose command line for this project, ready to use
$DOCKER_COMPOSE exec -T phpfpm bin/magento indexer:reindex "$@"

# hm's own commands work too
hm magento cache:flush
SH
chmod +x config/hm/commands/reindex.sh
hm reindex
```

## How they run

- The tool's own commands always win: a script named like one of them is never run.
- The script must be executable; its shebang decides the interpreter, and one with no `#!` line is
  run with `bash`. It gets the arguments that follow the name — without the tool's own flags
  (`--json`, `--yes`, `--no-color`...) — the terminal, and its exit code becomes `hm`'s.
- Before it runs, `hm` checks that Docker answers and that the project's Compose files are valid.
- It is looked up from the project's root (run `hm <name>` there, not from a subdirectory); from
  a git worktree it is the main checkout's script that runs.

## What the script gets

- Its environment carries the project's properties (`COMPOSE_PROJECT_NAME`, `DOMAIN`,
  `MAGENTO_DIR`, ...), `HM_ROOT`, `DOCKER_COMPOSE`, `MACHINE`, `HM_OUTPUT_FORMAT`
  (`text` or `json`), `HM_NON_INTERACTIVE`, the exit codes as `HM_EXIT_*` (`HM_EXIT_USAGE`,
  `HM_EXIT_SERVICE`, ...) and the colour variables.
- `DATA_DIR` is the tool's `data/` directory on disk, as in 1.x, so `"$DATA_DIR"/requirements.json`
  keeps working: `$HM_ASSETS_DIR/data` when that is set, otherwise the files compiled into the
  binary extracted once to `~/.hm/cache/data-<digest>` (a new build gets a new directory). Treat
  it as read-only. It is empty only when that directory cannot be written.
- **Incompatible change in 2.0, on purpose:** scripts no longer receive `HELPERS_DIR`,
  `TASKS_DIR` or `COMPONENTS_DIR`, because the shell tree they pointed at is gone. A script must
  be self-contained, like the one above: one that sourced `"$HELPERS_DIR"/...` or called
  `"$TASKS_DIR"/...` has to carry what it used, or call `hm` commands instead.

## Help for them

`hm --help` lists the project's commands; an optional `config/hm/commands/command_descriptions.json`
describes them in the same format as the tool's own `data/command_descriptions.json`, and
`hm <name> --help` prints that description.

The project's commands are not in the `hm --json` catalogue ([JSON output](json.md)).
