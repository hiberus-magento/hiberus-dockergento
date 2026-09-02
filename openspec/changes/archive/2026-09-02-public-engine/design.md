# Design

## Why a package and not a second binary

The shape that suggests itself is a Go binary that owns Docker, called by the commands. That is
the seam this migration removed, and the numbers are why: on this machine `bin/run` costs 112 ms
to start and answer `--version`, the Go binary costs 18, and `docker compose version` costs 125.
With an interpreter in front, every command pays that before anything happens, plus the boundary
to the helper and the cost of serialising the answer back.

There is something worse than the time. A shell in front cannot ask questions concurrently, and
that is where most of the gains came from: `describe` asks six independent questions at once
(285 ms to 94), `doctor` seventeen (260 to 85). If the helper does that instead, the helper is the
command and the shell in front is a 112 ms toll.

The same argument applies to a CLI in another language talking to a Go engine over a pipe. The
seam worth having is between the domain and the outside world, and that one is the ports.

## What the facade is for

Six adapters have to be assembled to describe a project, and the knowledge of which compose files
a project is built from — its own, the platform overlay, and the proxy or worktree overlay it does
not declare — lived in the command line. Any second consumer would have got that wrong, and
getting it wrong is not cosmetic: without the proxy overlay a project reads as publishing ports it
does not publish.

So it moved into the engine, and with it everything else the command line was doing that was not
about terminals: resolving the project, merging the properties over the tool's defaults, building
the interpolation environment, deciding whether a project is routed through the proxy.

What the command line kept is what it is for: parsing arguments, choosing text or JSON, colour,
exit codes, and the name the tool was invoked as. It imports the facade and the domain types.
That is the test of the design — not that it compiles, but that the most demanding consumer needs
nothing else.

## What a consumer that is not the CLI has to be told

Two things it would otherwise get wrong, so they are options rather than defaults:

- **Where the tool is installed.** It defaults to the directory of the running executable, which
  is right for `hm` and is the web server's own binary for anything else. It matters: the tool's
  `data/properties.json` holds the defaults every project's file is merged over, and its compose
  template answers "which ports would an environment need here" when there is no project.
- **How to announce a slow step.** The command line paints it; an HTTP adapter would send it. The
  engine calls a function and has no opinion.

## Where HTTP goes

`internal/api/`, beside `internal/cli/`, in this binary — a mode (`hm serve`) rather than a daemon.
There is no long-lived state to own: Docker holds the containers and the registry is a file. The
day that changes — watching Docker events for a live interface, collecting orphans in the
background — it becomes one, and the ports are what make that an added adapter rather than a
rewrite.
