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

#### Make a query

```bash
hm mysql -q "SELECT * FROM core_config_data"
```

> You can combine with -d option
## Options

| Name                     | Description                                                                                                                     | Example                          |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------- | -------------------------------- |
| `-d`, `--definers`       | Clean DEFINER in sql file and create a new file in the same path with name <original_name>_cleaned.sql. Must be used with `-i`. | `hm mysql -d`                    |
| `-i`, `--import`         | Import database and set configs for local environment.                                                                          | `hm mysql -i /dump.sql`          |
| `-q`, `--query`          | It is used to make queries.                                                                                                     | `hm mysql -q "SELECT * FORM ..."`|
