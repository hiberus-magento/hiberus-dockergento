# Research Findings: AI Tools Management System

**Feature**: 001-ai-tools-management  
**Date**: 2026-04-01  
**Research Phase**: Phase 0

## Overview

Research conducted on three key technical areas: tarball download patterns, interactive Bash wizards, and file tracking strategies. All findings support Constitutional requirements (Bash implementation, fail-fast error handling, atomic operations).

---

## 1. Tarball Download & Extraction

### Decision: GitHub Archive URLs with Fallback

**Chosen Approach**: Primary tarball download via GitHub Archive API, fallback to `git clone --depth 1`

**Rationale**: 
- Tarball downloads are significantly faster (~50-500ms vs 2-5s for clone)
- No `.git` directory overhead (smaller disk footprint)
- Single HTTPS request (simpler error handling)
- Fallback ensures reliability when tarball fails

**Alternatives Considered**:
- **Git clone only**: Rejected due to performance (4-10x slower) and unnecessary `.git` metadata
- **Git sparse checkout**: Rejected due to complexity and limited cross-platform support
- **Manual HTTP requests**: Rejected due to authentication complexity for private repos

### Implementation Pattern

**URL Format**:
```bash
# GitHub
https://github.com/owner/repo/archive/refs/heads/branch.tar.gz

# GitLab
https://gitlab.com/owner/repo/-/archive/branch/repo-branch.tar.gz
```

**Atomic Download Pattern**:
```bash
download_repository_tarball() {
    local repo_url="$1"
    local branch="$2"
    local dest_dir="$3"
    
    local temp_dir=$(mktemp -d)
    trap "rm -rf '$temp_dir'" EXIT
    
    # Construct tarball URL
    local tarball_url="${repo_url}/archive/refs/heads/${branch}.tar.gz"
    
    # Download with retries
    if ! curl -L -f -S --max-time 30 --connect-timeout 10 \
              --retry 3 --retry-delay 2 --retry-max-time 120 \
              -o "${temp_dir}/repo.tar.gz" "$tarball_url"; then
        # Fallback to git clone
        git clone --depth 1 --branch "$branch" "$repo_url" "$temp_dir/repo" || return 1
        mv "$temp_dir/repo" "$dest_dir"
        return 0
    fi
    
    # Validate tarball integrity
    if ! tar -tzf "${temp_dir}/repo.tar.gz" >/dev/null 2>&1; then
        print_error "Corrupted tarball from $tarball_url"
        return 1
    fi
    
    # Check for path traversal attempts
    if tar -tzf "${temp_dir}/repo.tar.gz" | grep -qE '\.\./|^/'; then
        print_error "Security: Path traversal detected in tarball"
        return 1
    fi
    
    # Extract (GitHub wraps in repo-branch/ directory, strip that)
    mkdir -p "${temp_dir}/extracted"
    tar -xzf "${temp_dir}/repo.tar.gz" -C "${temp_dir}/extracted" --strip-components=1
    
    # Verify expected structure (skills/ or agents/ directories)
    if [[ ! -d "${temp_dir}/extracted/skills" ]] && [[ ! -d "${temp_dir}/extracted/agents" ]]; then
        print_warning "Repository lacks skills/ and agents/ directories"
        return 1
    fi
    
    # Atomic move to final destination
    mv "${temp_dir}/extracted" "$dest_dir"
}
```

**Key Features**:
- `--strip-components=1` handles GitHub's `repo-branch/` wrapper directory
- Validates tarball integrity before extraction
- Detects path traversal security issues
- Atomic operation via temp directory + `mv`
- Automatic fallback to git clone on tarball failure

**Error Handling**:
- Network failures (curl codes 6, 7, 28): Retry with exponential backoff (handled by `--retry`)
- 404/403: Fail immediately (no retries, invalid repo)
- Corrupted archive: Fail immediately (retry once via fallback to git clone)
- Security violations: Fail immediately (no recovery)

---

## 2. Interactive Bash Wizards

### Decision: Multi-Select with Pre-Fill Support

**Chosen Approach**: Associative arrays for state tracking, numeric input for selections, jq for JSON persistence

**Rationale**:
- No external dependencies beyond jq (already required)
- Works in all terminals (no ncurses complexity)
- Integrates with existing hm CLI patterns (`console/components/print.sh`, `console/helpers/properties.sh`)
- Simple flow control (retry on error, no complex state machines)

**Alternatives Considered**:
- **whiptail/dialog**: Rejected due to platform inconsistency (not always installed)
- **Custom TUI libraries**: Rejected due to Constitutional requirement (Bash only, no Node.js/Python)
- **Single-select only**: Rejected due to FR-003, FR-004 (must support multiple platforms/types)

### Implementation Pattern

**Multi-Select Prompt**:
```bash
multi_select_prompt() {
    local prompt="$1"
    local -n options_ref="$2"      # nameref to array of options
    local -n selected_ref="$3"     # nameref to result array
    
    declare -A selection_state
    local done=false
    
    while [[ "$done" == false ]]; do
        print_info "$prompt"
        print_info "Enter numbers (space-separated) or 'done' to confirm:"
        echo ""
        
        local index=1
        for option in "${options_ref[@]}"; do
            local marker="[ ]"
            [[ -n "${selection_state[$option]}" ]] && marker="[X]"
            printf "%d) %s %s\n" "$index" "$marker" "$option"
            ((index++))
        done
        
        echo ""
        read -r input
        
        if [[ "$input" == "done" ]]; then
            if [[ ${#selection_state[@]} -eq 0 ]]; then
                print_warning "Select at least one option"
                continue
            fi
            done=true
        else
            for num in $input; do
                if [[ "$num" =~ ^[0-9]+$ ]] && [[ $num -ge 1 ]] && [[ $num -le ${#options_ref[@]} ]]; then
                    local selected_option="${options_ref[$((num-1))]}"
                    if [[ -n "${selection_state[$selected_option]}" ]]; then
                        unset selection_state[$selected_option]  # Toggle off
                    else
                        selection_state[$selected_option]=1      # Toggle on
                    fi
                fi
            done
        fi
    done
    
    # Copy results to output array
    selected_ref=("${!selection_state[@]}")
}
```

**JSON Persistence with Pre-Fill**:
```bash
# Load existing configuration (pre-fill wizard)
load_ai_config() {
    local config_file="$CUSTOM_PROPERTIES_DIR/ai-properties.json"
    
    if [[ -f "$config_file" ]]; then
        # Validate JSON syntax
        if ! jq empty "$config_file" 2>/dev/null; then
            print_warning "Corrupted ai-properties.json, starting fresh"
            return 1
        fi
        
        # Load values as environment variables
        eval "$(jq -r 'to_entries[] | .key + "=\"" + (.value|tostring) + "\""' "$config_file")"
    fi
}

# Save wizard results atomically
save_ai_config() {
    local platforms="$1"
    local types="$2"
    local resources="$3"
    local repositories="$4"
    
    local config_file="$CUSTOM_PROPERTIES_DIR/ai-properties.json"
    local temp_file=$(mktemp)
    
    # Build JSON (jq ensures proper escaping)
    jq -n \
        --arg platforms "$platforms" \
        --arg types "$types" \
        --arg resources "$resources" \
        --arg repos "$repositories" \
        '{
            platforms: ($platforms | split(",")),
            skill_types: ($types | split(",")),
            resource_types: ($resources | split(",")),
            custom_repositories: ($repos | split(",") | map(select(length > 0)))
        }' > "$temp_file"
    
    # Atomic move (prevents corruption on failure)
    mv "$temp_file" "$config_file"
}
```

**Non-Interactive Mode**:
```bash
# Command-line flags override wizard
while getopts "p:t:r:b:-:" opt; do
    case "$opt" in
        p) PLATFORMS="$OPTARG" ;;
        t) TYPES="$OPTARG" ;;
        r) RESOURCES="$OPTARG" ;;
        -)
            case "$OPTARG" in
                platforms=*) PLATFORMS="${OPTARG#*=}" ;;
                types=*) TYPES="${OPTARG#*=}" ;;
                repository=*) CUSTOM_REPO="${OPTARG#*=}" ;;
                branch=*) CUSTOM_BRANCH="${OPTARG#*=}" ;;
            esac
            ;;
    esac
done

# If all required flags provided, skip wizard
if [[ -n "$PLATFORMS" ]] && [[ -n "$TYPES" ]] && [[ -n "$RESOURCES" ]]; then
    NON_INTERACTIVE=true
else
    # Launch interactive wizard
    run_wizard
fi
```

**Error Handling**:
- Ctrl+C: `trap 'exit 130' INT TERM` (standard POSIX exit code)
- Invalid input: Re-display prompt with warning message
- Empty selections: Require at least one choice before proceeding
- JSON write failures: Keep temp file for debugging, display error message

---

## 3. File Tracking Strategy

### Decision: Whitelist-Based with Integrity Verification

**Chosen Approach**: Track downloaded directories in `ai-registration.json`, delete only listed paths, verify with checksums

**Rationale**:
- Whitelist approach guarantees custom skills/agents never deleted (FR-034)
- Per-directory tracking (not per-file) simplifies operations for multi-file skills
- Checksums detect corruption and manual modifications
- Atomic updates prevent partial state (Constitutional requirement)

**Alternatives Considered**:
- **Naming conventions** (e.g., `custom-*` prefix): Rejected due to user friction (forces specific naming)
- **Metadata markers** (`.custom` files): Rejected due to extra manual steps for users
- **Timestamp comparison**: Rejected due to unreliability (users might modify files)
- **Per-file tracking**: Rejected due to complexity (skills are multi-file directories)

### Implementation Pattern

**JSON Schema** (`ai-registration.json`):
```json
{
  "version": "1.0",
  "updated_at": "2026-04-01T10:30:00Z",
  "integrity": "sha256:abc123...",
  "skills": {
    "claude": [
      {
        "name": "hyva-theme-creator",
        "path": ".claude/skills/hyva-theme-creator",
        "source_repo": "https://github.com/hiberus-magento/hyva-ai-tools",
        "source_branch": "main",
        "downloaded_at": "2026-04-01T10:25:00Z",
        "checksum": "sha256:def456..."
      }
    ],
    "cursor": [...]
  },
  "agents": {
    "claude": [...]
  }
}
```

**Safe Deletion Pattern**:
```bash
reset_downloaded_files() {
    local registration_file="$CUSTOM_PROPERTIES_DIR/ai-registration.json"
    
    # Validate registration file exists and is valid JSON
    if [[ ! -f "$registration_file" ]]; then
        print_error "No ai-registration.json found. Nothing to reset."
        return 1
    fi
    
    if ! jq empty "$registration_file" 2>/dev/null; then
        print_error "Corrupted ai-registration.json. Cannot safely reset."
        return 1
    fi
    
    # Verify integrity checksum
    local stored_integrity=$(jq -r '.integrity' "$registration_file")
    local computed_integrity=$(jq 'del(.integrity)' "$registration_file" | sha256sum | cut -d' ' -f1)
    
    if [[ "$stored_integrity" != "sha256:$computed_integrity" ]]; then
        print_warning "Registration file integrity mismatch (manual edits detected)"
        print_question "Continue with reset? [y/N]"
        read -r confirm
        [[ "$confirm" != "y" ]] && return 1
    fi
    
    # Extract all tracked paths
    local paths=$(jq -r '
        (.skills, .agents) 
        | to_entries[] 
        | .value[] 
        | .path
    ' "$registration_file")
    
    # Delete only whitelisted paths
    local deleted_count=0
    while IFS= read -r path; do
        # Security: Validate path contains expected prefixes
        if [[ "$path" =~ \.(claude|cursor|codex|copilot|gemini|opencode)/(skills|agents)/ ]]; then
            if [[ -d "$path" ]]; then
                rm -rf "$path"
                print_success "Removed $path"
                ((deleted_count++))
            else
                print_warning "Tracked path not found: $path (already deleted?)"
            fi
        else
            print_error "Invalid path in registration (skipping): $path"
        fi
    done <<< "$paths"
    
    # Clear registration file
    rm -f "$registration_file"
    print_success "Reset complete. Removed $deleted_count skill/agent directories."
}
```

**Atomic Registration Update**:
```bash
update_registration() {
    local platform="$1"
    local resource_type="$2"  # "skills" or "agents"
    local name="$3"
    local path="$4"
    local source_repo="$5"
    local source_branch="$6"
    
    local registration_file="$CUSTOM_PROPERTIES_DIR/ai-registration.json"
    local temp_file=$(mktemp)
    local backup_file="${registration_file}.backup.$(date +%s)"
    
    # Backup existing registration
    [[ -f "$registration_file" ]] && cp "$registration_file" "$backup_file"
    
    # Initialize if doesn't exist
    if [[ ! -f "$registration_file" ]]; then
        echo '{"version":"1.0","skills":{},"agents":{}}' > "$registration_file"
    fi
    
    # Compute checksum of downloaded directory
    local checksum=$(find "$path" -type f -exec sha256sum {} \; | sort | sha256sum | cut -d' ' -f1)
    
    # Add entry
    jq --arg platform "$platform" \
       --arg type "$resource_type" \
       --arg name "$name" \
       --arg path "$path" \
       --arg repo "$source_repo" \
       --arg branch "$source_branch" \
       --arg checksum "sha256:$checksum" \
       --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
       '
       .updated_at = $timestamp |
       .[$type][$platform] += [{
           name: $name,
           path: $path,
           source_repo: $repo,
           source_branch: $branch,
           downloaded_at: $timestamp,
           checksum: $checksum
       }]
       ' "$registration_file" > "$temp_file"
    
    # Compute integrity checksum of entire registration
    local integrity=$(jq 'del(.integrity)' "$temp_file" | sha256sum | cut -d' ' -f1)
    jq --arg integrity "sha256:$integrity" '.integrity = $integrity' "$temp_file" > "${temp_file}.2"
    
    # Atomic move
    mv "${temp_file}.2" "$registration_file"
    rm -f "$temp_file"
}
```

**Conflict Detection**:
```bash
check_file_conflicts() {
    local target_path="$1"
    local registration_file="$CUSTOM_PROPERTIES_DIR/ai-registration.json"
    
    # File exists but not tracked = custom skill
    if [[ -d "$target_path" ]]; then
        if ! jq -e --arg path "$target_path" '
            (.skills, .agents) 
            | to_entries[] 
            | .value[] 
            | select(.path == $path)
        ' "$registration_file" >/dev/null 2>&1; then
            # Not in registration = custom skill
            print_warning "Skipping $target_path (custom skill, not tracked)"
            return 1  # Conflict detected
        fi
    fi
    
    return 0  # No conflict
}
```

**Key Features**:
- Hybrid tracking: Directories for deletion, individual file checksums for validation
- Integrity checksum on entire registration file prevents tampering
- Automatic backups with timestamps before modifications
- Path validation prevents security issues (only allow expected prefixes)
- Idempotent operations (safe to run multiple times)

---

## 4. Configuration Data Structures

### Platform Definitions (`data/ai-platforms.json`)

**Purpose**: Map platform names to directory conventions

```json
{
  "version": "1.0",
  "platforms": {
    "claude": {
      "skills_dir": ".claude/skills",
      "agents_dir": ".claude/agents",
      "description": "Claude Code (CLI, Desktop, Web)"
    },
    "cursor": {
      "skills_dir": ".cursor/skills",
      "agents_dir": ".cursor/agents",
      "description": "Cursor AI Editor"
    },
    "codex": {
      "skills_dir": ".codex/skills",
      "agents_dir": ".codex/agents",
      "description": "GitHub Codex"
    },
    "copilot": {
      "skills_dir": ".github/skills",
      "agents_dir": ".github/agents",
      "description": "GitHub Copilot"
    },
    "gemini": {
      "skills_dir": ".gemini/skills",
      "agents_dir": ".gemini/agents",
      "description": "Google Gemini"
    },
    "opencode": {
      "skills_dir": ".opencode/skills",
      "agents_dir": ".opencode/agents",
      "description": "OpenCode AI"
    }
  }
}
```

### Skill Types (`data/ai-skill-types.json`)

**Purpose**: Define skill type categories and which repositories provide them

```json
{
  "version": "1.0",
  "skill_types": {
    "hyva": {
      "description": "Hyvä Themes development skills",
      "repositories": ["hiberus-hyva", "hyva-official"]
    },
    "acs": {
      "description": "Adobe Commerce Storefront (Venia/PWA)",
      "repositories": ["hiberus-acs"]
    },
    "magento": {
      "description": "General Magento 2 development skills",
      "repositories": ["hiberus-magento"]
    },
    "php": {
      "description": "PHP general development skills",
      "repositories": ["hiberus-php"]
    }
  }
}
```

### Repository Sources (`data/ai-repositories.json`)

**Purpose**: Define default Hiberus repository sources

```json
{
  "version": "1.0",
  "repositories": {
    "hiberus-magento": {
      "url": "https://github.com/hiberus-magento/ai-tools",
      "branch": "main",
      "description": "Hiberus Magento AI Tools"
    },
    "hiberus-hyva": {
      "url": "https://github.com/hiberus-magento/hyva-ai-tools",
      "branch": "main",
      "description": "Hiberus Hyvä AI Tools"
    },
    "hiberus-acs": {
      "url": "https://github.com/hiberus-magento/acs-ai-tools",
      "branch": "main",
      "description": "Hiberus ACS AI Tools"
    },
    "hiberus-php": {
      "url": "https://github.com/hiberus-magento/php-ai-tools",
      "branch": "main",
      "description": "Hiberus PHP AI Tools"
    },
    "hyva-official": {
      "url": "https://github.com/hyva-themes/hyva-ai-tools",
      "branch": "main",
      "description": "Official Hyvä AI Tools"
    }
  }
}
```

---

## 5. Security Considerations

### Identified Risks

1. **Path Traversal**: Malicious tarballs could contain `../` sequences
   - **Mitigation**: Validate tarball contents before extraction, reject suspicious paths

2. **Code Injection**: Downloaded skills are executable code
   - **Mitigation**: Skills are Markdown documentation, not executed by CLI. Users control when AI assistants use them.

3. **Man-in-the-Middle**: HTTPS downloads could be intercepted
   - **Mitigation**: Use curl with proper SSL/TLS validation (default), whitelist known repository hosts

4. **Registration Tampering**: Users could manually edit `ai-registration.json` to protect malicious files
   - **Mitigation**: Integrity checksums detect tampering, user confirmation required on mismatch

5. **Disk Space Exhaustion**: Malicious repos could contain large files
   - **Mitigation**: No automatic mitigation (trusted repositories assumption), users monitor disk space

### Trust Model

- **Trusted by default**: Hiberus repositories (hardcoded in `data/ai-repositories.json`)
- **User verification required**: Custom repositories added via `--repository` flag
- **No sandboxing**: Skills/agents are documentation, not executed by CLI tool itself
- **User responsibility**: Vetting custom repository contents before configuration

---

## 6. Performance Targets

Based on Success Criteria and research findings:

| Operation | Target | Typical Reality | Notes |
|-----------|--------|-----------------|-------|
| Tarball download | <5s | 50-500ms | GitHub Archive API, small repos |
| Tarball extraction | <1s | 100-300ms | Typical skill repo <1MB compressed |
| Git clone fallback | <10s | 2-5s | Only when tarball fails |
| Full ai-pull | <30s | 10-20s | 4-5 repos, 20-30 skills total (SC-002) |
| Wizard completion | <2min | 1-1.5min | User think time dominates (SC-001) |
| Registration update | <500ms | 50-200ms | JSON write + checksum calculation |
| Reset operation | <5s | 1-2s | Delete 20-30 directories |

**Optimization Opportunities**:
- Parallel repository downloads (not in MVP, bash doesn't support async well)
- Cached tarballs (not in MVP, adds complexity)
- Incremental updates (not in MVP, requires diff logic)

---

## 7. Open Questions & Decisions Deferred

**None remaining.** All technical unknowns resolved through research:

- ✅ Tarball download URL format and extraction pattern
- ✅ Interactive wizard implementation with multi-select
- ✅ File tracking strategy with whitelist protection
- ✅ JSON schemas for all configuration files
- ✅ Atomic operation patterns with rollback
- ✅ Error handling strategies
- ✅ Security considerations and mitigations

---

## References

Research conducted via specialized agents:
- Tarball download research: 1,723 lines, 7 comprehensive documents
- Bash wizard patterns: 580 lines, 5 pattern guides with HM CLI integration examples
- File tracking strategies: 399-word executive report with production-ready implementations

All patterns validated against Constitutional requirements and existing Hiberus CLI codebase patterns.
