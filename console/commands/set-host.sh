#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/properties.sh
source "$HELPERS_DIR"/domain_resolution.sh

modify_database=true

#
# Set base url in local etc/hosts en magento database
#
set_local_host() {
    if [ "$#" -gt 0 ]; then
        DOMAIN=$1
        save_properties
        shift
    fi

    if [[ "$#" -gt 0 && $1 != "--no-database" ]]; then
        modify_database=false
        shift
    fi

    #
    # Only write to /etc/hosts when something has to be written.
    #
    # A wildcard resolver for the TLD — ours, or one the machine already had — makes the entry
    # pointless, and the entry is what costs a password prompt per project and leaves a line
    # behind forever.
    #
    if hm_domain_resolves_locally "$DOMAIN"; then
        print_info "$DOMAIN already resolves to this machine, so /etc/hosts was left alone.\n"
    elif ! grep -qE "[[:space:]]$DOMAIN([[:space:]]|$)" /etc/hosts; then
        print_info "Your system password is needed to add an entry to /etc/hosts...\n"
        echo "0.0.0.0 ::1 $DOMAIN" | sudo tee -a /etc/hosts
    fi

    if [[ -n "$DOMAIN" ]] && $modify_database; then
        print_info "Set "
        print_link "https://$DOMAIN/"
        print_info " to web/secure/base_url and web/secure/base_url.\n"

        # Add domain in core_config_data table
        "$COMMANDS_DIR"/magento.sh config:set web/secure/base_url https://"$DOMAIN"/
        "$COMMANDS_DIR"/magento.sh config:set web/unsecure/base_url https://"$DOMAIN"/
    fi
}

set_local_host "$@"