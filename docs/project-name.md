# The project name

The compose project name is the environment's identity: it names the containers, the network
and — the part that matters — the **volumes**. Change it on an existing environment and its
database is still there, attached to a name nobody asks for any more.

## How it is resolved

1. **`COMPOSE_PROJECT_NAME` in `config/docker/properties.json`**, if it has a value. This always
   wins, and nothing adjusts it.
2. **Otherwise, the project's root directory**, using the same rule Docker Compose applies.

Never the other way round. Every project that has the property set behaves exactly as it did
before this existed.

## The derivation rule

It is Compose's own rule, not an approximation of it: lowercase, keep only `[a-z0-9_-]`, and trim
leading dashes and underscores.

| Directory | Name |
|---|---|
| `MyShop` | `myshop` |
| `mi tienda` | `mitienda` |
| `shop.local` | `shoplocal` |
| `acentúado` | `acentado` |
| `my_shop-1` | `my_shop-1` |
| `--shop--` | `shop--` |
| `___` | no name — refused |

Accented characters are **dropped**, not transliterated, because that is what Compose does. A
test compares our derivation against `docker compose config` on real directories, so if Compose
ever changes the rule we find out from a failing test rather than from a missing volume.

The directory used is the **project root**, so a worktree resolves to its main checkout: a
worktree does not get an identity of its own (see [worktree](worktree.md)).

A directory that yields nothing usable is refused with exit code `4`:

```console
$ hm describe
Error: No project name can be derived from ___
Try: Rename the directory, or set COMPOSE_PROJECT_NAME in config/docker/properties.json
```

## Why `setup` may not write it down

`config/docker/properties.json` is committed, so whatever it says travels to every clone of the
project. Recording the name that the directory would have produced anyway is what made a second
clone inherit the first one's identity — same containers, same volumes, neither of them asked
for.

So `hm setup` records the name **only when it is a decision**:

| What you do | What is written |
|---|---|
| Accept the proposed name (the directory's) | nothing |
| Type a different name | `COMPOSE_PROJECT_NAME` |
| Run `setup` on a project that already has one | it is kept |

A project that already has a name never loses it. Removing it would be renaming somebody's
environment, and volumes do not follow a rename.

## Two copies of the same project

With no name recorded, two directories are two environments — which is what you want when you
clone a project to compare a branch:

```bash
git clone git@…/shop.git shop-review
cd shop-review && hm start      # its own containers, its own database
```

With a name recorded, both copies share it, because someone decided that on purpose.

## Changing the name of an existing project

There is no command for it, deliberately. Editing `COMPOSE_PROJECT_NAME` renames the
environment: the old containers and volumes stay behind under the old name. If that is what you
want, take a [snapshot](mysqldump.md) first, then `hm down -v`, change the name and set the
environment up again.
