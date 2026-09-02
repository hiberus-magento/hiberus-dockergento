# Design

## Why the templates and not the snapshots

Both are `hm db`, and there it ends. A snapshot is a dump: a file, portable, for keeping. A
template is the data directory itself, copied byte for byte into a volume: not portable across
server versions, and standing up in seconds.

`worktree` needs the second one. A branch environment with its own database is affordable because
cloning a template is a file copy; with an import it would be tens of minutes per branch and
nobody would use it.

Porting by family is not the same as porting half a command. `freeze` and `snapshot` do not share
a line of logic — they share a word.

## What nothing here does

Talk to a database server. Every one of these operations replaces or copies the files underneath
one, which is what makes it fast and is also why the environment has to be down for a clone: a
server that finds its data directory changed underneath it does not notice until much later, and
what it does then is lose data quietly.

## The three guardrails, kept

**Nothing running.** A clone is refused while anything of the project is up.

**The image has to match.** A data directory is not portable across server versions: 10.6 files
under 10.2 produce a server that starts, complains, and loses data in ways that are found much
later. The image that made the template is recorded on it and compared.

**Replacing data asks for the project's name.** Not a letter. A blind `y` is a reflex; typing the
name means the sentence was read.

## Where the volume and the image come from

The resolved configuration, never a name built here. The project name can be overridden, the volume
can be renamed in an overlay, and a guess that is right for most projects is exactly the kind of
thing that destroys the data of the rest.

The copy runs in the project's own database image. It is already on the machine, so nothing is
pulled, and its GNU `cp -a` reproduces a data directory — ownership, sockets, sparse files — where
busybox's would need arguing with. The destination is emptied first: a data directory restored on
top of another is neither of the two.
