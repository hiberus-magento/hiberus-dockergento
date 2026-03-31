# 1. Overview and Objectives

## 1.1 Tool Purpose

Hiberus Dockergento is a CLI tool that **simplifies and abstracts the complexity of working with Docker environments for Magento 2 development**. It acts as a command router that translates common Magento development operations into appropriate Docker/Docker Compose commands.

## 1.2 Problems It Solves

1. **Docker Complexity**: Eliminates the need for developers to deeply understand Docker and Docker Compose commands. Instead of executing `docker-compose exec phpfpm php bin/magento cache:clean`, the developer simply runs `hm magento cache:clean`.

2. **Version Management and Compatibility**: Automatically configures the correct versions of services (PHP, MariaDB, Elasticsearch/OpenSearch, Redis, Varnish) based on the specified Magento version, following official compatibility specifications.

3. **Platform Differences**: Automatically handles differences between macOS and Linux, applying platform-specific performance configurations (such as delegated volumes on Mac) and file permissions (on Linux).

4. **Simplified Complex Operations**: Encapsulates complex tasks such as:
   - Database imports with DEFINER cleanup
   - Media and database transfers from remote servers
   - Vendor synchronization between host and container on macOS
   - Automatic SSL and local hosts configuration
   - Data anonymization with Masquerade

5. **Environment Consistency**: Ensures all developers work with the same container configuration, regardless of their operating system or local setup.

6. **Optimized Workflow**: Provides specialized commands for the complete Magento development cycle: from project creation (`hm create-project`), through development (`hm composer`, `hm magento`), to testing (`hm test-unit`, `hm test-integration`).

## 1.3 Scope

The tool supports:
- **Magento Versions**: 2.3.0 to 2.4.7 (and their patched versions)
- **Operating Systems**: macOS and Linux
- **Dockerized Services**: PHP (7.2-8.3), MariaDB (10.2-10.6), Elasticsearch (5.6-7.17), OpenSearch (1.2-2.12), Redis (5.0-7.2), Varnish (6.0-7.1), RabbitMQ 3.9, Mailhog, Hitch
- **Operations**: Container management, Magento installation, database management, xDebug debugging, asset compilation, test execution
