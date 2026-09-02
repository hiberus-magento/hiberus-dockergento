# `hm serve`: the web dashboard, and a defect it uncovered

## Why

The engine became usable as a library so that something other than a terminal could ask it
questions. This is that something: a dashboard that lists the environments on this machine, shows
what each project is and what is wrong with it, and starts or stops it.

It behaves like the proxy, because that is the shape this tool already has for machine-wide
things: you bring it up once, it says where it is, and it stays there. It does not hold a
terminal.

## What Changes

- **`hm serve up | down | status`**, mirroring `hm proxy`. Bare `hm serve` brings it up, prints the
  link and returns — and only once the server actually answers, because a command that returns
  early hands somebody a link that fails on the first try.
- **An HTTP adapter** over the same calls the command line makes. `GET /api/environments` returns
  the document `hm list --json` prints, down to the wrapper, and a test compares them: a dashboard
  that reimplemented "what is running" would answer something slightly different the first time
  either changed.
- **A dashboard page**, embedded in the binary. No build step and nothing to install.
- **Two rules instead of a framework**: it listens on loopback only, and every request needs a
  token that travels in the printed link. This API reads database credentials and stops
  environments, and a page on the internet can point a name at 127.0.0.1 and make a browser talk
  to it — so the `Host` header is checked too.
- **`--port`** for a machine where 8420 is taken, and `--foreground` for debugging.

## The defect this uncovered

A flag before the command name — `hm --no-json describe` — never reached the Go implementation.
It fell through to the shell one, silently, and looked exactly like it had worked.

Including to the tests. Several assertions comparing text output character for character were
comparing the shell implementation against itself and proving nothing. With the flags parsed
properly, two real differences appeared:

- **`describe` was not drawing the rule** above and below the project's name.
- **`logs` coloured its prefixes when the output was not a terminal.** Compose decides that from
  whether the stream is a terminal; calling the same function it does fixes it.

The JSON comparisons were never affected — those put the flag after the command — so what was
verified stays verified. The text ones now are too.
