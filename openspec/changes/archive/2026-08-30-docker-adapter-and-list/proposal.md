# The daemon, through its API, and the first command across

## Why

Everything this tool does with Docker it does by running `docker` and reading what comes out. That
costs a process per question and a format string that three different readers have to agree on,
and it is a contract nobody promised: the output of a CLI can change between versions.

This is the stage where that stops. And the way to know it stopped correctly is to port one
command and compare the two implementations, byte for byte, rather than to reason about it.

`list` is that command: read-only, entirely about Docker, with a JSON contract to compare and a
table to compare. Nothing it does can damage anything.

## What Changes

- **An adapter for the daemon** over its API, with the container inventory in one query instead of
  one per environment.
- **It finds the daemon the way the CLI does.** The SDK reads `DOCKER_HOST` and nothing else,
  while Colima, Docker Desktop and Rancher all leave it unset and keep the socket in the CLI's
  context store. Without this the binary reports "Docker is not running" on a machine where it is.
- **`hm list` is Go**, and answers exactly what the shell one answered — the same table, the same
  document, the same exit codes.
- The grouping of containers into environments is a use case tested against fakes, so the cases
  that matter — an environment whose directory is gone, a project without our labels, a container
  missing one — are exercised without a daemon.
- **A defect found by the comparison**: the shell implementation sorted the list with `sort`,
  which is locale-dependent. `magento_dev` came before `magento-demo` under one locale and after
  it under another. Now sorted in the C locale, which is what makes the two agree at all.
