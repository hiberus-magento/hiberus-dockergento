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

| Name                     | Description                                             | Example                     |
| ------------------------ | ------------------------------------------------------- | --------------------------- |
| `-s` , `--switch`        | Stop all docker containers before starting.             | `hm start -s`               |

