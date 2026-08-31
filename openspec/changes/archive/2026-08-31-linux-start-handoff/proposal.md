# Starting is one command on both platforms again

## Why

`start` and `restart` were ported to Go for macOS only. On Linux the whole command went to the
shell implementation, because starting there also matches the container's user and group ids to
the host's and writes the project's domains into the container's `/etc/hosts`.

The milliseconds were never the point of fixing that. The cost of a command that exists twice is
that everything since — the proxy handling, the port-conflict refusal, the structured errors, the
bind-mount check — reached macOS and not Linux, and every change from here would have to be made
in two places or silently apply to one platform.

The reason those steps are not ported is concrete and does not go away by trying harder: the
second of them reads the project's domains out of the database through `hm mysql`, which is not
ported. Porting this would mean porting that first.

## What Changes

- **`start` and `restart` are Go on both platforms.** The Compose part, the proxy, the refusals
  and the error contract are one implementation.
- **`hm post-start` is a command**, holding the steps that only Linux needs. It is a command and
  not a block inside `start` because two things bring environments up now, and one copy of those
  steps is the difference between them agreeing and them drifting. On macOS it does nothing, and
  it is not even invoked there — asking would cost a shell process on a command where that is a
  fifth of the time.
- **A defect this uncovered, on Linux**: the self-routing entries point at Hitch, and a project
  routed through the global proxy has no Hitch — the proxy overlay deletes it, because Hitch was
  only there to give Varnish the HTTPS it does not have. The step demanded it anyway, so `hm
  start` brought the whole environment up and then failed with "Service 'hitch' is not running".
  Every time, for every project on the proxy, on Linux.
