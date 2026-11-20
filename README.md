# Hiberus Dockergento

Docker environments for Magento 2 projects, driven by one command: `hm`. Please visit our [Dockerhub repository](https://hub.docker.com/u/hiberusmagento).

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://opensource.org/licenses/gpl-3.0)
[![DockerHub](https://img.shields.io/badge/DockerHub-hiberusmagento-2496ED?logo=docker&logoColor=white)](https://hub.docker.com/u/hiberusmagento)
[![Magento](https://img.shields.io/badge/Magento-2.4.9-EE672F?logo=magento&logoColor=white)](https://experienceleague.adobe.com/docs/commerce-operations/release/notes/overview.html)

<img alt="Hiberus Dockergento Schema" src="schema.png" width="700"/>

## What hm is

`hm` sets up and runs a Magento 2 development environment in Docker without asking you to think
in containers. `hm setup` picks the PHP, MariaDB, search, Redis and Varnish versions that match
your Magento version and writes the Compose files; after that, `hm start`, `hm magento cache:clean`,
`hm composer install` or `hm mysql -i dump.sql` do what they say, inside the right container.

Since 2.0, `hm` is a single Go binary. It talks to Docker through the Docker API and uses Compose
as a library, so it no longer depends on Bash or `jq` to run, and every command speaks both
readable text and JSON. On top of the everyday commands it brings a dashboard for every
environment on the machine (`hm tui`), one environment per git branch (`hm worktree`), a shared
proxy so several projects can be up at once (`hm proxy`), database snapshots (`hm db`), a
temporary public address (`hm share`) and read-only tools for AI agents over MCP (`hm mcp`).

## Requirements

- macOS or Linux
- [Docker](https://docs.docker.com/get-docker/) with the Compose v2 plugin, and the daemon running.
  `hm` embeds Compose as a library; the plugin is used by `hm docker-compose` and by project
  commands.
- To install: `curl` (or `wget`) and a SHA-256 tool (`shasum` or `sha256sum`), which every macOS
  and Linux has. `hm` is a single self-contained binary: no git checkout, `jq` or Go is needed,
  except to build it from source as a contributor.
- Optional, for the commands that use them: [mkcert](https://github.com/FiloSottile/mkcert)
  (`hm ssl` tries to install it when it is missing: Homebrew on macOS, the release binary on Linux), [fzf](https://github.com/junegunn/fzf) (picking
  from a list), `rsync` and `ssh` (`hm transfer-media`, `hm transfer-db`).

## Docker images

- **Nginx**: `1.18`, `1.28`, `1.30`
- **PHP**: `7.2`, `7.3`, `7.4`, `8.1`, `8.2`, `8.3`, `8.4`, `8.5`
- **MariaDB**: `10.2`, `10.3`, `10.4`, `10.6`, `11.4`, `11.8`, `12.3`
- **RabbitMQ**: `3.9`, `4.2`, `4.3`
- **Redis**: `5.0`, `6.2`, `7.0`
- **Valkey**: `9`
- **ElasticSearch**: `5.6`, `6.5`, `7.17`
- **OpenSearch**: `1.2`, `2.5`, `3`
- **Mailhog**: `1.0`
- **Varnish**: `6.0`, `7.1`
- **Hitch**: `1.7`

> MariaDB `11.4` / `11.8` / `12.3` use the official `mariadb:11.4` / `mariadb:11.8` / `mariadb:12.3` images directly (no Hiberus wrapper).
> Valkey `9` uses the official `valkey/valkey:9-alpine` image directly (no Hiberus wrapper). It is a drop-in replacement for Redis and is wired through the same `redis` service slot, so no Magento configuration changes are required.

## Magento compatible versions

- **Magento 2.4**: `2.4.0`, `2.4.1`, `2.4.2`, `2.4.3`, `2.4.4`, `2.4.5`, `2.4.6`, `2.4.7`, `2.4.8`, `2.4.9`.
- **Magento 2.3**: `2.3.0`, `2.3.1`, `2.3.2`, `2.3.3`, `2.3.4`, `2.3.5`, `2.3.6`, `2.3.7`.

_(All patched versions are also compatible — run `hm compatibility` to see the full list)_

## Versions and release candidates

The installation is a git checkout, so moving between versions is a command:

```bash
hm --version              # exactly what you are running, including commits ahead of the tag
hm switch --list          # what is available
hm switch 2.0.0-rc.1      # try a release candidate
hm switch --stable        # back to stable
```

| Branch or tag | Meaning |
|---|---|
| `main` | Stable. Reached through a pull request, tagged `X.Y.Z` |
| `feature/*` | Work in progress |
| `release/X.Y.Z` | Freeze and validate |
| `X.Y.Z-rc.N` | Release candidates, shared for internal validation |

While pinned to a tag, `hm update` refuses to run so that it cannot take you off the version
you are validating. See [switch](docs/switch.md).

<br>

## Installation

hm is one binary. Install it with:

```bash
curl -fsSL https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh | bash
```

The installer downloads the latest release of 2.0 or later for your system (macOS or Linux, amd64
or arm64), checks it against the release's `checksums.txt`, and installs it:

- at `/usr/local/bin/hm` when that directory is writable, otherwise at `~/.local/bin/hm`;
- at the same path as an existing 1.x `hm` link, which it replaces, so there is only one `hm`;
- or where you say: `--dir <directory>` (or `HM_INSTALL_DIR`).

It also writes the shell completion to `~/.hm/completion/` and adds one line to your profile
(`~/.zshrc`, or `~/.bashrc` / `~/.bash_profile` on macOS for bash) that loads it.

Options go after `bash -s --`:

```bash
curl -fsSL .../install.sh | bash -s -- --version 2.0.0   # a particular release (HM_VERSION)
curl -fsSL .../install.sh | bash -s -- --pre             # allow a pre-release (HM_PRE=1)
```

The installer needs only curl (or wget). The full guide, including a manual install and
building from source, is [docs/installation.md](docs/installation.md).

### Update

Run `hm update`, or run the installer again: it upgrades to the latest release or says it is
already up to date.

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh | bash -s -- --uninstall
```

It removes the binary it installed, the completion and its profile line. hm's own state in `~/.hm`
is kept; delete it with `rm -rf ~/.hm` if you want.

### Upgrading from 1.x

Run the installer above. It replaces the `/usr/local/bin/hm` link (sudo is asked for only if that
directory is not yours) and the 1.x completion line in your profile. The old `~/hm` clone is left
where it is: the installer tells you it is no longer used, whether it has local changes, and the
command to delete it. A 1.x version cannot be installed with this installer; see
[docs/installation.md](docs/installation.md#upgrading-from-1x).

<br>

## Quick start

### An existing project

From your project's directory:

```bash
cd <your_project>
hm setup                    # answers a few questions and writes the Docker configuration
hm start                    # brings the environment up
hm mysql -i dump.sql        # imports a database and points the store at this machine
hm launch                   # opens the storefront in the browser
```

`hm setup` writes `config/docker/properties.json`, `docker-compose.yml` and the platform overlays
(`docker-compose.dev.mac.yml`, `docker-compose.dev.linux.yml`). If you want the Docker files
outside your project (not tracked by git), run `hm setup` from a wrapper directory and answer the
`Magento root dir` question with the relative path of your project:

```
 ./wrapper_folder
    |__ config/
    |__ docker-compose.yml
    |__ docker-compose.dev.linux.yml
    |__ docker-compose.dev.mac.yml
    |__ <your_project>/
        |__ app/
        |__ ...
```

See [setup](docs/setup.md) and [mysql](docs/mysql.md).

### A new project

From a new, empty directory:

```bash
hm create-project
```

See [create-project](docs/create-project.md).

### When something does not work

```bash
hm doctor             # diagnose the machine and the project
hm logs -f phpfpm     # follow a service's log
hm describe           # versions, services, URLs and state of this project
hm version            # what to paste into a bug report: hm, Docker and Compose versions
```

See [doctor](docs/doctor.md), [logs](docs/logs.md), [describe](docs/describe.md) and
[version](docs/version.md).

## Commands

`hm --help` lists every command, grouped as below; `hm <command> --help` explains one. The
global options `--json`, `--no-json`, `--yes`, `--force` and `--no-color` go before the command
name. After it they work too, except for the commands that hand their arguments to another
program (`exec`, `bash`, `magento`, `composer`, `mysql`, `logs`, `docker-compose`, ...), where
they belong to that program: `hm composer show --format=json` asks Composer for JSON. See [Output, exit codes and non-interactive use](#output-exit-codes-and-non-interactive-use)).

**Environment**

| Command | What it does |
|---|---|
| [setup](docs/setup.md) | Dockerize a Magento project |
| [start](docs/start.md) / [stop](docs/stop.md) / [restart](docs/restart.md) | Start, stop or restart the services |
| [rebuild](docs/rebuild.md) | Rebuild the services with the current configuration |
| [down](docs/down.md) | Stop and remove containers, networks and images (`-v` also volumes) |
| [logs](docs/logs.md) | Show the logs of the project or of the services named |
| [launch](docs/launch.md) | Open the storefront, admin, mail catcher, RabbitMQ or search in the browser |
| [describe](docs/describe.md) | Everything that defines the current project |
| [doctor](docs/doctor.md) | Diagnose the machine and the current project |
| [list](docs/list.md) | Every Dockergento environment on this machine |
| [tui](docs/tui.md) | The dashboard: every environment on one screen |
| [clean](docs/clean.md) | Collect what environments that no longer exist left behind |
| [proxy](docs/proxy.md) | The global proxy, so several projects can be up at once |
| [worktree](docs/worktree.md) | An environment per branch, with its own address and data |
| [share](docs/share.md) | Give this project a temporary public address |
| [tunnel](docs/tunnel.md) | A temporary way in to a service that is not routed by name |
| `post-start` | Apply what the platform needs after an environment comes up |

**Magento**

| Command | What it does |
|---|---|
| [magento](docs/magento.md) | Run `bin/magento` |
| [composer](docs/composer.md) | Run Composer inside the PHP container |
| [install](docs/install.md) | Run `setup:install` with the project's configuration |
| [n98-magerun](docs/n98-magerun.md) | Run n98-magerun |
| [npm](docs/npm.md) | Run npm inside the PHP container |
| [grunt](docs/grunt.md) | Prepare Grunt and compile a theme's styles |
| [purge](docs/purge.md) | Remove generated code, caches, static files and preprocessed views |
| [test-unit](docs/test-unit.md) / [test-integration](docs/test-integration.md) | Run the project's unit or integration tests |

**Database**

| Command | What it does |
|---|---|
| [mysql](docs/mysql.md) | Open a client, run a query (`-q`) or import a dump (`-i`) |
| [mysqldump](docs/mysqldump.md) | Export the database |
| [db](docs/db.md) | Named snapshots and data-directory templates of this project's database |
| [transfer-db](docs/transfer-db.md) | Bring a database from a remote environment |
| [masquerade](docs/masquerade.md) | Anonymise the database |
| `dbeaver` / `sequelace` / `tableplus` | Open a desktop client on this project's database |

**Files**

| Command | What it does |
|---|---|
| [copy-to-container](docs/copy-to-container.md) / [copy-from-container](docs/copy-from-container.md) | (macOS) Copy between the host and the container |
| [transfer-media](docs/transfer-media.md) | Bring media files from a remote environment |

**Tools**

| Command | What it does |
|---|---|
| [bash](docs/bash.md) / [exec](docs/exec.md) | A shell, or a command, inside a container (`-r` as root) |
| [debug-on](docs/debug-on.md) / [debug-off](docs/debug-off.md) | Enable or disable Xdebug |
| [varnish-on](docs/varnish-on.md) / [varnish-off](docs/varnish-off.md) | Turn Varnish on or off |
| [ssl](docs/ssl.md) | Install an SSL certificate for the environment |
| [set-host](docs/set-host.md) | Set a host name in `/etc/hosts` and `core_config_data` |
| [config-env](docs/config-env.md) | Apply the project's configuration for the local environment |
| [create-project](docs/create-project.md) | Create a new Magento project in the current directory |
| [compatibility](docs/compatibility.md) | Magento version compatibility |
| [verify](docs/verify.md) | Run the code checks this project has installed |
| [docker-compose](docs/docker-compose.md) | Run `docker compose` with this project's configuration |
| [docker-stop-all](docs/docker-stop-all.md) | Stop every running container |
| [cloud](docs/cloud.md) / [cloud-login](docs/cloud-login.md) | Adobe Commerce Cloud CLI |
| `web` | Bring up the web interface and print the link to open it |

**Versions**

| Command | What it does |
|---|---|
| [version](docs/version.md) | The version of hm, Docker and Compose |
| [update](docs/update.md) | Update the installation and its binary |
| [switch](docs/switch.md) | Move the installation to another version or branch |

**AI tools**

| Command | What it does |
|---|---|
| [mcp](docs/mcp.md) | Read-only tools for agents over the Model Context Protocol |
| [ai-context](docs/ai-context.md) | Write the project context an agent reads before it starts |
| [permissions](docs/permissions.md) | Print the permission configuration an agent should be given |
| `ai-init` / `ai-pull` / `ai-reset` / `ai-doctor` | Configure, download, remove and check AI skills and agents ([skills](docs/skills.md)) |

## Project commands

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

- The tool's own commands always win: a script named like one of them is never run.
- The script must be executable; its shebang decides the interpreter, and one with no `#!` line is
  run with `bash`. It gets the arguments that follow the name — without the tool's own flags
  (`--json`, `--yes`, `--no-color`...) — the terminal, and its exit code becomes `hm`'s.
- Before it runs, `hm` checks that Docker answers and that the project's Compose files are valid.
- It is looked up from the project's root (run `hm <name>` there, not from a subdirectory); from
  a git worktree it is the main checkout's script that runs.
- Its environment carries the project's properties (`COMPOSE_PROJECT_NAME`, `DOMAIN`,
  `MAGENTO_DIR`, ...), `HM_ROOT`, `DOCKER_COMPOSE`, `MACHINE`, `HM_OUTPUT_FORMAT`
  (`text` or `json`), `HM_NON_INTERACTIVE`, the exit codes as `HM_EXIT_*` (`HM_EXIT_USAGE`,
  `HM_EXIT_SERVICE`, ...) and the colour variables.
- **Incompatible change in 2.0, on purpose:** scripts no longer receive `HELPERS_DIR`,
  `TASKS_DIR` or `COMPONENTS_DIR`, because the shell tree they pointed at is gone. A script must
  be self-contained, like the one above: one that sourced `"$HELPERS_DIR"/...` or called
  `"$TASKS_DIR"/...` has to carry what it used, or call `hm` commands instead.
- `hm --help` lists the project's commands; an optional `config/hm/commands/command_descriptions.json`
  describes them in the same format as the tool's own `data/command_descriptions.json`, and
  `hm <name> --help` prints that description.

## Output, exit codes and non-interactive use

Every command can be consumed by a person or by a machine (a script, a CI job, an AI
agent). The rules are the same everywhere:

**Output format.** Informational commands print readable text when their output goes to a
terminal, and JSON when it is piped or redirected. Force either one with `--json` or
`--no-json`.

```bash
hm describe              # readable text in a terminal
hm describe | jq .       # JSON, because stdout is not a terminal
hm describe --json       # JSON on demand
```

Commands whose output *is* data are never wrapped: `mysqldump`, `mysql`,
`copy-from-container` and the passthrough commands (`exec`, `bash`, `magento`,
`composer`, `npm`, `n98-magerun`, `grunt`, `test-*`, `cloud`, `docker-compose`) always
emit exactly what they produce.

**Errors** go to stderr in both formats, so `hm <command> > file` never captures a
failure as if it were output. In JSON mode an error looks like:

```json
{ "schema_version": 1, "command": "start", "ok": false,
  "error": { "code": 3, "type": "docker_unavailable",
             "message": "Docker is not running",
             "hint": "Start Docker and try again" } }
```

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0`  | Success |
| `1`  | Generic error |
| `2`  | Invalid arguments or unknown command |
| `3`  | Docker daemon unavailable |
| `4`  | Not a configured Dockergento project |
| `5`  | A required service is not running |
| `6`  | Refused on purpose, for example from a git worktree ([details](docs/worktree.md)) |

**Colour.** Colour marks state (error, warning, success) and highlights what you can act
on; everything else stays in your terminal's own colour. The decision follows the usual
ecosystem standards, in this order of precedence:

| | Effect |
|---|---|
| `--no-color` | Never colour, whatever else is set |
| `NO_COLOR` (set and non-empty) | Never colour — the [no-color.org](https://no-color.org/) standard |
| `TERM=dumb` or empty | Never colour |
| `FORCE_COLOR` / `CLICOLOR_FORCE` | Colour even when the output is piped, for CI logs that render ANSI |
| stdout is not a terminal | No colour |

The text is identical either way: removing the colour never removes information.

**Non-interactive mode.** With `--yes` or `HM_NON_INTERACTIVE=1` no command waits for
input: questions with a default answer use it, and a question that cannot be guessed
fails with exit code `2` and a message naming the option to pass instead. How the tool asks
when it does: [questions](docs/questions.md).

```bash
HM_NON_INTERACTIVE=1 hm setup -p myproject -d myproject.local -r .
```

## The dashboard

`hm tui` opens every environment on this machine on one screen: what each one is doing,
which branch it is on, the warnings from `doctor`, and the actions one key away — start,
stop, restart, follow logs, open in the browser.

It does not need to run inside a project, and it has no privileges of its own: the data is
the JSON from `list`, `describe` and `doctor`, and every action runs the real command inside
that environment's directory, so the same protections apply. See [tui](docs/tui.md).

## Several projects and branches at once

**The proxy.** Without it every project publishes ports 80, 443, 3306... on the machine, so only
one can be up at a time. With `hm proxy up` and `"USE_PROXY": "true"` in a project's
`config/docker/properties.json`, projects stop publishing ports and are told apart by their
domain. See [proxy](docs/proxy.md) and [dns](docs/dns.md).

**Worktrees.** `hm worktree add <branch>` creates a git worktree for the branch and gives it an
environment of its own — its own address, containers and database — next to the main one:

```bash
hm worktree add feature/checkout
hm worktree list
hm worktree remove feature-checkout
```

From a git worktree that has no environment of its own, the commands that would act on the main
environment are refused (exit `6`) instead of repointing or destroying it; `--force` runs one of
them anyway, for that invocation only. See [worktree](docs/worktree.md).

**Database copies.** `hm db snapshot` before something risky and `hm db restore` after;
`hm db freeze` and `hm db clone` stand a second environment's database up in seconds. See
[db](docs/db.md).

**Sharing.** `hm share` gives the project a temporary public address until you press Ctrl-C;
`hm tunnel <service>` opens a temporary way in to a service such as `db` or `rabbitmq`. See
[share](docs/share.md) and [tunnel](docs/tunnel.md).

## AI agents

- `hm mcp` is a Model Context Protocol server with read-only tools — describe the project, list
  environments, run the diagnosis, read a service's log, run one read-only SQL statement.
  `hm mcp --config` prints the entry to add to your MCP client; `hm mcp --write` also offers a few
  bounded write tools. See [mcp](docs/mcp.md).
- `hm ai-context` writes the project context an agent reads before it starts, and
  `hm permissions` prints the permission configuration an agent should be given. See
  [ai-context](docs/ai-context.md) and [permissions](docs/permissions.md).
- `hm ai-init`, `hm ai-pull`, `hm ai-reset` and `hm ai-doctor` manage the AI skills and agents of
  a project; the skills that teach an agent this tool live in `skills/`. See
  [skills](docs/skills.md).

## Shell completion

`hm completion bash` and `hm completion zsh` print the completion script, built from the
commands the binary itself declares. The installer writes it to `~/.hm/completion/` for you; to
load it by hand, add `source <(hm completion bash)` to `~/.bashrc`, or
`source <(hm completion zsh)` to `~/.zshrc` after `compinit`. See [completion](docs/completion.md).

## Working on hm itself

The binary is self-contained: `data/`, `docker-compose/` and `skills/` are compiled into it with
`go:embed`, so it reads nothing from beside the executable and an edit to one of those files
needs a rebuild (`go build -o bin/hm ./cmd/hm`) to be seen.

To try an edit without rebuilding, point `HM_ASSETS_DIR` at a checkout:

```bash
HM_ASSETS_DIR=~/src/hiberus-dockergento hm compatibility
```

When it is set, every one of those files is read from that directory (laid out like the root of
the repository) instead of from the binary, with no fallback to the embedded copy: a file missing
there is an error. Project scripts under `config/hm/commands` get `DATA_DIR` set to
`$HM_ASSETS_DIR/data` then, and empty otherwise.

The store and admin settings `hm install` passes to `setup:install` live in `~/.hm/config.json`,
outside the installation, shared by every project.

## Where to look next

- [docs/](docs/): one page per command, plus [questions](docs/questions.md),
  [project-name](docs/project-name.md), [environment-labels](docs/environment-labels.md),
  [mail](docs/mail.md) and [progress](docs/progress.md).
- [ARCHITECTURE.md](ARCHITECTURE.md): how the tool is built — the Go binary, its router, the
  hexagonal layers and how Docker and Compose are driven.
- [CLAUDE.md](CLAUDE.md): building, testing and changing the tool.
- [MIGRATION.md](MIGRATION.md) and [ROADMAP.md](ROADMAP.md): the history of the move from Bash to
  Go, and what comes next (in Spanish).

<br>

## Related Projects

### Hiberus Magento AI Tools

[hiberus-magento/ai-tools](https://github.com/hiberus-magento/ai-tools) — AI-powered skills and agents for Magento 2 that extend AI coding assistants (Claude, Copilot, Cursor, Gemini, etc.) with expert Magento knowledge. Dockergento integrates with ai-tools to provide an agile, AI-assisted development workflow.

<br>

## Thanks to

* This project is based on [Dockergento](https://github.com/ModestCoders/magento2-dockergento). Special thanks to **ModestCoders** for their work.
* Several improvements have been inspired on [Docker-magento](https://github.com/markshust/docker-magento).

<br>

## Copyright

[(c) Hiberus Tecnología](https://hiberus.com).

<br/>

## License

[GNU General Public License, version 3 (GPLv3)](https://opensource.org/licenses/gpl-3.0).
