# Implementation Plan: AI Tools Management System

**Branch**: `001-ai-tools-management` | **Date**: 2026-04-01 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-ai-tools-management/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Implement three new Bash commands (`hm ai-init`, `hm ai-pull`, `hm ai-reset`) to download and manage AI coding assistant skills/agents from remote Git repositories. The system uses an interactive wizard for initial configuration, maintains team-shared configuration in `ai-properties.json`, tracks downloaded files in local `ai-registration.json`, and protects custom user-created skills/agents. Supports multiple AI platforms (claude, cursor, codex, copilot, gemini, opencode) and skill types (hyva, acs, magento, php) with extensible configuration stored in tool's `data/` directory.

## Technical Context

**Language/Version**: Bash 4.0+ (Constitutional requirement: Bash Implementation Consistency)  
**Primary Dependencies**: 
- curl (tarball downloads via HTTPS)
- tar (archive extraction)
- jq (JSON parsing for configuration files)
- git (fallback download method)
- Docker/Docker Compose (inherited from existing Hiberus CLI infrastructure)

**Storage**: File-based JSON configuration
- `config/docker/ai-properties.json` (committed, team-shared configuration)
- `config/docker/ai-registration.json` (local state, gitignored)
- `data/ai-platforms.json` (tool's platform definitions)
- `data/ai-skill-types.json` (tool's skill type definitions)
- `data/ai-repositories.json` (default Hiberus repository sources)

**Testing**: Manual testing on macOS and Linux platforms (per existing CLI patterns)
- Interactive wizard flows
- Command routing validation
- File download and extraction verification
- Custom skills preservation validation
- Atomic operation rollback testing

**Target Platform**: macOS and Linux development environments (per existing CLI)

**Project Type**: CLI extension (three new commands for existing Hiberus Dockergento CLI tool)

**Performance Goals**: 
- Wizard completion < 2 minutes (SC-001)
- Pull/update operations < 30 seconds for typical repos with < 50 files (SC-002)
- Team adoption without wizard < 1 minute (SC-003)

**Constraints**: 
- Must follow Command Router Architecture (Constitutional requirement)
- Must use fail-fast error handling with `set -euo pipefail` (Constitutional NON-NEGOTIABLE)
- Must respect Configuration Hierarchy Integrity (runtime args → project config → defaults)
- Must be backward compatible with existing CLI commands
- Network-dependent operations (repository access via HTTPS)
- Atomic download operations (rollback on partial failure per FR-035, FR-036)

**Scale/Scope**: 
- 6 AI platforms initially (extensible via configuration)
- 4 skill types initially (extensible via configuration)
- Multiple repository sources per project (default Hiberus + custom project repos)
- Typical repository size: < 50 skill/agent directories per repo

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Bash Implementation Consistency ✅
**Status**: PASS  
**Justification**: All three commands (ai-init, ai-pull, ai-reset) will be implemented as Bash scripts in `console/commands/`. Helper functions for wizard, download, and file tracking will be in `console/tasks/` and `console/helpers/`. Uses existing Bash infrastructure (curl, tar, jq).

### II. Command Router Architecture ✅
**Status**: PASS  
**Justification**: Commands follow standard pattern:
- `console/commands/ai-init.sh`
- `console/commands/ai-pull.sh`
- `console/commands/ai-reset.sh`
- Metadata entries in `data/command_descriptions.json`
- Routed through `bin/run` entry point

### III. Docker Abstraction Priority ✅
**Status**: PASS (N/A for this feature)  
**Justification**: Commands do not interact with Docker services directly. They manage AI tool configuration files and download skills/agents to project directories. This is development tooling that sits alongside Docker orchestration, not within it.

### IV. Fail-Fast Error Handling ✅
**Status**: PASS  
**Justification**: All scripts will use `set -euo pipefail` at the top. Includes:
- URL validation before downloads (FR-024)
- Repository structure validation (FR-038, FR-039)
- JSON parsing validation (FR-031)
- Atomic operations with rollback on failure (FR-035, FR-036)
- Clear error messages via `console/components/print.sh` functions

### V. Platform-Specific Optimization ✅
**Status**: PASS (N/A for this feature)  
**Justification**: File downloads and JSON operations are platform-agnostic. Skills/agents are written to standard directories (`.claude/skills/`, etc.) which work identically on macOS and Linux. No volume mounts or platform-specific overlays required.

### VI. Configuration Hierarchy Integrity ✅
**Status**: PASS  
**Justification**: Follows standard hierarchy:
1. Runtime args: `--platforms=claude,cursor --types=hyva,magento --repository=URL --branch=main`
2. Project config: `config/docker/ai-properties.json` (team-shared)
3. Global defaults: `data/ai-platforms.json`, `data/ai-skill-types.json`, `data/ai-repositories.json`

User-provided values never overridden. Wizard pre-fills existing values when reconfiguring (FR-023).

### VII. Backward Compatibility ✅
**Status**: PASS  
**Justification**: New commands do not modify existing command behavior. Adds three new commands to existing CLI. Configuration files stored in existing `config/docker/` directory pattern. No breaking changes to existing infrastructure.

**GATE RESULT**: ✅ ALL CHECKS PASSED - Proceed to Phase 0 Research

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
bin/
└── run                              # Entry point (routes to commands)

console/
├── commands/
│   ├── ai-init.sh                   # NEW: Interactive wizard + auto-pull
│   ├── ai-pull.sh                   # NEW: Download/update from repos
│   └── ai-reset.sh                  # NEW: Remove downloaded skills/agents
├── tasks/
│   ├── ai_wizard.sh                 # NEW: Interactive prompts logic
│   ├── ai_download.sh               # NEW: Tarball/git clone download
│   ├── ai_extract.sh                # NEW: Extract and install skills/agents
│   └── ai_registration.sh           # NEW: Track downloaded files
├── helpers/
│   ├── properties.sh                # EXISTING: Reuse for config loading
│   └── docker_validation.sh         # EXISTING: No changes needed
└── components/
    ├── print.sh                     # EXISTING: Reuse for messages
    └── input.sh                     # EXISTING: Reuse for wizard prompts

data/
├── command_descriptions.json        # UPDATE: Add 3 new command entries
├── ai-platforms.json                # NEW: Platform → directory mappings
├── ai-skill-types.json              # NEW: Skill type definitions
└── ai-repositories.json             # NEW: Default Hiberus repo sources

config/docker/                       # Per-project configuration
├── properties.json                  # EXISTING: No changes
├── ai-properties.json               # NEW: User's platform/type/repo choices
└── ai-registration.json             # NEW: Tracking downloaded files (gitignored)

.gitignore                           # UPDATE: Add config/docker/ai-registration.json
```

**Structure Decision**: CLI Extension Pattern

This feature extends the existing Hiberus Dockergento CLI tool by adding three new commands that follow the established Command Router Architecture. All implementation is in Bash scripts organized into commands (user-facing), tasks (reusable logic), and helpers (utilities). Configuration follows the existing pattern of storing project-specific settings in `config/docker/` and tool defaults in `data/`. No new top-level directories required—everything integrates into existing structure.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

**No violations detected.** All constitutional principles satisfied.
