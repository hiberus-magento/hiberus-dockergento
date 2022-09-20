#!/usr/bin/env bash

source "$COMPONENTS_DIR"/print_message.sh

get_equivalent_version_if_exit() {
    equivalent_version=$("${TASKS_DIR}/get_equivalent_version.sh" "$1")
    if [[ "$equivalent_version" = "null" ]]; then
        print_warning "\nWe don't have support for the version $1 "
        print_info "\nPlease, write any version between all versions supported or press Ctrl - C to exit"
        ${COMMAND_BIN_NAME} compatibility
        read -r MAGENTO_VERSION
        get_equivalent_version_if_exit "$MAGENTO_VERSION"
    fi

    export EQUIVALENT_VERSION=$equivalent_version
}

#
# Get magento version
#
get_magento_version() {
    DEFAULT_MAGENTO_VERSION="2.4.4"

    if [ $# == 0 ]; then
        print_question "Magento version: " "$DEFAULT_MAGENTO_VERSION"
        read -r MAGENTO_VERSION

        if [[ $MAGENTO_VERSION == '' ]]; then
            MAGENTO_VERSION=$DEFAULT_MAGENTO_VERSION
        fi
    elif [[ $1 == '--yyy' ]]; then
        MAGENTO_VERSION=$DEFAULT_MAGENTO_VERSION
    else
        MAGENTO_EDITION=$1
    fi

    get_equivalent_version_if_exit "$MAGENTO_VERSION"
    export MAGENTO_VERSION=$MAGENTO_VERSION   
}

#
# Get magento edition
#
get_magento_edition() {
    AVAILABLE_MAGENTO_EDITIONS="community enterprise"
    DEFAULT_MAGENTO_EDITION="community"

    if [ $# == 0 ]; then
        print_question "Magento edition:\n"
        select MAGENTO_EDITION in ${AVAILABLE_MAGENTO_EDITIONS}; do
            if $("${TASKS_DIR}/in_list.sh" "${MAGENTO_EDITION}" "${AVAILABLE_MAGENTO_EDITIONS}"); then
                break
            fi

            if $("${TASKS_DIR}/in_list.sh" "${REPLY}" "${AVAILABLE_MAGENTO_EDITIONS}"); then
                MAGENTO_EDITION=$REPLY
                break
            fi
            echo "invalid option '${REPLY}'"
        done
    elif [[ $1 == '--yyy' ]]; then
        MAGENTO_EDITION=$DEFAULT_MAGENTO_EDITION
    else
        MAGENTO_EDITION=$1
    fi

    export MAGENTO_EDITION=$MAGENTO_EDITION
}

#
# Get base url
#
get_domain() {
    DEFAULT_DOMAIN="magento-${COMMAND_BIN_NAME}.local/"
    local PROJECT_NAME
    PROJECT_NAME=$(basename "$PWD")

    if [ $# == 0 ]; then
        print_question "Define domain " "$(echo $PROJECT_NAME | awk '{print tolower($0)}').local"
        read -r DOMAIN

        if [[ $DOMAIN == '' ]]; then
            DOMAIN="${PROJECT_NAME}.local"
        fi
    elif [[ $1 == '--yyy' ]]; then
        DOMAIN=$DEFAULT_DOMAIN
    else
        DOMAIN=$1
    fi

    # Transform domain name to lowercase
    DOMAIN=$(echo $DOMAIN | awk '{print tolower($0)}')

    export DOMAIN=$DOMAIN
}

#
# Ask magento directory
#
get_magento_root_directory() {
    print_question "Magento root dir " "$MAGENTO_DIR"

    read -r answer_magento_dir
    MAGENTO_DIR=${answer_magento_dir:-$MAGENTO_DIR}

    if [ "$MAGENTO_DIR" != "." ]; then
        print_info "Setting custom magento dir: '$MAGENTO_DIR'\n"
        MAGENTO_DIR=$(sanitize_path "$MAGENTO_DIR")
        print_warning "------ $DOCKER_COMPOSE_FILE ------\n"
        sed_in_file "s#/html/var/composer_home#/html/$MAGENTO_DIR/var/composer_home#gw /dev/stdout" "$DOCKER_COMPOSE_FILE"
        print_warning "--------------------\n"
        print_warning "------ $DOCKER_COMPOSE_FILE_MAC ------\n"
        sed_in_file "s#/app:#/$MAGENTO_DIR/app:#gw /dev/stdout" "$DOCKER_COMPOSE_FILE_MAC"
        sed_in_file "s#/vendor#/$MAGENTO_DIR/vendor#gw /dev/stdout" "$DOCKER_COMPOSE_FILE_MAC"
        print_warning "--------------------\n"
        print_warning "------ $DOCKER_CONFIG_DIR/nginx/conf/default.conf ------\n"
        sed_in_file "s#/var/www/html#/var/www/html/$MAGENTO_DIR#gw /dev/stdout" "$DOCKER_CONFIG_DIR/nginx/conf/default.conf"
        print_warning "--------------------\n"
    fi
}