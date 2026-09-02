# Design

## Intention, not observation

The registry holds what somebody decided: that this worktree is on the agent profile, that it
reads the main checkout's dependencies, that it was given schema seven. It does not hold what
exists — which containers are there is asked of Docker, because a registry goes on claiming an
environment exists after somebody removed it by hand.

That line is why the inventory is still built from container labels and always will be. The
registry answers the questions Docker cannot.

## Why a database and not better files

Two operations decide it, and both are about more than one row at a time.

**Handing out a slot.** The lowest free number, then a write. Two agents doing that at once with a
file each read the same number and both take it, and the symptom is two branches sharing a
database. It is not a rare race: it is what happens the first time somebody runs two agents in
parallel. There is a test that runs twelve at once and checks that twelve different slots come
back.

**Removing a worktree.** Its registration and its allocation have to go together. A foreign key
does that in one statement; two files need a lock and a promise.

## The one thing SQLite makes you learn

Write-ahead logging lets a reader and a writer work at once, which is what several agents are. But
a transaction that begins by reading and then writes has to be rejected outright when another
writer committed in between — the snapshot it read is already stale, and the busy timeout does not
cover it because there is nothing to wait for.

The concurrency test failed on exactly that, with `database is locked`, before anything was wired
anywhere. The fix is to take the write lock at the start of the transaction rather than when the
first write arrives, which turns the rejection into a wait.

## Both topologies, one schema

A project carries its topology. A worktree in an orchestrated project gets a row in `allocations`;
in a classic one it gets nothing, and the table stays empty. Everything in an allocation is derived
from the slot rather than stored field by field, so two worktrees cannot end up with the same
schema through a bad write, and the derivation is pure and tested without a database.

The bound comes from Redis and nothing else: Magento keeps the cache, the page cache and the
sessions apart, so three databases per worktree, and every other service isolates by a name that
does not run out.

## Bringing across what is already there

This runs on machines with worktrees people are working in. So the import is idempotent — a
registration already there is updated, not duplicated — and one unreadable record is skipped rather
than abandoning the other twenty.

One thing it cannot know: the JSON records where a worktree is, but not where its project is. So a
project imported that way has an empty root, and learns it the first time a command runs in it —
which is why saving a project never blanks a root already recorded.
