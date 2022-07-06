#!/bin/bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/input_info.sh

#
# Overwrite file consent
#
overwrite_file_consent() {
    local target_file=$1

    if [[ -f "$target_file" ]]; then
        print_question "Overwrite $target_file? [Y/n]?"
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
# Create composer.josn and composer.lock if these not exits. If exist composer project, get magento version
#
check_composer_files_exist() {
    if [ ! -f "$MAGENTO_DIR/composer.json" ]; then
        print_info "Creating non existing '$MAGENTO_DIR/composer.json'\n"
        mkdir -p "$MAGENTO_DIR"
        echo "{}" >"$MAGENTO_DIR"/composer.json
    fi

    if [ ! -f "$MAGENTO_DIR/composer.lock" ]; then
        print_info "Creating non existing '$MAGENTO_DIR/composer.lock'\n"
        echo "{}" >"$MAGENTO_DIR"/composer.lock
    fi
}

#
# Check vendor/bin
#
check_vendor_bin() {
    if [[ "$MAGENTO_DIR/vendor/bin" != "$BIN_DIR" ]]; then
        print_warning "Warning:$MAGENTO_DIR bin dir is not inside magento dir\n"
        print_default "  Magento dir: '$MAGENTO_DIR\n"
        print_default "  Bin dir: $BIN_DIR'\n"
        print_warning "Edit $MAGENTO_DIR/composer.json accordingly and execute:\n"
        print_code "  $COMMAND_BIN_NAME composer install\n"
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

    # Manage composer files
    overwrite_file_consent "$COMPOSER_DIR/composer.json"
    check_composer_files_exist

    # Create docker environment
    $COMMAND_BIN_NAME setup "$EQUIVALENT_VERSION" "$DOMAIN"

    # Manage git files
    overwrite_file_consent ".gitignore"

    # Create project tmp directory
    CREATE_PROJECT_TMP_DIR="$COMMAND_BIN_NAME-create-project-tmp"
    $COMMAND_BIN_NAME exec sh -c "rm -rf $CREATE_PROJECT_TMP_DIR"

    # Execute composer create-project and copy composer.json
    $COMMAND_BIN_NAME exec composer create-project \
        --no-install \
        --repository=https://repo.magento.com/ \
        magento/project-"$MAGENTO_EDITION"-edition \
        "$CREATE_PROJECT_TMP_DIR" \
        "$MAGENTO_VERSION"

    $COMMAND_BIN_NAME exec sh -c "cat $CREATE_PROJECT_TMP_DIR/composer.json > $COMPOSER_DIR/composer.json"

    # Copy .gitignore
    if [ -f "$CREATE_PROJECT_TMP_DIR/.gitignore" ]; then
        CONTAINER_ID=$($DOCKER_COMPOSE ps -q phpfpm)
        docker cp "$CONTAINER_ID":"$WORKDIR_PHP"/"$CREATE_PROJECT_TMP_DIR"/.gitignore .gitignore
    fi

    # Remove temporal directory
    $COMMAND_BIN_NAME exec sh -c "rm -rf $CREATE_PROJECT_TMP_DIR"

    check_vendor_bin
    $COMMAND_BIN_NAME composer install

    $COMMAND_BIN_NAME install "$DOMAIN"

    # Magento commands
    $COMMAND_BIN_NAME magento setup:upgrade
    $COMMAND_BIN_NAME magento deploy:mode:set developer

    print_info "Open "
    print_question "https://$DOMAIN/\n"
}

# Check if command "jq" exists
if ! command -v jq &>/dev/null; then
    print_error "Required 'jq' not found"
    print_question "https://stedolan.github.io/jq/download/"
    exit 0
fi

init_docker
