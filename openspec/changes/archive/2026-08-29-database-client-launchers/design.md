# Design

## Three commands, one implementation

`tableplus`, `sequelace` and `dbeaver` are the same command with a different last line, so they
are three files of four lines over one task. Naming them separately is the point: `hm tableplus`
is what somebody types when they want TablePlus, and a `--client=` option would be a worse way of
saying the same thing.

## The connection comes from the same place everything else does

Host port, user, password and database name are read from the resolved compose configuration, the
way `hm describe --with-secrets` reads them. Nothing is guessed and nothing is duplicated: a
project that renamed its database or changed its port gets the right connection without anybody
updating a saved profile.

## Not opening a client that cannot connect

A project routed through the global proxy publishes no database port — the overlay removes it,
because MySQL carries no hostname and Traefik cannot route it. Opening TablePlus at
`127.0.0.1:3306` there would connect it to whatever else is listening, or to nothing.

So when there is no published port the command stops and names `hm tunnel db`, which is the thing
that opens one. It does not open the tunnel itself: `hm tunnel` deliberately runs in the
foreground, so that the door closes when the command ends, and a launcher that opened a detached
relay would leave one behind for somebody to find later. `--port` exists for when a tunnel is
already open.

## A missing client is not a dead end

`open -a TablePlus` fails when TablePlus is not installed. What the person needed was the
connection, so they get it: the string is printed with the name of the missing application. It
costs three lines and turns a failed command into a useful one.
