#!/bin/bash
set -euo pipefail

usage() {
  printf "${YELLOW}Usage:${COLOR_RESET}\n"
  echo "  create-project"
  echo ""
  printf "${YELLOW}Description:${COLOR_RESET}\n"
  echo "  This command creates a new magento project from scratch"
}

if [ "$#" != 0 ] && [ "$1" == "--help" ]; then
  usage
  exit 0
fi

overwrite_file_consent() {
  TARGET_FILE=$1

  if [[ -f "${TARGET_FILE}" ]]; then
    read -p "overwrite ${TARGET_FILE}? (y/n [n])? " ANSWER_OVERWRITE_TARGET
    if [ "${ANSWER_OVERWRITE_TARGET}" != "y" ]; then
      printf "${RED} Setup interrupted. This commands needs to overwrite this file.${COLOR_RESET}\n"
      exit 1
    fi
  fi
}

# ----------------------------------
# Get magento edition
# ----------------------------------
printf "${BLUE}Magento edition:\n${COLOR_RESET}"
AVAILABLE_MAGENTO_EDITIONS="community commerce"
DEFAULT_MAGENTO_EDITION="community"
select MAGENTO_EDITION in ${AVAILABLE_MAGENTO_EDITIONS}; do
    if $(${TASKS_DIR}/in_list.sh "${MAGENTO_EDITION}" "${AVAILABLE_MAGENTO_EDITIONS}"); then
        break
    fi
    if $(${TASKS_DIR}/in_list.sh "${REPLY}" "${AVAILABLE_MAGENTO_EDITIONS}"); then
        MAGENTO_EDITION=${REPLY}
        break
    fi
    echo "invalid option '${REPLY}'"
done

if [[ $MAGENTO_EDITION == '' ]]; then
  MAGENTO_EDITION=$DEFAULT_MAGENTO_EDITION
fi

# ----------------------------------
# Get magento version
# ----------------------------------
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

# ----------------------------------
# Create enviroment
# ----------------------------------
${COMMANDS_DIR}/setup.sh "${MAGENTO_VERSION}"

# [WIP] current point in refactor
overwrite_file_consent "${COMPOSER_DIR}/composer.json"
overwrite_file_consent ".gitignore"

if ! $(${TASKS_DIR}/in_list.sh "${MAGENTO_EDITION}" "${AVAILABLE_MAGENTO_EDITIONS}") ; then
  printf "${RED} Setup interrupted. Invalid edition '${MAGENTO_EDITION}' ${COLOR_RESET}\n"
  exit 1
fi

${TASKS_DIR}/start_service_if_not_running.sh ${SERVICE_APP}

CREATE_PROJECT_TMP_DIR="${COMMAND_BIN_NAME}-create-project-tmp"
${COMMANDS_DIR}/exec.sh sh -c "rm -rf ${CREATE_PROJECT_TMP_DIR}/*"
${COMMANDS_DIR}/exec.sh composer create-project --no-install --repository=https://repo.magento.com/ magento/project-${MAGENTO_EDITION}-edition ${CREATE_PROJECT_TMP_DIR} ${MAGENTO_VERSION}

echo " > Copying project files into host"
${COMMANDS_DIR}/exec.sh sh -c "cat ${CREATE_PROJECT_TMP_DIR}/composer.json > ${COMPOSER_DIR}/composer.json"
CONTAINER_ID=$(${DOCKER_COMPOSE} ps -q ${SERVICE_PHP})
if [ -f "${CREATE_PROJECT_TMP_DIR}/.gitignore" ]; then
  docker cp ${CONTAINER_ID}:${WORKDIR_PHP}/${CREATE_PROJECT_TMP_DIR}/.gitignore .gitignore
fi
${COMMANDS_DIR}/exec.sh sh -c "rm -rf ${CREATE_PROJECT_TMP_DIR}"

echo ""

if [[ "${MAGENTO_DIR}/vendor/bin" != "${BIN_DIR}" ]];
then
  printf "${YELLOW}Warning:${COLOR_RESET} bin dir is not inside magento dir\n"
  echo "  Magento dir: '${MAGENTO_DIR}'"
  echo "  Bin dir: '${BIN_DIR}'\n"
  printf "${YELLOW}Edit ${MAGENTO_DIR}/composer.json accordingly and execute:\n"
  echo ""
  echo "  ${COMMAND_BIN_NAME} composer install"
  echo ""
  exit 0
fi

# pending composer command
# ${COMMANDS_DIR}/composer.sh install
