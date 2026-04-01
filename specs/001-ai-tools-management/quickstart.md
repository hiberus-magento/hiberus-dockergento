# Quickstart Guide: AI Tools Management

**Feature**: 001-ai-tools-management  
**For**: Developers using Hiberus Dockergento CLI  
**Time to Complete**: 5 minutes

## What This Feature Does

The AI Tools Management system allows you to automatically download and manage AI coding assistant skills and agents for your Magento 2 projects. Instead of manually copying skills from various repositories, you can configure once and let the tool handle downloads, updates, and cleanup while protecting your custom skills.

**Three new commands**:
- `hm ai-init` - Set up your AI tools configuration
- `hm ai-pull` - Update your skills/agents  
- `hm ai-reset` - Remove downloaded files (keeps custom ones)

---

## Prerequisites

- Hiberus Dockergento CLI installed and working
- Magento 2 project with `config/docker/` directory
- Internet connection (for downloading from repositories)
- Git and jq installed (usually already available)

---

## Quick Start: 5-Minute Setup

### Step 1: Initialize AI Tools (2 minutes)

Run the interactive wizard:

```bash
hm ai-init
```

You'll be asked three questions:

**Question 1: Resource types**
```
What would you like to download?
1) [ ] Skills
2) [ ] Agents

Enter numbers (space-separated) or 'done': 1 2
```
→ Select both (enter `1 2`)

**Question 2: AI Platforms**
```
Which AI platforms do you use?
1) [ ] claude     - Claude Code (CLI, Desktop, Web)
2) [ ] cursor     - Cursor AI Editor
3) [ ] codex      - GitHub Codex
4) [ ] copilot    - GitHub Copilot
5) [ ] gemini     - Google Gemini
6) [ ] opencode   - OpenCode AI

Enter numbers (space-separated) or 'done': 1 2
```
→ Select the platforms you use (e.g., `1 2` for Claude and Cursor)

**Question 3: Skill Types**
```
What type of skills does your project need?
1) [ ] hyva       - Hyvä Themes development
2) [ ] acs        - Adobe Commerce Storefront (PWA)
3) [ ] magento    - General Magento 2 development
4) [ ] php        - PHP general development

Enter numbers (space-separated) or 'done': 1 3
```
→ Select your project types (e.g., `1 3` if using Hyvä and Magento)

**Result**: Skills and agents are automatically downloaded to your project!

```
[OK] Downloaded 12 skills for claude
[OK] Downloaded 8 skills for cursor
[OK] Configuration saved to config/docker/ai-properties.json
[OK] AI tools initialization complete!
```

### Step 2: Verify Installation

Check that skills were downloaded:

```bash
ls -la .claude/skills/
ls -la .cursor/skills/
```

You should see directories for each downloaded skill.

### Step 3: Commit Configuration (Team Sharing)

The configuration file should be committed so your team can use the same setup:

```bash
git add config/docker/ai-properties.json
git commit -m "Add AI tools configuration"
git push
```

**Note**: The `ai-properties.json` file is designed for team sharing via git. The `ai-registration.json` file (which tracks downloaded files locally) is automatically excluded via `.gitignore`, preventing merge conflicts. Each team member maintains their own local registration state.

**Done!** Your AI assistants can now use the downloaded skills.

---

## Common Scenarios

### Scenario 1: Team Member Cloning Project

**Situation**: Another developer clones your project and wants the same AI tools.

**Solution**: They just run one command:

```bash
hm ai-pull
```

This reads `config/docker/ai-properties.json` (already in the repo) and downloads all configured skills/agents automatically—no wizard needed!

```
[INFO] Loading AI tools configuration...
[OK] Downloaded 15 skills and 5 agents
[OK] AI tools update complete!
```

---

### Scenario 2: Adding a New AI Platform

**Situation**: You initially set up for Claude, but now you also use Cursor.

**Solution**: Run `hm ai-init` again—it pre-fills your existing choices:

```bash
hm ai-init
```

Existing selections are already marked `[X]`:
```
Which AI platforms do you use?
1) [X] claude     - Claude Code
2) [ ] cursor     - Cursor AI Editor
3) [ ] codex      - GitHub Codex
...

Enter numbers (space-separated) or 'done': 2
```
→ Just add the new platform (enter `2` to toggle Cursor on)

Skills for both platforms are downloaded, and the configuration is updated.

---

### Scenario 3: Updating Skills (New Versions Available)

**Situation**: Hiberus released new skills, and you want to update.

**Solution**: Run the update command:

```bash
hm ai-pull
```

This downloads any new or updated skills while preserving your custom ones:

```
[INFO] Processing 3 repositories...
[OK] Updated 5 skills
[WARN] Skipping existing custom skill: .claude/skills/my-custom-skill
[OK] AI tools update complete!
```

---

### Scenario 4: Creating Custom Skills

**Situation**: You want to create project-specific skills for your team.

**Solution**: Just create the directory manually:

```bash
mkdir -p .claude/skills/my-project-skill
# Add your skill files...
```

**Important**: Custom skills are automatically protected!
- `hm ai-pull` will skip your custom skill (not overwrite it)
- `hm ai-reset` will preserve your custom skill (not delete it)

The tool only tracks files it downloaded, so anything you create manually is safe.

---

### Scenario 5: Cleaning Up (Remove Downloaded Skills)

**Situation**: You want to remove all auto-downloaded skills but keep your custom ones.

**Solution**: Run the reset command:

```bash
hm ai-reset
```

You'll see what will be deleted:

```
[INFO] The following files will be removed:
  - .claude/skills/hyva-theme-creator
  - .claude/skills/magento-module-generator
  - .cursor/skills/hyva-theme-creator

[QUESTION] Remove these files? [y/N]: y
```

Custom skills are automatically preserved:

```
[INFO] Custom skills detected (not removing):
  - .claude/skills/my-custom-skill

[OK] Removed 3 directories, preserved 1 custom file.
```

After reset, you can run `hm ai-pull` again to re-download.

---

## Non-Interactive Mode (CI/Scripting)

If you need to automate setup (e.g., in a setup script), you can skip the wizard:

```bash
# All required flags provided = non-interactive
hm ai-init --platforms=claude,cursor --types=hyva,magento --resources=skills,agents
```

This is useful for:
- Automated project setup scripts
- CI/CD pipelines
- Dockerfile initialization

---

## Advanced: Custom Repositories

**Situation**: Your company has internal AI skills in a private repository.

**Solution**: Add your repository during `hm ai-init`:

```bash
hm ai-init --repository=https://github.com/company/ai-tools --branch=main
```

Or when prompted in the wizard:
```
Add custom repository? [y/N]: y
Repository URL: https://github.com/company/ai-tools
Branch [main]: develop
```

Skills from both default Hiberus repositories AND your custom repository will be downloaded.

**Team members** automatically get your custom repository skills when they run `hm ai-pull` (configuration is committed).

---

## Troubleshooting

### Problem: "No configuration found. Run 'hm ai-init' first."

**Cause**: You ran `hm ai-pull` or `hm ai-reset` without first running `hm ai-init`.

**Fix**: Run the initialization command:
```bash
hm ai-init
```

---

### Problem: "Corrupted ai-properties.json"

**Cause**: The configuration file has invalid JSON syntax.

**Fix**: Delete and recreate:
```bash
rm config/docker/ai-properties.json
hm ai-init
```

---

### Problem: Repository download fails with timeout

**Cause**: Network issues or repository unavailable.

**Fix**: The command automatically continues with other repositories:
```
[WARN] Failed to download from https://github.com/repo (timeout)
[INFO] Continuing with remaining repositories...
```

Just retry later:
```bash
hm ai-pull
```

---

### Problem: "Skipping existing custom skill" warning

**Cause**: You have a skill directory that wasn't downloaded by the tool.

**Fix**: This is normal behavior! The tool is protecting your custom skill. No action needed unless you want to force re-download:
```bash
hm ai-pull --force
```

---

### Problem: Skills not appearing in AI assistant

**Cause**: AI assistant might need restart or skills directory not recognized.

**Fix**: 
1. Verify skills are in correct location:
   ```bash
   ls -la .claude/skills/
   ```
2. Restart your AI assistant (Claude Code CLI, Cursor, etc.)
3. Check assistant's settings for skills directory path

---

## Command Reference Quick Sheet

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `hm ai-init` | Set up configuration | First time, or adding platforms |
| `hm ai-pull` | Download/update skills | After cloning project, or getting updates |
| `hm ai-pull --force` | Force re-download all | After manual modifications, corruption recovery |
| `hm ai-reset` | Remove downloaded files | Cleaning up, troubleshooting |
| `hm ai-reset --confirm` | Remove without prompt | Scripting, automation |

---

## Configuration Files Overview

| File | Purpose | Committed? |
|------|---------|-----------|
| `config/docker/ai-properties.json` | Your configuration (platforms, types) | ✅ Yes (team shares) |
| `config/docker/ai-registration.json` | Tracking of downloaded files | ❌ No (local state) |
| `.claude/skills/` | Downloaded skills for Claude | ❌ No (downloaded on-demand) |
| `.cursor/skills/` | Downloaded skills for Cursor | ❌ No (downloaded on-demand) |

**What to commit**:
- ✅ `config/docker/ai-properties.json`
- ❌ Everything else (gitignored automatically)

---

## Best Practices

### For Team Lead (Initial Setup)

1. Run `hm ai-init` and configure for your project's needs
2. Test that skills downloaded successfully
3. Commit `ai-properties.json` to the repository
4. Document any custom repositories in project README
5. Share command with team: "Run `hm ai-pull` after cloning"

### For Team Members (Joining Project)

1. Clone the repository
2. Run `hm ai-pull` (reads existing configuration)
3. Start coding with AI assistants
4. Create custom skills if needed (automatically protected)

### For Updates

1. When Hiberus releases updates: Run `hm ai-pull`
2. Test new skills with your AI assistant
3. Share update with team: "New skills available, run `hm ai-pull`"

### For Custom Skills

1. Create in appropriate directory: `.claude/skills/my-skill/`
2. Add your skill documentation and examples
3. **Do NOT commit** custom skills to the main repo (keep them local or separate repo)
4. Custom skills are automatically protected from `ai-pull` and `ai-reset`

---

## Real-World Example Workflow

**Day 1: Project Lead Sets Up**
```bash
# Initialize AI tools
hm ai-init
# Select: claude, cursor platforms
# Select: hyva, magento skill types
# Select: skills and agents

# Add company's internal repository
# URL: https://github.com/acme/magento-ai-skills
# Branch: main

# [OK] Downloaded 25 skills and 10 agents

# Commit configuration
git add config/docker/ai-properties.json
git commit -m "Configure AI tools for project"
git push
```

**Day 2: Developer Joins Team**
```bash
# Clone project
git clone https://github.com/acme/magento-project
cd magento-project

# Set up environment
hm setup

# Get AI tools (one command!)
hm ai-pull
# [OK] Downloaded 25 skills and 10 agents

# Start coding with Claude/Cursor
# Skills are ready to use!
```

**Week 2: Create Custom Skill**
```bash
# Developer creates project-specific skill
mkdir -p .claude/skills/acme-checkout-workflow
echo "# ACME Checkout Workflow" > .claude/skills/acme-checkout-workflow/README.md
# Add documentation...

# Update AI tools (custom skill is preserved)
hm ai-pull
# [WARN] Skipping existing custom skill: .claude/skills/acme-checkout-workflow
# [OK] Updated 3 skills
```

**Month 2: Switch to Different Project**
```bash
# Clean up AI tools for this project
hm ai-reset --confirm
# [OK] Removed 25 skills and 10 agents

# Custom skill was preserved
ls .claude/skills/
# acme-checkout-workflow  (still there!)
```

---

## Next Steps

After completing this quickstart:

1. **Try using a skill**: Ask your AI assistant to use one of the downloaded skills
2. **Explore downloaded skills**: Look in `.claude/skills/` to see what's available
3. **Share with team**: Commit `ai-properties.json` so others can run `hm ai-pull`
4. **Create custom skills**: Add project-specific skills as needed
5. **Stay updated**: Run `hm ai-pull` periodically for new skills

---

## Getting Help

**Command help**:
```bash
hm ai-init --help
hm ai-pull --help
hm ai-reset --help
```

**View configuration**:
```bash
cat config/docker/ai-properties.json | jq .
```

**View tracked files**:
```bash
cat config/docker/ai-registration.json | jq .
```

**Hiberus Dockergento documentation**:
```bash
hm help
```

---

## Summary

**5-minute setup**:
1. Run `hm ai-init` → Answer 3 questions
2. Commit `config/docker/ai-properties.json`
3. Team members run `hm ai-pull` → Done!

**Key benefits**:
- ✅ Automatic downloads from multiple repositories
- ✅ Team-shared configuration (everyone gets same skills)
- ✅ Custom skills automatically protected
- ✅ One-command updates (`hm ai-pull`)
- ✅ Safe cleanup (`hm ai-reset` preserves custom skills)

**Remember**:
- Configuration is committed (team-shared)
- Downloaded files are local (not committed)
- Custom skills are automatically protected
- Updates are one command: `hm ai-pull`
