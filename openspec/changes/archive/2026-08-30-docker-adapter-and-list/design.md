# Design

## Why `list` first

Because a port has to be *checked*, and checking means running both and comparing. That rules out
starting with anything that changes something: two `hm start`s cannot be compared, only run.

`list` reads. It reads the one thing the whole 2.0 depends on — the container inventory — so the
adapter it needs is the adapter everything else needs. And it has both output shapes, so the
comparison covers the table a person reads and the document a program parses.

## Finding the daemon

The SDK's `FromEnv` reads `DOCKER_HOST`. Every installation the department uses leaves that unset:
the endpoint lives in `~/.docker/config.json` as the current context, and in the context store
under a directory named by the SHA-256 of the context name.

That layout is undocumented in the sense that nobody promised it, and stable in the sense that
every docker CLI in existence depends on it. Reading it is the difference between a binary that
works on the team's machines and one that works on a machine with `DOCKER_HOST` exported, which is
nobody's.

The shell implementation had this for free by invoking `docker`. It is the first thing the
migration has to pay for, and it is worth writing down that this is the shape of the cost: things
the CLI did for us that the API does not.

## What the comparison found

The list was sorted with `sort`, and `sort` under `en_US.UTF-8` ignores punctuation in its primary
weight. `magento_dev` sorted before `magento-demo` on the machine this was written on and after it
in the C locale. The order of a list of environments depended on the machine's locale.

Go sorts by bytes, so it could not reproduce that, and the two implementations disagreed. The fix
is in the shell one: sort in the C locale. It is the second locale-dependent defect this migration
has surfaced by making a second implementation agree with the first, which is an argument for
porting by comparison rather than by reading.
