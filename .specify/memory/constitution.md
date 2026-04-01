# Hiberus Dockergento Constitution

<!--
Sync Impact Report - Constitution Initialization
================================================
Version: 1.0.0 (Initial ratification)
Date: 2026-04-01

Changes:
- Initial constitution created from template
- Established 7 core principles for Bash-based CLI development
- Defined governance and compliance requirements

Modified Principles: N/A (initial version)
Added Sections:
  - Core Principles (7 principles)
  - Technical Constraints
  - Quality Standards
  - Governance

Removed Sections: N/A (initial version)

Template Sync Status:
  ✅ .specify/templates/plan-template.md - verified alignment
  ✅ .specify/templates/spec-template.md - verified alignment
  ✅ .specify/templates/tasks-template.md - verified alignment
  ✅ .specify/templates/checklist-template.md - verified alignment

Follow-up TODOs: None
-->

## Core Principles

### I. Bash Implementation Consistency

All internal implementation MUST be written in Bash shell scripts. This principle ensures:
- Command implementations in `console/commands/*.sh` use Bash
- Task modules in `console/tasks/*.sh` use Bash
- Helper utilities in `console/helpers/*.sh` use Bash
- No mixing of languages within the core tool (external tools like Docker, jq, etc. are acceptable dependencies)

**Rationale**: Maintaining a single implementation language reduces cognitive overhead, ensures consistent error handling, and simplifies maintenance. Bash is the natural choice for a CLI tool that orchestrates Docker commands and system operations.

**How to apply**: When adding new features or commands, always implement using Bash scripts following existing patterns. Reject proposals to introduce Python, Node.js, or other scripting languages for core functionality.

### II. Command Router Architecture

All user-facing commands MUST follow the Command Router pattern:
- Entry point at `bin/run` routes all commands
- Each command is a standalone script in `console/commands/`
- Commands are registered via metadata in `data/command_descriptions.json`
- Validation and help text generation is centralized

**Rationale**: The Command Router pattern provides a clean separation of concerns, makes the command structure discoverable, and enables consistent validation and help generation across all commands.

**How to apply**: New commands MUST be added as separate files in `console/commands/` with corresponding metadata. Never bypass the router or create alternative entry points.

### III. Docker Abstraction Priority

Commands MUST abstract Docker complexity from users:
- User-facing commands should be intuitive and Magento-focused (e.g., `hm magento cache:clean`)
- Docker/Docker Compose commands are implementation details
- Platform differences (macOS vs Linux) must be transparent to users
- Service configuration should be automatic based on Magento version

**Rationale**: The core value proposition is eliminating Docker complexity. Users should think about Magento development tasks, not container orchestration.

**How to apply**: When designing new commands, prioritize user intent over technical implementation. If a command requires users to understand Docker concepts, redesign it.

### IV. Fail-Fast Error Handling (NON-NEGOTIABLE)

All Bash scripts MUST use `set -euo pipefail`:
- `set -e`: Exit immediately on any command failure
- `set -u`: Treat unset variables as errors
- `set -o pipefail`: Propagate pipeline failures

Additionally:
- Validate prerequisites before execution (Docker daemon, service status, file existence)
- Provide clear error messages via `console/components/print.sh` functions
- Never suppress errors silently

**Rationale**: Fail-fast prevents cascading failures and data corruption. In a tool managing development environments with databases and code, silent failures are unacceptable.

**How to apply**: Every script must begin with `set -euo pipefail`. Add validation checks before operations that could fail. Use semantic print functions (`print_error`, `print_warning`) for all error reporting.

### V. Platform-Specific Optimization

Platform differences MUST be handled transparently but optimally:
- macOS: Use delegated volume mounts for performance, handle vendor sync specially
- Linux: Use standard bind mounts, implement automatic permission fixes
- Detection and selection must be automatic
- Platform-specific code lives in separate overlay files (`.dev.mac.yml`, `.dev.linux.yml`)

**Rationale**: macOS and Linux have fundamentally different filesystem performance characteristics with Docker. Optimizing for each platform without breaking the user experience is essential.

**How to apply**: When adding features that interact with volumes or filesystem, always consider both platforms. Test on both or document platform-specific behavior.

### VI. Configuration Hierarchy Integrity

Configuration MUST follow a strict precedence hierarchy:
1. Runtime arguments (highest priority)
2. Project-specific configuration (`config/docker/*.properties`)
3. Global defaults (`data/properties.json`)

Additionally:
- Never override user-provided values
- Changes to configuration require explicit user action
- State persistence must be reliable and atomic

**Rationale**: Predictable configuration behavior builds user trust. Users must be confident their customizations won't be overwritten unexpectedly.

**How to apply**: When loading configuration, respect the hierarchy. Document which values can be overridden and when. Use property loading helpers consistently.

### VII. Backward Compatibility

Changes MUST maintain backward compatibility unless explicitly documented as breaking:
- Existing commands retain their behavior
- New options are additive, not destructive
- Configuration file formats remain compatible
- Deprecations are announced and alternatives provided

**Rationale**: This tool manages critical development environments. Breaking changes can disrupt entire teams. Stability is a feature.

**How to apply**: Before modifying existing command behavior, verify no existing usage patterns break. Add deprecation warnings before removing features. Version breaking changes clearly in release notes.

## Technical Constraints

### Technology Stack Requirements

The following external dependencies are REQUIRED and must be validated:
- Docker (daemon running)
- Docker Compose (v1 or v2 with automatic detection)
- Bash (version 4.0+)
- jq (for JSON parsing)
- git (for version management)

Optional but recommended:
- oh-my-zsh (for shell integration)
- Homebrew (for macOS dependency management)

### Service Version Compatibility

Service versions MUST align with official Magento compatibility specifications:
- Version matrices in `data/requirements.json` must be authoritative
- Automatic version selection based on detected Magento version
- Manual overrides allowed but validated against compatibility matrix

### Docker Compose Template System

Docker configuration MUST use the template-overlay pattern:
- Base template: `docker-compose.template.yml`
- Platform overlays: `docker-compose.dev.{mac,linux}.yml`
- Generated files: `docker-compose.yml` + overlay
- Templates use variable substitution from properties

## Quality Standards

### Code Quality Requirements

All code contributions MUST meet these standards:
- Follow existing code style and patterns
- Use semantic variable names (no single-letter variables except loop counters)
- Include comments for complex logic or non-obvious decisions
- Validate all user inputs before use
- Use helper functions from `console/helpers/` for common operations

### Testing Requirements

Changes MUST be validated through:
- Manual testing on target platforms (macOS and/or Linux)
- Verification with multiple Magento versions when applicable
- Testing both success and failure paths
- Documentation of test scenarios in PR descriptions

Integration testing focus areas:
- Command routing and validation
- Docker Compose generation and service startup
- Platform-specific overlays
- Database operations (import, export, transfer)
- Configuration loading and persistence

### Documentation Requirements

All user-facing changes MUST include:
- Updated command documentation in `docs/` directory
- Updated `README.md` if adding new features
- Help text updates in `data/command_descriptions.json`
- Architecture documentation updates if changing core patterns

## Governance

### Constitution Authority

This Constitution supersedes all other development practices and guidelines. When conflicts arise:
1. Constitution principles take precedence
2. Architecture documentation provides implementation guidance
3. Code comments and existing patterns serve as examples

### Amendment Process

Constitution amendments require:
1. Documented justification (what problem does this solve?)
2. Impact analysis on existing principles
3. Migration plan for affected code
4. Version bump following semantic versioning:
   - **MAJOR**: Removes or fundamentally redefines principles
   - **MINOR**: Adds new principles or materially expands guidance
   - **PATCH**: Clarifies wording, fixes typos, refines non-semantic details

### Compliance and Review

All contributions MUST be reviewed for constitutional compliance:
- PRs must demonstrate adherence to core principles
- Breaking constitutional principles requires explicit justification and approval
- Complexity added must be justified with clear benefits
- Use `ARCHITECTURE.md` and `CLAUDE.md` for runtime development guidance

### Continuous Improvement

Constitutional principles should evolve based on:
- Lessons learned from production issues
- User feedback and pain points
- Technical debt patterns that emerge
- New platform or technology requirements

**Version**: 1.0.0 | **Ratified**: 2026-04-01 | **Last Amended**: 2026-04-01
