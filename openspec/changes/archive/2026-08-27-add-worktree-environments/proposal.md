# An environment per branch, on demand

## Why

A git worktree is a second working directory of the same repository, and it is how you review a
branch without stashing what you are doing — or how three agents work on three tasks at once.
Today it is a trap the tool has to defend against: the compose project name is committed, so a
worktree inherits it, and `hm start` from one repoints the main environment's bind mounts at the
worktree while `down -v` destroys it. WT-01 made those commands refuse rather than obey.

Refusing is the right answer to the accident and no answer at all to the need. What people
actually want is the branch running somewhere they can look at it, next to the main environment
rather than instead of it.

Everything that made that impractical is now in place: the global proxy gives each environment a
name instead of a port, and database templates give it data in seconds instead of an import. What
is missing is the command that puts them together.

## What Changes

- **`hm worktree add <branch>`** creates the git worktree, gives it its own compose project, its
  own subdomain, its dependencies and its database, and starts it.
- **Profiles** decide what runs: `lite` (PHP alone), `agent` (PHP, nginx, database, search and
  Redis — a Magento that answers over HTTP), and `full` (the whole stack). A branch environment
  that also runs Varnish, TLS termination, a mail catcher and a message queue costs more than the
  branch is worth.
- **`hm worktree list`** shows the branch environments of this project and their state.
- **`hm worktree remove <name>`** destroys the environment and removes the worktree.
- A registered worktree resolves against **itself**: its own project name, its own volumes, its
  own bind mounts. WT-01's refusals stay exactly as they are for a worktree that has no
  environment of its own, which is still the dangerous case.
- Dependencies are not installed again: on Linux `vendor/` and `node_modules/` are linked to the
  main checkout, on macOS the code volume is copied, which is the same idea in the terms the
  platform allows.
- The database comes from a template (`hm db freeze`), cloned in seconds.
