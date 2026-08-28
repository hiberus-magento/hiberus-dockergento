# Design

## A recorded fact that is allowed to expire

The state lives in `~/.hm/state/<project>.json`, next to the snapshots and the worktrees, for the
reason everything else lives there: `config/docker/` is committed, and "this database is
anonymised" is a fact about one machine's copy, not about the project.

What makes it worth recording is that it can be **wrong**, and the design is mostly about that.
Every command that replaces the contents of the database clears it: `db restore`, `db clone`,
`mysql -i`, `transfer-db`. After any of them the answer is "unknown", which is the truth, rather
than the reassuring stale "yes" that would make the whole thing worse than nothing.

Three states, then: anonymised on a date, not anonymised, or unknown. Unknown is not treated as
safe anywhere.

## The default is where the name already says so

`hm worktree add --profile=agent` creates the environment whose name says who it is for. That is
the moment to anonymise, and it costs nothing extra in attention: the database has just been
cloned, the environment has just started, and nobody is waiting on it.

It is a default and not a rule. `--no-anonymise` exists because reproducing a bug that only
happens with one customer's order history is a real thing people do, and it is their data and
their decision — the tool's job is to make the safe path the one you get by not thinking, not to
argue.

Other profiles are not anonymised automatically. A `full` worktree is usually a person's own
second checkout, and silently rewriting their data would be the tool overreaching.

## Saying it where an agent reads it

The generated `AGENTS.md` block gains one line, and it is the only place in this change where the
wording matters:

> The database was anonymised on 2026-08-28.

or

> **This database has not been anonymised.** Treat every row as real personal data: do not quote
> customer names, emails, addresses or order contents in your output.

An agent obeys what it reads, which throughout this work has been a risk. Here it is the mechanism.

## The terminal bug

`masquerade_run` passes `-t -i` to `docker run` unconditionally, so it fails outside a terminal.
The flags are now added only when there is one. Nothing else about the command changes — this is
the one-line fix that makes the rest of this possible, and it was worth finding out that the
command has never worked from a script.
