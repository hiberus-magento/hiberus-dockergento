# AI Tools Agent Detection and Dockergento Skills Support

**Branch**: `005-ai-tools-dynamic-discovery`  
**Status**: Draft  
**Created**: 2026-04-07

## Problem Statement

After real-world testing of the AI Tools Management System, two critical bugs were identified:

### 1. Agents Not Being Detected

The installation process reports "No matching agents found" despite agents existing in the repository.

**Current behavior:**
```
Installing agents for claude...
No matching agents found for types: magento,php
```

**Actual repository structure:**
```
agents/
├── effort-estimator.md
├── story-generator.md
├── effort-item-extractor.md
└── rfp-analyzer.md
```

**Root cause**: The agent detection logic in `ai_extract.sh` only processes **directories**, but agents are **individual `.md` files**.

**Lines 119-122 in `console/tasks/ai_extract.sh`:**
```bash
for item_path in "${source_dir}"/*; do
    if [[ ! -d "${item_path}" ]]; then
        continue  # Skips files
    fi
```

### 2. Dockergento Skills Not Available

Skills with `dockergento-*` prefix exist in `hiberus-magento/ai-tools` repository but are never shown in the wizard.

**Missing skills** (~31% of repository):
- `dockergento-database-exporter`
- `dockergento-shell-executor`
- `dockergento-xdebug-toggle`
- `dockergento-mysql-controller`
- `dockergento-varnish-controller`

**Root cause**: The skill type `dockergento` doesn't exist in `data/ai-skill-types.json`.

## Proposed Solution

### Part 1: Fix Agent Detection with Directory Support

**Approach**: Support both flat `.md` files AND directory-organized agents.

**Rationale**: Provides flexibility for repositories to organize agents in subdirectories while extracting all `.md` files to platform's flat agent directory.

**Supported structures:**

**Flat** (current `hiberus-magento/ai-tools`):
```
agents/
├── effort-estimator.md
└── story-generator.md
```

**Organized** (future repositories):
```
agents/
├── project/
│   └── effort-estimator.md
└── code/
    └── analyzer.md
```

Both extract to:
```
.claude/agents/
├── effort-estimator.md
└── analyzer.md
```

**Implementation overview:**

1. Create `find_agent_files()` - recursive `.md` file discovery
2. Create `install_agents_from_directory()` - agent-specific installer
3. Update `install_from_repository()` - branch on resource type
4. Update `install_filtered()` - branch on resource type
5. Note: Agents are NOT filtered by skill types (they're cross-cutting tools)

### Part 2: Add Dockergento Skill Type

**Implementation:**

Update `data/ai-skill-types.json`:
```json
{
  "skill_types": {
    "dockergento": {
      "name": "Dockergento CLI",
      "description": "Dockergento development environment tools",
      "tags": ["cli", "docker", "devops", "magento"]
    }
  }
}
```

## Success Criteria

1. ✅ `dockergento` appears in skill types wizard
2. ✅ All 5 `dockergento-*` skills install when type selected
3. ✅ Agents from flat structure install correctly
4. ✅ Agents from nested directories install correctly (flattened)
5. ✅ Registration tracks installed agents (`.md` files)
6. ✅ Backward compatible with existing configurations

## Testing Strategy

**Test 1: Dockergento Skills**
```bash
hm ai-reset
hm ai-init  # Select: skills, claude, dockergento
ls -la .claude/skills/ | grep dockergento  # Expected: 5 directories
```

**Test 2: Flat Agents**
```bash
hm ai-init  # Select: agents, claude
ls -la .claude/agents/*.md  # Expected: 4 files from hiberus-magento/ai-tools
```

**Test 3: Nested Agents**
```bash
# Create test repo with nested structure
mkdir -p /tmp/test-repo/agents/cat-a
echo "---\nname: a1\n---" > /tmp/test-repo/agents/cat-a/agent-1.md
# Modify config to use test repo
hm ai-pull
test -f .claude/agents/agent-1.md && echo "✓ Extracted from nested"
```

## Implementation Priority

1. **Phase 1** (TRIVIAL): Add dockergento to `ai-skill-types.json`
2. **Phase 2** (CRITICAL): Fix agent detection in `ai_extract.sh`

**Files modified**:
- `data/ai-skill-types.json`
- `console/tasks/ai_extract.sh`

## Related Documentation

- Parent Spec: `specs/001-ai-tools-management/spec.md`
- Previous Release: `changelogs/v1.3.1.md`
