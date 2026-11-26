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

### Progress

On a terminal, the steps that can take minutes without printing anything say they are still
running:

- the download over SSH shows how much of the dump has arrived (`Downloading the dump: 412 MB received (3m10s)`). Nothing is drawn until the first byte arrives, so a passphrase, host key or second-factor question from SSH stays readable;
- the dump without SSH, saving `core_config_data`, the restore and putting `core_config_data` back show their name and how long they have taken.

Under `--json`, `NO_COLOR`, `HM_NO_PROGRESS` or without a terminal nothing is drawn.

SSH runs with `ConnectTimeout=30`, `ServerAliveInterval=30` and `ServerAliveCountMax=4`: a host that does not answer, or a connection that drops during the dump, ends the transfer with an error instead of waiting forever. Authentication stays interactive: keys with a passphrase, the agent and host key confirmation work as before.

### Failures

The transfer stops at the first step that fails, and says which step, why, and the last lines that step printed:

```text
Creating the database dump on ssh.eu-3.magento.cloud failed (the dump exited with status 2):
  mariadb-dump: Got error: 2013: Lost connection to server during query when dumping table `sales_order` at row: 1200
The whole transfer is logged in /home/me/.hm/logs/transfer-db-shop-20261126-101500.log
```

The exit code is the failed step's status, and under `--json` the same message is the `error.message` of the failure document, with type `transfer_failed`.

- **The dump.** Each part of the dump pipeline (`mariadb-dump | sed | gzip`) reports its own failure, which works in any login shell on the remote server, `sh` included. A failed dump is never copied or restored.
- **Error 1412.** When the dump fails with `Error 1412: Table definition has changed, please retry transaction` (a table altered on the live database while it was being read), only the dump is taken again, up to three attempts in all, saying `The table definition changed during the dump, retrying (2/3)...`. Any other error stops at once.
- **The restore.** The restore runs the client with `--force`, so a statement this server refuses does not leave half a database. The statements it skipped are counted and reported at the end (`The restore skipped 2 SQL errors; the first: ERROR 1146 (42S02) at line 12: ...`), and all of them are in the log. A dump that cannot be read to the end, or a client that cannot connect, stops the transfer.
- **`bin/magento` steps** print on the terminal as they run; when one fails, the failure names it and its status.

### Log

Every run writes a log to `~/.hm/logs/transfer-db-<project>-<date>-<time>.log` (next to `HM_STATE_DIR` when that variable moves hm's state elsewhere): each step with its time and command, what it printed and how it ended, every dump attempt included. A finished transfer prints the path after `All done!`, and every failure ends with it. The remote database password never appears in the log or on the terminal: it is written as `***`. If the log cannot be created the transfer says so and goes on without it.

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