## 6. I/O and Error Handling

### 6.1 Error Handling Philosophy

The tool implements a **"fail-fast"** approach: errors immediately stop execution to avoid inconsistent states. This is achieved through systematic use of `set -euo pipefail` in all scripts.

### 6.2 Bash Execution Modes

#### 6.2.1 Standard Configuration

**All scripts begin with:**
```bash
#!/usr/bin/env bash
set -euo pipefail
```

**Flag meanings:**

- **`-e`** (errexit): Terminates the script if any command returns a non-zero exit code
  ```bash
  docker ps  # If fails (Docker not running)
  # The script terminates here, does not continue
  ```

- **`-u`** (nounset): Treats undefined variables as errors
  ```bash
  echo $UNDEFINED_VAR  # Error: variable is not defined
  exit 1
  ```

- **`-o pipefail`**: In pipelines, returns the exit code of the first command that fails
  ```bash
  docker ps | grep phpfpm  # If docker ps fails, the pipeline fails
  # Without pipefail: only the exit code of grep would matter
  ```

#### 6.2.2 Temporary Error Disabling

Some commands expect errors as part of their logic:

```bash
# console/tasks/start_service_if_not_running.sh

set +e  # Disable "exit on error"
$DOCKER_COMPOSE exec -T "$service" sh -c "echo 'check'" &>/dev/null
service_running_error=$?  # Capture exit code
set -e  # Re-enable "exit on error"

if [ $service_running_error == 1 ]; then
    $COMMAND_BIN_NAME start  # Service not running, start it
fi
```

**When to use**:
- When the exit code is part of the logic (not an actual error)
- When validating existence of optional resources
- In test/validation commands

### 6.3 Structured Output System

#### 6.3.1 Semantic Print Functions

**Defined in**: `console/components/print_message.sh`

| Function | Color | Use | Example |
|---------|-------|-----|---------|
| `print_info()` | Green | Informational messages, progress | "Starting containers..." |
| `print_error()` | Red | Errors that stop execution | "Docker is not running!" |
| `print_warning()` | Yellow | Warnings, does not stop execution | "jq version is less than 1.6" |
| `print_processing()` | Default + 🚀 | Operations in progress | "🚀 Fixing permissions" |
| `print_question()` | Blue | Input prompts | "Magento version:" |
| `print_code()` | Brown | Commands, paths, code | "hm setup" |
| `print_link()` | Blue underline | URLs | "https://myproject.local/" |
| `print_table()` | Cyan | Tabular data | Table headers |
| `print_highlight()` | White | Emphasis | Important titles |
| `print_default()` | No color | Normal text | General content |

**Implementation**:
```bash
print_info() {
    printf "$GREEN%b$COLOR_RESET" "$1"
}

print_error() {
    printf "$RED%b$COLOR_RESET" "$1"
}
```

**ANSI color codes**:
```bash
BLUE="\033[0;34m"
GREEN="\033[0;32m"
RED="\033[0;31m"
YELLOW='\033[0;33m'
COLOR_RESET="\033[0m"
```

#### 6.3.2 Usage Patterns

**Error with exit**:
```bash
if [[ ! -f $import_database ]]; then
    print_warning "No such file: $OPTARG\n"
    exit 0
fi
```

**Long operation progress**:
```bash
print_processing "Waiting for everything to spin up..."
sleep 5
print_processing "Fixing permissions"
"$TASKS_DIR"/fix_linux_permissions.sh
print_processing "Permissions fix finished"
```

**Informative messages with code**:
```bash
print_info "Execute "
print_code "$COMMAND_BIN_NAME --help"
print_default " to see commands available\n"
```

### 6.4 User Input Handling

#### 6.4.1 Interactive Input Functions

**Defined in**: `console/components/input_info.sh`

**`custom_question()`**: Text response question

```bash
custom_question "Magento version:" "2.4.6"
magento_version=${REPLY:-2.4.6}  # $REPLY contains the answer
```

**Features**:
- Shows default value in brackets
- If user presses Enter without typing, uses the default
- Result in global variable `$REPLY`

**`custom_select()`**: Selection menu

```bash
options=("community" "enterprise")
custom_select "Magento edition:" "${options[@]}"
edition=$REPLY  # $REPLY contains the selected option
```

**Implementation** (uses native Bash `select`):
```bash
custom_select() {
    local label="$1"
    shift
    local options=("$@")
    
    print_question "$label"
    select opt in "${options[@]}"; do
        if [[ -n $opt ]]; then
            export REPLY=$opt
            break
        fi
    done
}
```

**`confirm()`**: Yes/no question

```bash
confirm "Are you satisfied with these versions? [Y/n]"
case $REPLY in
    [Yy]*) continue_setup ;;
    [Nn]*) change_versions ;;
esac
```

#### 6.4.2 Input from STDIN

**Detection**:
```bash
# console/commands/mysql.sh

if [ ! -t 0 ]; then
    # STDIN has content (redirection or pipe)
    print_info "Importing database from stdin ...\n"
    docker exec -i $mysql_container bash -c "mysql ..." 
else
    # Normal interactive mode
    process_options "$@"
fi
```

**Usage**:
```bash
cat dump.sql | hm mysql
hm mysql < dump.sql
```

#### 6.4.3 Prompts with Default Values

**Pattern for optional inputs**:
```bash
read -p "$(print_question "SSH Host" "$ssh_host")" input_ssh_host
ssh_host=${input_ssh_host:-${ssh_host}}  # Use default if input empty
```

**Complete example** (transfer-db):
```bash
# Defaults
ssh_host="ssh.eu-3.magento.cloud"
sql_port="3306"

# Prompt with visible default
read -p "$(print_question "SSH Host" "$ssh_host")" input_ssh_host
read -p "$(print_question "Database Port" "$sql_port")" input_sql_port

# Apply defaults if user didn't enter anything
ssh_host=${input_ssh_host:-${ssh_host}}
sql_port=${input_sql_port:-${sql_port}}
```

### 6.5 Validations and Error Messages

#### 6.5.1 Docker Validation

**Function**: `is_docker_service_running()` in `console/helpers/docker.sh`

```bash
is_docker_service_running() {
    if [[ ! $(docker info >/dev/null 2>&1; echo $?) -eq 0 ]]; then
        print_warning "Docker is not running!\n"
        exit 1
    fi
}
```

**Output redirection**: `>/dev/null 2>&1`
- `>/dev/null`: Discards stdout
- `2>&1`: Redirects stderr to stdout (also discarded)
- Only the exit code matters (`$?`)

#### 6.5.2 Service Validation

**Function**: `is_run_service()` in `console/helpers/docker.sh`

```bash
is_run_service() {
    is_docker_service_running
    local service="${1:-phpfpm}"
    container_id=$(docker ps -qf name="$COMPOSE_PROJECT_NAME"-"$service")
    
    if [ -z "$container_id" ]; then
        print_warning "Error: $service service is not running!\n"
        exit 1
    fi
}
```

**Used by**:
- `mysql.sh`: Requires "db" service
- `grunt.sh`: Requires "phpfpm" service
- `transfer-db.sh`: Requires "phpfpm" and "db"

#### 6.5.3 Docker Compose Config Validation

**Script**: `console/tasks/validate_docker_compose.sh`

```bash
CONFIG_IS_VALID=$($DOCKER_COMPOSE config -q && echo true || echo false)

if ! $CONFIG_IS_VALID ; then
    print_error "\nDocker is not properly configured. Please execute:\n\n"
    print_default "  $COMMAND_BIN_NAME setup\n"
    exit 1
fi
```

**Validates**:
- Correct YAML syntax
- Valid volume/network references
- Required environment variables
- Version compatibility

#### 6.5.4 Argument Validation

**Pattern with getopts**:
```bash
while getopts ":i:q:d:a" options; do
    case "$options" in
        i) import_database=${OPTARG} ;;
        q) query "$OPTARG"; exit ;;
        d) clean_definers=true ;;
        a) anonymisation=true ;;
        ?)
            print_error "The command is not correct\n\n"
            print_info "Use this format\n"
            get_usage "$(basename ${0%.sh})"
            exit 1
        ;;
    esac
done
```

**`?` case**: Captures invalid options

#### 6.5.5 File Existence Validation

```bash
import_database=${OPTARG/"~"/$HOME}  # Expand ~ to full path

if [[ ! -f $import_database ]]; then
    print_warning "No such file: $OPTARG\n"
    exit 0
fi
```

#### 6.5.6 Operating System Validation

**Script**: `console/tasks/set_machine_specific_properties.sh`

```bash
unameout="$(uname -s)"
case "$unameout" in
    Linux*) MACHINE="linux" ;;
    Darwin*) MACHINE="mac" ;;
    *)
        MACHINE="UNKNOWN"
        print_error "Error: Unsupported system type\n"
        print_error "System must be a Macintosh or Linux\n"
        exit 1
    ;;
esac
```

### 6.6 Output Redirection and Suppression

#### 6.6.1 Redirection Patterns

**Discard all output**:
```bash
command &>/dev/null
# Equivalent to:
command >/dev/null 2>&1
```

**Discard only stderr**:
```bash
command 2>/dev/null
```

**Example in use**:
```bash
# Test if command exists
if ! command -v rsync &>/dev/null; then
    print_error "Error: Rsync command not available\n"
    exit 1
fi
```

#### 6.6.2 Output Capture

**Capture in variable**:
```bash
container_id=$($DOCKER_COMPOSE ps -q phpfpm)
jq_version=$(jq --version | cut -d'-' -f2)
```

**Capture exit code**:
```bash
$DOCKER_COMPOSE exec -T "$service" sh -c "echo 'check'" &>/dev/null
service_running_error=$?
```

#### 6.6.3 Input Redirection to Containers

**SQL file import**:
```bash
docker exec -i $mysql_container bash -c "mysql -u\"root\" -p\"password\" \"magento\"" < dump.sql
```

**From pipe**:
```bash
cat dump.sql | docker exec -i $mysql_container bash -c "mysql ..."
```

**`-i` flag**: Keeps STDIN open for the container

### 6.7 Logging and Traceability

#### 6.7.1 Progress Messages

**Long operations show progress**:
```bash
print_info "Importing database from file ...\n"
docker exec -i $mysql_container bash -c "mysql ..." < $import_database

print_info "Anonymising database in localhost...\n"
masquerade_run

print_info "Setup completed!!!\n"
```

#### 6.7.2 Visible Docker Commands

**Important commands are shown before execution**:
```bash
print_processing "bin/magento $final_command"
"$COMMANDS_DIR"/magento.sh $final_command
```

#### 6.7.3 No Persistent Logging

**Design decision**: The tool **does not maintain its own logs**

**Reasons**:
- Docker Compose logs are accessible via `docker-compose logs`
- Direct terminal output allows real-time debugging
- Avoids log rotation management
- Users can redirect output if they need logs: `hm start > log.txt 2>&1`

**To view service logs**:
```bash
hm docker-compose logs -f phpfpm
hm docker-compose logs --tail=100 db
```

### 6.8 Signal Handling and Cleanup

#### 6.8.1 Interruption with Ctrl+C

**Behavior**: `set -e` ensures that Ctrl+C terminates the script cleanly

**Long-running commands**:
```bash
# grunt watch keeps process running
docker-compose exec phpfpm bash -c "grunt exec:magento && grunt watch:magento"
# Ctrl+C stops the watch and exits the script
```

#### 6.8.2 No Custom Signal Handlers

The tool **does not use traps** for signals (SIGINT, SIGTERM)

**Reason**: Docker containers handle signals independently, no additional cleanup required on the host

### 6.9 Exit Codes

#### 6.9.1 Conventions

| Exit Code | Meaning | Examples |
|-----------|---------|----------|
| `0` | Success | Command completed correctly |
| `1` | General error | Docker not running, service unavailable, file doesn't exist |
| Exit code of last command | Propagated by `set -e` | If `docker-compose up` fails, its exit code is propagated |

**All errors use `exit 1`**:
```bash
if [ ! -f "$file" ]; then
    print_error "File not found\n"
    exit 1
fi
```

#### 6.9.2 Exit Code Propagation

**With `set -e`**, the exit code of the first failing command is propagated:

```bash
# If docker-compose up fails with exit code 125
$DOCKER_COMPOSE up -d
# Script terminates with exit code 125
```

**User can capture**:
```bash
hm start
echo $?  # Shows the exit code of hm start
```

### 6.10 Implemented Best Practices

#### 6.10.1 Fail-Fast

✅ **Does**: Terminates immediately on errors
```bash
set -euo pipefail
docker info  # If fails, the entire script terminates
```

❌ **Avoids**: Continuing after errors
```bash
# BAD: Without set -e
docker info  # Fails but the script continues
docker-compose up  # Executes anyway
```

#### 6.10.2 Descriptive Messages

✅ **Does**: Errors explain what went wrong and what to do
```bash
print_error "Docker is not properly configured. Please execute:\n\n"
print_default "  $COMMAND_BIN_NAME setup\n"
```

❌ **Avoids**: Cryptic errors
```bash
echo "Error"
exit 1
```

#### 6.10.3 Early Validations

✅ **Does**: Validates preconditions before expensive operations
```bash
is_docker_service_running     # Validate first
is_run_service "db"            # Validate service
# Now yes, execute import (long operation)
docker exec -i $mysql_container bash -c "mysql ..." < dump.sql
```

#### 6.10.4 Safe Input

✅ **Does**: Sanitizes and validates inputs
```bash
import_database=${OPTARG/"~"/$HOME}  # Expand ~
if [[ ! -f $import_database ]]; then
    print_warning "No such file: $OPTARG\n"
    exit 0
fi
```

✅ **Does**: Confirmations for destructive operations
```bash
read -p "Are you sure you want to anonymise your database? [Y/n]: " confirmation
if [ -z "$confirmation" ] || [ "$confirmation" == 'Y' ]; then
    masquerade_run
fi
```

#### 6.10.5 Clean Outputs

✅ **Does**: Discards irrelevant output, shows only what's important
```bash
# Test command: discard output
if ! command -v rsync &>/dev/null; then
    print_error "Error: Rsync command not available\n"
fi

# Informative command: show output
print_info "Importing database from file ...\n"
docker exec -i $mysql_container bash -c "mysql ..." < $import_database
```
