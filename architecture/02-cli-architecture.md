## 2. CLI Architecture

### 2.1 Design Pattern: Command Router

The tool implements a **Command Router** pattern. The `cmd/hm` Go binary is the entry point and answers every command: `internal/cli/run.go`'s switch routes each one to its handler. The binary also answers the help (`hm`, `hm --help`, `hm <cmd> --help`) and `--version`/`-v`. What the switch does not know is a project-defined command, `config/hm/commands/<name>.sh`, run by `internal/cli/project_command.go`; any other name is `command_not_found`. There is no Bash entry point: `bin/run` and the `console/` tree were deleted once the binary answered everything. `MIGRATION.md` has the history of the migration.

### 2.2 Execution Flow

```
User executes: hm <command> [options] [arguments]
          ↓
   cmd/hm → internal/cli.Run
          ↓
   1. Global flags taken out (--json, --no-json, --yes, ...)
          ↓
   2. The switch in run.go picks the handler (internal/cli/<command>.go)
          ↓
   3. The handler parses its own options (an undeclared one is a usage error, exit 2)
          ↓
   4. The engine (dockergento.Engine, behind the `commands` interface in internal/cli/engine.go)
      runs the use case in dockergento/app
          ↓
   5. The use case talks to Docker, Compose, the filesystem... through dockergento/ports,
      implemented by dockergento/adapters/*
          ↓
   6. The handler renders the answer: text, or the JSON envelope
```

Anything not in the switch:

```
   internal/cli.Run → projectCommand (internal/cli/project_command.go)
          ↓
   config/hm/commands/<command>.sh, or command_not_found (exit 2)
```

### 2.3 Main Components

#### 2.3.0 The Go binary

- `cmd/hm`: `main`, built with `go build -o bin/hm ./cmd/hm`
- `internal/cli`: the driving adapter — argument parsing, output, one file per command
- `dockergento/app`: use cases, written against ports
- `dockergento/core`: pure rules and types (compose rendering, properties, refusals)
- `dockergento/ports` and `dockergento/adapters/*`: the interfaces and their implementations, wired in `dockergento/dockergento.go` (the `Engine` facade)
- `dockergento/contract`: the JSON envelope and the exit codes, shared by every door
- The other doors over the same engine: the dashboard (`internal/cli/tui*.go`), the MCP server (`internal/cli/mcp*.go`) and the HTTP API (`internal/api`, started by `hm web`)

Dependencies point inward: `core` imports nothing of the tool's, `app` imports `core` and `ports`, adapters implement ports, and `internal/*` reaches the engine through the facade. Handlers call it through the narrow `commands` interface (`internal/cli/engine.go`), built by the `newEngine` variable; tests replace that variable with a fake (`internal/cli/fake_engine_test.go`) and run handlers through `Run` with no daemon.

Adding a command: its entry in `data/command_descriptions.json` (which the help, its groups and the shell completion read), a `case` in `run.go`, its handler and test (against the fake engine) in `internal/cli`, the use case in `dockergento/app` with a method on the `commands` interface when the handler needs the engine, `docs/<command>.md`, and its row in `MIGRATION.md`'s command table. The constitution (`.specify/memory/constitution.md`) lists the rules a new command follows.

#### 2.3.1 Project-defined commands (`internal/cli/project_command.go`)

A project adds commands of its own as executable scripts at `config/hm/commands/<name>.sh` under its root. The router reaches them only after every routed name, so a command of the tool always wins. Before the script runs, the binary:

```
1. Resolves the project root (from a worktree, the main checkout's)
2. Builds the script's environment: COMMAND_BIN_NAME, COMMAND_TOOLNAME, COMMAND_BIN_DIR,
   DATA_DIR, HM_ROOT, COMMANDS_DIR / CUSTOM_COMMANDS_DIR (the project's commands directory),
   the merged properties, the colour variables, HM_EXIT_*, MACHINE, HM_OUTPUT_FORMAT,
   COMPOSE_PROJECT_NAME (configured, or derived from the directory), DOCKER_COMPOSE
3. Checks that Docker answers and that the Compose configuration is valid (cached per file)
4. Runs the script with the remaining arguments, and returns its exit code
```

The environment is the one the shell entry point exported, so scripts written against it keep working — except that `HELPERS_DIR`, `TASKS_DIR` and `COMPONENTS_DIR` are no longer set: the shell tree they pointed at is gone. That is a deliberate incompatible change of 2.0 — project scripts must be self-contained. A script with no `#!` line is run with `bash`, which is what executing it from the shell entry point amounted to.

#### 2.3.2 What is left of `console/`

Only `console/hm-completion.bash`, the shell completion `generate_completion.sh` writes from `data/command_descriptions.json`. It stays at that path because users' shell profiles source it there.

#### 2.3.3 Properties Loading System

Properties are loaded through a **cascade merge** system:

```bash
# Order of precedence (highest to lowest):
1. Project properties: config/docker/properties.json
2. Default properties: data/properties.json
```

**Loading process** (`dockergento/adapters/fsprops`):
```
1. Load data/properties.json (default values)
2. If config/docker/properties.json exists, merge it over the defaults
3. A directory with no properties of its own is not a project: nothing is returned
4. COMPOSE_PROJECT_NAME, when the project does not configure it, is the name Docker Compose
   derives from the directory (core.DeriveProjectName, applied in app.Resolver)
```

**Key properties:**
- `MAGENTO_DIR`: Magento root directory (default: ".")
- `DOCKER_COMPOSE_FILE`: Main docker-compose file
- `DOCKER_COMPOSE_FILE_MAC/LINUX`: OS-specific overlays
- `USER_PHP`, `GROUP_PHP`: User/group inside PHP container
- `WORKDIR_PHP`: Working directory in container (default: "/var/www/html")
- `COMPOSE_PROJECT_NAME`: Project name for container isolation

#### 2.3.4 Validation and Routing

Each handler validates for itself: its own options (an undeclared one is a usage error, exit 2), then whether it needs a project (exit 4 outside one) and a running Docker daemon (exit 3). A name the switch does not route is looked up as a project command, and otherwise refused with `command_not_found` and the hint `hm --help`.

#### 2.3.5 Help System and Metadata

Command metadata is stored in `data/command_descriptions.json`:

```json
{
  "magento": {
    "description": "Execute Magento console commands",
    "args": [{"name": "magento-subcommand", "description": "..."}],
    "example": "magento cache:clean"
  }
}
```

**Usage:**
- `hm --help`: Lists all available commands
- `hm <command> --help`: Shows command-specific help

### 2.4 Special Router Features

1. **Custom Commands Support**: Projects can define additional commands in `config/hm/commands/` that integrate automatically

2. **Global Options Processing**: Before routing, takes out `--help`, `-h`, `--version`, `-v`, `--json`, `--no-json`, `--yes`, `--no-color`

3. **Dynamic Docker Compose Configuration**: the compose files are chosen for the detected operating system (`Engine.ComposeFiles`), and a project command gets them as `DOCKER_COMPOSE`

4. **Exit Early Pattern**: Validations fail fast with descriptive error messages before executing costly operations
