# `worktree list` and `worktree remove` in Go

## Why

`worktree` is the command that will make the SQLite registry the live source, and it is the
biggest one in the tool: 537 lines, three subcommands, and `add` alone touches git, the proxy, the
database templates, the dependency sharing and the anonymiser.

Two of the three do not. `list` reads the registrations and asks Docker what is running; `remove`
takes an environment down, removes the worktree and forgets the registration. Both are tractable,
and both read and write exactly the files the shell implementation's `add` writes — so there is no
moment where the two disagree. That is what makes porting half of it safe.

## What Changes

- **`worktree list` and `worktree remove` are Go**, with the same documents, the same table and
  the same refusals, every one compared against the shell implementation.
- **`add` stays in shell**, and a test says it still gets there.
- **The storage does not change.** The registrations are still the JSON files beside the
  worktrees. Swapping the adapter for the SQLite registry is its own change, in one move, once
  `add` is ported too — because a Go half reading SQLite while a shell half writes JSON is exactly
  the disagreement this avoided.

## What is kept, and why it is worth reading twice

- **The state comes from the containers and the directory, not from the registration.** A registry
  that answered "running" about something somebody stopped by hand would be worse than not asking.
- **A directory somebody deleted by hand is reported as missing**, and the tidy path is named:
  `remove` needs the directory, and what collects a registration without one is `clean`.
- **Uncommitted changes refuse the removal.** Containers and databases can be rebuilt in seconds;
  uncommitted code cannot be rebuilt at all, so git's own refusal is repeated rather than worked
  around.
- **The environment goes down with its volumes.** The database a branch was given was a copy, and
  leaving it behind is how a machine fills up with the data of branches nobody works on any more.
