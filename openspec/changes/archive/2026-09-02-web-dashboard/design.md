# Design

## Like the proxy, not like a dev server

A dev server holds the terminal it was started from and dies with it. That is right for something
you are editing and wrong for something you glance at. This is machine-wide and long-lived, like
the proxy: `up`, `down`, `status`, and a link.

Detaching is a session of its own so that closing the terminal does not take it down, a state file
beside the proxy's holding the pid, the port and the token, and a log next to it. `down` signals
the pid; `status` reports; starting it twice finds the first one rather than fighting it for the
port.

`up` waits until the server answers before returning. It is the difference between a link that
works and a link that works the second time.

## Two rules, and why they are not optional

The API reads database credentials on request and stops environments. Two things stand in front of
it:

- **Loopback only.** Not a default somebody can widen — a laptop on a shared network is the normal
  case, not the exception.
- **A token, on every request.** It travels in the link the command prints, which is how a browser
  can be given it without a header. The state file holding it is readable only by its owner.

The second rule is not enough on its own. A name on the internet can be pointed at 127.0.0.1, and
then a page served from it is same-origin with this server as far as the browser is concerned — it
can read the responses. Refusing any `Host` that is not loopback stops that, and costs one
comparison.

What is deliberately absent: user accounts, sessions, TLS. There is one user, it is whoever is
sitting at the machine, and adding a login to a loopback socket protects nobody from anything.

## The page, and no build step

One HTML file with its CSS and its script, embedded in the binary. A bundler would mean Node in
the release pipeline, which today builds four binaries on a macOS runner with nothing but Go.

That is a first version and not a position. The contract is the API; when the page needs more than
one file can hold, it becomes a TypeScript build that produces static files, and the API does not
notice.

## Where the flags are parsed

The tool's own flags are now taken out wherever they appear, before dispatching — which is what the
shell implementation does, including its rule that for commands whose output is a child process's
(`exec`, `magento`, `composer`, `logs`, …) a flag after the command name belongs to the child.

`hm --no-json serve` reaching the shell entry point first is why the resolved format is also read
back from the environment: bash parses the flags itself and exports the answer, and without
reading it the format was lost crossing over.
