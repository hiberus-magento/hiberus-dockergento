# stop

Stops the environment's services, keeping the containers and everything in them.

```bash
hm stop
hm stop phpfpm         # one service
hm stop --snapshot     # save the database first
hm stop -t 5           # give each container 5 seconds before killing it
```

`hm start` brings it back exactly as it was.

## `-t`, `--timeout`

How many seconds each container is given to stop before it is killed. Without it, the wait is
Compose's own default (the service's `stop_grace_period`, or 10 seconds). `0` kills at once.

```bash
hm stop -t 5
hm stop --timeout=30 db
```

It is written `-t N`, `-t=N`, `--timeout N` or `--timeout=N`. A missing value, or one that is not
a whole number of seconds, is a usage error (exit 2), and so is any other option: it is refused
rather than taken for the name of a service.

## `--snapshot`

Stopping is often the last thing you do before leaving a project, and sometimes the step before a
`hm down -v`. `--snapshot` saves the database first, using [`hm db snapshot`](db.md):

```bash
hm stop --snapshot
```

The copy is asked for rather than always taken: stopping is an everyday, quick operation, and one
that sometimes takes a minute because it is dumping a database would be an unpleasant surprise.

**If the snapshot fails, the environment is left running.** A stopped environment and no copy,
after asking for one, is the worst of the three possible outcomes.

## JSON output

With `--json`, a successful stop prints one envelope on stdout (see [JSON output](json.md)):

```json
{
  "schema_version": 1,
  "command": "stop",
  "ok": true,
  "data": {
    "elapsed_ms": 3120,
    "project": "my-project",
    "services": [],
    "snapshot": true,
    "timeout": 5,
    "whole_environment": true
  }
}
```

`timeout` is `null` when `-t` was not given. An empty `services` list means the whole environment.

## What it does not do

It does not remove containers or volumes — that is [`hm down`](down.md). It does not free disk
space, and a stopped environment still holds its ports.
