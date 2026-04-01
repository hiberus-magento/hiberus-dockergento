# Command Schemas: AI Tools Management

**Feature**: 001-ai-tools-management  
**Date**: 2026-04-01  
**Contract Type**: CLI Command Interface

## Overview

This document defines the public interface contracts for the three new CLI commands. These commands follow the existing Hiberus Dockergento CLI patterns and are registered in `data/command_descriptions.json`.

---

## Command: hm ai-init

### Purpose

Initialize AI tools configuration through interactive wizard or command-line flags. Downloads configured skills/agents after setup.

### Syntax

```bash
# Interactive mode (wizard)
hm ai-init

# Non-interactive mode (all flags required)
hm ai-init --platforms=PLATFORMS --types=TYPES [--resources=RESOURCES] [--repository=URL --branch=BRANCH]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `--platforms` / `-p` | comma-separated | No* | - | AI platforms to configure (claude,cursor,codex,copilot,gemini,opencode) |
| `--types` / `-t` | comma-separated | No* | - | Skill types to download (hyva,acs,magento,php) |
| `--resources` / `-r` | comma-separated | No | skills,agents | Resource types (skills, agents, or both) |
| `--repository` | URL | No | - | Custom repository HTTPS URL |
| `--branch` / `-b` | string | No | main | Branch for custom repository |

\* Required for non-interactive mode. If any required flag is missing, wizard launches automatically.

### Examples

**Interactive wizard**:
```bash
hm ai-init
# Prompts for platforms, types, resources
# Downloads configured skills/agents
# Saves configuration to config/docker/ai-properties.json
```

**Non-interactive with platforms and types**:
```bash
hm ai-init --platforms=claude,cursor --types=hyva,magento
# Downloads skills and agents for claude and cursor
# Includes hyva and magento skill types
```

**Non-interactive with custom repository**:
```bash
hm ai-init --platforms=claude --types=magento --repository=https://github.com/company/ai-tools --branch=develop
# Downloads from both default Hiberus repos AND custom company repo
```

**Reconfiguring existing setup**:
```bash
hm ai-init
# Wizard pre-fills existing values from config/docker/ai-properties.json
# User can modify selections
# Auto-triggers ai-pull with new configuration
```

### Behavior

1. **Check for existing configuration**:
   - If `config/docker/ai-properties.json` exists: Pre-fill wizard with current values
   - If not exists: Start with empty configuration

2. **Configuration mode selection**:
   - If all required flags provided: Non-interactive mode (skip wizard)
   - If any flag missing: Interactive wizard mode

3. **Interactive wizard flow**:
   - Prompt: Select resource types (skills, agents, or both) - multi-select
   - Prompt: Select AI platforms - multi-select
   - Prompt: Select skill types - multi-select
   - Prompt: Add custom repository? (y/n)
     - If yes: Prompt for URL and branch
   - Display summary, confirm
   - Validate all selections

4. **Save configuration**:
   - Atomically write to `config/docker/ai-properties.json`
   - Include timestamps (created_at, updated_at)

5. **Trigger download**:
   - Automatically execute `ai-pull` action with new configuration
   - Display progress information

6. **Exit codes**:
   - `0`: Success (configuration saved and downloads completed)
   - `1`: User cancelled wizard
   - `2`: Invalid flag values
   - `3`: Configuration write failed
   - `4`: Download operation failed (but configuration was saved)

### Output

**Success**:
```
[INFO] Starting AI tools initialization...
[INFO] Selected platforms: claude, cursor
[INFO] Selected skill types: hyva, magento
[INFO] Downloading from 3 repositories...
[OK] Downloaded 12 skills for claude
[OK] Downloaded 8 skills for cursor
[OK] Configuration saved to config/docker/ai-properties.json
[OK] AI tools initialization complete!
```

**Wizard cancellation**:
```
[INFO] AI tools initialization cancelled by user.
```

**Validation error**:
```
[ERROR] Invalid platform: invalid-platform
[ERROR] Valid platforms: claude, cursor, codex, copilot, gemini, opencode
```

### Side Effects

- Creates/updates `config/docker/ai-properties.json` (committed to git)
- Creates platform directories if they don't exist (e.g., `.claude/skills/`)
- Downloads skills/agents to platform directories
- Creates/updates `config/docker/ai-registration.json` (local, gitignored)
- Triggers `hm ai-pull` action internally

### Dependencies

- Requires internet connectivity for repository downloads
- Requires write permissions to project directory
- Requires `config/docker/` directory to exist
- Requires `data/ai-platforms.json`, `data/ai-skill-types.json`, `data/ai-repositories.json`

---

## Command: hm ai-pull

### Purpose

Download or update skills/agents based on existing configuration in `ai-properties.json`. Preserves custom skills/agents that weren't downloaded by the tool.

### Syntax

```bash
hm ai-pull [--force]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `--force` / `-f` | flag | No | false | Re-download all tracked files even if they exist |

### Examples

**Standard update**:
```bash
hm ai-pull
# Reads config/docker/ai-properties.json
# Downloads/updates skills and agents from configured repositories
# Skips existing files unless source repo has updates
```

**Force re-download**:
```bash
hm ai-pull --force
# Re-downloads all files regardless of existing state
# Useful for recovering from manual modifications
```

### Behavior

1. **Validate prerequisites**:
   - Check `config/docker/ai-properties.json` exists
   - If not exists: Display error, suggest running `hm ai-init` first
   - Validate JSON structure

2. **Load configuration**:
   - Read platforms, skill_types, resource_types, custom_repositories
   - Resolve repository sources (defaults + custom)

3. **For each repository**:
   - Download tarball via GitHub Archive API
   - If tarball fails: Fallback to `git clone --depth 1`
   - Validate repository structure (must have `skills/` or `agents/` directories)
   - If neither directory exists: Log warning, skip repository, continue with others

4. **For each skill/agent in repository**:
   - Construct target path based on platform and resource type
   - Check for conflicts (file exists but not in ai-registration.json)
     - If conflict: Log warning, skip file, continue
     - If not conflict: Extract files to target directory
   - Calculate checksum of downloaded directory
   - Update entry in ai-registration.json

5. **Finalize**:
   - Compute integrity checksum of entire registration
   - Atomically save `config/docker/ai-registration.json`
   - Display summary (files downloaded, files skipped, warnings)

6. **Exit codes**:
   - `0`: Success (all operations completed)
   - `1`: Configuration file missing or invalid
   - `2`: No repositories accessible (all downloads failed)
   - `3`: Partial success (some repositories failed but at least one succeeded)
   - `4`: Registration write failed

### Output

**Success**:
```
[INFO] Loading AI tools configuration...
[INFO] Downloading from 3 repositories...
[INFO] Processing hiberus-magento (https://github.com/hiberus-magento/ai-tools)...
[OK] Downloaded 5 skills for claude
[OK] Downloaded 3 agents for claude
[INFO] Processing hiberus-hyva (https://github.com/hiberus-magento/hyva-ai-tools)...
[OK] Downloaded 7 skills for cursor
[WARN] Skipping existing custom skill: .claude/skills/my-custom-skill
[OK] Updated ai-registration.json
[OK] AI tools update complete! Downloaded 15 files, skipped 1 custom file.
```

**Configuration missing**:
```
[ERROR] No AI tools configuration found.
[INFO] Run 'hm ai-init' to set up AI tools.
```

**Repository unreachable**:
```
[WARN] Failed to download from https://github.com/hiberus-magento/ai-tools (timeout)
[INFO] Continuing with remaining repositories...
[OK] Downloaded 8 files from 2 repositories.
```

**Network interruption (atomic rollback)**:
```
[ERROR] Download interrupted during extraction.
[ERROR] Rolling back partial changes...
[ERROR] AI tools update failed. Please retry 'hm ai-pull'.
```

### Side Effects

- Downloads/updates skills/agents to platform directories
- Creates platform directories if they don't exist
- Creates/updates `config/docker/ai-registration.json` (local, gitignored)
- Skips (preserves) custom skills/agents not tracked in registration
- Creates backup of registration file before modifications

### Dependencies

- Requires `config/docker/ai-properties.json` to exist
- Requires internet connectivity for repository downloads
- Requires write permissions to project directory
- Requires curl, tar, and jq commands

---

## Command: hm ai-reset

### Purpose

Remove all skills/agents that were downloaded by `hm ai-init` or `hm ai-pull`. Preserves custom skills/agents created manually by users.

### Syntax

```bash
hm ai-reset [--confirm]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `--confirm` / `-y` | flag | No | false | Skip confirmation prompt |

### Examples

**Interactive confirmation**:
```bash
hm ai-reset
# Prompts for confirmation before deletion
# Removes only files listed in ai-registration.json
# Preserves custom skills/agents
```

**Auto-confirm**:
```bash
hm ai-reset --confirm
# No prompts, immediately removes tracked files
```

### Behavior

1. **Validate prerequisites**:
   - Check `config/docker/ai-registration.json` exists
   - If not exists: Display error "No tracked files to reset", exit with code 1
   - Validate JSON structure
   - If corrupted: Display error "Cannot safely reset with corrupted registration", exit with code 2

2. **Verify integrity**:
   - Compute current integrity checksum
   - Compare with stored integrity field
   - If mismatch: Display warning "Registration file modified manually", require user confirmation

3. **Confirmation**:
   - If `--confirm` flag not provided:
     - Display list of files/directories to be deleted
     - Prompt: "Remove these files? [y/N]"
     - If user enters anything other than 'y': Cancel operation, exit code 0

4. **Delete tracked files**:
   - Extract all paths from registration (skills and agents)
   - For each path:
     - Validate path prefix (security check: must contain platform-specific pattern)
     - If validation fails: Skip with error message, continue
     - If directory exists: Remove recursively
     - If directory doesn't exist: Log warning (already deleted)
   - Count successful deletions

5. **Finalize**:
   - Remove `config/docker/ai-registration.json`
   - Display summary (files deleted, warnings)

6. **Exit codes**:
   - `0`: Success or user cancelled
   - `1`: Registration file missing
   - `2`: Registration file corrupted
   - `3`: No files deleted (all paths invalid or missing)

### Output

**Success**:
```
[INFO] Loading AI tools registration...
[INFO] The following files will be removed:
  - .claude/skills/hyva-theme-creator
  - .claude/skills/magento-module-generator
  - .claude/agents/code-reviewer
  - .cursor/skills/hyva-theme-creator
[QUESTION] Remove these files? [y/N]: y
[OK] Removed .claude/skills/hyva-theme-creator
[OK] Removed .claude/skills/magento-module-generator
[OK] Removed .claude/agents/code-reviewer
[OK] Removed .cursor/skills/hyva-theme-creator
[OK] Cleared ai-registration.json
[OK] AI tools reset complete! Removed 4 directories.
```

**User cancellation**:
```
[INFO] The following files will be removed:
  - .claude/skills/hyva-theme-creator
[QUESTION] Remove these files? [y/N]: n
[INFO] Reset cancelled.
```

**Registration missing**:
```
[ERROR] No ai-registration.json found. Nothing to reset.
[INFO] AI tools have not been initialized, or already reset.
```

**Registration corrupted**:
```
[ERROR] ai-registration.json is corrupted or invalid JSON.
[ERROR] Cannot safely reset. Please run 'hm ai-pull' to regenerate registration.
```

**Integrity mismatch**:
```
[WARN] Registration file integrity mismatch (manual edits detected).
[WARN] Stored checksum: sha256:abc123...
[WARN] Computed checksum: sha256:def456...
[QUESTION] Continue with reset? [y/N]: 
```

**Custom skills preserved**:
```
[INFO] Custom skills detected (not removing):
  - .claude/skills/my-custom-skill
  - .cursor/agents/my-custom-agent
[OK] AI tools reset complete! Removed 4 directories, preserved 2 custom files.
```

### Side Effects

- Deletes all directories listed in `config/docker/ai-registration.json`
- Removes `config/docker/ai-registration.json` file
- Preserves custom skills/agents not listed in registration
- Does NOT modify `config/docker/ai-properties.json` (configuration persists)

### Dependencies

- Requires `config/docker/ai-registration.json` to exist
- Requires write permissions to project directory
- Requires jq command for JSON parsing

---

## Common Patterns

### Error Handling

All commands follow fail-fast pattern with `set -euo pipefail`:

```bash
#!/bin/bash
set -euo pipefail

# Validation checks before operations
if [[ ! -f "$config_file" ]]; then
    print_error "Configuration file not found"
    exit 1
fi

# Clear error messages
print_error "Failed to download repository: $repo_url"
```

### Output Functions

All commands use standard CLI output functions from `console/components/print.sh`:

```bash
print_info "Informational message"
print_success "Operation succeeded"
print_warning "Non-fatal issue"
print_error "Fatal error occurred"
print_question "User prompt?"
```

### Configuration Loading

Standard pattern for loading configuration:

```bash
source "$(dirname "$0")/../helpers/properties.sh"

CUSTOM_PROPERTIES_DIR="${MAGENTO_DIR}/config/docker"
config_file="${CUSTOM_PROPERTIES_DIR}/ai-properties.json"

if [[ ! -f "$config_file" ]]; then
    print_error "No configuration found. Run 'hm ai-init' first."
    exit 1
fi

# Validate JSON
if ! jq empty "$config_file" 2>/dev/null; then
    print_error "Corrupted configuration file: $config_file"
    exit 1
fi
```

### Atomic Operations

Standard pattern for atomic file updates:

```bash
temp_file=$(mktemp)
trap "rm -f '$temp_file'" EXIT

# Write to temp file
jq ... > "$temp_file"

# Validate temp file
jq empty "$temp_file" || exit 1

# Atomic move
mv "$temp_file" "$target_file"
```

---

## Command Registration

### data/command_descriptions.json

Each command requires an entry:

```json
{
  "ai-init": {
    "description": "Initialize AI tools configuration and download skills/agents",
    "usage": "hm ai-init [--platforms=PLATFORMS] [--types=TYPES] [--resources=RESOURCES] [--repository=URL --branch=BRANCH]",
    "category": "development",
    "examples": [
      "hm ai-init",
      "hm ai-init --platforms=claude,cursor --types=hyva,magento",
      "hm ai-init --repository=https://github.com/company/ai-tools --branch=main"
    ]
  },
  "ai-pull": {
    "description": "Download or update AI skills/agents based on configuration",
    "usage": "hm ai-pull [--force]",
    "category": "development",
    "examples": [
      "hm ai-pull",
      "hm ai-pull --force"
    ]
  },
  "ai-reset": {
    "description": "Remove downloaded AI skills/agents, preserve custom ones",
    "usage": "hm ai-reset [--confirm]",
    "category": "development",
    "examples": [
      "hm ai-reset",
      "hm ai-reset --confirm"
    ]
  }
}
```

---

## Backward Compatibility

These commands are entirely new and do not modify any existing command behavior. They integrate with existing infrastructure:

- Use existing `console/helpers/properties.sh` for configuration loading
- Use existing `console/components/print.sh` for output
- Store configuration in existing `config/docker/` directory
- Follow existing command routing pattern via `bin/run`
- Compatible with existing git workflow (configuration is committed)

---

## Security Considerations

### URL Validation

All repository URLs must be validated:

```bash
if [[ ! "$repo_url" =~ ^https:// ]]; then
    print_error "Only HTTPS URLs are supported: $repo_url"
    exit 1
fi
```

### Path Validation

All registration paths must match expected patterns:

```bash
if [[ ! "$path" =~ \.(claude|cursor|codex|github|gemini|opencode)/(skills|agents)/ ]]; then
    print_error "Invalid path in registration: $path"
    exit 1
fi
```

### Tarball Safety

Validate tarball contents before extraction:

```bash
# Check for path traversal attempts
if tar -tzf "$tarball" | grep -qE '\.\./|^/'; then
    print_error "Security: Path traversal detected in tarball"
    exit 1
fi
```

### Integrity Verification

Verify registration file integrity:

```bash
stored_integrity=$(jq -r '.integrity' "$registration_file")
computed_integrity=$(jq 'del(.integrity)' "$registration_file" | sha256sum | cut -d' ' -f1)

if [[ "$stored_integrity" != "sha256:$computed_integrity" ]]; then
    print_warning "Registration file integrity mismatch (manual edits detected)"
fi
```

---

## Testing Contracts

### Manual Test Scenarios

**hm ai-init**:
1. Run on fresh project → wizard completes, files downloaded
2. Run with all flags → non-interactive, files downloaded
3. Run with existing config → wizard pre-fills values
4. Cancel wizard → no files created
5. Invalid flag values → error message displayed

**hm ai-pull**:
1. Run without config → error, suggests ai-init
2. Run with config → files downloaded
3. Repository unreachable → warning, continues with others
4. Custom skill exists → skips with warning
5. Network interruption → rollback, registration unchanged

**hm ai-reset**:
1. Run without registration → error message
2. Run with registration → prompts, deletes files
3. Run with --confirm → no prompt, deletes files
4. Cancel confirmation → no files deleted
5. Custom skills present → preserved, not deleted

### Contract Verification

Each command must:
- Return correct exit codes
- Display output in specified format
- Follow atomic operation patterns
- Respect fail-fast error handling
- Use standard CLI output functions
- Validate all inputs before operations
- Create backups before destructive operations
