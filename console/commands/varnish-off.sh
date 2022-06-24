#!/usr/bin/env bash
set -euo pipefail

if [ -z "$(docker ps | grep varnish)" ]; then
    printf "${RED}Error: Varnish is not running!${COLOR_RESET}\n"
    exit
fi
if [ -z "$(docker ps | grep phpfpm)" ]; then
    printf "${RED}Error: PHP is not running!${COLOR_RESET}\n"
    exit
fi

# Modify Varnish configuration
docker-compose exec -uroot varnish sed -i 's/#\+return(pass); #skip-varnish/ return(pass); #skip-varnish/g' /etc/varnish/default.vcl
docker-compose restart varnish

# Disable full page cache
docker-compose exec phpfpm bin/magento c:d full_page

printf "${GREEN}Varnish cache disabled!${COLOR_RESET}\n"
