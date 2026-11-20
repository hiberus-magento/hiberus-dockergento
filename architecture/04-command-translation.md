## 4. Command Translation Layer

Each command is a `case` in `internal/cli/run.go`, a handler in `internal/cli/<command>.go` and,
when it does more than run one thing in a container, a use case in `dockergento/app`. This section
describes what each kind of command turns into.

### 4.1 Translation Principle

Each `hm` command translates a Magento-level operation into one or more operations on the
project's containers:

1. Resolve the project (`projectOr`, or `codeProjectOr` for commands that work on the code: from a
   worktree with no environment of its own, those say whose containers they reach)
2. Select the target service
3. Build the command to run there, with the right user and terminal
4. Run it through the engine — `Engine.Exec`, which uses Compose as a library
   (`composelib.Orchestrator.Exec`) — and return its exit code

No `docker` or `docker compose` process is started for this: `composelib` and `dockerd` talk to
the Docker API. The one exception is `hm docker-compose`, a passthrough to the Compose CLI
(`adapters/composecli`).

### 4.2 Translation Patterns by Category

#### 4.2.1 Commands that run one thing in the PHP container

**Pattern**: `hm <command> [args]` → exec in `phpfpm` (the `inside` helper in
`internal/cli/php.go`), attached to the terminal, a TTY only when stdin and stdout are terminals.

| User command | Service | What runs there |
|---|---|---|
| `hm magento cache:clean` | phpfpm | `php ./bin/magento cache:clean` |
| `hm composer install` | phpfpm | `composer install` (on macOS see 4.5.2) |
| `hm npm install` | phpfpm | `npm install` |
| `hm n98-magerun sys:info` | phpfpm | `bash -c "n98-magerun sys:info"` |
| `hm test-unit [path]` | phpfpm | `<BIN_DIR>/phpunit --config ./dev/tests/unit/phpunit.xml.dist [path]` |
| `hm test-integration [path]` | phpfpm | `cd ./dev/tests/integration && .../phpunit --config phpunit.xml [path]` |

The arguments are passed through untouched, so `hm magento setup:upgrade --keep-generated` reaches
`bin/magento` as typed, and the tool's global flags after the command name belong to the program
(`hm composer show --format=json`).

#### 4.2.2 Commands with a chosen container

`hm exec` and `hm bash` default to `phpfpm`; `-r` runs as root.

| Command | Target service | Why |
|---|---|---|
| `hm bash`, `hm exec` | **phpfpm** (or the one named) | PHP, Composer and the Magento CLI live there |
| `hm mysql`, `hm mysqldump` | **db** | The MariaDB server and its client |
| `hm magento`, `hm composer`, `hm npm`, `hm grunt` | **phpfpm** | They need PHP, Node and the Magento filesystem |

```
hm bash -r            → exec in phpfpm as root: bash
hm exec -r chown ...  → exec in phpfpm as root: chown ...
```

#### 4.2.3 Database commands

| User command | What happens |
|---|---|
| `hm mysql` | An interactive client in `db` (`Engine.Console`) |
| `hm mysql -q "SELECT ..."` | One statement, its output to stdout (`Engine.Query`) |
| `hm mysql -i dump.sql` | Import a file, then point the store at this machine (`Engine.Import`); `-d` removes `DEFINER`s first, `-a` anonymises afterwards |
| `hm mysql < dump.sql` | Import from stdin, streamed into the client (`Engine.Feed`) |
| `hm mysqldump file.sql` | Dump through this process into the file, so the dumper's own messages never land inside the dump (`Engine.Dump`) |

The client and dumper are `mariadb`/`mariadb-dump` when the image has them, `mysql`/`mysqldump`
otherwise, with the credentials of the db container's environment (`MYSQL_USER`,
`MYSQL_PASSWORD`, `MYSQL_DATABASE`, `MYSQL_ROOT_PASSWORD`).

#### 4.2.4 Commands made of several steps

**`hm grunt Vendor/theme es_ES`** (`internal/cli/grunt.go`): each step is one `bash -c` in
`phpfpm`, and the first that fails stops the rest:

```
1. Copy package.json.sample, grunt-config.json.sample and Gruntfile.js.sample into place
2. npm install && npm update
3. Write the theme into dev/tools/grunt/configs/local-themes.js
4. grunt exec:magento && grunt watch:magento   (holds the terminal until interrupted)
```

**`hm transfer-db`** (`internal/cli/transfer_db.go`): asks for the SSH and database coordinates
(Magento Cloud defaults offered), then:

```
1. In db: dump the remote database (mariadb-dump/mysqldump through the SSH connection),
   remove DEFINERs, gzip into /tmp/db.sql.gz
2. Optionally keep this environment's core_config_data, admin_user and tfa_user_config
3. In db: import /tmp/db.sql.gz, then restore what was kept
4. Optionally anonymise, then ask before each of app:config:import, indexer:reindex and
   cache:flush in phpfpm
```

**`hm transfer-media`** runs `rsync` over SSH on the host, copying a remote store's `pub/media`
into `./pub`, keeping what is already here and leaving the caches behind.

### 4.3 What a Command Can Read

#### 4.3.1 Container environment

Values defined in the Compose files are read inside the container, never interpolated by the
host:

- **db**: `MYSQL_ROOT_PASSWORD`, `MYSQL_DATABASE`, `MYSQL_USER`, `MYSQL_PASSWORD`
- **phpfpm**: `COMPOSER_VERSION`, `PHP_IDE_CONFIG`
- **rabbitmq**: `RABBITMQ_DEFAULT_USER`, `RABBITMQ_DEFAULT_PASS`

When the host needs them (the database credentials for `masquerade`, for example), it reads them
from the db container rather than assuming them.

#### 4.3.2 Project properties

Paths inside the container come from the project's properties (section 5): `WORKDIR_PHP`
(default `/var/www/html`) and `BIN_DIR` (default `./vendor/bin`).

### 4.4 Option and Argument Handling

#### 4.4.1 Declared options

`data/command_descriptions.json` declares each command's options with their short and long forms
(`{"short": "p", "long": "project-name"}`). Each handler parses its own arguments and accepts the
forms declared there; an option it does not know is a usage error (exit 2). The same declaration
feeds `hm <command> --help` and the shell completion, so they cannot drift from what is accepted.

#### 4.4.2 Global flags

`globals` (`internal/cli/globals.go`) takes `--json`, `--no-json`, `--yes`, `--force` and
`--no-color` out before routing. After the command name they are taken out too, except for the
commands that hand their arguments to another program (`exec`, `bash`, `magento`, `composer`,
`npm`, `n98-magerun`, `grunt`, `test-*`, `cloud`, `cloud-login`, `masquerade`, `mysql`,
`mysqldump`, `docker-compose`, `copy-to-container`, `copy-from-container`, `logs`), where they
belong to that program.

### 4.5 Special Translation Cases

#### 4.5.1 Commands that do not need containers

| Command | What it does | Why |
|---|---|---|
| `hm setup` | Renders `docker-compose.yml` and the overlays, writes properties | The files must exist before any container |
| `hm create-project` | Creates the Magento project and its configuration | Initialisation before containers |
| `hm update`, `hm switch` | Move the git checkout of the tool and get its binary | Updates the tool itself |
| `hm compatibility` | Reads `data/requirements.json` | Local metadata |
| `hm transfer-media` | `rsync` from the remote host | More efficient than through a container |
| `hm list`, `hm tui` | Read every environment from container labels | Machine-wide, not a project |

#### 4.5.2 Commands with conditional logic

**`hm composer` on macOS** (`composerMirroringVendor` in `internal/cli/php.go`): for `install`,
`update`, `require` and `remove`, where the code lives in a volume:

```
1. Restart phpfpm
2. Copy the host's vendor into the container
3. Run Composer inside
4. Copy the whole workdir back out over the host's
```

A failing step stops the rest with its own code. From a git worktree with no environment of its
own it is refused (exit 6), because it would rewrite the main checkout's dependencies; on Linux
and for every other subcommand it is a plain exec.

#### 4.5.3 Commands composed of other operations

- `hm restart` stops and starts the services named.
- `hm purge` is one `rm -rf` in phpfpm over what Magento can generate again: `var/cache`,
  `generated`, `pub/static`, `var/view_preprocessed`, `var/page_cache`, `var/generation` and
  `dev/tests/integration/tmp` (never `var/log`).
- `hm install` runs `setup:install` with the project's configuration, then applies the local
  settings (section 5.6).
- `hm start` brings the services up and, on Linux, runs the post-start steps (`hm post-start`
  runs them on their own).

### 4.6 Reference Table

| User command | Service | What it becomes | Complexity |
|---|---|---|---|
| `hm bash` | phpfpm | exec `bash` | Simple |
| `hm magento <cmd>` | phpfpm | exec `php ./bin/magento <cmd>` | Simple |
| `hm composer <cmd>` | phpfpm | exec `composer <cmd>` | Conditional (macOS) |
| `hm mysql` | db | interactive client | Simple |
| `hm mysql -i dump.sql` | db | import, then local settings | Medium |
| `hm grunt <theme> <locale>` | phpfpm | four sequential steps | High |
| `hm transfer-db` | db | remote dump over SSH, import, restore | High |
| `hm setup` | - | host files and Compose rendering | High |
| `hm start` | - | Compose `up` through the library, then post-start | Medium |
| `hm exec <cmd>` | phpfpm | exec `<cmd>` | Simple |
| `hm exec -r <cmd>` | phpfpm | exec `<cmd>` as root | Simple |
| `hm docker-compose <args>` | - | the Compose CLI with the project's files (`composecli`) | Simple |
