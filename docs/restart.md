# hm restart

Restart project containers or specific container.

## Usage

```bash
hm restart [-t seconds] [service...]
```

## Description

The `restart` command is used to restart service containers.

You can restart specific service container:

```bash
hm restart [ phpfpm | mginx | ... ]
```

A restart is a stop and then a start, so a change to the compose configuration is running
afterwards.

## `-t`, `--timeout`

How many seconds each container is given to stop before it is killed, during the restart's stop.
Without it, the wait is Compose's own default (the service's `stop_grace_period`, or 10 seconds).

```bash
hm restart -t 5
hm restart --timeout=30 phpfpm
```

It is written `-t N`, `-t=N`, `--timeout N` or `--timeout=N`. A missing value, or one that is not
a whole number of seconds, is a usage error (exit 2), and so is any other option.

## JSON output

With `--json`, a successful restart prints one envelope on stdout (see [JSON output](json.md)):

```json
{
  "schema_version": 1,
  "command": "restart",
  "ok": true,
  "data": {
    "elapsed_ms": 5230,
    "project": "my-project",
    "services": ["phpfpm"],
    "timeout": null,
    "whole_environment": false
  }
}
```
