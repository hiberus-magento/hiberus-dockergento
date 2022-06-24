#!/bin/bash
set -euo pipefail

#
# Set base url in local etc/hosts en magento database
#
set_local_host() {
    if [ "$#" -gt 0 ]; then
        DOMAIN=$1
    fi

    # Add domain in /etc/hosts
    if ! grep -q "${DOMAIN}" /etc/hosts; then
        echo "Your system password is needed to add an entry to /etc/hosts..."
        echo "0.0.0.0 ::1 ${DOMAIN}" | sudo tee -a /etc/hosts
    fi

    if [ "$#" -gt 1 ] && [ "$2" != "--no-database" ]; then
        echo -p "${YELLOW}Set ${BLUE}https://${DOMAIN}/${YELLOW} to web/secure/base_url and web/secure/base_url${COLOR_RESET}"
        # Add domain in core_config_data table
        "${COMMANDS_DIR}/magento.sh" config:set web/secure/base_url https://"${DOMAIN}"/
        "${COMMANDS_DIR}/magento.sh" config:set web/unsecure/base_url https://"${DOMAIN}"/
    fi
}

set_local_host "$@"
