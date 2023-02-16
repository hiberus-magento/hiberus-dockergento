#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

if [ -z "$(docker ps | grep phpfpm)" ]; then
    print_error "Error: PHP is not running!\n"
    exit
fi

docker-compose exec phpfpm bash -c "magento-cloud $@"