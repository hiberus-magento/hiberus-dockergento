# The registry: one file, both topologies, and slots that are handed out atomically

## Why

What this machine knows about its projects lives in a directory of small JSON files, one per
worktree and one per project's anonymisation state, with a lock made out of `mkdir` around them.
It works for what it does today. It cannot do what the next batch of commands needs.

Two things it cannot do. A slot has to be handed out atomically — two agents registering a
worktree at the same moment must not be given the same database schema — and removing a worktree
has to take its allocation with it in the same breath, or a project runs out of schemas it is not
using. A lock around a directory can be made to do both; a transaction already is one.

And the shape has to be right before the commands are ported. The two topologies must be modelled
from the first day: adding the orchestrated one later means migrating the state of everyone
already using the tool, which is the one kind of change that cannot be rolled back quietly.

## What Changes

- **A SQLite registry** at `~/.hm/hm.db`: projects, branch environments, the anonymisation state of
  each environment, and the allocations of the orchestrated topology. Pure Go, no cgo, so the
  Linux binaries stay cross-compilable.
- **Both topologies modelled**, with the allocations table empty in `classic` and there from the
  start.
- **Slots handed out in a transaction**, reused when a worktree goes, and bounded by what Redis
  actually has: three databases per worktree, so 128 of them is 42 worktrees.
- **What the shell implementation wrote is brought across**, idempotently, skipping a record it
  cannot read rather than abandoning the rest.
- **`hm-go-registry`**, a diagnostic like the other two, so the registry can be looked at while the
  commands that will own it are still shell.

No command changes. The registry is not the live source yet: `worktree` is still the shell
implementation's and still writes JSON, and two writers disagreeing is worse than one writer being
old. That switch happens in one move when `worktree` is ported.

## What it costs

The published binary goes from 60.6 MB to 64.5 MB.
