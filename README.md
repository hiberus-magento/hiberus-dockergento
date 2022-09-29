# Hiberus Dockergento

Docker environment for Magento 2 projects. Please visit our [Dockerhub repository](https://hub.docker.com/u/hiberusmagento).

<img alt="Hiberus Dockergento Schema" src="schema.png" width="700"/>

## Docker images

- **Nginx**: `1.18`
- **PHP**: `7.2`, `7.3`, `7.4`, `8.1`
- **MariaDB**: `10.2`, `10.3`, `10.4`
- **RabbitMQ**: `3.9`
- **Redis**: `5.0`, `6.2`
- **ElasticSearch**: `5.6`, `6.5`, `7.17`
- **OpenSearch**: `1.2`
- **Mailhog**: `1.0`
- **Varnish**: `6.0`, `7.1`
- **Hitch**: `1.7`

<br>

## Magento compatible versions

- **Magento 2.4**: `2.4.0`, `2.4.1`, `2.4.2`, `2.4.3`, `2.4.4`.
- **Magento 2.3**: `2.3.0`, `2.3.1`, `2.3.2`, `2.3.3`, `2.3.4`, `2.3.5`, `2.3.6`, `2.3.7`.

_(All patched versions are also compatible)_

<br>

## Install Hiberus CLI

```bash
curl https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/setup.sh | bash
```

<br>

## Create environment
### Existing project
* You can launch following command for creating a Docker environment for an **existing project** (from the project directory):
```bash
cd <your_project>
hm setup
```
* If you want the files are outside your project to doesn't be tracked by git.
```bash
cd wrapper_folder
hm setup
``` 
Answer this question with the relative path of your project
<br>
<span style="color: steelblue;" >Magento root dir:  </span> <span style="color: #c0c0c0" ><your_project></span>

This will be the result
```
 ./wrapper_folder
    |__ config/
    |__ docker-compose.yml
    |__ docker-composedev.linux.yml
    |__ docker-composedev.mac.yml
    |__ <your_project>/
        |__ app/
        |__ ...
```
[setup doc](docs/setup.md)
---
### New project
You can launch following command for creating a Docker environment for a **new project** (from a new empty directory):
```bash
hm create-project
```
<br>


## Available commands

You can see all available commands by launching following command:
```bash
hm --help
```

<br>

## Thanks to

This project is based on [Dockergento](https://github.com/ModestCoders/magento2-dockergento). Special thanks to **ModestCoders** for their work.

<br>

## Copyright

[(c) Hiberus Tecnología](https://hiberus.com)

<br/>

## Licence

[GNU General Public License, version 3 (GPLv3)](https://opensource.org/licenses/gpl-3.0)
