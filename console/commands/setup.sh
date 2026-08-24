#!/usr/bin/env bash

set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh

dump=""
force_setup=false

#
# `--mail=<service>` is pulled out before getopts, which only understands short options.
#
mail_choice=""
setup_args=()

for argument in "$@"; do
    case "$argument" in
        --mail=*) mail_choice="${argument#--mail=}" ;;
        *)        setup_args[${#setup_args[@]}]="$argument" ;;
    esac
done

set -- ${setup_args[@]+"${setup_args[@]}"}

#
# Ask sql file and launch mysql import process
#
ask_dump() {
    custom_question "Path of database dump file (sql):"
    local path=$REPLY

    # Fix error with home relative REPLY
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
    local flow_database_opt=("Import sql Dump" "Magento installation")
    custom_select "How do you want create database?" "${flow_database_opt[@]}"

    if [[ $REPLY == "Import sql Dump" ]]; then
        ask_dump
    fi
}

#
# Which mail catcher this project uses.
#
# Asked only when the project has not decided yet: an existing project keeps what it has, and
# nobody is prompted about a service they already configured. Mailhog is the default answer, so
# accepting everything leaves a project exactly as it was built before this choice existed.
#
get_mail_service() {
    local current="${MAIL_SERVICE:-mailhog}"

    if [ -n "$mail_choice" ]; then
        current="$mail_choice"
    elif ! is_non_interactive && [ "${USE_DEFAULT_SETTINGS:-false}" != "true" ]; then
        local options=("mailhog" "mailpit")
        custom_select "Which mail catcher? (mailpit is the maintained one)" "${options[@]}"
        current="${REPLY:-mailhog}"
    fi

    case "$current" in
        mailhog | mailpit) ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "unknown_mail_service" \
                "'$current' is not a mail catcher this tool knows about" \
                "$COMMAND_BIN_NAME setup --mail=mailpit"
            ;;
    esac

    export MAIL_SERVICE="$current"
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
        DOMAIN=${DOMAIN:-}
        project_name=${project_name:-$COMPOSE_PROJECT_NAME}
        domain=${domain:-$DOMAIN}
        magento_root_directory=${magento_root_directory:-$MAGENTO_DIR}
    fi
    
    get_project_name "${project_name:-}"
    get_domain "${domain:-}"
    get_magento_root_directory "${magento_root_directory:-}"
    get_mail_service
    
    if [[ -z $dump ]] && ! ${install_option:-false}; then
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
            export USE_DEFAULT_SETTINGS=true
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
            exit "$HM_EXIT_USAGE"
        ;;
    esac
done

setup_execute