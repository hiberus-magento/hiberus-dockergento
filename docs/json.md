# JSON output

Every command that answers with data can answer in JSON, so that a script or an AI agent reads a
document instead of parsing sentences. Ask for it with `--json`; `--no-json` forces text. Without
either, a terminal gets text and anything else (a pipe, a file) gets JSON.

## The envelope

Every answer has the same shape, whichever command it came from:

```json
{
  "schema_version": 1,
  "command": "start",
  "ok": true,
  "data": { "...": "what the command did or found" }
}
```

A failure has `"ok": false` and an `error` instead of `data`, and it is written on **stderr**, never
on stdout:

```json
{
  "schema_version": 1,
  "command": "start",
  "ok": false,
  "error": {
    "code": 3,
    "type": "docker_failed",
    "message": "what went wrong",
    "hint": "the command that fixes it, when there is one"
  }
}
```

`code` is also the process's exit code: 0 success, 1 error, 2 usage error, 3 Docker failed, 4 not
a project, 5 unknown service, 6 refused on purpose (for example from a git worktree). Branch on the
exit code or on `error.type`, not on the message.

## What the tool can do: the catalogue

`hm --json` (or `hm help --json`) prints every command of the tool in one envelope, with
`"command": "help"`:

```json
{
  "schema_version": 1,
  "command": "help",
  "ok": true,
  "data": {
    "commands": [
      {
        "name": "start",
        "description": "Start docker services",
        "usage": "start [-s|--switch] <service-name>",
        "example": "start phpfpm",
        "options": [{ "short": "s", "long": "switch", "description": "..." }],
        "arguments": [{ "name": "service-name", "description": "...", "multiple": false }],
        "safety": "caution",
        "passthrough": false,
        "group": "environment",
        "mac_only": false
      }
    ],
    "groups": [{ "id": "environment", "title": "Environment" }],
    "global_options": [{ "short": "", "long": "json", "description": "..." }]
  }
}
```

`hm <command> --help --json` (or `hm help <command> --json`) prints that command's entry alone as
`data`. A command the tool does not declare is a failure: exit 2, `error.type` `command_not_found`.

- Every field comes from the tool's own `data/command_descriptions.json`, the same declaration the
  text help is printed from. Every field is always present; `example` is `""` and `options` or
  `arguments` are `[]` when there are none.
- `usage` is written without the binary's name: prefix it with `hm`.
- `safety` is `safe`, `caution` or `dangerous`, the classification `hm permissions` uses.
- `passthrough` is `true` for the commands whose output, and whose options after the command name,
  belong to the program they run (`magento`, `composer`, `exec`...): they never print an envelope.
- `mac_only` commands are listed on every machine; the text help shows them only on macOS.
- The project's own commands (`config/hm/commands`) are not in the catalogue.

Only `--json` asks for the catalogue. Without it, `hm` and `hm --help` print the text help even into
a pipe, and `hm help` without `--json` is not a command of the tool.

## Commands that change something

These commands print nothing of their own on stdout in text mode. With `--json`, when they succeed,
stdout holds exactly one success envelope and nothing else; progress and announcements go to
stderr. `elapsed_ms` is the wall time of the command, in milliseconds.

| Command | `data` |
| ------- | ------ |
| [`start`](start.md) | `project`, `services`, `whole_environment`, `stopped_others`, `url`, `elapsed_ms` |
| [`stop`](stop.md) | `project`, `services`, `whole_environment`, `snapshot`, `timeout`, `elapsed_ms` |
| [`restart`](restart.md) | `project`, `services`, `whole_environment`, `timeout`, `elapsed_ms` |
| [`rebuild`](rebuild.md) | `project`, `services`, `whole_environment`, `elapsed_ms` |
| `post-start` | `project`, `elapsed_ms` |
| [`purge`](purge.md) | `project`, `removed`, `elapsed_ms` |
| [`down`](down.md) | `destroyed`, `volumes`, `snapshot` |
| [`debug-on`](debug-on.md) | `xdebug`, `version`, `host`, `settings` |
| [`debug-off`](debug-off.md) | `xdebug` |

- `services` is the list of services named on the command line; an empty list, with
  `whole_environment: true`, means the whole environment.
- `timeout` is the `-t` given, or `null` for Compose's own default.
- `url` is the storefront's address (`https://<domain>/`), or `null` when the project has no domain.
  For every URL, including the admin's, use [`hm describe --json`](describe.md).

`varnish-on`, `set-host`, `copy-from-container` and `mysqldump` print an envelope of their own as
well.

Commands whose stdout belongs to a program inside a container — `magento`, `composer`, `exec`,
`bash`, `logs`, `mysql`, `npm`, `grunt` and the like — print that program's output, never an
envelope on success.
