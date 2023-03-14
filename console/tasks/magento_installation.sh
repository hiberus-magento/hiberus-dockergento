#!/usr/bin/env bash
set -euo pipefail 

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

#
# Ask sql file and launch mysql import process
#
create_database() {
    if [[ $# -gt 0 && -n $1 ]]; then
        if [[ -e $MAGENTO_DIR/app/code ]]; then
            mv $MAGENTO_DIR/app/code $MAGENTO_DIR/app/code-tmp-for-install
        fi
        print_info "Importing database: "
        print_default "$1\n"
        "$COMMANDS_DIR"/mysql.sh < "$1"
        "$COMMANDS_DIR"/mysql.sh -q "DELETE FROM admin_user;"
        "$COMMANDS_DIR"/install.sh
        if [[ -e $MAGENTO_DIR/app/code-tmp-for-install ]]; then
            mv $MAGENTO_DIR/app/code-tmp-for-install $MAGENTO_DIR/app/code
        fi
    else
        "$COMMANDS_DIR"/install.sh
    fi
}

# Magento installation and database
"$COMMANDS_DIR"/composer.sh install
create_database "$@"

# Magento commands
"$COMMANDS_DIR"/magento.sh setup:upgrade
"$COMMANDS_DIR"/magento.sh deploy:mode:set developer

# Consolidate environment
"$COMMANDS_DIR"/restart.sh

# Domain and certificate
"$COMMANDS_DIR"/ssl.sh "$DOMAIN"
"$COMMANDS_DIR"/set-host.sh "$DOMAIN" --no-database
