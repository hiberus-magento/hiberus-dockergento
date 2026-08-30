# Design

## What was measured before deciding

Three questions, in this order, because the first two could have ended it:

1. **Is the library usable from outside?** `api.Compose` is documented as the interface for third
   parties, and `compose.NewComposeService` is the way in. It needs a `command.Cli`, which is also
   what resolves the daemon properly — the context store, `DOCKER_HOST`, the config file — and
   replaces the endpoint resolution this tool was doing by hand.
2. **Is what it creates the same thing?** Same configuration hash, both directions, no recreation
   either way, `docker compose ps` sees both. There is an integration test whose only job is to
   keep that true.
3. **Is it worth it?** Creating an environment: 555 ms to 265 ms. Over one already up: 320 ms to
   95 ms. Stopping is unchanged, because stopping is ten seconds of waiting for containers to
   notice a signal and the process was never the cost.

The time is the smaller half of the answer. The larger half is that progress and logs arrive as
calls rather than as lines on a terminal, which is what an HTTP or MCP adapter needs and what no
amount of parsing the command's output would have given.

## The cost, stated plainly

The published binary goes from 8.5 MB to 60.6 MB, and the dependency graph from 70 modules to 426
— docker/cli, containerd and buildkit's trees. Stripping is already on. It cannot be linked away:
`api.Compose` declares `Build` and `Push`, so their implementations are reachable.

`docker/cli`, `docker/docker`, `go-connections` and `compose-go` are pinned to the versions
compose v2.40.3 itself requires, rather than to the newest. Letting them float pulled in a
different `docker/cli` and the moby module split with it, which is how a library and its host
disagree about a type at runtime.

## The compose file list, which was wrong

Two files the project declares, and up to one it does not. A project routed through the global
proxy carries a third that removes its published ports and adds the routing; a branch environment
carries an overlay next to its registration, outside the checkout, with its profile and its
address. They are never both loaded: the proxy overlay claims the main environment's address.

The Go layer loaded neither, because the worktree registry was a placeholder that always answered
"no branch environment". `describe` and `doctor` were reading a proxied project as publishing
ports it does not publish, and a branch environment as the main one. That is a defect in what is
already shipped, and it had to be fixed before `start` could exist at all — a `start` that
resolved a branch environment to the main checkout would recreate the main environment with the
worktree's mounts.

## Where the two implementations differ, and why

- **`restart` is a stop and a start**, as the shell implementation did it, and deliberately not
  Compose's own `restart` — which restarts the containers exactly as they are and does not pick up
  a change to the compose file. Somebody who edits the configuration and runs `restart` expects
  the change to be running afterwards. Being faster at the wrong thing is not an improvement.
- **An unchanged stopped container is not recreated.** `docker compose up` replaces it; this does
  not. What must still be replaced, is: a changed configuration and an updated image both recreate
  the container, and both are tested. The difference is churn the command does and this does not.
- **The proxy is not started for a branch environment.** It does not carry the proxy overlay, so
  starting the proxy for it achieves nothing — and could refuse the start outright when another
  environment happens to hold port 80.
- **Dependencies on a bind mount are refused as a structured error** rather than as a paragraph
  printed to stdout, which is what the output contract asks for and what a `--json` caller can
  read.
- **`start` and `restart` stay with the shell implementation on Linux.** Starting an environment
  there also matches the container's user and group ids to the host's and writes the project's
  domains into the container's `/etc/hosts`, and neither is ported. A `start` that quietly skipped
  them would leave an environment that looks up and cannot write to its own files. The boundary is
  one condition, in one place, and it goes when those tasks go.
