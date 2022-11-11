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

    "$COMMAND_BIN_NAME" copy-to-container vendor
}

#
# Synchronize al content from container to host
#
sync_all_from_container_to_host() {
    # Copy all default files magento into container
    default_files_magento=$(cat < "$DATA_DIR/default_files_magento.json" | jq -r 'keys | join(" ")')

    "$COMMAND_BIN_NAME" copy-to-container $default_files_magento

    "$COMMAND_BIN_NAME" stop

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
    print_error "\n  $COMMAND_BIN_NAME create-project\n\n"
    exit 1
fi

# Manage composer commands
if [[ "$#" != 0 && ("$1" == "install" || "$1" == "update" || "$1" == "require" || "$1" == "remove") ]]; then
    # Composer validation
    print_info "Validating composer before doing anything\n"
    validation_output=$("$COMMANDS_DIR"/exec.sh composer validate)
    if [ $? == 1 ]; then
        print_default "$validation_output"
        exit 1
    fi

    # Check magento2-base
    module_path="$MAGENTO_DIR/vendor/magento/magento2-base"
    exits_in_container=$("$COMMANDS_DIR"/exec.sh sh -c "[ -f $module_path/composer.json ] && echo true || echo false")
    exits_in_host=$([ -f "$module_path"/composer.json ] && echo true || echo false)

    if [[ $exits_in_host == true && $exits_in_container == *false* ]]; then
        print_error "Magento is not set up yet in container. Please remove 'magento2-base' and try again.\n"
        print_default "\n   rm -rf $module_path\n"
        exit 1
    fi

    # Execute install con mirror wrapper
    if [[ "$#" != 0 &&
        ("$MACHINE" == "mac") ]]; then
        mirror_vendor_host_into_container
        "$COMMAND_BIN_NAME" exec composer "$@"
        sync_all_from_container_to_host
    else
        "$COMMAND_BIN_NAME" exec composer "$@"
    fi
else
    "$COMMAND_BIN_NAME" exec composer "$@"
fi