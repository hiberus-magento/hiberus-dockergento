#!/bin/bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

#
# Overwrite file consent
#
overwrite_file_consent() {
    local target_file=$1

    if [[ -f "$target_file" ]]; then
        print_question "Overwrite $target_file? [Y/n]? "
        read -r answer_overwrite_target
        if [ -z "$answer_overwrite_target" ]; then
            answer_overwrite_target="y"
        fi
        if [ "$answer_overwrite_target" != "y" ]; then
            print_error "Setup interrupted. This commands needs to overwrite this file."
            exit 1
        fi
    fi
}

#
# Initialize command script
#
init_docker() {
    # Get magento version information
    get_magento_edition
    get_magento_version
    get_domain

    # Create docker environment
    get_magento_root_directory
    "$TASKS_DIR"/version_manager.sh "$MAGENTO_VERSION"
    docker-compose -f docker-compose.yml up -d
    container_id=$($DOCKER_COMPOSE ps -q phpfpm)

    # Also make sure alternate auth.json is setup (Magento uses this internally)
    $COMMAND_BIN_NAME exec [ -d "./var/composer_home" ] && \
    $COMMAND_BIN_NAME exec cp /var/www/.composer/auth.json ./var/composer_home/auth.json
    
    # Execute composer create-project and copy composer.json
    $COMMAND_BIN_NAME exec composer create-project \
        --no-install \
        --repository=https://repo.magento.com/ \
        magento/project-"$MAGENTO_EDITION"-edition="$MAGENTO_VERSION" "$MAGENTO_DIR"

    # Copy all to host
    files_in_container=$($COMMAND_BIN_NAME exec ls $WORKDIR_PHP)
    docker cp "$container_id":"$WORKDIR_PHP"/composer.json "$MAGENTO_DIR"

    # Create empty composer.lock
    echo "{}" > "$MAGENTO_DIR"/composer.lock
    
    # Run docker-compose especified files of OS
    "$COMMAND_BIN_NAME" restart

    # Magento instalation
    "$TASKS_DIR"/magento_installation.sh

    print_info "Open "
    print_link "https://$DOMAIN/\n"
}

# Check if command "jq" exists
if ! command -v jq &>/dev/null; then
    print_error "Required 'jq' not found"
    print_link "https://stedolan.github.io/jq/download/\n"
    exit 0
fi

init_docker
