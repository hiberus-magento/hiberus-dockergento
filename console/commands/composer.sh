#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh

mirror_vendor_host_into_container() {
    print_info "Mirror vendor into container before executing composer\n"

    if [ ! -d "$MAGENTO_DIR/vendor" ]; then
        print_default " > creating '$MAGENTO_DIR/vendor' in host\n"
        mkdir -p "$MAGENTO_DIR/vendor"
    fi

    "$COMMAND_BIN_NAME" copy-to-container vendor
}

sync_all_from_container_to_host() {
    # IMPORTANT:
    # Docker cp from container to host needs to be done in a not running container.
    # Otherwise the docker.hyperkit gets crazy and breaks the bind mounts
    "$COMMAND_BIN_NAME" stop

    print_info "Copying all files from container to host\n"
    print_info " > removing vendor in host: '$HOST_DIR/$MAGENTO_DIR/vendor/*'\n"
    rm -rf "$HOST_DIR"/"$MAGENTO_DIR"/vendor/*

    print_info " > copying 'phpfpm:${WORKDIR_PHP}/.' into '$HOST_DIR\n"
    containser_id=$($DOCKER_COMPOSE ps -q phpfpm)
    docker cp "$containser_id":"$WORKDIR_PHP"/. "$HOST_DIR"

    # Start containers again because we needed to stop them before mirroring
    "$COMMAND_BIN_NAME" start
}

# Exit when user tries composer create-project
if [[ "$#" != 0 && "$1" == "create-project" ]]; then
    print_error "create-project is not compatible with $COMMAND_BIN_NAME. Please use:\n"
    print_error "\n  $COMMAND_BIN_NAME create-project\n\n"
    exit 1
fi

# Set composer directory
composer_dir_option="--working-dir=$COMPOSER_DIR"

# Exit if user wishes to set composer working directory
if [[ "$#" != 0 &&
    ($@ == *" -d "* || $@ == *" -d="* ||
    $@ == "-d "* || $@ == "-d="* ||
    $@ == *" --working-dir "* || $@ == *" --working-dir="* ||
    $@ == "--working-dir "* || $@ == "--working-dir="*) ]]; then
    print_error "Composer directory option not compatible with hiberus docker. This option is automatically set:\n"
    print_default "\n    --working-dir=$COMPOSER_DIR\n"
    exit 1
fi


if [[ "$#" != 0 &&
    ("$MACHINE" == "mac") &&
    ("$1" == "install" || "$1" == "update" || "$1" == "require" || "$1" == "remove") ]]; then
   
    print_info "Validating composer before doing anything\n"
    validation_output=$("$COMMANDS_DIR"/exec.sh composer validate "$composer_dir_option") ||
    if [ $? == 1 ]; then
        print_default "$validation_output"
        exit 1
    fi
print_info "Tras validacion
\n"
    module_path="$MAGENTO_DIR/vendor/magento/magento2-base"
    exitsts_in_container=$("$COMMANDS_DIR"/exec.sh sh -c "[ -f $module_path/composer.json ] && echo true || echo false")
    exitsts_in_host=$([ -f "$module_path"/composer.json ] && echo true || echo false)
    if [[ $exitsts_in_host == true && $exitsts_in_container == *false* ]]; then
        print_error "Magento is not set up yet in container. Please remove 'magento2-base' and try again.\n"
        print_default "\n   rm -rf $HOST_DIR/$module_path\n"
        exit 1
    fi

    # wrapper over composer
    mirror_vendor_host_into_container
    "$COMMAND_BIN_NAME" exec composer "$@" "$composer_dir_option"
    sync_all_from_container_to_host
else
    "$COMMAND_BIN_NAME" exec composer "$@" "$composer_dir_option"
fi
