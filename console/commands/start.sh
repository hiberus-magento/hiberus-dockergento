#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

print_info "Starting containers in detached mode\n\n"

if [ "$#" == 0 ]; then
    $DOCKER_COMPOSE up -d
else
    $DOCKER_COMPOSE up -d "$@"
fi

"$TASKS_DIR"/validate_bind_mounts.sh

if [[ "$MACHINE" == "linux" ]]; then
    print_processing "Waiting for everything to spin up..."
    sleep 5
    print_processing "Fixing permissions"
    "$TASKS_DIR"/fix_linux_permissions.sh
    print_processing "Permissions fix finished"
    print_processing "Configuring self-routing domains..."
    "$TASKS_DIR"/set_etc_hosts.sh
    print_processing "All done!"
fi
