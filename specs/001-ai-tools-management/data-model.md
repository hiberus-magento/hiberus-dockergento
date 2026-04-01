# Data Model: AI Tools Management System

**Feature**: 001-ai-tools-management  
**Date**: 2026-04-01  
**Phase**: 1 - Design

## Overview

This feature uses file-based JSON storage for configuration and state tracking. All data structures follow the existing Hiberus CLI pattern of storing configuration in `config/docker/` (project-specific) and `data/` (tool defaults).

---

## Entity Definitions

### 1. AI Configuration (ai-properties.json)

**Purpose**: Stores user's platform choices, project types, resource types, and repository sources. Shared across team members via git (committed).

**Location**: `config/docker/ai-properties.json`

**Schema**:
```json
{
  "version": "1.0",
  "platforms": ["claude", "cursor"],
  "skill_types": ["hyva", "magento"],
  "resource_types": ["skills", "agents"],
  "custom_repositories": [
    {
      "url": "https://github.com/company/ai-tools",
      "branch": "main"
    }
  ],
  "created_at": "2026-04-01T10:00:00Z",
  "updated_at": "2026-04-01T10:00:00Z"
}
```

**Fields**:
- `version` (string, required): Schema version for future migrations
- `platforms` (array<string>, required): Selected AI platforms (claude, cursor, codex, copilot, gemini, opencode)
- `skill_types` (array<string>, required): Selected skill type categories (hyva, acs, magento, php)
- `resource_types` (array<string>, required): What to download ("skills", "agents", or both)
- `custom_repositories` (array<object>, optional): Additional repository sources beyond defaults
  - `url` (string, required): Git repository HTTPS URL
  - `branch` (string, required): Git branch name
- `created_at` (ISO 8601 timestamp, required): Initial configuration timestamp
- `updated_at` (ISO 8601 timestamp, required): Last modification timestamp

**Validation Rules**:
- At least one platform required
- At least one skill_type required
- At least one resource_type required ("skills", "agents", or both)
- Custom repository URLs must be valid HTTPS URLs
- All values must match definitions in `data/ai-platforms.json` and `data/ai-skill-types.json`

**Lifecycle**:
1. Created by `hm ai-init` after wizard completion or flag parsing
2. Updated by `hm ai-init` when user reconfigures (pre-fills existing values)
3. Read by `hm ai-pull` to determine what to download
4. Committed to git for team sharing
5. Never deleted (persists across ai-reset operations)

**Relationships**:
- References platform definitions in `data/ai-platforms.json`
- References skill type definitions in `data/ai-skill-types.json`
- Custom repositories augment defaults from `data/ai-repositories.json`

---

### 2. AI Registration (ai-registration.json)

**Purpose**: Tracks which skills and agents were downloaded by the tool. Enables selective removal during reset and prevents deletion of custom resources. Local state, not committed (gitignored).

**Location**: `config/docker/ai-registration.json`

**Schema**:
```json
{
  "version": "1.0",
  "updated_at": "2026-04-01T10:30:00Z",
  "integrity": "sha256:abc123def456...",
  "skills": {
    "claude": [
      {
        "name": "hyva-theme-creator",
        "path": ".claude/skills/hyva-theme-creator",
        "source_repo": "https://github.com/hiberus-magento/hyva-ai-tools",
        "source_branch": "main",
        "downloaded_at": "2026-04-01T10:25:00Z",
        "checksum": "sha256:def456abc789..."
      }
    ],
    "cursor": [
      {
        "name": "magento-module-generator",
        "path": ".cursor/skills/magento-module-generator",
        "source_repo": "https://github.com/hiberus-magento/ai-tools",
        "source_branch": "main",
        "downloaded_at": "2026-04-01T10:27:00Z",
        "checksum": "sha256:789abc123def..."
      }
    ]
  },
  "agents": {
    "claude": [
      {
        "name": "code-reviewer",
        "path": ".claude/agents/code-reviewer",
        "source_repo": "https://github.com/hiberus-magento/ai-tools",
        "source_branch": "main",
        "downloaded_at": "2026-04-01T10:28:00Z",
        "checksum": "sha256:456def789abc..."
      }
    ]
  }
}
```

**Fields**:
- `version` (string, required): Schema version for future migrations
- `updated_at` (ISO 8601 timestamp, required): Last modification timestamp
- `integrity` (string, required): SHA256 checksum of entire registration (excluding integrity field itself) for tamper detection
- `skills` (object<platform, array<SkillEntry>>, required): Downloaded skills organized by platform
- `agents` (object<platform, array<AgentEntry>>, required): Downloaded agents organized by platform

**SkillEntry / AgentEntry Fields**:
- `name` (string, required): Skill/agent directory name
- `path` (string, required): Absolute or relative path to skill/agent directory
- `source_repo` (string, required): Origin Git repository URL
- `source_branch` (string, required): Git branch name
- `downloaded_at` (ISO 8601 timestamp, required): Download timestamp
- `checksum` (string, required): SHA256 hash of all files in directory (detects manual modifications)

**Validation Rules**:
- Paths must contain expected prefixes: `.claude/`, `.cursor/`, `.codex/`, `.github/`, `.gemini/`, `.opencode/`
- Paths must contain `/skills/` or `/agents/` segment
- Integrity checksum must match computed value when loading file
- Platform names must match ai-properties.json configuration
- All checksums must be SHA256 format (`sha256:...`)

**Lifecycle**:
1. Created by `hm ai-init` after first successful download
2. Updated by `hm ai-pull` on each download/update operation (atomic)
3. Read by `hm ai-reset` to identify files to delete
4. Cleared/removed by `hm ai-reset` after successful cleanup
5. Regenerated from scratch if corrupted (treat as missing, per FR-031, FR-033)
6. **Never committed** to git (added to `.gitignore`)

**Integrity Protection**:
- Compute SHA256 of entire JSON excluding `integrity` field
- Store as `integrity` field value
- On load: recompute and compare, warn if mismatch
- User confirmation required if tampering detected before operations

**Relationships**:
- References paths created by `hm ai-pull` download operations
- Consulted by `hm ai-reset` for deletion whitelist
- Independent of ai-properties.json (registration persists even if properties change)

---

### 3. Platform Definition (data/ai-platforms.json)

**Purpose**: Maps platform names to directory conventions. Stored in tool's data/ directory for easy extension.

**Location**: `data/ai-platforms.json` (part of CLI tool, not project-specific)

**Schema**:
```json
{
  "version": "1.0",
  "platforms": {
    "claude": {
      "skills_dir": ".claude/skills",
      "agents_dir": ".claude/agents",
      "description": "Claude Code (CLI, Desktop, Web)",
      "enabled": true
    },
    "cursor": {
      "skills_dir": ".cursor/skills",
      "agents_dir": ".cursor/agents",
      "description": "Cursor AI Editor",
      "enabled": true
    },
    "codex": {
      "skills_dir": ".codex/skills",
      "agents_dir": ".codex/agents",
      "description": "GitHub Codex",
      "enabled": true
    },
    "copilot": {
      "skills_dir": ".github/skills",
      "agents_dir": ".github/agents",
      "description": "GitHub Copilot",
      "enabled": true
    },
    "gemini": {
      "skills_dir": ".gemini/skills",
      "agents_dir": ".gemini/agents",
      "description": "Google Gemini",
      "enabled": true
    },
    "opencode": {
      "skills_dir": ".opencode/skills",
      "agents_dir": ".opencode/agents",
      "description": "OpenCode AI",
      "enabled": true
    }
  }
}
```

**Fields**:
- `version` (string, required): Schema version
- `platforms` (object<platform_name, PlatformDef>, required): Platform definitions
  - `skills_dir` (string, required): Relative path for skills directory
  - `agents_dir` (string, required): Relative path for agents directory
  - `description` (string, required): Human-readable description for wizard
  - `enabled` (boolean, required): Whether platform is available for selection

**Validation Rules**:
- Directory paths must be relative (no leading `/`)
- Directory paths must start with `.` (hidden directories by convention)
- No duplicate directory paths across platforms

**Extensibility**:
To add new platform:
1. Add entry to `platforms` object
2. No code changes required
3. Automatically appears in wizard options

**Relationships**:
- Referenced by `hm ai-init` wizard to display platform choices
- Referenced by `hm ai-pull` to determine target directories
- Platform names referenced in ai-properties.json

---

### 4. Skill Type Definition (data/ai-skill-types.json)

**Purpose**: Defines skill type categories and which repositories provide them. Stored in tool's data/ directory for easy extension.

**Location**: `data/ai-skill-types.json` (part of CLI tool, not project-specific)

**Schema**:
```json
{
  "version": "1.0",
  "skill_types": {
    "hyva": {
      "description": "Hyvä Themes development skills",
      "repositories": ["hiberus-hyva", "hyva-official"],
      "enabled": true
    },
    "acs": {
      "description": "Adobe Commerce Storefront (Venia/PWA)",
      "repositories": ["hiberus-acs"],
      "enabled": true
    },
    "magento": {
      "description": "General Magento 2 development skills",
      "repositories": ["hiberus-magento"],
      "enabled": true
    },
    "php": {
      "description": "PHP general development skills",
      "repositories": ["hiberus-php"],
      "enabled": true
    }
  }
}
```

**Fields**:
- `version` (string, required): Schema version
- `skill_types` (object<type_name, TypeDef>, required): Skill type definitions
  - `description` (string, required): Human-readable description for wizard
  - `repositories` (array<string>, required): Repository names (references ai-repositories.json)
  - `enabled` (boolean, required): Whether type is available for selection

**Validation Rules**:
- Repository names must exist in `data/ai-repositories.json`
- At least one repository per skill type

**Extensibility**:
To add new skill type:
1. Add entry to `skill_types` object
2. Add corresponding repository to `data/ai-repositories.json`
3. No code changes required
4. Automatically appears in wizard options

**Relationships**:
- Referenced by `hm ai-init` wizard to display skill type choices
- Repository names reference `data/ai-repositories.json` entries
- Type names referenced in ai-properties.json

---

### 5. Repository Source (data/ai-repositories.json)

**Purpose**: Defines default Hiberus repository sources. Can be augmented with custom repositories via ai-properties.json.

**Location**: `data/ai-repositories.json` (part of CLI tool, not project-specific)

**Schema**:
```json
{
  "version": "1.0",
  "repositories": {
    "hiberus-magento": {
      "url": "https://github.com/hiberus-magento/ai-tools",
      "branch": "main",
      "description": "Hiberus Magento AI Tools",
      "enabled": true
    },
    "hiberus-hyva": {
      "url": "https://github.com/hiberus-magento/hyva-ai-tools",
      "branch": "main",
      "description": "Hiberus Hyvä AI Tools",
      "enabled": true
    },
    "hiberus-acs": {
      "url": "https://github.com/hiberus-magento/acs-ai-tools",
      "branch": "main",
      "description": "Hiberus ACS AI Tools",
      "enabled": true
    },
    "hiberus-php": {
      "url": "https://github.com/hiberus-magento/php-ai-tools",
      "branch": "main",
      "description": "Hiberus PHP AI Tools",
      "enabled": true
    },
    "hyva-official": {
      "url": "https://github.com/hyva-themes/hyva-ai-tools",
      "branch": "main",
      "description": "Official Hyvä AI Tools",
      "enabled": true
    }
  }
}
```

**Fields**:
- `version` (string, required): Schema version
- `repositories` (object<repo_name, RepoDef>, required): Repository definitions
  - `url` (string, required): Git repository HTTPS URL
  - `branch` (string, required): Default branch name
  - `description` (string, required): Human-readable description
  - `enabled` (boolean, required): Whether repository is active

**Validation Rules**:
- URLs must be valid HTTPS URLs
- URLs must point to Git repositories (validated during download)
- Branch names must be valid Git branch identifiers

**Extensibility**:
To add new repository:
1. Add entry to `repositories` object
2. No code changes required
3. Available for skill type definitions

**Relationships**:
- Referenced by skill type definitions (`data/ai-skill-types.json`)
- Augmented by custom repositories in ai-properties.json
- Used by `hm ai-pull` to determine download sources

---

## State Transitions

### AI Configuration Lifecycle

```
[No Config] 
    |
    | hm ai-init (wizard or flags)
    v
[Config Created] <----- hm ai-init (reconfigure, pre-fills values)
    |
    | Team member clones repo
    v
[Config Exists] -----> hm ai-pull (reads config, downloads)
    |
    | User updates selections
    v
[Config Updated] -----> hm ai-pull (auto-triggered after init)
```

### AI Registration Lifecycle

```
[No Registration]
    |
    | hm ai-init or hm ai-pull (first successful download)
    v
[Registration Created] <----- hm ai-pull (adds/updates entries)
    |
    | hm ai-reset
    v
[Registration Cleared/Removed]
    |
    | hm ai-pull again
    v
[Registration Recreated]
```

### Downloaded Skills/Agents Lifecycle

```
[Not Downloaded]
    |
    | hm ai-pull (downloads from repo)
    v
[Downloaded & Tracked] <----- hm ai-pull (updates if newer version)
    |                      |
    |                      | Conflict detected (custom file exists)
    |                      v
    |                  [Skipped with Warning]
    |
    | hm ai-reset
    v
[Removed]
```

### Custom Skills/Agents Lifecycle

```
[User Creates Manually]
    |
    v
[Custom Skill/Agent] -----> hm ai-pull (ignored, not tracked)
    |
    v
[Custom Skill/Agent] -----> hm ai-reset (preserved, not deleted)
```

---

## Data Flow

### hm ai-init Flow

```
User Input (wizard/flags)
    ↓
Validate selections
    ↓
Load existing ai-properties.json (if exists)
    ↓
Merge/override with new selections
    ↓
Save to ai-properties.json (atomic)
    ↓
Trigger ai-pull action
    ↓
Download skills/agents
    ↓
Update ai-registration.json (atomic)
```

### hm ai-pull Flow

```
Load ai-properties.json
    ↓
Validate configuration
    ↓
Resolve repositories (defaults + custom)
    ↓
For each repository:
    ↓
    Download tarball (or git clone fallback)
    ↓
    Validate structure (skills/ or agents/ dirs)
    ↓
    For each skill/agent:
        ↓
        Check conflicts (existing file not tracked?)
        ↓
        If conflict: Skip with warning
        ↓
        If no conflict: Extract to platform directory
        ↓
        Update ai-registration.json entry
    ↓
Compute registration integrity checksum
    ↓
Save ai-registration.json (atomic)
```

### hm ai-reset Flow

```
Load ai-registration.json
    ↓
Validate JSON structure
    ↓
Verify integrity checksum
    ↓
If tampered: Warn user, require confirmation
    ↓
Extract all tracked paths
    ↓
For each path:
    ↓
    Validate path prefix (security check)
    ↓
    If exists: Delete directory
    ↓
    If not exists: Warn (already deleted?)
    ↓
Clear/remove ai-registration.json
```

---

## Storage Patterns

### Atomic Write Pattern

All configuration/registration updates use this pattern to prevent corruption:

```bash
# 1. Write to temp file
jq ... > "$temp_file"

# 2. Validate temp file
jq empty "$temp_file" || exit 1

# 3. Atomic move (single filesystem operation)
mv "$temp_file" "$target_file"
```

### Backup Pattern

Before modifying registration:

```bash
backup_file="${registration_file}.backup.$(date +%s)"
[[ -f "$registration_file" ]] && cp "$registration_file" "$backup_file"
```

### Rollback Pattern

On download failure:

```bash
trap "rm -rf '$temp_dir'" EXIT  # Cleanup on any exit
# ... perform downloads in temp dir ...
# Only on complete success: mv "$temp_dir" "$final_dir"
```

---

## Validation & Constraints

### Cross-Entity Constraints

1. **Platform Names**: Must be consistent across ai-properties.json, ai-registration.json, and data/ai-platforms.json
2. **Repository References**: Skill types reference repositories that must exist in data/ai-repositories.json
3. **Path Prefixes**: Registration paths must match platform directory conventions from data/ai-platforms.json
4. **Resource Types**: ai-properties.json resource_types determines whether skills/, agents/, or both are downloaded

### Integrity Checks

1. **JSON Syntax**: All files validated with `jq empty` before use
2. **Schema Versioning**: Version field allows future migrations
3. **Checksum Validation**: Registration integrity field detects tampering
4. **Path Security**: Only allow expected path prefixes in registration (prevent arbitrary file deletion)

### Error Recovery

1. **Corrupted ai-properties.json**: Warn user, start fresh configuration
2. **Corrupted ai-registration.json**: Treat as missing, regenerate on next pull
3. **Missing directories**: Create platform directories before download (FR-022)
4. **Partial downloads**: Rollback temp files, keep existing registration unchanged (FR-035, FR-036)

---

## Extension Points

### Adding New Platforms

1. Edit `data/ai-platforms.json`, add entry
2. No code changes required
3. Automatically available in wizard

### Adding New Skill Types

1. Edit `data/ai-skill-types.json`, add entry
2. Add corresponding repository to `data/ai-repositories.json`
3. No code changes required
4. Automatically available in wizard

### Adding New Repositories

1. Edit `data/ai-repositories.json`, add entry
2. No code changes required
3. Available for skill type definitions
4. Users can also add custom repos via `--repository` flag or wizard

---

## Git Integration

### Committed Files (Team-Shared)

- `config/docker/ai-properties.json` - Configuration shared across team
- `data/ai-platforms.json` - Platform definitions
- `data/ai-skill-types.json` - Skill type definitions
- `data/ai-repositories.json` - Default repository sources

### Ignored Files (Local State)

- `config/docker/ai-registration.json` - Local tracking of downloads
- `config/docker/ai-registration.json.backup.*` - Registration backups

### .gitignore Entry

```gitignore
# AI Tools local state (FR-029)
config/docker/ai-registration.json
config/docker/ai-registration.json.backup.*
```

---

## Performance Considerations

### File Sizes

- ai-properties.json: <1 KB typically (small configuration)
- ai-registration.json: ~1-5 KB (20-30 skills/agents tracked)
- data/*.json: <10 KB each (metadata only)

### I/O Operations

- JSON reads: Cached in memory during command execution
- JSON writes: Atomic (single `mv` operation)
- Checksums: Computed once per download operation
- Registration backups: One per modification (timestamped)

### Scalability

- Supports hundreds of skills/agents without performance degradation
- Registration file size grows linearly with tracked files
- No database required (file-based storage sufficient)
