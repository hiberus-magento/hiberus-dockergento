#!/usr/bin/env bash
set -euo pipefail

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
    echo "Error: Unsupported system type"
    echo "System must be a Macintosh or Linux"
    echo ""
    echo "System detection determined via uname command"
    echo "If the following is empty, could not find uname command: $(which uname)"
    echo "Your reported uname is: $(uname -s)"
    exit 1
fi

export MACHINE="$MACHINE"
export DOCKER_COMPOSE_FILE_MACHINE="${DOCKER_COMPOSE_FILE_MACHINE}"
