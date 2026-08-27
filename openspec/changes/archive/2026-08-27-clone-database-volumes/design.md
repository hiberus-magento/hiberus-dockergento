# Design

## The copy is a volume, not a dump

A frozen template is a Docker volume, `hm-template-<project>-<name>`, holding a byte copy of the
project's data directory. Restoring it is `cp -a` between two volumes; there is no SQL parser, no
index rebuild and no `setup:upgrade` in the path.

The copy is made with the project's own database image rather than `alpine`: it is already on the
machine, so nothing is pulled, and its GNU `cp` copies a data directory — sockets, sparse files,
permissions — without the surprises busybox has with unusual file types.

## The database has to be stopped

Copying a running server's data directory gives a torn copy: InnoDB keeps dirty pages in memory
that are not in the files yet. Recovery on the copy would usually rescue it, and "usually" is not
a property a backup should have.

So `freeze` stops the database container for the duration of the copy and starts it again, saying
so before it does. When the environment is already down it just copies. `clone` requires the
environment to be down — it is replacing the file the server has open — and says which command
brings it down rather than doing it silently.

## Version compatibility is recorded, not assumed

A MariaDB 10.6 data directory does not open under 10.2, and putting one under the other produces
a server that starts, complains and loses data in interesting ways. The template therefore records
the image that produced it (`hm.db_image`) and `clone` compares it with the image the target
project runs. A mismatch is refused, naming both versions, unless the user insists with `--force`.

This is the check the image matrix tests (DB-01) made obvious: the same tool serves databases from
MariaDB 10.2 to 11, and nothing else in the tool crosses data between them.

## Addressing: `<project>/<name>`

Snapshots live under the project that made them and are never seen from elsewhere. Templates
exist precisely to be used from elsewhere — the whole point is a second environment of the same
code. So a template's full name is `<origin-project>/<name>`, a bare `<name>` means the current
project's, and `hm db clone shop/base` from a derived environment is explicit about whose data it
is taking.

## Why not reuse `snapshot`

They answer different questions. A snapshot is portable, small, and readable a year and two
MariaDB versions later; a template is fast and tied to a server version. Overloading one verb with
a `--fast` flag would hide exactly the difference that matters — one you can send to a colleague,
the other you cannot.

## Templates and `hm clean`

A template carries `hm.project` and `hm.root`, like everything else the tool creates, so
`hm clean` treats it by the rule it already applies: collectable only once the project directory
it came from is gone. Templates do not get an exemption, because a template of a project nobody
has any more is exactly what `clean` is for.
