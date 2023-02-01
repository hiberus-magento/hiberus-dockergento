# hm bash

Execute an interactive bash shell on the container.

## Usage

```bash
hm bash [OPTIONS]
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `bash` command is used to launch a bash shell on the `phpfpm` container.

```bash
hm bash -r
```

## Options

| Name                    | Description                                 | Example             |
| ----------------------- | ------------------------------------------- | ------------------- |
| `-r`, `--root`          | Execute with root user                      | `hm bash -r`        |