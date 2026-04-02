# Changelog

## [2.0.0] - 2026-03-17

### Added

- `speckit.iterate.define` command — analyze a change request against current spec state and implementation progress, write a reviewable `pending-iteration.md` plan
- Two-phase define-and-apply workflow with a persistent, editable checkpoint between phases
- Implementation progress detection (git diffs + task states) built into the define phase
- Task completion marking from git evidence during apply

### Changed

- `speckit.iterate.apply` now reads from `pending-iteration.md` instead of analyzing from scratch — enforces the two-phase workflow
- `apply` updates all spec artifacts (spec.md, plan.md, tasks.md, data-model.md, etc.) so users can skip `speckit.plan` and `speckit.tasks` and go straight to `speckit.implement`
- Primary handoff from `apply` is now `speckit.implement` instead of `speckit.iterate.sync`
- Iteration log in spec.md now includes task completion data (absorbed from the old sync log format)

### Removed

- `speckit.iterate.sync` command — task completion detection folded into `apply`; standalone drift reconciliation removed
- `speckit.tasks` removed from required commands (no longer needed as a dependency)

## [1.0.0] - 2026-03-04

### Added

- `speckit.iterate.apply` command — apply change requests to spec documents with scope classification, impact summaries, and cross-artifact consistency validation
- `speckit.iterate.sync` command — detect drift between code and documentation and update specs to match reality
- Handoff support to core commands (`speckit.tasks`, `speckit.analyze`, `speckit.implement`)
