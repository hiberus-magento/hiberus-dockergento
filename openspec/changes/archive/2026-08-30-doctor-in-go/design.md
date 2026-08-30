# Design

## Seventeen checks, one shape

A check declares its id, whether it needs a project, and a function. The runner stamps the id and
the scope onto whatever it returns, which is what makes the scope a single fact rather than one
repeated in seventeen places — the same thing `doctor_requires_project` did in the shell, minus
the requirement that every check remember to call it.

They run at once and are collected in declaration order, so the report reads the same every time.
A diagnosis whose lines moved between runs would be read as a diagnosis that changed.

Each check is bounded on its own. The shell implementation ran each in its own process with an
alarm, for a good reason: a check that hangs must not take the diagnosis with it. Here the check
is abandoned and the line says it timed out, which is the part that matters — a silent omission
would read as "nothing to report about that".

## Asking twice is the thing being fixed

Five checks want the Compose configuration and two want the container list, and in the shell
implementation each one paid for its own. They are loaded once, before anything starts, and handed
down.

That is also why `compose-config` reports on an error the loader already produced rather than
running `docker compose config -q` again. The wording of an invalid configuration differs from
Compose's own for that reason — the id, the severity and the action do not.

## Two subprocesses that were never needed

`openssl x509 -checkend` was how the certificate's expiry was read, which meant the check silently
did nothing on a machine without openssl — it reported the certificate present and stopped. Go
parses the certificate.

`sysctl -n hw.memsize` and `/proc/meminfo` are the same question with two answers, so they are two
files behind one function, chosen at build time. That is where a platform difference belongs.

## Where the two implementations differ, and why

Three deliberate differences, none of which changes an id, a severity or an action:

- **The process holding a port, on Linux.** `ss` prints no process name and the shell
  implementation took the first column of the line, which is the word `LISTEN`. Reporting no name
  makes the message fall back to "processes on the host".
- **The reason a Compose configuration is invalid**, as above.
- **Paths are resolved against the project root**, not against the working directory. `hm doctor`
  from a subdirectory looked for `ssl.pem` and `composer.lock` where it was standing, and answered
  about files that were not there.

Everything else is compared against the shell implementation by a test, whole: the document, the
report with colour off, the report with colour on, the exit code, and the diagnosis of a directory
that is not a project.

## The fingerprint has to keep matching

`hm ai-context` is still the shell implementation, and the digest it writes into `AGENTS.md` is
what this check compares against. So the value is not recomputed in a nicer shape: it is an md5 of
the same five facts, with the keys sorted and the separators compact, and with the newline `jq`
leaves behind — which is in the digests already sitting in people's checkouts.

The unit test pins it against a value produced by `jq | md5` rather than by this code, because a
test that agrees with the implementation about a hash proves nothing.
