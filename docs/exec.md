# hm exec

Execute an action on the container.

## Usage

```bash
hm exec [OPTIONS]
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `exec` command is used to execute an action on the `phpfpm` container.

```bash
hm exec -r
```

## Options

| Name                    | Description                                 | Example             |
| ----------------------- | ------------------------------------------- | ------------------- |
| `-r`, `--root`          | Execute with root user                      | `hm exec -r`        |