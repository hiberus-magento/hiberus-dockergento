# The global proxy

## Why

One router per machine, holding 80 and 443 and sending each request to the project whose domain it
carries. It is what makes several projects up at once possible, it is what branch environments are
built on, and it was the last thing in the second batch that other ported commands still reached
through the shell: starting an environment that needs the proxy ran `proxy up` in bash.

## What Changes

- **`proxy up`, `proxy down` and `proxy status` are Go**, with the same answers and the same
  refusals as the shell implementation, compared against it.
- **The compose file it generates is byte for byte the one bash writes.** That is not tidiness:
  the two halves have to name the same project and mount the same directories, or starting the
  proxy with one and stopping it with the other would be two different things with one name.
- **It is written through a temporary and a rename, and only when it changed.** The file is shared
  by every project on the machine, so rewriting it in place on every start is a global file being
  mutated while other projects may be reading it, for no gain.
- **What it is routing comes from the proxy's own API**, not from our idea of what should be
  routed: a container with the labels and a router that never came up look identical from outside,
  and the question worth answering is whether a request would arrive.
- **Starting an environment that needs the proxy no longer goes through the shell to start it.**

## The container in the way

The proxy needs 80 and 443, and a project that does not use it publishes those itself, so the two
cannot be up at once. Both halves now name the same container when they refuse — asked port by
port, 80 before 443, which is what the shell implementation does. With a full stack up one
container holds 80 and another holds 443, and naming a different one would make the same situation
read as two different problems.
