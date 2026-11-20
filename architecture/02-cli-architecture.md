## 2. CLI Architecture

### 2.1 Design Pattern: Command Router

The tool implements a **Command Router** pattern. The `cmd/hm` Go binary is the entry point and answers every command: `internal/cli/run.go`'s switch routes each one to its handler. What the switch does not know falls through, arguments untouched, to the `bin/run` Bash bridge, which now only serves help (`hm`, `hm --help`, `hm <cmd> --help`), `--version`, project-defined commands in `config/hm/commands/*.sh`, and the `command_moved_to_binary` refusal for a known command that reaches it directly. `MIGRATION.md` has the history of the migration.

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
   internal/cli.Run → bin/run
          ↓
   help / --version / config/hm/commands/<command>.sh / command_moved_to_binary refusal
```

### 2.3 Main Components

#### 2.3.0 The Go binary

- `cmd/hm`: `main`, built with `go build -o bin/hm ./cmd/hm`
- `internal/cli`: the driving adapter — argument parsing, output, one file per command
- `dockergento/app`: use cases, written against ports
- `dockergento/core`: pure rules and types (compose rendering, properties, refusals)
- `dockergento/ports` and `dockergento/adapters/*`: the interfaces and their implementations, wired in `dockergento/dockergento.go`

Adding a command: a `case` in `run.go`, its handler and test in `internal/cli`, the use case in `dockergento/app`, and its entry in `data/command_descriptions.json`, which the help and the shell completion read.

#### 2.3.1 Entry Point (`bin/run`)

The Bash bridge. It keeps the structure it had as the dispatcher, but no command script ships in `console/commands/` any more: `check_command_dir` finds only project-defined commands, and refuses a known command with `command_moved_to_binary`.

```bash
# Structure of bin/run
1. resolve_absolute_dir()    # Resolves absolute paths for the CLI
2. init_dirs()                # Exports system directory variables
3. import_dependencies()      # Loads tasks, helpers, components
4. check_command_dir()        # Validates if the command exists
5. docker_env_variables()     # Configures DOCKER_COMPOSE and COMPOSE_PROJECT_NAME
6. validate_command()         # Verifies that Docker is running
7. execute()                  # Executes the specific command
```

**Exported environment variables:**
- `COMMAND_BIN_NAME="hm"`
- `COMMAND_TOOLNAME="Hiberus Dockergento"`
- `COMMAND_BIN_DIR`: Path to the CLI root directory
- `COMMANDS_DIR`: Path to `console/commands/` (no longer shipped; replaced by `CUSTOM_COMMANDS_DIR` for a project command)
- `CUSTOM_COMMANDS_DIR`: Path to project custom commands (if they exist)
- `TASKS_DIR`: Path to `console/tasks/`
- `HELPERS_DIR`: Path to `console/helpers/`
- `COMPONENTS_DIR`: Path to `console/components/`
- `DATA_DIR`: Path to `data/`

#### 2.3.2 Functional Directory Structure

What is left under `console/` is what `bin/run` loads:

**`console/tasks/`**: `load_properties.sh` (loads and merges JSON properties), `set_machine_specific_properties.sh` (detects Mac/Linux), `set_environment_labels.sh`, `worktree_env.sh`, ...

**`console/helpers/`**: `docker.sh`, `global_options.sh`, `process_hm_options.sh`, `print_help.sh` (help system), `version.sh`, `project_name.sh`, ...

**`console/components/`**: `print_message.sh` (colored printing: print_info, print_error, print_warning), `print_json.sh`

#### 2.3.3 Properties Loading System

Properties are loaded through a **cascade merge** system:

```bash
# Order of precedence (highest to lowest):
1. Project properties: config/docker/properties.json
2. Default properties: data/properties.json
```

**Loading process** (`console/tasks/load_properties.sh`):
```bash
1. Load data/properties.json (default values)
2. If config/docker/properties.json exists, merge over defaults
3. Convert JSON to environment variables with jq:
   {"MAGENTO_DIR": "."} → export MAGENTO_DIR="."
4. Use 'set -a' to auto-export all variables
```

**Key properties:**
- `MAGENTO_DIR`: Magento root directory (default: ".")
- `DOCKER_COMPOSE_FILE`: Main docker-compose file
- `DOCKER_COMPOSE_FILE_MAC/LINUX`: OS-specific overlays
- `USER_PHP`, `GROUP_PHP`: User/group inside PHP container
- `WORKDIR_PHP`: Working directory in container (default: "/var/www/html")
- `COMPOSE_PROJECT_NAME`: Project name for container isolation

#### 2.3.4 Validation and Routing

**Command validation** (`check_command_dir`, in `bin/run`):
```bash
1. Search in config/hm/commands/<command>.sh (project-defined commands)
2. If it is a command declared in data/command_descriptions.json, refuse with
   command_moved_to_binary: the binary answers it
3. Otherwise, show error and suggest 'hm --help'
```

**Docker validation** (`validate_command`):
- Commands like `setup`, `create-project`, `compatibility`, `update` don't require Docker
- Rest of commands execute `is_docker_service_running` before proceeding
- Validates existence of `docker-compose.yml` before execution

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

2. **Global Options Processing**: Before routing, processes `--help`, `-h`, `--version`, `-v`

3. **Dynamic Docker Compose Configuration**: 
   ```bash
   DOCKER_COMPOSE="docker-compose -f docker-compose.yml -f docker-compose.dev.mac.yml"
   ```
   Built dynamically based on the detected operating system

4. **Cascade Sourcing**: Commands can load additional helpers/tasks/components as needed via `source`

5. **Exit Early Pattern**: Validations fail fast with descriptive error messages before executing costly operations
