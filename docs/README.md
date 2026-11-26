# hm documentation

One page per command, plus the guides below. `hm --help` lists every command and
`hm <command> --help` explains one; these pages say why, and what to expect.

## Guides

| Page | Read it when |
|---|---|
| [Installation](installation.md) | Installing, updating, uninstalling, or moving from 1.x |
| [Getting started](getting-started.md) | Your first project, troubleshooting, several projects at once |
| [Supported versions](versions.md) | Which PHP, MariaDB, search and Redis each Magento version gets |
| [Output, exit codes and non-interactive use](output.md) | Scripting `hm`, running it in CI, or from an agent |
| [JSON output](json.md) and [progress](progress.md) | Reading the JSON envelope and the step stream |
| [Project commands](project-commands.md) | Adding your own `hm <name>` to a project |
| [Questions](questions.md) | How the tool asks, and how to answer ahead of time |
| [Project name](project-name.md), [environment labels](environment-labels.md), [mail](mail.md), [dns](dns.md) | Naming, labelling and routing environments |
| [Performance](performance.md) | Changing `hm` itself without making it slower: the budgets and what they measure |

Elsewhere in the repository: [ARCHITECTURE.md](../ARCHITECTURE.md) (how the tool is built),
[CONTRIBUTING.md](../CONTRIBUTING.md) (working on `hm` itself), [MIGRATION.md](../MIGRATION.md) and
[ROADMAP.md](../ROADMAP.md) (the history of the move from Bash to Go and what comes next, in
Spanish).

## Commands

Grouped as `hm --help` groups them. The global options are in
[Output, exit codes and non-interactive use](output.md#global-options).

**Environment**

| Command | What it does |
|---|---|
| [setup](setup.md) | Dockerize a Magento project |
| [start](start.md) / [stop](stop.md) / [restart](restart.md) | Start, stop or restart the services |
| [rebuild](rebuild.md) | Rebuild the services with the current configuration |
| [down](down.md) | Stop and remove containers, networks and images (`-v` also volumes) |
| [logs](logs.md) | Show the logs of the project or of the services named |
| [launch](launch.md) | Open the storefront, admin, mail catcher, RabbitMQ or search in the browser |
| [describe](describe.md) | Everything that defines the current project |
| [status](status.md) | How the environment is doing: containers, URLs, xdebug, mode, data and last errors |
| [doctor](doctor.md) | Diagnose the machine and the current project |
| [list](list.md) | Every Dockergento environment on this machine |
| [tui](tui.md) | The dashboard: every environment on one screen |
| [clean](clean.md) | Collect what environments that no longer exist left behind |
| [proxy](proxy.md) | The global proxy, so several projects can be up at once |
| [worktree](worktree.md) | An environment per branch, with its own address and data |
| [share](share.md) | Give this project a temporary public address |
| [tunnel](tunnel.md) | A temporary way in to a service that is not routed by name |
| `post-start` | Apply what the platform needs after an environment comes up |

**Magento**

| Command | What it does |
|---|---|
| [magento](magento.md) | Run `bin/magento` |
| [composer](composer.md) | Run Composer inside the PHP container |
| [install](install.md) | Run `setup:install` with the project's configuration |
| [n98-magerun](n98-magerun.md) | Run n98-magerun |
| [npm](npm.md) | Run npm inside the PHP container |
| [grunt](grunt.md) | Prepare Grunt and compile a theme's styles |
| [purge](purge.md) | Remove generated code, caches, static files and preprocessed views |
| [test-unit](test-unit.md) / [test-integration](test-integration.md) | Run the project's unit or integration tests |

**Database**

| Command | What it does |
|---|---|
| [mysql](mysql.md) | Open a client, run a query (`-q`) or import a dump (`-i`) |
| [mysqldump](mysqldump.md) | Export the database |
| [db](db.md) | Named snapshots and data-directory templates of this project's database |
| [transfer-db](transfer-db.md) | Bring a database from a remote environment |
| [masquerade](masquerade.md) | Anonymise the database |
| `dbeaver` / `sequelace` / `tableplus` | Open a desktop client on this project's database |

**Files**

| Command | What it does |
|---|---|
| [copy-to-container](copy-to-container.md) / [copy-from-container](copy-from-container.md) | (macOS) Copy between the host and the container |
| [transfer-media](transfer-media.md) | Bring media files from a remote environment |

**Tools**

| Command | What it does |
|---|---|
| [bash](bash.md) / [exec](exec.md) | A shell, or a command, inside a container (`-r` as root) |
| [debug-on](debug-on.md) / [debug-off](debug-off.md) | Enable or disable Xdebug |
| [varnish-on](varnish-on.md) / [varnish-off](varnish-off.md) | Turn Varnish on or off |
| [ssl](ssl.md) | Install an SSL certificate for the environment |
| [set-host](set-host.md) | Set a host name in `/etc/hosts` and `core_config_data` |
| [config-env](config-env.md) | Apply the project's configuration for the local environment |
| [create-project](create-project.md) | Create a new Magento project in the current directory |
| [compatibility](compatibility.md) | Magento version compatibility |
| [verify](verify.md) | Run the code checks this project has installed |
| [docker-compose](docker-compose.md) | Run `docker compose` with this project's configuration |
| [docker-stop-all](docker-stop-all.md) | Stop every running container |
| [cloud](cloud.md) / [cloud-login](cloud-login.md) | Adobe Commerce Cloud CLI |
| [completion](completion.md) | Print the shell completion script for bash or zsh |
| `web` | Bring up the web interface and print the link to open it |

**Versions**

| Command | What it does |
|---|---|
| [version](version.md) | The version of hm, Docker and Compose |
| [update](update.md) | Replace this hm with the newest release, when there is a newer one |
| [switch](switch.md) | Install another release (2.0 or later) in place of this hm |

**AI tools**

| Command | What it does |
|---|---|
| [mcp](mcp.md) | Tools for agents over the Model Context Protocol, read-only unless `--write` |
| [ai-context](ai-context.md) | Write the project context an agent reads before it starts |
| [permissions](permissions.md) | Print the permission configuration an agent should be given |
| `ai-init` / `ai-pull` / `ai-reset` / `ai-doctor` | Configure, download, remove and check AI skills and agents ([skills](skills.md), [ai-init](ai-init.md)) |

A project can add commands of its own: see [project commands](project-commands.md).
