#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

if [ -z "$(docker ps | grep varnish)" ]; then
    print_error "Error: Varnish is not running!\n"
    exit
fi
if [ -z "$(docker ps | grep phpfpm)" ]; then
    print_error "Error: PHP is not running!\n"
    exit
fi

# Modify Varnish configuration
docker-compose exec -uroot varnish sed -i 's/#\+return(pass); #skip-varnish/ return(pass); #skip-varnish/g' /etc/varnish/default.vcl
docker-compose restart varnish

# Disable full page cache
docker-compose exec phpfpm bin/magento c:d full_page

$COMMAND_BIN_NAME purge
$COMMAND_BIN_NAME magento ca:cl

print_info "Varnish cache disabled!\n"
