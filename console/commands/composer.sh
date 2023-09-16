#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/array_manager.sh

#
# Copy vendor and magento defaults files into container
#
copy_vendor_to_container() {
    if [ ! -d "$MAGENTO_DIR"/vendor ]; then
        mkdir -p "$MAGENTO_DIR/vendor"
    fi
    "$COMMANDS_DIR"/copy-to-container.sh "vendor"
}

#
# Synchronize al content from container to host
#
sync_all_from_container_to_host() {
    container_id=$($DOCKER_COMPOSE ps -q phpfpm)

    # Copy all default files magento into container
    default_files_magento=$(cat < "$DATA_DIR/default_files_magento.json" | jq -r 'keys | join(" ")')

    "$COMMANDS_DIR"/copy-to-container.sh $default_files_magento
    "$COMMANDS_DIR"/stop.sh "phpfpm"

    print_info "Copying all files from container to host\n"
    print_processing "Removing vendor in host: $MAGENTO_DIR/vendor/*"
    rm -rf "$MAGENTO_DIR"/vendor/*

    print_processing "Copying phpfpm:${WORKDIR_PHP}/. into $MAGENTO_DIR"
    
    docker cp "$container_id":"$WORKDIR_PHP"/. "$MAGENTO_DIR"

    # Start containers again because we needed to stop them before mirroring
    "$COMMANDS_DIR"/start.sh "phpfpm"
}

#
# Exit when user tries composer create-project
#
check_create_project() {
    if [[ "$#" != 0 && "$1" == "create-project" ]]; then
        print_error "create-project is not compatible with $COMMAND_BIN_NAME. Please use:\n"
        print_code "\n  $COMMAND_BIN_NAME create-project\n\n"
        exit 1
    fi
}

#
# Manage composer commands
#
composer_excute() {
    check_create_project "$@"

    need_sync_container="install update require remove"
    if [[ "$MACHINE" == "mac" && "$#" != 0 ]] && in_array "$1" "$need_sync_container"; then  
        "$COMMANDS_DIR"/restart.sh "phpfpm"
        copy_vendor_to_container
        "$COMMANDS_DIR"/exec.sh composer "$@"
        sync_all_from_container_to_host
    else
        "$COMMANDS_DIR"/exec.sh composer "$@"
    fi
}

composer_excute "$@"