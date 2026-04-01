# Tasks: AI Tools Management System

**Input**: Design documents from `/specs/001-ai-tools-management/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Not explicitly requested in specification - manual testing approach per existing CLI patterns.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

This is a CLI extension for Hiberus Dockergento. All paths follow existing CLI patterns:
- Commands: `console/commands/`
- Tasks: `console/tasks/`
- Helpers: `console/helpers/`
- Data: `data/`
- Configuration: `config/docker/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and configuration files that all commands will use

- [X] T001 Create platform definitions in data/ai-platforms.json
- [X] T002 Create skill type definitions in data/ai-skill-types.json
- [X] T003 Create default repository sources in data/ai-repositories.json
- [X] T004 Add command descriptions to data/command_descriptions.json
- [X] T005 Update .gitignore to exclude config/docker/ai-registration.json and verify ai-properties.json is committable (FR-028, FR-029)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared helper functions that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Implement JSON configuration I/O in console/tasks/ai_registration.sh (load/save ai-properties.json and ai-registration.json with atomic writes)
- [X] T007 [P] Implement tarball download function in console/tasks/ai_download.sh (curl with retry, GitHub archive API)
- [X] T008 [P] Implement git clone fallback function in console/tasks/ai_download.sh (git clone with depth 1)
- [X] T009 [P] Implement tarball extraction with validation in console/tasks/ai_extract.sh (tar extraction, directory filtering)
- [X] T010 Implement atomic file operations in console/tasks/ai_extract.sh (temp dir, validate, atomic mv)
- [X] T011 [P] Implement registration tracking logic in console/tasks/ai_registration.sh (add/remove entries, SHA256 checksums, whitelist queries)
- [X] T012 [P] Implement repository structure validation in console/tasks/ai_extract.sh (check for skills/ or agents/ directories)
- [X] T013 [P] Implement URL validation in console/tasks/ai_download.sh (HTTPS only, format validation)

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Initial AI Tools Configuration (Priority: P1) 🎯 MVP

**Goal**: Enable developers to configure AI tools via interactive wizard and automatically download skills/agents

**Independent Test**: Run `hm ai-init` on fresh project, complete wizard with platform/type selections, verify skills downloaded to correct directories and config files created

### Implementation for User Story 1

- [X] T014 [P] [US1] Implement multi-select wizard for resource types in console/tasks/ai_wizard.sh (skills/agents/both selection)
- [X] T015 [P] [US1] Implement multi-select wizard for platforms in console/tasks/ai_wizard.sh (claude/cursor/codex/copilot/gemini/opencode)
- [X] T016 [P] [US1] Implement multi-select wizard for skill types in console/tasks/ai_wizard.sh (hyva/acs/magento/php)
- [X] T017 [US1] Implement optional repository/branch prompt in console/tasks/ai_wizard.sh (custom repo support)
- [X] T018 [US1] Implement non-interactive flag parsing in console/commands/ai-init.sh (--platforms, --types, --resources, --repository, --branch)
- [X] T019 [US1] Implement configuration save logic in console/commands/ai-init.sh (write ai-properties.json with timestamps)
- [X] T020 [US1] Implement repository resolution in console/commands/ai-init.sh (merge default repos from data/ with custom repos)
- [X] T021 [US1] Implement platform directory creation in console/commands/ai-init.sh (create .claude/skills, .cursor/agents, etc.)
- [X] T022 [US1] Implement download orchestration in console/commands/ai-init.sh (iterate repositories, download, extract, register)
- [X] T023 [US1] Implement auto-trigger of ai-pull after wizard in console/commands/ai-init.sh (call pull function after config save)
- [X] T024 [US1] Implement progress display in console/commands/ai-init.sh (repository being fetched, files installed)
- [X] T025 [US1] Implement error handling and rollback in console/commands/ai-init.sh (atomic operations, clear error messages)
- [X] T026 [US1] Add fail-fast error handling with set -euo pipefail to console/commands/ai-init.sh

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Updating AI Tools (Priority: P2)

**Goal**: Enable developers to update skills/agents from repositories while preserving custom files

**Independent Test**: Run `hm ai-pull` on project with existing ai-properties.json, verify downloaded skills updated and custom skills preserved

### Implementation for User Story 2

- [X] T027 [P] [US2] Implement configuration validation in console/commands/ai-pull.sh (check ai-properties.json exists and valid)
- [X] T028 [P] [US2] Implement conflict detection in console/tasks/ai_extract.sh (check if skill/agent name exists before extraction)
- [X] T029 [US2] Implement skip-with-warning logic in console/tasks/ai_extract.sh (preserve existing on conflict, log warning, continue)
- [X] T030 [US2] Implement download orchestration in console/commands/ai-pull.sh (read config, iterate repos, download for each platform/type)
- [X] T031 [US2] Implement registration update logic in console/commands/ai-pull.sh (update ai-registration.json only on full success)
- [X] T032 [US2] Implement custom skills preservation in console/commands/ai-pull.sh (whitelist approach: only touch files in registration)
- [X] T033 [US2] Implement repository failure handling in console/commands/ai-pull.sh (skip unreachable repos, continue with available)
- [X] T034 [US2] Implement --force flag in console/commands/ai-pull.sh (override existing files on conflict)
- [X] T035 [US2] Add fail-fast error handling with set -euo pipefail to console/commands/ai-pull.sh

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Reconfiguring AI Platform Settings (Priority: P2)

**Goal**: Enable developers to modify existing configuration without losing previous setup

**Independent Test**: Run `hm ai-init` twice - first with basic config, then add new platforms - verify cumulative configuration works

### Implementation for User Story 3

- [X] T036 [US3] Implement configuration pre-fill in console/tasks/ai_wizard.sh (read existing ai-properties.json, mark selections)
- [X] T037 [US3] Implement toggle logic in console/tasks/ai_wizard.sh (allow adding/removing platforms and types)
- [X] T038 [US3] Implement incremental repository list in console/commands/ai-init.sh (merge existing custom repos with new)
- [X] T039 [US3] Implement cumulative download in console/commands/ai-init.sh (download for all platforms including newly added)
- [X] T040 [US3] Implement configuration merge logic in console/commands/ai-init.sh (preserve existing timestamps, update only changed fields)

**Checkpoint**: At this point, User Stories 1, 2, AND 3 should all work independently

---

## Phase 6: User Story 4 - Resetting Downloaded AI Tools (Priority: P3)

**Goal**: Enable developers to remove auto-downloaded skills/agents while preserving custom ones

**Independent Test**: Run `hm ai-reset` on project with both downloaded and custom skills, verify only downloaded removed and registration cleared

### Implementation for User Story 4

- [X] T041 [P] [US4] Implement registration validation in console/commands/ai-reset.sh (check ai-registration.json exists and valid)
- [X] T042 [P] [US4] Implement corrupted registration handling in console/commands/ai-reset.sh (refuse to operate with error message)
- [X] T043 [US4] Implement file list extraction in console/commands/ai-reset.sh (read paths from ai-registration.json)
- [X] T044 [US4] Implement confirmation prompt in console/commands/ai-reset.sh (show files to be deleted, ask Y/N)
- [X] T045 [US4] Implement --confirm flag in console/commands/ai-reset.sh (skip prompt for automation)
- [X] T046 [US4] Implement selective deletion in console/commands/ai-reset.sh (remove only tracked files, preserve custom)
- [X] T047 [US4] Implement registration cleanup in console/commands/ai-reset.sh (clear or remove ai-registration.json after success)
- [X] T048 [US4] Implement summary display in console/commands/ai-reset.sh (count removed files, list preserved custom files)
- [X] T049 [US4] Add fail-fast error handling with set -euo pipefail to console/commands/ai-reset.sh

**Checkpoint**: At this point, User Stories 1, 2, 3, AND 4 should all work independently

---

## Phase 7: User Story 5 - Custom Repository Support (Priority: P3)

**Goal**: Enable developers to add company-internal or project-specific repositories

**Independent Test**: Run `hm ai-init --repository=URL --branch=main` and verify skills from both default and custom repos downloaded

### Implementation for User Story 5

- [X] T050 [P] [US5] Implement custom repository prompt in console/tasks/ai_wizard.sh (optional add custom repo Y/N)
- [X] T051 [P] [US5] Implement repository URL/branch input in console/tasks/ai_wizard.sh (validate HTTPS, branch name)
- [X] T052 [US5] Implement repository list merging in console/commands/ai-init.sh (combine data/ai-repositories.json with custom)
- [X] T053 [US5] Implement custom repo persistence in console/commands/ai-init.sh (save in ai-properties.json custom_repositories array)
- [X] T054 [US5] Implement custom repo loading in console/commands/ai-pull.sh (read custom repos from config, download)
- [X] T055 [US5] Implement custom repo failure handling in console/commands/ai-pull.sh (warn if unreachable, continue)

**Checkpoint**: All user stories should now be independently functional

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple commands and final validation

- [X] T056 [P] Add usage help text to all three commands (ai-init.sh, ai-pull.sh, ai-reset.sh)
- [X] T057 [P] Implement integrity checksum validation in console/tasks/ai_registration.sh (SHA256 for registration file)
- [X] T058 [P] Implement directory checksum tracking in console/tasks/ai_registration.sh (SHA256 for each skill/agent)
- [X] T059 [P] Add detailed logging for debugging in all commands (use console/components/print.sh)
- [X] T060 Verify all commands follow existing CLI patterns (color output, error handling, messaging)
- [X] T061 Verify configuration hierarchy respected (runtime args → project config → tool defaults)
- [X] T062 Test wizard completion time meets SC-001 (use `time` command, complete wizard with 2 platforms + 2 types, verify < 120 seconds)
- [X] T063 Test ai-pull performance meets SC-002 (use `time` command, download ~40 skills from test repo, verify < 30 seconds)
- [X] T064 Test team adoption flow meets SC-003 (use `time` command, run ai-pull on project with existing config, verify < 60 seconds)
- [X] T065 Edge case validation: custom skills preservation, --force flag, config persistence
- [X] T066 All core functionality verified: init (14s), pull (15s), reset, custom file protection

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-7)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P2 → P3 → P3)
- **Polish (Phase 8)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Independent (reads config, updates files)
- **User Story 3 (P2)**: Depends on User Story 1 (reuses wizard, modifies existing config) - But can be tested independently
- **User Story 4 (P3)**: Can start after Foundational (Phase 2) - Independent (only reads registration, deletes files)
- **User Story 5 (P3)**: Integrates with User Story 1 (wizard extension) - But can be tested independently with flags

### Within Each User Story

- Wizard components (T014-T017) can be built in parallel
- Download/extract functions already in Foundational phase
- Command orchestration builds on wizard and foundational functions
- Error handling and validation added last within each story

### Parallel Opportunities

**Phase 1 (Setup)**: All tasks can run in parallel
- T001 (platforms.json), T002 (skill-types.json), T003 (repositories.json) are independent files
- T004 (command_descriptions.json), T005 (.gitignore) can run alongside

**Phase 2 (Foundational)**: Most tasks can run in parallel
- T007-T008 (download functions) are independent
- T009-T010 (extraction) are independent
- T011-T012 (registration/validation) are independent
- T006 depends on nothing (just loads JSON)
- T013 (URL validation) is independent

**Phase 3 (User Story 1)**: Wizard tasks parallel, then sequential orchestration
- T014-T017 (wizard components) can run in parallel (different functions in same file)
- T018 (flag parsing) can run parallel with wizard
- T019-T026 must run somewhat sequentially (orchestration depends on wizard/flags)

**Phase 4 (User Story 2)**: Validation and conflict detection in parallel
- T027-T028 can run in parallel (different functions)
- T030-T035 somewhat sequential (orchestration)

**Phase 8 (Polish)**: Most tasks can run in parallel
- T056-T059 (help text, checksums, logging) all independent
- T060-T066 (validation/testing) should run sequentially

---

## Parallel Example: Phase 2 (Foundational)

```bash
# Launch all foundational functions in parallel:
Task: "Implement tarball download function in console/tasks/ai_download.sh"
Task: "Implement git clone fallback function in console/tasks/ai_download.sh"
Task: "Implement tarball extraction with validation in console/tasks/ai_extract.sh"
Task: "Implement registration tracking functions in console/tasks/ai_registration.sh"
Task: "Implement repository structure validation in console/tasks/ai_extract.sh"
Task: "Implement URL validation in console/tasks/ai_download.sh"
```

## Parallel Example: User Story 1 (Wizard Components)

```bash
# Launch all wizard components together:
Task: "Implement multi-select wizard for resource types in console/tasks/ai_wizard.sh"
Task: "Implement multi-select wizard for platforms in console/tasks/ai_wizard.sh"
Task: "Implement multi-select wizard for skill types in console/tasks/ai_wizard.sh"
Task: "Implement optional repository/branch prompt in console/tasks/ai_wizard.sh"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T005)
2. Complete Phase 2: Foundational (T006-T013) - CRITICAL blocks all stories
3. Complete Phase 3: User Story 1 (T014-T026)
4. **STOP and VALIDATE**: Test `hm ai-init` wizard flow independently
5. Deploy/demo if ready - developers can configure and download AI tools

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add User Story 1 → Test `hm ai-init` independently → Deploy (MVP!)
3. Add User Story 2 → Test `hm ai-pull` independently → Deploy
4. Add User Story 3 → Test reconfiguration flow independently → Deploy
5. Add User Story 4 → Test `hm ai-reset` independently → Deploy
6. Add User Story 5 → Test custom repos independently → Deploy
7. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together (Phases 1-2)
2. Once Foundational is done (T013 complete):
   - Developer A: User Story 1 (ai-init wizard)
   - Developer B: User Story 2 (ai-pull updates)
   - Developer C: User Story 4 (ai-reset cleanup)
3. User Story 3 depends on US1 completion (wizard reuse)
4. User Story 5 depends on US1 completion (wizard extension)
5. Polish phase after all desired stories complete

---

## Notes

**Terminology**: Throughout implementation, "AI tools" refers collectively to "skills" and "agents" (the downloadable resources). "Resource types" is used in code/config for the selection mechanism.

- All commands MUST use `set -euo pipefail` (Constitutional requirement IV)
- All commands route through `bin/run` entry point (Constitutional requirement II)
- Atomic operations with temp directories and rollback on failure (FR-035, FR-036)
- Tarball downloads preferred (4-10x faster), git clone as fallback (research.md)
- Whitelist approach for custom skills: only files in ai-registration.json are tracked (FR-034)
- Configuration hierarchy: runtime args → project config → tool defaults (Constitutional requirement VI)
- Use existing Hiberus CLI helper functions from console/components/print.sh and console/helpers/properties.sh
- Platform and skill type definitions in data/ enable easy extension without code changes
- Commit ai-properties.json (team-shared), gitignore ai-registration.json (local state)
- Each user story should be independently completable and testable
- Verify each checkpoint before proceeding to next story
