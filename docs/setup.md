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
* From root Magento directory or parent directory of Magento project

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
execute	* If you use `-i` option, during setup process the CLI will execute the standard installation of Magento (`bin/magento setup:install`)

		* Before executing this command you will be asked for basic configurations.
			`hm setup -i`
		* You can put `-u` to enable the usage of your stored settings.
			`hm setup -u -i`
	* If you use `-q` option, during setup process the CLI will not execute installation of Magento and will import the referenced dump
		  `hm setup -q="/path-to-folder/dump.sql"`
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
	|__ docker-composedev.linux.yml
	|__ docker-composedev.mac.yml
	|__ <magento_root_directory>/
		|__ app/
		|__ ...
```

## Options

| Name                     | Description                                             | Example                                |
| ------------------------ | ------------------------------------------------------- | -------------------------------------- |
| `-d` , `--domain`        | Domain of project. Only accept lowercase.               | `hm setup -d my-domain.local`          |
| `-D`, `--dump`           | Path to sql dump to import.                             | `hm setup -D="/User/..../dump.sql"`    |
| `-f`, `--force`          | Used for rebuilt `docker-compose` file.s                | `hm setup -f`                          |
| `-i`, `--install`        | Execute Magento installation.                           | `hm setup -i`                          |
| `-p`, `--project-name`   | Project path. Use relative path.                        | `hm setup -p ./my_project`             |
| `-r`, `--root-directory` | Path to root directory where Magento project is.        | `hm setup -r "./project"`              |
| `-u`, `--use-default`    | Use default user settings.                              | `hm setup -u -i`                       |
