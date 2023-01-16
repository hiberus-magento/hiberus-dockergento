#!/usr/bin/env bash

set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh

dump=""
domain=""
project_name=""
magento_root_directory=""
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

while getopts ":D:p:r:f" options; do
    case "$options" in
        f)
            # Force
            force_setup=true
        ;;
        D)
            # Dump
            if [[ -f ${OPTARG} ]]; then
                dump="${OPTARG}"
            else
                print_warning "No such file: ${OPTARG}\n"
                ask_dump
            fi
        ;;
        p)
            # Project name
            project_name="${OPTARG}"
            domain="$project_name.local"
            echo "project_name " $project_name
        ;;
        r)
            # Magento root 
            magento_root_directory=${OPTARG}
        ;;
        ?)
            print_error "The command is not correct\n\n"
            print_info "Use this format\n"
            source "$HELPERS_DIR"/print_usage.sh
            get_usage "setup"
            exit 1
        ;;
    esac
done

# Prepare environment
get_project_name $project_name
get_domain $domain
get_magento_root_directory $magento_root_directory
if [[ -n $dump ]]; then
    choice_database_mode_creation
fi
create_docker_compose 

 # Start services
"$TASKS_DIR"/start_service_if_not_running.sh "$SERVICE_APP"

# Magento installation
"$TASKS_DIR"/magento_installation.sh "$dump"

print_info "\nSetup completed!\n"

print_info "Open "
print_link "https://$DOMAIN/\n"