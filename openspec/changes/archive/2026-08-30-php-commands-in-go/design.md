# Design

## One thing underneath

`magento`, `composer` and `exec` all run something in the php container, and they are one function
with three callers. The terminal is asked for only when there is one, and the exit code that comes
back is the inner command's — a wrapper that flattened it would break everything that branches on
it, which for `bin/magento` is every deployment script anybody has.

## The path that is not the same thing

On macOS `composer install` is not "run composer in the container". It restarts php, copies the
dependencies in, runs Composer, stops php, **deletes the host's vendor directory**, copies the
container's whole tree back over it, and starts php again.

That is a sequence with a `rm -rf` on somebody's source tree in the middle of it, built on
`copy-to-container`, which is not ported. Porting the four lines that call it and leaving the rest
in shell would be the worst of both. So the whole invocation goes to the shell implementation, and
the condition that decides it is one function with a test.

Everything else — `show`, `dump-autoload`, `--version`, and all four on Linux, where there is no
mirror because there is no mirror to do — runs in Go.

## Why the release moved to macOS

Compose's file watcher reaches FSEvents through cgo. A darwin binary built with `CGO_ENABLED=0`
fails to compile, with a wall of undefined symbols from a package nobody imports directly, so the
failure does not explain itself.

A macOS runner builds all four: darwin natively, darwin/amd64 through Apple's toolchain, and both
Linux architectures with cgo off, which cross-compiles from anywhere. The workflow builds all four
before publishing anything, so a platform that stopped building fails the release rather than
shipping half of it.

This was verified rather than assumed: the four binaries were built and each was run on its own
platform — the darwin ones here, the Linux ones inside containers, one of them talking to the real
daemon.

## The proxy, both ways

A project routed through the global proxy carries a generated overlay that removes its published
ports and adds the routing; one that is not, does not. The tool has to serve both, and the answer
is not a flag: the overlay is loaded because it is there. A project that turned the proxy on in its
properties and has not been set up again is still publishing its ports, and reading it any other
way would describe an environment that does not exist.

A branch environment never takes the proxy's overlay — it claims the main environment's address —
and takes its own instead, from beside its registration outside the checkout.
