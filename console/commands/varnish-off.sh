#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

is_run_service "varnish"
is_run_service "phpfpm"

# Modify Varnish configuration
$DOCKER_COMPOSE exec -uroot varnish sed -i 's/#\+return(pass); #skip-varnish/ return(pass); #skip-varnish/g' /etc/varnish/default.vcl
"$COMMANDS_DIR"/restart.sh varnish

# Disable full page cache
"$COMMANDS_DIR"/magento.sh cache:disable full_page

"$COMMANDS_DIR"/purge.sh
"$COMMANDS_DIR"/magento.sh cache:clean

print_info "Varnish cache disabled!\n"
