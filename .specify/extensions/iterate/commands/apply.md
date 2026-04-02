---
description: Apply a pending iteration to spec documents — update all artifacts that speckit.implement relies on, then hand off to implementation.
handoffs:
  - label: Continue Implementation
    agent: speckit.implement
    prompt: Continue implementing tasks with the updated spec documents
    send: true
  - label: Analyze For Consistency
    agent: speckit.analyze
    prompt: Run a project analysis for consistency after the iteration
    send: true
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

If the user provides arguments, treat them as adjustments or notes for the apply process (e.g., "skip task completion marking" or "only update spec.md and tasks.md"). The primary source of truth for what to change is `pending-iteration.md`.

## Outline

Goal: Execute the iteration plan defined in `pending-iteration.md` by updating all spec artifacts that `speckit.implement` relies on. After apply completes, the user can go directly to `/speckit.implement` — skipping `/speckit.plan` and `/speckit.tasks` since this command already handles those updates.

### 1. Initialize Feature Context

Run `.specify/scripts/bash/check-prerequisites.sh --json --paths-only` from repo root **once**. Parse JSON payload fields:

- `FEATURE_DIR`
- `FEATURE_SPEC`
- (Optionally capture `IMPL_PLAN`, `TASKS` for downstream use.)

If JSON parsing fails, abort and instruct the user to run `/speckit.specify` or verify the feature branch environment.

For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

### 2. Load Pending Iteration

Check for `FEATURE_DIR/pending-iteration.md`. If it does **not** exist, abort with:

> No pending iteration found. Run `/speckit.iterate.define <change request>` first to define what you want to change.

If it exists, read and parse the file:

- **Frontmatter**: `status`, `created`, `change_request`, `scope`
- **Change Summary**: What this iteration does
- **Implementation Progress**: Current state context
- **Impact Assessment**: Which artifacts need updates and how
- **Risk Checks**: Acknowledged risks
- **Planned Changes**: Specific changes per artifact

If `status` is not `pending`, warn the user that this iteration may have already been applied.

### 3. Load Current Artifacts

Read all available artifacts from FEATURE_DIR:

- **REQUIRED**: `spec.md` — requirements, user stories, edge cases, scope
- **IF EXISTS**: `plan.md` — tech stack, architecture, file structure, phases
- **IF EXISTS**: `tasks.md` — task list with IDs, phases, checkboxes, dependencies
- **IF EXISTS**: `data-model.md` — entities, attributes, relationships
- **IF EXISTS**: `research.md` — technical decisions and constraints
- **IF EXISTS**: `quickstart.md` — integration/test scenarios
- **IF EXISTS**: `checklists/` — quality checklists

Build an internal model of the feature's current state.

### 4. Confirm Before Applying

Present a brief confirmation to the user:

```markdown
## Ready to Apply Iteration

**Change**: <change_request from pending-iteration.md>
**Scope**: <scope from pending-iteration.md>
**Artifacts to update**: <list from impact assessment>
**Defined on**: <created date>

**Apply now?** (yes / no)
```

Wait for user confirmation. If the user says no, suggest they edit `pending-iteration.md` or re-run `/speckit.iterate.define`.

### 5. Apply Changes to Artifacts

Execute the planned changes from `pending-iteration.md`. Update each affected artifact **in order of dependency** to maintain consistency:

**Order**: spec.md → data-model.md → plan.md → tasks.md → quickstart.md → research.md

For each artifact, follow the specific changes listed in the `## Planned Changes` section of the pending iteration file. Use the guidance below for each artifact type:

#### spec.md updates

- Add, modify, or remove requirements (FR-XXX) preserving the existing numbering scheme. When adding, use the next available number.
- Add, modify, or remove user stories. Preserve priority labels (P1, P2, etc.).
- Update edge cases if the change introduces new boundary conditions.
- Update scope boundaries (In scope / Out of scope) if the change adjusts what's included.
- Update assumptions if the change invalidates or adds new ones.
- Update success criteria if measurable outcomes change.

#### data-model.md updates (if affected)

- Add, modify, or remove entities/attributes/relationships.
- Preserve existing formatting and structure.

#### plan.md updates (if affected)

- Update architecture, file structure, or phase descriptions.
- If new files or components are introduced, add them to the project structure.
- Update phase descriptions if the change shifts work between phases.

#### tasks.md updates (if affected)

- **Adding tasks**: Assign the next available Task ID (TXXX). Place in the correct phase. Add `[P]` and `[Story]` markers following existing conventions. Use checkbox format: `- [ ] TXXX ...`
- **Modifying tasks**: Update the description in place. Preserve the Task ID and checkbox state.
- **Removing tasks**: Do NOT renumber — IDs are stable references. Strike through with a note: `~~TXXX~~ (removed in iteration YYYY-MM-DD)`.
- **Re-scoping phases**: If tasks move between phases, update both the source and target phase sections.
- Update the Dependencies & Execution Order section if task relationships change.
- Update the Implementation Strategy summary table if phase composition changes.

#### quickstart.md updates (if affected)

- Add or modify test/integration scenarios to cover the new scope.

#### research.md updates (if affected)

- Document any new technical decisions or constraints introduced by the change.

**Save each artifact immediately after updating it.**

### 6. Mark Task Completions from Git

Using the implementation progress data from `pending-iteration.md` (the "Potential task completions to mark" field), verify and mark completed tasks:

- For each task listed as a potential completion, confirm the mapped files exist in git changes.
- Change `- [ ]` to `- [x]` for confirmed completions in `tasks.md`.
- If a task was listed but cannot be confirmed, leave it as `[ ]` and note it in the completion report.

This step is **supplementary** — if no task completions are detected, skip it cleanly.

### 7. Add Iteration Log Entry

Add an entry under a `## Iterations` section in `spec.md` (create the section if missing, place after Clarifications):

```markdown
### Iteration YYYY-MM-DD: <short title>

**Change**: <1-sentence description from pending-iteration.md>
**Scope**: <scope classification>
**Artifacts updated**: <list of artifacts that were modified>
**Tasks added**: TXXX, TYYY (if any)
**Tasks removed**: TZZZ (if any)
**Tasks marked complete**: TAAA, TBBB (if any, from git detection)
```

### 8. Cross-Artifact Consistency Validation

After all updates, perform a quick consistency scan:

- Every requirement in spec.md should have at least one task in tasks.md that addresses it (warn if not — don't auto-generate tasks).
- Every task in tasks.md should reference files/components that exist in plan.md's structure (warn on orphans).
- No contradictory statements across artifacts (e.g., spec says "add tab" but tasks still reference removing it).
- Task dependency graph has no cycles and respects phase ordering.

### 9. Clean Up

Delete `FEATURE_DIR/pending-iteration.md` to prevent accidental re-application.

### 10. Report Completion

Output a structured summary:

```markdown
## Iteration Applied

**Change applied**: <1-sentence summary>
**Scope**: <scope classification>
**Date**: YYYY-MM-DD

### Artifacts Updated

| Artifact | Sections Changed |
|----------|-----------------|
| spec.md | <list> |
| plan.md | <list> |
| tasks.md | <list> |

### Task Changes

| Action | Task IDs | Details |
|--------|----------|---------|
| Added | TXXX, TYYY | <brief description> |
| Modified | TZZZ | <what changed> |
| Removed | — | — |
| Marked complete | TAAA | <confirmed from git> |

### Consistency Warnings (if any)

- <warning 1>
- <warning 2>

### Next Steps

Your spec documents are updated and ready for implementation:

- `/speckit.implement` — continue implementation with the updated specs (recommended)
- `/speckit.analyze` — verify cross-artifact consistency before implementing
- `/speckit.iterate.define` — define another iteration if more changes are needed
```

## Behavior Rules

- **Require a pending iteration** — never apply changes without `pending-iteration.md`. If the user wants a single-step workflow, they should run `define` first.
- **Follow the plan** — execute the specific changes listed in `pending-iteration.md`. Do not add, skip, or reinterpret changes beyond what the plan specifies (unless the user provides adjustments via `$ARGUMENTS`).
- **Preserve completed work** — never silently remove or modify completed tasks (`[x]`). If the iteration plan acknowledged risks to completed tasks, follow through as planned.
- **Stable IDs** — never renumber existing Task IDs or Requirement IDs. Append new ones at the end of the sequence.
- **Minimal edits** — only touch sections directly affected by the planned changes. Do not reformat or restructure unrelated content.
- **No implementation** — this command updates documentation only. It does not write application code.
- **Atomic saves** — save each artifact immediately after updating it to minimize context loss.
- **Clean up after success** — always delete `pending-iteration.md` after a successful apply.
- **Respect user intent** — if something in the pending iteration file is unclear or seems wrong, ask before proceeding rather than guessing.

Context for iteration: $ARGUMENTS
