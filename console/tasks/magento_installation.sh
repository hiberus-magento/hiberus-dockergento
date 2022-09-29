#!/usr/bin/env bash
set -euo pipefail 

source "$COMPONENTS_DIR"/input_info.sh

#
# Ask sql file and launch mysql import process
#
import_database() {
    print_question "Path of database (sql): "
    read -re dump

    if [[ -f "$dump" ]]; then
        "$COMMANDS_DIR"/mysql.sh < "$dump"
        "$COMMANDS_DIR"/mysql.sh "-e DELETE FROM admin_user;"
    else
        print_warning "No such file: $dump\n"
        import_database
    fi
}

#
# Create database
#
create_database() {
    if [[ $# -gt 0 && $1 == "setup" ]]; then
        flow_database_opt="mysql(recommended) install"

        print_info "If your project has many custom modules it´s possible that install command can fail.\n"
        print_question "Choose an option:\n"

        select REPLY in $flow_database_opt; do
            if [[ " $flow_database_opt " == *" $REPLY "* ]]; then
                if [[ $REPLY == mysql* ]]; then
                    import_database
                fi
                break
            fi
            echo "Invalid option '$REPLY'"
        done
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