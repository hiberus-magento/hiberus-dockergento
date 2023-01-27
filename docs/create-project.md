# hm create-project

Create a new magento project

## Usage

```bash
hm create-project [OPTIONS]
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `create-project` command is used for installing Magento project and creating local environment for this Magento.

This command offers a complete comfort to create your docker environment. It disponses of multiples options and implements a logic to improve the usage.

Create a new magento project in a few minutes.


## Examples
#### Create project and answer the questions

```bash
hm create-project
```

#### Create magento with default settings

```bash
hm create-project -d
```
- **Project name**: folder name in lowercase.
- **Domain**: folder name in lowercase plus subfix`.local`
- **Root** directory: "."
- **Edition**: community
- **Version**: last version

#### Create project with custom options

```bash
hm create-project -e community -v 2.4.5 -p project-name -d domain.local -r .
```

> You can combine with -d option
## Options

| Name                     | Description                                             | Example                                         |
| ------------------------ | ------------------------------------------------------- | ----------------------------------------------- |
| `-d`, `--default`        | Use default settings                                    | `hm create-project -d`                          |
| `-e`, `--edition`        | define magento edition (community|entrerprise)          | `hm create-project -e community`                |
| `-p`, `--project-name`   | Project path. Use relative path                         | `hm create-project -p ./my_project`             |
| `-r`, `--root-directory` | Path to root directory when is magento project          | `hm create-project -r "./project"`              |
| `-v`, `--version`        | Define magento version                                  | `hm create-project -v 2.4.5-p1`                 |
