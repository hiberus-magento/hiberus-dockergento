# Moving a project's files between the machine and its container

## Why

These two commands exist because of macOS. There the code is copied into a volume rather than
mounted, which is what makes PHP fast enough to work in, and the price is that the two sides are
two places: what Composer wrote inside has to be brought out, and what an editor wrote outside has
to be taken in.

They are also the piece `ssl` needs — a certificate has to get into the container that serves it —
so porting them is what unblocks it.

## What Changes

- **`copy-to-container` and `copy-from-container` are Go**, through the daemon's own copy, which
  takes and gives a tar stream: the same thing `docker cp` does, without the process.
- **A path that is a bind mount inside the container is refused**, as it was. The two sides are
  already the same file there, and copying one onto the other is a way to lose whichever was newer.
- **The destination directory is made first.** The daemon answers "could not find the file" about
  the *destination* when there is none, which reads as though what was being copied did not exist.
- **A directory copied out replaces what is there** rather than nesting a second copy inside the
  first, which is what a mirror has to do to be a mirror.
- **Ownership is put right afterwards**: a file the daemon wrote belongs to root, and PHP does not
  run as root.

## What is not carried across

Symbolic links are followed rather than copied as links — a link carried across is a link to
somewhere that does not exist on the other side — and a link to nothing is skipped rather than
failing the copy, because it is somebody's leftover and the rest of the tree is what was asked for.
Devices and sockets are not written at all: a project's code does not contain one, and recreating
one is not something this should be doing.
