# hm create-project

Create a new Magento project

## Usage

```bash
hm create-project [OPTIONS]
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `create-project` command is used to install Magento project and create local environment for this Magento.

This command offers a complete comfort to create your docker environment. It provides multiple options and implements a logic to improve its usage.

Manage to create a Magento project in no time.


## Examples
#### Create project and answer the questions

```bash
hm create-project
```

#### Create a Magento project with default settings.

```bash
hm create-project -u
```
- **Project name**: folder name in lowercase.
- **Domain**: folder name in lowercase plus suffix `.local`.
- **Root directory**: `"."`.
- **Edition**: community.
- **Version**: newest version.

#### Create project with custom options

```bash
hm create-project -e community -v 2.4.5 -p project-name
```

> You can combine it with the `-u` option: an option given explicitly wins over its default.

## Options

Each takes its value as the next word or, in the long form, after `=` (`--version=2.4.7`). Any
other option, or an argument that is not an option's value, is a usage error (exit 2).

| Name                     | Description                                             | Example                            |
| ------------------------ | ------------------------------------------------------- | ---------------------------------- |
| `-u`, `--use-default`    | Use default settings (see above).                       | `hm create-project -u`             |
| `-e`, `--edition`        | Define Magento edition (community \| enterprise).       | `hm create-project -e community`   |
| `-p`, `--project-name`   | Project name; the domain is `<project-name>.local`.     | `hm create-project -p my_project`  |
| `-r`, `--root-directory` | Path to root directory where Magento project is.        | `hm create-project -r "./project"` |
| `-v`, `--version`        | Define Magento version (`hm compatibility` lists them). | `hm create-project -v 2.4.5-p1`    |
