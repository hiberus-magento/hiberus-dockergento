# Design

## No command is ported here

That is the discipline of a strangler and it is worth stating, because the temptation is to port
one "to prove it works". Proving it works is what the comparison test does: the two
implementations answer the same question and a test compares them.

What this stage owns is the shape — where things live, what depends on what, and the bridge — so
that porting a command later is a small, boring change rather than an argument about structure.

## No dependencies yet, not even the obvious one

Cobra is the obvious choice and it is not here. For a bridge it brings nothing: the arguments are
passed through untouched, and its help would *replace* the shell implementation's — which is
exactly the thing that must not change while the team is using both. It comes in when the first
real command lands and there is something to build a help text for.

The same for a Docker library: nothing here talks to Docker.

## The hexagon, kept honest by the tests

`internal/core` imports nothing of ours and prints nothing. `internal/app` is written against
interfaces. The adapters are the only place that knows about git, the filesystem or the shell
implementation.

The proof that it is worth the arrangement is in the tests: resolving a project — including the
worktree cases that the shell version could only exercise against a real checkout, with a real
git repository and a registered environment — runs in microseconds against fakes. That is what
makes it possible to test the parts of 2.0 that do not exist yet: reconciliation, the registry,
the collection of what is abandoned.

## The bridge is permanent

It is not scaffolding. A project can add commands of its own under `config/hm/commands`, and
those will always be shell. The port stays in the domain for that reason, and the long tail of
commands can stay where it is for as long as it makes sense.

## Two names that cannot collide

`hm-go-version` and `hm-go-project` are prefixed on purpose. A command called `version` or
`project` would shadow, now or later, one the shell implementation has or might have — and the
whole point of this stage is that nothing the user runs behaves differently.
