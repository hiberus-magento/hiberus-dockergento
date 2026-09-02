# Design

## Prepare, create, settle

Three phases, and the boundary between them is what can still be refused.

**Prepare** decides everything and creates nothing: the name, the address, the path, and every
refusal. Nothing on disk has changed when it fails.

**Create** makes the worktree and writes the registration and the overlay, under one lock. Without
it the four steps — is the name free, create the worktree, write the overlay, write the
registration — are four moments in which another agent can do the same thing, and both end up
believing they own it.

**Settle** is the dependencies, the address, the data and starting it. None of it fails the
command, because by then the environment exists and is registered.

## The overlay, read rather than run

It is a pure function from a profile, a domain, a network and a list of services to a compose
file. That is not tidiness: every mistake it can make is silent. A repeated service key in YAML is
not a merge — the last one wins and the earlier block disappears. A router pointing at a service
the profile removed is a 404 with an explanation nobody has. A mount list without its final newline
puts the next service key on the same line and the document stops being YAML.

All of those are read in a test. And the whole thing was generated for the three profiles, with
and without mounts, and compared against the shell implementation's output.

## Dependencies: mounted, never linked

Composer's autoloader computes its base directory from `dirname($vendorDir)`, and PHP resolves
`__DIR__` to the real path behind a symlink. With a link, the worktree's own modules are never
registered and its code never runs — which was found with a real PHP, the only way that class of
bug is found.

Read-only, because a `composer require` in one branch would otherwise corrupt what the main
checkout and five other worktrees are reading. And only while the locks match: different locks mean
the branch changed its dependencies, and sharing them would be a lie.

## The lock, and a defect in reading it

The shell implementation writes the holder's pid into the lock directory with a trailing newline.
Parsing that as a number without trimming fails, which marks the lock stale, which breaks it — so
the Go half would have walked straight through a lock the shell half was holding, which is the one
thing the lock exists to stop.
