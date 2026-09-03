# The registry becomes the live source

## Why

The registry has been a directory of small JSON files since it existed, and a SQLite store beside
it that nothing read: the schema, the slots and the import were built first so that the swap would
be a change of adapter and not a change of behaviour. What stood in the way was that two
implementations wrote those files — the ported half and `console/commands/worktree.sh` — and a
registry with two writers cannot move without one of them being left behind writing to the old
place.

Porting the three subcommands removed the reason for the second writer. This removes the writer.

## What Changes

- **`console/commands/worktree.sh` delegates**, the way `web` already does: it hands the arguments
  to the binary and reports plainly when there is none. 537 lines become about 25, and the command
  has one implementation.
- **The adapter changes underneath everything at once.** The engine hands out the SQLite-backed
  registrations, so the CLI, the HTTP API and the diagnostic all read the same rows. That last one
  matters more than it looks: a diagnostic that reads a different database from the one the
  commands use is a diagnostic that lies.
- **What the 1.x wrote is carried across, not abandoned.** The old directory is read on the way in
  — cheap, one listing — and a registration found there is brought in rather than duplicated. A
  machine with branch environments running keeps them, without anybody being asked to migrate.
- **Forgetting a worktree clears both.** The row, the overlay and the file the old implementation
  wrote. Leaving that last one would bring the environment back on the next listing, because that
  file is exactly what is read on the way in.
- **The shell half reads the registry through the binary.** It cannot open the database, and a
  bridged command run from a branch environment that found no registration would resolve to the
  main environment — which is the case WT-01 exists to refuse, with the main environment's mounts
  repointed and its database dropped by a `setup:upgrade` meant for a branch. So the binary hands
  the registration over in the environment when it bridges, and the shell entry point run on its
  own asks the binary for it.
- **The overlay stays a file**, in the same place with the same name. It is a compose file that
  Docker reads, and a compose file in a database is one nothing can load.

## What this does not change

No command answers differently. That is the point, and it is what the tests were rewritten to say:
the ones that compared the two implementations were comparing an implementation with itself once
the shell one became a delegator, so they now assert the behaviour instead — with the comparisons
they replace still in the history, which is why the registration and the overlay this writes are
the ones the shell implementation wrote.
