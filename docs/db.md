# db

Named copies of this project's database, for the moment before something risky.

```bash
hm db snapshot --name=before-upgrade
hm db list
hm db restore before-upgrade
hm db remove before-upgrade
```

Frozen copies of the data directory, for standing a second environment up in seconds:

```bash
hm db freeze --name=base
hm db templates
hm db clone shop/base
hm db drop shop/base
```

## Why not just `hm mysqldump`

`hm mysqldump` and `hm mysql -i` already move a database in and out of a file. What was missing is
management: somewhere for the copies to live, a name to call them by, and a list to see them in.
Without that, saving before a `setup:upgrade` depends on somebody thinking of it — which is why it
does not happen.

Use `hm mysqldump` to export somewhere specific, or to hand a database to someone else. Use
`hm db` for the copies you take while working.

## Where they live

`~/.hm/snapshots/<project>/`, outside the project, for two reasons that matter more than
tidiness:

- `config/docker/` is committed, so a copy kept there would end up in somebody's commit — and they
  are tens of megabytes.
- **They survive `hm down -v`.** A copy stored inside the environment being destroyed would be
  stored in exactly the wrong place.

Grouped by the project's [resolved name](project-name.md), so two projects never mix.

## Taking one

```bash
hm db snapshot                      # named after the date
hm db snapshot --name=before-upgrade
hm --force db snapshot --name=before-upgrade   # overwrite one that exists
```

The project keeps running: the copy is taken with `--single-transaction`, from a consistent
snapshot and without locking tables. Routines, triggers and events are included — a copy that
restores a Magento without them is not a copy of that Magento. Everything is compressed.

## Restoring

```bash
hm db restore before-upgrade
```

**This is destructive and does not come back.** The database is emptied first, so that restoring
returns exactly what was copied rather than a mixture of the copy and whatever was created
afterwards.

You are asked to type the project's name to confirm. A blind `y` is a reflex; typing the name means
the sentence was read. In a script, `hm --yes db restore <name>` skips the question.

The site stays up while it runs, but its database is being rewritten underneath it — expect
strange behaviour for those seconds, and flush the cache afterwards:

```bash
hm magento cache:flush
```

## Listing

```bash
hm db list
hm db list --json
```

Name, when it was taken and how much space it uses. In JSON, `.data.snapshots[]` with `name`,
`taken_at` and `size`.

## Clearing them

`remove` deletes one by name. To reclaim space:

```bash
hm db clear          # every snapshot of this project
hm db clear --all    # every snapshot of every project on this machine
```

Both list what they are about to delete and ask you to type the name of what is being destroyed —
the project's name, or `all`. There is no undo and they are the only copies, so a `y` typed by
reflex is not enough. `hm --yes db clear` skips the question in a script.

## Every database version the tool supports

Projects here run anything from MariaDB 10.2 to 12.3, and the tools were renamed along the way:
`mysqldump` became `mariadb-dump`, `mysql` became `mariadb`. Both names are resolved inside the
container, so the same command works on all of them.

That is checked, not assumed: `tests/integration/db_image_matrix_test.sh` takes a snapshot and
restores it on every image in `data/requirements.json`, verifying the data, the routines and the
triggers all come back and that nothing created after the snapshot survives it.

| Image | Verified |
|---|---|
| `hiberusmagento/mariadb:10.2` | ✓ |
| `hiberusmagento/mariadb:10.3` | ✓ |
| `hiberusmagento/mariadb:10.4` | ✓ |
| `hiberusmagento/mariadb:10.6` | ✓ |
| `mariadb:11.4` | ✓ |
| `mariadb:12.3` | ✓ |

By default the test only uses images already on the machine, so a normal run stays fast. Set
`HM_TEST_DB_MATRIX=1` to pull and check every one.

## Templates: the same data, without the import

A snapshot is a dump. Restoring one means a server parsing SQL and rebuilding indexes — tens of
minutes on a real catalogue, for data that already exists, byte for byte, in a volume on the same
disk.

A **template** is that volume: a byte copy of the data directory, frozen under a name.

```bash
hm db freeze --name=base
```

The database is stopped while it copies and started again afterwards. That is not caution for its
own sake: a running InnoDB keeps pages in memory that are not in the files yet, so a copy taken
underneath it is a crash to recover from rather than a copy.

Standing an environment up from it costs the time of a file copy:

```bash
hm db clone base       # this project's own template
hm db clone shop/base  # the template of another project
```

The qualified form is the point of the feature: the environment that needs a database is rarely
the one that made it. A second checkout, a branch environment, a colleague's project on the same
machine — all of them clone by naming whose data they are taking.

Cloning replaces files under a stopped server, so:

- It refuses while the environment is running, and says to `hm stop` first.
- It asks you to type the project name if there is already a database there.
- It refuses a template made with a different database image, naming both. A 10.6 data directory
  under 10.2 produces a server that starts, complains, and loses data in ways that are found much
  later.

`hm db templates` lists what exists, with the image each came from and its size. `hm db drop`
deletes one.

### Which of the two to use

| | Snapshot | Template |
|---|---|---|
| Shape | Compressed dump | Copy of the data directory |
| Cost to restore | Minutes to tens of minutes | Seconds |
| Survives a version change | Yes | No — tied to the image |
| Can be sent to a colleague | Yes | No |
| Lives in | `~/.hm/snapshots/` | A Docker volume |

Take a snapshot before something risky, and to keep. Freeze a template for the database you
recreate again and again.

### Templates and disk

They are full copies, so a template costs what the database costs. `hm db templates` shows the
size of each, and `hm clean` collects the templates of projects whose directory no longer exists,
like everything else the tool creates.

## Opening it in a client

```bash
hm tableplus          # or hm sequelace, or hm dbeaver
hm tableplus --print  # just the connection string
```

The host port, user, password and database name are read from the resolved configuration, so a
project that renamed its database or moved its port opens correctly without anybody updating a
saved profile — which is what goes stale about saved profiles.

If the client is not installed, the connection string is printed anyway with the name of what was
missing: what you needed was the connection.

**A project routed through the [proxy](proxy.md) publishes no database port.** MySQL carries no
hostname, so Traefik cannot route it and the overlay removes the port; there is nothing on your
machine to point a client at. The command says so and names the one that opens a door:

```bash
hm tunnel db                      # leave it running
hm tableplus --port=<the port>    # in another terminal
```

## Real customer data

```bash
hm masquerade
```

Anonymises the database in place, with the [masquerade](https://github.com/elgentos/masquerade)
tool. A dump from production has real names, addresses, emails, phone numbers and order history in
it.

**A successful anonymisation is recorded**, with its date, so that `hm describe`, `hm doctor` and
the [context an agent reads](ai-context.md) can stop guessing. And it **expires**: restoring a
snapshot, cloning a template, importing a dump or transferring a database clears the record,
because whatever those brought in has not been anonymised. Three states — anonymised, not,
unknown — and unknown is never treated as safe.

Since 1.7.0 it also works without a terminal, which it never did: `docker run -t -i` was passed
unconditionally, so it failed from CI, from a script and from an agent with `the input device is
not a TTY`.

`hm worktree add --profile=agent` anonymises by default. With an agent, this stops being good
practice and becomes compliance: what an agent reads goes to a model, over a network, outside the
company.

## Where snapshots come from on their own

`hm down -v` offers to take one before destroying the environment, and `hm stop --snapshot` takes
one before stopping. See [down](down.md) and [stop](stop.md).

## What this is not

Not a backup tool. These are local working copies: no encryption, no rotation, nothing remote, and
nothing shared between machines. For anything that matters beyond your laptop, use whatever the
project already uses.
