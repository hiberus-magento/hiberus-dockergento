#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh

print_info "Rebuilding and starting containers in detached mode\n"

if [ "$#" == 0 ]; then
    $COMMAND_BIN_NAME stop
    $DOCKER_COMPOSE up --build -d "${SERVICE_APP}"
else
    $COMMAND_BIN_NAME stop "$@"
    $DOCKER_COMPOSE up --build -d "$@"
fi

# shellcheck source=/dev/null
"$TASKS_DIR"/validate_bind_mounts.sh

if [[ "$MACHINE" == "linux" ]]; then
    print_default "Waiting for everything to spin up...\n"
    sleep 5
    print_processing "Fixing permissions"
    "$TASKS_DIR"/fix_linux_permissions.sh
    print_processing "Permissions fix finished"
fi
