#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/properties.sh
source "$HELPERS_DIR"/hosts.sh

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

    # Add domain in /etc/hosts, one entry per address family, repairing the malformed entry
    # earlier versions wrote. What that means and why is in helpers/hosts.sh.
    if hm_hosts_needs_repair /etc/hosts "$DOMAIN"; then
        print_info "Your system password is needed to add an entry to /etc/hosts...\n"

        #
        # Written through a copy rather than edited in place.
        #
        # `sed -i` is not portable: BSD sed takes the argument after `-i` as the backup
        # extension, so `sed -i -E` on macOS loses the extended regex, deletes nothing, exits 0
        # and leaves /etc/hosts-E behind. And a rename from a temporary directory would not
        # carry the owner, the mode and, on macOS, the file flags this file has, which is why it
        # is copied over the original.
        #
        local hosts_file
        hosts_file=$(mktemp)
        hm_hosts_repaired /etc/hosts "$DOMAIN" > "$hosts_file"

        # Never over an empty file: this one is how the machine resolves every name it knows
        if [ -s "$hosts_file" ]; then
            sudo cp "$hosts_file" /etc/hosts
        else
            print_error "/etc/hosts could not be read, so it was left alone.\n"
        fi

        rm -f "$hosts_file"
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