# hm masquerade

Command to anonymise the database. With the help of a library for creating auto-generated content [FakerPHP](https://fakerphp.github.io/), it is possible to replace sensitive database information with similar invented content. Additional useful information on the types of 'formatter' that exist can be found in the [documentation](https://fakerphp.github.io/formatters/).

## Usage

```bash
hm masquerade
```

## Description

The `masquerade` command is used to anonymise the database of a local Magento project.

Internally, the masquerade tool has default configuration files that we find useful across the board for Magento projects, but you can include project-specific settings: 

To add proyect config files create the following folder: config/docker/masquerade/magento2 and include de yaml files (.yaml) inside the newly created folder (magento2). If these guidelines are not followed, the tool will not take the added files into account.

## Transfer DB

In addition to being able to manually execute the anonymisation via the masquerade command, this anonymisation has also been included in the [transfer-db](transfer-db.md) command flow with no option to skip it, to ensure that data transferred from an external environment does not contain sensitive information in development environments.