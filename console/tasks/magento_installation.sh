#!/usr/bin/env bash
set -euo pipefail 

source "$COMPONENTS_DIR"/input_info.sh
source "$HELPERS_DIR"/binary.sh

#
# Ask sql file and launch mysql import process
#
create_database() {
    if [[ $# -gt 0 && -n $1 ]]; then
        "$COMMANDS_DIR"/mysql.sh < "$1"
        "$COMMANDS_DIR"/mysql.sh -q "DELETE FROM admin_user;"
    fi
    
    "$COMMANDS_DIR"/install.sh ${USE_DEFAULT_SETTINGS:+-u}

}

# Magento installation and database
"$COMMANDS_DIR"/composer.sh install
create_database "$@"

# Magento commands
hm_run_binary magento setup:upgrade
hm_run_binary magento deploy:mode:set developer

# Consolidate environment
"$COMMANDS_DIR"/restart.sh

# Domain and certificate
"$COMMANDS_DIR"/ssl.sh "$DOMAIN"
hm_run_binary set-host "$DOMAIN" --no-database
