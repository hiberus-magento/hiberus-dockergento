# Design

## The registration lives outside the checkout

A worktree shares the repository's tracked files, and `config/docker/properties.json` is one of
them. Writing the worktree's project name there would put it in somebody's commit and change the
main environment's identity at the same time.

So a branch environment is registered in `~/.hm/worktrees/<project>/<name>.json`, next to the
snapshots and the proxy, with its path, profile, branch, domain and compose project name. The
checkout stays exactly as git left it.

That file is also the switch. `hm` already detects a worktree; now it asks a second question —
does this one have an environment of its own? — and only then resolves against the worktree:
`HM_ROOT` becomes the worktree's directory, the project becomes `<project>-<name>`, and the
overlay joins the compose invocation. A worktree with no registration keeps WT-01's behaviour
unchanged, refusals included, because that is still the case that destroys data.

## Profiles are services removed, not services listed

The overlay resets the services the profile does not want (`service: !reset null`), the same
mechanism the proxy overlay already uses to remove `hitch`. Listing services to start instead
would mean every command that talks to compose learning about profiles; removing them means the
configuration is simply smaller, and `hm start`, `hm describe`, the dashboard and the doctor all
see the truth without being told.

| Profile | Services | For |
|---|---|---|
| `lite` | phpfpm | Running code, no HTTP |
| `agent` | phpfpm, nginx, db, search, redis | A Magento that answers |
| `full` | everything | What the main environment runs |

`agent` keeps search because a Magento without it fails on the first reindex, which is not the
kind of surprise to leave in an environment meant for unattended work.

## The proxy is a requirement, not an option

Each branch environment publishing its own ports would be the port lottery the proxy was built to
end — with N environments instead of two. So `hm worktree add` requires the project to be routed
through the proxy and says so plainly when it is not: the overlay resets every published port and
routes `<worktree>.<project domain>` instead. The wildcard certificate already covers it.

## Dependencies: the same idea, two platforms

On Linux the code is bind mounted, so `vendor/` and `node_modules/` are symlinks to the main
checkout: instant, no gigabytes duplicated, and both are git-ignored so nothing appears in
`git status`. If the branch changes dependencies, its own `hm composer install` replaces the link.

On macOS the code lives in a named volume, so there is nothing to link: the main environment's
volume is copied instead, with the same volume-to-volume copy the database templates use. It is
seconds and it duplicates the space, which is the price macOS charges for not bind mounting.

Only `vendor/` and `node_modules/` are shared. `generated/`, `var/` and `pub/static` are compiled
per branch and sharing them would produce a class that belongs to another branch — the hardest
kind of bug to see.

## The database

`hm worktree add` clones the project's template, which is why DB-02 came first. With no template
it does not invent one: it says which command makes it and leaves the environment with an empty
database, because a branch environment sharing the main database would not be an isolated
environment at all — a `setup:upgrade` on the branch would land on everybody.
