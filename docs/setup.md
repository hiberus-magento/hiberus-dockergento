# hm setup

Create and start every necessary service for Magento 2 local environment.

## Usage

```bash
hm setup [OPTIONS]
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `setup` command is used to create local environment for different Magento 2 versions and editions.

This command offers a complete comfort to create your docker environment. It provides multiple options and implements a logic to improve its usage.

You can use it to:
* Create docker environment for existing project.
* Create docker environment for existing project from parent folder.
* Install dockerized project with `Hiberus Dockergento` after git clone.
* Rebuilt environment settings to change services or service versions.

## Cases
---
### Case project without Hiberus Dockergento
Project without a docker or with a docker different to `Hiberus Dockergento`.

#### Steps
* From root Magento directory or parent directory of Magento project.

```bash
hm setup [ OPTIONS ]
```

* In the terminal you should answer the questions. The CLI proposes default response basing in your location and settings.
---
### Case project with Hiberus Dockergento
Project has a docker with `Hiberus Dockergento`:

#### Steps
* Position in root of project.

```bash
hm setup [ OPTIONS ]
```

* If project has properties saved in `./config/docker/properties.json`, the CLI will just ask for settings not existing in this file.
* With `-i` (`--install`, `--clean-install`) the CLI runs the standard installation of Magento (`bin/magento setup:install`), asking for its basic configuration first: `hm setup -i`.
  * Add `-u` to use your stored settings: the installation runs as `hm install -u`, which takes the store and admin settings from `~/.hm/config.json` instead of asking: `hm setup -u -i`.
* With `-D <file>` (`--dump`, `--db-dump`) the CLI imports that dump instead of installing Magento: `hm setup -D /path-to-folder/dump.sql`.

Under `--yes`, one of the two must be given: see [install](install.md#bootstrapping-without-being-asked-anything).
---
## Configuration files out of Magento project

* If you want the docker configuration files to be out of your project not to be tracked by git.

```bash
cd wrapper_folder
hm setup --root-directory="<magento_root_directory>"
```

This will be the result
```bash
./wrapper_folder
	|__ config/
	|__ docker-compose.yml
	|__ docker-compose.dev.linux.yml
	|__ docker-compose.dev.mac.yml
	|__ <magento_root_directory>/
		|__ app/
		|__ ...
```

## Options

An option with a value takes it as the next word or, in the long form, after `=`
(`--domain=shop.test`); the short forms take it only as the next word. Any other option is a usage
error (exit 2).

| Name                     | Description                                             | Example                                |
| ------------------------ | ------------------------------------------------------- | -------------------------------------- |
| `-d` , `--domain`        | Domain of project. Only accept lowercase.               | `hm setup -d my-domain.local`          |
| `-D`, `--dump`, `--db-dump` | Path to sql dump to import.                          | `hm setup -D /User/..../dump.sql`      |
| `-f`, `--force`          | Regenerates the `docker-compose` files. `--force` also lifts the worktree protection, as it does for every command. | `hm setup -f`, `hm setup --force` |
| `-i`, `--install`, `--clean-install` | Execute Magento installation.               | `hm setup -i`                          |
| `-p`, `--project-name`   | Project name; the default domain is `<project-name>.local`. | `hm setup -p my_project`           |
| `-r`, `--root-directory` | Path to root directory where Magento project is.        | `hm setup -r "./project"`              |
| `-u`, `--use-default`    | Use default user settings.                              | `hm setup -u -i`                       |
| `--mail`                 | Mail catcher for this project: `mailhog` (default) or `mailpit`. See [mail](mail.md). | `hm setup --mail=mailpit` |
