# Tasks

## Registration and resolution

- [x] `console/tasks/worktree_env.sh`: registry paths, read and write, listing
- [x] Resolve a registered worktree against itself in `hm_resolve_project_root` and `bin/run`
- [x] Keep WT-01's refusals for unregistered worktrees

## The overlay

- [x] Generate the per-environment compose overlay: profile, ports reset, proxy routing
- [x] Profiles `lite`, `agent` and `full` as removed services
- [x] Route `<name>.<project domain>` through the global proxy

## The command

- [x] `hm worktree add <branch> [--profile] [--path] [--no-start]`
- [x] Require the proxy, the main checkout, and a name that is free
- [x] Link `vendor/` and `node_modules/` on Linux, copy the code volume on macOS
- [x] Clone the database template when there is one, and say so when there is not
- [x] `hm worktree list [--json]`
- [x] `hm worktree remove <name> [--force]`

## Verification

- [x] Unit tests: registry, profile service sets, name derivation
- [x] Integration test: the resolved configuration of each profile
- [x] Integration test: a registered worktree resolves against itself and is not refused
- [x] Integration test: add, list and remove against real containers
- [x] `docs/worktree.md`, changelog and backlog
