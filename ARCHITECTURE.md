# System Design Document (SDD)
# Hiberus Dockergento

> **Navigation**: This document serves as an index to the detailed architecture documentation. Each section is maintained in a separate file for better maintainability and focused reading.

---

## Document Structure

### [1. Overview and Objectives](architecture/01-overview-and-objectives.md)
**What the Hiberus Dockergento tool solves**

- Tool purpose and core concept
- Problems it addresses (Docker complexity, version management, platform differences, etc.)
- Scope: supported Magento versions, operating systems, and services

**When to read**: Start here to understand the "why" behind the tool's design decisions.

---

### [2. CLI Architecture](architecture/02-cli-architecture.md)
**Command Router design pattern and execution flow**

- Command Router pattern implementation
- Execution flow from user input to command processing
- Core components: Entry point, directory structure, property loading
- Command validation and routing mechanisms
- Help system and metadata management

**When to read**: Understanding how commands are captured, validated, and routed to their handlers.

---

### [3. Docker Orchestration](architecture/03-docker-orchestration.md)
**How the CLI interacts with Docker daemon and Docker Compose**

- Interaction with Docker and Docker Compose
- Docker Compose file generation from templates
- OS-specific overlays (macOS vs Linux strategies)
- State validation (daemon, configuration, services)
- Container lifecycle management patterns
- Volume management and project isolation

**When to read**: Understanding how the tool manages containers, volumes, and Docker Compose configurations.

---

### [4. Command Translation Layer](architecture/04-command-translation.md)
**How aliases (e.g., `hm magento`, `hm exec`) map to specific containers**

- Translation principles and patterns
- Command-to-container mapping
- Variable injection strategies
- Option and argument handling
- Complex command orchestration examples
- Complete command reference table

**When to read**: Understanding how user-friendly commands are translated into Docker operations.

---

### [5. State and Configuration Management](architecture/05-state-and-configuration.md)
**Where and how the tool reads environment variables and project configuration files**

- Configuration hierarchy and layers
- Configuration files (global and project-specific)
- Property loading and merging process
- System state variables
- Dynamic Magento configuration
- Tool-specific configuration (xDebug, SSL, Masquerade)

**When to read**: Understanding how configuration is loaded, merged, and persisted across sessions.

---

### [6. I/O and Error Handling](architecture/06-io-and-error-handling.md)
**Standards for output and OS exception handling**

- Fail-fast philosophy with `set -euo pipefail`
- Structured output system with semantic functions
- User input handling (interactive and stdin)
- Validation patterns (Docker, services, arguments, files)
- Output redirection and suppression
- Exit codes and best practices

**When to read**: Understanding error handling strategies and I/O patterns used throughout the codebase.

---

## Quick Reference

### By Task

- **Adding a new command**: Read sections 2, 4
- **Modifying Docker configuration**: Read sections 3, 5
- **Understanding configuration precedence**: Read section 5
- **Debugging command flow**: Read sections 2, 4, 6
- **Platform-specific behavior**: Read sections 3, 5
- **Error handling patterns**: Read section 6

### By Component

- **`bin/run`**: Section 2.3.1
- **`console/commands/`**: Sections 2.3.2, 4
- **`console/tasks/`**: Sections 2.3.2, 5
- **`console/helpers/`**: Sections 2.3.2, 6
- **`data/properties.json`**: Section 5.2.1
- **`docker-compose` templates**: Section 3.3

---

## Maintenance Notes

- Each section is independently maintained in `architecture/` directory
- Code examples are extracted directly from the codebase
- Keep examples synchronized when refactoring code
- Update this index when adding or restructuring sections

---

## Related Documentation

- [CLAUDE.md](CLAUDE.md) - Guide for Claude Code when working with this repository
- [README.md](README.md) - User-facing documentation and installation guide
- [docs/](docs/) - Individual command documentation
