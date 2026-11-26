# hm mysql

Command to import the database, to clean `DEFINER`, to make queries and to set configs after importing that database. 

## Usage

```bash
hm mysql [OPTIONS] | < dump.sql
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `mysql` command is used to manage the database of Magento project.

This command increment native functionality during import process.

## Examples
#### Import database

```bash
hm mysql < /.../dump.sql
```
> This way doesn't set config

#### Import database and set configs after import

```bash
hm mysql -i /.../dump.sql
```

#### Clean DEFINERs, import database and set configs after import 

```bash
hm mysql -d -i /.../dump.sql
```

#### Import database and anonymise it

```bash
hm mysql -a -i /.../dump.sql
```

#### Make a query

```bash
hm mysql -q "SELECT * FROM core_config_data"
```

Without options and with a terminal on stdin, it opens an interactive client.

## Options

Only the short forms are accepted; any other option is a usage error (exit 2).

| Name   | Description                                                                                                       | Example                           |
| ------ | ----------------------------------------------------------------------------------------------------------------- | --------------------------------- |
| `-i`   | Import a dump and set configs for local environment (`config/docker/local_settings.json`, see [config-env](config-env.md)). | `hm mysql -i /dump.sql`           |
| `-d`   | With `-i`: remove the `DEFINER` clauses while importing. The dump is filtered as it streams; no second file is written. | `hm mysql -d -i /dump.sql`        |
| `-a`   | With `-i`: anonymise the database after importing ([masquerade](masquerade.md)).                                  | `hm mysql -a -i /dump.sql`        |
| `-q`   | Run one statement and print its output.                                                                           | `hm mysql -q "SELECT * FROM ..."` |
