## 4. Command Translation Layer

### 4.1 Translation Principle

Each `hm` command is a **wrapper** that translates a high-level operation into one or more low-level Docker/Docker Compose commands. The translation includes:

1. Target container selection
2. Appropriate Docker command construction
3. Environment variable and path injection
4. Context-specific option handling

### 4.2 Translation Patterns by Category

#### 4.2.1 Direct Execution Commands

**Pattern**: `hm <command>` → `$DOCKER_COMPOSE exec <container> <command>`

| User Command | Container | Real Translation |
|----------------|-----------|-----------------|
| `hm magento cache:clean` | phpfpm | `$DOCKER_COMPOSE exec phpfpm php ./bin/magento cache:clean` |
| `hm composer install` | phpfpm | `$DOCKER_COMPOSE exec phpfpm composer install` |
| `hm npm install` | phpfpm | `$DOCKER_COMPOSE exec phpfpm npm install` |
| `hm n98-magerun sys:info` | phpfpm | `docker-compose exec phpfpm bash -c "n98-magerun sys:info"` |

**Typical implementation**:
```bash
# console/commands/magento.sh
"$COMMANDS_DIR"/exec.sh php ./bin/magento "$@"

# console/commands/composer.sh
"$COMMANDS_DIR"/exec.sh composer "$@"

# console/commands/npm.sh
"$COMMANDS_DIR"/exec.sh npm "$@"
```

#### 4.2.2 Commands with Specific Container

**Base**: `hm exec` is the fundamental translator

```bash
# console/commands/exec.sh
docker_compose_exec="$DOCKER_COMPOSE exec"

# Process options
if [[ "$1" == "-r" ]]; then
    exec_options="$exec_options -u root"
    shift
fi

# Execute in phpfpm by default
$docker_compose_exec phpfpm "$@"
```

**Command to Container Mapping**:

| Command | Target Container | Justification |
|---------|-------------------|---------------|
| `hm bash`, `hm exec` | **phpfpm** | Contains PHP, Composer, Magento CLI tools |
| `hm mysql`, `hm mysqldump` | **db** | Direct access to MariaDB server |
| `hm magento`, `hm composer`, `hm npm` | **phpfpm** | Require PHP interpreter and Magento filesystem |
| `hm grunt` | **phpfpm** | Node.js installed in PHP image for asset compilation |

#### 4.2.3 Database Commands

**MySQL Shell** (`hm mysql`):

```bash
# Interactive input
hm mysql
  ↓
$DOCKER_COMPOSE exec db bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""

# Import from file
hm mysql -i dump.sql
  ↓
docker exec -i $mysql_container bash -c "mysql ..." < dump.sql

# Import from stdin
cat dump.sql | hm mysql
  ↓
docker exec -i $mysql_container bash -c "mysql ..."

# Execute query
hm mysql -q "SELECT * FROM core_config_data"
  ↓
docker exec -e QUERY="..." $mysql_container bash -c 'mysql ... -e "$QUERY"'
```

**MySQL Dump** (`hm mysqldump`):

```bash
hm mysqldump backup.sql
  ↓
docker-compose exec db bash -c 'mysqldump --skip-triggers -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' > backup.sql
```

**Used environment variables**:
- `$MYSQL_ROOT_PASSWORD`: Root password (defined in docker-compose: "password")
- `$MYSQL_DATABASE`: Database name (defined in docker-compose: "magento")

#### 4.2.4 Complex Commands with Multiple Translations

**Example: `hm grunt`**

A single invocation generates multiple Docker commands:

```bash
hm grunt Vendor/theme es_ES
  ↓
# 1. Copy configuration files
docker-compose exec phpfpm bash -c "cp package.json.sample package.json"
docker-compose exec phpfpm bash -c "cp grunt-config.json.sample grunt-config.json"
docker-compose exec phpfpm bash -c "cp Gruntfile.js.sample Gruntfile.js"

# 2. Install dependencies
docker-compose exec phpfpm bash -c "npm install && npm update"

# 3. Configure theme
docker-compose exec phpfpm bash -c "echo 'module.exports = {...}' > dev/tools/grunt/configs/local-themes.js"

# 4. Compile
docker-compose exec phpfpm bash -c "grunt exec:magento && grunt watch:magento"
```

#### 4.2.5 Remote Transfer Commands

**Example: `hm transfer-db`**

Orchestrates SSH tunneling, remote mysqldump, and local mysql:

```bash
hm transfer-db --ssh-host=cloud.server --sql-host=database.internal
  ↓
# 1. Remote dump through SSH tunnel inside db container
docker-compose exec db bash -c "mysqldump -h'$sql_host' -u'$sql_user' ... | gzip > /tmp/db.sql.gz"

# 2. Local import from temporary file
docker-compose exec db bash -c "zcat /tmp/db.sql.gz | mysql -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE"

# 3. Reindex Magento
docker-compose exec phpfpm bin/magento indexer:reindex

# 4. Clean cache
docker-compose exec phpfpm bin/magento cache:flush
```

### 4.3 Variable Injection Strategies

#### 4.3.1 Host Environment Variables → Container

```bash
# Option 1: -e flag
docker exec -e QUERY="SELECT * FROM admin_user" $container bash -c 'mysql -e "$QUERY"'

# Option 2: Variables defined in docker-compose.yml
services:
  phpfpm:
    environment:
      PHP_IDE_CONFIG: serverName=localhost
      COMPOSER_VERSION: <composer_version>
```

#### 4.3.2 Container Environment Variables

Access to variables defined in `docker-compose.yml`:

```bash
# Use \$ syntax to evaluate inside container
docker-compose exec db bash -c "mysql -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE"
```

Available variables in containers:
- **db**: `$MYSQL_ROOT_PASSWORD`, `$MYSQL_DATABASE`, `$MYSQL_USER`, `$MYSQL_PASSWORD`
- **phpfpm**: `$COMPOSER_VERSION`, `$PHP_IDE_CONFIG`
- **rabbitmq**: `$RABBITMQ_DEFAULT_USER`, `$RABBITMQ_DEFAULT_PASS`

#### 4.3.3 Dynamic Paths

```bash
# Using $WORKDIR_PHP (default: /var/www/html)
"$COMMANDS_DIR"/exec.sh php $WORKDIR_PHP/bin/magento cache:clean

# Using $BIN_DIR (default: ./vendor/bin)
"$COMMANDS_DIR"/exec.sh $BIN_DIR/phpunit --config ./dev/tests/unit/phpunit.xml.dist
```

### 4.4 Option and Argument Handling

#### 4.4.1 Long to Short Option Translation

**Function**: `map_arguments.sh`

```bash
# data/command_descriptions.json defines the mapping
{
  "setup": {
    "opts": [
      {"name": {"short": "p", "long": "project-name"}},
      {"name": {"short": "d", "long": "domain"}},
      {"name": {"short": "i", "long": "install"}}
    ]
  }
}

# User executes:
hm setup --project-name=myproject --install

# Translates to:
hm setup -p myproject -i
```

#### 4.4.2 Argument Propagation

Using `"$@"` to propagate all arguments to the target command:

```bash
# User executes:
hm magento setup:upgrade --keep-generated

# In console/commands/magento.sh:
"$COMMANDS_DIR"/exec.sh php ./bin/magento "$@"
  ↓
# Propagates setup:upgrade --keep-generated intact to container
```

#### 4.4.3 User Control Options

**Execution as Root**:

```bash
# User executes:
hm bash -r

# Translates to:
$DOCKER_COMPOSE exec -u root phpfpm bash

# User executes:
hm exec -r chown -R app:app /var/www/html

# Translates to:
$DOCKER_COMPOSE exec -u root phpfpm chown -R app:app /var/www/html
```

### 4.5 Special Translation Cases

#### 4.5.1 Commands that Don't Use Docker

Some commands operate directly on the host:

| Command | Operation | Reason |
|---------|----------|-------|
| `hm setup` | Generates docker-compose.yml | Needs to create files before containers |
| `hm create-project` | Clones repository and generates configs | Initialization prior to containers |
| `hm update` | `git pull` in `~/hm/` | Updates the CLI tool itself |
| `hm compatibility` | Reads `data/requirements.json` | Queries local metadata |
| `hm ssl` | Generates certificates with mkcert on host | Requires access to host keychain |
| `hm transfer-media` | Direct host-to-host rsync | More efficient than through container |

#### 4.5.2 Commands with Conditional Logic

**`hm composer` on macOS**:

```bash
if [[ "$1" == "install" || "$1" == "update" || "$1" == "require" || "$1" == "remove" ]]; then
    # Special Mac flow: copy vendor to container, execute, sync back
    "$COMMANDS_DIR"/restart.sh "phpfpm"
    copy_vendor_to_container
    "$COMMANDS_DIR"/exec.sh composer "$@"
    sync_all_from_container_to_host
else
    # Other composer commands: execute directly
    "$COMMANDS_DIR"/exec.sh composer "$@"
fi
```

#### 4.5.3 Command Composition

Commands that internally call other `hm` commands:

```bash
# hm restart = hm stop + hm start
"$COMMANDS_DIR"/stop.sh "$@"
"$COMMANDS_DIR"/start.sh "$@"

# hm purge
hm magento cache:clean
rm -rf generated/ pub/static/* var/cache/* var/page_cache/* var/view_preprocessed/*

# hm install (inside magento_installation.sh)
hm composer install
hm magento setup:install
hm magento setup:upgrade
hm magento deploy:mode:set developer
```

### 4.6 Reference Table: Complete Command Mapping

| User Command | Container | Resulting Docker Command | Complexity |
|----------------|-----------|---------------------------|-------------|
| `hm bash` | phpfpm | `$DOCKER_COMPOSE exec phpfpm bash` | Simple |
| `hm magento <cmd>` | phpfpm | `$DOCKER_COMPOSE exec phpfpm php ./bin/magento <cmd>` | Simple |
| `hm composer <cmd>` | phpfpm | `$DOCKER_COMPOSE exec phpfpm composer <cmd>` | Conditional (Mac) |
| `hm mysql` | db | `$DOCKER_COMPOSE exec db mysql -uroot -ppassword magento` | Simple |
| `hm mysql -i dump.sql` | db | `docker exec -i <db_container> mysql ... < dump.sql` | Medium |
| `hm grunt <theme> <locale>` | phpfpm | 4 sequential docker-compose exec commands | High |
| `hm transfer-db` | db | Multiple exec for SSH tunnel, dump, import | High |
| `hm setup` | - | Host operations + docker-compose generation | High |
| `hm start` | - | `$DOCKER_COMPOSE up -d` + post-start tasks | Medium |
| `hm exec <cmd>` | phpfpm | `$DOCKER_COMPOSE exec phpfpm <cmd>` | Simple |
| `hm exec -r <cmd>` | phpfpm | `$DOCKER_COMPOSE exec -u root phpfpm <cmd>` | Simple |
