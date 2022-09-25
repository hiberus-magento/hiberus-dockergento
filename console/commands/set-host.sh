#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

#
# Set base url in local etc/hosts en magento database
#
set_local_host() {
    if [ "$#" -gt 0 ]; then
        DOMAIN=$1
    fi

    # Add domain in /etc/hosts
    if ! grep -q "$DOMAIN" /etc/hosts; then
        print_info "Your system password is needed to add an entry to /etc/hosts...\n"
        echo "0.0.0.0 ::1 $DOMAIN" | sudo tee -a /etc/hosts
    fi

    if [[ "$#" -eq 1 ]] || [[ "$#" -gt 1  &&  "$2" != "--no-database" ]]; then
        print_info "Set "
        print_link "https://$DOMAIN/"
        print_info " to web/secure/base_url and web/secure/base_url."

        # Add domain in core_config_data table
        "$COMMANDS_DIR"/magento.sh config:set web/secure/base_url https://"$DOMAIN"/
        "$COMMANDS_DIR"/magento.sh config:set web/unsecure/base_url https://"$DOMAIN"/
    fi
}

set_local_host "$@"