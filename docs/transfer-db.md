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

### Tables without data

Some tables are dumped with their structure but without their rows: nobody needs them on a development machine, and a live store alters some of them while the dump reads them, which is what makes a dump fail with error 1412. Three layers decide which:

1. **The default.** `TRANSFER_DB_STRUCTURE_ONLY` is `@saas` unless the project sets it: the Adobe Commerce data exporter feeds and their batch and sync tables.
2. **The project.** `TRANSFER_DB_STRUCTURE_ONLY` in `config/docker/properties.json` replaces the default: a comma-separated list of `@groups`, table names and `*` patterns. Keep `@saas` in it if you still want it; an empty value dumps every table with its data.

   ```json
   "TRANSFER_DB_STRUCTURE_ONLY": "@saas,@log,@sessions,my_vendor_import_*"
   ```

3. **The run.** `--structure-only=<list>` adds to the project's list for one run; `--full` dumps every table with its data, ignoring both.

The patterns are matched on the server the dump is taken from, against the tables `information_schema` lists for the database, in the same SSH session as the dump (no second connection, no second password prompt). The dump is then two passes into one file: the structure of the matched tables, and every other table with its data. Without SSH the same happens in the `db` container. A pattern may use letters, digits, `_`, `$` and `*` (any run of characters); anything else, or an unknown group, stops the transfer before it asks anything.

The run says which rules apply and which tables came without data:

```text
Tables to dump without data: @saas (--full dumps them with their data)
Creating database dump from origin server...
Dumped without data (2 tables): cde_products_feed, sales_data_exporter_orders_v2_sync_batches
```

With more than ten tables the line gives the count and the log's path, and the log names every table.

The groups are [n98-magerun2's `db:dump` table groups](https://github.com/netz98/n98-magerun2/blob/10.1.0/config.yaml) (as of 10.1.0), under the same names, plus `@saas`:

| Group | Tables |
| ----- | ------ |
| `@saas` | Data exporter feeds and queues: `cde_*`, `*_data_exporter_*`, `*_sync_batches`, `*_index_batches` |
| `@admin` | `admin*`, `magento_logging_event*`, `ui_bookmark`, `authorization_role`, `authorization_rule` |
| `@oauth` | `oauth_*` |
| `@log` | `log_url`, `log_url_info`, `log_visitor*`, `report_event`, `report_compared_product_index`, `report_viewed_*` |
| `@sessions` | `session`, `persistent_session` |
| `@stripped` | `@log @sessions @dotmailer @newrelic_reporting @temp @ee_changelog @indexer_batches system_config_snapshot` |
| `@sales` | Orders, invoices, shipments, credit memos, their sequences and grids, RMA, PayPal, `inventory_reservation` |
| `@quotes` | `quote`, `quote_*`, `negotiable_quote*` |
| `@customers` | Customers, addresses, wishlists, newsletter subscribers, vault tokens, companies, reward and balance |
| `@trade` | `@customers @sales @quotes @klarna @mailchimp` |
| `@development` | `@admin @oauth @trade @stripped @search @2fa @aggregated` |
| `@ee_changelog` | `*_cl` |
| `@indexer_batches` | `*_index_batches` |
| `@search` | `catalogsearch_*`, `search_query`, `search_synonyms` |
| `@idx` | `*_idx` |
| `@dotmailer` | `email_*` tables of dotdigital |
| `@2fa` | `msp_tfa_user_config`, `msp_tfa_trusted` |
| `@newrelic_reporting` | `reporting_*` |
| `@aggregated` | `*_aggregated`, `*_aggregated_*` |
| `@replica` | `*_replica` |
| `@klarna` | `klarna_*` |
| `@temp` | `*__temp*` |
| `@mailchimp` | `mailchimp_*` |

The exact lists are in `data/table-groups.json`.

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
| `--structure-only` | Also dump these tables without data: `@groups`, names and `*` patterns, added to `TRANSFER_DB_STRUCTURE_ONLY` | `hm transfer-db --structure-only=@log,@sessions` |
| `--full`          | Dump every table with its data, ignoring `TRANSFER_DB_STRUCTURE_ONLY` and `--structure-only` | `hm transfer-db --full` |