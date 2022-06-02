#!/bin/bash
set -euo pipefail

usage() {
  printf "${YELLOW}Usage:${COLOR_RESET}\n"
  echo "  create-project"
  echo ""
  printf "${YELLOW}Description:${COLOR_RESET}\n"
  echo "  This command creates a new magento project from scratch"
}

overwrite_file_consent() {
  local TARGET_FILE=$1

  if [[ -f "${TARGET_FILE}" ]];
  then
    printf "${RED} overwrite ${TARGET_FILE}? [y/n] ${COLOR_RESET}"
    read ANSWER_OVERWRITE_TARGET
    if [ "${ANSWER_OVERWRITE_TARGET}" != "y" ];
    then
      printf "${RED} Setup interrupted. This commands needs to overwrite this file.${COLOR_RESET}\n"
      exit 1
    fi
  fi
}

#
# Create composer.josn and composer.lock if these not exits. If exist composer project, get magento version
#
check_composer_files_exist() {
    if [ ! -f "${MAGENTO_DIR}/composer.json" ];
    then
        printf "${GREEN}Creating non existing '${MAGENTO_DIR}/composer.json'${COLOR_RESET}\n"
        mkdir -p ${MAGENTO_DIR}
        echo "{}" > ${MAGENTO_DIR}/composer.json
    fi

    if [ ! -f "${MAGENTO_DIR}/composer.lock" ];
    then
        printf "${GREEN}Creating non existing '${MAGENTO_DIR}/composer.lock'${COLOR_RESET}\n"
        echo "{}" > ${MAGENTO_DIR}/composer.lock
    fi
}

# 
# Get magento edition
# 
get_magento_edition() {
  printf "${BLUE}Magento edition:\n${COLOR_RESET}"

  AVAILABLE_MAGENTO_EDITIONS="community commerce"
  DEFAULT_MAGENTO_EDITION="community"
  select MAGENTO_EDITION in ${AVAILABLE_MAGENTO_EDITIONS};
  do
      if $(${TASKS_DIR}/in_list.sh "${MAGENTO_EDITION}" "${AVAILABLE_MAGENTO_EDITIONS}");
      then
          break
      fi
      if $(${TASKS_DIR}/in_list.sh "${REPLY}" "${AVAILABLE_MAGENTO_EDITIONS}");
      then
          MAGENTO_EDITION=${REPLY}
          break
      fi
      echo "invalid option '${REPLY}'"
  done

  if [[ $MAGENTO_EDITION == '' ]];
  then
    MAGENTO_EDITION=$DEFAULT_MAGENTO_EDITION
  fi
}

#
# Get magento version
# 
get_magento_version() {
  DEFAULT_MAGENTO_VERSION="2.4.4"
  printf "${BLUE}Magento version: ${COLOR_RESET}"

  read MAGENTO_VERSION

  if [[ $MAGENTO_VERSION == '' ]]; then
      MAGENTO_VERSION=$DEFAULT_MAGENTO_VERSION
  fi

  EQUIVALENT_VERSION=$(${TASKS_DIR}/get_equivalent_version.sh "${MAGENTO_VERSION}")
  if [[ "null" == "$EQUIVALENT_VERSION" ]];
  then
    echo -e "\n${RED}-----------------------------------------${COLOR_RESET}"
    echo -e "\n${RED}   The desired version is not supported${COLOR_RESET}"
    echo -e "\n${RED}-----------------------------------------${COLOR_RESET}\n"
    exit 1
  fi
}

#
#
#
check_vendor() {
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
}

init_docker() {
  overwrite_file_consent "${COMPOSER_DIR}/composer.json"
  check_composer_files_exist
  overwrite_file_consent ".gitignore"
  ${TASKS_DIR}/start_service_if_not_running.sh ${SERVICE_APP}
  CREATE_PROJECT_TMP_DIR="${COMMAND_BIN_NAME}-create-project-tmp"
  ${COMMANDS_DIR}/exec.sh sh -c "rm -rf ${CREATE_PROJECT_TMP_DIR}/*"
  ${COMMANDS_DIR}/exec.sh composer create-project --no-install --repository=https://repo.magento.com/ magento/project-${MAGENTO_EDITION}-edition ${CREATE_PROJECT_TMP_DIR} ${MAGENTO_VERSION}

  echo " > Copying project files into host"
  ${COMMANDS_DIR}/exec.sh sh -c "cat ${CREATE_PROJECT_TMP_DIR}/composer.json > ${COMPOSER_DIR}/composer.json"
  CONTAINER_ID=$(${DOCKER_COMPOSE} ps -q ${SERVICE_PHP})
  if [ -f "${CREATE_PROJECT_TMP_DIR}/.gitignore" ];
  then
    docker cp ${CONTAINER_ID}:${WORKDIR_PHP}/${CREATE_PROJECT_TMP_DIR}/.gitignore .gitignore
  fi
  ${COMMANDS_DIR}/exec.sh sh -c "rm -rf ${CREATE_PROJECT_TMP_DIR}"
}

# Check if command "jq" exists
if ! command -v jq  &> /dev/null
then
    printf "${RED}Required 'jq' not found${COLOR_RESET}\n"
    printf "${BLUE}https://stedolan.github.io/jq/download/${COLOR_RESET}\n"
    exit
fi

if [ "$#" != 0 ] && [ "$1" == "--help" ]
then
  usage
  exit 0
fi

get_magento_edition
get_magento_version
${COMMANDS_DIR}/setup.sh "${EQUIVALENT_VERSION}"
init_docker
check_vendor

# pending composer command
${COMMANDS_DIR}/composer.sh install