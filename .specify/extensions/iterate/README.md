# spec-kit-iterate

A [Spec Kit](https://github.com/github/spec-kit) extension for iterating on spec documents mid-implementation. Define what you want to change, review the plan, apply it to all your spec docs, and go straight back to building.

## What it does

Spec Kit's core workflow (`specify → plan → tasks → implement`) doesn't have a built-in way to refine scope once implementation has started. This extension fills that gap with a two-phase **define → apply** workflow that updates all the spec artifacts `speckit.implement` relies on.

| Command | Description |
|---------|-------------|
| `/speckit.iterate.define` | Analyze a change request against current spec state and implementation progress, then write a reviewable iteration plan |
| `/speckit.iterate.apply` | Apply the pending iteration to all spec documents — then hand off to `speckit.implement` |

## Workflow

```
/speckit.iterate.define Add a Reports tab
   ↓  analyzes specs + implementation progress
   ↓  writes pending-iteration.md for review

(optional: edit pending-iteration.md by hand)

/speckit.iterate.apply
   ↓  updates spec.md, plan.md, tasks.md, etc.
   ↓  marks completed tasks from git
   ↓  adds iteration log entry
   ↓  deletes pending-iteration.md

/speckit.implement
   ↓  continues implementation with updated docs
```

The key insight: after `apply`, you skip `/speckit.plan` and `/speckit.tasks` entirely — `apply` already updated those artifacts to reflect the iteration.

## Requirements

- [Spec Kit](https://github.com/github/spec-kit) >= 0.1.0
- Core commands: `speckit.analyze`, `speckit.implement`

## Installation

### From the community catalog

```bash
specify extension add iterate
```

Or install from the repository directly:

```bash
specify extension add iterate --from https://github.com/imviancagrace/spec-kit-iterate/archive/refs/tags/v2.0.0.zip
```

### From a local clone

```bash
git clone https://github.com/imviancagrace/spec-kit-iterate.git
cd /path/to/your-speckit-project
specify extension add --dev /path/to/spec-kit-iterate
```

After installation, verify:

```bash
specify extension list

# Should show:
#  ✓ Iterate (v2.0.0)
#     Iterate on spec documents with a two-phase define-and-apply workflow
#     Commands: 2 | Hooks: 0 | Status: Enabled
```

## Usage

### Defining an iteration

```
/speckit.iterate.define Remove the TPA Support card from the redesign
```

The command will:
- Load all spec artifacts and check implementation progress (task states + git diffs)
- Classify the change scope (feature-wide, phase-level, task-level, subtraction, or pivot)
- Present an impact summary with risk checks
- Write `pending-iteration.md` to your feature directory for review

You can edit `pending-iteration.md` by hand before applying — it's structured markdown.

### Applying an iteration

```
/speckit.iterate.apply
```

No arguments needed — the command reads from `pending-iteration.md`. It will:
- Update artifacts in dependency order: spec.md → data-model.md → plan.md → tasks.md → quickstart.md → research.md
- Mark tasks as complete when confirmed by git evidence
- Add an iteration log entry to spec.md
- Run a cross-artifact consistency validation
- Delete `pending-iteration.md` after success
- Hand off to `/speckit.implement` so you can continue building

## License

MIT
