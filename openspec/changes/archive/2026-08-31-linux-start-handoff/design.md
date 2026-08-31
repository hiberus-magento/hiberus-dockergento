# Design

## A hand-off, not a fork

The Go implementation does the Compose part and calls back into the shell for the piece that is
not ported. That is the same shape already used for the global proxy and for stopping every other
environment, and it is what a strangler is supposed to look like: the boundary is one call, in one
place, and it disappears when what is behind it moves.

What it replaces is worse than it looks. A command implemented twice does not stay the same
command — it stays the same name. Everything added to `start` since it was ported reached one
platform.

`post-start` guards on the platform itself, so the shell `start` and the Go one call the same
thing unconditionally and neither has to know which steps exist. Go does check before calling,
for one reason: on macOS there is nothing to do, and starting a shell to find that out would cost
a fifth of the command.

## What stays in shell, and why that is not laziness

Matching the ids could be ported today: it is an exec into two containers, comparing what
`getent passwd` says with the caller's own id, and a `usermod` plus a recursive `chown` when they
differ.

The other one cannot. Writing the project's domains into the container's `/etc/hosts` means
knowing what those domains are, and they are read out of `core_config_data` through `hm mysql` —
which is in a later batch. Porting half of `post-start` would leave the command split across two
implementations to save nothing.

## The Hitch defect

The entries point at the TLS terminator's address, and a project on the global proxy has no TLS
terminator: the overlay deletes it on purpose, since the proxy terminates TLS itself.

The step called `is_run_service "hitch"`, which fails with the service exit code, and `start` runs
with `set -e`. So the environment came up and the command then reported a failure naming a service
the project does not have and should not have.

With no terminator there is nothing to point the entries at, so they are skipped — and said out
loud rather than silently, because an environment that cannot resolve its own domain from inside
is a thing somebody will otherwise debug from scratch.
