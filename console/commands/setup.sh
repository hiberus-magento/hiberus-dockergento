#!/usr/bin/env bash

set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$TASKS_DIR"/setup_options.sh

#
# Read first, act afterwards. Every option this command documents is accepted, in its short and
# long forms, and anything wrong with them is said before a single file is created — which is the
# fix for a dump path that does not exist: it used to be a warning, after which the command
# carried on and asked the question interactively, so a pipeline hung instead of failing.
#
hm_setup_parse_options "$@"

dump="$SETUP_DUMP"
force_setup=$SETUP_FORCE
mail_choice="$SETUP_MAIL"
project_name="$SETUP_PROJECT_NAME"
domain="$SETUP_DOMAIN"
magento_root_directory="$SETUP_ROOT"
install_option=$SETUP_INSTALL

$SETUP_USE_DEFAULT && export USE_DEFAULT_SETTINGS=true

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

setup_execute