#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_message.sh

if [[ "$MACHINE" != "mac" ]]; then
    print_warning "This command is only for mac system.\b"
    exit 1
fi

path_args=""
for watch_path in "$@"; do
    path_args="$path_args -path $watch_path"
done

$DOCKER_COMPOSE run --rm "$SERVICE_UNISON" watch "$path_args"
