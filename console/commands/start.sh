#!/usr/bin/env bash
set -euo pipefail

stop_all=false
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/version.sh
source "$TASKS_DIR"/proxy.sh

start_execute() {
    if $stop_all ; then
        "$COMMANDS_DIR"/docker-stop-all.sh
    fi

    # A project routed through the proxy needs it up before it can be reached, and nobody should
    # have to remember that. It is not stopped on the way out: other projects depend on it.
    if hm_project_uses_proxy && ! hm_proxy_is_running; then
        local holder
        holder=$(hm_proxy_port_holder)

        if [ -n "$holder" ]; then
            source "$HELPERS_DIR"/exit_codes.sh
            hm_fail "$HM_EXIT_BLOCKED" "ports_taken" \
                "'$holder' is using port 80 or 443, which the proxy this project needs listens on" \
                "Stop that environment first, or set USE_PROXY to false here"
        fi

        print_info "Starting the global proxy\n"
        hm_proxy_up
    fi

    print_info "Starting containers in detached mode\n\n"

    if [ "$#" == 0 ]; then
        $DOCKER_COMPOSE up -d
        "$TASKS_DIR"/validate_bind_mounts.sh
    else  
        $DOCKER_COMPOSE up -d "$@"
    fi

    # What the platform needs afterwards, which on macOS is nothing. One copy of it, because the
    # Go implementation brings the environment up too and calls the same command
    "$COMMANDS_DIR"/post-start.sh
}

while getopts ":s" options; do
    case "$options" in
        s)
            stop_all=true
            shift
        ;;
        ?)
            source "$HELPERS_DIR"/print_usage.sh
            print_error "The command is not correct\n"
            print_info "Use this format\n"
            get_usage "$(basename ${0%.sh})"
            exit "$HM_EXIT_USAGE"
        ;;
    esac
done

start_execute "$@"
