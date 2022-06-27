#!/usr/bin/env bash
set -euo pipefail

if [[ "$MACHINE" != "mac" ]]; then
    echo -e "${RED} This command is only for mac system.${COLOR_RESET}"
    exit 1
fi

path_args=""
for watch_path in "$@"; do
    path_args="$path_args -path $watch_path"
done

$DOCKER_COMPOSE run --rm "$SERVICE_UNISON" watch "$path_args"
