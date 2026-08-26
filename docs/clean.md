# clean

Collects what environments that no longer exist left behind.

```bash
hm clean            # shows what could be collected, deletes nothing
hm clean --force    # deletes it, after listing it and asking
```

Does not need to run inside a project.

## Looking is the default

`hm clean` on its own **never deletes anything**. That is the behaviour, not an option you have to
remember: a `--dry-run` you must type protects the people who were already being careful, and the
danger is with whoever types the short form without reading the documentation.

## What it will touch

An environment is collectable only when **both** are true:

1. It carries this tool's `hm.*` labels — we made it.
2. The directory it was created from **no longer exists**.

A stopped project whose directory is still there is not rubbish; it is a stopped project. That is
the distinction `docker system prune` cannot make, and the reason this command exists.

## What it will not, and why

**Volumes carry no `hm.*` labels** — only the ones Compose adds. So a volume can only be attributed
through the containers of its project, and where those are gone, it cannot be attributed at all:
it could belong to a Compose stack somebody wrote by hand.

Those are listed separately, by name, and are **never deleted, not even with `--force`**:

```console
Cannot be attributed, so they are left alone

  Volumes carry no hm labels, so a project with no containers left could
  belong to anything. These are yours to judge:

  old-project_dbdata
  old-project_workspace
```

Deleting them is a decision for a person with a `docker volume rm`. We would rather leave rubbish
than delete somebody's data.

**Database snapshots are never part of a clean.** They live in `~/.hm/snapshots/` and are the last
thing anyone would want removed by a tidy-up. Use [`hm db clear`](db.md) for those.

**Images and build cache are not touched either.** They are shared between projects and expensive
to download again. `hm clean` reports how much they take and which Docker command clears them —
without running it. A tool that runs `prune` on your behalf is precisely the mistake this command
exists to avoid.

## Deleting

`--force` means "I want to delete", not "do not ask me". The list still appears, the space to be
freed is worked out, and there is still a confirmation:

```console
$ hm clean --force

Environments whose directory is gone

  old-project                  was at /Users/someone/projects/old-project

  1 container group(s), 7 volume(s)

Working out how much space this frees...

This deletes 1 environment(s) and 7 volume(s).
Their database snapshots are not touched.

Delete them? [y/N]:
```

Sizes are only computed on this path: working them out takes around 25 seconds on a machine with a
hundred volumes, which is too long for a command you run just to look.

`hm --yes clean --force` skips the question, for scripts.
