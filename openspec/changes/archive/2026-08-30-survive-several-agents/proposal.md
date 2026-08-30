# Survive several agents at once

## Why

The tool was written for one person doing one thing at a time, and it shows in three ways that
have nothing to do with features:

- **A worktree reads the main checkout's configuration.** The properties directory is resolved
  before the tool knows it is in a worktree. Reading the wrong file is the small half; the large
  half is that `save_properties` from a worktree **writes into the main checkout**.
- **Nothing is locked. Anywhere.** Not one `flock` in the repository. The registry is written with
  a plain redirection, a worktree is created in four steps with no exclusion between them, and
  four temporary files have fixed names. Two agents doing the same thing at the same time
  corrupt each other's state, and the failure is a half-written JSON file that nobody can explain
  a week later.
- **Nothing is checked for collisions.** Two projects can claim the same compose name, the same
  domain and the same Traefik rule. Traefik does not refuse it — the routers have different
  names — so the routing is simply ambiguous.

None of this is theoretical: it is the difference between running ten branch environments and
finding out on Monday that two of them were the same environment.

There is a second reason to do it now. The next decision — how urgently to migrate — is supposed
to be answered by launching ten worktrees at once and looking at what happens. With these defects
in place that experiment measures the defects, not the model.

## What Changes

- **One lock, used by everything that writes shared state.** Portable to macOS, which has no
  `flock(1)`.
- **Every shared file is written atomically**: a temporary with a unique name, then a rename.
- **A worktree resolves its own configuration.** The properties directory follows the worktree,
  so nothing reads or writes the main checkout's by accident.
- **`hm.agent` is stamped**, so the environments an agent works in can be told apart from the
  rest.
- **Collisions are refused before they are registered**: a compose project name already in use, a
  domain already routed by another environment, a branch whose name collapses onto an existing
  one.
- **`/etc/hosts` entries are marked and removable.** They are added and never removed today, so
  they accumulate for as long as the machine lives.
