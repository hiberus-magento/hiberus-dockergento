# hm mysql

Command to importing database, cleaning DEFINER, making queries and seting configs after import database. 

## Usage

```bash
hm mysql [OPTIONS] | < dump.sql
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `mysql` command is used for managing database of Magento project.

This command increment basic funcionality during import process

## Examples
#### Import database

```bash
hm mysql < /.../dump.sql
```
> This way not sets config

#### Import database and set configs after import

```bash
hm mysql -i /.../dump.sql
```

#### Clean DEFINERs, import database and set configs after import 

```bash
hm mysql -d -i /.../dump.sql
```

#### Mak a query

```bash
hm mysql -q "SELECT * FROM core_config_data"
```

> You can combine with -d option
## Options

| Name                     | Description                                                                                                 | Example                          |
| ------------------------ | ----------------------------------------------------------------------------------------------------------- | -------------------------------- |
| `-d`, `--definers`       | Clean DEFINER in sql and create file in the same path with name <original_name>_cleaned.sql. Used with `-i` | `hm mysql -d`                    |
| `-i`, `--import`         | Import database and set configuration for local environment                                                 | `hm mysql -i /dump.sql`          |
| `-q`, `--query`          | It is used to make queries                                                                                  | `hm mysql -q "SELECT * FORM ..."`|
