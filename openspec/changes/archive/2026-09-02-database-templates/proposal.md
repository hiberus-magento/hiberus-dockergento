# Database templates in Go: the half `worktree` is built on

## Why

`db` is 709 lines and two families that only share a name. **Snapshots** are dumps in a file:
portable, small, readable a year and two server versions later. **Templates** are byte copies of a
data directory in a volume: not portable at all, and available again in seconds instead of the
tens of minutes an import of the same data costs.

`worktree` — the command that makes the registry the live source, and the one this batch is
heading for — is built on the second. An environment per branch is only affordable because its
data is a file copy rather than an import.

So the templates are what gets ported, and the snapshots stay where they are for now. That
boundary is between two independent operations, not down the middle of one: `hm db freeze` and
`hm db snapshot` share a namespace and nothing else.

## What Changes

- **`db freeze`, `db templates`, `db clone` and `db drop` are Go**, with the same documents, the
  same tables and the same refusals — every one of them compared against the shell implementation.
- **Three new capabilities in the engine**: the Docker volumes, a one-off container with volumes
  attached, and the size of a data directory measured from inside one — because on macOS the
  volume lives in a virtual machine and its mountpoint does not exist out here.
- **The guardrails are kept**, and they are the reason this command is worth reading twice: the
  data directory of a running server is not replaced, a template made by another database version
  is refused, and replacing a database that has data in it asks for the project's name rather than
  a letter.

## What the spec says

Nothing new about templates. What they do was specified when they were built, in 1.7, and none of
it changes here — every document, table and refusal is compared against the shell implementation
by a test. What is new is the routing: a command answered by either implementation depending on
its subcommand, which is how a 709-line command gets ported at all.
