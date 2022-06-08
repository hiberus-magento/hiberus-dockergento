# Hiberus 

Docker environment for Magento 2 projects. Please visit our [Dockerhub repository](https://hub.docker.com/u/hiberusmagento).


## Docker images

- **Nginx**: `1.18`
- **PHP**: `7.2`, `7.3`, `7.4`, `8.1`
- **MySQL**: `5.7`, `8.0`
- **RabbitMQ**: `3.9`
- **Redis**: `5.0`, `6.2`
- **Elastic**: `5.6`, `6.5`, `7.17`
- **Mailhog**: `1.0`
- **Varnish**: `6.0`, `7.1`
- **Hitch**: `1.7`


## Magento compatible versions

- **Magento 2.4**: `2.4.0`, `2.4.1`, `2.4.2`, `2.4.3`, `2.4.4`.
- **Magento 2.3**: `2.3.0`, `2.3.1`, `2.3.2`, `2.3.3`, `2.3.4`, `2.3.5`, `2.3.6`, `2.3.7`.


## Install Hiberus CLI

1. Clone this repo

    ```bash
    cd ~
    git clone https://github.com/hiberus-magento/hiberus-dockergento.git
    ```

2. Add `hm` bin into your `$PATH`

    ```bash
    sudo ln -s $(pwd)/hiberus-dockergento/bin/run /usr/local/bin/hm
    ```
    
3. Open a new terminal tab/window and check that `hm` works

    ```bash
    which hm
    hm
    ```

4. Install `jq` dependency: [Download](https://stedolan.github.io/jq/download/)

## Thanks to

This project is based on [Dockergento](https://github.com/ModestCoders/magento2-dockergento). Special thanks to **ModestCoders** for their work. 


## Copyright

[(c) Hiberus Tecnología](https://hiberus.com)


## Licence

[GNU General Public License, version 3 (GPLv3)](https://opensource.org/licenses/gpl-3.0)

