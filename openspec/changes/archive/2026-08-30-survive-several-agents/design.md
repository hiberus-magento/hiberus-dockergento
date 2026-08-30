# Design

## A lock that works on both platforms

macOS has no `flock(1)`, so the lock is a directory: `mkdir` is atomic on every POSIX filesystem
and needs nothing installed. The process that creates it writes its pid inside and removes it on
exit, including on interrupt.

A lock held by a process that no longer exists is broken rather than waited on. That is not
politeness — an agent killed mid-command would otherwise block every other agent for ever, and
the person debugging it a week later has no way of knowing what the directory is.

Waiting is bounded. A command that cannot get the lock in a few seconds says which operation is
holding it and gives up, because a CLI that hangs is worse than one that fails.

## Atomic means a rename, and a unique name

Two things, both needed. A fixed temporary name (`$file.hm-tmp`) is not a temporary file, it is a
shared file with a longer name: two processes writing it interleave. And a redirection into the
final path leaves a half-written file when anything goes wrong.

So: `mktemp` next to the target, write, `mv`. The rename is atomic within a filesystem, so a
reader sees either the old file or the new one — never half of one.

## The worktree resolves its own configuration

The properties directory is derived from the project root, and the project root is not known
until the worktree registration has been read. The order was wrong, and the fix is to re-derive
it, and re-read the properties, once the worktree is resolved.

Re-reading is not elegant. The alternative — keying the registry by path instead of by project
name, so no properties are needed to resolve it — is better and belongs in 2.0, where the
registry is being rebuilt anyway. Here the cost is one extra read of a small JSON file, and the
benefit is that a worktree stops writing into somebody else's checkout.

## Collisions are refused, not resolved

When a name is taken, the tool says so and stops. It does not append a number.

A generated name is a name nobody chose, and in a tool where the name of a thing decides which
containers, which volumes and which database it uses, a name nobody chose is how you end up
working in an environment you did not mean to open.

## `/etc/hosts`: marked, so it can be found again

Entries are appended today and never removed, and there is nothing in the line to say who put it
there. New entries carry a marker comment, so the tool can list its own and remove them, and
`hm clean` can report the ones whose project no longer exists.

What it does not do is edit `/etc/hosts` on its own during a cleanup. It needs `sudo`, it is a
file other things depend on, and a tool that quietly rewrites it during an unrelated command is a
tool nobody trusts twice. It lists, and it gives the command.
