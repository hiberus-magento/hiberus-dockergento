#!/usr/bin/env bash
set -euo pipefail

stop_all=false
source "$COMPONENTS_DIR"/print_message.sh

start_execute() {
    if $stop_all ; then
        "$COMMANDS_DIR"/docker-stop-all.sh
    fi

    print_info "Starting containers in detached mode\n\n"

    if [ "$#" == 0 ]; then
        $DOCKER_COMPOSE up -d
        "$TASKS_DIR"/validate_bind_mounts.sh
    else  
        $DOCKER_COMPOSE up -d "$@"
    fi

    if [[ "$MACHINE" == "linux" ]]; then
        print_processing "Waiting for everything to spin up..."
        sleep 5
        print_processing "Fixing permissions"
        "$TASKS_DIR"/fix_linux_permissions.sh
        print_processing "Permissions fix finished"
        print_processing "Configuring self-routing domains..."
        "$TASKS_DIR"/set_etc_hosts.sh
    fi
}

while getopts ":s" options; do
    case "$options" in
        s)
            stop_all=true
            shift
        ;;
        ?)
            source "HELPERS_DIR"/print_usage.sh
            print_error "The command is not correct\n"
            print_info "Use this format\n"
            get_usage "$(basename ${0%.sh})"
            exit 1
        ;;
    esac
done

start_execute "$@"
