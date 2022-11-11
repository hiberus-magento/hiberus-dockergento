#!/usr/bin/env bash

set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh

dump=""
force_setup=false

#
# Ask sql file and launch mysql import process
#
ask_dump() {
    read -rep "$(print_question "Path of database dump file (sql): ")" path

    # Fix error with home relative path
    if [[ $path = "~/"* ]]; then
        path=${path/"~"/$HOME}
    fi

    if [[ -f "$path" ]]; then
        dump=$path
    else
        print_warning "No such file: $path\n"
        ask_dump
    fi
}

#
# Ask to user if prefers to import database or to execute magento install command
#
choice_database_mode_creation() {
    flow_database_opt="SQL-Dump Magento-Installation"

    print_info "\nIf your project has many custom modules it´s possible that install command can fail.\n"
    print_question "Choose an option:\n"

    select REPLY in $flow_database_opt; do
        if [[ " $flow_database_opt " == *" $REPLY "* ]]; then
            if [[ $REPLY == SQL* ]]; then
                ask_dump
            fi
            break
        fi
        echo "Invalid option '$REPLY'"
    done
}

#
# Create docker-compose files
#
create_docker_compose() {
    if [[ -f "docker-compose.yml" ]]; then
        if $force_setup || [[ -z "$(cat "docker-compose.yml" | grep "hiberus-magento")" ]]; then
          "$TASKS_DIR"/version_manager.sh
        fi
    else
      "$TASKS_DIR"/version_manager.sh
    fi
}

while getopts ":f" options; do
    case "${options}" in
        f)
            force_setup=true
        ;;
        *);;
    esac
done

# Prepare environment
get_project_name
get_domain
get_magento_root_directory
choice_database_mode_creation
create_docker_compose

 # Start services
"$TASKS_DIR"/start_service_if_not_running.sh "$SERVICE_APP"

# Magento installation
"$TASKS_DIR"/magento_installation.sh "$dump"

print_info "\nSetup completed!\n"

print_info "Open "
print_link "https://$DOMAIN/\n"