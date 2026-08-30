# Orchestration in Go: Compose as a library

## Why

Everything the tool does to an environment goes through Compose, and until now it went through the
`docker compose` *command*: start a process, have it re-read the files this tool had already read,
and find out what happened by reading a terminal.

`github.com/docker/compose/v2` is the same engine, importable. The reason not to use it was
written down as ADR-009 and it was a good reason: if what we create is not what `docker compose`
creates — the labels, the configuration hash — then the two implementations recreate each other's
containers on every command, and nothing says so until somebody loses a database.

That risk turned out to be measurable, so it was measured before anything was written. An
environment created through the library and one created by the command carry the same
configuration hash; neither recreates the other's containers; `docker compose ps` sees both. The
reason for the decision is gone, so the decision changes.

## What Changes

- **`start`, `stop`, `restart`, `logs` and `exec` are Go**, driving Compose in this process.
  Bringing an environment up goes from 555 ms to 265 ms, and over an environment already up from
  320 ms to 95 ms.
- **The compose file list is corrected**, which was wrong before and would have made `start`
  dangerous: a project routed through the global proxy carries a third file that removes its
  published ports, and a branch environment carries an overlay that lives outside the checkout.
  The Go layer loaded neither, so `describe` reported a proxied project as publishing ports it
  does not publish, and `start` in a branch environment would have recreated the main one.
- **The worktree registry is read by the Go layer**, which is what makes the above possible: until
  now it reported that no worktree ever had an environment of its own.
- **`logs` enumerates its options** instead of forwarding them, and says which services this
  project has when asked for one it does not.
- **The output stays the output people know**: Compose's own progress writer and its own log
  consumer, so the prefixes, the colours and the wording are not reimplemented slightly
  differently.
- **The price is the binary**: 8.5 MB to 60.6 MB published, and 70 modules to 426.
