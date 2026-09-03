# Tasks

- [x] `console/commands/worktree.sh` becomes a delegator, so bash is no longer a second writer
- [x] The SQLite-backed registrations implement the whole port, draining the old directory
- [x] The engine hands them out, and the diagnostic reads through the engine rather than its own
      database
- [x] Removal clears the row, the overlay and the legacy file
- [x] The bridge hands the registration to the shell implementation, and the shell entry point run
      on its own asks the binary for it
- [x] The worktree tests assert behaviour instead of comparing an implementation with itself
- [x] `MIGRATION.md` updated: the registry is live, and `_registry` has a new condition for dying
