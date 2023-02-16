#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

if [ -z "$(docker ps | grep phpfpm)" ]; then
    print_error "Error: PHP is not running!\n"
    exit
fi

print_default "Access to following link and generate a new API Token for Magento Cloud CLI:\n"
print_link "https://accounts.magento.cloud/user/api-tokens\n\n"

docker-compose exec phpfpm bash -c "magento-cloud auth:api-token-login"