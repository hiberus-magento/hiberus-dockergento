## 2. CLI Architecture

### 2.1 Design Pattern: Command Router

The tool implements a **Command Router** pattern written entirely in **Bash**. The execution flow follows a modular layered architecture that separates the responsibilities of initialization, validation, routing, and execution.

### 2.2 Execution Flow

```
User executes: hm <command> [options] [arguments]
          ↓
   bin/run (Entry Point)
          ↓
   1. Directory Initialization
          ↓
   2. Dependency Loading
          ↓
   3. Properties Loading (JSON → Environment Variables)
          ↓
   4. Global Options Processing (--help, --version)
          ↓
   5. Command Validation
          ↓
   6. Docker Variables Configuration
          ↓
   7. Docker and docker-compose Validation
          ↓
   8. Command Script Execution
          ↓
   console/commands/<command>.sh
```

### 2.3 Main Components

#### 2.3.1 Entry Point (`bin/run`)

The main script that acts as a dispatcher:

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
- `COMMANDS_DIR`: Path to `console/commands/`
- `CUSTOM_COMMANDS_DIR`: Path to project custom commands (if they exist)
- `TASKS_DIR`: Path to `console/tasks/`
- `HELPERS_DIR`: Path to `console/helpers/`
- `COMPONENTS_DIR`: Path to `console/components/`
- `DATA_DIR`: Path to `data/`

#### 2.3.2 Functional Directory Structure

**`console/commands/`**: Individual command scripts
- Each command is an independent `.sh` file
- Example: `magento.sh`, `composer.sh`, `mysql.sh`
- Supports custom commands in project's `config/hm/commands/`

**`console/tasks/`**: Reusable tasks shared between commands
- `load_properties.sh`: Loads and merges JSON properties
- `version_manager.sh`: Manages service version selection
- `magento_installation.sh`: Orchestrates Magento installation
- `set_machine_specific_properties.sh`: Detects OS (Mac/Linux)

**`console/helpers/`**: Auxiliary functions
- `docker.sh`: Docker state validations
- `properties.sh`: Project properties management
- `map_arguments.sh`: Translates long arguments to short ones
- `print_help.sh`: Help system

**`console/components/`**: Reusable UI components
- `print_message.sh`: Colored printing functions (print_info, print_error, print_warning)
- `input_info.sh`: Interactive input functions (custom_question, custom_select, confirm)
- `masquerade.sh`: Integration with anonymization tool

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

**Command validation** (`check_command_dir`):
```bash
1. Search in console/commands/<command>.sh
2. If not found, search in config/hm/commands/<command>.sh (custom commands)
3. If neither exists, show error and suggest 'hm --help'
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
- Supports conversion of long arguments to short ones (`--import` → `-i`)

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
