# Open the database in the client you already use

## Why

Connecting a GUI client to a project's database means finding four things — host port, user,
password, database name — that the tool already knows and nobody remembers. So people keep a
saved connection per project, and the saved connection goes stale the moment a port changes or a
project is recreated.

DDEV has `ddev tableplus` and `ddev sequelace` for exactly this, and they are among the first
things anybody who has used it asks for here.

## What Changes

- **`hm tableplus`**, **`hm sequelace`** and **`hm dbeaver`** open that client, connected to this
  project's database.
- **`--print`** prints the connection string instead of opening anything, for any other client,
  for a script, or for pasting.
- When the client is not installed, the connection string is printed anyway, with the name of
  what was missing. A command that says "not found" and nothing else has wasted the trip.
- When the project publishes no database port — which is what the global proxy does — the
  command says so and names `hm tunnel db`, rather than opening a client that will sit there
  failing to connect. `--port` takes the port a tunnel is already holding open.
