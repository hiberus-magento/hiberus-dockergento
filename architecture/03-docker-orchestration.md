## 3. Container Orchestration

### 3.1 Interaction with Docker and Docker Compose

The tool is an **abstraction layer over Docker Compose and the Docker daemon**, and since 2.0 it talks to both as libraries, not through their command lines:

| Adapter | What it does | How |
|---|---|---|
| `adapters/composelib` | Project lifecycle: `up`, `up --build`, `stop`, `down`, `logs`, `exec`, the services of a project | Compose as a Go library (`github.com/docker/compose/v2`), with the project's compose files |
| `adapters/dockerd` | The daemon: reachability, containers and their labels, exec and copy, one-off containers (masquerade, relays), volumes, networks | The Docker API (`github.com/docker/docker`) |
| `adapters/composecfg` | Loading and validating the Compose configuration | `compose-go` |
| `adapters/composecli` | `hm docker-compose <args>` only: a passthrough to the Compose CLI with the project's files | `os/exec` — the one documented subprocess of Compose |
| `adapters/traefik` | What the global proxy (`hm proxy`) is actually routing | Traefik's own API |

`adapters/toolinfo` also runs `docker compose version` to report the Compose version in `hm version` and `hm doctor`.

### 3.2 Central Variable: `DOCKER_COMPOSE`

The core of orchestration is the list of compose files for the platform. The Go binary builds it in `Engine.ComposeFiles` (`dockergento/dockergento.go`) and runs Compose through its adapters. A project-defined command (`config/hm/commands/<name>.sh`) gets the same list as the `DOCKER_COMPOSE` variable (`internal/cli/project_command.go`):

```bash
# What a project-defined command is handed
DOCKER_COMPOSE="docker compose -f docker-compose.yml -f docker-compose.dev.mac.yml"
# or
DOCKER_COMPOSE="docker compose -f docker-compose.yml -f docker-compose.dev.linux.yml"
```

**Usage** (in a project-defined command):
```bash
$DOCKER_COMPOSE up -d
$DOCKER_COMPOSE exec phpfpm php bin/magento cache:clean
$DOCKER_COMPOSE ps -q phpfpm
```

### 3.3 Docker Compose File Generation

#### 3.3.1 Template System

The tool generates `docker-compose.yml` files from templates with placeholders:

**Template** (`docker-compose/docker-compose.template.yml`):
```yaml
services:
  phpfpm:
    image: hiberusmagento/php:<php_version>
    environment:
      COMPOSER_VERSION: <composer_version>
  db:
    image: hiberusmagento/mariadb:<mariadb_version>
  search:
    image: hiberusmagento/search:<search_version>
  redis:
    image: hiberusmagento/redis:<redis_version>
  varnish:
    image: hiberusmagento/varnish:<varnish_version>
```

**Generation process** (`core.RenderCompose` in `dockergento/core/setup.go`, called by `app.Setup` during `hm setup`):

```bash
1. Read the requirements for the Magento version (data/requirements.json)
2. Replace each <service_version> marker with the service's image tag:
   <php_version>     → 8.2-buster
   <mariadb_version> → 10.6
   <search_version>  → 2.5-opensearch
   and <mail_service>/<mail_version> with the chosen mail catcher (mailhog or mailpit)
3. Write the result → docker-compose.yml
4. Copy OS-specific overlays:
   - docker-compose.dev.mac.template.yml → docker-compose.dev.mac.yml
   - docker-compose.dev.linux.template.yml → docker-compose.dev.linux.yml
```

#### 3.3.2 Operating System Overlay Strategy

**macOS** (`docker-compose.dev.mac.yml`):
- Uses **named volume** (`workspace`) for code
- Applies **:cached** strategy on specific bind mounts
- Selectively syncs: `app/`, `composer.json`, `composer.lock`, `.git/`, `config/`
- **Reason**: Improves I/O performance on macOS where native bind mounts are slow

```yaml
volumes:
  - workspace:/var/www/html                         # Named volume
  - {MAGENTO_DIR}/app:/var/www/html/app:cached     # Bind mount with cache
  - {MAGENTO_DIR}/composer.json:/var/www/html/composer.json:cached
```

**Linux** (`docker-compose.dev.linux.yml`):
- Uses **direct bind mount** of the complete directory
- No cache optimizations (not needed)

```yaml
volumes:
  - {MAGENTO_DIR}/.:/var/www/html  # Direct bind mount
```

### 3.4 Operating System Detection and Configuration

**Automatic process** (`Platform()` in `dockergento/dockergento.go`):

```bash
uname -s
  ↓
"Linux" → MACHINE="linux", DOCKER_COMPOSE_FILE_MACHINE="docker-compose.dev.linux.yml"
  ↓
"Darwin" → MACHINE="mac", DOCKER_COMPOSE_FILE_MACHINE="docker-compose.dev.mac.yml"
  ↓
Other → ERROR: "Unsupported system type"
```

A project-defined command is handed:
- `MACHINE`: "mac" or "linux"
- `DOCKER_COMPOSE_FILE_MACHINE`: Path to the correct overlay

### 3.5 Docker State Validations

#### 3.5.1 Docker Daemon Validation

**Function**: `Daemon.Reachable()` in `dockergento/adapters/dockerd/daemon.go`

```
Docker API ping
  ↓
Answers → Docker running, continue
  ↓
No answer → docker_unavailable, "Docker is not running", exit 3
```

**Executed by**: every command that needs the daemon, in its own handler; `setup`, `create-project`, `compatibility`, `update` and the help do not ask

#### 3.5.2 Docker Compose Configuration Validation

**Where**: before a project-defined command runs (`projectRun.validate` in `internal/cli/project_command.go`); the binary's own commands read the configuration through `composecfg` and refuse with `project_not_configured` when it does not load

```bash
$DOCKER_COMPOSE config -q
  ↓
Exit code 0 → Valid configuration
  ↓
Exit code ≠ 0 → "Docker is not properly configured. Please execute: hm setup"
```

Validates:
- YAML file syntax
- Volume and network references
- Required environment variables
- docker-compose version compatibility

#### 3.5.3 Running Services Validation

**Where**: the use cases that need a service up (`dockergento/app/container.go` and others) look up its container by the Compose project and service labels

```
Container found running → continue
  ↓
None → service_not_running, exit 5
```

**Used by**: Commands that require specific services (mysql, masquerade, transfer-db)

### 3.6 Container Interaction Patterns

#### 3.6.1 Command Execution in Containers

`Engine.Exec(dir, service, command, options)` → `composelib.Orchestrator.Exec`, which runs the
command in the service's container through the Docker API and returns the command's own exit
code. `core.ExecOptions` carries the user (`-r` is root), whether stdin is attached and whether a
TTY is allocated — only when stdin and stdout are terminals, which is what keeps a command that
works by hand from failing in CI.

```
hm magento cache:clean   → Exec(phpfpm, [php ./bin/magento cache:clean], user default)
hm exec -r chown ...     → Exec(phpfpm, [chown ...], user root)
```

#### 3.6.2 Lifecycle Management

| Command | Use case | Through |
|---|---|---|
| `hm start [service]` | `Engine.Start` | `Orchestrator.Up`, then on Linux the post-start steps (`dockergento/app/after_start.go`): the host's ids for PHP's user and group, and the self-routing hosts entries |
| `hm stop [service]` | `Engine.Stop` | `Orchestrator.Stop` |
| `hm restart [service]` | `Engine.Restart` | stop, then up |
| `hm rebuild [service]` | `Engine.Rebuild` | `Orchestrator.UpBuilding` (build only the services that have a build section) |
| `hm down [-v]` | `Engine.Down` | `Orchestrator.Down`; with `-v` it asks first whether to save a snapshot ([db](../docs/db.md)) |

`hm start -s` stops every other running container first (`StopEverything`), so the ports are
free.

#### 3.6.3 No Automatic Service Startup

A command that needs a service which is not running does not start it: `resolveContainer`
(`dockergento/app/container.go`) refuses with `service_not_running` (exit 5) and the hint
`hm start <service>`.

### 3.7 Volume and Data Management

#### 3.7.1 Defined Persistent Volumes

```yaml
volumes:
  dbdata:          # MariaDB data
  sockdata:        # Unix sockets PHP-FPM <-> Nginx
  searchdata:      # Elasticsearch indexes
  opensearchdata:  # OpenSearch indexes
  redisdata:       # Redis cache
  rabbitmqdata:    # RabbitMQ queues
  workspace:       # Source code (macOS only)
```

#### 3.7.2 Host-Container Synchronization on macOS

**Problem**: Vendor directory contains thousands of files, slow synchronization

**Solution**: Composer operations run inside the container and then sync to host
(`composerMirroringVendor` in `internal/cli/php.go`; copies through the Docker API,
`adapters/dockerd/copy.go`):

```
# Flow in hm composer install|update|require|remove (Mac)
1. Restart phpfpm
2. Copy the host's vendor into the container
3. Run Composer inside
4. Copy the whole workdir back out over the host's
```

### 3.8 Project Isolation

**Mechanism**: `COMPOSE_PROJECT_NAME` variable

```bash
# If COMPOSE_PROJECT_NAME="myproject"
# Containers are named: myproject_phpfpm_1, myproject_db_1, etc.
# Networks are named: myproject_default
# Volumes are named: myproject_dbdata

# Allows multiple simultaneous projects without conflicts
```

**Configuration**:
- Defined in project's `config/docker/properties.json`
- Derived from project name during `hm setup`
