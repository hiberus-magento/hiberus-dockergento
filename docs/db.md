# db

Named copies of this project's database, for the moment before something risky.

```bash
hm db snapshot --name=before-upgrade
hm db list
hm db restore before-upgrade
hm db remove before-upgrade
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

## Where snapshots come from on their own

`hm down -v` offers to take one before destroying the environment, and `hm stop --snapshot` takes
one before stopping. See [down](down.md) and [stop](stop.md).

## What this is not

Not a backup tool. These are local working copies: no encryption, no rotation, nothing remote, and
nothing shared between machines. For anything that matters beyond your laptop, use whatever the
project already uses.
