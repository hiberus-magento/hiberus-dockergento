# Design

## The requirement is inside the container, not on the host

That is the whole of it. PHP resolves `__DIR__` through symlinks, so `vendor` has to be a real
directory **where PHP sees it** — `/var/www/html/vendor` inside the container. A mount puts one
there; a symlink does not. Nothing has to exist on the host side of the worktree.

Compose appends volume entries across files rather than replacing them, so a mount added by the
worktree overlay sits on top of the code mount without disturbing it. Verified against the real
`docker compose config`, because getting that wrong would leave a worktree with no code at all.

## Why read-only, and what it costs

While a site runs, nothing writes to `vendor`: Magento writes to `generated/`, `var/` and
`pub/static`. So read-only costs nothing in normal use, and it converts the dangerous case —
`composer require` in one branch mutating a directory five other environments are reading — from
silent corruption into a refusal.

The caveat, stated rather than hidden: this is what the framework does, not something measured
here against a real store. A project whose module writes into `vendor` would find out on its first
request, with a clear error and nothing broken.

## Sharing is a decision taken once, and recorded

`composer.lock` is compared when the worktree is created. Equal locks mean the dependency trees
are identical and sharing is free. Different locks mean the branch genuinely changed
dependencies, and sharing would be a lie.

The answer is written into the worktree's registration (`vendor: shared | own`), so that
everything downstream — the overlay, `hm composer`, anybody debugging later — reads the decision
instead of guessing it again from files that may have changed since.

## macOS keeps its copy, for now

There the code lives in a named volume and the worktree gets its own, copied from the main one.
That is already correct: the copy includes `vendor` and it lands at the right path.

It is also expensive, and there is a better way that this change deliberately does not take:
Docker 26+ can mount a subdirectory of a volume (`volume.subpath`), which would let a worktree
mount `vendor` straight out of the main environment's code volume and copy only the code —
hundreds of megabytes less. Verified as available, but it changes a path that works, on the
platform where the tests cannot exercise a real store. It belongs to the 2.0 work, and it is
recorded there.
