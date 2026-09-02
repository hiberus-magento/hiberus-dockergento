# Design

## Porting half a command without splitting it

The three subcommands share one thing: the registrations. As long as both implementations read and
write the same files, which half answers is invisible — what one writes the other sees.

That is the difference between this and a split that would be wrong. `mysql -i` was one sequence;
`worktree add` and `worktree list` are two operations that happen to share a word, like `db
freeze` and `db snapshot`.

It also decides the order of what comes next. The registry cannot become the live source until
`add` is ported, because a Go half reading SQLite while a shell half writes JSON is a
disagreement with no good outcome: a branch environment `add` created that nothing else can see,
or a registration `remove` deleted that still exists.

## What `remove` takes, and in what order

The environment first, with its volumes, then the worktree, then a prune, then the registration.

The prune is not tidiness: a worktree whose directory somebody deleted by hand leaves a stale
administrative entry, and git refuses to reuse the name until it is gone — so the next `add` of
the same branch would fail for a reason that has nothing to do with what somebody is doing.

The registration and the overlay go together because they are one fact in two files. An overlay
with no registration is a compose file nothing loads; a registration with no overlay is an
environment that cannot be built.
