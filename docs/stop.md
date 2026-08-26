# stop

Stops the environment's services, keeping the containers and everything in them.

```bash
hm stop
hm stop phpfpm         # one service
hm stop --snapshot     # save the database first
```

`hm start` brings it back exactly as it was.

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

## What it does not do

It does not remove containers or volumes — that is [`hm down`](down.md). It does not free disk
space, and a stopped environment still holds its ports.
