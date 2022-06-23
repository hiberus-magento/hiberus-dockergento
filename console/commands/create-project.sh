#!/bin/bash
set -euo pipefail

#
# Overwrite file consent
#
overwrite_file_consent() {
    local TARGET_FILE=$1

    if [[ -f "${TARGET_FILE}" ]]; then
        printf "${RED}Overwrite %s? [y/n] ${COLOR_RESET}" "${TARGET_FILE}?"
        read -r ANSWER_OVERWRITE_TARGET
        if [ "${ANSWER_OVERWRITE_TARGET}" != "y" ]; then
            echo -e "${RED}Setup interrupted. This commands needs to overwrite this file.${COLOR_RESET}"
            exit 1
        fi
    fi
}

#
# Create composer.josn and composer.lock if these not exits. If exist composer project, get magento version
#
check_composer_files_exist() {
    if [ ! -f "${MAGENTO_DIR}/composer.json" ]; then
        echo -e "${GREEN}Creating non existing '${MAGENTO_DIR}/composer.json'${COLOR_RESET}"
        mkdir -p "${MAGENTO_DIR}"
        echo "{}" >"${MAGENTO_DIR}"/composer.json
    fi

    if [ ! -f "${MAGENTO_DIR}/composer.lock" ]; then
        echo -e "${GREEN}Creating non existing '${MAGENTO_DIR}/composer.lock'${COLOR_RESET}"
        echo "{}" >"${MAGENTO_DIR}"/composer.lock
    fi
}

#
# Check vendor/bin
#
check_vendor_bin() {
    if [[ "${MAGENTO_DIR}/vendor/bin" != "${BIN_DIR}" ]]; then
        echo -e "${YELLOW}Warning:${MAGENTO_DIR} bin dir is not inside magento dir\n"
        echo -e "  Magento dir: '${MAGENTO_DIR}"
        echo -e "  Bin dir: ${BIN_DIR}'\n"
        echo -e "${YELLOW}Edit ${MAGENTO_DIR}/composer.json accordingly and execute:\n"
        echo -e "  ${COMMAND_BIN_NAME} composer install\n"
    fi
}

#
# Initialize command script
#
init_docker() {
    source "${COMPONENTS_DIR}"/input_info.sh
    # Get magento version information
    get_magento_edition
    get_magento_version
    get_domain

    # Create docker environment
    ${COMMAND_BIN_NAME} setup "${EQUIVALENT_VERSION}" "${DOMAIN}"

    # Manage composer files
    overwrite_file_consent "${COMPOSER_DIR}/composer.json"
    check_composer_files_exist

    # Manage git files
    overwrite_file_consent ".gitignore"

    # Start services
    "${TASKS_DIR}/start_service_if_not_running.sh" "${SERVICE_APP}"

    # Create project tmp directory
    CREATE_PROJECT_TMP_DIR="${COMMAND_BIN_NAME}-create-project-tmp"
    ${COMMAND_BIN_NAME} exec sh -c "rm -rf ${CREATE_PROJECT_TMP_DIR}/*"

    # Execute composer create-project and copy composer.json
    ${COMMAND_BIN_NAME} exec composer create-project \
        --no-install \
        --repository=https://repo.magento.com/ \
        magento/project-"${MAGENTO_EDITION}"-edition \
        "${CREATE_PROJECT_TMP_DIR}" \
        "${MAGENTO_VERSION}"

    ${COMMAND_BIN_NAME} exec sh -c "cat ${CREATE_PROJECT_TMP_DIR}/composer.json > ${COMPOSER_DIR}/composer.json"

    # Copy .gitignore
    if [ -f "${CREATE_PROJECT_TMP_DIR}/.gitignore" ]; then
        CONTAINER_ID=$("${DOCKER_COMPOSE}" ps -q phpfpm)
        docker cp "${CONTAINER_ID}":"${WORKDIR_PHP}"/"${CREATE_PROJECT_TMP_DIR}"/.gitignore .gitignore
    fi

    # Remove temporal directory
    ${COMMAND_BIN_NAME} exec sh -c "rm -rf ${CREATE_PROJECT_TMP_DIR}"

    check_vendor_bin
    ${COMMAND_BIN_NAME} composer install

    ${COMMAND_BIN_NAME} install "$DOMAIN"

    # Magento commands
    ${COMMAND_BIN_NAME} magento setup:upgrade
    ${COMMAND_BIN_NAME} magento deploy:mode:set developer
    ${COMMAND_BIN_NAME} magento setup:static-content:deploy -f
    ${COMMAND_BIN_NAME} magento setup:di:compile

    ${COMMAND_BIN_NAME} ssl "$DOMAIN"
    ${COMMAND_BIN_NAME} set-host "$DOMAIN" --no-database

    echo -e "${YELLOW}Open ${BLUE}https://$DOMAIN/${COLOR_RESET}"
}

# Check if command "jq" exists
if ! command -v jq &>/dev/null; then
    echo -e "${RED}Required 'jq' not found${COLOR_RESET}"
    echo -e "${BLUE}https://stedolan.github.io/jq/download/${COLOR_RESET}"
    exit 0
fi

init_docker
