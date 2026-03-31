## 3. Container Orchestration

### 3.1 Interaction with Docker and Docker Compose

The tool **does not directly manage the Docker daemon**, but acts as an **abstraction layer over Docker Compose**. All interactions with containers are performed through system `docker-compose` and `docker` commands.

### 3.2 Central Variable: `DOCKER_COMPOSE`

The core of orchestration is the `DOCKER_COMPOSE` variable, built dynamically:

```bash
# Configuration in bin/run
DOCKER_COMPOSE="docker-compose -f docker-compose.yml -f docker-compose.dev.mac.yml"
# or
DOCKER_COMPOSE="docker-compose -f docker-compose.yml -f docker-compose.dev.linux.yml"
```

**Usage:**
```bash
# All commands use this variable
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

**Generation process** (`console/tasks/write_from_docker-compose_templates.sh`):

```bash
1. Read REQUIREMENTS (JSON with service versions)
2. Build substitution regex:
   s/<php_version>/8.2-buster/g
   s/<mariadb_version>/10.6/g
   s/<search_version>/2.5-opensearch/g
3. Apply sed on template → docker-compose.yml
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

**Automatic process** (`console/tasks/set_machine_specific_properties.sh`):

```bash
uname -s
  ↓
"Linux" → MACHINE="linux", DOCKER_COMPOSE_FILE_MACHINE="docker-compose.dev.linux.yml"
  ↓
"Darwin" → MACHINE="mac", DOCKER_COMPOSE_FILE_MACHINE="docker-compose.dev.mac.yml"
  ↓
Other → ERROR: "Unsupported system type"
```

Exports variables:
- `MACHINE`: "mac" or "linux"
- `DOCKER_COMPOSE_FILE_MACHINE`: Path to the correct overlay

### 3.5 Docker State Validations

#### 3.5.1 Docker Daemon Validation

**Function**: `is_docker_service_running()` in `console/helpers/docker.sh`

```bash
docker info >/dev/null 2>&1
  ↓
Exit code 0 → Docker running, continue
  ↓
Exit code ≠ 0 → "Docker is not running!", exit 1
```

**Executed by**: All commands except `setup`, `create-project`, `compatibility`, `update`

#### 3.5.2 Docker Compose Configuration Validation

**Function**: `validate_docker_compose.sh`

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

**Function**: `is_run_service()` in `console/helpers/docker.sh`

```bash
# Example: is_run_service "db"
docker ps -qf name="$COMPOSE_PROJECT_NAME"-"$service" -qf name="$COMPOSE_PROJECT_NAME"_"$service"
  ↓
Container ID returned → Service running
  ↓
Empty → "Error: $service service is not running!", exit 1
```

**Used by**: Commands that require specific services (mysql, masquerade, transfer-db)

### 3.6 Container Interaction Patterns

#### 3.6.1 Command Execution in Containers

**Basic pattern**:
```bash
$DOCKER_COMPOSE exec [options] <service> <command>
```

**Examples:**
```bash
# Execute as app user (default)
$DOCKER_COMPOSE exec phpfpm php bin/magento cache:clean

# Execute as root
$DOCKER_COMPOSE exec -u root phpfpm chown -R app:app /var/www/html

# Execute without TTY (automated scripts)
$DOCKER_COMPOSE exec -T phpfpm composer install
```

#### 3.6.2 Lifecycle Management

**Service startup** (`hm start`):
```bash
$DOCKER_COMPOSE up -d [service]
  ↓
If Linux: 
  - sleep 5
  - fix_linux_permissions.sh
  - set_etc_hosts.sh
```

**Service stop** (`hm stop`):
```bash
$DOCKER_COMPOSE stop [service]
```

**Restart** (`hm restart`):
```bash
hm stop [service]
hm start [service]
```

**Rebuild** (`hm rebuild`):
```bash
$DOCKER_COMPOSE up --build -d [service]
```

**Destruction** (`hm down`):
```bash
$DOCKER_COMPOSE down [options]
# Example: hm down -v (removes volumes)
```

#### 3.6.3 Automatic Service Startup

**Function**: `start_service_if_not_running.sh`

```bash
# Try to execute command in service
$DOCKER_COMPOSE exec -T "$service" sh -c "echo 'check $service service is running'" &>/dev/null
  ↓
Exit code 1 (not running) → hm start
  ↓
Exit code 0 (running) → Continue
```

Used by commands that assume services are running (e.g., `setup`, `install`)

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

```bash
# Flow in hm composer install (Mac)
1. hm restart phpfpm
2. copy-to-container vendor  # Copy existing vendor to container
3. $DOCKER_COMPOSE exec phpfpm composer install  # Execute inside
4. Stop phpfpm
5. docker cp <container_id>:/var/www/html/. <host_path>  # Copy everything back
6. hm start phpfpm
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
