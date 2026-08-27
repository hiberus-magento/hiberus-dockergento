# Clone the database instead of importing it again

## Why

Every isolated environment needs a database, and today the only way to get one is to import a
dump: minutes of `mysql` parsing SQL for data that already exists, byte for byte, in a volume on
the same disk. On a real project the data directory is a few hundred megabytes — a copy of a few
seconds — while importing the equivalent dump takes tens of minutes.

That difference decides whether per-branch environments are practical. `hm worktree` (WT-02) is
supposed to give a branch its own environment on demand; if standing one up costs half an hour of
`setup:upgrade` and imports, nobody will do it twice.

Snapshots (`hm db snapshot`) already cover the other need — a portable copy that survives
`down -v` and can be handed to somebody else. A dump is the right shape for that: small, text,
version independent. It is the wrong shape for "give me this database again, now."

## What Changes

- **`hm db freeze`** saves the project's data directory as a reusable template: a Docker volume
  copied from the live one, recorded with the database image it came from.
- **`hm db clone`** builds this project's data directory from a template, in seconds.
- **`hm db templates`** lists what exists, with size, origin and age.
- **`hm db drop`** deletes a template.
- Templates are addressed as `<project>/<name>`, so an environment derived from a project — a
  worktree, a second checkout — can clone the parent's template by naming it.
- A template records the database image that produced it, and cloning refuses to put it under a
  different one, because a data directory is not portable across server versions.
