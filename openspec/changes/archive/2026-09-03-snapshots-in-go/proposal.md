# The other half of `db`: named copies of the database

## Why

`db` was ported down the middle: the templates, because `worktree` is built on them, and not the
snapshots. That boundary made sense while it lasted — the two families share nothing but the word
— but it leaves `down` unportable, because what `down -v` offers before destroying a database is
exactly a snapshot.

So this is the half that was left, and with it `db` stops being a command with two owners.

## What Changes

- **`db snapshot`, `db list`, `db restore`, `db remove` and `db clear` are Go**, with the same
  text, the same documents and the same refusals — compared against the shell implementation over
  a real MariaDB, including the size string, which is the one thing both have to measure rather
  than read.
- **The copy is still written by the database**, not composed here: `--single-transaction` so the
  project keeps working while it runs, and routines, triggers and events included, because a copy
  that restores a Magento without them is not a copy of that Magento.
- **It is written beside its destination and renamed only when complete.** An interrupted dump
  must not look like a usable snapshot, and the moment somebody reaches for one is the worst
  possible moment to find out.
- **Restoring empties the database first.** Restoring over one that kept living leaves whatever
  was created afterwards in place, and the result is a mixture of the two rather than the copy.
- **Confirming is the project's name typed out**, not a letter. A blind `y` is a reflex; typing
  the name means the sentence was read. Clearing every project asks for a different word again,
  because it is the only thing here that can destroy copies belonging to projects you are not
  standing in.
- **The record of the data having been anonymised is cleared by a restore**, for the same reason
  an import clears it: whatever that copy holds, nobody anonymised it afterwards.

## A defect found while porting it

The exec that captures output merged the two streams and gave up after sixty seconds. Both were
right for what it was written for — a query, whose answer somebody reads — and both are wrong for
a dump: a warning on the error stream would land *inside* the copy, and a real Magento database
takes minutes, so the copy would be cut off in the middle and still look finished. The command
that captures a copy now keeps the streams apart and has no deadline.

It would have been found the day somebody needed a copy, which is the worst day to find it.

## One divergence, on purpose

In JSON mode the shell implementation printed a sentence into the document stream when somebody
answered no. This answers with a document saying nothing was removed. A program reading the output
gets JSON in every ending, which is what the contract says.
