# Design

## A tar stream, not a process

The daemon's copy is the same operation `docker cp` performs, and it takes an archive. Packing and
unpacking it here is a hundred lines and removes a subprocess from a path that runs on every macOS
composer install — and it is the only way to control what happens to links, to modes and to
anything that is neither a file nor a directory.

## Nothing lands outside the target

The stream says what its entries are called, and it comes from a container. Every path is resolved
against the target and refused if it leaves it. A copy that can be talked into writing anywhere is
worth guarding against even when the container is one this tool started.
