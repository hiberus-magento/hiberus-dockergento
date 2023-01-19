#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

#
# Copy vendor and magento defaults files into container
#
mirror_vendor_host_into_container() {
    print_info "Mirror vendor into container before executing composer\n"

    if [ ! -d "$MAGENTO_DIR"/vendor ]; then
        print_processing "Creating '$MAGENTO_DIR/vendor' in host"
        mkdir -p "$MAGENTO_DIR/vendor"
    fi
    "$COMMANDS_DIR"/copy-to-container.sh vendor
}

#
# Synchronize al content from container to host
#
sync_all_from_container_to_host() {
    # Copy all default files magento into container
    default_files_magento=$(cat < "$DATA_DIR/default_files_magento.json" | jq -r 'keys | join(" ")')

    "$COMMANDS_DIR"/copy-to-container.sh $default_files_magento
    "$COMMANDS_DIR"/stop.sh

    print_info "Copying all files from container to host\n"
    print_processing "Removing vendor in host: '$MAGENTO_DIR/vendor/*'"
    rm -rf "$MAGENTO_DIR"/vendor/*

    print_processing "Copying 'phpfpm:${WORKDIR_PHP}/.' into '$MAGENTO_DIR"
    container_id=$($DOCKER_COMPOSE ps -q phpfpm)
    docker cp "$container_id":"$WORKDIR_PHP"/. "$MAGENTO_DIR"

    # Start containers again because we needed to stop them before mirroring
    "$COMMAND_BIN_NAME" start
}

# Exit when user tries composer create-project
if [[ "$#" != 0 && "$1" == "create-project" ]]; then
    print_error "create-project is not compatible with $COMMAND_BIN_NAME. Please use:\n"
    print_code "\n  $COMMAND_BIN_NAME create-project\n\n"
    exit 1
fi

# Manage composer commands
if [[ "$#" != 0 && 
    ("$1" == "install" || 
    "$1" == "update" || 
    "$1" == "require" || 
    "$1" == "remove") ]]; then

    # Check magento2-base
    module_path="$MAGENTO_DIR/vendor/magento/magento2-base/composer.json",0
    "$COMMANDS_DIR"/exec.sh [ -f $module_path ] && not_exits_in_container=false || not_exits_in_container=true
    [ -f $module_path ] && exits_in_host=true || exits_in_host=false

    if $exits_in_host && $exits_in_host; then
        rm -rf $module_path
    fi
    
    # Execute install con mirror wrapper
    if [[ "$#" != 0 && ("$MACHINE" == "mac") ]]; then
        mirror_vendor_host_into_container
        "$COMMANDS_DIR"/exec.sh composer "$@"
        sync_all_from_container_to_host
    else
        "$COMMANDS_DIR"/exec.sh composer "$@"
    fi
else
    "$COMMANDS_DIR"/exec.sh composer "$@"
fi

