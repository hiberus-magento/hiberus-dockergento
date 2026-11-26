# hm masquerade

Command to anonymise the database. With the help of a library for creating auto-generated content [FakerPHP](https://fakerphp.github.io/), it is possible to replace sensitive database information with similar invented content. Additional useful information on the types of 'formatter' that exist can be found in the [documentation](https://fakerphp.github.io/formatters/).

## Usage

```bash
hm masquerade
```

## Description

The `masquerade` command is used to anonymise the database of a local Magento project. It asks
for confirmation first, because it cannot be undone (`--yes` answers it). It runs the
`hiberusmagento/masquerade` image as a one-off container on the database's network, with the
database credentials read from the db container.

Internally, the masquerade tool has default configuration files that we find useful across the board for Magento projects, but you can include project-specific settings: 

To add proyect config files create the following folder: config/docker/masquerade/magento2 and include de yaml files (.yaml) inside the newly created folder (magento2). If these guidelines are not followed, the tool will not take the added files into account.

## Transfer DB

The same anonymisation is offered by [transfer-db](transfer-db.md): it asks whether to anonymise
the transferred database (yes by default), and `--anonymise=true` or `--anonymise=false` answers
it on the command line. `hm mysql -a -i dump.sql` anonymises after an import.