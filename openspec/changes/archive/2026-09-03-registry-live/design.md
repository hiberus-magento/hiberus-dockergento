# Design

## Draining, not migrating

There is no migration step, no version marker and nothing to run. Every read of the registry looks
at the old directory first and brings across whatever is still there. It is idempotent — a
registration already recorded is updated rather than duplicated — and it costs one directory
listing when there is nothing, which is the case on every machine after the first command.

The alternative was a one-off migration on upgrade. It is worse for the same reason a migration is
always worse here: it happens once, at a moment nobody chose, and if it half-fails the machine is
left in a state neither implementation understands. Draining has no such moment.

## Removing has to be symmetric

The read path is what makes removal delicate. If `Forget` cleared the row and left the JSON, the
next listing would drain it back in and the worktree would return from the dead — with its
containers gone, so it would show as `missing`, which is worse than either being there or not.

So `Forget` removes the row, the overlay, and the legacy file, and takes the project's directory
with it when nothing is left in it.

## The half that cannot read a database

Two ways in, and neither is a second registry.

When the binary bridges a command it has already resolved the project, so it hands the
registration over as environment entries. Nothing is looked up twice and no process is started.

When the shell entry point is run directly — a machine installing the fallback, somebody calling
`bin/run` — there is no handover, so it asks the binary. One process, only inside a branch
environment, and only when nothing was handed over.

What is *not* supported is writing a registration by hand and expecting it to stay a file. It is
read once, brought in, and removed.

## Where the database lives

Beside everything else this machine records, and it follows the state directory when that is
redirected. Tests redirect it; without that a test that registers branch environments would leave
them in the registry the machine actually uses, and every later command would list them.

## What the fallback is for

If the database cannot be opened — a read-only home, a disk with nothing left on it — the engine
falls back to reading the directory rather than answering nothing. A registry that cannot be
opened is not a reason to refuse to describe a project.
