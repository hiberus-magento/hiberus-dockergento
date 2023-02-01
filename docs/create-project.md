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
hm create-project -d
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

> You can combine it with -d option
## Options

| Name                     | Description                                             | Example                            |
| ------------------------ | ------------------------------------------------------- | ---------------------------------- |
| `-d`, `--default`        | Use default settings.                                   | `hm create-project -d`             |
| `-e`, `--edition`        | Define Magento edition (community \| enterprise).       | `hm create-project -e community`   |
| `-p`, `--project-name`   | Project path. Use relative path.                        | `hm create-project -p ./my_project`|
| `-r`, `--root-directory` | Path to root directory where Magento project is.        | `hm create-project -r "./project"` |
| `-v`, `--version`        | Define Magento version,                                 | `hm create-project -v 2.4.5-p1`    |
