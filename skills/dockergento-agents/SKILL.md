---
name: dockergento-agents
description: Work autonomously on a Hiberus Dockergento (hm) project - branch environments with hm worktree, the read-only MCP server, generated permissions, and the JSON and exit-code contract every command speaks. Use when running unattended, when several tasks need their own environment, or when setting an agent up on this repository.
---

# Dockergento: working as an agent

## The contract every command speaks

Commands that report something answer in JSON when stdout is not a terminal — which is always
your case — with the same envelope:

```json
{"ok": true, "schema_version": 1, "command": "describe", "data": { }}
```

Failures go to stderr with `ok: false`, a `type`, a `message` and a `hint`, and the exit code says
what kind of failure it was:

| Code | Means | What to do |
|---|---|---|
| 0 | Fine | — |
| 2 | Called wrong | Read the usage in the error, fix the call |
| 3 | Docker is not running | Say so. Do not start Docker without being asked |
| 4 | Not a Dockergento project | You are in the wrong directory |
| 5 | A service is not running | `hm start`, or start the one service |
| 6 | Refused on purpose | **Read the message.** It is protecting data or another environment |

Nothing waits for input when `HM_NON_INTERACTIVE=1` is set or `--yes` is passed. Set it once for
an unattended session rather than answering prompts you cannot see.

## Do not guess what the project is

```bash
hm describe --json      # versions, services, URLs, admin path, deploy mode, state
hm list --json          # every environment on the machine
hm doctor --json        # what is wrong
```

These answer with the environment stopped. Reading `docker-compose.yml`, `app/etc/env.php` or
`composer.lock` to work the same things out gives you the configured value, not the resolved one.

## An environment of your own, per task

```bash
hm worktree add feature/checkout            # branch, containers, address and data
hm worktree add feature/checkout --profile=lite
hm worktree list --json
hm worktree remove feature-checkout
```

A branch environment is a git worktree with its own compose project, its own subdomain and its own
database, cloned from a template in seconds. Profiles decide what runs:

| Profile | Services | For |
|---|---|---|
| `lite` | phpfpm | Running code. No HTTP |
| `agent` | phpfpm, nginx, db, search, redis | A Magento that answers. The default |
| `full` | everything | What the main environment runs |

Once it exists, `hm` commands run in that directory act on that environment and nothing else. Run
`hm worktree remove` when the task is finished: it destroys the containers and the volumes,
removes the worktree and refuses if there is uncommitted work.

**From a worktree without an environment of its own**, `hm start`, `hm rebuild` and `hm down` are
refused with exit code 6. That refusal is not an obstacle to work around with `--force`: those
commands would repoint the main environment's mounts at your directory and destroy its database.

## The MCP server

```bash
hm mcp --config     # the entry to add to an MCP client's configuration
```

It offers read-only tools over the project — `describe_project`, `list_environments`,
`check_environment`, `service_logs` and `database_query` — and nothing that changes anything.
`database_query` accepts one `SELECT`, `SHOW`, `DESCRIBE` or `EXPLAIN` and refuses everything
else, comments included.

## Permissions

```bash
hm permissions          # what an agent needs to work
hm permissions --strict # what an agent needs to only look
```

Every command declares whether it is `safe`, `caution` or `dangerous`, and this turns that into
the permission configuration to give an agent. It prints; it writes to no file.

Worth knowing what the classification means for you: `hm exec`, `hm bash`, `hm mysql` and
`hm docker-compose` are dangerous **because they run whatever they are given** — being allowed to
run them is being allowed to run anything. Prefer the typed command: `hm magento`, `hm composer`,
`hm logs`, `hm db`.

## Before handing work back

```bash
hm verify --changed     # the checks this project has, on what changed
```

## The three things not to do

1. **Do not destroy data you were not asked to destroy.** `hm down -v`, `hm db restore`,
   `hm db clear`, `hm clean --force` and `hm purge` all remove something. Ask.
2. **Do not act on another environment.** Commands act on the project of the current directory;
   `hm docker-stop-all` and `hm clean` do not, which is why they ask.
3. **Do not work around an exit code 6.** It is the one code the tool uses to say "this is
   refused on purpose", and every one of them exists because it destroyed somebody's work once.
