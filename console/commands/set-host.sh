#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/properties.sh

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

    # Add domain in /etc/hosts, one entry per address family.
    # Legacy versions wrote a single "0.0.0.0 ::1 <domain>" line, which is malformed:
    # glibc parses "::1" as an alias instead of an address, and Firefox refuses to
    # connect to 0.0.0.0. Repair any legacy entry before appending.
    if grep -qE "^0[.]0[.]0[.]0[[:space:]]+::1[[:space:]]+${DOMAIN//./\\.}([[:space:]]|$)" /etc/hosts \
        || ! grep -qE "[[:space:]]${DOMAIN//./\\.}([[:space:]]|$)" /etc/hosts; then
        print_info "Your system password is needed to add an entry to /etc/hosts...\n"
        sudo sed -i -E "/^0[.]0[.]0[.]0[[:space:]]+::1[[:space:]]+${DOMAIN//./\\.}([[:space:]]|$)/d" /etc/hosts
        printf '127.0.0.1 %s\n::1 %s\n' "$DOMAIN" "$DOMAIN" | sudo tee -a /etc/hosts
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