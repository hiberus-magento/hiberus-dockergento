# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hiberus Dockergento is a CLI tool (`hm`) for creating and managing Docker-based Magento 2 development environments. The project provides pre-configured Docker images for Magento 2.3.x - 2.4.9 with the matching PHP, MariaDB, search engine (Elasticsearch/OpenSearch), Redis/Valkey and other service versions.

**Core Concept**: This project is a command router that translates simplified Magento 2-specific commands into operations on the dockerized services. For example, `hm magento cache:clean` runs the Magento CLI inside the PHP container through Compose, which the binary uses as a library (`dockergento/adapters/composelib`), and the Docker API (`dockergento/adapters/dockerd`).

**Implementation Language**: The `cmd/hm` Go binary is the entry point; `internal/cli/run.go` answers every invocation: the 66 commands, the help (`hm`, `--help`, `<cmd> --help`), `--version`, and a project's own scripts under `config/hm/commands` (`internal/cli/project_command.go`). There is no Bash entry point any more: `bin/run` and the `console/` tree are gone, and the shell completion is printed by the binary itself (`hm completion bash|zsh`, `internal/cli/completion.go`) — `MIGRATION.md` keeps the history.

## Architecture Documentation

**Detailed architecture documentation is available in [ARCHITECTURE.md](ARCHITECTURE.md)** which serves as an index to segmented documentation files.

### When to Reference Architecture Docs

**DON'T load architecture docs for**:
- Simple command usage questions
- Quick fixes or small changes
- Standard development tasks already documented below

**DO reference architecture docs when**:
- Adding new commands or major features
- Modifying core routing or orchestration logic
- Understanding complex subsystems (state management, configuration hierarchy, etc.)
- Debugging command flow or error handling
- Making platform-specific changes (macOS vs Linux)

### Architecture Quick Reference

| Topic | File | Use Case |
|-------|------|----------|
| Why this tool exists, what problems it solves | `architecture/01-overview-and-objectives.md` | Understanding design decisions |
| Command routing, validation, execution flow | `architecture/02-cli-architecture.md` | Adding commands, modifying router |
| Docker/Docker Compose interaction | `architecture/03-docker-orchestration.md` | Container management, volumes |
| Command-to-container translation | `architecture/04-command-translation.md` | Understanding how commands map |
| Configuration loading and state | `architecture/05-state-and-configuration.md` | Properties, environment variables |
| Error handling, I/O patterns | `architecture/06-io-and-error-handling.md` | Adding validations, output |

**Tip**: Use specific architecture files only when needed. Don't read entire architecture for simple tasks.

## Quick Architecture Overview

**For detailed architecture, see [ARCHITECTURE.md](ARCHITECTURE.md) and its segmented documentation.**

### Key Components

- **Entry point**: `cmd/hm` - the Go binary (`go build -o bin/hm ./cmd/hm`); `bin/hm` is not versioned
- **Router**: `internal/cli/run.go` - one `case` per command; the rest of `internal/cli` parses arguments and renders output (text or JSON envelope)
- **Engine seam**: `internal/cli/engine.go` - the `commands` interface the handlers call, built by `newEngine`; tests swap in a fake
- **Use cases**: `dockergento/app` - what each command does, written against ports
- **Domain rules**: `dockergento/core` - pure functions and types (compose rendering, properties, refusals)
- **Ports and adapters**: `dockergento/ports` (interfaces) and `dockergento/adapters/*`, wired in `dockergento/dockergento.go` (the `Engine` facade): `composelib` (Compose as a library), `dockerd` (Docker API), `composecli` (the `hm docker-compose` passthrough, the only Compose subprocess), `composecfg`, `fsprops` (properties), `gitvcs`, `registry` (worktree environments, SQLite in `~/.hm/hm.db`), `selfrun`, `machine`, `toolinfo`, `traefik` (the proxy), `hmstate`, `magentofiles`, `osfs`
- **Contract**: `dockergento/contract` - the JSON envelope and the exit codes, shared by every door
- **Other doors**: `internal/cli/tui*.go` (dashboard), `internal/cli/mcp*.go` (MCP server), `internal/api` (HTTP, behind `hm web`)
- **Governance**: `.specify/memory/constitution.md` - the principles every change follows
- **Project commands**: `internal/cli/project_command.go` - runs a project's own `config/hm/commands/<name>.sh` for any name the router does not know
- **Data**: `data/*.json` - Configuration (requirements, properties, command descriptions)
- **Templates**: `docker-compose/*.yml` - Docker Compose templates
- **Embedded assets**: `assets.go` (package `hm`, module root) compiles `data/`, `docker-compose/` and `skills/` into the binary with `go:embed`; `hm.Assets()` is that `fs.FS`, or `os.DirFS($HM_ASSETS_DIR)` when the contributor override is set. It reaches readers through `dockergento.Options.Assets` (and `toolAssets` in `internal/cli`); nothing reads tool files from beside the executable, so tests inject an `fstest.MapFS`. A new data file must be added to the `//go:embed` list

### Command Flow (Simplified)

```
hm <command> [args]
  → cmd/hm → internal/cli.Run (global flags, then the switch in run.go)
  → handler in internal/cli (parses its options)
  → engine (dockergento.Engine) → use case in dockergento/app
  → ports → adapters: Compose library / Docker API / filesystem / git
  → anything the switch does not know: the project's config/hm/commands/<name>.sh, or command_not_found
```

### Key Files for Common Tasks

- **Add command**: an entry in `data/command_descriptions.json` (help, groups and `hm completion` read it, so the completion follows with a rebuild), a `case` in `internal/cli/run.go`, its handler in `internal/cli/<command>.go` with `_test.go` against the fake engine (`internal/cli/fake_engine_test.go`), the use case in `dockergento/app`, a method on the `commands` interface (`internal/cli/engine.go`) when it needs the engine, `docs/<command>.md`, and its row in `MIGRATION.md`'s command table (`tests/unit/migration_status_test.sh` checks it)
- **Add a container operation**: a port method in `dockergento/ports`, implemented in `composelib` or `dockerd` — not a `docker` subprocess
- **Modify service versions**: Edit `data/requirements.json`
- **Change default properties**: Edit `data/properties.json`
- **Modify Docker config**: Edit `docker-compose/docker-compose.template.yml`

## Development Commands

### Building and testing hm

```bash
go build -o bin/hm ./cmd/hm                              # the binary
DOCKER_HOST=unix:///nonexistent go test ./...            # Go tests; never against the real daemon
DOCKER_HOST=unix:///nonexistent ./tests/run.sh unit      # the fast shell suites (never tests/unit)
./tests/run.sh tests/integration/<suite>_test.sh         # one shell suite (many need Docker)
gofmt -l ./cmd ./internal ./dockergento && go vet ./...  # what CI checks besides the tests
```

- Tests go through the public interface; handlers are tested through `Run` with the fake engine, and stable output is pinned with goldens under `testdata/`.
- `test/e2e` builds the binary and uses whatever Docker it finds, which is why `DOCKER_HOST` is cut for every broad run.
- Commits: English, Conventional Commits, no AI attribution (no `Co-Authored-By`). Push, PR and release are a human decision. See `ROADMAP.md` for the rest of the rules.

### Using hm: environment setup

```bash
# Create environment for existing project
hm setup

# Create environment with options
hm setup -p=project-name --domain=project.local -D=/path/to/dump.sql

# Force regenerate docker-compose files
hm setup -f

# Create new Magento project
hm create-project
```

### Container Management

```bash
# Start all containers
hm start

# Start and stop all other containers first
hm start -s

# Stop containers
hm stop

# Restart specific service
hm restart phpfpm

# Rebuild containers
hm rebuild

# Remove containers and volumes
hm down -v
```

### Magento Commands

```bash
# Execute Magento CLI
hm magento cache:clean
hm magento setup:upgrade

# Run composer
hm composer install
hm composer require vendor/package

# Access PHP container
hm bash
hm bash -r  # as root

# Execute command in container
hm exec ls -lah
hm exec -r some-command  # as root
```

### Database Operations

```bash
# Import database
hm mysql -i /path/to/dump.sql

# Import with DEFINER cleanup
hm mysql -d -i /path/to/dump.sql

# Import with anonymization
hm mysql -a -i /path/to/dump.sql

# Execute query
hm mysql -q "SELECT * FROM core_config_data"

# Export database
hm mysqldump /path/to/output.sql

# Transfer from remote (interactive)
hm transfer-db
```

### Magento tests

```bash
# Run unit tests
hm test-unit

# Run integration tests
hm test-integration

# Run specific test
hm test-unit path/to/TestFile.php
```

### Debugging

```bash
# Enable xDebug
hm debug-on

# Disable xDebug
hm debug-off
```

### Other Utilities

```bash
# Clear generated code
hm purge

# Anonymize database
hm masquerade

# Configure Grunt and compile theme
hm grunt Vendor/theme locale_LOCALE

# Transfer media files
hm transfer-media

# Check Magento version compatibility
hm compatibility

# Update hm tool
hm update
```

## Platform-Specific Behavior

### Mac

- Uses `docker-compose.dev.mac.yml` overlay with delegated volume mounts for performance
- Composer operations (`install`, `update`, `require`, `remove`) copy vendor to container, run inside, then sync back to host
- Commands `copy-to-container` and `copy-from-container` available for manual syncing

### Linux

- Uses `docker-compose.dev.linux.yml` overlay with bind mounts
- Automatically fixes permissions after container start (`dockergento/app/after_start.go`)
- Sets `/etc/hosts` entries for self-routing domains

## Service Stack

Docker services defined in `docker-compose.template.yml`:

- **phpfpm** - PHP-FPM (7.2, 7.3, 7.4, 8.1, 8.2, 8.3, 8.4, 8.5)
- **nginx** - Nginx (1.18, 1.28, 1.30)
- **db** - MariaDB (10.2, 10.3, 10.4, 10.6, 11.4, 11.8, 12.3)
- **search** - Elasticsearch (5.6, 6.5, 7.17) or OpenSearch (1.2, 2.5, 3)
- **redis** - Redis (5.0, 6.2, 7.0) or Valkey 9
- **varnish** - Varnish (6.0, 7.1)
- **hitch** - TLS termination proxy
- **mail catcher** - Mailhog or Mailpit (`hm setup --mail=`), web UI on port 8025
- **rabbitmq** - Message queue (ports 5672, 15672)

`data/requirements.json` is the authoritative version matrix; the README lists the images.

## Important Notes

- All containers use images from `hiberusmagento/` on Docker Hub
- Database credentials: root password is "password", magento user/pass is "magento"/"magento"
- RabbitMQ credentials: user/password
- Magento root directory defaults to `.` but can be configured in properties
- The tool manages `COMPOSE_PROJECT_NAME` for project isolation
- Commands that modify docker-compose configuration require containers to be recreated
