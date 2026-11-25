# hm start

Start project containers or specific container.

## Usage

```bash
hm start [OPTIONS] [ARGUMENT]
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `start` command is used to start service containers.

If you change to another project, you can use `-s` to stop containers before starting current project containers.

You can start specific service container:

```bash
hm start [ phpfpm | mginx | ... ]
```


## Options

| Name               | Description                                  | Example        |
| ------------------ | -------------------------------------------- | -------------- |
| `-s` , `--switch`  | Stop all docker containers before starting.  | `hm start -s`  |

## JSON output

With `--json`, a successful start prints one envelope on stdout (see [JSON output](json.md)):

```json
{
  "schema_version": 1,
  "command": "start",
  "ok": true,
  "data": {
    "elapsed_ms": 8421,
    "project": "my-project",
    "services": [],
    "stopped_others": false,
    "url": "https://my-project.local/",
    "whole_environment": true
  }
}
```

`services` lists the services named on the command line; an empty list means the whole
environment. `url` is `null` when the project has no domain. A failure prints its error envelope on
stderr instead, as before.