#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh

print_info "Starting containers in detached mode\n\n"

if [ "$#" == 0 ]; then
    $DOCKER_COMPOSE up -d
else
    $DOCKER_COMPOSE up -d "$@"
fi

"$TASKS_DIR"/validate_bind_mounts.sh

if [[ "$MACHINE" == "linux" ]]; then
    print_procesing "Waiting for everything to spin up..."
    sleep 5
    print_procesing "Fixing permissions"
    "$TASKS_DIR"/fix_linux_permissions.sh
    print_procesing "Permissions fix finished"
fi
