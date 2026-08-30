# The binary takes over, and nothing changes

## Why

The migration to Go is a strangler and not a rewrite: the binary becomes the entry point and runs
the shell implementation for everything not ported yet. This is the stage where that becomes
true, and its whole measure of success is that **the team installs it and notices nothing**.

Doing it first, before any command is ported, is what makes the rest safe. Every command that
moves afterwards moves against a working bridge, with the shell version one line away as the
reference — and with a test comparing the two answers rather than somebody reading both.

## What Changes

- A Go module with the domain, the ports and the use cases separated from the adapters, so that
  the CLI, the MCP server and later an HTTP API are three ways into the same logic.
- **The bridge**: anything the binary does not implement runs the shell CLI with the arguments
  untouched, wired to the same terminal, returning the same exit code.
- **The project resolution in Go** — root, worktree, properties — with the order the shell version
  got wrong, and tested against the shell version's answer.
- Two commands the shell implementation does not have, prefixed so they can never collide with a
  real one: `hm-go-version` says which binary this is, and `hm-go-project` prints what the Go
  layer resolved. They exist so the layer can be checked without changing what any real command
  does.
- **CI from the first commit**: format, `vet`, the Go tests and the shell unit suite.
- `MIGRATION.md`, which says where the migration is and how to carry on, with a test that keeps it
  honest.
