#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$HELPERS_DIR"/exit_codes.sh

#
# Stop every container on the machine.
#
# The name says what it does, and it still gets typed meaning "stop this". It reaches other
# people's projects and things that have nothing to do with Dockergento, so it says how far it
# reaches before doing it.
#
# Nothing is destroyed here — what is protected is somebody else's work in progress.
#

running=$(docker ps -q 2>/dev/null)

if [ -z "$running" ]; then
    print_warning "No containers running\n"
    exit 0
fi

total=$(printf '%s\n' "$running" | grep -c .)
mine=$(docker ps -q --filter "label=com.docker.compose.project=${COMPOSE_PROJECT_NAME:-}" 2>/dev/null |
    grep -c . || true)
others=$((total - ${mine:-0}))

if ! is_non_interactive; then
    printf '\n'
    print_warning "This stops $total container(s) on this machine.\n"

    if [ "$others" -gt 0 ]; then
        print_warning "$others of them do not belong to '${COMPOSE_PROJECT_NAME:-this project}'.\n"
    fi

    printf '\n'
    confirm "Stop them all? [y/N]: "

    case "$REPLY" in
        Y | y) ;;
        *)
            print_info "Nothing was stopped.\n"
            exit 0
            ;;
    esac
fi

print_info "Stopping $total container(s)\n"
docker stop $running
