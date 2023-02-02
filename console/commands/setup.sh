#!/usr/bin/env bash

set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh

export USE_DEAFULT_SETTINGS=false
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

    # install and use default
    print_info "\nIf your project has many custom modules it's possible that install command can fail.\n"
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
        else
            source "$HELPERS_DIR"/properties.sh
            save_properties
        fi
    else
      "$TASKS_DIR"/version_manager.sh
    fi
}

#
# Prepare final summary
#
summary_process() {
    print_info "\nSetup completed!!!\n\n"
    print_info "Open "
    print_link "https://$DOMAIN/\n\n"
}

#
# Execute setup command
#
setup_execute() {
    # Prepare environment
    if [[ -f "$CUSTOM_PROPERTIES_DIR"/properties.json ]]; then
        DOMAIN=${DOMAIN:=""}
        project_name=${project_name:-$COMPOSE_PROJECT_NAME}
        domain=${domain:-$DOMAIN}
        magento_root_directory=${magento_root_directory:-$MAGENTO_DIR}
    fi
    get_project_name ${project_name:=""}
    get_domain ${domain:=""}
    get_magento_root_directory ${magento_root_directory:=""}
    
    if [[ -z $dump ]] && ! ${install_option:=false}; then
        choice_database_mode_creation
    fi

    create_docker_compose 

    # Start services
    "$TASKS_DIR"/start_service_if_not_running.sh "nginx"
    # Magento installation
    "$TASKS_DIR"/magento_installation.sh "$dump"

    summary_process
}

# Process options
while getopts ":D:p:d:r:fui" options; do
    case "$options" in
        D)
            # Dump
            if [[ -f $OPTARG ]]; then
                dump="$OPTARG"
            else
                print_warning "No such file: $OPTARG\n"
            fi
        ;;
        p)
            # Project name
            project_name="$OPTARG"
        ;;
        d)
            # Domain
            domain="$OPTARG"
        ;;
        r)
            # Magento root 
            magento_root_directory="$OPTARG"
        ;;
        i)
            # Choise magento install option
            install_option=true
        ;;
        u)
            # Use saved user settings
            export USE_DEAFULT_SETTINGS=true
        ;;
        f)
            # Force
            force_setup=true
        ;;
        ?)
            print_error "The command is not correct\n\n"
            print_info "Use this format\n"
            source "$HELPERS_DIR"/print_usage.sh
            get_usage "$(basename ${0%.sh})"
            exit 1
        ;;
    esac
done

setup_execute