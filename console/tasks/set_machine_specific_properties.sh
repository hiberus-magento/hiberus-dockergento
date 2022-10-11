#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

unameout="$(uname -s)"
case "$unameout" in
Linux*)
    MACHINE="linux"
    DOCKER_COMPOSE_FILE_MACHINE="${DOCKER_COMPOSE_FILE_LINUX}"
    ;;
Darwin*)
    MACHINE="mac"
    DOCKER_COMPOSE_FILE_MACHINE="${DOCKER_COMPOSE_FILE_MAC}"
    ;;
*)
    MACHINE="UNKNOWN"
    ;;
esac

if [[ "$MACHINE" == "UNKNOWN" ]]; then
    print_error "Error: Unsupported system type\n"
    print_error "System must be a Macintosh or Linux\n\n"
    print_error "System detection determined via uname command\n"
    print_error "If the following is empty, could not find uname command: $(which uname)\n"
    print_error "Your reported uname is: $(uname -s)\n"
    exit 1
fi

export MACHINE="$MACHINE"
export DOCKER_COMPOSE_FILE_MACHINE="${DOCKER_COMPOSE_FILE_MACHINE}"
