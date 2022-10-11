#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

if [[ "$MACHINE" == "mac" ]]; then
    bind_mount_path=$("$TASKS_DIR"/get_bind_mount_path.sh "$WORKDIR_PHP/$MAGENTO_DIR/vendor")

    if [[ $bind_mount_path != false ]]; then
        print_error "\nVendor cannot be a bind mount. Please do the following:\n\n"
        print_default "  1. Remove from your docker-compose configuration:\n"
        print_default "      - ./<host_path>:$bind_mount_path\n\n"
        print_default "  2. Execute:\n"
        print_default "      $COMMAND_BIN_NAME rebuild\n\n"
        exit 1
    fi
fi
