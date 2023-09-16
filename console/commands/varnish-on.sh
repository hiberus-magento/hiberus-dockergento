#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

is_run_service "varnish"
is_run_service "phpfpm"

# Modify Varnish configuration
$DOCKER_COMPOSE exec -uroot varnish sed -i 's/^[^#]\+return(pass); #skip-varnish/#return(pass); #skip-varnish/g' /etc/varnish/default.vcl
"$COMMAMDS_DIR"/restart.sh varnish

# Enable full page cache
"$COMMAMDS_DIR"/magento.sh cache:enable full_page

print_info "Varnish cache enabled!\n"
