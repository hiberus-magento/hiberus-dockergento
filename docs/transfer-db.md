# hm transfer-db

This command transfers database from external source to local and import.

## Usage

```bash
hm transfer-db [OPTIONS]
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `transfer-db` command is used to transfer database from external source.

On a terminal, copying the dump into the database container shows the same progress line as
[`copy-to-container`](copy-to-container.md#progress), against the size of the dump on this machine.

## Options

| Name              | Description                                  | Example           |
| ----------------- | ------------------ | ------------------------------------------- |
| `--ssh-host`      | Ssh host name.     | `hm transfer-db --ssh-host <host-name>`     |
| `--ssh-user`      | Ssh host name.     | `hm transfer-db --ssh-user <host-user>`     |
| `--sql-host`      | Database host      | `hm transfer-db --sql-host <host-name>`     |
| `--sql-user`      | Database user      | `hm transfer-db --sql-user <user>`          |
| `--sql-db`        | Database name      | `hm transfer-db --sql-db <db-name>`         |
| `--sql-password`  | Database password  | `hm transfer-db --sql-password <password>`  |
| `--anonymise`     | Anonymise database. Values: `true`, `false`. If omitted, prompts user | `hm transfer-db --anonymise=true` or `hm transfer-db --anonymise=false` |