#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

is_run_service "varnish"
is_run_service "phpfpm"

# Modify Varnish configuration
docker-compose exec -uroot varnish sed -i 's/^[^#]\+return(pass); #skip-varnish/#return(pass); #skip-varnish/g' /etc/varnish/default.vcl
docker-compose restart varnish

# Enable full page cache
docker-compose exec phpfpm bin/magento cache:enable full_page

print_info "Varnish cache enabled!\n"
