#!/usr/bin/env bash
set -euo pipefail

DOCKER_CONFIG_DIR="config/docker"

copy() {
    SOURCE_PATH=$1
    TARGET_PATH=$2
    mkdir -p $(dirname ${TARGET_PATH})
    cp -Rf ${SOURCE_PATH} ${TARGET_PATH}
}

sanitize_path() {
    PATH_TO_SANITIZE=$1

    SANITIZED_PATH=${PATH_TO_SANITIZE#/}
    SANITIZED_PATH=${SANITIZED_PATH#./}
    SANITIZED_PATH=${SANITIZED_PATH%/}
    echo "${SANITIZED_PATH}"
}

sed_in_file() {
    SED_REGEX=$1
    TARGET_PATH=$2
    if [[ "${MACHINE}" == "mac" ]];
    then
        sed -i '' "${SED_REGEX}" "${TARGET_PATH}"
    else
        sed -i "${SED_REGEX}" "${TARGET_PATH}"
    fi
}

add_git_bind_paths_in_file() {
    GIT_FILES=$1
    FILE_TO_EDIT=$2
    SUFFIX_BIND_PATH=$3

    BIND_PATHS=""
    while read FILENAME_IN_GIT; do
        if [[ "${MAGENTO_DIR}" == "${FILENAME_IN_GIT}" ]] || \
            [[ "${MAGENTO_DIR}" == "${FILENAME_IN_GIT}/"* ]] || \
            [[ "${FILENAME_IN_GIT}" == "vendor" ]] || \
            [[ "${FILENAME_IN_GIT}" == "${DOCKER_COMPOSE_FILE%.*}"* ]]; then
            continue
        fi
        NEW_PATH="./${FILENAME_IN_GIT}:/var/www/html/${FILENAME_IN_GIT}"
        BIND_PATH_EXISTS=$(grep -q -e "${NEW_PATH}" ${FILE_TO_EDIT} && echo true || echo false)
        if [ "${BIND_PATH_EXISTS}" == true ]; then
            continue
        fi
        if [ "${BIND_PATHS}" != "" ]; then
            BIND_PATHS="${BIND_PATHS}\\
      " # IMPORTANT: This must be a new line with 6 indentation spaces.
        fi
        BIND_PATHS="${BIND_PATHS}- ${NEW_PATH}${SUFFIX_BIND_PATH}"

    done <<< "${GIT_FILES}"

    printf "${YELLOW}"
    echo "------ ${FILE_TO_EDIT} ------"
    sed_in_file "s|# {FILES_IN_GIT}|${BIND_PATHS}|w /dev/stdout" "${FILE_TO_EDIT}"
    echo "--------------------"
    printf "${COLOR_RESET}"
}

calculate_version() {
    MAGENTO_COMPOSER_VERSION=$(cat "${PWD}/composer.json" | jq -r '.require | to_entries | map(select(.key | match("(magento)/(magento-cloud-metapackage|product-community-edition)"))) | .[].value')
    MAGENTO_VERSIONS=$(${TASKS_DIR}/get_magento_versions.sh)

    IFS=' '
    read -ra ADDR <<< "$MAGENTO_VERSIONS"
    for i in "${ADDR[@]}";
    do
        if [$i ]
    done
}

aks_version() {
    printf "${BLUE}Magento version:\n ${COLOR_RESET}"

    AVAILABLE_MAGENTO_VERSIONS=$(${TASKS_DIR}/get_magento_versions.sh)
    DEFAULT_MAGENTO_VERSION="2.4.4"
    select MAGENTO_VERSION in ${AVAILABLE_MAGENTO_VERSIONS}; do
        if $(${TASKS_DIR}/in_list.sh "${MAGENTO_VERSION}" "${AVAILABLE_MAGENTO_VERSIONS}"); then
            break
        fi
        if $(${TASKS_DIR}/in_list.sh "${REPLY}" "${AVAILABLE_MAGENTO_VERSIONS}"); then
            MAGENTO_VERSION=${REPLY}
            break
        fi
        echo "invalid option '${REPLY}'"
    done

    if [[ $MAGENTO_EDITION == '' ]]; then
        MAGENTO_VERSION=$DEFAULT_MAGENTO_VERSION
    fi

    echo "${MAGENTO_VERSION}"
}

# If there are arguments
if [ "$#" != 0 ];
then
    ${TASKS_DIR}/get_requeriments.sh $1
else
    ${TASKS_DIR}/get_requeriments.sh aks_version
fi

printf "${GREEN}Setting up docker config files${COLOR_RESET}\n"
copy "${COMMAND_BIN_DIR}/${DOCKER_CONFIG_DIR}/" "${DOCKER_CONFIG_DIR}"
copy "${COMMAND_BIN_DIR}/docker-compose/docker-compose.template.yml" "${DOCKER_COMPOSE_FILE}"
copy "${COMMAND_BIN_DIR}/docker-compose/docker-compose.dev.linux.template.yml" "${DOCKER_COMPOSE_FILE_LINUX}"
copy "${COMMAND_BIN_DIR}/docker-compose/docker-compose.dev.mac.template.yml" "${DOCKER_COMPOSE_FILE_MAC}"

#
# TODO: templating over docker-compose*
#

#
# Ask magento directory
#
printf "${BLUE}Magento root dir: ${COLOR_RESET}[${MAGENTO_DIR}] "
read ANSWER_MAGENTO_DIR

MAGENTO_DIR=${ANSWER_MAGENTO_DIR:-${MAGENTO_DIR}}

if [ "${MAGENTO_DIR}" != "." ];
then
	printf "${GREEN}Setting custom magento dir: '${MAGENTO_DIR}'${COLOR_RESET}\n"
    MAGENTO_DIR=$(sanitize_path "${MAGENTO_DIR}")
    printf "${YELLOW}"
    echo "------ ${DOCKER_COMPOSE_FILE} ------"
	sed_in_file "s#/html/var/composer_home#/html/${MAGENTO_DIR}/var/composer_home#gw /dev/stdout" "${DOCKER_COMPOSE_FILE}"
	echo "--------------------"
    echo "------ ${DOCKER_COMPOSE_FILE_MAC} ------"
	sed_in_file "s#/app:#/${MAGENTO_DIR}/app:#gw /dev/stdout" "${DOCKER_COMPOSE_FILE_MAC}"
	sed_in_file "s#/vendor#/${MAGENTO_DIR}/vendor#gw /dev/stdout" "${DOCKER_COMPOSE_FILE_MAC}"
    echo "--------------------"
    echo "------ ${DOCKER_CONFIG_DIR}/nginx/conf/default.conf ------"
    sed_in_file "s#/var/www/html#/var/www/html/${MAGENTO_DIR}#gw /dev/stdout" "${DOCKER_CONFIG_DIR}/nginx/conf/default.conf"
    echo "--------------------"
    printf "${COLOR_RESET}"
fi

#
# Create composer.josn and composer.lock if these not exits
#
if [ ! -f "${MAGENTO_DIR}/composer.json" ];
then
    printf "${GREEN}Creating non existing '${MAGENTO_DIR}/composer.json'${COLOR_RESET}\n"
    mkdir -p ${MAGENTO_DIR}
    echo "{}" > ${MAGENTO_DIR}/composer.json
fi

if [ ! -f "${MAGENTO_DIR}/composer.lock" ]; then
    printf "${GREEN}Creating non existing '${MAGENTO_DIR}/composer.lock'${COLOR_RESET}\n"
    echo "{}" > ${MAGENTO_DIR}/composer.lock
fi

#
# Update git setting in docker-compose
#
printf "${GREEN}Setting bind configuration for files in git repository${COLOR_RESET}\n"

if [[ -f ".git/HEAD" ]];
then
    GIT_FILES=$(git ls-files | awk -F / '{print $1}' | uniq)
    if [[ "${GIT_FILES}" != "" ]];
    then
        add_git_bind_paths_in_file "${GIT_FILES}" "${DOCKER_COMPOSE_FILE_MAC}" ":delegated"
    else
        echo " > Skipped. There are no files added in this repository"
    fi
else
    echo " > Skipped. This is not a git repository"
fi

#
# Set propierties in <root_project>/<docker_config>/propierties
#
printf "${GREEN}Saving custom properties file: '${DOCKER_CONFIG_DIR}/properties'${COLOR_RESET}\n"
cat << EOF > ./${DOCKER_CONFIG_DIR}/properties
MAGENTO_DIR="${MAGENTO_DIR}"
BIN_DIR="${BIN_DIR}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME}"
EOF

# Stop running containers in case that setup was executed in an already running project
${COMMANDS_DIR}/stop.sh

echo ""
printf "${YELLOW}-------- IMPORTANT INFO: -----------${COLOR_RESET}\n"
echo ""
echo "   Docker bind paths were automatically added here:"
echo ""
echo "      * ${DOCKER_COMPOSE_FILE_MAC}"
echo ""
echo "   Please check that they are right or edit them accordingly."
echo "   Be aware that vendor cannot be bound for performance reasons."
echo ""
printf "${YELLOW}-------------------------------------${COLOR_RESET}\n"

echo ""
printf "${GREEN}Hiberus docker set up successfully!${COLOR_RESET}\n"
echo ""
