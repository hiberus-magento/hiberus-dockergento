# Design

## Describing a checkout is a question for git

It stays a subprocess, and so does the Compose version: Compose is a CLI plugin, so its version is
not something the daemon can be asked. The daemon's own version is asked of the daemon, which is
one fewer process and the answer that matters when the two disagree.

## Where the newline goes

Git prints a line, and the helper that runs it returns what git printed. Trimming inside that
helper would break the callers that split its output into lines — listing tracked files is one —
so the answers that are a single line are trimmed where they are read. The alternative is a JSON
document with `"branch": "release/2.0.0\n"` in it, which is what happened first.
