# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hiberus Dockergento is a CLI tool (`hm`) for creating and managing Docker-based Magento 2 development environments. The project provides pre-configured Docker images for various Magento versions (2.3.x - 2.4.7) with appropriate PHP, MariaDB, search engine (Elasticsearch/OpenSearch), Redis, and other service versions.

**Core Concept**: This project is a command router that translates simplified Magento 2-specific commands into Docker commands that interact with various dockerized services. For example, `hm magento cache:clean` is translated into the appropriate `docker-compose exec` command to run Magento CLI inside the PHP container.

**Implementation Language**: The entire internal implementation is written in Bash scripts. All commands, tasks, helpers, and components are `.sh` shell scripts.

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

- **Entry point**: `bin/run` - Main executable, command router
- **Commands**: `console/commands/*.sh` - Individual command implementations
- **Tasks**: `console/tasks/*.sh` - Reusable functions shared across commands
- **Helpers**: `console/helpers/*.sh` - Utility functions (docker validation, properties)
- **Components**: `console/components/*.sh` - UI functions (print, input)
- **Data**: `data/*.json` - Configuration (requirements, properties, descriptions)
- **Templates**: `docker-compose/*.yml` - Docker Compose templates

### Command Flow (Simplified)

```
hm <command> [args]
  → bin/run (routes command)
  → Load properties (data/ + config/docker/)
  → Validate Docker
  → Execute console/commands/<command>.sh
  → Translate to docker-compose exec or docker commands
```

### Key Files for Common Tasks

- **Add command**: Create `console/commands/mycommand.sh` + entry in `data/command_descriptions.json`
- **Modify service versions**: Edit `data/requirements.json`
- **Change default properties**: Edit `data/properties.json`
- **Modify Docker config**: Edit `docker-compose/docker-compose.template.yml`

## Development Commands

### Environment Setup

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

### Testing

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
- Automatically fixes permissions after container start (`console/tasks/fix_linux_permissions.sh`)
- Sets `/etc/hosts` entries for self-routing domains

## Service Stack

Docker services defined in `docker-compose.template.yml`:

- **phpfpm** - PHP-FPM (versions: 7.2, 7.3, 7.4, 8.1, 8.2, 8.3)
- **nginx** - Nginx 1.18
- **db** - MariaDB (versions: 10.2, 10.3, 10.4, 10.6)
- **search** - Elasticsearch (5.6, 6.5, 7.17) or OpenSearch (1.2, 2.5, 2.12)
- **redis** - Redis (versions: 5.0, 6.2, 7.0, 7.2)
- **varnish** - Varnish (versions: 6.0, 7.1)
- **hitch** - TLS termination proxy
- **mailhog** - Email testing (port 8025)
- **rabbitmq** - Message queue (ports 5672, 15672)

## Important Notes

- All containers use images from `hiberusmagento/` on Docker Hub
- Database credentials: root password is "password", magento user/pass is "magento"/"magento"
- RabbitMQ credentials: user/password
- Magento root directory defaults to `.` but can be configured in properties
- The tool manages `COMPOSE_PROJECT_NAME` for project isolation
- Commands that modify docker-compose configuration require containers to be recreated
