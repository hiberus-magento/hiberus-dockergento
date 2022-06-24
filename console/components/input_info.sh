#!/usr/bin/env bash

#
# Get magento version
#
get_magento_version() {
    DEFAULT_MAGENTO_VERSION="2.4.4"

    if [ $# == 0 ]; then
        printf "%bMagento version: %b[%s] " "$BLUE" "$COLOR_RESET" "$DEFAULT_MAGENTO_VERSION"
        read -r MAGENTO_VERSION

        if [[ $MAGENTO_VERSION == '' ]]; then
            MAGENTO_VERSION=$DEFAULT_MAGENTO_VERSION
        fi
    elif [[ $1 == '--yyy' ]]; then
        MAGENTO_VERSION=$DEFAULT_MAGENTO_VERSION
    else
        MAGENTO_EDITION=$1
    fi

    EQUIVALENT_VERSION=$("${TASKS_DIR}/get_equivalent_version.sh" "${MAGENTO_VERSION}")

    if [[ "null" == "$EQUIVALENT_VERSION" ]]; then
        echo -e "\n${RED}-----------------------------------------${COLOR_RESET}"
        echo -e "\n${RED}   The desired version is not supported${COLOR_RESET}"
        echo -e "\n${RED}-----------------------------------------${COLOR_RESET}\n"
        exit 1
    else
        export MAGENTO_VERSION=$MAGENTO_VERSION
        export EQUIVALENT_VERSION=$EQUIVALENT_VERSION
    fi
}

#
# Get magento edition
#
get_magento_edition() {
    AVAILABLE_MAGENTO_EDITIONS="community commerce"
    DEFAULT_MAGENTO_EDITION="community"

    if [ $# == 0 ]; then
        printf "%bMagento edition:%b\n" "$BLUE" "$COLOR_RESET"
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
        printf "%bDefine base url: %b[ https:/%s.local/ ] " "${BLUE}" "${COLOR_RESET}" "/${PROJECT_NAME}"
        read -r DOMAIN

        if [[ $DOMAIN == '' ]]; then
            DOMAIN="${PROJECT_NAME}.local"
        fi
    elif [[ $1 == '--yyy' ]]; then
        DOMAIN=$DEFAULT_DOMAIN
    else
        DOMAIN=$1
    fi

    export DOMAIN=$DOMAIN
}
