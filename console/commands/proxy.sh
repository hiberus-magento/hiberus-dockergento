#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/version.sh
source "$TASKS_DIR"/proxy.sh

usage() {
    print_info "The global proxy: one per machine, so several projects can be up at once\n\n"
    print_default "  $COMMAND_BIN_NAME proxy up\n"
    print_default "  $COMMAND_BIN_NAME proxy down\n"
    print_default "  $COMMAND_BIN_NAME proxy status\n\n"
}

do_up() {
    if ! hm_proxy_compose_is_recent_enough; then
        hm_fail "$HM_EXIT_ERROR" "compose_too_old" \
            "The proxy needs Docker Compose $HM_PROXY_MIN_COMPOSE or newer, and this is $(get_docker_compose_version)" \
            "Update Docker Desktop, or leave the proxy off in your projects"
    fi

    if hm_proxy_is_running; then
        is_json_output && json_success "proxy" "$(jq -n '{running: true, started: false}')" ||
            print_info "The proxy is already running.\n"
        return 0
    fi

    local holder
    holder=$(hm_proxy_port_holder)

    if [ -n "$holder" ]; then
        hm_fail "$HM_EXIT_BLOCKED" "ports_taken" \
            "'$holder' is already using port 80 or 443, which the proxy needs" \
            "Stop that environment first: it does not go through the proxy"
    fi

    print_info "Starting the proxy...\n"
    hm_proxy_up

    if ! hm_proxy_is_running; then
        hm_fail "$HM_EXIT_DOCKER" "proxy_failed" \
            "The proxy did not start" \
            "docker compose -p $HM_PROXY_PROJECT -f $HM_PROXY_DIR/docker-compose.yml logs"
    fi

    if is_json_output; then
        json_success "proxy" "$(jq -n '{running: true, started: true}')"
        return 0
    fi

    print_info "Ready. It listens on 80 and 443, and routes by domain.\n"
}

do_down() {
    if ! hm_proxy_is_running; then
        is_json_output && json_success "proxy" "$(jq -n '{running: false, stopped: false}')" ||
            print_info "The proxy is not running.\n"
        return 0
    fi

    # Stopping the proxy takes every routed site down with it, so it is worth saying so
    print_info "Stopping the proxy. Any project routed through it becomes unreachable.\n"
    hm_proxy_down

    if is_json_output; then
        json_success "proxy" "$(jq -n '{running: false, stopped: true}')"
        return 0
    fi

    print_info "Stopped.\n"
}

do_status() {
    local running=false
    hm_proxy_is_running && running=true

    local routes=""
    $running && routes=$(hm_proxy_routes)

    if is_json_output; then
        json_success "proxy" "$(jq -n \
            --argjson running "$running" \
            --arg network "$HM_PROXY_NETWORK" \
            --arg routes "$routes" \
            '{running: $running, network: $network,
              routes: ($routes | split("\n") | map(select(length > 0) | split("\t") |
                  {host: .[0], status: .[1]}))}')"
        return 0
    fi

    printf '\n'
    if $running; then
        print_heading "The proxy is running\n\n"
    else
        print_heading "The proxy is not running\n\n"
        print_default "  $COMMAND_BIN_NAME proxy up\n\n"
        return 0
    fi

    if [ -z "$routes" ]; then
        print_default "  Nothing is routed through it yet.\n\n"
        return 0
    fi

    printf '%s\n' "$routes" | while IFS=$'\t' read -r host status; do
        [ -n "$host" ] && printf '  %-38s %s\n' "https://$host/" "$status"
    done
    printf '\n'
}

case "${1:-status}" in
    up)     do_up ;;
    down)   do_down ;;
    status) do_status ;;
    --help | -h) usage ;;
    *)
        hm_fail "$HM_EXIT_USAGE" "unknown_subcommand" \
            "'$1' is not something $COMMAND_BIN_NAME proxy does" \
            "$COMMAND_BIN_NAME proxy up | down | status"
        ;;
esac
