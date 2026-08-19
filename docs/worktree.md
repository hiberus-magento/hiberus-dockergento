# Working with git worktrees

A git worktree is a second working directory of the same repository, commonly used to run
several AI agents or several tasks in parallel without stashing. Dockergento understands
them.

## What happens from a worktree

Everything is resolved against the **main checkout**: its compose files, its properties
and its containers. That is deliberate — the environment belongs to the main checkout, and
a worktree shares it.

```bash
cd ../my-project-feature-x     # a worktree
hm describe                    # describes the environment of the main checkout
hm mysql -q "SELECT 1"         # queries its database
hm magento cache:clean         # runs against its containers
```

## What is refused

These are blocked from a worktree, with exit code `6`:

`start` · `stop` · `restart` · `rebuild` · `down` · `setup` · `install` ·
`create-project` · `docker-stop-all`

The reason is not caution for its own sake. The compose project name lives in
`config/docker/properties.json`, which is committed, so a worktree inherits it and Docker
Compose treats it as *the same project*. Running `up` from there recreates the existing
containers with the mounts of the worktree, silently repointing the main environment at
it — and `down -v` destroys it, database included.

```
$ hm start
You are in a git worktree (feature-x).
The environment belongs to the main checkout:
  /Users/me/projects/my-project

Running hm start here would recreate or destroy that environment with the mounts of
this worktree.

  Run it from the main checkout, or repeat with --force if that is really what you want.
```

`--force` applies to that single invocation. There is no variable and no setting that
turns the guardrails off permanently, so they cannot quietly stop existing.

## Important: the containers serve the main checkout's code

This is not a per-worktree environment. When you run `hm magento`, `hm composer` or the
tests from a worktree, they execute against the code of the main checkout, not the code
you are looking at. The CLI says so every time:

```
Note: the containers serve the code of /Users/me/projects/my-project, not this worktree.
```

Per-worktree environments need a shared proxy and database snapshots; they are a separate
piece of work.

## Escape hatch

`HM_PROJECT_DIR` forces which directory is treated as the main checkout, for setups where
git is unavailable or the layout is unusual.

```bash
HM_PROJECT_DIR=/Users/me/projects/my-project hm describe
```
