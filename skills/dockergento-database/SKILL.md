---
name: dockergento-database
description: Query, import, dump, copy and restore the database of a Hiberus Dockergento (hm) Magento project, including snapshots and data directory templates. Use for any SQL against a local Magento, before any risky migration, and when an environment needs data.
---

# Dockergento: the database

The database runs in the `db` service (MariaDB). Everything here is run from the project
directory.

## Running a query

```bash
hm mysql -q "SELECT path, value FROM core_config_data WHERE path LIKE 'web/%' LIMIT 20"
```

`-q` is the option for a query. There is no `-e`. `hm mysql` with no options opens an interactive
client, which is not what you want when you are not a person.

Prefer a `LIMIT`. A Magento `sales_order` or `catalog_product_entity` will answer with more rows
than anybody can read.

## Before anything risky, take a copy

```bash
hm db snapshot --name=before-upgrade
hm db list
hm db restore before-upgrade
hm db remove before-upgrade
```

Snapshots are compressed dumps in `~/.hm/snapshots/<project>/`. They survive `hm down -v`, which
is the moment they exist for. **`hm db restore` replaces the database and does not come back** —
it asks for the project name to be typed, so run it only when you were asked to.

`hm stop --snapshot` takes one on the way out, and `hm down -v` offers one before destroying
anything.

## Standing an environment up fast

```bash
hm db freeze --name=base     # freeze this data directory as a reusable template
hm db templates              # what exists, with size and database image
hm db clone shop/base        # build this project's data directory from one
hm db drop shop/base
```

A template is a byte copy of the data directory in a Docker volume: seconds to clone, where
importing the same data as a dump is tens of minutes. It is tied to the database version it came
from, and cloning refuses a different one.

`hm db freeze` stops the database while it copies. `hm db clone` needs the environment stopped and
replaces whatever data is there.

**Snapshot or template?** A snapshot is portable, small and readable two server versions later —
that is the one to keep or to hand to somebody. A template is fast and tied to this machine — that
is the one to build environments from.

## Importing and exporting

```bash
hm mysql -i dump.sql            # import
hm mysql -d -i dump.sql         # import, stripping DEFINER clauses
hm mysqldump /path/to/out.sql   # export
hm transfer-db                  # bring one from a remote environment
```

A dump from production almost always needs `-d`: its `DEFINER` users do not exist locally and
the import fails on the first view or trigger.

## Real customer data

```bash
hm masquerade
```

Anonymises the database in place. A dump from production has names, addresses, emails and orders
in it. If you are about to work on one — and especially if it is going anywhere near a model,
a log or a shared machine — anonymise it first, and say that you did.

## After changing data

Magento caches and indexes will not agree with the database until told:

```bash
hm magento cache:flush
hm magento indexer:reindex
```

## What not to do

- Do not connect to the database with a container name and `docker exec`. `hm mysql` finds the
  right container for this project; a name matched by substring can be another project's database.
- Do not write to `core_config_data` by hand when `hm magento config:set` exists.
- Do not run `hm db clear`, `hm db restore` or an import over a database you were not asked to
  replace.
