# worktree

An environment per branch, next to the main one instead of on top of it.

```bash
hm worktree add feature/checkout          # branch, environment and data, ready
hm worktree list
hm worktree remove feature-checkout
```

## What this replaces

A git worktree is a second working directory of the same repository — how you look at a branch
without stashing what you are doing, and how several agents work on several tasks at once.

Until now the tool could only defend itself from one. The compose project name is committed, so a
worktree inherits it: `hm start` from a worktree repointed the main environment's bind mounts at
the worktree, and `hm down -v` destroyed it with its database. [Those refusals](#the-refusals-stay)
are still there, and still right, for a worktree with no environment of its own.

This gives it one.

## What `add` does

1. Creates the git worktree (and the branch, if it does not exist yet).
2. Registers it in `~/.hm/worktrees/<project>/<name>.json` — **outside the checkout**, because
   `config/docker/properties.json` is a committed file and writing the worktree's project name
   there would travel in somebody's commit.
3. Writes the compose overlay that expresses its profile and its routing.
4. Gives it dependencies without installing them again.
5. Clones the database from a [template](db.md#templates-the-same-data-without-the-import).
6. Starts it, unless `--no-start`.

From then on, `hm` commands run in that directory act on that environment. Nothing else changes:
the main environment keeps its name, its containers and its address.

## Profiles

| Profile | Services | For |
|---|---|---|
| `lite` | phpfpm | Running code. No HTTP, no address |
| `agent` | phpfpm, nginx, db, search, redis | A Magento that answers. **The default** |
| `full` | everything the project has | What the main environment runs |

A branch environment that also runs Varnish, TLS termination, a mail catcher and a message queue
costs more than the branch is worth. `agent` keeps the search engine because a Magento without one
fails on the first reindex — not a surprise to leave in an environment meant for unattended work.

The profile is expressed by *removing* services from the configuration, not by listing the ones to
start. So `hm describe`, `hm doctor` and the dashboard see the truth without being told about
profiles at all.

## The address

`https://<name>.<the project's domain>`, through the [global proxy](proxy.md). Which is why the
proxy is required: every branch environment publishing its own ports would be the port collision
the proxy exists to end, with as many environments as branches.

The wildcard certificate already covers the subdomain. If the domain reaches your machine through
`/etc/hosts` rather than a wildcard resolver, the new subdomain needs a line of its own —
`hm worktree add` says so when that is the case. See [dns](dns.md).

The auxiliary interfaces (mail, queue, search) are not routed for a branch environment. Reach one
when you need it with [`hm tunnel`](tunnel.md).

## Dependencies

Not installed again, which is the difference between a minute and half an hour:

- **Linux**: `vendor/` and `node_modules/` are symlinks to the main checkout. Both are
  git-ignored, so nothing appears as modified. If the branch changes dependencies, its own
  `hm composer install` replaces the link with real files.
- **macOS**: the code lives in a named volume, so there is nothing to link — the main
  environment's volume is copied instead. Seconds, and it duplicates the space. That is what
  macOS charges for not bind mounting.

Only those two are shared. `generated/`, `var/` and `pub/static` are compiled per branch, and a
class from another branch is the hardest kind of bug to see.

## The database

Cloned from the project's template:

```bash
hm db freeze          # once, in the main checkout
hm worktree add feature/checkout
```

With no template the environment is still created, with an empty database, and the tool says
which command would have given it data. It does not fall back to sharing the main database: a
`setup:upgrade` on the branch would then land on everybody, which is the opposite of an isolated
environment.

## The data an agent will read

`--profile=agent` anonymises the cloned database before handing the environment over. Not because
anonymising is tidy, but because an agent reads the database and what it reads goes to a model,
over a network, outside the company — and a development database is a copy of production, with
real names, addresses, emails and orders in it.

```bash
hm worktree add feature/x --profile=agent                   # anonymised
hm worktree add feature/x --profile=agent --no-anonymise    # not
```

`--no-anonymise` exists because reproducing a bug that only happens with one customer's order
history is a real thing people do. It is their data and their decision; the tool's job is to make
the safe path the one you get by not thinking.

`lite` and `full` are not anonymised automatically: those are usually a person's own second
checkout, and rewriting their data uninvited would be the tool overreaching.

If the anonymisation fails, the environment is still created and the failure is said out loud — an
environment that was supposed to be anonymised and is not is exactly what this exists to prevent.

## The refusals stay

A worktree with **no** registered environment behaves exactly as before: `start`, `stop`,
`rebuild`, `down` and the rest are refused, naming the main checkout. That is still the case that
repoints somebody's mounts and destroys their data.

`hm worktree add` itself must be run from the main checkout.

## Removing one

```bash
hm worktree remove feature-checkout
```

Destroys the environment with its volumes, removes the git worktree and deletes the registration.
It refuses while there is uncommitted work in the worktree: the containers and the database can be
rebuilt in seconds, and the code cannot be rebuilt at all.

## What this is not

Not a way to run somebody else's branch on a shared machine, and not isolation in the security
sense: the environments share a Docker daemon, a proxy and a certificate. It is a way to have more
than one branch of your own project running at once.
