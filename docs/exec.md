# hm exec

Execute a command on the container.

## Usage

```bash
hm exec [-r] <command> [arguments]
```

## Description

The `exec` command runs a command in the `phpfpm` container and returns that command's exit code.
A terminal is allocated only when stdin and stdout are terminals, so the same line works by hand
and in CI.

```bash
hm exec ls -lah
hm exec -r chown -R app:app var
```

Without a command it is a usage error (`missing_command`, exit 2). Everything after `exec`
belongs to the command, so the tool's own flags go before it: `hm --json exec ...`.

## Options

| Name                    | Description                                         | Example                  |
| ----------------------- | --------------------------------------------------- | ------------------------ |
| `-r`                    | Execute with root user. Only as the first argument. | `hm exec -r whoami`      |
