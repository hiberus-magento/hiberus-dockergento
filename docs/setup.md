# hm setup

Create and start all neccesary service for Magento 2 local environment

## Usage

```bash
hm setup [OPTIONS]
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `setup` command is used for creating local environment for differents Magento 2 versions and editions.

This command offers a complete comfort to create your docker environment. It disponses of multiples options and implements a logic to improve the usage.

You can use for:
* Create docker environment for existing project.
* Create docker environment for existing project from parent folder.
* Install dockerized project with `hiberus dockergento` after git clone.
* Rebuilt environment settings to change services or service versions.

## Cases
---
### Case project without hiberus dockergento
Project without dockerize or with a dockerize different to `hiberus dockergento`.

#### Steps
* From root Magento directory or parent directory of Magento project

```bash
hm setup [ OPTIONS ]
```

* In the terminal you should answer the questions. The CLI proposes default response if base your location and settings.
---
### Case project with hiberus dockergento
Project is dockerized with `hiberus dockergento` and execute:

#### Steps
* Position in root of project.

```bash
hm setup [ OPTIONS ]
```

* If project has properties set in `./config/docker/properties.json`, the CLI will just ask the settings not existing in this file.
	* If you use `-i` option, during setup process the CLI will be standard installation of Magento (`bin/magento setup:install`)
		* Before execute this command you will be asked for basic configurations.
			`hm setup -i`
		* You can put `-u` to enable the usage of your stored settings.
			`hm setup -u -i`
	* If you use `-q`option, during setup process the CLI will not be installation of Magento and will import the referenced dump
		  `hm setup -q="/path-to-folder/dump.sql"`
---
## Configuration files out of magento progent

* If you want the files are out of your project to doesn't be tracked by git.

```bash
cd wrapper_folder
hm setup --root-directory="<magento_root_directory>"
```

This will be the result
```bash
./wrapper_folder
	|__ config/
	|__ docker-compose.yml
	|__ docker-composedev.linux.yml
	|__ docker-composedev.mac.yml
	|__ <magento_root_directory>/
		|__ app/
		|__ ...
```

## Options

| Name                     | Description                                             | Example                                |
| ------------------------ | ------------------------------------------------------- | -------------------------------------- |
| `-d` , `--domain`        | Domain of project. Only acept lowercase                 | `hm setup -d my-domain.local`          |
| `-D`, `--dump`           | Path to sql dump to import                              | `hm setup -D="/User/..../dump.sql"`    |
| `-f`, `--force`          | Used for rebuilt `docker-compose` files                 | `hm setup -f`                          |
| `-i`, `--install`        | Execute magento instalation                             | `hm setup -i`                          |
| `-p`, `--project-name`   | Project path. Use relative path                         | `hm setup -p ./my_project`             |
| `-r`, `--root-directory` | Path to root directory when is magento project          | `hm setup -r "./project"`              |
| `-u`, `--use-default`    | Use default user settings                               | `hm setup -u -i`                       |
