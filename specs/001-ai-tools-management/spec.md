# Feature Specification: AI Tools Management System

**Feature Branch**: `001-ai-tools-management`  
**Created**: 2026-04-01  
**Status**: Draft  
**Input**: User description: "AI tools initialization and management commands (ai-init, ai-pull, ai-reset) for downloading and managing skills/agents from remote repositories"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Initial AI Tools Configuration (Priority: P1)

A developer working on a new Magento 2 project wants to set up AI coding assistants (like Claude, Cursor, or Copilot) with project-specific skills and agents. They run `hm ai-init` and are guided through an interactive wizard that asks about their AI platform choices (claude, cursor, etc.), project type (hyva, magento, php), and whether they need skills, agents, or both. After configuration, the tool automatically downloads the appropriate files from configured repositories and saves the configuration for team sharing.

**Why this priority**: This is the foundation - without initial setup, no other AI tools features can be used. It's the entry point for the entire feature.

**Independent Test**: Can be fully tested by running `hm ai-init` on a fresh project, completing the wizard, and verifying that skills/agents are downloaded to the correct directories and configuration files are created.

**Acceptance Scenarios**:

1. **Given** a Magento project without AI tools configuration, **When** developer runs `hm ai-init`, **Then** an interactive wizard starts asking about platforms, project types, and resource types
2. **Given** wizard is completed with selections (claude + hyva + skills), **When** wizard finishes, **Then** skills are downloaded to `.claude/skills/` and `config/docker/ai-properties.json` is created with the configuration
3. **Given** configuration is saved, **When** another developer clones the project and runs `hm ai-pull`, **Then** they automatically get the same skills/agents without running the wizard

---

### User Story 2 - Updating AI Tools (Priority: P2)

A developer receives notification that new skills or agents are available in the remote repositories. They run `hm ai-pull` and the tool reads the existing `ai-properties.json` configuration, fetches updates from the configured repositories, and downloads any new or updated skills/agents while preserving custom local skills/agents that were created manually.

**Why this priority**: Keeping AI tools up-to-date ensures developers benefit from improvements and new features. This is secondary to initial setup but critical for ongoing maintenance.

**Independent Test**: Can be tested by running `hm ai-pull` on a project with existing AI configuration, then verifying that downloaded skills are updated while custom skills remain untouched.

**Acceptance Scenarios**:

1. **Given** existing AI configuration in `ai-properties.json`, **When** developer runs `hm ai-pull`, **Then** tool downloads/updates skills and agents from configured repositories
2. **Given** custom skills exist alongside downloaded ones, **When** `hm ai-pull` executes, **Then** only downloaded skills are updated and custom skills are preserved
3. **Given** repositories have new skills available, **When** `hm ai-pull` completes, **Then** `ai-registration.json` is updated to reflect newly downloaded files

---

### User Story 3 - Reconfiguring AI Platform Settings (Priority: P2)

A developer initially configured only Claude but now wants to add Cursor support. They run `hm ai-init` again, and the wizard pre-fills existing configuration values, allowing them to modify selections (add cursor platform, keep existing hyva type). After completion, both Claude and Cursor skills are available.

**Why this priority**: Teams evolve their tooling, and developers may use different AI platforms. Reconfiguration must be seamless without losing existing setup.

**Independent Test**: Can be tested by running `hm ai-init` twice - first with basic config, then again to add new platforms - and verifying cumulative configuration works correctly.

**Acceptance Scenarios**:

1. **Given** existing `ai-properties.json` with claude configuration, **When** developer runs `hm ai-init`, **Then** wizard shows current values as defaults
2. **Given** wizard allows modification of existing selections, **When** developer adds cursor to platforms and completes wizard, **Then** both claude and cursor skills are downloaded
3. **Given** updated configuration is saved, **When** wizard completes, **Then** `ai-pull` action runs automatically with new configuration

---

### User Story 4 - Resetting Downloaded AI Tools (Priority: P3)

A developer wants to clean up their project by removing all automatically downloaded skills/agents while keeping their custom ones. They run `hm ai-reset` and the tool reads `ai-registration.json` to identify exactly which files were downloaded by `ai-init`/`ai-pull`, removes only those files, and leaves custom skills/agents intact.

**Why this priority**: Cleanup functionality is useful but not critical to core workflow. Developers can manually delete files if needed.

**Independent Test**: Can be tested by running `hm ai-reset` on a project with both downloaded and custom skills, then verifying only downloaded ones are removed and registration file is cleared.

**Acceptance Scenarios**:

1. **Given** project has downloaded and custom skills/agents, **When** developer runs `hm ai-reset`, **Then** only files listed in `ai-registration.json` are deleted
2. **Given** custom skills exist (not in registration), **When** `hm ai-reset` executes, **Then** custom skills remain untouched
3. **Given** reset completes successfully, **When** checking `ai-registration.json`, **Then** file is cleared or removed

---

### User Story 5 - Custom Repository Support (Priority: P3)

A developer wants to use skills from a company-internal repository in addition to default Hiberus repositories. They run `hm ai-init --repository=https://github.com/company/ai-tools --branch=main` or specify it during the wizard, and the tool fetches skills from both default and custom repositories.

**Why this priority**: Custom repositories enable enterprise/project-specific extensions, but the core functionality works without them.

**Independent Test**: Can be tested by running `hm ai-init` with custom repository flags and verifying skills from both default and custom sources are downloaded.

**Acceptance Scenarios**:

1. **Given** custom repository URL and branch provided, **When** `hm ai-init` or `hm ai-pull` executes, **Then** tool fetches from both default Hiberus repos and custom repo
2. **Given** custom repository configuration in `ai-properties.json`, **When** other developers run `hm ai-pull`, **Then** they also fetch from custom repository
3. **Given** custom repository is unreachable, **When** `hm ai-pull` runs, **Then** tool shows warning but continues with available repositories

---

### Edge Cases

- What happens when a configured repository is unreachable or returns 404? → System logs warning and continues with available repositories (FR-027)
- How does the system handle conflicting skill names from different repositories? → Skip conflicting skills, preserve existing, log warning, continue (FR-030)
- What if `ai-registration.json` is corrupted or manually edited incorrectly? → Treat as missing: init/pull regenerate it, reset refuses to operate (FR-031, FR-032, FR-033)
- How does the tool behave if a user manually deletes files tracked in `ai-registration.json`?
- What happens when running `hm ai-pull` without any existing `ai-properties.json` configuration?
- How does the tool handle partial downloads (network interruption during fetch)? → Atomic operation: rollback partial downloads, leave registration unchanged, display error for manual retry (FR-035, FR-036, FR-037)
- What if a user runs `hm ai-reset` when no AI tools have been initialized?
- How does the system handle repository branch changes or branch deletions?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide an `hm ai-init` command that launches an interactive wizard for AI tools configuration
- **FR-002**: Wizard MUST ask users to select resource types (skills, agents, or both)
- **FR-003**: Wizard MUST ask users to select one or more AI platforms (claude, codex, copilot, cursor, gemini, opencode)
- **FR-004**: Wizard MUST ask users to select one or more project skill types (hyva, acs, magento, php)
- **FR-005**: System MUST support optional command-line flags for non-interactive configuration (e.g., `--platforms=claude,cursor --types=hyva,magento`)
- **FR-006**: System MUST accept optional `--repository` and `--branch` flags to add custom repositories
- **FR-007**: System MUST download skills/agents from remote Git repositories via tarball download (GitHub archive URLs)
- **FR-008**: System MUST support fallback to git clone if tarball download fails
- **FR-009**: System MUST place downloaded skills in platform-specific directories (e.g., `.claude/skills`, `.cursor/agents`)
- **FR-010**: System MUST save wizard configuration to `config/docker/ai-properties.json`
- **FR-011**: System MUST track downloaded files in `config/docker/ai-registration.json` with metadata (source repo, timestamp, file paths)
- **FR-012**: `hm ai-init` MUST automatically trigger `ai-pull` action after wizard completion
- **FR-013**: System MUST provide an `hm ai-pull` command that reads `ai-properties.json` and downloads/updates skills and agents
- **FR-014**: `hm ai-pull` MUST update `ai-registration.json` to reflect newly downloaded or updated files
- **FR-015**: `hm ai-pull` MUST NOT remove or modify custom skills/agents that are not tracked in `ai-registration.json`
- **FR-016**: System MUST provide an `hm ai-reset` command that removes only files listed in `ai-registration.json`
- **FR-017**: `hm ai-reset` MUST preserve custom skills/agents not present in `ai-registration.json`
- **FR-018**: `hm ai-reset` MUST clear or remove `ai-registration.json` after successful cleanup
- **FR-019**: System MUST use existing Hiberus CLI configuration infrastructure (similar to `properties.json` storage in `config/docker/`)
- **FR-020**: System MUST allow platform and skill type definitions to be easily extensible via configuration files
- **FR-021**: System MUST support multiple repository sources (default Hiberus repos + custom project repos)
- **FR-022**: System MUST create platform directories if they don't exist before downloading
- **FR-023**: When `ai-init` is run on a project with existing configuration, wizard MUST pre-fill current values as defaults
- **FR-024**: System MUST validate repository URLs before attempting download
- **FR-025**: System MUST display progress information during downloads (repository being fetched, files being installed)
- **FR-026**: System MUST handle download errors gracefully with clear error messages
- **FR-027**: System MUST skip repositories that cannot be reached rather than failing entire operation
- **FR-030**: System MUST skip conflicting skill/agent names during download, preserve existing files, log warning message, and continue with remaining downloads
- **FR-031**: System MUST validate `ai-registration.json` structure on load; if corrupted or invalid JSON, treat as missing file
- **FR-032**: `hm ai-reset` MUST refuse to operate if `ai-registration.json` is missing or corrupted, displaying error message that no tracked files exist to reset
- **FR-033**: `hm ai-init` and `hm ai-pull` MUST regenerate `ai-registration.json` from scratch if file is missing or corrupted
- **FR-034**: System MUST identify custom skills/agents using whitelist approach: any skill/agent directory NOT listed in `ai-registration.json` is treated as custom and protected from modification or deletion
- **FR-035**: System MUST implement atomic download operations: if network interruption occurs during tarball fetch or extraction, rollback any partial files
- **FR-036**: System MUST NOT update `ai-registration.json` unless all downloads for current operation complete successfully
- **FR-037**: System MUST display clear error message on partial download failure instructing user to retry `hm ai-pull`
- **FR-038**: System MUST validate repository structure after download: accept repositories containing at least one of `skills/` or `agents/` directories
- **FR-039**: System MUST log warning message if repository lacks both `skills/` and `agents/` directories, then continue processing remaining repositories
- **FR-040**: System MUST allow repositories to contain only `skills/` or only `agents/` subdirectories (not required to have both)
- **FR-028**: `ai-properties.json` MUST be designed for version control (committed to project repository)
- **FR-029**: `ai-registration.json` MUST be added to `.gitignore` as it's local state

### Key Entities

- **AI Configuration (ai-properties.json)**: Stores user's platform choices, project types, resource types (skills/agents), and repository sources. This is the source of truth for what should be installed and is shared across team members via git.

- **AI Registration (ai-registration.json)**: Tracks which skills and agents were downloaded by the tool, including source repository, download timestamp, file paths, and checksums. This enables selective removal during reset and prevents deletion of custom resources. This is local state and not committed.

- **Repository Source**: Represents a Git repository containing skills/agents, with URL, branch, and optional filters for which subdirectories to scan (skills/, agents/). Default Hiberus repositories are configured in tool's `data/` directory, custom repositories can be added per-project.

- **Platform Definition**: Configuration mapping platform names (claude, cursor, etc.) to their directory conventions (.claude/skills, .cursor/agents, etc.). Stored in tool's `data/` directory for easy extension.

- **Skill Type Definition**: Configuration defining skill type categories (hyva, magento, php, acs) and which repositories/filters to use for each. Stored in tool's `data/` directory for easy extension.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developers can complete initial AI tools configuration wizard (`hm ai-init`) in under 2 minutes
- **SC-002**: Running `hm ai-pull` completes skill/agent updates in under 30 seconds for typical repository sizes (< 50 files)
- **SC-003**: Team members can adopt existing AI configuration and download skills without wizard interaction (just `hm ai-pull`) in under 1 minute
- **SC-004**: 95% of users successfully complete `hm ai-init` wizard without errors on first attempt *(Note: Post-launch metric - measured through user feedback and support requests, not buildable instrumentation)*
- **SC-005**: `hm ai-reset` removes only downloaded skills/agents with zero accidental deletion of custom resources
- **SC-006**: Configuration files (`ai-properties.json`) can be committed to git and used by entire team without conflicts *(Note: Validated through multi-user quickstart scenarios and .gitignore verification)*
- **SC-007**: System gracefully handles unreachable repositories with clear error messages and continues with available sources

## Clarifications

### Session 2026-04-01

- Q: How does the system handle conflicting skill names from different repositories? → A: Skip conflicting skills with warning - preserve existing skill, log warning message, continue with other downloads
- Q: What if `ai-registration.json` is corrupted or manually edited incorrectly? → A: Treat as missing - if corrupted, treat file as non-existent, allow init/pull to regenerate, make reset refuse to operate
- Q: How does the system detect custom vs downloaded skills/agents? → A: Absence from registration - any skill/agent directory NOT listed in `ai-registration.json` is treated as custom (whitelist approach)
- Q: How does the tool handle partial downloads (network interruption during fetch)? → A: Clean failure - rollback any partial downloads, leave registration unchanged, display error requiring manual retry of `ai-pull`
- Q: What happens when a repository lacks `skills/` or `agents/` directories? → A: Permissive with warning - accept repositories with at least one valid directory (skills/ or agents/), log warning if neither found, continue with other repositories

## Assumptions

- Users have stable internet connectivity to access remote Git repositories
- Remote repositories follow standard directory structure with `skills/` and/or `agents/` subdirectories (at least one required, validated per FR-038)
- Platform directory conventions follow standard patterns (e.g., `.claude/skills/`, `.cursor/agents/`)
- Git repositories are publicly accessible or authentication is already configured in the user's system
- Tarball downloads via HTTPS are preferred method; git clone is fallback when tarball fails
- Project root is a Magento 2 project with existing `config/docker/` directory structure
- Skills and agents are organized as directories (not individual files) within their respective folders
- Downloaded skills/agents do not have conflicting names across different repositories
- Custom skills/agents created by users have different names than those in remote repositories
- Repository maintainers version their skills/agents through git commits, not internal versioning schemes
- Users running `hm ai-init` have write permissions to project directory
- The tool will be used primarily in development environments, not production
- Default Hiberus repositories contain general Magento, PHP, and Hyvä skills; project-specific repositories contain custom extensions
