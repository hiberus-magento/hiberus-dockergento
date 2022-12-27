#!/usr/bin/env bash
set -euo pipefail 

source "$COMPONENTS_DIR"/input_info.sh

#
# Ask sql file and launch mysql import process
#
create_database() {
    if [[ $# -gt 0 && -n $1 ]]; then
        "$COMMANDS_DIR"/mysql.sh < "$1"
        "$COMMANDS_DIR"/mysql.sh "DELETE FROM admin_user;"
    fi
    "$COMMANDS_DIR"/install.sh "$DOMAIN"
}

# Magento installation and database
"$COMMANDS_DIR"/composer.sh install
create_database "$@"

# Magento commands
"$COMMANDS_DIR"/magento.sh setup:upgrade
"$COMMANDS_DIR"/magento.sh deploy:mode:set developer

# Domain and certificate
"$COMMANDS_DIR"/ssl.sh "$DOMAIN"
"$COMMANDS_DIR"/set-host.sh "$DOMAIN" --no-database

"$COMMANDS_DIR"/restart.sh