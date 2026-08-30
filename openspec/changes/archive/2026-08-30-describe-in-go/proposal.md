# `describe` in Go, and Compose read as a library

## Why

`describe` is the command run most often, the first thing an agent asks, and the one with the
richest contract — versions, services, addresses, state, credentials on request. If a port can
reproduce that exactly, it can reproduce anything the tool answers.

It is also where the cost of the shell implementation is clearest. Every read path calls
`docker compose config`, which costs between 90 and 260 ms, and there are fourteen such calls
across the tool. `describe` alone pays it once, plus three subprocesses — the tool's version from
git, Compose's own version, and looking inside the php container for Xdebug — one after another,
because that is all a shell can do.

## What Changes

- **Compose configuration is read with `compose-go`**, the library Compose itself uses to parse,
  merge, interpolate and validate. 1.7 ms instead of 58 ms for the same answer, checked against
  `docker compose config` by a test rather than assumed.
- **`hm describe` is Go**, byte for byte what the shell one answers: the document, the table with
  colour off, the credentials when asked for and never otherwise, and the same exit codes.
- **The six independent questions are asked at once.** They do not depend on each other, and this
  is the command that runs most often: 285 ms to 94 ms.
- **A defect the comparison found**: the Go properties reader did not merge the tool's own
  defaults with the project's, so every value a project never set — `WORKDIR_PHP`, the compose
  file names, the mail catcher — came out empty. It looked right until the two were compared.
