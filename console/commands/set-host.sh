#!/usr/bin/env bash
set -euo pipefail

DOMAIN="magento-${COMMAND_BIN_NAME}.local"

#
# Set base url in local etc/hosts en magento database
#
set_local_host() {
  if [ "$#" -gt 0 ]; then
    DOMAIN=$1
  fi

  # Add domain in /etc/hosts
  if ! grep -q ${DOMAIN} /etc/hosts; then
    echo "Your system password is needed to add an entry to /etc/hosts..."
    echo "0.0.0.0 ::1 ${DOMAIN}" | sudo tee -a /etc/hosts
  fi

  # Add domain in core_config_data table
  printf "${YELLOW}Set ${BLUE}https://${DOMAIN}/${YELLOW} to web/secure/base_url and web/secure/base_url${COLOR_RESET}\n"
  "${COMMANDS_DIR}/magento.sh" config:set web/secure/base_url https://${DOMAIN}/
  "${COMMANDS_DIR}/magento.sh" config:set web/unsecure/base_url https://${DOMAIN}/
}

set_local_host "$@"