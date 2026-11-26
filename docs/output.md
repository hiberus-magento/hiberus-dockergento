# Output, exit codes and non-interactive use

Every command can be consumed by a person or by a machine (a script, a CI job, an AI
agent). The rules are the same everywhere.

## Output format

Informational commands print readable text when their output goes to a terminal, and JSON when it
is piped or redirected. Force either one with `--json` or `--no-json`.

```bash
hm describe              # readable text in a terminal
hm describe | jq .       # JSON, because stdout is not a terminal
hm describe --json       # JSON on demand
```

Commands that change something (`start`, `stop`, `restart`, `rebuild`, `purge`, `down`, ...)
answer with a success document under `--json`, and `hm --json` lists every command with its
options and safety level, so an agent can discover what exists in one call. `--dry-run` on
`down`, `clean`, `db restore` and `purge` says what would be removed and changes nothing;
`--progress=jsonl` writes the steps of long commands to stderr as JSON lines. See
[JSON output](json.md) and [progress](progress.md).

Commands whose output *is* data are never wrapped: `mysqldump`, `mysql`,
`copy-from-container` and the passthrough commands (`exec`, `bash`, `magento`,
`composer`, `npm`, `n98-magerun`, `grunt`, `test-*`, `cloud`, `docker-compose`) always
emit exactly what they produce.

## Global options

The global options `--json`, `--no-json`, `--yes`, `--force`, `--no-color` and `--progress=jsonl`
go before the command name. After it they work too, except for the commands that hand their
arguments to another program (`exec`, `bash`, `magento`, `composer`, `mysql`, `logs`,
`docker-compose`, ...), where they belong to that program: `hm composer show --format=json` asks
Composer for JSON.

## Errors

Errors go to stderr in both formats, so `hm <command> > file` never captures a failure as if it
were output. In JSON mode an error looks like:

```json
{ "schema_version": 1, "command": "start", "ok": false,
  "error": { "code": 3, "type": "docker_unavailable",
             "message": "Docker is not running",
             "hint": "Start Docker and try again" } }
```

## Exit codes

| Code | Meaning |
|------|---------|
| `0`  | Success |
| `1`  | Generic error |
| `2`  | Invalid arguments or unknown command |
| `3`  | Docker daemon unavailable |
| `4`  | Not a configured Dockergento project |
| `5`  | A required service is not running |
| `6`  | Refused on purpose, for example from a git worktree ([details](worktree.md)) |

## Colour

Colour marks state (error, warning, success) and highlights what you can act on; everything else
stays in your terminal's own colour. The decision follows the usual ecosystem standards, in this
order of precedence:

| | Effect |
|---|---|
| `--no-color` | Never colour, whatever else is set |
| `NO_COLOR` (set and non-empty) | Never colour — the [no-color.org](https://no-color.org/) standard |
| `TERM=dumb` or empty | Never colour |
| `FORCE_COLOR` / `CLICOLOR_FORCE` | Colour even when the output is piped, for CI logs that render ANSI |
| stdout is not a terminal | No colour |

The text is identical either way: removing the colour never removes information.

## Non-interactive mode

With `--yes` or `HM_NON_INTERACTIVE=1` no command waits for input: questions with a default answer
use it, and a question that cannot be guessed fails with exit code `2` and a message naming the
option to pass instead. How the tool asks when it does: [questions](questions.md).

`hm setup` is the usual case: the project name, the domain and the root directory have defaults,
but *import a dump or install Magento* does not, so it has to be given on the command line
([details](install.md#bootstrapping-without-being-asked-anything)):

```bash
HM_NON_INTERACTIVE=1 hm setup -p myproject -d myproject.local -r . --db-dump=./dump.sql
```
