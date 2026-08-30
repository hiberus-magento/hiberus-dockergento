# `doctor` in Go

## Why

The diagnosis is the command with the most outside world in it: seventeen checks, five of which
ask Docker, four of which ask the machine, and the rest of which read files. It is also the
command that answers the question somebody has when nothing works — which makes it the worst one
to be slow, and the worst one to lose a line from.

In the shell implementation each check is a separate process with a five-second alarm, and the
Compose configuration is resolved five separate times because five checks want it. Loading it once
and asking the seventeen questions concurrently is most of the difference between 260 ms and
85 ms.

## What Changes

- **`hm doctor` is Go**, all seventeen checks, identical to the shell implementation: the same ids
  in the same order, the same wording, the same actions, the same exit code, and the same report
  byte for byte with colour both on and off.
- **The two expensive answers are asked once** — the container list and the Compose configuration
  — instead of once per check that wants them.
- **Two things a check needed a subprocess for, it no longer does**: the certificate's expiry is
  read from the certificate rather than from `openssl`, and the memory of the machine from the
  kernel rather than from `sysctl`.
- **A defect fixed on the way**: on Linux, where ports are listed with `ss`, a conflict was
  reported as taken by a process called "LISTEN" — the first column of the wrong line. `ss` names
  no process, so the message now says "processes on the host", which is true.
- **A defect the unit tests caught**: the fingerprint that decides whether the generated agent
  context is stale is an md5 of what `jq` printed, and `jq` prints a newline. Without it every
  generated context on every project would have read as stale the day the binary shipped.
