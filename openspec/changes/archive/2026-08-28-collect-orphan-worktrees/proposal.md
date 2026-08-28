# Collect the branch environments whose worktree is gone

## Why

`hm worktree remove` is the tidy way to get rid of a branch environment, and it needs the
directory to still be there. People do not always take the tidy way: they run `git worktree
remove` themselves, or delete the folder, or move the project.

The containers and volumes of one of those are already collected — they carry `hm.root`, and
`hm clean` collects what has no directory left. What is not collected is the registration in
`~/.hm/worktrees`, which nothing else deletes. It shows up in `hm worktree list` as `missing` for
ever, and it refuses the name if somebody wants that branch environment back.

## What Changes

- **`hm clean` lists branch environments whose worktree is gone**, alongside the environments it
  already lists, and collects them with `--force`: containers, volumes and the registration.
- Their containers and volumes are removed **by name**, because the directory that held the
  compose configuration is exactly what is missing.
- **`hm worktree list` points at it** when it has something to say `missing` about, rather than
  reporting a state with no way out of it.
