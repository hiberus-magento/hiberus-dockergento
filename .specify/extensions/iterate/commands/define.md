---
description: Define an iteration on the current feature — analyze the change request against current spec state and implementation progress, then write a reviewable iteration plan.
handoffs:
  - label: Apply Iteration
    agent: speckit.iterate.apply
    prompt: Apply the pending iteration to spec documents
    send: true
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

The text the user typed after `/speckit.iterate.define` **is** the change request. It describes what should be added, modified, or removed from the current feature — either the whole feature or a specific phase/subtask.

## Outline

Goal: Analyze a requested change against the feature's current spec state and implementation progress, then produce a reviewable iteration plan written to `pending-iteration.md`. This command does **not** modify any spec artifacts — it only writes the pending iteration file.

### 1. Initialize Feature Context

Run `.specify/scripts/bash/check-prerequisites.sh --json --paths-only` from repo root **once**. Parse JSON payload fields:

- `FEATURE_DIR`
- `FEATURE_SPEC`
- (Optionally capture `IMPL_PLAN`, `TASKS` for downstream use.)

If JSON parsing fails, abort and instruct the user to run `/speckit.specify` or verify the feature branch environment.

For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

### 2. Check for Existing Pending Iteration

Check if `FEATURE_DIR/pending-iteration.md` already exists. If it does:

- Warn the user: "A pending iteration already exists. Running define again will overwrite it."
- Show the existing file's `change_request` and `created` date from its frontmatter.
- Ask the user to confirm before proceeding, or suggest running `/speckit.iterate.apply` first.

### 3. Load Current Artifacts

Read all available artifacts from FEATURE_DIR:

- **REQUIRED**: `spec.md` — requirements, user stories, edge cases, scope
- **IF EXISTS**: `plan.md` — tech stack, architecture, file structure, phases
- **IF EXISTS**: `tasks.md` — task list with IDs, phases, checkboxes, dependencies
- **IF EXISTS**: `data-model.md` — entities, attributes, relationships
- **IF EXISTS**: `research.md` — technical decisions and constraints
- **IF EXISTS**: `quickstart.md` — integration/test scenarios
- **IF EXISTS**: `checklists/` — quality checklists

Build an internal model of the feature's current state: user stories, requirements, tasks, phases, scope boundaries, and assumptions.

### 4. Assess Implementation Progress

Determine what has already been built so the iteration plan accounts for reality.

#### A. Task state analysis

Parse `tasks.md` (if it exists) to determine:

- Total tasks and how many are complete (`[x]`) vs remaining (`[ ]`)
- Which phase the feature is currently in (first phase with incomplete tasks)
- Any tasks marked as blocked or dependent on incomplete work

#### B. Git-based progress detection

Run these commands to detect code changes on the feature branch:

```bash
# Changed files on this branch vs its base
git diff --name-status $(git merge-base HEAD main)..HEAD

# Recent commits on this branch
git log --oneline $(git merge-base HEAD main)..HEAD

# Uncommitted changes
git diff --name-only
git diff --name-only --staged
```

Cross-reference changed files with tasks in `tasks.md`:

- **Mapped**: File appears in a task description — that task is likely complete or in-progress
- **Unmapped**: File was changed but no task references it — note as adhoc work

#### C. Build progress summary

Produce a structured understanding of:

- Tasks completed vs remaining (with IDs)
- Current implementation phase
- Files changed that map to tasks (potential completions to mark during apply)
- Any adhoc changes not covered by existing tasks

### 5. Analyze the Change Request

Parse the user's change request (`$ARGUMENTS`) and classify it:

**Scope classification:**

| Scope | Signal | Example |
|-------|--------|---------|
| Feature-wide | Mentions overall goal, new user story, scope boundary change, new requirement | "Add a third tab for Reports" |
| Phase-level | References a specific phase by name or number | "In Phase 2, also migrate the FAQ section" |
| Task-level | References a specific task ID or narrow file-level change | "T007 should also copy the FAQ parser" |
| Subtraction | Explicitly removes scope, user story, or requirement | "Remove the TPA Support card from the redesign" |
| Pivot | Fundamentally changes the approach or architecture | "Switch from tab removal to accordion collapse" |

**Impact assessment** — determine which artifacts need updating:

| Change affects... | Update |
|-------------------|--------|
| What the feature does (scope, requirements, stories) | spec.md |
| How it's built (architecture, file structure, tech decisions) | plan.md |
| What work items exist (new/modified/removed tasks) | tasks.md |
| Data entities or relationships | data-model.md |
| Test scenarios or integration points | quickstart.md |
| Technical decisions or constraints | research.md |

**Cross-reference with implementation progress:**

- If the change touches completed tasks, flag the risk (completed work may need rework).
- If the change affects the current phase, note which in-progress tasks are impacted.
- If the change adds scope, determine which phase the new work belongs in.

### 6. Present Change Impact Summary

Before writing the pending iteration file, present a concise impact summary to the user:

```markdown
## Iteration: Change Impact Summary

**Change request**: <1-sentence summary of what the user asked>

**Scope**: <Feature-wide | Phase N | Task TXXX | Subtraction | Pivot>

### Implementation Progress

- **Tasks**: N of M complete
- **Current phase**: Phase X
- **Adhoc changes detected**: <count, if any>

### Artifacts to update

| Artifact | Action | Details |
|----------|--------|---------|
| spec.md | Modify | <which sections and why> |
| plan.md | Modify | <what changes> |
| tasks.md | Add/Modify/Remove | <which tasks affected> |

### Risk check

- [ ] No completed tasks (`[x]`) are invalidated by this change
- [ ] No scope boundary violations (change stays within feature intent)
- [ ] No downstream dependency breaks (tasks that depend on modified tasks)

**Write this iteration plan?** (yes / no / adjust)
```

Wait for user confirmation before writing the file.

**If completed tasks are invalidated**: Warn the user explicitly — "Tasks TXXX–TYYY are already marked complete but would be affected by this change. They may need re-implementation after apply." Ask how to proceed.

**If the change is a Pivot**: Warn that this is a significant direction change and recommend running `/speckit.specify` with an updated description instead, unless the user explicitly wants an in-place iteration.

### 7. Write Pending Iteration File

On user confirmation, write `FEATURE_DIR/pending-iteration.md` with the following structure:

```markdown
---
status: pending
created: YYYY-MM-DD
change_request: "<original user input, verbatim>"
scope: "<Feature-wide | Phase N | Task TXXX | Subtraction | Pivot>"
---

## Change Summary

<1-sentence description of what this iteration does>

## Implementation Progress

- **Tasks completed**: TXXX, TYYY (N of M total)
- **Current phase**: Phase X
- **Files changed on branch**: <count>
- **Potential task completions to mark**: TZZZ (mapped from git)
- **Adhoc changes**: <brief note, or "None">

## Impact Assessment

| Artifact | Action | Details |
|----------|--------|---------|
| spec.md | Modify | <sections and why> |
| plan.md | Modify | <what changes> |
| tasks.md | Add/Modify | <tasks affected> |
| data-model.md | No change | — |

## Risk Checks

- [x] No completed tasks invalidated (or: Tasks TXXX affected — user acknowledged)
- [x] No scope boundary violations
- [x] No downstream dependency breaks

## Planned Changes

### spec.md
- <specific change 1: e.g., "Add FR-012 for Reports tab requirement">
- <specific change 2: e.g., "Add user story for report generation">
- <specific change 3: e.g., "Update scope boundaries to include reporting">

### plan.md
- <specific change 1: e.g., "Add ReportsTab component to file structure">
- <specific change 2: e.g., "Add Phase 3 for reporting implementation">

### tasks.md
- <specific change 1: e.g., "Add T014–T016 for Reports tab tasks in Phase 3">
- <specific change 2: e.g., "Mark T005, T008 as complete (confirmed via git)">

### data-model.md
- (No changes)

### quickstart.md
- <specific change, if any>

### research.md
- <specific change, if any>
```

### 8. Report and Next Steps

After writing the file, confirm to the user:

```markdown
## Iteration Defined

**Pending iteration written to**: `FEATURE_DIR/pending-iteration.md`

You can:
- **Review/edit** the file directly before applying
- **Re-run** `/speckit.iterate.define` to regenerate with a different request
- **Apply** with `/speckit.iterate.apply` to update all spec documents
```

## Behavior Rules

- **Never modify spec artifacts** — this command only writes `pending-iteration.md`. All spec changes happen in `/speckit.iterate.apply`.
- **Safe to re-run** — running define again overwrites the pending iteration file (with user confirmation if one exists).
- **Respect user intent** — if the change request is ambiguous, ask one clarifying question before proceeding (do not guess on high-impact decisions).
- **Account for reality** — always check implementation progress before planning changes. An iteration plan that ignores completed work is useless.
- **Preserve completed work by default** — flag risks when completed tasks are affected, but don't refuse the iteration. The user decides.
- **Be specific in planned changes** — the pending iteration file should list concrete changes (add FR-012, modify Phase 2 tasks, etc.), not vague descriptions. This is what `apply` will execute.

Context for iteration: $ARGUMENTS
